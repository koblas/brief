# SCENARIO-16 Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- **One predicate, `(refinish).verdict()`, decides all three outcomes** — SCENARIO-06's
  no-op and SCENARIO-16's two refusals are branches of the same function, not parallel
  conjunctions; splitting them lets the no-op and the refusal drift apart over the same
  five facts (`done`, `handoffRecorded`, `handoffMatches`, `stateMatches`, `specTicked`).
- **`specTicked` is never a divergence trigger.** `finish_test.go`
  `Test_reports_a_specification_write_that_cannot_be_committed` leaves the tree at done +
  handoff matches + state matches + spec un-ticked and requires a same-argument retry to
  converge; folding the spec conjunct into the refusal reddens it (mutation-verified).
- **A missing or unreadable handoff file exempts a done step from both refusals entirely** —
  a crash-then-hand-edit tree and a pre-handoff-file migrated tree both land there, and
  `finish` is the only path to a done step (R10), so refusing is a dead end.
- **The refusal names the divergent file, not the divergence generically** — handoff arm
  carries the handoff file's absolute path, state arm carries `cfg.StateFile`'s absolute
  path, `Line` 0, both wrapping `ErrAlreadyFinished`, both exit 1 with R14a's
  `(no files changed)` tail.
- **The state arm does not use `scaffold.StateSource`** — `cli/finish.go:99` rewrites that
  placeholder to the caller's `--state` path, which points at the file the user already has
  instead of the record they must read. `alreadyFinishedRefusal`'s state call passes the
  recorded `statePath` (already computed earlier in `Finish`) directly.
- **Handoff is checked before state** (rows 3 before 4 in `verdict`), so a both-differ call
  reports the handoff — R14a's "names the first thing wrong", matching the write order.

**Left unbuilt** — named so nobody assumes it exists:

- `--force`, `--if-state-matches` (R20 defers it explicitly), any diff/finding output on a
  divergence (R9), `FinishResult` — no owner.
- Caps (17/18), `scaffold.HandoffSource` / `--handoff` source upgrade (17), state-heading
  check (19), `markdown.Headings` + checklist parser (20), `depends-on` check (21).
- No un-finish / un-done verb, and none is planned — the documented escape from a refusal
  is to edit the recorded file directly.

**Traps** — things that look right and are not:

- **`brief`'s own pipeline hits this wall.** A `developer` in fix mode re-running `finish`
  on an already-done step with a regenerated STATE.md body is now **refused** where it
  previously overwrote silently. STATE.md's crossover note ("safely re-runnable") remains
  true only for the identical-inputs case it actually claims; the differing case is new as
  of this scenario. Resolution: read the recorded file and edit it directly, or re-run with
  the recorded body.
- **`snapshotTree` alone under-proves "nothing lands"** — it compares bytes, so a refusal
  taken *after* a byte-identical rewrite would still pass it. Every refusal test in this
  scenario pairs it with the `pinnedModTime`/`pinModTimes`/`modTimes` probe from
  `finish_idempotent_test.go`, proving "refused before the first write" rather than merely
  "rewrote identical bytes".
- **`assert.Contains` on the refusal text is unfalsifiable here** — before this scenario the
  code printed `brief finish: SCENARIO-01 is done` on exactly the differing-handoff inputs,
  so a `Contains` assertion on the new refusal text would still pass with the guard deleted.
  Both the scaffold-level tests and the CLI slice test assert `Equal`/whole-string.
- Measured cases (b)/(c)/(d) (see SCENARIO-16.md) rewrote **all four** files under the old
  code; only the divergent one changed content, the other three were rewritten
  byte-identically. A checksum-only probe sees two unchanged files where four writes
  occurred — another reason the mtime probe is load-bearing, not optional.
- `finish_idempotent_test.go:87` and `:119` (pre-scenario line numbers) were written *as*
  the arms this scenario inverts, and their comments said so. They were rewritten in place
  — `Test_re_finishing_a_done_step_with_a_different_handoff_is_refused` and
  `Test_re_finishing_a_done_step_with_a_different_state_body_is_refused` — not left green
  unchanged; leaving them green would mean the refusal never landed.
- The switch in `(*Server).Finish` has an explicit `case refinishWrite:` with a comment
  ("falls through to the four writes below") purely to satisfy `golangci-lint`'s
  `exhaustive` check — `refinishWrite` does no branch-specific work, it is the
  fall-through-to-normal-writes case.

## Session notes

- Baseline measured before any change: `go test -v ./... 2>&1 | grep -c -- "--- PASS"` gave
  339, matching STATE.md's inherited count.
