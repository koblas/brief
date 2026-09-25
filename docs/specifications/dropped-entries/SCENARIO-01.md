---
id: SCENARIO-01
status: done
---

# SCENARIO-01: A dropped state entry is reported, not refused

## Scenario

```gherkin
Scenario: SCENARIO-01 — A dropped state entry is reported, not refused
  Given a feature whose STATE.md lists "- X (SCENARIO-02)" under Traps at line 17
  When I finish SCENARIO-03 with a state body that omits it
  Then the step is done and the exit code is 0
  And stdout is "WARN  <state-rel>:17  dropped from Traps, tagged SCENARIO-02: X (SCENARIO-02)"
  And the stderr success line says "(dropped 1 entry, listed on stdout)"
```

User-visible contract (from spec *Surface & Copy*, verbatim):
`brief finish <feature> <step> --handoff <path|-> --state <path|->` in text mode. Stdout gets
one row per drop, `WARN  <state-rel>:<line>  dropped from <heading>, tagged <tag>: <excerpt>`
(`untagged: ` when the entry has no tag), in old-file line order, no header, no indent. The
stderr success line's `replaced <state-rel>` clause becomes
`replaced <state-rel> (dropped N entries, listed on stdout)` (singular `1 entry`). Exit 0.
Refusal (1) and usage (2) paths are unchanged here; asserting that they print no rows is
SCENARIO-09.

Where the code lives, and why:
- **Entry scanner → `internal/platform/markdown`** (new `entries.go`). It needs the
  unexported `sectionSpan` and `fenceState`, and it follows `FirstUnchecked`'s precedent of
  1-based line numbers counted over the whole body (D6). It is generic: it knows about list
  items under a heading, not about state files. It does not reuse `checklistItemRe`, which
  accepts leading whitespace and matches only `- [ ]` items. D1 needs column 0 and `- ` /
  `* ` / `N. `.
- **Normalization, multiset diff, tag, rule and the drop type → `internal/scaffold`** (new
  `dropped.go`). `scaffold` is the only consumer. `assemble` never needs any of it, so there is
  nothing shared to move down.
- **Excerpt, detail and row rendering → `internal/cli/finish.go`**. The feature package does no
  terminal formatting. `detail` is built by one helper, because SCENARIO-04's JSON `detail`
  must be the same string.

Rule classification (`dropped-debt` vs `dropped-entry`) and `untagged` rendering are built
**now**, with both arms tested. Hard-coding `dropped-entry` would be silently wrong for
open-debt entries, and an untested arm fails the coverage gate.

Product Verdict item 2 (the finding-shape doc amendment) is scheduled here, because this is
the first scenario that renders a row. Item 4 (the Open debts tag) is left to SCENARIO-02.

## Implementation Plan

### Red

- [x] Step 1: `internal/platform/markdown/entries_test.go` `Test_list_entries_*` — table: `- `, `* ` and `N. ` items at column 0 under the heading, each with its whole-body 1-based line. A paragraph line and an indented line yield nothing; each is placed first in the section or after a blank line, never directly under an item, so SCENARIO-07's continuation rule does not break the case. Heading absent yields nothing. Fails: the scanner does not exist yet.
- [x] Step 2: `internal/scaffold/dropped_internal_test.go` `Test_trailing_tag_*` — table for the unexported tag extractor: `(SCENARIO-02)` gives the tag; no parens, `()`, `(a b)` and a mid-text `(x)` give none. Fails: the extractor is missing.
- [x] Step 3: `internal/scaffold/finish_dropped_test.go` `Test_finish_reports_an_entry_the_new_state_body_omits` — Server-level test against the rwfs fixture. The old STATE has one entry kept and `- X (SCENARIO-02)` under `## Traps` at line 17; the new body drops only X. Assert `FinishResult.Dropped` is exactly one element: Line 17, Heading `Traps`, Tag `SCENARIO-02`, Text `X (SCENARIO-02)`, Rule `dropped-entry`. Fails: the field does not exist or is empty.
- [x] Step 4: `internal/scaffold/finish_dropped_test.go` `Test_finish_classifies_an_open_debts_drop_as_dropped_debt` — an entry dropped from under `## Open debts` carries Rule `dropped-debt` and an empty Tag. Fails: wrong rule.
- [x] Step 5: `internal/scaffold/finish_dropped_test.go` `Test_finish_reports_drops_in_old_file_line_order_across_headings` — drops under two headings, with the later heading earlier in the old file, come back in old-line order. Fails: they come back in heading order or unordered.
- [x] Step 5a: `internal/scaffold/finish_result_test.go` — extend the existing no-op test, and add a zero-drop write case: `Dropped` is empty **and non-nil** on both. Fails: the field is missing or nil.
- [x] Step 6: `internal/cli/finish_internal_test.go` `Test_finish_prints_a_warn_row_for_a_dropped_entry_mem` — through `cli.Run` with the mem fixture. The old STATE has X at line 17 plus a kept entry. Assert stdout is exactly the one row with `<state-rel>:17`, the stderr line contains `(dropped 1 entry, listed on stdout)`, and err is nil. Fails: stdout is empty.
- [x] Step 7: `internal/cli/finish_internal_test.go` `Test_finish_pluralises_the_drop_count_and_renders_untagged_rows_mem` — two drops, one untagged. Assert both rows, the `untagged: ` form, and `(dropped 2 entries, listed on stdout)` on the next-step variant of the success line. Fails: no rows and no suffix.
- [x] Step 8: `internal/cli/finish_internal_test.go` `Test_finish_complete_line_carries_the_drop_suffix_mem` — the last open step finishes with one drop; the `… is complete` variant carries the suffix. Fails: the suffix is missing.
- [x] Step 9: `internal/cli/finish_internal_test.go` `Test_drop_excerpt_*` — table for the unexported excerpt helper: 80 runes stays uncut; 81 runes is cut to 80 plus `…`; multibyte text is cut on a rune boundary, never a byte. Fails: the helper is missing.

