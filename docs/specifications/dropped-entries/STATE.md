# dropped-entries — current state

Scenarios complete: SCENARIO-01..10. Feature done. Fix pass 1 closed two MAJORs
(overlapping configured headings, an ignored stdout write error). Last updated by fix pass 1.

## Binding decisions

- Entry scanner (`markdown.Entries`, `markdown.HeadingLine`) lives in
  `internal/platform/markdown`; the diff, tag, rule and `DroppedEntry`
  (`internal/scaffold/dropped.go`) live in `internal/scaffold` — its only consumer.
- The diff is **pooled**: every old entry across all four state headings and every new entry
  across all four are matched as one multiset on normalized text, sorted by **old-file line**.
  Surplus old occurrences of a duplicated text — the tail of `idxs` in old-file line order —
  are dropped (`idxs[len(idxs)-surplus:]`). Before pooling, each physical line (old and new
  body) is attributed to its one **nearest-enclosing** configured heading
  (`poolOccurrences`/`nearestHeading`: the scannable heading with the greatest
  `markdown.HeadingLine` ≤ the entry's line) and deduped by `Entry.Line`, never to whichever
  heading's own scan reached it first. Two configured headings can nest (e.g. `open-debts`
  one ATX level deeper than `traps`), and a heading configured with no leading `#` never
  terminates its own section; either lets more than one scan reach the same line, which
  pre-fix false-dropped a move and double-reported a real drop (fix pass 1).
- `entryItemText` strips only `- `/`* `/`N. `, never a `[ ]`/`[x]` checkbox, so
  reword/re-tag/re-tick each yield a different `normalizeEntryText` string. A column-0
  thematic break (`* * *`, `- - -`, `***`, `---`) is checked first and is never an entry, even
  though `* `/`- ` alone reads as a bullet marker (`thematicBreakRe`, fix pass 1).
- `markdown.Entries` folds continuation lines: every line after a column-0 item that is not
  blank, a column-0 item, a heading, or a fence delimiter joins the item's `Text`. `Entry.Line`
  stays fixed at the item's own first line. A fence ends the entry in progress; a deeper
  heading ends continuation but not the section.
- `DroppedEntry.Line` is whole-body 1-based over the **old** body only. `.Heading` is display
  text; `dropRuleFor` compares the **configured** heading string. `DropRule.Severity()`
  (always `SeverityWarn`) is decided in `scaffold`, mirroring
  `assemble.Finding`/`doctor.Check`'s own Severity shape — `cli` only stringifies it (fix pass
  1). Drops are computed only on `FinishFS`'s `refinishWrite` path, just before
  `applyFinishWrites`; `refinishNoop` returns its own literal `Dropped: []DroppedEntry{}`. D5
  holds at two points: that refinish switch never computes `dropped` before a refusal
  returns, and `runFinish`'s error branch never renders `res` on a non-nil `err`.
- `cli`'s row detail is built by one helper (`dropDetail`/`dropExcerpt`, cut at 80 runes) and
  one row writer (`writeDroppedRows`, both in `internal/cli/finish_dropped.go`), reused
  verbatim by `finishDroppedEntries` for JSON's `detail`. `writeDroppedRows` stops at the
  first failed stdout write and `runFinish` returns an internal error (exit 1) instead of
  printing the success line — the state is already replaced by then (fix pass 1).
  `finishDocument.DroppedEntries` is the last field, never `omitempty`; a new `--json` field
  must update `jsonFieldsParagraph` too.
- `droppedEntries` scans only `scannableHeadings(headings)`, computed once per call: an empty
  or duplicate-valued configured heading contributes zero entries — the only guard, since
  `config.Resolve` already refuses that before `brief finish`/`cli.Run` can reach it.
- `finishLong` carries the drop-reporting paragraph between the flag-body prose and
  `jsonFieldsParagraph(...)`'s output — pinned byte-identical in two independent goldens
  (`help_test.go`, `help_json_test.go`).

## Left unbuilt

None — all ten scenarios implemented; fix pass 1 closed both its MAJORs.

## Traps

- `checklistItemRe` (`markdown/checklist.go`) is not the entry grammar: it accepts leading
  indentation and matches only `- [ ]`/`- [x]`. `entryItemRe` is deliberately column-0-only;
  sub-items stay continuation text of their parent.
- `dropExcerpt` cuts at 80 runes: a fixture proving folding or dedup by its text content must
  stay under that.
- A CLI-level re-finish test must reuse one `*rwfs.Mem` across every call — a second
  `tree.mem()` call discards an earlier finish's writes.
- `markdown.HeadingLine`, `Section` and `Entries` all match a configured heading by **literal
  string equality**, not ATX shape — a heading with no leading `#` still "matches" a plain
  body line. `poolOccurrences`' nearest-enclosing fix depends on `HeadingLine` staying
  consistent with `Entries`' own anchor search.

## Open debts

- **Mid-write gap — unowned, dies unless re-opened.** Drops are computed before the write; a
  later write failure in the same call suppresses the rows, and a same-argument retry then
  diffs the new state against itself and reports nothing. Rare; not closed by this feature.
- **Unowned, dies unless re-opened:** an unterminated fence in the OLD body hides later
  sections; the 80-rune excerpt cut can split a grapheme cluster or leave a trailing space
  before "…"; `headingDisplay` could move to `markdown`; `Entries`' two-loop extraction;
  white-box file header comments. Deferred at fix pass 1 as MINOR/NIT, no constructible
  failure.
