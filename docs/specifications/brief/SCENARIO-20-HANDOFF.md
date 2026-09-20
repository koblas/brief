# SCENARIO-20 Handoff: A step with an open checklist item cannot be finished

## What landed

`internal/platform/markdown` gained `FirstUnchecked(body, heading string)
(line int, text string, found bool)`, the checklist-item counterpart of
`UnterminatedFence`: it scans the section under `heading` (fence-aware,
same-or-higher-level stop, sourced from a new unexported `sectionSpan`
extracted out of `sectionRange`'s existing walk) for the first item
matching `^\s*- \[([ xX])\]` that is not ticked, returning its 1-based line
number in the **whole** body and its right-trimmed text. `found` is
`false` alike for an absent heading, an empty section, and a fully-ticked
section — none of those three trigger a refusal, so the caller never needs
to tell them apart.

`(*scaffold.Server).Finish` calls a new `checkStepChecklist(stepBody,
stepPath, cfg.ChecklistHeading)`, placed immediately after
`checkArgumentHeadings` and before the specification read — after the
argument band (so a divergent state heading is reported first), ahead of
the spec read and `(refinish).verdict()` (so a done step with both an open
item and a divergent handoff reports the open item, never
`ErrAlreadyFinished`). `stepBody` is the whole file as read at
`finish.go:146`, not `ParseFrontmatter`'s discarded remainder — feeding the
remainder would under-count the line number by exactly the frontmatter's
length.

New sentinel `ErrOpenChecklistItem` in `internal/scaffold/errors.go`, added
to `doc.go`'s sentinel enumeration. No `internal/cli` change: a checklist
refusal's `RefusalError.Path` is always a real step-file path, never a
`StateSource`/`HandoffSource` placeholder, so the existing placeholder-swap
`switch` in `cli/finish.go` needs nothing new.

`scaffold/render.go`'s own `checklistItemRe` (`[ x]`, used by
`tickProgressEntry`/`insertProgressEntry` against the progress list) is
**not** widened to accept `X` — that regexp's `tickProgressEntry` flips a
literal `"[ ]"` to `"[x]"` by string replacement, so matching `"[X]"` there
would report a tick it never performed. The two regexes stay deliberately
separate; `markdown.checklistItemRe` is the only one that accepts `X`.

## Refusal copy

```
brief finish: <abs step path>:<line>: checklist item "two" is not ticked; tick it with [x] once it is done, or remove it, and retry (no files changed)
```

When the item's text is empty (`- [ ]` alone), the problem drops the `%q`:
`checklist item is not ticked`. `Line` is the item's 1-based line in the
whole step file, not within the checklist section.

## Tests added (20: 11 markdown + 8 scaffold + 1 cli)

`internal/platform/markdown/checklist_test.go` (new file):

- `Test_finds_the_first_unchecked_item_with_its_line_number_in_the_whole_body`
  — the load-bearing pin: frontmatter + title + prose above the checklist,
  landing the item at line 12.
- `Test_reports_no_unchecked_item_when_every_item_is_ticked` /
  `..._when_the_section_holds_no_items` / `..._when_the_heading_is_absent`
  — the three non-refusal shapes.
- `Test_ignores_an_unchecked_item_inside_a_fenced_block`.
- `Test_stops_at_the_next_heading_of_the_same_level`.
- `Test_counts_an_indented_item`, `Test_does_not_treat_a_star_bullet_as_an_item`,
  `Test_does_not_treat_an_uppercase_X_item_as_unchecked` — grammar
  boundary, one variable each; only observable at this layer, never
  through `Finish`.
- `Test_trims_the_carriage_return_from_an_item_in_a_CRLF_body`.
- `Test_returns_empty_text_for_a_bare_unchecked_item` — a bare `- [ ]` with
  no text, `found == true` and `text == ""` (the empty-text branch
  `checkStepChecklist` reads).

`internal/scaffold/finish_checklist_test.go` (new file):

