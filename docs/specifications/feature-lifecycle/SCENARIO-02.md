---
id: SCENARIO-02
status: done
---

# SCENARIO-02: start names an empty acceptance section and a zero-item checklist

## Scenario

Scenario: SCENARIO-02 start names an empty acceptance section and a zero-item checklist
  Given an open step whose acceptance section is whitespace-only, or whose checklist heading holds zero items
  When I run "brief start <feature>" in text mode or with --json
  Then each is named as a shortfall row with the ruled copy, in acceptance-then-checklist order before state rows
  And the exit code is 0 and the brief still prints
  And an acceptance section with any text, or a checklist with at least one item, names nothing

User-visible contract (`brief start <feature>`, exit 0, brief on stdout in both modes). Headings
shown are the defaults; the configured headings are interpolated.

- Text-mode stderr, whitespace-only acceptance body:
  `brief start: <path>: "## Scenario" is empty; write the step's acceptance criteria under it`
- Text-mode stderr, checklist heading present with 0 items:
  `brief start: <path>: "## Implementation Plan" has no checklist items; add them as "- [ ]" lines before implementing, since brief finish refuses a step with none`
- Row order: the acceptance row (absent or empty, never both), then the checklist row, then the
  existing state-heading rows.
- `--json`: the same rows, in the same order, in `shortfalls[]` (`detail` = the text before the
  `;`, `fix` = the text after it); zero stderr bytes.
- Names nothing: acceptance body with any non-whitespace text (an HTML comment counts); checklist
  with at least one item, ticked or not.
- Unchanged: absent acceptance heading row; absent checklist heading refusal (exit 1).

## Implementation Plan

### Red

- [x] Step 1: `internal/platform/markdown/checklist_test.go` `Test_CountChecklistItems_*`: one table of counts. Cover no heading (not found), heading with no items (0, found), ticked and unticked items both counted, an indented (nested) item, an item inside a fence not counted, an item under a `###` subheading of the section counted, an item in the next same-level section not counted, and CRLF. Fails because the function does not exist yet.
- [x] Step 2: `internal/platform/conform/conform_test.go` `Test_ChecklistItemCount_*`. Cover an absent heading (not found), a present heading with zero items, and a fenced "- [ ]" line that is not counted. Fails because the function does not exist yet.
- [x] Step 3: `internal/assemble/assemble_test.go` `Test_start_reports_a_whitespace_only_acceptance_section_as_a_shortfall`. The fixture body must hold spaces and a tab, not just blank lines, because `markdown.Section` already trims blank lines. Assert the exact Detail and Fix built from `fixtureConfig()`'s custom `AcceptanceHeading`. Fails because no row is emitted.
- [x] Step 4: `internal/assemble/assemble_test.go` `Test_start_reports_a_checklist_with_no_items_as_a_shortfall`. Include a fenced "- [ ]" line in the section. Assert the exact Detail and Fix built from the custom `ChecklistHeading`. Fails because no row is emitted.
- [x] Step 5: `internal/assemble/assemble_test.go` `Test_start_names_nothing_for_an_acceptance_comment_or_a_single_checklist_item`. Table of three cases: acceptance holding only `<!-- ... -->`, one unticked item, one ticked item. Assert `Shortfalls` is empty. This is a control arm, so it is green on arrival; it must go red under the Verify mutations.
- [x] Step 6: `internal/assemble/assemble_test.go` `Test_start_orders_acceptance_then_checklist_then_state_shortfalls`. Fixture: empty acceptance, zero items, one state heading missing. Assert exactly three rows in that order. Fails because only the state row exists.
- [x] Step 7: `internal/cli/start_internal_test.go` `Test_start_names_an_empty_acceptance_and_an_empty_checklist_before_state_rows_mem`. Same three-kind fixture. Assert the exact stderr lines (ruled copy, default headings), exit 0, and that stdout still carries the brief. Fails because stderr holds only the state line.
- [x] Step 8: `internal/cli/start_internal_test.go` `Test_start_json_lists_the_empty_acceptance_and_empty_checklist_rows_in_order_mem`. Same fixture with `--json`. Assert zero stderr bytes and the `shortfalls[]` detail/fix values in order. Fails because only one row is present.
- [x] Step 9: `internal/cli/start_internal_test.go` `Test_start_on_a_freshly_scaffolded_step_names_the_empty_acceptance_and_the_empty_checklist_mem`. Build the step with `new feature` → `new step` → `start` on one shared `rwfs.Mem`, as the existing fresh-scaffold test does. Assert exactly two stderr lines: acceptance first, checklist second. Fails because stderr is empty.
- [x] Step 10: `internal/cli/start_internal_test.go`: update `Test_start_says_nothing_about_a_present_but_empty_convention_mem` and rename it `Test_start_says_nothing_about_a_present_but_empty_state_heading_mem`. Give the acceptance section text so the test keeps pinning only the rule that empty state headings are silent. Without that text it would fail once Green lands.
- [x] Step 11: find every other pin that breaks. Take a `git archive b80fa31` export under `$TMPDIR`. In its `StartFS`, append two dummy rows per open step: one when the acceptance body is whitespace-only, one when the checklist has no items. Run `go test ./...` there and read the failures. For each failing test the fixture was not about emptiness: give that fixture acceptance text or an item. Do not relax its stderr or `Shortfalls` assertion. Known survivor: `Test_start_on_a_freshly_scaffolded_step_reports_no_missing_acceptance_heading_shortfall_mem` only asserts NotContains, so it stays valid.

