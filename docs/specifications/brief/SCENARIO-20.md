# SCENARIO-20: A step with an open checklist item cannot be finished

## Scenario

```gherkin
Scenario: SCENARIO-20 A step with an open checklist item cannot be finished  [orig: 06d]
  Given a step with an open checklist item
  When I finish it
  Then the refusal names that item and its line
  And nothing is written
```

## Measured behaviour today (binary built from e507819, defaults, no `.brief.yaml`)

`brief new feature widgets` + `brief new step widgets` writes a step file whose last line is
the bare `## Implementation Plan` heading — **an empty checklist section**. `finish` never
reads `cfg.ChecklistHeading` at all (`grep -rn ChecklistHeading --include=*.go`: only
`internal/assemble` and `internal/scaffold/render.go`'s progress-list helpers).

Five shapes, each `brief finish widgets SCENARIO-01 --handoff <f> --state <f>`:

| checklist under `## Implementation Plan`                     | exit | stderr                             | files |
| ------------------------------------------------------------ | ---- | ---------------------------------- | ----- |
| (a) no items at all (as `new step` writes it)                 | 0    | `brief finish: SCENARIO-01 is done` | all four writes land |
| (b) `- [x] one` / `- [x] two`                                  | 0    | same                                | all four |
| (c) `- [x] one` / `- [ ] two`                                  | 0    | same                                | all four |
| (d) only a `- [ ]` inside a ``` fence                          | 0    | same                                | all four |
| (e) mix: indented `- [ ]`, `* [ ]`, `- [X]`, `- [ ]` in `## Notes` | 0    | same                            | all four |

So: **nothing about the step's checklist is enforced today**, in any shape. Case (c) is the
one this scenario turns into a refusal; (a), (b), (d) and (e) must keep succeeding.

## Decisions this plan takes

1. **Item grammar — `^\s*- \[[ xX]\]`, `-` bullet only.** Leading whitespace allowed (a
   hand-edited or nested item indented two or four spaces still counts); `- [ ]` is open;
   `- [x]` and `- [X]` are ticked. `* [ ]` / `+ [ ]` / `- []` / `- [  ]` are **prose, not
   items** — open question 6 settles the on-disk format as `- [ ]` / `- [x]`, and
   `scaffold/render.go`'s existing `checklistItemRe` already spells exactly that. Two
   disagreeing definitions of "checklist item" in one binary is the trap to avoid, so the new
   grammar is the same shape, widened only by `X`. **Do not point `render.go` at the new
   grammar**: `tickProgressEntry` does `strings.Replace(line, "[ ]", "[x]", 1)`, so a widened
   `checklistItemRe` matching `- [X]` would report found-and-ticked without ticking anything.
2. **Zero items is not a refusal**, and neither is an absent checklist heading. SCENARIO-13
   deliberately did not refuse a present-but-empty checklist on `start`, and `new step` writes
   exactly that (measured case (a)) — refusing here would break `new step` → `finish` on the
   first run. The trigger is *an unchecked item*, nothing else. A missing heading degrades
   (14's rule: `Section.Found == false` is a shortfall, not a refusal; `check` (22) owns
   reporting it).
3. **One scanner, in `internal/platform/markdown`.** `fenceState`, `findHeading` and
   `headingLevelOf` are unexported, so a scanner outside that package would be a second fence
   implementation. `markdown.Section` cannot be reused directly: it returns
   `trimBlankLines(...)` output and no offset, so a line number derived from it is wrong.
   **`markdown.Headings` is NOT needed** and stays unbuilt (20 does not own it after all):
   `findHeading` + `fenceState` + the existing same-or-higher-level heading stop is all the
   section boundary this needs.
4. **Placement: immediately after `checkArgumentHeadings`, before the specification read, and
   before `(refinish).verdict()`.** The band grows one entry: handoff cap → state cap → state
   fence → state headings → **open checklist item** → specification read → progress entry →
   state file → `verdict()`. Ordering rule to state: each check reads no more of the tree than
   the next (argument bytes, then the step body already in hand, then other files). Before
   `verdict()` follows 19's precedent recorded in STATE.md — a precondition narrows R11 rather
   than being skipped for a done step. 16's handoff-missing exemption does **not** transfer: it
   exempted a defect in `finish`'s own output, where a refusal is a dead end; an unticked box is
   the caller's own content and the refusal tells them exactly how to clear it.
5. **Refusal copy (R14a, write refusal → tail present), naming the step file and the item's
   line:** `brief finish: <abs step path>:<line>: checklist item "two" is not ticked; tick it
   with [x] once it is done, or remove it, and retry (no files changed)`. `RefusalError.Line`
   carries the item's **1-based line number in the whole step file** (the m20c shape: line 12,
   past frontmatter and title), rendered by the existing `cli/refusal.go` branch. Item text is
   right-trimmed so a CRLF step file carries no stray `\r` into the message; when the text is
   empty (`- [ ]` alone) the problem reads `checklist item is not ticked`, with no `%q`.
   Exit 1, stdout empty, one line on stderr. `internal/cli` needs **no** code change — the
   `Path` is a real file, not a `StateSource`/`HandoffSource` placeholder.
