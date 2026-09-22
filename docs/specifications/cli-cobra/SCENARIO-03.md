---
id: SCENARIO-03
status: done
---

# SCENARIO-03: Undefined short flag is a one-line usage error

## Scenario

Scenario: SCENARIO-03 Undefined short flag is a one-line usage error
  When I run "brief new feature -x payments"
  Then stderr is exactly "brief new feature: unknown shorthand flag: 'x' in -x; run 'brief new feature <name>'"
  And the exit code is 2

## User-visible contract

- Command line: `brief <leaf> … -<undefined shorthand> …` for every leaf (`start`, `finish`,
  `status`, `check`, `new feature`, `new step`).
- stdout: empty. stderr: exactly one line, `brief <path>: <pflag message>; run '<invocation>'`
  (S01/S02's `SetFlagErrorFunc` frame, unchanged). Exit code: 2, error wraps `cli.ErrUsage`.
- pflag message shapes, observed on the built binary before planning:
  - `-x` → `unknown shorthand flag: 'x' in -x`
  - `-xy` (all-undefined cluster) → `unknown shorthand flag: 'x' in -xy` (whole cluster quoted)
  - `-hx` (defined `-h` first) → `unknown shorthand flag: 'x' in -x` (pflag consumes `h` and
    quotes only the residual cluster; help is NOT printed, stdout is empty)
  - `-xh` → `unknown shorthand flag: 'x' in -xh` — same shape as `-xy`, adds nothing; no row.
- Out of scope, already pinned elsewhere: bare `brief -x` (unknown command, `run_test.go`) and
  `brief new -x` (unknown type, `run_test.go`) — neither reaches the `FlagErrorFunc`.

## Expected outcome

Green on arrival. S01 ported dispatch to cobra and the root `SetFlagErrorFunc` already
produces this frame for every leaf; `run_test.go` and `new_step_test.go` already assert the
`-x` line for `new feature` and `new step`. No production change is planned. The work is the
dedicated table STATE.md assigns to S03, plus mutation evidence that its rows can go red.

## Implementation Plan

- [x] Step 1: `internal/cli/flag_error_test.go` `Test_reports_an_undefined_short_flag_as_one_usage_line_naming_the_command_invocation` — new table in the S02 shape (`ErrorIs ErrUsage`, `ExitCode == 2`, empty stdout, literal `oneLine` stderr); rows in this order: `new feature -x payments` (the scenario's example, first), `start -x demo`, `finish demo SCENARIO-01 -x`, `status -x`, `check -x`, `new step -x demo` (green on arrival — report as such)
- [x] Step 2: same table — grouped-shorthand rows: `new feature -xy payments` (whole cluster quoted, `in -xy`) and `new feature -hx payments` (residual cluster quoted, `in -x`; empty stdout proves the consumed `-h` did not print help) (green on arrival)
- [x] Step 3: mutation — register a shorthand `x` on the `new feature` leaf only (still-valid code, stashed per the standing brief); expect exactly the `new feature -x`, `-xy` and `-hx` rows red and the other leaves' rows green; restore and prove byte-identical (verification)
- [x] Step 4: control arm for the `-hx` row — build the binary (`go build -o "$TMPDIR/brief" ./cmd/brief`) and run `brief new feature -h payments` once; confirm help reaches stdout and stderr is empty. This differs from the `-hx` row only by the undefined `x`, so it proves the row's empty-stdout assertion is not satisfied by "help never prints on this leaf". Record the observed result in the report (verification)
- [x] Step 5: `go build ./...`, `go test ./...` unpiped — report the measured count and delta (expected: one new top-level test with 8 subtests, nothing removed); `go test -race ./internal/cli/...`; `golangci-lint run ./...` → mark SCENARIO-03 done in specification.md; fold the Handoff below into STATE.md and strike the now-stale shorthand item ("`unknown shorthand flag: 'x' in -x` … owned by S03") from STATE.md's **Left unbuilt**

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Shorthand flag-error assertions live in `Test_reports_an_undefined_short_flag_as_one_usage_line_naming_the_command_invocation` in `flag_error_test.go`, one row per leaf plus the `-xy`/`-hx` cluster rows — S04's single-dash long flags (`-json`) are also shorthand clusters to pflag and must go in S04's own table, not be appended here, so each scenario's table stays traceable to its Gherkin.
- No production change: pflag's shorthand wording passes through the S01 frame verbatim. A later scenario that customizes flag-error wording (e.g. S07's template work) must keep these rows byte-identical or change them explicitly.

**Left unbuilt** — named so nobody assumes it exists:
- `--help` / `-h` next to an undefined flag in either order — owned by S06; the `-hx` row only pins the shorthand-cluster case, not `--help --bogus` / `--bogus --help`.
- Single-dash long flags (`-json`, `-help`) — owned by S04.
- Consolidating the duplicates — `run_test.go` `Test_returns_a_usage_error_when_a_flag_is_not_defined` (`new feature -x p`) and `new_step_test.go` `Test_returns_a_usage_error_when_a_flag_is_not_defined_for_step` (`new step -x p`) now overlap two rows of the S03 table. Deliberately not deleted here (out of scope, removes SCENARIO-01 coverage); unowned — leave to the reviewer pass or a later cleanup.

**Traps** — things that look right and are not:
- `-hx` quotes `in -x`, not `in -hx`: pflag strips the consumed defined shorthand before reporting. Writing the expected string by analogy with `-xy` gets it wrong.
- `-h` is the only defined shorthand in the tree (cobra's auto-added help flag); no leaf registers a `P`-variant flag. A future leaf that adds a shorthand collides with these rows' `x`/`y` only if it picks those letters — pick another undefined letter in the rows rather than dropping a leaf.
- Bare `brief -x` and `brief new -x` look like shorthand-flag errors but are not: root and `new` use `DisableFlagParsing`, so they surface as unknown command / unknown type from `runRoot`/`runNew`. Do not add them to the shorthand table.
