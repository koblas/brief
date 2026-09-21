# SCENARIO-02: Undefined long flag is a one-line usage error

## Scenario

```gherkin
Scenario: SCENARIO-02 Undefined long flag is a one-line usage error
  When I run "brief start --bogus demo"
  Then stderr is exactly "brief start: unknown flag: --bogus; run 'brief start <feature>'"
  And stdout is empty and the exit code is 2
```

## User-visible contract

- Command: `brief <leaf path> … --bogus …` for every leaf (`start`, `finish`, `status`,
  `check`, `new feature`, `new step`).
- stderr: exactly one line, `brief <leaf path>: unknown flag: --bogus; run '<invocation>'`.
- stdout: empty. Exit code: 2 (`cli.ExitCode`), error `errors.Is(err, cli.ErrUsage)`.
- No `run*` function executes (flag parsing fails before `RunE`).

## What already exists (green on arrival — expected)

SCENARIO-01 built the root `SetFlagErrorFunc` in `internal/cli/cli.go` (~line 150) and
already pinned exact long-flag wording for two leaves:
- `start_test.go` `Test_returns_a_usage_error_for_an_unknown_start_flag` — the exact scenario
  line for `start --bogus demo`, stdout empty, `ErrUsage`; it does **not** assert
  `ExitCode == 2`.
- `finish_test.go:454` — exact line for `finish demo SCENARIO-01 --bogus`.

Not pinned exactly: `status --bogus` (only `Contains "brief status"` + one line),
`check --bogus` (no stderr assertion at all), and no long-flag test at all for
`new feature` / `new step` (only the shorthand `-x`, which S03 owns).

So no production change is expected. The new test is **green on arrival** because S01's
port already produces this frame; the red for this scenario is the mutation step below,
not a manufactured failure. If any row comes out red, stop and report — that is a real
defect in S01's frame, not something to paper over.

## Implementation Plan

- [x] Step 1: `internal/cli/flag_error_test.go` `Test_reports_an_undefined_long_flag_as_one_usage_line_naming_the_command_invocation` — table test through `cli.Run` (black-box `cli_test` package, `t.TempDir()` wd), one row per leaf: `start --bogus demo` (the scenario's exact example, first row), `finish demo SCENARIO-01 --bogus`, `status --bogus`, `check --bogus`, `new feature --bogus payments`, `new step --bogus demo`. Each row asserts `ErrorIs ErrUsage`, `ExitCode == 2`, empty stdout, and the exact full stderr line via the existing `oneLine` helper, with the expected string written out literally per row (expected: green on arrival — say so)
  Expected literals (write these out; do not build them from `finishInvocation` or any other production constant):
  - `brief start: unknown flag: --bogus; run 'brief start <feature>'`
  - `brief finish: unknown flag: --bogus; run 'brief finish <feature> <step> --handoff <path> --state <path>'`
  - `brief status: unknown flag: --bogus; run 'brief status'`
  - `brief check: unknown flag: --bogus; run 'brief check [feature]'`
  - `brief new feature: unknown flag: --bogus; run 'brief new feature <name>'`
  - `brief new step: unknown flag: --bogus; run 'brief new step <feature>'`
- [x] Step 2: mutation-verify the `SetFlagErrorFunc` framing — each mutation stashed per `.claude/rules/agent-briefs.md`, run with `go test ./internal/cli/ -run Test_reports_an_undefined_long_flag -v`, restored, `diff`-proved byte-identical; report which rows reddened. The discriminating claim is **which rows of the new table** redden. Existing S01 exact-stderr assertions (`start_test.go:462/488/499`, `finish_test.go:454`, `run_test.go:163`, `new_step_test.go:60`) will also redden under M1-M3 if the whole package is run — that is expected and is **not** a failed control (verify)
  - M1: replace `strings.TrimPrefix(cmd.CommandPath(), "brief ")` with `cmd.Name()` → must redden the `new feature` and `new step` rows and **no** top-level row (top-level rows coincide under both expressions; only nested rows discriminate)
  - M2: read the invocation from `cmd.Root().Annotations` → must redden every row (proves the invocation half is pinned per command)
  - M3: return `err` unwrapped from the func instead of `usageError(...)` → must redden every row (no `ErrUsage`, exit 1, empty stderr under `SilenceErrors`)
  - M4: change the `status` leaf's `invocation` argument in `leafCommand(...)` → must redden exactly the `status` row. `status` is chosen deliberately: before this scenario its flag-error test only asserted `Contains "brief status"`, so this is the mutation the new table newly catches; `start`/`finish` are already pinned by S01 tests
- [x] Step 3: `go build ./...`, `go test ./...` (unpiped, report count and delta — expect +6 subtests / +1 top-level test), `go test -race ./internal/cli/...`, `golangci-lint run ./...` (verify)
- [x] Step 4: all tests green → mark SCENARIO-02 done in `specification.md`, update `STATE.md` (update)

Do **not** delete or rewrite the S01 tests in `start_test.go` / `finish_test.go` /
`status_test.go` / `check_test.go` that overlap this table; they are S01's port evidence.
Duplication is acceptable; a reviewer may later fold them.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- The long-flag error line for every leaf is `brief <CommandPath minus "brief ">: <pflag error verbatim>; run '<Annotations["invocation"]>'` — S03 (shorthand), S04 (`-json`), S05 (missing value), S06 (`--help` + bogus) all assert through the same root `SetFlagErrorFunc`; none may add a per-command FlagErrorFunc.
- `flag_error_test.go` is the home for flag-parse error tables — S03/S04/S05 add their tables (or rows) there rather than scattering them per command file.
- Each row's expected stderr is a literal string, never built from the production `invocation` constants (e.g. `finishInvocation`) — building it from the constant would pin nothing (see M4).

**Left unbuilt** — named so nobody assumes it exists:
- Shorthand (`unknown shorthand flag: 'x' in -x`) table rows — S03.
- Single-dash long flag (`-json`) wording — S04.
- `flag needs an argument: --handoff` — S05.
- `--help` adjacent to `--bogus` ordering — S06 (already asserted in `start_test.go:479-499`, still owned there).

**Traps** — things that look right and are not:
- `new` is `DisableFlagParsing`; `new --bogus` is handled by `runNew`, not the FlagErrorFunc, and has a different message. The rows must name the leaf (`new feature --bogus …`), not `new --bogus`.
- `start_test.go` asserts raw `stderr.String()` with a trailing `\n`; the new table uses `oneLine` (trims it, and asserts exactly one newline). Both are intentional — don't normalize either.
- Asserting `ErrorIs ErrUsage` alone does not prove exit 2 (`ExitCode` is the contract); keep the explicit `ExitCode == 2` assertion in every row.
- Top-level-only rows cannot tell `cmd.Name()` from the trimmed `CommandPath()` (they coincide for `start`, `status`, …); only the `new feature` / `new step` rows catch M1. Dropping them makes the path half of the frame unpinned.