### Green

- [x] Step 10: `internal/platform/markdown/entries.go` — exported scanner over one heading's section. Fence-aware via `sectionSpan` and `fenceState`. Column-0 `- ` / `* ` / `N. ` items only. Returns each entry's whole-body 1-based first line and its raw text, in a shape that can grow continuation lines (SCENARIO-07) without changing the signature.
- [x] Step 11: `internal/scaffold/dropped.go` — the `DroppedEntry` type (Rule, Heading, Line, Tag, Text), the rule constants `dropped-entry` and `dropped-debt`, and the unexported helpers: whitespace normalization (D2, CR stripped first), trailing-tag extraction (D3), heading display text (the leading ATX `#` run and its space stripped), and the pooled multiset diff (D2) that returns drops sorted by old line.
- [x] Step 12: `internal/scaffold/finish.go` `FinishResult.Dropped` plus `FinishFS` — compute the drops from `stateBytes` vs `state` in the `refinishWrite` path, after the `switch` and immediately before `applyFinishWrites`. Leave the validation band and the write order untouched. Set a non-nil empty slice on the R11 no-op return as well.
- [x] Step 13: `internal/cli/finish.go` `runFinish` text branch — write one row per `res.Dropped` to `out.stdout`. Build the `replaced <rel>[ (dropped N entr{y|ies}, listed on stdout)]` clause **once**, before the Next/complete branch, and feed it to both `Fprintf` variants. Add the unexported excerpt and detail helpers, with the `WARN` severity as a named constant.

### Sweep

- [x] Step 14: `internal/platform/markdown/doc.go` — add the new scanner to the package overview; doc comment on the scanner states D1's column-0 and fence rules (sweep)
- [x] Step 15: `internal/scaffold/finish.go` — `FinishResult` doc comment gains `Dropped` (old-line order, empty never nil, computed only on the write path); `DroppedEntry` doc states Heading is display text and Tag is `""` when untagged (sweep)
- [x] Step 16: `docs/specifications/brief/specification.md` R14a and the Default profile's **Findings** paragraph, plus `docs/specifications/brief/STATE.md` "Findings render …" decision — amend to `<SEVERITY>  <path>[:<line>]  <detail>`. Add: "`check` groups rows under a `<feature>  (in flight|complete)` header; `finish` prints them bare" (Product Verdict item 2) (sweep)
- [x] Step 17: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

### Verify

- [x] Step 18: full verification per `.claude/rules/agent-briefs.md` from start commit `62fa2e6`. Mutations, one at a time:
  - (a) in the scaffold diff, stop subtracting the new body's entries (report every old entry) → `Test_finish_reports_an_entry_the_new_state_body_omits` goes red (more than one element);
  - (b) in the markdown scanner, report the section-relative index instead of the whole-body line → `Test_finish_prints_a_warn_row_for_a_dropped_entry_mem` goes red (`:17` lost);
  - (c) in the scaffold heading-display helper, return the configured heading unstripped → `Test_finish_prints_a_warn_row_for_a_dropped_entry_mem` goes red (`dropped from ## Traps`);
  - (d) in the rule classification, always return `dropped-entry` → `Test_finish_classifies_an_open_debts_drop_as_dropped_debt` goes red.

