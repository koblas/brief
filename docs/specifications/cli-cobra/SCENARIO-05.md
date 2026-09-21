# SCENARIO-05: Flag missing its value

## Scenario

```gherkin
Scenario: SCENARIO-05 Flag missing its value
  When I run "brief finish demo SCENARIO-01 --handoff"
  Then stderr is exactly "brief finish: flag needs an argument: --handoff; run 'brief finish <feature> <step> --handoff <path> --state <path>'"
  And the exit code is 2
```

## Context

Test-only scenario. **Expected green on arrival**: the root `SetFlagErrorFunc` frame from
S01 already wraps pflag's `flag needs an argument` error. A throwaway probe through
`cli.Run` (deleted afterwards, tree clean) showed the exact outputs below. Do not
manufacture a red. Prove each row with the mutations in Step 5.

`finish` is the only leaf with value-taking flags (`--handoff`, `--state`, both
`String`). `--json` is a bool with pflag's implicit `NoOptDefVal`, so it can never be
"missing its value". No other leaf gets a row.

User-visible contract, pinned from the probe. Every case writes nothing to stdout and exits 2 with `cli.ErrUsage`.
The invocation `I` is `brief finish <feature> <step> --handoff <path> --state <path>`:

- `finish demo SCENARIO-01 --handoff` → `brief finish: flag needs an argument: --handoff; run 'I'`
- `finish demo SCENARIO-01 --handoff h.md --state` → `... flag needs an argument: --state; run 'I'`
- `finish demo SCENARIO-01 --state s.md --handoff` → `... flag needs an argument: --handoff; run 'I'`
- `finish --handoff` (no positionals) → `... flag needs an argument: --handoff; run 'I'`. Flag
  parsing runs before `runFinish`'s argument-count check.
