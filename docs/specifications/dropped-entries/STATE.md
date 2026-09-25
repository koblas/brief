# dropped-entries — current state

Scenarios complete: SCENARIO-01, SCENARIO-02, SCENARIO-03. Last updated by SCENARIO-03.

## Binding decisions

- Entry scanner (`markdown.Entries`, `internal/platform/markdown/entries.go`) lives in
  `internal/platform/markdown`; the diff, tag, rule and `DroppedEntry`
  (`internal/scaffold/dropped.go`) live in `internal/scaffold` — `scaffold` is the only
  consumer, `assemble` needs none of it (SCENARIO-01).
- The diff is **pooled**: every old entry across all four state headings and every new entry
  across all four are matched as one multiset on normalized text, sorted by **old-file line**,
  never by heading-processing order — required for SCENARIO-05's "moved between state headings
  is not a drop" and SCENARIO-07's "duplicate dropped once, at its later occurrence"; already
  exercised by a fixture whose file order reverses the configured heading order (SCENARIO-01).
- `DroppedEntry.Line` is whole-body 1-based over the **old** body only. `.Heading` is display
  text (leading `#` run + one space stripped); `dropRuleFor` compares the **configured**
  heading string against `cfg.StateHeadings.OpenDebts`, never the display text. `.Tag` is `""`
  untagged (SCENARIO-01). SCENARIO-02 proved the heading display and `dropDetail`'s untagged
  branch against `## Open debts` end-to-end through the CLI with no production edit needed.
  `dropRuleFor`'s `dropped-debt` classification stays proven only at Server level (text mode
  never renders `Rule`) until SCENARIO-04's JSON field.
- `normalizeEntryText`'s interior whitespace-run collapse (a doubled space or a tab collapsed
  to one space) is proven end-to-end through the CLI as of SCENARIO-03, with no production
  edit needed — SCENARIO-01 already built it generically. Mutation-verified: swapping the
  collapse for a plain `TrimSpace` reddens exactly that test (false-positive drops on the
  doubled-space and tab entries, old-file lines 3 and 9). The same fixture's trailing-space
  entry is covered but not discriminating — `entryItemText`'s `trimEOL` already strips it
  before `normalizeEntryText` runs, so neither mutation touches it (see Traps). CR-strip is
  still unpinned — SCENARIO-08.
- Drops are computed only on `FinishFS`'s `refinishWrite` path, just before
  `applyFinishWrites` — pure, no error path. The R11 no-op returns a non-nil, empty
  `FinishResult.Dropped` (SCENARIO-01).
- `cli`'s row detail (`"dropped from <heading>, tagged|untagged <tag>: <excerpt>"`) is built by
  one helper (`dropDetail`/`dropExcerpt` in `internal/cli/finish_dropped.go`) so SCENARIO-04's
  JSON `detail` field can reuse it verbatim instead of rebuilding the string.
- Finding shape amended to the live `<SEVERITY>  <path>[:<line>]  <detail>` in three places
  (R14a and the Default profile's Findings paragraph in
  `docs/specifications/brief/specification.md`, plus the STATE.md decision in
  `docs/specifications/brief/STATE.md`), each also noting `check` groups under a feature header
  while `finish` prints bare (SCENARIO-01, Product Verdict item 2).

## Left unbuilt

- `dropped_entries` in `finishDocument`/`jsonFieldsParagraph`, including the JSON `rule`
  field — SCENARIO-04. That element is the first place `"rule":"dropped-debt"` is provable
  end-to-end through the CLI (text mode never renders `Rule`); SCENARIO-04's fixture set must
  include an Open-debts drop.
- `finishLong`'s drop-reporting `--help` paragraph — SCENARIO-10.
- Continuation lines/indented sub-items in `markdown.Entries` (SCENARIO-07); the
  empty/duplicate heading guard (SCENARIO-08); refusal/no-row assertions (SCENARIO-09);
  CR-strip's own CLI-level proof, a `\r\n` fixture (SCENARIO-08).

## Traps

- `markdown.Section(body, "")` / `sectionSpan(lines, "")` match the first blank line — an
  empty configured heading would scan a random region; SCENARIO-08's guard must trigger on
  this explicitly, not rely on `Entries` failing closed.
- `sectionSpan` anchors on the **first** line equal to heading, so a duplicated heading reads
  only its first section; SCENARIO-08 must detect the duplicate itself, since scanning alone
  reports nothing wrong.
- `checklistItemRe` (`markdown/checklist.go`) is not the entry grammar: it accepts leading
  indentation and matches only `- [ ]`/`- [x]`. `entryItemRe` is deliberately separate.
- A drop fixture must keep at least one old entry: with nothing kept, a diff that ignores the
  new body entirely still passes.
- SCENARIO-07's continuation-line folding will change today's "a line directly under an item
  is ignored" behavior — a fixture relying on that must not survive unexamined.
- `entryItemText` trims trailing `" \t\r"` before `normalizeEntryText` ever runs, so a
  trailing-space-only fixture doesn't exercise the collapse mutation — it's included for
  delta coverage, not as proof. A trailing-CRLF fixture likely can't redden SCENARIO-08's own
  CR-strip mutation either, for the same reason: only an interior bare CR is live for that one.

## Open debts

- **Mid-write gap — unowned, dies unless re-opened.** Drops are computed before the write; if
  the state rename lands and a later write in the same call fails, R14a suppresses the rows on
  that failed run, and a same-argument retry then diffs the new state against itself and
  reports nothing — one entry can vanish unremarked. Rare; not closed by this feature
  (recorded in specification.md's own *Decisions*).
