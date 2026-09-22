---
id: SCENARIO-16
status: done
---

# SCENARIO-16: Finishing a done step with different inputs is refused

## Scenario

```gherkin
Scenario: SCENARIO-16 Finishing a done step with different inputs is refused  [orig: new]
  Given a step already finished
  When I finish it again with a handoff that differs from the recorded one
  Then the command is refused, naming the divergence and what to do instead
  And every file is byte-identical
  # R11 read literally would succeed and discard the new handoff, leaving the caller
  # believing it recorded something it did not.
```

## Measured behaviour today (the defect)

Built `./cmd/brief`, scaffolded `widgets` + `SCENARIO-01`, finished once with
`h1.md`/`s1.md`, then re-ran `brief finish widgets SCENARIO-01` four ways, checksumming
all four files before and after each run (each case on its own copy of the finished tree):

| case | handoff | state | exit | stderr                              | files                                    |
| ---- | ------- | ----- | ---- | ----------------------------------- | ---------------------------------------- |
| a    | same    | same  | 0    | `brief finish: SCENARIO-01 is done` | all 4 byte-identical (SCENARIO-06 no-op) |
| b    | differs | same  | 0    | `brief finish: SCENARIO-01 is done` | handoff file **overwritten**             |
| c    | same    | differs | 0  | `brief finish: SCENARIO-01 is done` | state file **overwritten**               |
| d    | differs | differs | 0  | `brief finish: SCENARIO-01 is done` | handoff **and** state **overwritten**    |

(b)-(d) are the failure R11 names: the caller is told `is done` while the previously
recorded content is discarded — or, symmetrically, while their new content replaces a
record they never asked to change. (b)-(d) become exit-1 refusals in this scenario.
(a) must stay exactly as measured.

Two convergence paths were also measured and must survive unchanged:

- handoff file deleted, identical inputs → exit 0, handoff file rewritten.
- handoff file deleted, **different** state → exit 0, state rewritten.

## The shared predicate

SCENARIO-06's no-op and SCENARIO-16's refusal are branches of **one** evaluation, not two
parallel conjunctions that can drift. `internal/scaffold/finish.go` today computes a single
`identical` bool at the site where `stateBytes` and the recorded handoff are read. That
site becomes:

- an unexported struct `refinish` carrying the five facts already in scope — `done`
  (`fm.Done()`), `handoffRecorded` (the handoff file read without error), `handoffMatches`,
  `stateMatches`, `specTicked` (`newSpec == string(specBytes)`);
- one method `(refinish) verdict() refinishVerdict` returning exactly one of
  `refinishWrite`, `refinishNoop`, `refinishHandoffDiverged`, `refinishStateDiverged`;
- one `switch` on that verdict in `Finish`, replacing the `if fm.Done() && identical`.

Rows, in the order `verdict` decides them:

1. `!done` → `refinishWrite`.
2. `done && !handoffRecorded` → `refinishWrite`. **The exemption**, see below.
3. `done && !handoffMatches` → `refinishHandoffDiverged`.
4. `done && !stateMatches` → `refinishStateDiverged`.
5. `done && specTicked` → `refinishNoop` (R11, SCENARIO-06).
6. `done && !specTicked` → `refinishWrite` (the crash-after-step-file retry).

**`specTicked` participates only in the noop-vs-write split (rows 5/6), never in a
diverged arm.** The progress tick is derived from the specification, not a caller input, so
an un-ticked entry means "half-applied write to repair", not "different inputs". The proof
is an existing, untouched test: `internal/scaffold/finish_test.go`
`Test_reports_a_specification_write_that_cannot_be_committed` blocks the fourth write, so
the tree is left at done + handoff matches + state matches + spec un-ticked, then retries
with the *same* arguments and requires convergence. A naive `done && !identical → refuse`
reddens it. That reasoning goes in a comment on `verdict`, citing the test by name — the
`specTicked` field sitting in scope is otherwise an invitation to fold it in.

Rows 3 and 4 are ordered handoff-first, matching the write order and R14a's "names the
first thing wrong", so case (d) is reported as a handoff divergence.

`handoffMatches` keeps the `handoffReadErr == nil` conjunct it has today, so rows 2 and 3
are order-insensitive on that pair and an absent file cannot sneak into `handoffMatches`
through `string(nil) == ""`.

