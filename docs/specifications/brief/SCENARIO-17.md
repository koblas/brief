---
id: SCENARIO-17
status: done
depends-on: []
---

# SCENARIO-17: An over-cap handoff is refused and nothing lands

## Scenario

```
Scenario: SCENARIO-17 An over-cap handoff is refused and nothing lands  [orig: 06a]
  Given an open step whose checklist is complete
  When I finish it with a handoff over the configured cap
  Then the refusal names the measured line count and the cap
  And both files are byte-identical
```

## Measured starting point (do not re-measure)

Built `./cmd/brief` and ran `brief finish widgets SCENARIO-01 --handoff <n-line body> --state <state>`
against a freshly scaffolded feature. Handoff bodies of 59, 60, 61 and 200 lines all exit 0,
print `brief finish: SCENARIO-01 is done`, and write all four files.

**No cap is enforced anywhere today.** `cfg.HandoffCapLines`, `cfg.StateCapLines` and
`cfg.DefaultOutputBudgetBytes` are declared in `internal/platform/config/config.go:57-59`
(defaults 60 / 80 / 8192) and referenced by nothing outside `config` and its own
`resolve_test.go`. `finish.go`'s `Finish` doc comment says so in as many words: handoff and
state are "written verbatim, neither checked against a length cap or a required-heading
schema". SCENARIO-17 is the first consumer of any cap.

## Decisions this scenario takes

1. **Units are lines, not bytes or runes.** The config keys are `handoff-cap-lines` /
   `state-cap-lines` and the Gherkin says "the measured line count" — the unit is named by
   the key, so no bytes-versus-runes question arises and the refusal copy says "lines".
   A line count is `strings.Count(body, "\n")`, plus 1 when the body is non-empty and does
   not end in `"\n"`; `""` is 0 lines and a trailing newline never adds a phantom line.
   `"\r\n"` counts once, like every other line ending.
2. **Cap source:** `cfg.HandoffCapLines`, yaml `handoff-cap-lines`, shipped default **60**.
   Read from the `Config` the `Server` was constructed with — never a package constant.
3. **Where the check runs:** inside `(*scaffold.Server).Finish`, in the existing
   argument-validation band — immediately **after** the frontmatter parse and immediately
   **before** `checkArgumentFence(state, StateSource, "state")` (`finish.go:147`). This band
   already sits before the `r.verdict()` switch (`finish.go:~218`), so the codebase has
   already decided that an invalid *argument* pre-empts a would-be no-op: an identical
   re-finish whose state carries an unclosed fence is refused today. The handoff cap matches
   that slot rather than inventing a new rule. Consequence, pinned by a test: **an over-cap
   handoff aimed at an already-done step reports the cap, not `ErrAlreadyFinished`.**
4. **Exact refusal copy** (R14a, write refusal, so the tail is present), exit **1**, stderr,
   stdout empty:

   ```
   brief finish: <handoff source>: handoff is 61 lines, over the cap of 60; cut the handoff to 60 lines or fewer, or raise handoff-cap-lines in .brief.yaml, and retry (no files changed)
   ```

   `RefusalError.Line` is **0** — the Gherkin asks for a count, not a location, and `cap+1`
   would be arithmetic on the cap rather than an observed line; `check` (22) sets a real line
   when it reports an on-disk file. `<handoff source>` is the `--handoff` path, or `<stdin>`
   when it was `-`: `Finish` never learns where the bytes came from, so the refusal carries
   the new `scaffold.HandoffSource` placeholder that `cli/finish.go` swaps, exactly as it
   already does for `scaffold.StateSource`.
5. **Boundary:** a body of exactly `cfg.HandoffCapLines` lines is **accepted**; `cap+1` is
   **refused**. Both are pinned by their own test, and the comparison operator is
   mutation-verified (step 15).

## Implementation Plan

