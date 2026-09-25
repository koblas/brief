---
id: SCENARIO-08
status: done
---

# SCENARIO-08: Only list items under the configured state headings are entries

## Scenario

```gherkin
Scenario: SCENARIO-08 — Only list items under the configured state headings are entries
  Given list lines inside a fence, paragraph lines, list items under other sections,
        an empty or duplicated configured heading, and CRLF line endings in the old body
  When the new body omits them
  Then no false row is printed and line numbers stay correct for CRLF
```

## Surface check (before planning the guard)

`droppedEntries` (`internal/scaffold/dropped.go`) consumes `config.StateHeadings` only through
`headings.Ordered()` (four strings, called once for the old-collection loop and once for the
new-count loop) and `dropRuleFor(heading, headings.OpenDebts)` (string equality). No other
method or field of `config.StateHeadings` is used, so the whole surface a guard must cover is
just: what `Ordered()` may legally return in production (four non-empty, pairwise-distinct
strings) versus what a caller *could* construct it with (empty or duplicate values).

Confirmed by reading `internal/platform/config/validate.go`'s `validateHeading`:
**`config.Resolve` (the CLI's own config-loading path) already refuses an empty or duplicate
`state-headings.*` value** before any `scaffold.Server` is built — `violations` checks every
state heading pairwise against itself and against `progress-heading`/`checklist-heading`/
`acceptance-heading`. `FinishFS`, however, takes a bare `config.Config` and never re-validates
it, and `checkArgumentHeadings` → `conform.MissingHeading` (`internal/platform/conform/conform.go`,
read directly) does not reject an empty or duplicate heading either — it calls
`markdown.Section(body, heading)` per heading and only refuses when `found == false`;
`Section(body, "")` matches an arbitrary blank line, so it trivially "finds" one, and a
duplicate value is simply looked up twice with the same trivially-true result. **Net: the guard
is unreachable through `brief finish`/`cli.Run`, but is real and reachable at the `scaffold`
package level** — a caller that builds `config.Config` by hand (exactly what this package's own
tests already do) can still hand `FinishFS` broken headings. Test there, per the architect
brief; no `// unreachable:` marker needed since the path is genuinely exercised, just not from
the CLI.

Confirmed by reading `internal/scaffold/finish.go`: `checkArgumentHeadings` runs only against
the **new** `state` argument, never against the old on-disk body, so "old file lacks a
heading" and "old file is empty" both reach `droppedEntries` unobstructed. A **missing** old
state file is different and already covered: `FinishFS` `Lstat`s `cfg.StateFile` and refuses
(`ErrMalformedFeature`, "state file is missing") before `droppedEntries` is ever called —
`internal/scaffold/finish_test.go` `Test_refuses_a_missing_state_file_on_finish` already pins
this, at exit 1. The outcome table's "No old state file, or empty | none | 0" row is right only
for the *empty* half; a genuinely missing file never reaches drop computation at all and never
exits 0. This scenario does not add a test for the missing-file half — see Handoff for the
table correction to flag downstream.

## Implementation Plan

### Red

- [x] Step 1: `internal/scaffold/finish_dropped_test.go`
      `Test_finish_ignores_list_lines_that_are_not_true_entries` — table, one case each for a
      list line inside a fenced code block, a paragraph line, and a list item under a fifth,
      non-configured heading; each case's old body also carries one genuine entry under a
      configured heading that the new body drops. Assert the exact one-row slice (the genuine
      drop only) per case, so an implementation that over- or under-scans cannot pass
      vacuously. Expected **green on arrival** — `markdown.Entries` already excludes fenced and
      paragraph lines (D1, pinned at the `markdown` level by `entries_test.go`) and
      `droppedEntries` only ever calls `Entries` with one of the four configured heading
      strings — this is the first proof of both facts at the `FinishFS` level.
- [x] Step 2: `internal/scaffold/finish_dropped_test.go`
      `Test_finish_attributes_no_drop_to_a_heading_absent_from_the_old_body` — old body missing
      one configured heading's section entirely (no line equal to it at all) while a different
      configured heading's entry is genuinely dropped; asserts exactly one row, attributed to
      the present heading. Expected **green on arrival** — `markdown.Entries` already returns
      nil for an absent heading (`sectionSpan`'s `ok == false`).
- [x] Step 3: `internal/scaffold/finish_dropped_test.go`
      `Test_finish_reports_no_drops_from_an_empty_old_state_file` — two cases differing in one
      variable, the old body's own content: an empty (zero-byte) old body against a new body
      carrying the four required headings, asserting `Dropped` empty; and, as the control arm,
      a non-empty old body carrying one entry under a configured heading that the new body
      drops, asserting the exact one-row slice. Expected **green on arrival**.
- [x] Step 4: `internal/scaffold/finish_dropped_test.go`
      `Test_finish_excludes_an_empty_configured_heading_from_scanning` — via a new
      `finishDroppedWithConfig(t, mem, newState, cfg)` helper (`finishDropped` becomes a thin
      wrapper calling it with `config.Default()`), a hand-built `config.Config`
      (`config.Default()` with `StateHeadings.OpenDebts` blanked to `""` — a field distinct
      from the one holding the real entry, so the guard cannot accidentally erase both); old
      body carries a genuine, later-dropped entry under Traps (untouched, real heading) *and*
      a fifth, non-configured `## Notes` section with its own entry, both placed after the
      body's first blank line; new body omits both. Asserts the exact one-row slice (the Traps
      drop only). **Fails against current code**: today, scanning the blanked
      heading calls `markdown.Entries(oldBody, "")`, which — per `markdown.Section`'s own
      documented behavior — anchors on the first blank line and never finds a same-or-shallower
      heading to end the section, so it scans to end of file. That picks up the real entry a
      second time (a duplicate row) and the `## Notes` entry once (a wholly false row): three
      rows today, one expected. Needs the Green guard below.
- [x] Step 5: `internal/scaffold/finish_dropped_test.go`
      `Test_finish_excludes_a_duplicated_configured_heading_from_scanning` — a hand-built
      `config.Config` with two `StateHeadings` fields set to the identical, non-empty text
      (e.g. Binding decisions and Traps both `"## Shared"`); old body carries one entry under
      the shared heading and, separately, one entry under Left unbuilt (kept distinct), both
      genuinely dropped in the new body. Asserts the exact one-row slice — the Left-unbuilt
      entry only; D7 reads a duplicated heading as contributing zero entries from either
      position, not one. **Fails against current code**: `headings.Ordered()` yields the
      duplicated string twice, so the old-collection loop scans that section twice and appends
      the same physical entry twice, producing two rows for it plus the one Left-unbuilt row —
      three today, one expected. Needs the same Green guard as Step 4.
- [x] Step 6: `internal/scaffold/finish_dropped_test.go`
      `Test_finish_keeps_correct_line_numbers_and_blank_line_handling_under_crlf` — old body
      using `\r\n` line endings throughout: one entry genuinely dropped (asserts its row's
      `Line` is the entry's correct 1-based physical line — the assertion this step actually
      turns on), and, under a different heading, one *kept* entry immediately followed by a
      bare-`\r` blank line and then a paragraph line. The **new** body carries that kept
      entry's own section in plain `\n` line endings, entry immediately followed by an
      ordinary blank line and the same paragraph line — deliberately asymmetric with the old
      body's CRLF, so only the old side's fold depends on the blank-line check; the new side's
      blank line reads as blank (`""`) regardless of that check. Asserts the exact one-row
      slice (the genuine drop only) — proving the kept entry's continuation folding stopped at
      the old body's `\r` blank line rather than swallowing the paragraph line, which would
      otherwise change its normalized text and report it as a second, false drop that the
      symmetric new side would not also produce. Expected **green on arrival**: `Line` is
      counted over `strings.Split(body, "\n")`, unaffected by a trailing `\r` on each element,
      and `isContinuationLine` already tests blankness with `strings.TrimSpace(line) == ""`,
      which a bare `"\r"` line satisfies.
- [x] Step 7: `internal/scaffold/finish_dropped_test.go`
      `Test_finish_normalizes_an_interior_carriage_return_on_a_continuation_line` — old body
      with one entry whose continuation line (SCENARIO-07 folding) carries a literal `\r` *not*
      at the line's own edge (e.g. mid-word — a stray embedded carriage return `TrimSpace`
      cannot reach because it isn't a leading or trailing character), matched in the new body by
      the same logical entry written cleanly, with no embedded `\r`; and, under a different
      heading, one unrelated entry genuinely dropped, present in the old body only. Asserts the
      exact one-row slice (the unrelated drop only) — proving the interior-`\r` entry matched
      its clean counterpart and did not additionally appear as a second, false drop. Expected
      **green on arrival**: this is the fixture SCENARIO-03 could not construct (a plain
      trailing-CRLF line never reaches `normalizeEntryText`'s own
      `ReplaceAll(raw, "\r", "")` with a `\r` still in `raw`, since `trimEOL`/`TrimSpace` strip
      it first) — see this scenario's Verify mutation for the reddening proof.

### Green

- [x] Step 8: `internal/scaffold/dropped.go` — add unexported `scannableHeadings(headings
      config.StateHeadings) []string`, with its own doc comment: filters `headings.Ordered()`
      to the heading strings that are non-empty and appear exactly once (D7) — an empty value
      or one shared by more than one configured field contributes nothing, from either
      position.
- [x] Step 9: `internal/scaffold/dropped.go` `droppedEntries` — replace both loops' `range
      headings.Ordered()` with `range scannableHeadings(headings)` (old-collection loop and
      new-count loop); update the function's doc comment to name this exclusion.

### Sweep

- [x] Step 10: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

### Verify

- [x] Step 11: full verification per `.claude/rules/agent-briefs.md`. Four mutations, each
      verified individually and restored/diffed clean before the next:
      - In `scannableHeadings`, disable only the empty-heading exclusion → Step 4 goes red;
        Step 5 must stay green.
      - Restore, then disable only the duplicate-heading exclusion → Step 5 goes red; Step 4
        must stay green.
      - Restore, then in `normalizeEntryText` disable the interior-CR strip (let `strings.Fields`
        see the raw `\r` instead of a pre-deleted one) → Step 7 goes red (the interior `\r`
        now acts as a token separator instead of being deleted, so the two entries no longer
        normalize to the same text and a false-positive drop row appears).
      - Restore, then in `isContinuationLine` change the blank-line test from
        `strings.TrimSpace(line) == ""` to a raw `line == ""` comparison → Step 6 goes red (a
        bare `"\r"` line no longer reads as blank, so the trailing paragraph line folds into
        the kept entry and it is reported as a second, false drop). Restore, diff
        byte-identical.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `droppedEntries` scans only `scannableHeadings(headings)`, never raw `headings.Ordered()`: an
  empty or duplicate-valued configured heading contributes zero entries, from every position
  it appears at (D7) — do not special-case "keep the first of a duplicate pair" later.
- The empty/duplicate-heading guard is real production code in `scaffold` but **unreachable
  through `brief finish`/`cli.Run`**: `config.Resolve` (`internal/platform/config/validate.go`
  `validateHeading`) already refuses an empty or pairwise-duplicate `state-headings.*` value
  before a `scaffold.Server` is ever built. It is tested by constructing `config.Config`
  directly, below the CLI — do not add a `cli.Run`-level test for it.

**Left unbuilt** — named so nobody assumes it exists:
- `finishLong`'s drop-reporting prose paragraph — SCENARIO-10, unchanged by this scenario.
- The refusal-carries-no-`dropped_entries` proof — SCENARIO-09, unchanged by this scenario.
- **Spec correction owed**: specification.md's outcome table row "No old state file, or empty
  | none | 0" is wrong for the missing-file half — `Test_refuses_a_missing_state_file_on_finish`
  (`internal/scaffold/finish_test.go`, pre-existing) proves a missing state file refuses at
  exit 1, before `droppedEntries` ever runs. Only "empty" belongs on that row at exit 0. Flag
  for the final product-vision pass to amend the table; out of this scenario's own scope
  (triage marked the validation band untouched).

**STATE.md maintenance** — the developer folding this scenario's Handoff into STATE.md should
retire, not just add:
- The `## Left unbuilt` line naming SCENARIO-08 as owning "the empty/duplicate configured-heading
  guard, the CRLF fixture, and the fence/paragraph-outside-heading exclusion proofs" — all now
  built; remove the line rather than leaving it stale.
- The Traps line "`markdown.Section(body, "")` / `sectionSpan(lines, "")` match the first blank
  line — an empty configured heading would scan a random region" — superseded by the binding
  decision above (`scannableHeadings` now excludes it before `Entries` is ever called); fold
  into one line if kept at all, since the mechanism is now handled, not merely a known risk.
- The Traps line about `checklistItemRe` vs `entryItemRe` is unrelated and stays.
- The Traps line "The fold's blank-line check must treat a CRLF line (a bare `"\r"`) as blank,
  via `strings.TrimSpace`, never raw `line == ""` — SCENARIO-08 owns the dedicated CRLF
  fixture" — retire this half, now pinned by Step 6 and its mutation. Keep the same bullet's
  `dropExcerpt` 80-rune half, which is still live and untouched by this scenario.
- Add one new, two-line binding decision: `normalizeEntryText`'s explicit
  `ReplaceAll(raw, "\r", "")` is load-bearing only for an interior `\r` a continuation line
  leaves behind (SCENARIO-07 folding); a trailing-CRLF line never reaches it with a `\r` still
  present, since `trimEOL`/`TrimSpace` strip it first (proven by Step 7 and its mutation).

**Traps** — things that look right and are not:
- `checkArgumentHeadings` (`conform.MissingHeading`) does **not** reject an empty or duplicate
  configured heading either — `markdown.Section(body, "")` matches the first blank line, so the
  check trivially "passes". `scannableHeadings` is the only guard; do not read `FinishFS`'s
  validation band as a second line of defense.
- A plain whole-file CRLF fixture with no interior `\r` cannot redden the CR-strip mutation —
  every `\r` it contains sits at a line's own trailing edge, already stripped by
  `trimEOL`/`TrimSpace` before `normalizeEntryText` runs. Only an interior `\r`, mid-continuation-
  line (Step 7), does.
- An `Empty`/zero-row assertion alone proves nothing if the guard over-filters — every guard
  test above (Steps 3–7) pairs the excluded shape with a genuine, still-expected drop in the
  same fixture and asserts the exact resulting slice, not bare absence.