- `Test_finish_refuses_a_step_with_an_open_checklist_item` — the whole
  `*RefusalError` (path, line 13, problem naming `"second thing"`),
  `errors.Is(ErrOpenChecklistItem)`, plus both nothing-landed probes
  (`snapshotTree` byte-identity and `pinModTimes`/`modTimes`).
- `Test_finish_accepts_a_step_whose_checklist_is_empty` — the `new step` →
  `finish` path (measured case (a)); asserts the four writes actually
  landed (status: done, spec tick, state bytes, handoff file), not merely
  a nil error.
- `Test_finish_accepts_a_step_with_no_checklist_heading`.
- `Test_finish_ignores_an_unchecked_item_outside_the_checklist_section`.
- `Test_finish_reports_the_open_checklist_item_rather_than_a_missing_specification`
  — pins the check ahead of the spec read.
- `Test_finish_reports_a_state_body_missing_a_heading_rather_than_the_open_checklist_item`
  — pins the check after the argument band.
- `Test_finish_reports_the_open_checklist_item_on_a_done_step_with_a_divergent_handoff`
  — pins the check ahead of `(refinish).verdict()`.
- `Test_finish_refuses_a_bare_open_checklist_item_without_a_quoted_empty_string`
  — asserts the whole `err.Error()` for a bare `- [ ]` item, no `%q` in the
  copy; this is the test that actually exercises `checkStepChecklist`'s
  empty-text branch (every other fixture's item carries text).

`internal/cli/finish_test.go`:

- `Test_finish_refuses_a_step_with_an_open_checklist_item` — through
  `cli.Run`, `Equal` on the whole stderr line (absolute path, `:15`, the
  `(no files changed)` tail), exit 1, byte-empty stdout.

## Fixture enumeration — zero broke

The architect flagged `internal/scaffold/finish_test.go`'s `STEP-10`
fixture (`- [ ] pending`, ~line 416) as a likely wrong-sentinel hazard.
Running the full suite after wiring the check found it does **not**
break: that test (`Test_does_not_treat_an_entry_whose_id_merely_starts_with_the_finished_id`)
finishes `STEP-1`, whose own checklist is fully ticked — `STEP-10`'s open
item belongs to a *different* step file that call never touches. Every
other pre-existing fixture in `scaffold`, `assemble` and `cli` already
carries a fully-ticked (or heading-absent) checklist on the step each test
finishes. `go test ./...` after the full implementation (including the
advisor-flagged empty-text tests below) is exit 0 with a test-count delta
of exactly +20 — matching the 20 new tests one for one, confirming no
pre-existing fixture newly refuses and none needed updating.

## Mutation verification (Step 9, `FirstUnchecked`; Step 18, ordering),
each stashed/copied and restored byte-identical, diffed to confirm

- Drop the fence guard → red: `Test_ignores_an_unchecked_item_inside_a_fenced_block`.
- Stop at `len(lines)` instead of `sectionEnd` → red:
  `Test_stops_at_the_next_heading_of_the_same_level`.
- Count lines from the heading instead of from byte 0 (`i - headingIdx`)
  → red: `Test_finds_the_first_unchecked_item_with_its_line_number_in_the_whole_body`.
- Treat `[X]` as unchecked → red: `Test_does_not_treat_an_uppercase_X_item_as_unchecked`.
- Return `found` for a section with zero items (`sectionEnd >
  headingIdx+1`) → red on **both** layers at once:
  `Test_reports_no_unchecked_item_when_the_section_holds_no_items`
  (markdown) and `Test_finish_accepts_a_step_whose_checklist_is_empty`
  (scaffold) — a run checking only the markdown test would under-report.
- Move `checkStepChecklist` above the argument band → red:
  `Test_finish_reports_a_state_body_missing_a_heading_rather_than_the_open_checklist_item`.
- Move it below the specification read → red:
  `Test_finish_reports_the_open_checklist_item_rather_than_a_missing_specification`.
- Move it below the `verdict()` switch → red:
  `Test_finish_reports_the_open_checklist_item_on_a_done_step_with_a_divergent_handoff`.
- Flip `checkStepChecklist`'s empty-text guard (`text != ""` →
  `text == ""`) → red:
  `Test_finish_refuses_a_bare_open_checklist_item_without_a_quoted_empty_string`
  only — the branch this advisor-flagged gap otherwise left uncovered.

