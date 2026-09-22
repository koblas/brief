---
id: SCENARIO-06
status: done
depends-on: []
---

# SCENARIO-06: --help next to an undefined flag is a usage error in either order

## Scenario

```gherkin
Scenario: SCENARIO-06 --help next to an undefined flag is a usage error in either order
  When I run "brief start --help --bogus" or "brief start --bogus --help"
  Then each is a one-line usage error with exit 2 and nothing on stdout
```

## User-visible contract

- `brief <leaf> --help --bogus` and `brief <leaf> --bogus --help`, for every leaf (`start`,
  `finish`, `status`, `check`, `new feature`, `new step`): stderr is exactly S02's line for
  that leaf, e.g. `brief start: unknown flag: --bogus; run 'brief start <feature>'`; stdout
  empty; `errors.Is(err, cli.ErrUsage)`; `cli.ExitCode(err) == 2`.
- Same for the shorthand: `brief start -h --bogus` and `brief start --bogus -h`.
- Control: `brief <leaf> --help` alone and `brief start -h` alone print help to stdout, empty
  stderr, nil error, exit 0.

## Expected outcome: green on arrival

No production change is expected. cobra's `ParseFlags` runs before any help check, and pflag
stops at the first bad token, so the R14 `SetFlagErrorFunc` frame already fires in both
orders. The existing `start_test.go` pair (`Test_returns_a_usage_error_when_help_precedes_an_undefined_flag`,
`..._when_an_undefined_flag_precedes_help`) passes today (verified while planning). Do not
manufacture a red: the proof is the mutation in Step 4. If any row's actual stderr differs
from its literal, **stop and report** — do not edit the literal to match.

Row scope is a decision, not an omission: `--help` gets all six leaves x both orders
(12 rows), matching S02/S03's per-leaf precedent. `-h` gets `start` only, both orders
(2 rows): cobra's `InitDefaultHelpFlag` registers `-h` identically on every leaf, so per-leaf
`-h` rows prove nothing more about the frame. Root and `new` are excluded (they are
`DisableFlagParsing`; STATE trap). The `-hx` cluster is S03's; only separated `-h` forms here.

## Implementation Plan