- RED confirmed as a compile-fail: after rewriting the two existing tests and adding the
  five new ones to reference `scaffold.ErrAlreadyFinished` before that sentinel existed,
  `go vet ./internal/scaffold/...` reported `undefined: scaffold.ErrAlreadyFinished`.
- GREEN reached in one pass after adding `ErrAlreadyFinished` (errors.go), the `refinish`
  struct/`verdict()` method and `alreadyFinishedRefusal` helper, and wiring the `switch
  r.verdict()` into `Finish` in place of `if fm.Done() && identical`. No implementation
  iteration was needed beyond that — `go test ./internal/scaffold/...` was green on the
  first run after the production edit.
- The CLI slice test (`Test_refuses_a_re_finish_whose_handoff_differs_from_the_recorded_one`)
  needed no `internal/cli` production change, as the plan predicted: the existing
  `*scaffold.RefusalError` branch in `renderRefusal` already renders `ErrAlreadyFinished`'s
  refusal with the `(no files changed)` tail and `cli.ExitCode` already maps it to 1.
- `Test_finishing_an_already_finished_step_a_second_time_prints_the_same_line_and_succeeds`
  (`cli/finish_test.go:83`) was verified green on arrival, unchanged: it re-finishes with the
  **same** argv both times, so `verdict()` reaches `refinishNoop` and the behaviour is
  identical to before this scenario.
- Mutation verification used `cp`/`diff` against `$TMPDIR`, not `git stash` — this worktree's
  stash stack is shared across sessions per the environment reminder, and `git` access itself
  required `dangerouslyDisableSandbox: true` for every Bash call touching the repo (the
  sandbox's read-deny list covers `/Users/koblas` broadly; only the Read/Write/Edit tools and
  sandbox-disabled Bash calls could reach the worktree).
- All four mutations from the plan's Step 15 reddened exactly the predicted tests (see
  Verification below) and each restore diffed byte-identical against the pre-mutation copy
  before proceeding to the next mutation.
- `golangci-lint run ./...` initially failed with two findings, both fixed: `exhaustive`
  wanted an explicit `case refinishWrite:` in the `switch r.verdict()` in `Finish`, and `lll`
  flagged one 269-character line in the new CLI test's expected-stderr string, wrapped across
  three string-literal concatenations.

## Verification

- `go build ./...` — exit 0.
- `go test ./...` — exit 0, all packages ok, 0 skips.
- `go test -race ./internal/scaffold/... ./internal/cli/...` — exit 0.
- `golangci-lint run ./...` — 0 issues (two findings surfaced and fixed first; see above).
- Test count: 345 passing (`grep -c -- "--- PASS"`, unanchored so subtests count, matching
  this pipeline's convention), delta **+6** from the 339 baseline — two tests were rewritten
  in place (no count change) and six are new: `Test_re_finishing_a_done_step_whose_handoff_file_is_missing_accepts_a_different_state`,
  `Test_re_finishing_a_done_step_with_both_inputs_differing_names_the_handoff_first`,
  `Test_a_refused_re_finish_leaves_every_file_byte_identical`,
  `Test_the_snapshot_probe_sees_a_write_on_a_legitimate_finish`,
  `Test_re_finishing_a_done_step_with_an_un_ticked_entry_and_a_different_handoff_is_refused`
  (scaffold), and `Test_refuses_a_re_finish_whose_handoff_differs_from_the_recorded_one`
  (cli).
- Mutation (a), delete row 3 (handoff arm): reddened
  `Test_re_finishing_a_done_step_with_a_different_handoff_is_refused` and
  `Test_re_finishing_a_done_step_with_both_inputs_differing_names_the_handoff_first` (the
  latter falls through to row 4 and names `NOTES.md` instead of the handoff file — the
  predicted second red).
- Mutation (b), delete row 4 (state arm): reddened only
  `Test_re_finishing_a_done_step_with_a_different_state_body_is_refused`; the both-differ
  test still passed (still hits row 3).
- Mutation (c), drop the `handoffRecorded` exemption (row 2): reddened
  `Test_re_finishing_a_done_step_whose_handoff_file_is_missing_rewrites_it` and
  `Test_re_finishing_a_done_step_whose_handoff_file_is_missing_accepts_a_different_state`.
- Mutation (d), fold `!specTicked` into the state-divergence trigger: reddened
  `Test_reports_a_specification_write_that_cannot_be_committed` (finish_test.go),
  `Test_re_finishing_a_done_step_whose_progress_entry_was_un_ticked_re_ticks_it`, and
  `Test_the_modification_time_probe_sees_a_write_when_the_progress_entry_diverges`.
- `go doc ./internal/scaffold` read after Steps 12/13, confirming the package-doc and
  `Finish` doc comments describe the three outcomes (no-op, handoff-diverged refusal,
  state-diverged refusal) and the absent-handoff exemption.
