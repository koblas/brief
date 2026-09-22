---
id: SCENARIO-09
status: done
---

# SCENARIO-09: check --json

## Scenario

Scenario: SCENARIO-09 check --json
  When I run "brief check --json"
  Then the document has counts {error, warn} and features[] with findings {severity, rule, absolute path, line or null, detail}
  And the exit code is 1 when any ERROR exists, with no error object

## User-visible contract

- `brief check [feature] --json` → exactly one compact document + `\n` on stdout, **zero
  stderr bytes** on every non-refusal path (findings, WARN-only, no findings, named feature).
- Document key order: `{"schema":1,"command":"check","ok":<bool>,"exit_code":<int>,
  "counts":{"error":N,"warn":M},"features":[{"name","path","in_flight","findings":[
  {"severity","rule","path","line","detail"}]}]}`. No `error` key, ever, on these paths.
- Any ERROR finding → `ok:false`, `exit_code:1`, process exit 1 (R2 `ok == (exit_code==0)`,
  R4 no error object). WARN-only or no findings → `ok:true`, `exit_code:0`, exit 0.
- No findings → `"counts":{"error":0,"warn":0},"features":[]` (never `null`).
- `severity` is `"ERROR"`/`"WARN"` (upper-case, the `assemble.Severity` value, same as text).
  `rule` is the R8 id. `path` and feature `path` absolute. `line` is `null` when
  `Finding.Line == 0` (whole-file), else the integer. `detail` raw (never flattened).
