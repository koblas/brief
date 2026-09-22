---
id: SCENARIO-12
status: done
---

# SCENARIO-12: --version --json in either order

## Scenario

```gherkin
Scenario: SCENARIO-12 --version --json in either order
  When I run "brief --version --json" or "brief --json --version"
  Then the document has the common header plus version (or "(devel)")
  And any other trailing token after --version is still "takes no arguments"
```

## User-visible contract

- `brief --version --json` / `brief --json --version` → stdout exactly one compact document
  `{"schema":1,"command":"brief","ok":true,"exit_code":0,"version":"<v>"}` + `\n`; `<v>` is
  `Main.Version` verbatim (no `brief ` prefix), `"(devel)"` when `ReadBuildInfo` reports
  `ok=false` or an empty version. stderr empty, exit 0.
- `brief --version` (no `--json`) → unchanged: `brief <v>\n`, exit 0.
- `brief --version extra --json`, `brief --version --json extra`, `brief --json --version
  extra` → usage-error JSON document (`kind` usage, `command` "brief", exit 2), `message` =
  the text line `brief: '--version' takes no arguments; run 'brief --version'`.
- `brief --version=x --json` → usage-error JSON document, `message` = `brief: '--version'
  takes no value; run 'brief --version'`, exit 2.
- `brief --version --json=x` → unchanged: text usage error (R5, S01), not a document.

## What exists (no step needed)

- `scanJSONFlag` already strips every `--json`, so `runRoot` sees `[--version]` in both
  orders and `out.json == true` — the relaxation itself is already in place; only the arm's
  output is wrong (prints text). `Test_version_with_json_relaxes_the_sole_argument_rule`
  (`json_usage_test.go`) currently pins that text output.
- Root usage errors already render as documents via `out.usageError` (the `forCommand(cmd)`
  reporter, `commandName` → "brief"). `--version extra --json` is already a row of
  `Test_json_mode_usage_error_message_is_the_text_mode_line`.
- `Test_reports_a_version_flag_with_trailing_arguments_as_taking_no_arguments`
  (`run_test.go`) no longer carries a `--version --json` row — verified by reading it; no
  test pins `--version --json` as an error.

## Implementation Plan

- [x] Step 1: `internal/cli/version_internal_test.go`
  `Test_version_json_is_one_exact_document` — one exact-bytes golden (`assert.Equal`, not
  `JSONEq`) through unexported `run` with a fake reader stamping a released tag, argv
  `--version --json` only (Step 3 owns "either order"); stderr empty, `NoError` (red —
  prints `brief v…` text today)
- [x] Step 2: `internal/cli/version_internal_test.go`
  `Test_version_json_reports_devel_when_the_build_stored_no_version` — decode `version`
  against fake readers: empty version, `ok=false` with a stamped version, no build info
  (`nil,false`); each expects `"(devel)"` (red)
- [x] Step 3: `internal/cli/json_usage_test.go`
  `Test_version_with_json_relaxes_the_sole_argument_rule` — rewrite from the text pin to the
  black-box document pin through `cli.Run`: both argv orders, exit 0, stderr empty, header
  `command` "brief" / `ok` true / `exit_code` 0, `version` equal to the value plain
  `brief --version` prints in the same test binary with its `brief ` prefix trimmed (captured,
  not a literal). Update its doc comment: no longer defers to S12 (red)
- [x] Step 4: `internal/cli/cli.go` `versionString(readBuildInfo)` — extract the bare-version
  rule (verbatim `Main.Version`, `"(devel)"` on `ok=false` or empty) out of `versionLine`;
  `versionLine` becomes `"brief " + versionString(...)`. Existing text version tests stay
  green (refactor inside the green phase)
- [x] Step 5: `internal/cli/cli.go` `versionDocument` — `jsonHeader` embedded first, by value,
  then `Version string \`json:"version"\`` (new)