- [x] Step 1: `internal/platform/markdown/lines_test.go` `Test_counts_lines_...` — `""`→0, `"a"`→1, `"a\n"`→1, `"a\nb"`→2, `"a\nb\n"`→2, CRLF body, body of only `"\n"`→1 (red)
- [x] Step 2: `internal/platform/markdown/lines.go` `CountLines` — the shared counter (green)
- [x] Step 3: `internal/platform/markdown/doc.go` — one sentence naming `CountLines` as the cap counter both the write path and `check` use (update)
- [x] Step 4: fixture-cap enumeration — count the lines of **every** handoff body handed to `Finish`/`cli.Run` across `internal/scaffold/scaffold_test.go`, `finish_test.go`, `finish_idempotent_test.go` (`newFinishedFixture` → `fx.newHandoff`) and `internal/cli/finish_test.go`; enumerate, do not estimate (update)
- [x] Step 5: `internal/scaffold/scaffold_test.go` `fixtureConfig` — set `HandoffCapLines` to a small value distinct from 60 and strictly above the maximum found in Step 4, so every cap test proves the value is read from config (update)
- [x] Step 6: `internal/scaffold/finish_cap_test.go` `Test_refuses_a_handoff_one_line_over_the_configured_cap` — `Finish` with a `cap+1`-line handoff; assert `errors.Is(err, scaffold.ErrOverCap)`, `RefusalError.Path == scaffold.HandoffSource`, `Line == 0`, and `Equal` on the whole `Error()` string (red)
- [x] Step 7: `internal/scaffold/errors.go` — `ErrOverCap` sentinel (shared with 18) and the `HandoffSource` placeholder const, both documented (new)
- [x] Step 8: `internal/scaffold/finish.go` — the cap check in the argument band, after the frontmatter parse and before `checkArgumentFence` (green)
- [x] Step 9: `internal/scaffold/finish_cap_test.go` `Test_accepts_a_handoff_of_exactly_the_configured_cap` — plus a second body of the same line count with no trailing newline, both succeed and both land (green)
- [x] Step 10: `internal/scaffold/finish_cap_test.go` (**`package scaffold_test`**, like `scaffold_test.go` and `finish_idempotent_test.go` — not the internal `package scaffold` that `replace_internal_test.go` uses) `Test_a_refused_over_cap_handoff_leaves_every_file_byte_identical` — `newFinishFixture`/`snapshotTree`/`pinModTimes`/`modTimes` are then in scope; pair the snapshot with the mtime probe (snapshot alone under-proves it), and do not redeclare the helpers or their existing control arms (new)
- [x] Step 11: `internal/scaffold/finish_cap_test.go` `Test_an_over_cap_handoff_on_a_done_step_reports_the_cap_not_the_re_finish_refusal` — `newFinishedFixture`, then `Finish` with an over-cap handoff; `errors.Is(err, ErrOverCap)` true and `errors.Is(err, ErrAlreadyFinished)` false (new)
- [x] Step 12: `internal/cli/finish_test.go` `Test_refuses_a_handoff_over_the_cap_and_names_the_handoff_path` — through `cli.Run` against `config.Default()` (cap 60) with a 61-line file; `Equal` on the whole stderr line, exit code 1, stdout empty — the byte-identity and mtime proof lives at Step 10, so do not restate it here with a weaker probe (red)
- [x] Step 13: `internal/cli/finish.go` — second placeholder branch swapping `scaffold.HandoffSource` for `sourceLocator(*handoffPath)`, beside the existing `StateSource` branch (green)
- [x] Step 14: `internal/cli/finish_test.go` `Test_names_stdin_when_the_piped_handoff_is_over_the_cap` — `--handoff -`, refusal names `<stdin>` (new)
- [x] Step 15: mutation-verify, one variable at a time, each stashed per the standing brief — (a) delete the cap check → Step 6's test reddens, Step 9's stays green; (b) flip the comparison to `>=` → Step 9's at-cap test reddens and Step 6's over-by-one stays green. Report which test each reddened (verify)
- [x] Step 16: `internal/scaffold/finish.go` `Finish` doc comment — delete the "neither checked against a length cap" clause and insert the handoff cap into the enumerated check order at its real position; `go doc ./internal/scaffold` is the contract (update)
- [x] Step 17: verification per the standing brief, then tick SCENARIO-17 in `specification.md` and rewrite `docs/specifications/brief/STATE.md` from this Handoff (update)

## Out of scope — do not build

- **SCENARIO-18** state cap (`cfg.StateCapLines` stays unconsumed), **19** required-heading
  check, **20** open-checklist refusal, **21** `depends-on` refusal, **22** `check`.
- **R13's output budget** (`cfg.DefaultOutputBudgetBytes`) — a *byte* budget on the *read*
  path with truncation semantics, an unrelated cap on an unrelated command. Do not conflate.
- No `--force`, no cap override flag, no validation of the cap *values* themselves.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- Caps are measured in **lines**, by `markdown.CountLines` — `strings.Count(body,"\n")` plus 1
  for a non-empty body with no trailing newline; `""` is 0. It lives in `platform/markdown`,
  not `scaffold`, because `check` (22) needs the same count from the read side and `assemble`
  may not import `scaffold`.
