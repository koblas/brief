---
id: SCENARIO-03
status: done
---

# SCENARIO-03: finish refuses an unplanned open step

## Scenario

```gherkin
Scenario: SCENARIO-03 finish refuses an unplanned open step
  Given an open step whose checklist heading is absent, or present with zero items
  When I run "brief finish <feature> <step> --handoff <path> --state <path>"
  Then it refuses with the ruled line, exit 1, and changes no files
  And a done step's identical re-finish still succeeds without writing
  And "brief check", including "--hook claude-code", reports no row for such a step
  And start's zero-item hint and finish's refusal count items with the same function
```

User-visible contract (the spec's `## Surface & Copy`, verbatim). `<heading>` is
`%q` of `cfg.ChecklistHeading`, which is `"## Implementation Plan"` by default:

- Heading absent: stderr `brief finish: <path>: no <heading> heading found; add it with the step's checklist items, tick them, and retry (no files changed)`, exit 1, stdout empty. `--json` gives the common refusal document with `error.kind: "refusal"`, `line: null`, and nothing on stderr.
- Heading present, 0 items: stderr `brief finish: <path>:<heading line>: <heading> has 0 checklist items, needs at least 1; add the step's items as "- [x]" lines once done, and retry (no files changed)`, exit 1. `--json` sets `line` to the heading's 1-based line in the whole file.
- Done step with identical inputs: idempotent success, no write. Done step with divergent inputs: the existing `ErrAlreadyFinished` refusal. An empty or absent acceptance section never refuses.
- `internal/cli` needs no change. `classifyRefusal` renders every `*scaffold.RefusalError` generically, adding the `(no files changed)` tail, and `jsonLine` maps `Line == 0` to `null`.

## Implementation Plan

### Red

- [x] Step 1: `internal/scaffold/finish_checklist_test.go` `Test_finish_refuses_an_open_step_with_no_checklist_heading`. Replaces `Test_finish_accepts_a_step_with_no_checklist_heading` and keeps its fixture (the heading renamed away). Asserts `ErrorIs` the new sentinel, `refusal.Line == 0`, the exact `Error()` string, and an unchanged `fx.mem.Snapshot()`. Fails because finish succeeds today.
- [x] Step 2: `internal/scaffold/finish_checklist_test.go` `Test_finish_refuses_an_open_step_whose_checklist_holds_no_items`. Table test on a STEP-02 body with frontmatter and the heading at line 10. Rows:
  - empty section
  - a section whose only `- [x]` lines sit inside a fenced block (the counter must not see them)
  - the same body with CRLF line endings

  Every row asserts the new sentinel, `Line == 10`, the exact `Error()` string and an unchanged snapshot. Keep the frontmatter, so the whole-file line (10) differs from the frontmatter-stripped line. Fails because finish succeeds today.
- [x] Step 3: `internal/scaffold/finish_checklist_test.go` `Test_finish_refuses_a_freshly_scaffolded_step`. Replaces the on-disk `Test_finish_accepts_a_step_whose_checklist_is_empty`, now on `rwfs.Mem` via `createFeatureFS` / `newStepFS`, and runs NewFeature → NewStep → FinishFS. Asserts the zero-item refusal at the scaffolded heading's line (a literal counted from the `step_test.go` golden) and an unchanged snapshot. Fails because finish succeeds today.
- [x] Step 4: `internal/scaffold/finish_checklist_test.go` `Test_finish_reports_a_state_body_missing_a_heading_rather_than_the_empty_checklist`. Mirrors the existing open-item ordering test: argument refusals still come first. Green on arrival: it guards the slot, and goes red if the new check is placed ahead of the argument checks.
- [x] Step 5: `internal/scaffold/finish_idempotent_test.go` `Test_re_finishing_a_done_step_with_no_checklist_items_and_the_same_inputs_is_a_noop`. Table over {no heading, zero items}. Built from `newFinishedFixtureFS` plus `putStepFS` of a done body, with the snapshot taken after the put. Asserts NoError and an unchanged snapshot. Green on arrival: it guards the done-status gate.
- [x] Step 6: `internal/scaffold/finish_idempotent_test.go` `Test_re_finishing_a_done_step_with_no_checklist_items_and_a_different_handoff_is_refused_as_already_finished`. Same table. Asserts `ErrorIs(ErrAlreadyFinished)` and `NotErrorIs` the new sentinel. Green on arrival: it guards the done-status gate.
- [x] Step 7: `internal/scaffold/finish_checklist_test.go` `Test_finish_accepts_an_open_step_whose_acceptance_section_is_empty_or_absent`. Table over {no acceptance heading, whitespace-only acceptance section}, each with one ticked item. Asserts NoError and `status: done` written. Green on arrival: it pins Rule 5 against a check that grows to cover acceptance.
- [x] Step 8: update the two existing breakages. Both call Finish on a freshly scaffolded step and must get one ticked item under the checklist heading before finishing. Do not loosen them.
  - `internal/scaffold/finish_headings_test.go` `Test_accepts_the_state_body_new_feature_writes` (disk): add the item.
  - `internal/scaffold/step_test.go` `Test_the_handoff_probe_sees_a_handoff_file_after_a_finish`: add the item in the setup it shares with `Test_writes_no_handoff_file` (or in both tests), so the control arm still differs from its claim in one variable only.
- [x] Step 9: `internal/cli/finish_internal_test.go` `Test_finish_refuses_an_unplanned_open_step_mem`. Table test through `run`:
  - no-heading row: the step body's heading is replaced, and the want line has no `:line`
  - zero-item row: `newMemFinishFixture("")`, with the heading at line 13

  Asserts `ExitCode == 1`, empty stdout, the exact stderr line with the `brief finish: ` prefix and the `(no files changed)` tail, and step-file bytes unchanged on read-back. Fails because finish exits 0 today.
- [x] Step 10: `internal/cli/finish_internal_test.go` `Test_finish_json_refuses_an_unplanned_open_step_mem`. Same two rows with `--json`. Asserts:
  - `error.kind == "refusal"`
  - `exit_code == 1`
  - `ok == false`
  - `line` is `null` for the no-heading row and `13` for the zero-item row
  - stderr is empty

  Fails because finish succeeds today.
- [x] Step 11: `internal/cli/check_internal_test.go` `Test_check_reports_no_row_for_a_step_with_no_checklist_items_mem`. Table over {open, done} × {no heading, zero items}, in an otherwise conforming feature (`memConformingSpec`, `memConformingState`). Asserts exit 0, empty stdout and the one-line no-findings stderr. Green on arrival: this is the leak detector for Rule 6.
- [x] Step 12: `internal/cli/check_hook_disk_test.go` `Test_check_hook_counts_no_error_for_an_unplanned_step`. Reuses the fixture from `Test_check_hook_reports_additional_context_when_the_same_feature_has_an_open_step` (over-cap STATE.md plus an open step with an unticked item, which yields exactly 1 ERROR). Adds unplanned open and done steps (no heading, zero items). Asserts that `hookAdditionalContext` equals the exact `1 ERROR finding` line. Green on arrival: this is a contract pin.

### Green

- [x] Step 13: `internal/scaffold/errors.go`: add the exported sentinel `ErrUnplannedStep`. One sentinel covers both refusals, with a one-line doc.
- [x] Step 14: `internal/scaffold/finish.go`: add the unexported `checkStepPlanned(stepBody, stepPath, heading)`, which returns `*RefusalError`.
  - It counts with `conform.ChecklistItemCount` and takes the heading's line from `markdown.HeadingLine` against the whole `stepBody`.
  - It carries the ruled Problem/Fix copy for the two cases (absent: `Line` 0; zero: the heading line).
  - It never calls `markdown.CountChecklistItems` directly.
- [x] Step 15: `internal/scaffold/finish.go` `FinishFS`: call `checkStepPlanned` only when `!fm.Done()`, right beside `checkStepChecklist`. That keeps it after the argument checks and before `checkStepDependencies`.

### Sweep

- [x] Step 16: `internal/scaffold/finish.go` `checkStepChecklist` doc: remove "A checklist with no items, or an absent heading, is never refused." In its place, say that an open step with no checklist items or no heading is refused by `checkStepPlanned` (`ErrUnplannedStep`) (sweep)
- [x] Step 17: `internal/scaffold/errors.go` `ErrOpenChecklistItem` doc: replace the "is never refused this way" clause with a pointer to `ErrUnplannedStep` for an open step. Leave `conform.OpenChecklistItem`'s "never a violation" doc untouched, because `assemble.Check` depends on that contract (sweep)
- [x] Step 18: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

### Verify

- [x] Step 19: full verification per `.claude/rules/agent-briefs.md` from start commit `c27c97e`. Expected `test-stats` deltas:
  - `internal/scaffold`: +5 top-level and −1 disk test (the empty-checklist end-to-end test moves to mem)
  - `internal/cli`: +3 mem and +1 disk (hook)

  Mutate each of these individually and record which test goes red:
  - (a) Drop the `!fm.Done()` gate in `FinishFS`: Steps 5 and 6 go red.
  - (b) Drop the heading-absent half of `checkStepPlanned`'s condition: Steps 1 and 9 (no-heading row) go red.
  - (c) Drop the zero-count half: Steps 2, 3 and 9 (zero-item row) go red.
  - (d) Feed the frontmatter-stripped body to `markdown.HeadingLine`: Step 2 goes red (the line is not 10).
  - (e) Temporarily make `conform.OpenChecklistItem` return a Violation when `ChecklistItemCount` reports absent or 0: the **done** rows of Step 11 go red. The open rows cannot see this mutation, because check skips open steps.

## Handoff

**Binding decisions** (a later scenario must not contradict these without saying so):
- `scaffold.ErrUnplannedStep` is the one sentinel for both new refusals, declared in `internal/scaffold/errors.go`. The spec never ruled on it (new exported Go API), so the final product-vision pass may rename it. It wraps a `*RefusalError`, and CLI rendering is generic, so there is no cli change.
- `checkStepPlanned` runs only for an open step (`!fm.Done()`), in `checkStepChecklist`'s slot: after the handoff/state argument checks, before `checkStepDependencies`. The resulting asymmetry is deliberate:
  - The unticked-item check (`checkStepChecklist`) still fires on done steps, and `Test_finish_reports_the_open_checklist_item_on_a_done_step_with_a_divergent_handoff` pins that.
  - The emptiness check does not fire on done steps, because Rule 3 requires done-step re-finish to stay idempotent or keep refusing divergence.
- Finish counts through `conform.ChecklistItemCount`, never `markdown.CountChecklistItems` (Rule 4, one decision point shared with `assemble.StartFS`). It takes the heading line from `markdown.HeadingLine` on the whole file. Both go through `findHeading` (fence-aware and CRLF-trimmed), so a "found" count always has a line.
- Refusal copy lives in `scaffold`, not `conform`: the counter returns a count and each caller attaches its own copy. The new predicate must not go into `conform.OpenChecklistItem`, or `assemble.Check` inherits it (Rule 6).

**Left unbuilt** (named so nobody assumes it exists):
- The `finishLong` help sentence about the new refusal, and the R11 / `new step` amendment in `docs/specifications/brief/specification.md`: both SCENARIO-04.
- No `Finish` doc-comment clause was added. The rule is documented once, on `checkStepPlanned` and `ErrUnplannedStep`.

**Traps** (things that look right and are not):
- The hook's observable is only the ERROR count. A leaked row would be WARN (`RuleChecklist`) and would not move it, so Step 12 is a contract pin, not a leak detector. Step 11's done rows are the leak detector (mutation e).
- `ParseFrontmatter`'s second return is the stripped body. Feeding it to `HeadingLine` gives a wrong line that still "looks" plausible, so Step 2's fixture must keep frontmatter to catch it.
- Only four existing tests break, all in `internal/scaffold`, found by probing a `git archive` export of `c27c97e` with a crude open-only refusal:
  - `Test_finish_accepts_a_step_whose_checklist_is_empty`
  - `Test_finish_accepts_a_step_with_no_checklist_heading`
  - `Test_accepts_the_state_body_new_feature_writes`
  - `Test_the_handoff_probe_sees_a_handoff_file_after_a_finish`

  No cli or cmd test breaks, because cli fixtures carry items. After Step 8, any scaffold test that NewStep's and then Finishes needs a ticked item.
- STATE.md's SCENARIO-02 trap is now true: start's zero-item fix text, "brief finish refuses a step with none", describes shipped behaviour.