**`verdict` is a strict refinement of SCENARIO-06.** Row 5 is reached only when
`done && handoffRecorded && handoffMatches && stateMatches && specTicked`, which is
bit-for-bit today's `fm.Done() && identical`. The no-op's truth condition is therefore
unchanged and the only behavioural delta is rows 3 and 4 — a reviewer can verify that by
reading `verdict`, without running anything. Keeping tests `:173`/`:189` green only
samples it.

## The refusal copy

Both arms return a `*scaffold.RefusalError` wrapping one new sentinel,
`ErrAlreadyFinished` (`errors.New("step already finished with different inputs")`).
`cli/refusal.go`'s existing `*scaffold.RefusalError` branch renders it and `cli.ExitCode`
maps it to 1 — **no `internal/cli` change is needed for rendering or exit code**.

The message names the **specific divergent file** rather than refusing generically: the
caller's only route forward is to read the recorded bytes and compare, and a generic "your
inputs differ" makes them diff two files to find out which. R14a's "measured value and the
limit" has no number here; the divergent input's identity is the measured fact.

`Path` is the recorded file on disk, absolute:

- handoff → `<featureDir>/<stepPattern.ID(n)+cfg.HandoffFileSuffix>`
- state → `<featureDir>/<cfg.StateFile>`

`Line` is 0 (whole-file). **Do not route the state arm through `scaffold.StateSource`** —
`cli/finish.go`'s placeholder substitution would repoint the message at the caller's
`--state` input, which is the file they already have, not the record they need to read.
`StateSource` stays reserved for refusals about the argument's own bytes (the fence check).

Exact rendered lines (the whole of stderr; stdout is empty; exit 1):

```
brief finish: <abs>/SCENARIO-01-HANDOFF.md: step "SCENARIO-01" is already done and the given handoff differs from the one recorded here; diff the handoff you passed against it, then edit this file directly if the new handoff is correct (no files changed)
brief finish: <abs>/STATE.md: step "SCENARIO-01" is already done and the given state differs from the one recorded here; diff the state you passed against it, then edit this file directly if the new state is correct (no files changed)
```

The `Fix` says "the handoff/state you passed", not "your `--handoff` file": `--handoff -`
and `--state -` read from stdin, where no such file exists.

The `Fix` names only actions that exist. There is no `--force`, and R20 explicitly defers
`--if-state-matches`; naming either would invent a contract 17-21 do not own.

## The absent-handoff exemption (row 2 — the subtlest interaction)

A done step whose handoff file is **missing or unreadable** stays repairable: `verdict`
returns `refinishWrite` without consulting `handoffMatches` or `stateMatches`. Two real
trees land there — a crash between the state write and the step write followed by a hand
edit, and a migrated tree whose step was marked done before handoff files existed. With no
recorded handoff there is nothing to diverge *from*, and since `finish` is the only path to
a done step (R10) a refusal here would be a dead end. Measured case (f) above already
exercises the version of this with a differing state; that row is the one genuinely
uncovered by today's tests and gets its own test below.

## Implementation Plan

Verification commands and the mutation/stash protocol are in `.claude/rules/agent-briefs.md`.
Run everything from the worktree root.