6. **Scope.** 21 (unfinished `depends-on`) and 22 (`check`) are out. When both 20 and 21 apply,
   **20 is reported first** — see Handoff; 21 must insert its check after this one.

## User-visible contract

- `brief finish <feature> <step> --handoff <path> --state <path>` — unchanged command line.
- Open item in the step's checklist → exit **1**, stdout empty, the single R14a stderr line above,
  every file in the feature directory byte-identical and mtime-unchanged.
- Empty checklist, absent checklist heading, all items ticked, `- [ ]` only inside a fence,
  `- [ ]` only in a later section → exit **0**, `brief finish: <step> is done` on stderr, four
  writes land (unchanged from today).
- Done step + open item + divergent handoff → the checklist refusal, exit 1, **not**
  `ErrAlreadyFinished`.

## Implementation Plan

- [x] Step 1: `internal/platform/markdown/checklist_test.go` `Test_finds_the_first_unchecked_item_with_its_line_number_in_the_whole_body` — the load-bearing pin: a body with frontmatter, a title and prose above the checklist, asserting the exact 1-based line and the item text (red)
- [x] Step 2a: `internal/platform/markdown/section.go` — extract `sectionSpan(lines []string, heading string) (headingIdx, sectionEnd int, ok bool)` out of `sectionRange`'s existing walk (the part before `lineOffsets`), and have `sectionRange` call it; byte offsets stay unexported, R21 intact (update)
- [x] Step 2b: `internal/platform/markdown/checklist.go` `FirstUnchecked(body, heading string) (line int, text string, found bool)` — scans `headingIdx+1..sectionEnd` from `sectionSpan`, returning `i+1`; mirrors `UnterminatedFence`'s `(int, string, bool)` shape. **Do not re-implement the same-or-higher-level stop** — a fourth boundary walk in one package is the "two scanners disagree" trap, and 22 would inherit it (green)
- [x] Step 3: `checklist_test.go` `Test_reports_no_unchecked_item_when_every_item_is_ticked` / `..._when_the_section_holds_no_items` / `..._when_the_heading_is_absent` — the three non-refusal pins (pins; they pass on arrival — earn them in Step 9 with the widening mutation)
- [x] Step 4: `checklist_test.go` `Test_ignores_an_unchecked_item_inside_a_fenced_block` — measured case (d) (red against a fence-blind scan)
- [x] Step 5: `checklist_test.go` `Test_stops_at_the_next_heading_of_the_same_level` — an open item under a following `## Notes` is not this section's (red)
- [x] Step 6: `checklist_test.go` `Test_counts_an_indented_item` (four-space indent counts) + `Test_does_not_treat_a_star_bullet_as_an_item` + `Test_does_not_treat_an_uppercase_X_item_as_unchecked` — grammar boundary, one variable per test; these three are only observable here, never through `Finish` (red/pin)
- [x] Step 7: `checklist_test.go` `Test_trims_the_carriage_return_from_an_item_in_a_CRLF_body` — the `cli/finish_test.go:526` shape (red)
- [x] Step 8: `internal/platform/markdown/doc.go` — add `FirstUnchecked` to the package doc, in the same "the one X" voice as `Section`/`CountLines`/`UnterminatedFence`; no scenario ids in comments (update)
- [x] Step 9: mutation pass on `FirstUnchecked`, one at a time, each stashed per the standing brief: drop the fence guard (Step 4 red) · stop at `len(lines)` instead of `sectionEnd` (Step 5 red) · count lines from the heading instead of from byte 0 (Step 1 red) · treat `[X]` as unchecked (Step 6 red) · return found for a section with zero items (**Step 3 red *and* Step 13 red once it exists** — report both, a run that only checks Step 3 under-reports)
- [x] Step 10: `internal/scaffold/errors.go` `ErrOpenChecklistItem` — new sentinel, doc'd like its neighbours (new)
- [x] Step 11: `internal/scaffold/finish_checklist_test.go` `Test_finish_refuses_a_step_with_an_open_checklist_item` — Server-level: asserts the whole `*RefusalError` (`Path` = absolute step path, `Line` = the item's line, `Problem` naming the item, `errors.Is` the new sentinel) plus **both** nothing-landed probes — `snapshotTree` byte-identity and the `pinModTimes`/`modTimes` probe (red)
- [x] Step 12: `internal/scaffold/finish.go` `checkStepChecklist` + its call site immediately after `checkArgumentHeadings` — takes `stepBody` (the **whole file**, as read at `finish.go:146`), `stepPath` and `cfg.ChecklistHeading`; empty item text drops the `%q` (green)
- [x] Step 13: `finish_checklist_test.go` `Test_finish_accepts_a_step_whose_checklist_is_empty` — the `new step` → `finish` path of measured case (a); assert exit-equivalent success *and* that the four writes landed, so it cannot pass vacuously (pin)
- [x] Step 14: `finish_checklist_test.go` `Test_finish_accepts_a_step_with_no_checklist_heading` + `Test_finish_ignores_an_unchecked_item_outside_the_checklist_section` — 14's degrade rule and the section boundary, at the `Finish` level (pin)
- [x] Step 15: `finish_checklist_test.go` `Test_finish_reports_the_open_checklist_item_rather_than_a_missing_specification` — fixture with an open item *and* a deleted `specification.md`; pins the check ahead of the spec read (red)
- [x] Step 16: `finish_checklist_test.go` `Test_finish_reports_a_state_body_missing_a_heading_rather_than_the_open_checklist_item` — pins the check *after* the argument band (red)
- [x] Step 17: `finish_checklist_test.go` `Test_finish_reports_the_open_checklist_item_on_a_done_step_with_a_divergent_handoff` — done step + open item + handoff differing from the recorded one reports `ErrOpenChecklistItem`, never `ErrAlreadyFinished`; pins the check ahead of `verdict()` (red)
- [x] Step 18: mutation pass on the three ordering facts, **individually** (a one-sided ordering test also passes with the *other* check deleted — STATE.md's standing trap): move the call above `checkArgumentHeadings` (Step 16 red) · move it below the spec read (Step 15 red) · move it below the `verdict()` switch (Step 17 red)
- [x] Step 19: `internal/cli/finish_test.go` `Test_finish_refuses_a_step_with_an_open_checklist_item` — command slice through `cli.Run`: exit code 1, stdout byte-empty, the **whole** stderr line asserted with `Equal` (absolute path, `:<line>`, the `(no files changed)` tail), no cli code change expected (red)
- [x] Step 20: fixture enumeration — run `go test ./...` unpiped from the repo root and fix **every** fixture that newly refuses, not a list read off one file (STATE.md: a plan's fixture list under-counts). At least one exists: `internal/scaffold/finish_test.go` ~line 416, the `STEP-10` body carrying `- [ ] pending`, finished successfully by the id-boundary test. A clean first run means you did not look. Report the test-count delta (update)
- [x] Step 21: `internal/scaffold/finish.go` `Finish` doc comment + `internal/scaffold/doc.go` — extend the check-order narrative with the checklist check and its position, and add `ErrOpenChecklistItem` to the package doc's sentinel list; state the rule, never the scenario id (update)
- [x] Step 22: verification per the standing brief — the touched packages for `-race` are `./internal/platform/markdown/...`, `./internal/scaffold/...` and `./internal/cli/...`; then rewrite `docs/specifications/brief/STATE.md` and tick SCENARIO-20 in `specification.md`

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- Checklist item grammar is `^\s*- \[[ xX]\]`, hyphen bullet only, any leading whitespace, `x`/`X` ticked — `markdown.FirstUnchecked` is the sole reader of it; `check` (22) must call it rather than re-scan.
- `render.go`'s `checklistItemRe` stays `[ x]` and stays separate, deliberately — widening it to `X` makes `tickProgressEntry`'s `strings.Replace(line, "[ ]", "[x]", 1)` report a tick it did not perform. Two grammars, one reason.
- A checklist with **zero** items, or no checklist heading at all, is never a refusal — `new step` writes a bare heading, so refusing breaks `new step` → `finish` on the first step (measured: exit 0 today). Only an unchecked item refuses. 13's and 14's read-side rules agree.
- The checklist check sits after `checkArgumentHeadings`, before the specification read, and **before `(refinish).verdict()`** — ordered by how much of the tree each check reads. This narrows R11 further, the same way 19 did: a re-finish of a done step is a no-op only if its checklist is also complete. 16's missing-handoff exemption does not extend here.
- **21's `depends-on` refusal goes after this check**, so a step that is both un-ticked and blocked reports the checklist item first: it is in the file the caller is finishing and costs no extra I/O. 21 owns the test that pins its own position.
- `RefusalError.Line` is the item's 1-based line in the **whole step file**, not within the section. `cli` is untouched: the `Path` is a real file path, so no placeholder branch is needed.

**Left unbuilt** — named so nobody assumes it exists:

- `markdown.Headings` — still unbuilt; 20 did not need it (`findHeading` + `fenceState` sufficed). Owner: 22.
- `markdown.ChecklistItems` / any all-items listing, and any per-item finding output — owner: `check` (22), which may widen `FirstUnchecked` into a list.
- Truncation of a very long item's text in the refusal line — belongs to R13's unowned output budget, still unconsumed.
- Nothing reports a `* [ ]` or `- []` line as an open item; a hand-written step using those bullets finishes silently. Owner: `check` (22) as a finding, or nobody.

**Traps** — things that look right and are not:

- `markdown.Section` returns `trimBlankLines(...)` output with no offset — computing the line number from it, or from the heading index, silently yields a plausible small number. The Step 1 fixture pushes the item to line 12 for exactly this reason.
- The other plausible-small-number cause is the call site: `finish.go:151` is `fm, _, err := stepfile.ParseFrontmatter(stepBody)`, and feeding `checkStepChecklist` that discarded remainder instead of `stepBody` yields a line number short by exactly the frontmatter length. The correct input is `stepBody`, the whole file.
- `markdown.Section(body, "")` returns `found == true` (`findHeading` matches the first blank line), so an empty configured `checklist-heading` would make "the checklist section" everything after the first blank line. Pre-existing hole under the heading-value-validation debt; not fixed here.
- `internal/scaffold/finish_test.go`'s `STEP-10` fixture (`- [ ] pending`) is finished successfully today. It fails with a *wrong sentinel*, not a build break — the dangerous kind. Enumerate fixtures by running the suite, never by reading one file.
- A `- [X]`, `* [ ]` or indented-item decision is **unobservable through `Finish`**: "ticked" and "not an item" both mean "proceed". Only the platform-level test discriminates them; a Server-level test claiming to pin the grammar is vacuous.
- An ordering test is vacuous without the reverse mutation, and the three ordering facts must be mutated one at a time — disabling two at once proves neither.