- `finish demo SCENARIO-01 --handoff= --state s.md` → `brief finish: --handoff is required; run 'I'`
  (a distinct path: pflag accepts the empty value and `runFinish`'s `handoffPath == ""` guard fires)
- `finish demo SCENARIO-01 --handoff h.md --state=` → `brief finish: --state is required; run 'I'`
- `finish demo SCENARIO-01 --handoff --state s.md` → `brief finish: too many arguments; run 'I'`.
  **Trap**: pflag takes `--state` as the *value* of `--handoff`, and `s.md` becomes a third
  positional. No "needs an argument" error appears.
- `finish demo SCENARIO-01 --handoff --state` → `brief finish: --state is required; run 'I'`
  (probed). `--handoff` swallows `--state`, so the guard fires on `--state`, not on `--handoff`.
  Pin this as a second case in Step 3.

## Implementation Plan

- [x] Step 1: `internal/cli/flag_error_test.go` `Test_reports_a_flag_missing_its_value_as_one_usage_line_naming_the_command_invocation` — S05's table with the 4 "flag needs an argument" rows above. Same row shape and body as S02–S04's tables (`require.ErrorIs` `cli.ErrUsage`, `ExitCode` 2, empty stdout, `oneLine` stderr equal to a **literal**). `t.TempDir()` wd, no fixture (green on arrival)
- [x] Step 2: `internal/cli/flag_error_test.go` `Test_reports_an_empty_flag_value_as_that_flag_being_required` — 2 rows (`--handoff=`, `--state=`) with the other flag given a value. Same assertions, `t.TempDir()` wd. Pins that `=` with an empty value gets past flag parsing and reaches `runFinish`'s required-flag guard (green on arrival)
- [x] Step 3: `internal/cli/flag_error_test.go` `Test_takes_the_next_flag_as_the_value_of_a_flag_missing_its_value` — 2 rows, same assertions:
  - `finish demo SCENARIO-01 --handoff --state s.md` → the `too many arguments` literal
  - `finish demo SCENARIO-01 --handoff --state` → the `--state is required` literal The doc comment states the rule: pflag takes the next token as the value even when it starts with `--`. It does not describe this as a design choice (green on arrival)
- [x] Step 4: doc comment on each new test. It states the rule under test and that expected stderr is literal. Do not cite scenario ids as narrative beyond the existing `is SCENARIO-0N's table` form the file already uses
- [x] Step 5: mutation-verify each test **individually**. Keep a copy of the file in `$TMPDIR` (**no bare `git stash`/`pop`**: use `cp` to `$TMPDIR` and restore it, or `git stash push -m "<unique-tag>"` + `apply <sha>` + drop by tag), then `diff` to prove it is byte-identical:
  - `internal/cli/cli.go`: give the `handoff` flag a **non-empty** `NoOptDefVal`. pflag ignores `""`. After this change `--handoff` with nothing after it no longer needs a value. Expected red:
    - the 3 `--handoff` rows of Step 1 fail on wording, because they now reach `runFinish` guards instead of the flag-error frame
    - Step 3 also goes red. This is expected, because it tests the same property: `--handoff` no longer swallows `--state`
    - the `--state` row of Step 1 and both Step 2 rows stay green. `--handoff=` sets the value explicitly, so `NoOptDefVal` never applies to it

    Repeat with `state`: only the `--state` row of Step 1 goes red. Any other red pattern means the mutation cascaded. In that case stop and report
  - `internal/cli/finish.go`: in `runFinish`, reword or neutralise the `handoffPath == ""` case (a still-valid mutation) → Step 2's `--handoff=` row goes red. Do the same for the `statePath == ""` row
  - Step 3 is covered by the `handoff` `NoOptDefVal` mutation above, which reddens both of its rows. It needs no mutation of its own
- [x] Step 6: `go build ./...`, `go test ./...` (unpiped, report count + delta: expected +3 top-level tests and +8 subtests (4 + 2 + 2 table rows, each run with `t.Run`), `go test -race ./internal/cli/...`, `golangci-lint run ./...`
- [x] Step 7: all tests green → mark SCENARIO-05 done in `specification.md`; rewrite `STATE.md` (Scenarios complete 01..05; move "Missing-value flag-error table" out of Left unbuilt; add the trap below)

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Missing-value errors use the same root `SetFlagErrorFunc` frame as S02–S04, and pflag's
  wording `flag needs an argument: --<name>` passes through verbatim. S05's rows live in
  `flag_error_test.go` as three tests (missing value / empty `=` value / next-flag-consumed),
  and each expected stderr is a literal.
- `--handoff=` / `--state=` are **not** flag-parse errors. They reach `runFinish`'s
  `--handoff is required` / `--state is required` guard. If a scenario moves required-flag
  checking into cobra (`MarkFlagRequired`), it changes these two rows' wording and must update
  them. Cobra's required-flag error has different copy and does not treat `""` as missing.

**Left unbuilt** — named so nobody assumes it exists:
- Rows for leaves other than `finish`: none has a value-taking flag. A future `String`/`Int`
  flag on any leaf needs a row added to S05's table.
- `start demo --json=` → `invalid argument "" for "--json" flag: strconv.ParseBool: ...`
  through R14 (observed, exit 2). No scenario covers it and nothing pins it. The Go-internal
  `strconv.ParseBool` wording reaches the user. Flag it for the final product-vision pass.

**Traps** — things that look right and are not:
- `finish demo SCENARIO-01 --handoff --state s.md` does **not** report "flag needs an
  argument". pflag takes `--state` as the handoff path, so the result is `too many arguments`.
  With exactly two tokens after it (`--handoff --state`) and no `s.md`, the result is
  `--state is required`. Any test that expects a missing-value error from a flag followed by
  another flag is wrong.
- `finish --handoff` without positionals still gives the flag error, not the argument-count
  error, because flag parsing runs first. Do not "fix" this row to expect `too many`/`missing`
  wording.
- pflag's `parseLongArg` takes `NoOptDefVal` before "consume the next token", and only
  `--flag=value` bypasses it. Mutating one flag's `NoOptDefVal` therefore changes the parse
  of **every** space-separated occurrence of that flag in the suite, not just the row under
  test — including a row that merely uses that flag as unrelated setup. Verified: mutating
  `handoff`'s `NoOptDefVal` reddened Step 2's `--state=` row (its `--handoff h.md` setup is
  space-separated); mutating `state`'s reddened Step 2's `--handoff=` row the same way. The
  plan's Step 5 prediction that "both Step 2 rows stay green" under each `NoOptDefVal`
  mutation does not hold — one of the two always reddens, for a fully explained reason, not
  a cascade. Step 2's own guard is proven correctly isolated by the separate `finish.go`
  mutations (each reddens exactly its own Step 2 row) — see those results before assuming a
  `NoOptDefVal` mutation misbehaved.
- Neutralizing `runFinish`'s `statePath == ""` guard also reddens Step 3's second row
  (`--handoff --state`, no trailing value): that row's expected message
  (`--state is required`) is produced by that exact guard, since pflag leaves `statePath`
  unset when `--handoff` swallows `--state` as its own value. Expected, not a cascade.
