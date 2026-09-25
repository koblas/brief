---
id: SCENARIO-07
status: done
---

# SCENARIO-07: A multi-line entry is one entry; a duplicate dropped once is reported once

## Scenario

```gherkin
Scenario: SCENARIO-07 — A multi-line entry is one entry; a duplicate dropped once is reported once
  Given an entry with continuation lines and indented sub-items
  When the new body keeps it reflowed onto one line
  Then no row is printed
  And given the same text twice in the old body and once in the new
  Then one row is printed, at the later occurrence's line
```

## Implementation Plan

### Red

- [x] Step 1: `internal/platform/markdown/entries_test.go` `Test_Entries_folds_a_wrapped_continuation_line_into_the_entry_text` — a column-0 item followed by an unindented wrapped second physical line (a "lazy continuation" — D1 folds it regardless of indentation) and then an indented sub-item. Asserts `Text` joins all three with a single space, each line's own surrounding whitespace trimmed first, with the sub-item's own list marker kept (only the parent item's own leading marker is stripped, once, at the start). Asserts `Line` stays the item's own first line (D6) even though `Text` now spans later lines. Fails today because `Entries` never folds continuation lines, so `Text` is only the item's own first line.
- [x] Step 2: `internal/platform/markdown/entries_test.go` `Test_Entries_continuation_stops_at_blank_line_next_item_heading_and_fence` — table where every case carries exactly one continuation line between the item and the boundary being pinned, and asserts the exact resulting `Text` (item plus that one line, nothing from past the boundary) rather than only an absence — an implementation that folds nothing would otherwise pass this table vacuously:
  - a blank line after the continuation line, then a second item — the first entry's `Text` stops at the continuation line, the second item is its own entry.
  - a second column-0 item directly after the continuation line, no blank line — same stop, immediately.
  - a heading deeper than the section's own configured heading (e.g. a `###` inside a `##` section) directly after the continuation line, followed by a second column-0 item — the first entry's `Text` stops at the continuation line, and the item after the `###` comes back as its own `Entry` rather than being swallowed or excluded, proving the `###` neither folds into the first entry nor ends the section (`sectionSpan`'s own end-of-section rule only ends on same-or-shallower level).
  - an opening fence directly after the continuation line, non-item text inside the fence, a closing fence, then a non-blank non-item line after it — the entry's `Text` stops at the continuation line; neither the fenced content nor the post-fence line folds into it, and neither produces an `Entry` of its own.
  Fails today on every case: none of today's single continuation lines fold in at all, so the exact-`Text` assertion (item + the one line) does not hold before Green.
- [x] Step 3: `internal/cli/finish_dropped_internal_test.go` `Test_finish_reports_no_drop_when_a_wrapped_entry_is_reflowed_to_one_line_mem` — mirrors `Test_finish_reports_no_drops_when_the_new_body_only_reflows_whitespace_mem`'s shape (`newMemFinishFixtureWithState`, `memWriteInput`, `runFinishMem`, `memWantFinishCompleteLine`). The old `STATE.md` body carries the multi-line entry (an unindented wrapped continuation line plus an indented sub-item, its own marker kept) under one heading, plus a second, single-line entry kept unchanged under another heading — matching STATE.md's own trap that a drop fixture must keep at least one old entry, so a diff that ignored the new body entirely cannot pass by accident. The new body carries the kept entry unchanged and the multi-line entry's words reflowed onto a single line, including the sub-item's own kept marker written literally (not paraphrased into prose); all four configured headings present. Keep the multi-line entry under 80 runes normalized, so `dropExcerpt`'s cut (proven in `Test_dropExcerpt`) cannot mask a folding bug by truncating the very words that prove it. Asserts stdout is empty and stderr is byte-identical to the zero-drop success line. Fails today: the old entry's un-folded `Text` (its first line only) never matches the new entry's full reflowed text, so a spurious `dropped-entry` WARN row is printed.
- [x] Step 4: `internal/cli/finish_dropped_internal_test.go` `Test_finish_reports_a_dropped_multiline_entry_at_its_first_line_mem` — control arm for Step 3, differing in exactly one variable: the same old body (multi-line entry plus the kept entry), but the new body omits the multi-line entry entirely instead of reflowing it, keeping the other entry unchanged. Asserts exactly one WARN row, at the multi-line entry's **first** old-file line (D6), whose text-mode excerpt carries the fully folded normalized text — the wrapped line's and the sub-item's words included, not cut (fixture stays under 80 runes for the same reason as Step 3). Fails today: the un-folded old text is short and wrong, so the reported excerpt omits the continuation content.
- [x] Step 5: `internal/cli/finish_dropped_internal_test.go` `Test_finish_reports_a_duplicated_entry_dropped_once_at_its_later_occurrence_mem` — the old body carries the identical normalized text as two separate column-0 items under one heading, at two different old-file lines; the new body carries it once. Asserts exactly one WARN row, at the **second** (later) old-file line. Confirms D2's "surplus old occurrences — the last ones in old-file order — are dropped." Reading `internal/scaffold/dropped.go`'s `droppedEntries` shows `idxs[len(idxs)-surplus:]` already selects the tail of the line-ordered group, i.e. the last occurrences — so this test is expected **green on arrival**; report it as such rather than manufacturing a red. If it reddens instead, that is real evidence the multiset picks the first occurrence, not the last — stop and report before touching `dropped.go`.

### Green

- [x] Step 6: `internal/platform/markdown/entries.go` `Entries` — fold continuation lines into the preceding column-0 item's `Text`: every following non-blank line that is not itself a column-0 item, a heading, or a fence delimiter joins the entry, each line's own surrounding whitespace trimmed and the pieces joined with a single space, whatever the line's own indentation — an indented sub-item's own marker is never stripped, only the parent item's own leading marker is stripped once, at the start. `Line` is set once, at the item's own first line, and never advances as later lines fold in. The entry ends at a blank line, the next column-0 item, a heading, or an opening fence delimiter. Pinned decision: a fence delimiter ends the entry in progress and begins the existing fence-skip state; its contents are never entries and are never folded into the entry before it, and the line immediately after the closing fence starts fresh (never folds into the entry before the fence either).

### Sweep

- [x] Step 7: fix what `go build ./... && golangci-lint run ./...` reports (sweep)
- [x] Step 8: `internal/platform/markdown/doc.go`, `entries.go` (`Entry` / `Entries` doc comments) — document continuation-line folding, indented-sub-item folding, the fence-ends-the-entry rule, and that `Line` never moves off the item's own first line; the package doc's "an indented line is ignored, never an entry of its own" line is still true but now needs to say what an indented line becomes instead. Also reword `entries_test.go`'s existing "(built in a later scenario)" / "a later scenario builds" comments in `Test_Entries_recognizes_every_column_zero_marker` and `Test_Entries_excludes_lines_that_are_not_column_zero_items`, now false once Step 6 lands (sweep)
- [x] Step 9: grep `internal/scaffold` and `internal/cli` state fixtures (existing dropped-entries tests from SCENARIO-01–06) for a column-0 item line immediately followed by a non-blank, non-item line — STATE.md's own trap names this pattern — and re-check each hit's expectation against the new folding behavior; fix any fixture whose intent depended on that line being ignored (sweep)

### Verify

- [x] Step 10: full verification per `.claude/rules/agent-briefs.md` — `go build ./...`; the full `go test ./...` run with coverage; `go test -race ./internal/platform/markdown/... ./internal/scaffold/... ./internal/cli/...`; `golangci-lint run ./...`; the coverage gate and test-stats scripts. Plus three named mutations, each restored and diffed byte-identical after:
  - Disable continuation folding (make the new fold loop in `Entries` a no-op, reverting to "one line per entry") → Step 3 goes red (the wrapped old entry no longer matches the reflowed new one, so a spurious row appears).
  - Flip `dropped.go`'s surplus selection to take the head of `idxs` instead of the tail (`idxs[:surplus]`) → Step 5 goes red (reports the first old-file line instead of the second).
  - Relax `entryItemRe`/`entryItemText`'s column-0 anchor to also match an indented item → Step 1 goes red (the sub-item wrongly becomes its own `Entry` instead of continuation text, so the parent's `Text` loses it) and Step 3 goes red too (the old entry now splits into two texts instead of one, so it no longer matches the new reflowed text either).

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- A fence delimiter ends the entry in progress, and the line immediately after the closing fence starts fresh rather than folding into the entry before the fence — pinned to resolve D1's silence on fence-vs-continuation ordering. SCENARIO-08's own fence/list-in-fence fixtures must not assume fence-adjacent content is silently absorbed into a neighboring entry.
- Continuation-line marker stripping is asymmetric: the parent item's own leading `- `/`* `/`N. ` marker is stripped once, at `Text`'s start; a sub-item's own marker, folded in as continuation content, is kept verbatim. Changing that later silently changes every multi-line entry's identity (its normalized text).
- `Entry.Line` is fixed at the item's own first line and never advances during folding (D6) — proven by Step 1's direct assertion and Step 4's dropped-entirely control arm.
- `dropped.go`'s duplicate-selection (`idxs[len(idxs)-surplus:]`) already implements D2's "last occurrence dropped" correctly, confirmed by reading before any change — SCENARIO-07 adds CLI-level coverage and a mutation guard for it, not a fix. Do not "simplify" that slice expression later without re-running the duplicate-drop test.
- A heading deeper than the section's own configured heading (e.g. a `###` inside a `##` section) does not end the *section* (`sectionSpan` only ends on same-or-shallower level) but **does** end an in-progress *entry*'s continuation, per D1's "a heading" with no level qualifier. These are two different boundaries at two different scopes — do not conflate them when touching either.

**Left unbuilt** — named so nobody assumes it exists:
- The empty/duplicate configured-heading guard, the CRLF fixture, and the fence/paragraph-outside-heading exclusion proofs — SCENARIO-08's scope, untouched here.
- The refusal-carries-no-`dropped_entries` proof and the identical-re-finish proof — SCENARIO-09.
- `finishLong`'s drop-reporting prose paragraph — SCENARIO-10.

**Traps**:
- `entryItemRe`/`entryItemText` stays column-0-anchored (no leading `\s*`). Do not relax it to fold an indented item into its own `Entry` — that is exactly the mutation Step 10 guards against; sub-items must stay continuation text of their parent.
- Do not assume Step 5 needs a red — read `dropped.go` before assuming the duplicate-pick bug exists; it does not, verified by inspection here.
- `dropExcerpt` cuts at 80 runes. Any fixture meant to prove folding by its text content (Steps 3 and 4) must stay under that, or the cut — not a folding bug — is what the assertion ends up reflecting.
- The fold's blank-line check must treat a CRLF line (a bare `"\r"` after `strings.Split(body, "\n")`) as blank — test it after `trimEOL`/`TrimSpace`, never as a raw `line == ""` comparison. Getting this wrong here will surface as a false continuation fold on SCENARIO-08's own CRLF fixture, not as a failure in this scenario's own tests.
- `STATE.md` is at its ~80-line cap. When rewriting it at the end of this scenario, distil rather than append: fold SCENARIO-01–06's still-binding decisions down and drop anything superseded (e.g. the "continuation lines" line in today's "Left unbuilt", once built), instead of adding a seventh bullet on top of an already-full file.