- [x] Step 1: `internal/scaffold/finish_idempotent_test.go` `Test_re_finishing_a_done_step_with_a_different_handoff_replaces_it` — rewrite as `Test_re_finishing_a_done_step_with_a_different_handoff_is_refused`: `require.ErrorIs(scaffold.ErrAlreadyFinished)`, assert the `*RefusalError`'s `Path` is the handoff file and `Problem`/`Fix` match the copy above (red — update)
- [x] Step 2: `internal/scaffold/finish_idempotent_test.go` `Test_re_finishing_a_done_step_with_a_different_state_body_replaces_it` — rewrite as `..._is_refused`, same shape, `Path` is `cfg.StateFile` (red — update)
- [x] Step 3: `internal/scaffold/finish_idempotent_test.go` `Test_re_finishing_a_done_step_with_both_inputs_differing_names_the_handoff_first` — pins R14a's first-thing-wrong order for measured case (d) (red — new)
- [x] Step 4: `internal/scaffold/finish_idempotent_test.go` `Test_a_refused_re_finish_leaves_every_file_byte_identical` — pinned to the **state** arm (handoff present and matching, state differs); `snapshotTree` before/after the refusal, plus `pinModTimes`/`modTimes`/`pinnedModTime` so the claim is "refused before the first write", not merely "rewrote identical bytes" (red — new)
- [x] Step 5: `internal/scaffold/finish_idempotent_test.go` `Test_the_snapshot_probe_sees_a_write_on_a_legitimate_finish` — probe-sanity arm: the same `snapshotTree` probe over a first, successful `Finish` must show a difference (new)
- [x] Step 6: `internal/scaffold/finish_idempotent_test.go` `Test_re_finishing_a_done_step_whose_handoff_file_is_missing_accepts_a_different_state` — the absent-handoff exemption with a diverging state; measured case (f). **This is Step 4's control arm**: identical to it but for the handoff file's presence, one variable, opposite outcome, same probe. Step 5 flips both doneness and divergence, so it cannot play that role (red — new)
- [x] Step 7: `internal/scaffold/finish_idempotent_test.go` `Test_re_finishing_a_done_step_with_an_un_ticked_entry_and_a_different_handoff_is_refused` — pins that an un-ticked progress entry is not an escape hatch from the divergence arms (red — new)
- [x] Step 8: `internal/scaffold/errors.go` `ErrAlreadyFinished` — sentinel + doc comment naming both arms and the absent-handoff exemption (new)
- [x] Step 9: `internal/scaffold/finish.go` `refinishVerdict`, `refinish`, `(refinish).verdict` — the single predicate, with the `specTicked`-is-not-a-divergence comment citing `Test_reports_a_specification_write_that_cannot_be_committed` (green — new)
- [x] Step 10: `internal/scaffold/finish.go` `(*Server).Finish` — replace `identical` + `if fm.Done() && identical` with the `switch verdict`; it stays where `identical` is computed today, i.e. after `checkArgumentFence(state, …)` so a malformed state argument still refuses first, and at/after `SetStatus` so `ErrNoStatusField` keeps its documented pre-write position (green — update)
- [x] Step 11: `internal/cli/finish_test.go` `Test_refuses_a_re_finish_whose_handoff_differs_from_the_recorded_one` — slice test through `cli.Run`: `assert.Equal` on the **whole** stderr string (a `Contains` would pass while `brief finish: … is done` is also printed, which is half the defect), stdout empty, `cli.ExitCode(err) == 1`, all four files byte-identical (new)
- [x] Step 12: `internal/scaffold/finish.go` `(*Server).Finish` doc comment — insert the divergence refusal into the enumerated check order at its real position and rewrite the sentence "any single divergence, including an absent handoff file, writes as normal", which this scenario makes false; state the three outcomes (update)
- [x] Step 13: `internal/scaffold/doc.go` — same correction to the package-doc paragraph on `Finish`'s no-op; verify with `go doc ./internal/scaffold` (update)
- [x] Step 14: `internal/scaffold/finish_idempotent_test.go` `Test_re_finishing_a_done_step_whose_handoff_file_is_missing_rewrites_it` — its doc comment claims to prove handoff-existence is part of the *identity* conjunct; after this scenario the sharper claim is that absence is an **exemption from the divergence refusal**. Reword, do not change the body (update)
- [x] Step 15: mutation-verify, one at a time, stashing each, and report which mutation reddened which test:
  - (a) delete row 3 (the handoff arm) → Steps 1 **and 3** redden. Step 3's both-differ case falls through to row 4 and reports a state divergence, so its "handoff is named" assertion fails too — that second red is expected, not a defect in the implementation.
  - (b) delete row 4 (the state arm) → **only** Step 2's test reddens. Step 3 still hits row 3 and passes.
  - (c) drop the `handoffRecorded` exemption (row 2) → `Test_re_finishing_a_done_step_whose_handoff_file_is_missing_rewrites_it` (`:106`) and Step 6's test redden.
  - (d) add `!specTicked` to the divergence trigger → `finish_test.go` `Test_reports_a_specification_write_that_cannot_be_committed` reddens, and so do `:133` and `:153`.
