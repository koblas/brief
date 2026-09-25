---
id: SCENARIO-05
status: done
---

# SCENARIO-05: Moving an entry between state headings is not a drop; moving it out is

## Scenario

```gherkin
Scenario: SCENARIO-05 — Moving an entry between state headings is not a drop; moving it out is
  Given an entry under Traps
  When the new body carries it under Binding decisions
  Then no row is printed
  But when the new body carries it only under a non-state heading
  Then a row is printed for it
```

`droppedEntries` (`internal/scaffold/dropped.go`) already pools old and new entries across all
four configured headings as one multiset keyed on normalized text alone — heading is not part
of identity (SCENARIO-01's binding decision). No existing test drives the same entry text
through two different headings, so this pooling is currently unguarded by any assertion.
**Expected green on arrival** for both arms — say so and why.

## Implementation Plan

### Red

- [x] Step 1: `internal/cli/finish_dropped_internal_test.go`
  `Test_finish_treats_moving_an_entry_between_state_headings_as_no_drop_but_moving_it_out_as_a_drop_mem`
  — one fixture family through `cli.Run` (`newMemFinishFixtureWithState` + `runFinishMem`),
  table-driven over exactly one variable: the destination heading of `- moved entry`. The old
  body is identical in both cases: `## Traps` carries `- kept trap` then `- moved entry`
  (`- moved entry` at old-file line 9), the other three headings present and empty. Both new
  bodies carry all four configured headings present (an omitted one refuses with
  `ErrMissingStateHeading`, which would make the empty-stdout arm pass vacuously — D5) plus a
  trailing `## Notes` heading, `## Traps` keeping `- kept trap` so the diff is never vacuous
  (an all-empty-old-entries fixture proves nothing per SCENARIO-03's Traps). Case A ("moved
  between state headings"): `- moved entry` sits under `## Binding decisions`, `## Notes`
  empty; expect `stdout` empty, `err` nil, `stderr` equal to
  `memWantFinishCompleteLine("demo", "SCENARIO-01")` (exact equality, not merely non-empty —
  distinguishes a true no-drop from a masked refusal). Case B ("moved to a non-state
  heading"): `- moved entry` sits under `## Notes` instead, `## Binding decisions` empty;
  expect `stdout` equal to
  `"WARN  "+stateRel+":9  dropped from Traps, untagged: moved entry\n"` (pins D6: the row
  reports the *old* heading and *old* line, not `Notes`) and `stderr` containing
  `"(dropped 1 entry, listed on stdout)"`. Before asserting Case B green, run
  `go doc ./internal/platform/markdown Entries` and `Section` to confirm a state section
  scan stops at the next heading (`## Notes` included) rather than running past it; if Case B
  reddens, report it as a `markdown` section-boundary defect (D1) rather than patching around
  it — out of this scenario's scope to fix.

### Green

- No production edit anticipated. If either case reddens for a reason other than the
  `markdown` boundary check above, report which assumption in `droppedEntries` or `markdown`
  broke, rather than patching around it.

### Sweep

- [x] Step 2: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

### Verify

- [x] Step 3: full verification per `.claude/rules/agent-briefs.md` from start commit
  `cfdae4c`. Mutation, in `droppedEntries` (`internal/scaffold/dropped.go`): make the diff
  per-heading instead of pooled by keying `newCounts` as
  `newCounts[h+"\x00"+normalizeEntryText(e.Text)]++` (inside the existing per-heading loop)
  and `groups` as `groups[o.heading+"\x00"+o.text]` (the `newCounts[text]` lookup inside the
  surplus loop must key the same way) — Case A ("moved between state headings") must go red
  (a false-positive row for `moved entry` now appears, keyed on its old heading `Traps`
  against a new-body count that no longer sees it under `Binding decisions`); Case B ("moved
  to a non-state heading") must stay green, since it already reports the same row before and
  after. Verify the two cases individually — restore, diff byte-identical.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- The pooled multiset diff needed no production change for SCENARIO-05 — it already ignores
  heading for identity (SCENARIO-01). SCENARIO-05 is the first, and so far only, test that
  would catch a regression to per-heading diffing; before this scenario no fixture moved the
  same entry text across two headings. `STATE.md`: fold this into the existing pooled-diff
  bullet rather than add a new one (replace "required for SCENARIO-05's …" with "proven
  end-to-end through the CLI by SCENARIO-05; a heading-keyed mutation reddens the
  moved-between-headings case only") — net line count should not grow. `STATE.md` is at the
  80-line cap; if more room is needed, the "Finding shape amended …" bullet documents a
  completed cross-doc edit and is a candidate to compress to one line.

**Left unbuilt** — named so nobody assumes it exists:
- Nothing new. SCENARIO-06 (reworded/re-tagged/re-ticked) and SCENARIO-07 (continuation
  lines, duplicate-text drops) remain as STATE.md already records them.

**Traps** — things that look right and are not:
- A moved-entry fixture that omits `## Notes` from one arm only is two variables, not one —
  keep `## Notes` present (empty where unused) in both new bodies.
- Omitting any of the four configured headings from a new body triggers
  `ErrMissingStateHeading`, which prints no rows (D5) — indistinguishable from a genuine
  zero-drop finish unless the test also asserts `err` is nil.
- A drop fixture with nothing kept lets a diff that ignores the new body entirely still pass;
  `- kept trap` under `## Traps` in both old and new bodies guards against that here too.