- [x] Step 1: `internal/cli/flag_error_test.go` `Test_reports_an_undefined_flag_as_a_usage_error_whichever_side_of_help_it_is_on` — 14-row table (6 leaves x `--help` both orders + `start` x `-h` both orders), same assertion shape as S02's table: `require.ErrorIs(err, cli.ErrUsage)`, exit 2, empty stdout, `oneLine(t, &stderr)` equals a literal written per row; reuse S02's arg shapes (e.g. `finish demo SCENARIO-01 ...`, `new feature ... payments`) (green on arrival)
- [x] Step 2: `internal/cli/flag_error_test.go` `Test_prints_help_for_the_h_shorthand_alone` — control arm for the `-h` rows: `start -h` returns nil, exit 0, empty stderr, stdout contains the literal `brief start reads; it never writes.` (the same pin `start_test.go` uses — without it, byte-equality passes when both outputs are empty), and stdout byte-equal to the stdout of `start --help` from the same test (new; `--help`-alone control already exists in `Test_accepts_the_double_dash_spelling_of_each_single_dash_flag_rejected_above`'s `helpRows` and `start_test.go` `Test_prints_the_start_usage_for_help` — reuse, do not duplicate). Without this, the `--bogus -h` row would pass even if `-h` were undefined, since pflag errors on `--bogus` first
- [x] Step 3: `go test ./internal/cli/ -run 'Test_reports_an_undefined_flag_as_a_usage_error_whichever_side_of_help_it_is_on|Test_prints_help_for_the_h_shorthand_alone' -v -count=1` — confirm every row passes; report the row count (verify)
- [x] Step 4: mutation-verify "help wins" (the regression this scenario guards) — `cp internal/cli/cli.go "$TMPDIR/cli.go.orig"`; in `Run`, just before `return root.ExecuteContext(ctx)`, add a loop over `argsCopy` that, on an element equal to `"--help"` or `"-h"`, calls `root.SetFlagErrorFunc(func(*cobra.Command, error) error { return nil })`; run `go test ./internal/cli/ -count=1 -v`; expect RED on all 14 Step 1 rows (both orders, both spellings) and on the two `start_test.go` pair tests, and on nothing else (planner probe with the pre-existing suite reddened exactly the pair) — count `--- FAIL` at subtest granularity: 16 expected (14 rows + the 2 pair tests). Stays-green control: `helpRows` in `Test_accepts_the_double_dash_spelling_of_each_single_dash_flag_rejected_above` and Step 2's `-h`-alone test contain `--help`/`-h`, so the pre-scan fires on them, but no parse error occurs — they must stay GREEN; if any reddens, the mutation is broader than claimed and the RED is not evidence (stop and report); `cp "$TMPDIR/cli.go.orig" internal/cli/cli.go`; `diff "$TMPDIR/cli.go.orig" internal/cli/cli.go` must be empty. No `git stash` in any form (verify)
- [x] Step 5: `go build ./...`, `go test ./...` (unpiped; report exact count and delta), `go test -race ./internal/cli/...`, `golangci-lint run ./...` (verify)
- [x] Step 6: `docs/specifications/cli-cobra/STATE.md` — remove the Left-unbuilt line "`--help` next to an undefined flag, either order — S06"; add the S06 table to the `flag_error_test.go` table list; extend the existing Open-debts duplication entry with the `start_test.go` pair (see Handoff); bump "Scenarios complete" (update)
- [x] Step 7: all tests green -> tick SCENARIO-06 in `docs/specifications/cli-cobra/specification.md` `## BDD Acceptance Progress` (update)

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- An undefined flag beats `--help`/`-h` in either order: usage error, exit 2, empty stdout —
  approved behavior change (spec Business Rules). S07's generated help template and S08's
  `help` command must not add any pre-dispatch "help wins" scan of args; Step 4's mutation is
  exactly that scan and reddens S06's table.
- S06 rows reuse S02's stderr literals verbatim — the frame is the one root
  `SetFlagErrorFunc`; `--help` in the args changes nothing about the line.
- `-h` rows are scoped to `start` deliberately (cobra's `InitDefaultHelpFlag` is identical per
  leaf). A leaf that defines its own `-h`/`help` flag must add its rows.

**Left unbuilt** — named so nobody assumes it exists:
- Per-leaf `-h --bogus` / `--bogus -h` rows for leaves other than `start` — by decision, unowned.
- `-h`-alone control for leaves other than `start` — unowned.

**Traps** — things that look right and are not:
- The obvious mutation `if cmd.Flags().Changed("help") { return nil }` inside
  `SetFlagErrorFunc` reddens only the help-FIRST order: pflag stops at the first bad token, so
  in `--bogus --help` the help flag is never parsed and `Changed("help")` is false. It also
  spills into S03 (`-hx`) and S04 (`-help`, `-handoff`) rows because their leading `h` sets
  the help flag. Planner-verified. Use Step 4's args pre-scan mutation instead.
- The `--bogus --help` order is protected structurally by pflag's stop-at-first-error, not by
  any brief code — there is no guard to point at; only the pre-scan mutation exercises it.
- `start_test.go`'s pair comment calls its second test a "control arm"; it is the other
  order, not a control. The real `--help`-alone control is `Test_prints_the_start_usage_for_help`.

**Open debt** (fold into STATE.md's existing duplication entry, do not open a new one):
- `start_test.go` `Test_returns_a_usage_error_when_help_precedes_an_undefined_flag` and
  `..._when_an_undefined_flag_precedes_help` duplicate S06's two `start --help` rows; kept
  because they compare raw `stderr.String()` including the trailing `\n`, which `oneLine`
  trims. Unowned.