- **One shared `scaffold.ErrOverCap` sentinel for both caps** — the `RefusalError.Path` says
  which body (`HandoffSource` vs `StateSource`), so 18 and 22 branch on cap-ness with one
  `errors.Is` and never need a second sentinel.
- **The cap check is in `Finish`'s argument band, before the `r.verdict()` switch** — same
  slot as `checkArgumentFence`, which already pre-empts a would-be no-op today. So an
  over-cap handoff on an already-done step reports the cap, not `ErrAlreadyFinished`. On an
  **open** step, trimming the body is the whole fix. On a **done** step it is two hops: the
  trimmed body then differs from the recorded one and 16's `ErrAlreadyFinished` fires, so the
  escape is 16's documented one — edit the recorded handoff file directly. Say that, do not
  imply one trim clears it; the legacy over-cap record is what R18 hands to `check` (22).
- **Comparison is `count > cap`** — exactly-at-cap is accepted. Mutation-verified both ways.
- **`scaffold.HandoffSource = "<handoff>"`** exists and `cli/finish.go` swaps it for
  `sourceLocator(*handoffPath)`. Any future refusal about the handoff *argument's* bytes uses
  it; a refusal about the handoff *file* on disk uses the real path instead.
- **Refusal copy is pinned**, tail included:
  `brief finish: <source>: handoff is <n> lines, over the cap of <cap>; cut the handoff to
  <cap> lines or fewer, or raise handoff-cap-lines in .brief.yaml, and retry (no files
  changed)`. `Line` is 0.

**Left unbuilt** — named so nobody assumes it exists:

- `cfg.StateCapLines` — still unconsumed; owner is SCENARIO-18. `cfg.DefaultOutputBudgetBytes`
  — still unconsumed; R13, no owner. Do not wire either "while you are in there".
- Validation of cap *values* (0 or negative is accepted as configured; a cap of 0 renders the
  cosmetically odd "is 1 lines") — unowned, adjacent to 19's heading/cap-value debt.
- No `--force`, no per-invocation cap override, no truncation — refusal is the only outcome.

**SCENARIO-18 is a small delta — what it reuses versus re-decides:**

- Reuses unchanged: `markdown.CountLines`, `ErrOverCap`, the existing `StateSource`
  placeholder and its `cli/finish.go` swap, the copy shape (swap "handoff"→"state",
  `handoff-cap-lines`→`state-cap-lines`), and `finish_cap_test.go`'s fixture and probes. Its
  slot is **between** the handoff cap check and `checkArgumentFence(state, …)`, so the handoff
  cap is reported first when both bodies are over — the handoff-first rule 16 set.
- Re-decides nothing structural. It adds `cfg.StateCapLines` to `fixtureConfig` under Step 4's
  enumeration discipline (`newStateBody` is 4 headings × 4 lines) and amends the same `Finish`
  doc-comment clause again.

**Traps** — things that look right and are not:

- `cli/finish.go`'s placeholder swap currently matches **only** `scaffold.StateSource`; a new
  `HandoffSource` refusal renders the literal `<handoff>` until Step 13 lands. That is what
  makes Step 12 red.
- Existing fixture handoff bodies are small but not trivial (`newFinishFixture`'s is 5 lines,
  and `finish_idempotent_test.go` reuses it) — a fixture cap chosen below one of them turns a
  dozen unrelated tests red for the wrong reason. Enumerate them (Step 4); 13's plan
  under-counted fixtures exactly this way.
- `snapshotTree` enumerates every non-directory entry, so a leftover temp file already shows
  up as a snapshot difference — no separate R12 temp-file sweep is needed. Both probes already
  have control arms (`Test_the_snapshot_probe_sees_a_write_on_a_legitimate_finish`,
  `Test_the_modification_time_probe_sees_a_write_when_the_progress_entry_diverges`) — cite
  them, do not duplicate them.
- `Finish`'s doc comment states the full ordered check list and currently *denies* that a cap
  is checked. Leaving it is a contract lie `go doc` publishes; Step 16 is not optional.
- **This repository's own bodies are over both caps**: its handoff files run 29–373 lines
  against a 60-line cap and `STATE.md` is 209 against an 80-line cap, so once 17 and 18 land
  `brief finish brief <step>` could not write them. It does not bite today (the tool is not
  self-hosted — `brief start brief` still fails on missing frontmatter) and R18 hands "what
  predates the tool" to `check` (22). Recorded, not planned for.
