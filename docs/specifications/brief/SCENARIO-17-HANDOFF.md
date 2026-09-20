# SCENARIO-17 Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- Caps are measured in **lines**, by `markdown.CountLines` — `strings.Count(body, "\n")` plus
  1 for a non-empty body with no trailing newline; `""` is 0. It lives in `platform/markdown`,
  not `scaffold`, because `check` (22) needs the same count from the read side and `assemble`
  may not import `scaffold`.
- **One shared `scaffold.ErrOverCap` sentinel for both caps** — `RefusalError.Path` says which
  body (`HandoffSource` vs `StateSource`), so 18 and 22 branch on cap-ness with one `errors.Is`
  and never need a second sentinel. The check itself is one generic helper,
  `checkArgumentCap(body []byte, source, label string, limit int) *RefusalError`, parameterised
  on the label so 18 reuses it verbatim with `"state"`/`StateSource`/`cfg.StateCapLines`.
- **The cap check is in `Finish`'s argument band, before the `r.verdict()` switch** — same slot
  as `checkArgumentFence`, immediately ahead of it. So an over-cap handoff on an already-done
  step reports the cap, not `ErrAlreadyFinished` (pinned by
  `Test_an_over_cap_handoff_on_a_done_step_reports_the_cap_not_the_re_finish_refusal`). On an
  **open** step, trimming the body is the whole fix. On a **done** step it is two hops: the
  trimmed body then differs from the recorded one and 16's `ErrAlreadyFinished` fires, so the
  escape is 16's documented one — edit the recorded handoff file directly.
- **Comparison is `count > limit`** (written as `n <= limit` accepts) — exactly-at-cap is
  accepted. Mutation-verified both ways (see Verification).
- **`scaffold.HandoffSource = "<handoff>"`** exists and `cli/finish.go` swaps it for
  `sourceLocator(*handoffPath)`, beside the existing `StateSource`/`sourceLocator(*statePath)`
  branch — now a `switch refusal.Path` rather than a single `if`.
- **Refusal copy is pinned**, tail included:
  `brief finish: <source>: handoff is <n> lines, over the cap of <cap>; cut the handoff to
  <cap> lines or fewer, or raise handoff-cap-lines in .brief.yaml, and retry (no files
  changed)`. `Line` is 0.
- `Finish`'s doc comment now states the handoff cap is checked, in its real position in the
  enumerated order (right after frontmatter parses, before the state-fence check) — `go doc
  ./internal/scaffold` confirms.

**Left unbuilt** — named so nobody assumes it exists:

- `cfg.StateCapLines` — still unconsumed; owner is SCENARIO-18. `cfg.DefaultOutputBudgetBytes`
  — still unconsumed; R13, no owner.
- Validation of cap *values* (0 or negative is accepted as configured) — unowned, adjacent to
  19's heading/cap-value debt.
- No `--force`, no per-invocation cap override, no truncation — refusal is the only outcome.

**SCENARIO-18 is a small delta** — reuse `markdown.CountLines`, `ErrOverCap`,
`checkArgumentCap` (call it a second time with `"state"`/`StateSource`/`cfg.StateCapLines`,
between the handoff cap check and `checkArgumentFence`), the `StateSource` placeholder swap
already in `cli/finish.go`, and `finish_cap_test.go`'s fixture/probes (`overCapHandoff` /
`overCapBody` generalise directly — rename or duplicate for the state body, one line count
each).

**Traps** — things that look right and are not:

- `cli/finish.go`'s placeholder swap matched **only** `scaffold.StateSource` before this
  scenario; `Test_refuses_a_handoff_over_the_cap_and_names_the_handoff_path` and
  `Test_names_stdin_when_the_piped_handoff_is_over_the_cap` were red for exactly that reason
  (`<handoff>` rendered literally) until the `switch` in Step 13 landed — confirmed by running
  them before that edit.
