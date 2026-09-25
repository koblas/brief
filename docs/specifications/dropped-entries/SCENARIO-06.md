---
id: SCENARIO-06
status: done
---

# SCENARIO-06: A reworded, re-tagged or re-ticked entry is reported as dropped

## Scenario

```gherkin
Scenario: SCENARIO-06 — A reworded, re-tagged or re-ticked entry is reported as dropped
  Given an entry whose text, tag, or checkbox marker changes in the new body
  When I finish
  Then a row reports the old text
```

`droppedEntries` (`internal/scaffold/dropped.go`) keys its pooled multiset diff on
`normalizeEntryText(e.Text)` alone (SCENARIO-01's binding decision), and `markdown.Entries`
(`internal/platform/markdown/entries.go`, `entryItemText`) strips only the `- `/`* `/`N. `
marker — a leading `[ ]`/`[x]` checkbox marker is left inside `Text` untouched. So a reworded,
re-tagged, or re-ticked entry already produces a different normalized string whose old
occurrence is absent from the new pooled set and is reported as a drop carrying the *old*
text/tag/line, the same mechanism SCENARIO-01/05 already exercise for outright removal and
heading moves. **Expected green on arrival for all three change kinds** — say so and why.
No new production surface: no new Store method, port, or adapter.

## Implementation Plan

### Red

- [x] Step 1: `internal/cli/finish_dropped_internal_test.go`
  `Test_finish_reports_a_reworded_re_tagged_or_re_ticked_entry_as_dropped_mem` — one
  table-driven `_mem` test through `cli.Run` (`newMemFinishFixtureWithState` + `runFinishMem`),
  **one shared old `STATE.md` body** (the same layout
  `Test_finish_treats_moving_an_entry_between_state_headings_...` uses: `## Traps` holds
  `- kept entry` at line 7 then `- [x] Fix the thing (TAG1)` at line 9, the other three
  headings present and empty), four cases that vary only the **new** body's line 9 so each
  case differs from the old body — and from the control — by exactly one variable:
  - "reworded": new line 9 `- [x] Fix the other thing (TAG1)` (wording changes, tag and
    marker do not).
  - "re-tagged": new line 9 `- [x] Fix the thing (TAG2)` (tag changes, wording and marker do
    not).
  - "re-ticked": new line 9 `- [ ] Fix the thing (TAG1)` (checkbox marker changes, wording and
    tag do not). Confirmed by reading `checkStepChecklist`
    (`internal/scaffold/finish.go:235,420-430`): the open-checklist refusal scans only the
    step file's own Implementation Plan (`stepBody`), never the `--state` argument, so an
    open `- [ ]` inside a state-heading entry cannot trigger `ErrOpenChecklistItem` and this
    case needs no separate old entry.
  - "whitespace-only reflow is not a drop" (control): new line 9
    `- [x] Fix  the   thing (TAG1)` (only interior whitespace changes — the one variable
    SCENARIO-03 already proves collapses to the same identity).
  For the three drop cases, want `stdout` **byte-identical across all three**:
  `"WARN  "+stateRel+":9  dropped from Traps, tagged TAG1: [x] Fix the thing (TAG1)\n"` — the
  row is wholly determined by the *old* body and does not vary with what the new body changed
  it to, which is the scenario's actual claim ("a row reports the old text"). For the control,
  want `stdout` empty and `stderr` equal to `memWantFinishCompleteLine("demo", "SCENARIO-01")`.
  `- kept entry` stays unchanged in every new body, so the diff is never vacuous (STATE.md's
  "a drop fixture must keep at least one old entry" trap). No existing test currently
  exercises an entry whose own text is mutated in place rather than removed or relocated —
  this table is the first to prove D2's "reword/re-tag/re-tick all count as removal" claim
  end-to-end.

### Green

- No production edit anticipated. If a case reddens, report which assumption in
  `droppedEntries` or `markdown.Entries`/`entryItemText` broke, rather than patching around it.

### Sweep

- [x] Step 2: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

### Verify

- [x] Step 3: full verification per `.claude/rules/agent-briefs.md` from start commit
  `1c369ae`. Two mutations in `internal/scaffold/dropped.go`, verified **individually**
  (restore and diff byte-identical between each):
  - **Checkbox-marker-stripped identity.** In `normalizeEntryText`, strip a leading
    `[ ]`/`[x]` (and the space after it) before collapsing whitespace. Only the "re-ticked"
    subtest must go red (its row disappears, since the old and new text now normalize
    identically); "reworded", "re-tagged" and the control subtest must stay green. Run the
    single subtest via `-run
    'Test_finish_reports_a_reworded_re_tagged_or_re_ticked_entry_as_dropped_mem/re-ticked'`.
  - **Tag excluded from identity.** In `droppedEntries`, key `groups` and `newCounts` on the
    trailing tag stripped from the normalized text (e.g. `trailingTagRe.ReplaceAllString(text,
    "")`, trimmed) instead of the full text — leave `DroppedEntry.Text`/`Tag` construction
    unchanged. Only the "re-tagged" subtest must go red (old TAG1 and new TAG2 bodies now
    match on their tag-stripped text, `[x] Fix the thing`); "reworded" and "re-ticked" still
    differ after tag-stripping (wording/marker differ) and must stay green, and the control
    already matches before any tag-stripping (whitespace alone collapses identically) so it
    stays green too. Run the single subtest via `-run
    'Test_finish_reports_a_reworded_re_tagged_or_re_ticked_entry_as_dropped_mem/re-tagged'`.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Reword/re-tag/re-tick need no dedicated production logic — they are the existing pooled
  multiset diff (SCENARIO-01/05) seeing a different `normalizeEntryText` string; a `- [ ] `/
  `- [x] ` marker survives into `Entry.Text` because `markdown.Entries`' `entryItemText` strips
  only the `- `/`* `/`N. ` marker, never the checkbox brackets — proven end-to-end by
  SCENARIO-06's table, mutation-verified against both a marker-stripped and a tag-stripped
  identity key individually.

**Left unbuilt** — named so nobody assumes it exists: unchanged from STATE.md —
SCENARIO-07 (continuation lines, duplicate-text-dropped-once), SCENARIO-08 (empty/duplicate
heading guard, CRLF proof), SCENARIO-09 (refusal-carries-no-`dropped_entries` proof), and
`finishLong`'s drop-reporting prose paragraph (SCENARIO-10).

**Traps** — things that look right and are not:
- Do not add a "strip the checkbox marker" or "strip the tag before matching" step to
  production code as a "cleanup" — either one silently breaks this scenario's re-ticked or
  re-tagged row; both are exactly the mutations this scenario's Verify step exercises to prove
  they must *not* exist.
- `STATE.md` was already at ~80 lines before this scenario. Fold this scenario's one new
  binding-decision fact into the existing pooled-diff bullet (it already covers SCENARIO-01/05
  and is the natural home for "reword/re-tag/re-tick need no new logic") rather than appending
  a new bullet — net line count should not grow, and should shrink if room is needed. If room
  is needed, SCENARIO-05's own Handoff already named the compression candidate: the "Finding
  shape amended …" bullet documents a completed cross-doc edit and can compress to one line.