- `features[]` in `GroupByFeature` order (Check's emission order, not sorted); `findings[]`
  in within-group order. A feature with no findings is absent.
- Unchanged: unknown feature / invalid config → R3 refusal document, exit 1; too many args /
  bad flag → usage document, exit 2.

## Implementation Plan

- [x] Step 1: `internal/cli/json_refusal_test.go` `Test_json_mode_check_findings_are_not_an_error_document` — **rewrite, do not delete** S02's interim pin: `check --json` on the ERROR fixture now decodes as one document with `ok:false`, `exit_code:1`, no `"error"` key, `counts.error > 0`, and `assert.Empty(stderr)` (not a text-vs-json comparison — text stderr carries the summary); exit 1; keep the `check --json ghost` control arm unchanged (red)
- [x] Step 2: `internal/cli/check_json_test.go` `Test_check_json_document_golden` — two-feature fixture: `alpha` in flight with an ERROR finding carrying a line (`in_flight:true`, `line:<n>`); `beta` fully done with a WARN whole-file finding (`in_flight:false`, `line:null` — e.g. the over-cap-handoff / missing-STATE shape the existing WARN tests build; NOT a feature-level producer, those hard-code `InFlight:true`). Exact-bytes `assert.Equal`, paths via `jsonString(t, abs)`, each `detail` and `line` captured from `assemble.NewServer(...).Check` on the same fixture, never a production literal; asserts exit 1 and empty stderr (red)
- [x] Step 3: `internal/cli/check_json_test.go` `Test_check_json_with_no_findings` — table over conforming repository and no-features repository: `counts` zeros, `"features":[]`, `ok:true`, exit 0, empty stderr; control arm: same fixture without `--json` writes `brief check: no findings` to stderr (red)
- [x] Step 4: `internal/cli/check_json_test.go` `Test_check_json_warn_only_is_ok` — WARN-only fixture: `ok:true`, `exit_code:0`, `counts.error == 0`, `counts.warn > 0`, exit 0, empty stderr (red)
- [x] Step 5: `internal/cli/check_json_test.go` `Test_check_json_scopes_to_the_named_feature` — two-feature fixture, `check alpha --json`: only `alpha` in `features[]`, counts only alpha's (red)
- [x] Step 6: `internal/cli/json.go` `(reporter).headerFor(exitCode)` — header builder taking the exit code; `successHeader()` becomes `headerFor(0)` so `newJSONHeader` stays the only place deriving `ok` (new)
- [x] Step 7: `internal/cli/check.go` `countFindings(groups) (errors, warns int)` — one severity count used by `checkSummary`, the JSON `counts`, and the exit decision, so `exit_code == 1 iff counts.error > 0` by construction (new / update `checkSummary` to use it)
- [x] Step 8: `internal/cli/check.go` `checkDocument` (embeds `jsonHeader` first, then `Counts`, `Features`), `checkCountsJSON`, `checkFeatureJSON`, `checkFindingJSON` (`Line *int`), and `checkFeatures(groups)` mapper returning a sized non-nil slice (new)
- [x] Step 9: `internal/cli/check.go` `runCheck` — JSON branch immediately after `srv.Check` returns cleanly, **above** the no-findings stderr write and above `displayFindings`; builds from the un-relativized `GroupByFeature(findings)`, header via `headerFor(ExitCode(...))` of the same error it returns; writes the document, returns `errCheckFindings` when `countFindings` reports an ERROR, else nil. Text path's ERROR return uses `countFindings` too (green)
- [x] Step 10: mutation-verify, one at a time, stashed (`git stash push -u -m "s09-mut-<n>"`, apply by SHA, prove byte-identical after restore):
  - JSON branch fed `displayFindings(wd, groups)` → reddens `Test_check_json_document_golden` (R6)
  - `successHeader()` instead of `headerFor(code)` → reddens `Test_json_mode_check_findings_are_not_an_error_document` and the golden (R2)
  - JSON branch moved below the no-findings stderr write → reddens `Test_check_json_with_no_findings` (R1)
  - `Line` always `&f.Line` → reddens the golden (`beta`'s `line:null`)
  - `checkFeatures` returning a nil slice for zero groups → reddens `Test_check_json_with_no_findings`
  - `detail` passed through `flattenTabwriterField` → reddens the golden only if a captured detail contains a tab/newline; if none does, say so and do not claim this mutation
- [x] Step 11: `go build ./...`, `go test ./...` (unpiped, report count + delta), `go test -race ./internal/cli/...`, `golangci-lint run ./...` → mark SCENARIO-09 done in specification.md, rewrite STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `check --json` with an ERROR is `ok:false, exit_code:1` with a success-shaped payload and
  **no `error` key** — R2 (`ok == (exit_code==0)`) and R4 (findings are data) together.
- `reporter.headerFor(exitCode)` is the header builder for any document whose exit code is
  non-zero without being an error document; `successHeader()` == `headerFor(0)`. `ok` is
  derived only in `newJSONHeader`.
- `severity` wire values are `"ERROR"`/`"WARN"` — the `assemble.Severity` const values are
  now wire strings (like `Rule`, never rename). `counts` keys are lower-case `error`/`warn`
  per the product-vision shape; the casing asymmetry is deliberate, do not "fix" either side.
- `line` is `null` for `Finding.Line == 0`, integer otherwise (mirrors `statusProblemJSON`).
- `features[]` order = `GroupByFeature` order (Check emission order); findings keep
  within-group order. Sorting either breaks the golden.
- The JSON branch sits before any stderr write and before `displayFindings` in `runCheck`;
  JSON never sees relativized paths.
- `countFindings` is the single severity count for summary, `counts` and exit decision.

**Left unbuilt** — named so nobody assumes it exists:
- `--json` flag row / "For scripts, use --json" sentence in `check`'s help and `checkLong` —
  S14.
- `assemble.RenderJSON` — still unowned; not used here.
- `new`/`finish`/`--version`/`help` documents — S10–S13.

**Traps** — things that look right and are not:
- `successHeader()` is hard-coded exit 0; using it on an ERROR run ships `ok:true` beside a
  real exit 1 and no pre-existing test catches it.
- The old S02 test compared `--json` stderr to text stderr; text stderr carries the summary,
  so a comparison would pass with a leaked summary. Assert `Empty`.
- The feature-level producers (`feature-symlink`, `feature-unreadable`) hard-code
  `InFlight:true`; an `in_flight:false` fixture needs a file-level WARN on a done feature.
- `errCheckFindings` is returned bare after the document write; nothing in `Run` renders it —
  routing it through `out.refusal` would add an error document and break R4.
