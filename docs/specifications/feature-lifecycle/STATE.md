# feature-lifecycle — current state

Scenarios complete: SCENARIO-01, SCENARIO-02. Last updated by SCENARIO-02.

## Binding decisions

- `stepSkeleton` (`internal/scaffold/render.go`) emits `cfg.AcceptanceHeading`, a blank
  line, then `cfg.ChecklistHeading` — both already-configurable fields. Do not reintroduce
  an acceptance-less skeleton. (SCENARIO-01)
- `conform.ChecklistItemCount(body []byte, heading string) (int, bool)` is the only item
  counter (Rule 4): `markdown.CountChecklistItems` and `FirstUnchecked` share one unexported
  scanner in `internal/platform/markdown/checklist.go`, so fence/indent/CRLF/`###`-subsection
  recognition is defined exactly once. `assemble.StartFS` calls `ChecklistItemCount` on the
  frontmatter-stripped body (`e.rest`) for the zero-item shortfall row; SCENARIO-03's finish
  refusal must call the same function for both its refusals (absent heading and zero items),
  taking the heading's line from `markdown.HeadingLine` against the *whole* file, not `e.rest`
  (line numbers differ; counts don't). A second counter, or a direct call to
  `markdown.CountChecklistItems` from `assemble` or `scaffold`, breaks the one decision point.
  (SCENARIO-02)
- The counter returns a count, not a `Violation`: start and finish carry different copy, so
  each caller builds its own row/refusal from the same number. (SCENARIO-02)
- `assemble.StartFS`'s `Brief.Shortfalls` order is fixed: acceptance (absent, or present with
  `strings.TrimSpace(step.Acceptance.Body) == ""`), then checklist (heading found, 0 items via
  `ChecklistItemCount`), then state headings. `--json` `shortfalls[]` uses the same order.
  `Step`/`Section` gained no count field — that would change the `--json` wire contract pinned
  by `Test_start_json_success_document_golden_mem`. (SCENARIO-02)

## Left unbuilt

- `brief finish`'s refusal of an open step whose checklist heading is absent or holds 0 items,
  and the reversed doc sentence on `internal/scaffold/finish.go`'s `checkStepChecklist` — owned
  by SCENARIO-03.
- The `startLong`/`finishLong`/root `--help` copy amendments, and the
  `docs/specifications/brief/specification.md` R11/new-step amendments — SCENARIO-04.
- Skill and CLAUDE.md snippet changes (Rule 8) — SCENARIO-05/06.

## Traps

- The zero-item row's fix text says "brief finish refuses a step with none". Not true until
  SCENARIO-03 ships — do not "fix" the copy; it is ruled by the spec's Surface & Copy.
- `assemble.Check` (and `--hook`) must not call `ChecklistItemCount` (Rule 6: check reports
  nothing new on a pre-existing tree).
- `markdown.Section` trims a whitespace-only section body to `""` (`trimBlankLines` treats any
  `strings.TrimSpace(line) == ""` line as blank, spaces/tabs included, not just zero-length
  lines). So `step.Acceptance.Body` is already `""` for a whitespace-only section by the time
  `StartFS` sees it: an "is it empty" guard there is proven by adding/removing the check
  entirely (Step 3's test goes red without it), not by swapping `TrimSpace(x)==""` for
  `x==""` — those two are provably identical once `Body` has already been trimmed. Verified by
  running exactly that mutation: it stayed green, as expected once you trace `Section`'s
  trimming through.
- A checklist-shortfall test whose *every* fixture case happens to carry exactly one item (a
  table meant to also cover "acceptance holds a comment") reddens entirely through the
  checklist path under an `== 0` → `<= 1` mutation, not distinctly per case. If a future test
  needs to isolate the acceptance-only arm, give it a checklist with 2+ items so the checklist
  guard can't also fire.
- Many `start` tests assert empty stderr or empty `Shortfalls` on fixtures with acceptance
  text and exactly one checklist item. A fixture of that kind that loses its item, or has its
  acceptance body cleared, now emits a shortfall row — checked by git-archive probe against
  `b80fa31`; only one pre-existing test needed updating
  (`Test_start_says_nothing_about_a_present_but_empty_state_heading_mem`, formerly
  `..._convention_mem`).

## Open debts

(none)
