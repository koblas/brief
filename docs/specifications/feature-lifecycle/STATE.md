# feature-lifecycle — current state

Scenarios complete: SCENARIO-01..03. Last updated by SCENARIO-03.

## Binding decisions

- `stepSkeleton` (`internal/scaffold/render.go`) emits `cfg.AcceptanceHeading`, a blank
  line, then `cfg.ChecklistHeading` — both already-configurable fields. Do not reintroduce
  an acceptance-less skeleton. (SCENARIO-01)
- `conform.ChecklistItemCount(body []byte, heading string) (int, bool)` is the only item
  counter (Rule 4): `markdown.CountChecklistItems` and `FirstUnchecked` share one unexported
  scanner. `assemble.StartFS`'s zero-item shortfall and `scaffold.checkStepPlanned`'s
  zero-item/absent-heading refusal both call it on the *whole* step file (not the
  frontmatter-stripped body), taking the heading's line from `markdown.HeadingLine` against
  that same whole body. The counter returns a count, not a `Violation`: each caller carries
  its own copy. A second counter, or a direct call to `markdown.CountChecklistItems` from
  `assemble` or `scaffold`, breaks the one decision point. (SCENARIO-02, SCENARIO-03)
- `assemble.StartFS`'s `Brief.Shortfalls` order is fixed: acceptance, then checklist, then
  state headings; `--json` `shortfalls[]` uses the same order. (SCENARIO-02)
- `scaffold.ErrUnplannedStep` is the one sentinel for finish's two new refusals (absent
  heading: `Line` 0; zero items: the heading's own line), declared in
  `internal/scaffold/errors.go`. `checkStepPlanned` runs only for an open step (`!fm.Done()`,
  `FinishFS`), in `checkStepChecklist`'s slot: after the argument checks, before
  `checkStepDependencies`. This is deliberately asymmetric with `checkStepChecklist`
  (`ErrOpenChecklistItem`), which still fires on a done step re-finish — a done step's
  identical re-finish must stay idempotent, and its divergent re-finish must keep the
  existing `ErrAlreadyFinished` refusal, so `checkStepPlanned` cannot run there. The
  acceptance section stays optional for finish (Rule 5): `checkStepPlanned` must never grow
  to cover it. The spec never ruled on `ErrUnplannedStep`'s name (new exported Go API), so
  the final product-vision pass may rename it; no `internal/cli` change was needed —
  `classifyRefusal` renders every `*scaffold.RefusalError` generically. (SCENARIO-03)

## Left unbuilt

- `finishLong`'s help sentence about the new refusal, `brief new feature`'s success line,
  root `--help`'s lifecycle paragraph, `startLong`'s amended sentence, and the
  `docs/specifications/brief/specification.md` R11/new-step amendments — all SCENARIO-04.
- Skill and CLAUDE.md snippet changes (Rule 8) — SCENARIO-05/06.

## Traps

- `assemble.Check` (and `--hook`) must not call `ChecklistItemCount`, and
  `conform.OpenChecklistItem` must keep treating an absent heading or zero items as "never a
  violation" (Rule 6: check reports nothing new on a pre-existing tree; `assemble.Check`
  depends on that contract for its own unticked-item rule, which only fires on a done step).
  Mutation-verified: making `OpenChecklistItem` flag absent/zero reddens only `check`'s done
  rows, since `check` skips its checklist rule entirely for an open step. (SCENARIO-02,
  SCENARIO-03)
- `markdown.Section` trims a whitespace-only section body to `""`, so a `TrimSpace(x)==""`
  guard on an already-extracted `Section` body is provably identical to `x==""`; prove such a
  guard by adding/removing it, not by swapping the comparison. (SCENARIO-02)
- `stepfile.ParseFrontmatter`'s second return is the frontmatter-stripped body: feeding it to
  `markdown.HeadingLine` instead of the whole file gives a wrong-but-plausible-looking line.
  Mutation-verified against `checkStepPlanned`. (SCENARIO-03)
- `fixtureConfig()` in `internal/scaffold/scaffold_test.go` is shared by ~12 test files; any
  scaffold test that `NewStep`s and then `Finish`es now needs a ticked checklist item, or it
  hits the new refusal. (SCENARIO-01, SCENARIO-03)

## Open debts

(none)
