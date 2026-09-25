# dropped-entries — current state

Scenarios complete: SCENARIO-01..08. Last updated by SCENARIO-08.

## Binding decisions

- Entry scanner (`markdown.Entries`, `internal/platform/markdown/entries.go`) lives in
  `internal/platform/markdown`; the diff, tag, rule and `DroppedEntry`
  (`internal/scaffold/dropped.go`) live in `internal/scaffold` — `scaffold` is the only
  consumer (SCENARIO-01).
- The diff is **pooled**: every old entry across all four state headings and every new entry
  across all four are matched as one multiset on normalized text, sorted by **old-file line**,
  never by heading-processing order (SCENARIO-05). Surplus old occurrences of a duplicated text
  — the tail of `idxs` in old-file line order — are dropped, i.e. the *later* occurrence
  (`idxs[len(idxs)-surplus:]`, mutation-guarded by SCENARIO-07's CLI-level proof; do not
  "simplify" that slice expression without re-running it).
- `entryItemText` strips only `- `/`* `/`N. `, never a `[ ]`/`[x]` checkbox, so
  reword/re-tag/re-tick each already yield a different `normalizeEntryText` string
  (SCENARIO-01/05/06); do not add a "strip the checkbox/tag" step as a "cleanup" later.
- `markdown.Entries` folds continuation lines (SCENARIO-07, D1): every line after a column-0
  item that is not itself blank, a column-0 item, a heading (any level), or a fence delimiter
  joins the item's `Text`, whatever its own indentation. `Entry.Line` stays fixed at the item's
  own first line (D6). A fence ends the entry in progress and starts the fence-skip scan; a
  heading deeper than the section's own ends an entry's continuation but not the *section*.
- `DroppedEntry.Line` is whole-body 1-based over the **old** body only. `.Heading` is display
  text; `dropRuleFor` compares the **configured** heading string, never the display text.
  Drops are computed only on `FinishFS`'s `refinishWrite` path, just before
  `applyFinishWrites` — pure, no error path; the R11 no-op returns a non-nil, empty
  `FinishResult.Dropped` (SCENARIO-01).
- `cli`'s row detail (`"dropped from <heading>, tagged|untagged <tag>: <excerpt>"`) is built by
  one helper (`dropDetail`/`dropExcerpt`, cut at 80 runes, in `internal/cli/finish_dropped.go`),
  reused verbatim by `finishDroppedEntries` for JSON's `detail` field (SCENARIO-04).
  `finishDocument.DroppedEntries` is the struct's **last** field, never `omitempty`. **Any new
  `--json` top-level field must update `jsonFieldsParagraph`'s call in the same change.**
- `droppedEntries` (`internal/scaffold/dropped.go`) scans only `scannableHeadings(headings)`,
  never raw `headings.Ordered()`: an empty or duplicate-valued configured heading contributes
  zero entries, from every position it appears at (D7) — do not special-case "keep the first of
  a duplicate pair" later (SCENARIO-08).
- The empty/duplicate-heading guard is real production code in `scaffold` but **unreachable
  through `brief finish`/`cli.Run`**: `config.Resolve` already refuses an empty or
  pairwise-duplicate `state-headings.*` value before a `scaffold.Server` is built. It is tested
  by constructing `config.Config` directly, below the CLI (SCENARIO-08).
- `normalizeEntryText`'s explicit `ReplaceAll(raw, "\r", "")` is load-bearing only for an
  interior `\r` a continuation line leaves behind (SCENARIO-07 folding); a trailing-CRLF line
  never reaches it with a `\r` still present, since `trimEOL`/`TrimSpace` strip it first
  (SCENARIO-08).

## Left unbuilt

- `finishLong`'s drop-reporting **prose** paragraph (the JSON field-list part is done) —
  SCENARIO-10.
- A refusal-carries-no-`dropped_entries` proof for a finish that **would** drop entries —
  SCENARIO-09. `Test_finish_json_refusal_is_unchanged_mem` refuses before any diff runs, so it
  pins only the error document's key *shape*, not D5's actual claim.

## Traps

- `checklistItemRe` (`markdown/checklist.go`) is not the entry grammar: it accepts leading
  indentation and matches only `- [ ]`/`- [x]`. `entryItemRe` is deliberately column-0-only —
  never relax it to fold an indented item into its own `Entry` (SCENARIO-07 mutation-guards
  this); sub-items must stay continuation text of their parent.
- A drop fixture must keep at least one old entry: with nothing kept, a diff that ignores the
  new body entirely still passes. Omitting any of the four configured headings from a new body
  refuses with `ErrMissingStateHeading`, printing no rows (SCENARIO-03/05).
- `dropExcerpt` cuts at 80 runes: a fixture meant to prove folding or dedup by its text content
  must stay under that, or the cut — not the behavior under test — is what the assertion
  reflects.

## Open debts

- **Mid-write gap — unowned, dies unless re-opened.** Drops are computed before the write; if
  the state rename lands and a later write in the same call fails, R14a suppresses the rows on
  that failed run, and a same-argument retry then diffs the new state against itself and
  reports nothing — one entry can vanish unremarked. Rare; not closed by this feature
  (recorded in specification.md's own *Decisions*).
