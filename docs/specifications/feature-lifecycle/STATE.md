# feature-lifecycle — current state

Scenarios complete: SCENARIO-01, SCENARIO-02. Last updated by SCENARIO-02.

## Binding decisions

- `stepSkeleton` (`internal/scaffold/render.go`) emits `cfg.AcceptanceHeading`, a blank
  line, then `cfg.ChecklistHeading` — both already-configurable fields. Do not reintroduce
  an acceptance-less skeleton. (SCENARIO-01)
- `conform.ChecklistItemCount(body []byte, heading string) (int, bool)` is the only item
  counter (Rule 4): `markdown.CountChecklistItems` and `FirstUnchecked` share one unexported
  scanner in `internal/platform/markdown/checklist.go`. `assemble.StartFS` calls it on the
  frontmatter-stripped body (`e.rest`) for the zero-item shortfall row; SCENARIO-03's finish
  refusal must call the same function for both its refusals (absent heading, zero items),
  taking the heading's line from `markdown.HeadingLine` against the *whole* file, not
  `e.rest` (counts match either body; line numbers don't). A second counter, or a direct
  call to `markdown.CountChecklistItems` from `assemble` or `scaffold`, breaks the one
  decision point. The counter returns a count, not a `Violation`: each caller carries its
  own copy. (SCENARIO-02)
- `assemble.StartFS`'s `Brief.Shortfalls` order is fixed: acceptance (absent, or present
  with `strings.TrimSpace(step.Acceptance.Body) == ""`), then checklist (heading found, 0
  items), then state headings. `--json` `shortfalls[]` uses the same order. `Step`/`Section`
  gained no count field — that would change the wire contract pinned by
  `Test_start_json_success_document_golden_mem`. (SCENARIO-02)

## Left unbuilt

- `brief finish`'s refusal of an open step whose checklist heading is absent or holds 0
  items, and the reversed doc sentence on `internal/scaffold/finish.go`'s
  `checkStepChecklist` — SCENARIO-03. This flips `finish` from exit 0 to exit 1 on those
  steps, so find every existing `finish` pin the same way SCENARIO-02 found its one
  survivor: probe a `git archive` export, don't grep test bodies for new copy.
- `startLong`/`finishLong`/root `--help` copy, and the
  `docs/specifications/brief/specification.md` R11/new-step amendments (must describe the
  scaffold SCENARIO-01 produces, pinned byte-for-byte in `internal/cli/start_internal_test.go`)
  — SCENARIO-04.
- Skill and CLAUDE.md snippet changes (Rule 8) — SCENARIO-05/06.

## Traps

- The zero-item row's fix text says "brief finish refuses a step with none" — not true
  until SCENARIO-03 ships; it is ruled copy, not a bug. (SCENARIO-02)
- `assemble.Check` (and `--hook`) must not call `ChecklistItemCount` (Rule 6: check reports
  nothing new on a pre-existing tree). (SCENARIO-02)
- `markdown.Section` trims a whitespace-only section body to `""` (any
  `strings.TrimSpace(line) == ""` line counts as blank). So a `TrimSpace(x)==""` guard on an
  already-extracted `Section` body is provably identical to `x==""`; that mutation cannot
  distinguish them. Prove such a guard by adding/removing it, not by swapping the
  comparison. (SCENARIO-02)
- `fixtureConfig()` in `internal/scaffold/scaffold_test.go` is shared by ~12 test files;
  only `step_test.go`'s golden-byte assertion reads `cfg.AcceptanceHeading` today.
  SCENARIO-03 lands in the same package — check that assertion still matches before
  changing the skeleton further. (SCENARIO-01)

## Open debts

(none)