### Green

- [x] Step 12: `internal/platform/markdown/checklist.go`. Add `CountChecklistItems(body, heading string) (int, bool)`. Move `FirstUnchecked`'s section and fence scan into one unexported item scanner that both functions call, so items are recognized in one place.
- [x] Step 13: `internal/platform/conform/conform.go`. Add `ChecklistItemCount(body []byte, heading string) (int, bool)`: the one counting decision point for `start`'s zero-item row now and `finish`'s zero-item and absent-heading refusal in SCENARIO-03. It returns no Violation, because start and finish carry different copy.
- [x] Step 14: `internal/assemble/assemble.go` `(*Server).StartFS`. In the briefed-step block, beside the existing absent-acceptance row, emit the empty-acceptance row (body whitespace-only after `strings.TrimSpace`). Then emit the zero-item row via `conform.ChecklistItemCount` on the frontmatter-stripped body. Both come before the state loop, and all copy is built from `s.cfg` headings.

### Sweep

- [x] Step 15: `internal/assemble/doc.go`, `Shortfall` doc in `brief.go`, `Start`/`StartFS` doc: name empty acceptance and a zero-item checklist as shortfalls, and add `internal/platform/conform` to the import list. (sweep)
- [x] Step 16: `internal/platform/conform/doc.go`: drop the "four predicates" count and name the checklist item counter. `internal/platform/markdown/doc.go`: name `CountChecklistItems`. (sweep)
- [x] Step 17: `internal/cli/start_internal_test.go`: delete the forward-reference comment that names SCENARIO-02 above the fresh-scaffold test. (sweep)
- [x] Step 18: fix what `go build ./... && golangci-lint run ./...` reports. (sweep)

### Verify

- [x] Step 19: full verification per `.claude/rules/agent-briefs.md` (start `b80fa31`). Run these mutations one at a time:
  - (a) In the shared item scanner, remove the `fence.open` skip. The fenced case in `Test_CountChecklistItems_*` and `markdown`'s existing `FirstUnchecked` fence coverage must both go red. If only one goes red, recognition is not shared.
  - (b) In `StartFS`, replace `strings.TrimSpace(body) == ""` with `body == ""`. `Test_start_reports_a_whitespace_only_acceptance_section_as_a_shortfall` must go red.
  - (c) Swap the empty-acceptance and zero-item blocks. `Test_start_orders_acceptance_then_checklist_then_state_shortfalls` and the Step 7 text test must go red.
  - (d) Change the zero-item guard `== 0` to `<= 1`. The one-item arm of Step 5's table must go red.

## Handoff

**Binding decisions:**
- `conform.ChecklistItemCount(body []byte, heading string) (int, bool)` is the only item counter (Rule 4). SCENARIO-03's finish refusal calls it for both refusals: `found == false` means the heading is absent, and count 0 means no items. Finish takes the heading's line from `markdown.HeadingLine`. A second counter, or a direct call to `markdown.CountChecklistItems` from `assemble` or `scaffold`, breaks the one decision point.
- `markdown.CountChecklistItems` and `FirstUnchecked` share one unexported scanner. Item recognition, meaning fences, indentation, CRLF and `###` subsections inside the section, is defined only there.
- The counter returns counts, not a `Violation`. Start and finish carry different copy (see the spec's Surface & Copy). Each caller builds its own row or refusal.
- An empty acceptance section means the section body is empty after `strings.TrimSpace`. Any other text names nothing, an HTML comment included. This rule lives in `assemble` only. Finish never looks at acceptance (Rule 5).
- Row order in `Brief.Shortfalls`: acceptance (absent or empty), then checklist (zero items), then state headings. `--json` `shortfalls[]` uses the same order.
- `Step` and `Section` gain no count field. Adding one would change the `--json` wire contract pinned by `Test_start_json_success_document_golden_mem`.

**Left unbuilt:**
- `brief finish`'s refusal of an open step whose checklist heading is absent or holds 0 items, and the reversed doc sentence on `internal/scaffold/finish.go` `checkStepChecklist`: SCENARIO-03.
- The `startLong` sentence amendment in `internal/cli`: SCENARIO-04.

**Traps:**
- The zero-item row's fix text says "brief finish refuses a step with none". That is not true until SCENARIO-03 ships. Do not "fix" the copy; it is ruled.
- `assemble.Check` (and `--hook`) must not call `ChecklistItemCount` (Rule 6: check reports nothing new).
- `markdown.Section` already trims blank lines. A blank-lines-only acceptance fixture cannot tell `TrimSpace` apart from `== ""`, so the fixture needs spaces or a tab.
- `stepFromEntry` works on the frontmatter-stripped body (`e.rest`), but finish passes the whole file. Counts match either way. Line numbers do not, so SCENARIO-03 must take the heading line from the whole file.
- Many start tests assert empty stderr or no `Shortfalls` on fixtures with acceptance text and one item. Any fixture of that kind that loses its item now emits a row.
