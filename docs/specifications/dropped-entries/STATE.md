# dropped-entries — current state

Scenarios complete: SCENARIO-01. Last updated by SCENARIO-01.

## Binding decisions

- Entry scanner (`markdown.Entries`, new `entries.go`) lives in `internal/platform/markdown`;
  the diff, tag, rule and `DroppedEntry` (new `scaffold/dropped.go`) live in `internal/scaffold`
  — `scaffold` is the only consumer, `assemble` needs none of it (SCENARIO-01).
- The diff is **pooled**: every old entry across all four state headings and every new entry
  across all four are matched as one multiset on normalized text, sorted by **old-file line**,
  never by heading-processing order — required for SCENARIO-05's "moved between state headings
  is not a drop" and SCENARIO-07's "duplicate dropped once, at its later occurrence"; already
  exercised here by a fixture whose file order reverses the configured heading order
  (SCENARIO-01).
- `DroppedEntry.Line` is whole-body 1-based over the **old** body only. `.Heading` is display
  text (leading `#` run + one space stripped); rule classification (`dropRuleFor`) compares
  the **configured** heading string against `cfg.StateHeadings.OpenDebts`, never the display
  text. `.Tag` is `""` untagged (SCENARIO-01).
- Drops are computed only on `FinishFS`'s `refinishWrite` path, just before
  `applyFinishWrites` — pure, no error path. The R11 no-op returns a non-nil, empty
  `FinishResult.Dropped` (SCENARIO-01).
- `cli`'s row detail (`"dropped from <heading>, tagged|untagged <tag>: <excerpt>"`) is built by
  one helper (`dropDetail`/`dropExcerpt` in `internal/cli/finish_dropped.go`) so SCENARIO-04's
  JSON `detail` field can reuse it verbatim instead of rebuilding the string.
- Finding shape amended to the live `<SEVERITY>  <path>[:<line>]  <detail>` in three places
  (R14a and the Default profile's Findings paragraph in
  `docs/specifications/brief/specification.md`, plus the STATE.md decision in
  `docs/specifications/brief/STATE.md`), each now also noting `check` groups under a feature
  header while `finish` prints bare (SCENARIO-01, Product Verdict item 2).

## Left unbuilt

- `dropped_entries` in `finishDocument`/`jsonFieldsParagraph` — SCENARIO-04.
- `finishLong`'s drop-reporting `--help` paragraph — SCENARIO-10.
- Continuation lines and indented sub-items in `markdown.Entries` — SCENARIO-07; today's
  scanner recognizes only single-line column-0 items, and an indented line is simply ignored.
- The empty/duplicate configured-heading guard — SCENARIO-08.
- Assertions that a refusal or usage error prints no rows/no `dropped_entries` — SCENARIO-09.
- The Open debts line's own trailing tag in the Default profile (Product Verdict item 4) —
  SCENARIO-02, expected green on arrival for the rule and for `untagged` (both built by
  SCENARIO-01); don't manufacture a red there.

## Traps

- `markdown.Section(body, "")` / `sectionSpan(lines, "")` match the first blank line — an
  empty configured heading would scan a random region; SCENARIO-08's guard must trigger on
  this explicitly, not rely on `Entries` failing closed.
- `sectionSpan` anchors on the **first** line equal to heading, so a duplicated heading reads
  only its first section; SCENARIO-08 must detect the duplicate itself, since scanning alone
  reports nothing wrong.
- `checklistItemRe` (`markdown/checklist.go`) is not the entry grammar: it accepts leading
  indentation and matches only `- [ ]`/`- [x]`. `entryItemRe` is deliberately separate.
- CR stripping and whitespace collapse are built (`normalizeEntryText`) but unpinned by any
  SCENARIO-01 assertion — SCENARIO-03 and SCENARIO-08 own that red.
- A drop fixture must keep at least one old entry: with nothing kept, a diff that ignores the
  new body entirely still passes.
- SCENARIO-07's continuation-line folding will change today's "a line directly under an item
  is ignored" behavior — a fixture relying on that must not survive unexamined.

## Open debts

- **Mid-write gap — unowned, dies unless re-opened.** Drops are computed before the write; if
  the state rename lands and a later write in the same call fails, R14a suppresses the rows on
  that failed run, and a same-argument retry then diffs the new state against itself and
  reports nothing — one entry can vanish unremarked. Rare; not closed by this feature
  (recorded in specification.md's own *Decisions*).