- Existing fixture handoff bodies are small but not trivial — `newFinishFixture.newHandoff` is
  5 lines. `fixtureConfig().HandoffCapLines` is set to 10, strictly above that, so no unrelated
  scaffold test trips on the new cap for the wrong reason. Enumerated every `Finish`/`cli.Run`
  call site's handoff argument (Step 4); the largest anywhere in the suite is 5 lines
  (scaffold) / 1 line (cli, all `config.Default()`'s cap-60 fixtures).
- `snapshotTree` enumerates every non-directory entry, so a leftover temp file already shows up
  as a snapshot difference — no separate temp-file sweep was added for the cap refusal.
  `Test_a_refused_over_cap_handoff_leaves_every_file_byte_identical` pairs `snapshotTree` with
  `pinModTimes`/`modTimes`, citing the existing control arms in `finish_idempotent_test.go`
  rather than duplicating them.
- **This repository's own bodies are over both caps**: its handoff files run 29–373 lines
  against a 60-line cap and `STATE.md` is over 200 against an 80-line cap (18's cap), so once
  17 and 18 land `brief finish brief <step>` could not write them. It does not bite today (the
  tool is not self-hosted). Recorded as an open debt below, owned by `check` (22).

## Session notes

- Baseline measured before any change: inherited STATE.md and the standing brief point at 345
  passing (`go test -v ./... 2>&1 | grep -c -- "--- PASS"`), confirmed on this branch before
  any edit.
- Step 1 RED confirmed as a compile-fail: `go test ./internal/platform/markdown/...` reported
  `undefined: markdown.CountLines` across all seven new test functions.
- Step 2 GREEN reached in one pass — `CountLines` needed no iteration.
- Steps 6/9/10/11 written together in `finish_cap_test.go`; `go vet ./internal/scaffold/...`
  confirmed RED as a compile-fail (`undefined: scaffold.ErrOverCap`) before `ErrOverCap` and
  `HandoffSource` existed.
- Steps 7/8 landed together (sentinel + placeholder const, then the check itself); all five new
  scaffold-level tests passed on the first run after that edit, and the full
  `internal/scaffold` suite stayed green (no regression in the existing 5-line-handoff
  fixtures, confirming Step 5's cap choice of 10 was high enough).
- Steps 12/14 written before Step 13: `go test ./internal/cli/...` on just the two new tests
  showed the exact predicted trap — stderr read `brief finish: <handoff>: handoff is 61
  lines, ...` instead of naming the real path / `<stdin>`. Step 13's `switch` fixed both in one
  edit, no further iteration.
- Step 16: rewrote the `Finish` doc comment's second and third paragraphs; `go doc
  ./internal/scaffold Server.Finish` read back afterward to confirm the enumerated check order
  now names the handoff cap in its real position and no longer denies a cap is checked.
- `golangci-lint run ./...` found one issue on the first pass: `testifylint`'s `require-error`
  on `assert.ErrorIs` in `Test_an_over_cap_handoff_on_a_done_step_reports_the_cap_not_the_re_finish_refusal`
  (the `errors.Is` conjunct governs whether the following `NotErrorIs` assertion is meaningful,
  so it should abort the test on failure). Changed to `require.ErrorIs`; second run was clean.

## Verification

- `go build ./...` — exit 0.
- `go test ./...` — exit 0, all packages ok, 0 skips.
- `go test -race ./internal/scaffold/... ./internal/cli/... ./internal/platform/markdown/...`
  — exit 0.
- `golangci-lint run ./...` — 0 issues (one finding surfaced and fixed first; see above).
- Test count: 359 passing (`grep -c -- "--- PASS"`, unanchored, subtests included), delta
  **+14** from the 345 baseline — 0 `--- FAIL`, 0 `--- SKIP`. The 14 new tests: seven
  `markdown.CountLines` cases (`lines_test.go`), five scaffold-level cap tests
  (`finish_cap_test.go`: `Test_refuses_a_handoff_one_line_over_the_configured_cap`,
  `Test_accepts_a_handoff_of_exactly_the_configured_cap`,
  `Test_accepts_a_handoff_of_exactly_the_configured_cap_with_no_trailing_newline`,
  `Test_a_refused_over_cap_handoff_leaves_every_file_byte_identical`,
  `Test_an_over_cap_handoff_on_a_done_step_reports_the_cap_not_the_re_finish_refusal`), and two
  CLI-level tests (`Test_refuses_a_handoff_over_the_cap_and_names_the_handoff_path`,
  `Test_names_stdin_when_the_piped_handoff_is_over_the_cap`).
- Mutation (a), delete the `checkArgumentCap` call from `Finish` (stashed via `cp` to
  `$TMPDIR`, restored and `diff`-confirmed byte-identical afterward — this worktree's git
  stash stack is shared across sessions, so `cp`/`diff` was used instead of `git stash`):
  reddened `Test_refuses_a_handoff_one_line_over_the_configured_cap` (`ErrOverCap` chain
  missing); `Test_accepts_a_handoff_of_exactly_the_configured_cap` stayed green, exactly as
  predicted.
- Mutation (b), flip `n <= limit` to `n < limit` (equivalent to flipping the refusal trigger to
  `>=`): reddened both `Test_accepts_a_handoff_of_exactly_the_configured_cap` and
  `Test_accepts_a_handoff_of_exactly_the_configured_cap_with_no_trailing_newline` (at-cap now
  refused); `Test_refuses_a_handoff_one_line_over_the_configured_cap` (over-by-one) stayed
  green — exactly as predicted. Restored and `diff`-confirmed byte-identical.
- `go doc ./internal/scaffold Server.Finish` read back after Step 16, confirming the contract.

## Open debts carried forward

- This repository's own handoffs (29–373 lines) and `STATE.md` (over 200 lines) are already
  over both the 60-line handoff cap and the 80-line state cap this scenario and its twin (18)
  introduce. Harmless today — `brief` is not self-hosted (`brief start brief` still fails on
  missing frontmatter) — but `brief finish brief <step>` will refuse on its own STATE.md once
  18 lands. Owner: `check` (22), per R18.
