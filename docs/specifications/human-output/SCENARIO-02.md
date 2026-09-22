---
id: SCENARIO-02
status: done
depends-on: []
---

# SCENARIO-02: --json turns refusals into the error document

## Scenario

```gherkin
Scenario: SCENARIO-02 --json turns refusals into the error document
  Given a feature whose state file is missing
  When I run "brief start demo --json"
  Then stdout is one document with ok false, exit_code 1, error.kind "refusal", and absolute path, problem and fix filled in
  And stderr is empty and the exit code is 1
```

Rules: R1 (one document on stdout, stderr empty, exit codes unchanged), R3 (error object),
R4 (check findings are not an error), R6 (absolute paths in JSON).

## User-visible contract

Every refusal/failure path that goes through `renderRefusal` today, on every command that
can refuse (`start`, `status`, `check`, `finish`, `new feature`, `new step`):

- **Text mode**: byte-identical to today. Same one line on stderr, empty stdout, exit 1.
- **`--json` mode**: one compact document on stdout, zero bytes on stderr, exit 1:
  header `schema 1`, `command` (`commandName`), `ok false`, `exit_code 1`, then `error`:
  - `kind`: `"refusal"` for `*config.InvalidConfigError`, `*scaffold.RefusalError`,
    `*assemble.RefusalError`, and a bare not-found sentinel (`assemble.ErrNoSuchFeature`
    — assemble returns it bare, e.g. `start ghost`, `check ghost`; scaffold always wraps it); `"failure"` for anything else.
  - `message`: the exact text-mode stderr line for the same argv, no trailing newline
    (including `(no files changed)` where the text line has it).
  - `path`: absolute. config → `InvalidConfigError.Path`; scaffold/assemble → the refusal's
    `Path`, joined onto `wd` when relative (finish's `--handoff`/`--state` substitution);
    `null` for `<stdin>`, for bare not-found, and for generic failure.
  - `line`: the refusal's `Line` when > 0, else `null` (always `null` for config/generic).
  - `problem`: config → flattened `Err` text; scaffold → flattened `Problem`; assemble →
    flattened `Detail`; bare not-found and generic → flattened `err.Error()`.
  - `fix` (always filled): config → `fix it or remove it to fall back to the shipped
    defaults`; scaffold/assemble → flattened `Fix`; bare not-found →
    `run 'brief status' to list the known features`; generic →
    `resolve the problem, then run '<usageHint(cmd)>' again`.
  - `files_changed`: `filesChangedFor(command)` — `false` for finish / new feature / new
    step, `null` for start / status / check.
- **Not error documents under `--json` (interim, stated here so nobody "fixes" them early)**:
  - `check --json` with findings → exactly the text-mode stdout (findings) and stderr
    (summary), exit 1 via `errCheckFindings`, no document. SCENARIO-09 owns the payload.
    `check --json` with no findings → text path, exit 0.
  - `status --json` with a malformed feature → text table + per-feature stderr lines, exit 0.
    SCENARIO-07 owns the payload.
  - `start --json` success → bare `assemble.Brief` JSON (SCENARIO-03).

## Implementation Plan

- [x] Step 1: `internal/cli/json_refusal_test.go` `Test_json_mode_renders_a_refusal_as_one_document` — exact-bytes golden for the Gherkin row (`start demo --json`, STATE.md missing; expected bytes built from the fixture's own `wd`) pinning key order, `null` line, `null` files_changed (red)
- [x] Step 2: `internal/cli/json_refusal_test.go` `Test_json_mode_refusal_matrix` — table through `cli.Run`, one representative per command plus every shape: start missing STATE.md (assemble), start unknown feature (bare not-found), status invalid `.brief.yaml` (config), check invalid `.brief.yaml` (config), check unknown feature (bare not-found), finish unknown step (scaffold), finish `--state` relative path whose content fails conformance (scaffold, path joined onto wd), finish `--state -` failing conformance (path `null`), finish `--handoff` nonexistent file (generic → `failure`), new feature on an existing feature (scaffold), new step on an unknown feature (scaffold). Each row asserts header, kind, exit 1, empty stderr, `message` == same argv's no-`--json` stderr line, fixture-derived `path` / `line`, `problem` and `fix` both non-empty and both contained in `message` where the text line carries them, `files_changed` per command (red)
- [x] Step 3: `internal/cli/json_refusal_test.go` `Test_json_mode_check_findings_are_not_an_error_document` — `check --json` on a feature with an ERROR finding: exit 1, stdout and stderr byte-identical to the same run without `--json`; control arm: `check --json ghost` on the same fixture does produce a document (green on arrival for the pin; state so)
- [x] Step 4: `internal/cli/json_refusal_test.go` `Test_json_mode_status_problem_stays_text_until_its_payload_lands` — `status --json` with one malformed feature: exit 0, output byte-identical to text mode (green on arrival; state so)
- [x] Step 5: `internal/cli/json.go` — `errorKindRefusal`, `errorKindFailure` constants; `wd` field on `reporter`; update the `jsonError` doc comment (path/line/problem are now filled by refusals) (new/update)
- [x] Step 6: `internal/cli/cli.go` `run` — set `wd` on the base `reporter` (update)
- [x] Step 7: `internal/cli/refusal.go` — split `renderRefusal` into an unexported pure classifier (err → kind, text line, path, line, problem, fix) and a `(reporter) refusal(err error) error` method that writes the line to stderr in text mode or the `errorDocument` to stdout in JSON mode (path absolutized against `r.wd`, `<stdin>` → null), returning err unchanged; delete `renderRefusal`. Classifier order is load-bearing: the three typed errors first, the bare `assemble.ErrNoSuchFeature` check after them (`scaffold.RefusalError` wraps `scaffold.ErrNoSuchFeature`, so a sentinel-first order strips its path/problem/fix and changes its text line). Absolutization touches only the JSON `path` field — the text line keeps the refusal's path exactly as today (green)
- [x] Step 8: `internal/cli/{start,status,check,finish,new}.go` — replace every `renderRefusal(out.stderr, "<cmd>", err)` with `out.refusal(err)`; leave `errCheckFindings` and status's per-row problem loop untouched (update)
- [x] Step 9: `internal/cli/start_test.go` — rewrite `Test_start_json_writes_no_bytes_to_stdout_when_it_refuses` (renamed to say it writes the error document) into the document assertion for both of its subtests; do not delete it — SCENARIO-01 handed it to this scenario to rewrite (update)
- [x] Step 10: update the `cli.go` comment near line 48 that names `renderRefusal(stderr, "<command>", err)` (update)
- [x] Step 11: mutation-verify individually (stash each, name the test that reddens): drop the config branch; drop the relative-path join; map `<stdin>` to a path; drop the bare not-found → refusal classification; move the bare-sentinel check above the typed checks (must redden the new-step-unknown-feature row's fixture-derived `path` assertion — the message-vs-text check cannot catch it, since both sides come from the same classifier); absolutize the path in the text line too (must redden the finish relative `--state` row's `message` check); route `errCheckFindings` through `out.refusal`; write the JSON document and the stderr line both
- [x] Step 12: `go build ./...`, `go test ./...` (unpiped, report count + delta), `go test -race ./internal/cli/...`, `golangci-lint run ./...` → mark SCENARIO-02 done in specification.md, rewrite STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `(reporter) refusal(err) error` is the one refusal seam; `renderRefusal` is gone. Every
  exit-1 refusal/failure renders through it — later text changes (S04 relative paths, S05
  known-features list) change the classifier's text line, and `message` follows for free.
- `message` and the text line come from the same classifier output — S04/S05/S06 must keep
  it that way, never build the text line separately.
- JSON `path` is absolute: joined onto `reporter.wd` when relative; `<stdin>` → `null`.
  R6's text-side relativization (S04+) must not leak into the JSON `path`.
- `kind` = `refusal` for the three typed refusals and bare `assemble.ErrNoSuchFeature`
  (assemble only returns it bare; scaffold always wraps it in a `RefusalError`); `failure`
  for everything else. Typed checks run before the sentinel check.
- Fallback fixes: config `fix it or remove it to fall back to the shipped defaults`; bare
  not-found `run 'brief status' to list the known features` (S05 may replace it with its
  known-features fix — say so); generic `resolve the problem, then run '<usageHint>' again`.
- `errCheckFindings` never renders an error document (R4). Until S09, `check --json` with
  findings prints exactly its text output, exit 1.

**Left unbuilt** — named so nobody assumes it exists:
- `check --json` findings payload — SCENARIO-09.
- `status --json` payload and per-feature `problem` object — SCENARIO-07.
- Header on `start --json` success — SCENARIO-03.
- Bare not-found `path` (the feature directory) and its known-features `fix` — SCENARIO-05.
- Write-failure paths returning `fmt.Errorf("brief <cmd>: %w", err)` directly (e.g.
  `RenderJSON`/`RenderFindings` write errors) do not render anything; they stay unrendered.

**Traps** — things that look right and are not:
- `assemble.Start` / `Check` return **bare** `ErrNoSuchFeature`, not a `*RefusalError` —
  without the sentinel check they classify as `failure`. The reverse trap: `errors.Is` on
  the sentinel also matches `scaffold.RefusalError`, so the sentinel check must come last.
- finish rewrites `scaffold.RefusalError.Path` to the user's `--state`/`--handoff` argument,
  which may be relative or `<stdin>`; every other refusal path is already absolute.
- `usageError` in `cli.go` (package func, text-only) is not `reporter.usageError`; the
  `--json=<v>` path must keep using the text one.
- A `problem ⊂ message` assertion is only meaningful where the text line carries the problem
  verbatim (all four shapes do today after `flattenOneLine`); assert against the flattened form.
