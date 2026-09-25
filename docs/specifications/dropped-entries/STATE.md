# dropped-entries — current state

Scenarios complete: SCENARIO-01..10. Feature done. Fix pass 1 closed two MAJORs (overlapping
configured headings, an ignored stdout write error) then corrected its own fixes: a
nested-subsection mis-attribution and a silently-unprintable error. Fix pass 2 closed two
mutation-confirmed under-tested MAJORs: `thematicBreakRe`'s marker/indentation coverage and
`poolOccurrences`' new-body dedup.

## Binding decisions

- Entry scanner (`markdown.Entries`, `markdown.HeadingLine`) lives in
  `internal/platform/markdown`; diff/tag/rule/`DroppedEntry` (`internal/scaffold/dropped.go`)
  live in `internal/scaffold` — its only consumer.
- The diff is **pooled**, on *both* the old and the new body: entries across all four state
  headings are matched as one multiset on normalized text, sorted by **old-file line**. Surplus
  old occurrences of a duplicated text — the tail of `idxs` in old-file line order — are
  dropped (`idxs[len(idxs)-surplus:]`). `poolOccurrences` (`attributeByLine` + `sortedByLine`)
  first keeps each physical line once: nested configured headings, or one with no leading `#`
  (section never terminates), let more than one heading's own scan reach the same line. That
  line goes to whichever heading whose scan *actually reached it* has the greatest
  `markdown.HeadingLine` — never "nearest by line number" nor "last scanned" (config order need
  not match nesting order). Getting this wrong has false-dropped a move, mis-attributed an
  unconfigured subsection's drop, and double-counted the *new* body's own nested overlap,
  masking a real drop as a false no-op.
- `entryItemText` strips only `- `/`* `/`N. `, never a `[ ]`/`[x]` checkbox, so
  reword/re-tag/re-tick each yield a different `normalizeEntryText` string. A thematic break —
  spaced or compact (no interior spaces), any of `-`/`_`/`*`, indented up to three spaces —
  reads like a bullet marker but is checked first (`thematicBreakRe`) and is never an entry.
- `markdown.Entries` folds continuation lines: every line after a column-0 item that is not
  blank, a column-0 item, a heading, a thematic break, or a fence delimiter joins the item's
  `Text` (a thematic break ends a fold too, per CommonMark, rather than being swallowed as
  text). `Entry.Line` stays fixed at the item's own first line; a deeper heading ends
  continuation but not the section.
- `DroppedEntry.Line` is whole-body 1-based over the **old** body only. `.Heading` is display
  text; `dropRuleFor` compares the **configured** heading string. Every drop's severity is the
  constant `scaffold.SeverityWarn`, used directly at both `cli` call sites (no per-`DropRule`
  method — the value never varies by rule). Drops are computed only on `FinishFS`'s
  `refinishWrite` path, before `applyFinishWrites`; `refinishNoop` returns its own literal
  `Dropped: []DroppedEntry{}`. D5 holds at two points: the refinish switch, and `runFinish`'s
  error branch never rendering `res` on a non-nil `err`.
- `cli`'s row detail is built by one helper (`dropDetail`/`dropExcerpt`, cut at 80 runes) and
  one row writer (`writeDroppedRows`, both in `internal/cli/finish_dropped.go`), reused
  verbatim by `finishDroppedEntries` for JSON's `detail`. `writeDroppedRows` stops at the
  first failed stdout write; `runFinish` reports it via `out.refusal(...)`, never a bare
  `fmt.Errorf` (`main.go` prints nothing itself). `finishDocument.DroppedEntries` is the last
  field, never `omitempty`; a new `--json` field must update `jsonFieldsParagraph` too.
- `droppedEntries` scans only `scannableHeadings(headings)`, computed once per call: an empty
  or duplicate-valued configured heading contributes zero entries, since `config.Resolve`
  already refuses that before `brief finish`/`cli.Run` can reach it. `finishLong` carries the
  drop-reporting paragraph pinned byte-identical in two goldens (`help_test.go`, `help_json_test.go`).

## Left unbuilt

None — all ten scenarios implemented.

## Traps

- `checklistItemRe` (`markdown/checklist.go`) is not the entry grammar; `entryItemRe` is
  deliberately column-0-only. `dropExcerpt` cuts at 80 runes — a fixture proving folding or
  dedup by its text content must stay under that. A CLI-level re-finish test must reuse one
  `*rwfs.Mem` across every call — a second `tree.mem()` call discards earlier writes.
- `markdown.HeadingLine`, `Section` and `Entries` match a configured heading by **literal
  string equality**, not ATX shape — no leading `#` still "matches" a plain body line, and its
  own section (`sectionSpan`) never terminates. A returned `fmt.Errorf` from an `internal/cli`
  command is invisible unless routed through `reporter.refusal`/`reporter.document` — `check`,
  `status` and `start` all return one bare today, a wider pre-existing bug this feature avoids
  but does not close.

## Open debts

- **Mid-write gap — unowned.** A later write failure after drops are computed suppresses the
  rows, and a same-argument retry then diffs the new state against itself and reports
  nothing. Rare; not closed by this feature.
- **No-`#` heading over-scans past later sections — unowned.** Its section never terminates
  (`sectionSpan`/`headingLevelOf` 0), so its scan can sweep an entry moved to a genuinely
  non-state section into its own new-body count, masking a real "moved out" drop; root cause
  is shared by `Start`/`check`, too broad to fix here.
- **Unowned, no constructible failure:** an unterminated fence in the OLD body hides later
  sections; the 80-rune excerpt cut can split a grapheme cluster or leave a trailing space
  before "…"; `headingDisplay` could move to `markdown`; misc. MINOR/NIT polish items.
