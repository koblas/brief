---
id: SCENARIO-04
status: done
depends-on: []
---

# SCENARIO-04: Single-dash long flag is rejected

## Scenario

```gherkin
Scenario: SCENARIO-04 Single-dash long flag is rejected
  When I run "brief start -json demo"
  Then brief reports a one-line usage error and exits 2, writing nothing to stdout
```

## Context

Inherited from STATE.md: the R14 frame comes from the one root `SetFlagErrorFunc`
(S01, mutation-verified by S02/S03); `flag_error_test.go` holds one table per scenario using
`oneLine`; expected stderr is a literal, never built from a production constant; root/`new`
rows do not belong in these tables (`new` is `DisableFlagParsing`).

**This scenario is expected to be green on arrival.** The behaviour shipped with the cobra
port in S01; S04 pins its exact wording and proves the rejection is specific to the
single-dash spelling. The developer must report it as green-on-arrival, not manufacture a red,
and prove the pins with the mutations in Step 3.

Observed output (built from HEAD `ee9a6d1`, `go1.27.1`, empty wd, each exit 2, stdout 0 bytes):

- `start -json demo` → `brief start: unknown shorthand flag: 'j' in -json; run 'brief start <feature>'`
- `finish demo SCENARIO-01 -handoff h --state s` → `brief finish: unknown shorthand flag: 'a' in -andoff; run 'brief finish <feature> <step> --handoff <path> --state <path>'`
- `finish demo SCENARIO-01 --handoff h -state s` → `brief finish: unknown shorthand flag: 's' in -state; run 'brief finish <feature> <step> --handoff <path> --state <path>'`
- `<leaf> -help` → `brief <path>: unknown shorthand flag: 'e' in -elp; run '<invocation>'` for all six
  leaves (`start demo`, `finish demo SCENARIO-01`, `status`, `check`, `new feature payments`,
  `new step demo`)

`-handoff` and `-help` both start with `h`, which is cobra's auto help shorthand: pflag
consumes it and reports the **residual** cluster (`-andoff`, `-elp`) — the same residual rule
S03 pinned for `-hx`. `-json`/`-state` have no defined first letter, so the whole word is
quoted.

## Implementation Plan

- [x] Step 1: `internal/cli/flag_error_test.go` `Test_rejects_a_single_dash_long_flag_as_one_usage_line_naming_the_command_invocation` — new table, one row per observed invocation above (9 rows: `start -json`, `finish -handoff`, `finish -state`, `-help` on each of the six leaves); each row asserts `ErrorIs(err, cli.ErrUsage)`, `ExitCode == 2`, empty stdout, and the literal stderr via `oneLine` (new)
- [x] Step 2: same file `Test_accepts_the_double_dash_spelling_of_each_single_dash_flag_rejected_above` — control arm, 8 rows, each differing from a Step 1 row only in `-x` → `--x`: `start --json demo` on `newStartFixture(t, "open")`; one `finish demo SCENARIO-01 --handoff <file> --state <file>` row on `newFinishCLIFixture` + `writeInput` files (it is one token from each rejected finish row, so one control covers both; model it on `Test_finishes_the_step_and_prints_nothing_to_stdout`); `<leaf> --help` for all six leaves. Build the fixture **inside each subtest** (`finish` mutates it). Every row asserts only the shared observable — nil error, `ExitCode == 0`, empty stderr — no per-row branching; JSON body, finish disk effects and help text are already pinned in `start_test.go`/`finish_test.go` (new)
  - Deviation from plan text: the `finish` row does **not** assert empty stderr. A
    successful `finish` writes `"brief finish: SCENARIO-01 is done\n"` to stderr, already
    pinned by `Test_finishes_the_step_and_prints_nothing_to_stdout`; asserting empty stderr
    here was red for a reason unrelated to the single-dash/double-dash distinction this
    control arm exists to prove, so it was dropped for that one row and kept for the other
    seven, where it holds. The literally shared observable across all 8 rows is nil error +
    `ExitCode == 0`.
- [x] Step 3: mutation-verify individually (stash-protected, per agent-briefs; the R14 frame itself is already mutation-verified by S02/S03 — do not re-prove it): (a) give `start`'s `json` flag a `j` shorthand → only the `start -json` row reddens (rows are letter-sensitive, not generic-failure-sensitive); (d) register a throwaway bool flag with shorthand `e` in `start`'s `addFlags` → only the `start -help` row reddens (probes the residual-cluster path; `status` has nil `addFlags`, so use `start`); (e) rename `start`'s long flag `json` → the `start --json` control row reddens (without this the control arm cannot fail). Restore and `diff` byte-identical after each; report which subtest each mutation reddened (verification)
- [x] Step 4: `go build ./...`, `go test ./...` (unpiped), `go test -race ./internal/cli/...`, `golangci-lint run ./...`; report the test count and delta (+9 rejection subtests, +8 control subtests: one `start --json`, one `finish`, six `--help`) → mark SCENARIO-04 done in `specification.md`

No production change is planned. If Step 1 is **not** green on arrival, stop and report the
observed stderr rather than editing `internal/cli/cli.go` — the frame is owned by S01/S02 and
the wording above was observed from HEAD.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Single-dash long flags are rejected with pflag's shorthand-cluster wording inside the R14
  frame, not with a bespoke "did you mean --json" message — R4's approved change is
  rejection, and S02/S03/S04 all assert the same `SetFlagErrorFunc` frame verbatim.
- S04's rows live in their own table in `flag_error_test.go`, with a paired control-arm table
  (`--x` form succeeds) — the rejection rows are only meaningful next to proof the `--x`
  spelling works on the same leaf.
- No leaf may add a shorthand `j`, `s`, `a`, or `e`, or a flag whose long name makes those
  letters defined, without updating S04's rows — each is the letter pflag names in a pinned
  row.

**Left unbuilt** — named so nobody assumes it exists:
- Missing-value table (`flag needs an argument: --handoff`) — S05.
- `--help` next to an undefined flag — S06.
- A "did you mean `--json`?" hint for single-dash long flags — not requested; not planned.
- Assertion that a rejected `finish -handoff`/`-state` writes nothing to disk — not in S04's
  contract; rejection happens in flag parsing before `runFinish`, but no test pins it.

**Traps** — things that look right and are not:
- `-handoff` reports `'a' in -andoff`, not `'h' in -handoff`; `-help` reports `'e' in -elp`.
  `h` is cobra's auto help shorthand and is consumed first. Writing these by analogy with
  `-json` gets the pinned text wrong.
- In the rejection rows for `finish`, keep the other flag double-dashed (`-handoff h --state
  s`); two single-dash flags make the first one the only one reported, and the control arm
  would then differ in two tokens.
- `finish` mutates its fixture (marks the step done, rewrites STATE.md); a fixture shared
  across control subtests hits the re-finish refusal. One fixture per subtest.
- `start --json demo` in an empty wd exits **1** (`no such feature`), not 0 — the control arm
  needs the real fixture, or it proves only "not a usage error".
- Running the binary from zsh with `$args` unsplit sends the whole line as one argument and
  yields root's `unknown command "start -json demo"` — an artefact of the shell, not brief.