Follow-up note (fix pass 1, MINOR test finding 8) — the mutation each table case rules out,
one line per table:
- `Test_trailing_tag_extraction` (`internal/scaffold/dropped_internal_test.go`): "a trailing
  token in parens" rules out never matching a trailing group at all; "no parens at all" rules
  out matching without one; "whitespace inside the parens" rules out accepting a token
  containing whitespace; "parens in the middle" rules out matching anywhere but the string's
  own tail. "Empty parens" pins the contract (untagged) rather than ruling out a mutation of
  its own: zero captured characters return "" the same way whichever exclusion set
  `trailingTagRe`'s character class names, so no mutation of that class discriminates this
  case specifically.
- `Test_dropExcerpt` (`internal/cli/finish_dropped_internal_test.go`): "exactly eighty runes"
  rules out cutting at or under the limit; "eighty-one runes" rules out never cutting (an
  off-by-one on the boundary); "multibyte text" rules out counting bytes instead of runes.
- `Test_Entries_recognizes_every_column_zero_marker`
  (`internal/platform/markdown/entries_test.go`): each of the three marker grammars rules out
  `entryItemRe` recognizing only one of "- ", "* " or "N. " instead of all three.
- `Test_Entries_excludes_lines_that_are_not_column_zero_items`
  (`internal/platform/markdown/entries_test.go`): "a paragraph line" rules out treating any
  non-blank line as an item; "an indented list item" rules out dropping entryItemRe's
  column-0 anchor.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Entry scanner lives in `internal/platform/markdown`; diff/tag/rule/`DroppedEntry` live in `internal/scaffold/dropped.go`. Reason: the scanner needs `sectionSpan`/`fenceState`, which are unexported, and `scaffold` is the only consumer of the rest (`assemble` never imports it).
- The diff is **pooled**: old entries come from all four state headings and are sorted by old line; new entries come from the new body's four state headings; the two are matched as one multiset. Reason: SCENARIO-05 needs a move between state headings to count as no drop, and D2's "surplus = last occurrences in old-file order" (SCENARIO-07) falls out of this. A per-heading diff breaks both.
- Line numbers are whole-body 1-based over the **old** body, split on `"\n"`. That keeps CRLF line numbers correct (SCENARIO-08) (D6).
- `DroppedEntry.Heading` is **display text**, with the leading `#` run and its space stripped (`## Traps` → `Traps`). The text row and SCENARIO-04's JSON `heading` both use it. Rule classification compares the *configured* heading against `cfg.StateHeadings.OpenDebts`, never the display text.
- `DroppedEntry.Tag` is `""` for untagged. The cli maps that to `untagged` in text and SCENARIO-04 maps it to JSON `null`.
- The detail string is built by one cli helper. SCENARIO-04's JSON `detail` must call it and not rebuild the string.
- Drops are computed only on the `refinishWrite` path, just before `applyFinishWrites`. The computation is pure and has no error path. The R11 no-op returns a non-nil empty `Dropped`.

**Left unbuilt** — named so nobody assumes it exists:
- `dropped_entries` in `finishDocument` / the `jsonFieldsParagraph` entry — SCENARIO-04.
- `finishLong` drop-reporting paragraph — SCENARIO-10.
- Continuation lines and indented sub-items in the scanner — SCENARIO-07. S01 recognises only single-line column-0 items.
- Empty/duplicate configured-heading guard (D7) — SCENARIO-08.
- Tests asserting that a refusal prints no rows or JSON — SCENARIO-09.
- Open debts line tag in the brief Default profile (Product Verdict item 4) — SCENARIO-02.

**Traps** — things that look right and are not:
- `markdown.Section(body, "")` / `sectionSpan(lines, "")` match the first blank line. An empty configured heading would scan a random region. S08 must guard it explicitly.
- `sectionSpan` anchors on the **first** line equal to the heading, so a duplicated heading silently reads only the first section. D7 says it contributes nothing, so S08 must detect the duplicate.
- `checklistItemRe` in `checklist.go` is not the entry grammar: it accepts indentation and matches only `- [ ]`. Reusing it would count indented sub-items as entries.
- SCENARIO-02 is expected to be **green on arrival** for the rule and for `untagged` (both built here). Its real content is the end-to-end open-debts row plus Product Verdict item 4. Do not manufacture a red.
- S01's scanner treats the line after an item as unrelated only because continuations are not built yet. SCENARIO-07 folds such lines into the parent entry, so fixtures must not rely on a line directly under an item being ignored.
- CR stripping and whitespace collapse are built in Step 11 but not pinned by any S01 assertion. SCENARIO-03 and SCENARIO-08 own the red for them.
- A drop fixture must keep at least one old entry. With nothing kept, a diff that ignores the new body still passes.