- [x] Step 16: `go build ./...`, `go test ./...` unpiped from the worktree root, `go test -race ./internal/scaffold/... ./internal/cli/...`, `golangci-lint run ./...`; report the exact test count and the delta
- [x] Step 17: `docs/specifications/brief/specification.md` — tick SCENARIO-16 in `## BDD Acceptance Progress`
- [x] Step 18: `docs/specifications/brief/STATE.md` — rewrite per the rolling-STATE convention, folding in the Handoff below

### Green on arrival — do not edit, do not manufacture a red

These already exist and must stay green unchanged. Report them as green-on-arrival.

- `finish_idempotent_test.go:173` `Test_re_finishing_a_done_step_with_the_same_inputs_preserves_every_modification_time`
- `finish_idempotent_test.go:189` `Test_re_finishing_a_done_step_with_the_same_inputs_leaves_every_file_byte_identical`

  These two are the R11 identity regression pins — the regression most likely to slip.

- `finish_idempotent_test.go:133` un-ticked entry re-ticked · `:153` mtime control arm ·
  `:208` open frontmatter forced write
- `finish_test.go` `Test_reports_a_{handoff,state,step_file,specification}_write_that_cannot_be_committed`
- `cli/finish_test.go:83` `Test_finishing_an_already_finished_step_a_second_time_prints_the_same_line_and_succeeds`

The full inventory of `Finish` call sites in tests was enumerated (`grep -rn "\.Finish(" internal --include="*_test.go"`,
37 sites) rather than estimated — STATE.md's "a plan's fixture-update list can under-count"
trap. Every other site finishes an **open** step, so `verdict` row 1 covers it.

### Out of scope

No caps of any kind (17/18), no state-heading check (19), no checklist parsing (20), no
`depends-on` check (21). No `--force`, no `--if-state-matches`, no diff output (R9), no
`FinishResult`. No new flag, no `internal/cli` rendering or exit-code change.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- **One predicate, `(refinish).verdict()`, decides all three outcomes** — SCENARIO-06's
  no-op and SCENARIO-16's two refusals are branches of the same function, not parallel
  conjunctions; splitting them lets the no-op and the refusal drift apart over the same
  five facts.
- **`specTicked` is never a divergence trigger** — `finish_test.go`
  `Test_reports_a_specification_write_that_cannot_be_committed` leaves the tree at done +
  handoff matches + state matches + spec un-ticked and requires a same-argument retry to
  converge; folding the spec conjunct into the refusal reddens it.
- **A missing or unreadable handoff file exempts a done step from the refusal entirely** —
  a crash-then-hand-edit tree and a pre-handoff-file migrated tree both land there, and
  `finish` is the only path to a done step (R10), so refusing is a dead end.
- **The refusal names the divergent file, not the divergence generically** — handoff arm
  carries the handoff file's path, state arm carries `cfg.StateFile`'s, `Line` 0, both
  wrapping `ErrAlreadyFinished`, both exit 1 with R14a's `(no files changed)` tail.
- **The state arm does not use `scaffold.StateSource`** — `cli/finish.go:99` rewrites that
  placeholder to the caller's `--state` path, which points at the file the user already
  has instead of the record they must read.
- **Handoff is checked before state**, so a both-differ call reports the handoff (R14a
  first-thing-wrong, matching the write order).

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
  true for the identical case it actually claims; the differing case is new. Resolution:
  read the recorded file and edit it directly, or re-run with the recorded body.
- **`snapshotTree` alone under-proves "nothing lands"** — it compares bytes, so a refusal
  taken *after* a byte-identical rewrite passes it. Pair it with the `pinnedModTime`
  probe from `finish_idempotent_test.go` to claim "refused before the first write".
- **`assert.Contains` on the refusal text is unfalsifiable here** — today's code prints
  `brief finish: <step> is done` on exactly these inputs, so a `Contains` assertion passes
  with the guard deleted. Assert `Equal` on the whole stderr string.
- Measured cases (b)/(c)/(d) rewrote **all four** files; only the divergent one changed
  content, the other three were rewritten byte-identically. A checksum-only probe sees
  two unchanged files where four writes occurred — another reason the mtime probe is the
  load-bearing one.
- `finish_idempotent_test.go:87` and `:119` were written *as* the arms this scenario
  inverts, and their comments say so. They are rewrites, not additions — leaving them
  green means the refusal never landed.