Each mutation run individually and restored (byte-identity confirmed by
diff against a pre-mutation copy) before the next.

## Green on arrival

Steps 3, 6, 13, 14, 15, 16, 17 and 19 passed the moment their production
code landed, with no red step of their own — expected, not a shortcut:

- Step 3's three non-refusal pins and Step 6's three grammar-boundary pins
  were satisfied by `FirstUnchecked`'s first correct implementation
  (Step 2b); they are earned retroactively by Step 9's mutation pass, not
  by a prior red.
- Steps 13 and 14 (empty checklist, no heading, item outside the section)
  were satisfied the moment `checkStepChecklist` existed, since decision 2
  (zero items / absent heading never refuses) was built into
  `FirstUnchecked` from the start, not bolted on after.
- Steps 15, 16 and 17 (the three ordering facts) were satisfied the moment
  `checkStepChecklist`'s call site landed at the position the plan
  specified — that is exactly why Step 18's mutation pass exists: to prove
  each ordering fact by moving the call and watching the matching test
  redden, since the tests alone, passing on arrival, prove nothing about
  position.
- Step 19 (the cli slice) was satisfied on arrival because no cli code
  change was needed — `RefusalError.Path` is already a real file path for
  this refusal, never a placeholder.

The advisor-flagged gap (the empty-item-text branch) is the one place a
new test was needed *and* still passed on arrival — `FirstUnchecked`
already handled it correctly; only `checkStepChecklist`'s branch lacked a
test discriminating it from the `%q` branch, closed by the mutation above.

## Verification

- `go build ./...` — clean.
- `go vet ./...` — clean.
- `go test ./...` — exit 0, unpiped. 400 `--- PASS` lines (`grep -c --
  "--- PASS"`, unanchored) vs 380 before this scenario — delta +20,
  matching the 20 new tests exactly (11 markdown + 8 scaffold + 1 cli). 0
  `--- SKIP`, 0 `--- FAIL`.
- `go test -race ./internal/platform/markdown/... ./internal/scaffold/...
  ./internal/cli/...` — clean.
- `golangci-lint run ./...` — 0 issues (one `perfsprint` finding on a test
  file's `fmt.Sprintf` and one `testifylint` finding on `assert.Equal(t,
  "", text)`, both fixed before this run — string concatenation and
  `assert.Empty` respectively).
- `go doc ./internal/platform/markdown FirstUnchecked` and `go doc
  ./internal/scaffold ErrOpenChecklistItem` — read correctly.

## Binding decisions

- Item grammar `^\s*- \[[ xX]\]`, hyphen bullet only, any leading
  whitespace, `x`/`X` ticked — `markdown.FirstUnchecked` is the sole
  reader; `check` (22) must call it rather than re-scan.
- `render.go`'s `checklistItemRe` stays `[ x]`, deliberately separate —
  widening it would make `tickProgressEntry` misreport a tick.
- Zero items, or no checklist heading at all, is never a refusal — only an
  unchecked item triggers. `new step` → `finish` on the first step keeps
  succeeding.
- Placement: after `checkArgumentHeadings`, before the specification read,
  before `(refinish).verdict()`. Narrows R11 further: a re-finish of a
  done step is a no-op only when its checklist is also complete.
- `RefusalError.Line` is the item's 1-based line in the whole step file.
  `cli` is untouched.
- **21's `depends-on` refusal goes after this check** — a step both
  un-ticked and blocked reports the checklist item first, in the file
  already open, at no extra I/O cost. 21 owns the test pinning its own
  position relative to this one.

## Left unbuilt / traps / debts — see rewritten STATE.md