- [x] Step 6: `internal/cli/cli.go` `runRoot` `argVersionFlag` sole-arg branch — when
  `out.json`, write `versionDocument{out.successHeader(), versionString(readBuildInfo)}` via
  `writeJSONDocument` and return its wrapped error (the `runFinish` pattern); otherwise the
  unchanged text line. The JSON branch runs before any text write (R1). `out` is already
  `forCommand(cmd)` at `runRoot`'s call site, so `command` renders "brief" — the Step 1
  golden confirms it. Update `runRoot`'s doc comment to name the JSON arm (green)
- [x] Step 7: `internal/cli/json_usage_test.go`
  `Test_json_mode_usage_error_message_is_the_text_mode_line` — add two rows, command "brief":
  `--version=x --json` (textArgs `--version=x`; the value arm) and `--json --version extra`
  (textArgs `--version extra`; `--json` leading). The existing `--version extra --json` row
  already covers the trailing-arg arm; add no third ordering. Expected green on arrival
  (stripping + `out.usageError` already cover them) — report it as such, do not manufacture
  a red
- [x] Step 8: mutation verification (stash per `.claude/rules/agent-briefs.md`), one at a
  time, each restored byte-identical:
  (a) drop the `out.json` branch in the `argVersionFlag` arm → Steps 1, 2, 3 red;
  (b) put `versionLine` (prefixed) into the document instead of `versionString` → Step 1
  red on the `version` field;
  (c) drop the `ok` check in `versionString` → Step 2's `ok=false` row red AND the existing
  text `Test_version_flag_reports_devel_when_the_build_stored_no_version` rows red (proves
  both paths share the one rule);
  (d) widen the arm's guard `len(args) == 1` → `>= 1` → Step 7's `--json --version extra`
  row and the existing `--version extra --json` row red, plus the text rows of
  `Test_reports_a_version_flag_with_trailing_arguments_as_taking_no_arguments` (say so in
  the comment); control: the `--version=x --json` row stays green.
  Record each mutation and the test it reddened in the test doc comments
- [x] Step 9: `docs/specifications/version-flag/specification.md` — two hunks, both required
  together: drop `or "brief --version --json"` from SCENARIO-03's When line, AND rewrite
  Product Verdict item 3's trailing clause so it no longer cites that row (e.g. "…plus
  `version`; SCENARIO-03 no longer covers `--version --json`"). `SCENARIO-03.md` is audit
  trail — leave it (update)
- [x] Step 10: `go build ./...`, `go test ./...` (unpiped, report count + delta),
  `go test -race ./internal/cli/...`, `golangci-lint run ./...` all green → mark SCENARIO-12
  done in `specification.md`; rewrite `STATE.md` (drop `--version` from S12/S13's "Left
  unbuilt" entry; add the binding decisions below)

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `versionString(readBuildInfo)` is the one version rule; `versionLine` = `"brief " +
  versionString`, and the JSON `version` field is the bare `versionString` — the text line and
  the document must never disagree on the `(devel)` fallback.
- `--version --json` document is `{header, version}`, `command` "brief" (root's
  `commandName`), always exit 0 — asking for the version never fails, in either mode.
- The root sole-argument relaxation is a consequence of `scanJSONFlag` stripping, not a
  special case in `runRoot`: `runRoot` still requires `len(args) == 1` after stripping, so
  any other trailing token (before or after `--json`) remains "takes no arguments". Do not
  add a `--json` check inside `runRoot`'s argument count.
- A value on `--version` still wins over a trailing argument (version-flag R8), in JSON mode
  too.

**Left unbuilt** — named so nobody assumes it exists:
- `help --json` / `<cmd> --help --json` document and `completion --json` document — S13;
  root `--help --json` still prints text help.
- `--json` help row on every command — S14.

**Traps** — things that look right and are not:
- A go test binary's `debug.ReadBuildInfo` reports `(devel)`, which is byte-identical to the
  fallback: a black-box `cli.Run` test cannot distinguish pass-through from fallback. Pin
  verbatim/fallback only through unexported `run` with a fake reader.
- `writeJSONDocument` returns an error; the `--version` JSON arm must return it, not `_ =` it
  (only `usageError` discards, deliberately).
