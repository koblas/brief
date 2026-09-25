# dropped-entries — current state

Scenarios complete: SCENARIO-01..06. Last updated by SCENARIO-06.

## Binding decisions

- Entry scanner (`markdown.Entries`, `internal/platform/markdown/entries.go`) lives in
  `internal/platform/markdown`; the diff, tag, rule and `DroppedEntry`
  (`internal/scaffold/dropped.go`) live in `internal/scaffold` — `scaffold` is the only
  consumer, `assemble` needs none of it (SCENARIO-01).
- The diff is **pooled**: every old entry across all four state headings and every new entry
  across all four are matched as one multiset on normalized text, sorted by **old-file line**,
  never by heading-processing order — proven end-to-end by SCENARIO-05 (heading-keyed mutation
  reddens only the moved-between-headings case); still needed for SCENARIO-07's "duplicate
  dropped once, at its later occurrence". Reword/re-tag/re-tick need no dedicated logic:
  `entryItemText` strips only `- `/`* `/`N. `, never a `[ ]`/`[x]` checkbox, so any of the
  three already yields a different `normalizeEntryText` string (SCENARIO-01/05/06).
- `DroppedEntry.Line` is whole-body 1-based over the **old** body only. `.Heading` is display
  text; `dropRuleFor` compares the **configured** heading string against
  `cfg.StateHeadings.OpenDebts`, never the display text. `.Tag` is `""` untagged (SCENARIO-01).
- Drops are computed only on `FinishFS`'s `refinishWrite` path, just before
  `applyFinishWrites` — pure, no error path. The R11 no-op returns a non-nil, empty
  `FinishResult.Dropped` (SCENARIO-01).
- `normalizeEntryText`'s interior whitespace-run collapse (a doubled space or a tab collapsed
  to one space) is proven end-to-end: swapping it for a plain `TrimSpace` reddens exactly that
  test (false-positive drops on the doubled-space/tab entries) (SCENARIO-01/03).
- `cli`'s row detail (`"dropped from <heading>, tagged|untagged <tag>: <excerpt>"`) is built by
  one helper (`dropDetail`/`dropExcerpt` in `internal/cli/finish_dropped.go`), reused verbatim
  by `finishDroppedEntries` for JSON's `detail` field (SCENARIO-04).
- `finishDroppedJSON`/`finishDroppedEntries` (`internal/cli/finish_dropped.go`) is
  `dropped_entries`'s element shape: `severity/rule/path/line/detail/heading/tag/text`, `Tag`
  `*string` nil for untagged. `finishDocument.DroppedEntries` is the struct's **last** field, a
  sized non-nil slice — never `omitempty` (SCENARIO-04).
- **Any new `--json` top-level field must update `jsonFieldsParagraph`'s call in the same
  change** — `Test_every_command_help_names_its_json_documents_top_level_fields` requires every
  non-header key appear as a whole word there. `finishLong` lists `dropped_entries`;
  SCENARIO-10 still owns the drop-reporting *prose* paragraph before it (SCENARIO-04).
- Finding shape amended to the live `<SEVERITY> <path>[:<line>] <detail>` in brief's own specs (SCENARIO-01).

## Left unbuilt

- `finishLong`'s drop-reporting **prose** paragraph (the JSON field-list part is done) —
  SCENARIO-10.
- Continuation lines/indented sub-items in `markdown.Entries` (SCENARIO-07); the
  empty/duplicate heading guard (SCENARIO-08); CR-strip's own CLI-level proof, a `\r\n`
  fixture (SCENARIO-08).
- A refusal-carries-no-`dropped_entries` proof for a finish that **would** drop entries —
  SCENARIO-09. `Test_finish_json_refusal_is_unchanged_mem` refuses before any diff runs, so it
  pins only the error document's key *shape*, not D5's actual claim.

## Traps

- `markdown.Section(body, "")` / `sectionSpan(lines, "")` match the first blank line — an
  empty configured heading would scan a random region; SCENARIO-08's guard must trigger on
  this explicitly. `sectionSpan` also anchors on the **first** line equal to heading, so a
  duplicated heading reads only its first section — SCENARIO-08 must detect the duplicate too.
- `checklistItemRe` (`markdown/checklist.go`) is not the entry grammar: it accepts leading
  indentation and matches only `- [ ]`/`- [x]`. `entryItemRe` is deliberately separate.
- A drop fixture must keep at least one old entry: with nothing kept, a diff that ignores the
  new body entirely still passes. Omitting any of the four configured headings from a new body
  refuses with `ErrMissingStateHeading`, printing no rows — indistinguishable from zero drops
  unless the test asserts `err` nil and exact stderr (SCENARIO-03/05).
- SCENARIO-07's continuation-line folding will change today's "a line directly under an item
  is ignored" behavior — a fixture relying on that must not survive unexamined. `entryItemText`
  trims trailing `" \t\r"` before `normalizeEntryText` runs, so a trailing-space-only fixture
  doesn't exercise the collapse mutation, and a trailing-CRLF fixture likely can't redden
  SCENARIO-08's own CR-strip mutation either.
- Do not add a "strip the checkbox marker" or "strip the tag before matching" step to
  `normalizeEntryText`/`droppedEntries` as a "cleanup" — either silently breaks a
  reworded/re-tagged/re-ticked row. Mutate the **identity key** (`groups`/`newCounts`) alone to
  prove it — mutating `normalizeEntryText` also blanks `DroppedEntry.Text` (one function feeds
  both), reddening `reworded`/`re-tagged` too, not just the case named (SCENARIO-06).

## Open debts

- **Mid-write gap — unowned, dies unless re-opened.** Drops are computed before the write; if
  the state rename lands and a later write in the same call fails, R14a suppresses the rows on
  that failed run, and a same-argument retry then diffs the new state against itself and
  reports nothing — one entry can vanish unremarked. Rare; not closed by this feature
  (recorded in specification.md's own *Decisions*).
