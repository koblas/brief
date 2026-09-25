# dropped-entries — current state

Scenarios complete: SCENARIO-01..09. Last updated by SCENARIO-09.

## Binding decisions

- Entry scanner (`markdown.Entries`) lives in `internal/platform/markdown`; the diff, tag, rule
  and `DroppedEntry` (`internal/scaffold/dropped.go`) live in `internal/scaffold` — `scaffold`
  is the only consumer (SCENARIO-01).
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
  text; `dropRuleFor` compares the **configured** heading string, never the display text. Drops
  are computed only on `FinishFS`'s `refinishWrite` path, just before `applyFinishWrites` —
  never on `refinishStateDiverged`'s refusal, and `refinishNoop` returns its own literal
  `Dropped: []DroppedEntry{}` rather than computing anything (SCENARIO-01/09).
- `cli`'s row detail (`"dropped from <heading>, tagged|untagged <tag>: <excerpt>"`) is built by
  one helper (`dropDetail`/`dropExcerpt`, cut at 80 runes, in `internal/cli/finish_dropped.go`),
  reused verbatim by `finishDroppedEntries` for JSON's `detail` field (SCENARIO-04).
  `finishDocument.DroppedEntries` is the struct's **last** field, never `omitempty`. **Any new
  `--json` top-level field must update `jsonFieldsParagraph`'s call in the same change.**
- `droppedEntries` scans only `scannableHeadings(headings)`, never raw `headings.Ordered()`: an
  empty or duplicate-valued configured heading contributes zero entries (D7) — the *only*
  guard, since `checkArgumentHeadings` (`markdown.Section(body, "")` matches the first blank
  line) does not reject one and is itself **unreachable through `brief finish`/`cli.Run`**:
  `config.Resolve` already refuses an empty or duplicate `state-headings.*` value first. Tested
  by constructing `config.Config` directly, below the CLI (SCENARIO-08).
- `normalizeEntryText`'s `ReplaceAll(raw, "\r", "")` only matters for a `\r` strictly between
  two non-whitespace characters — `strings.Fields` already splits a space-adjacent `\r` like a
  space (SCENARIO-08).
- D5 is enforced at **two independent points**, neither substituting for the other:
  `FinishFS`'s refinish switch never computes `dropped` before a refusal branch returns
  (Server-level state-diverged table), and `runFinish`'s `srv.Finish` error branch never
  renders `res` on a non-nil `err` (CLI-level refusal and its `--json` counterpart) (SCENARIO-09).

## Left unbuilt

- `finishLong`'s drop-reporting **prose** paragraph (the JSON field-list part is done) —
  SCENARIO-10.

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
- A CLI-level re-finish test must reuse one `*rwfs.Mem` across every call — a second
  `tree.mem()` call takes a fresh copy of `tree`'s entries and silently discards an earlier
  finish's writes, turning an intended re-finish into a first finish; every `--state`/
  `--handoff` input must be written into `tree` before the one `tree.mem()` call, or the
  missing-input path refuses inside `readSource` before `srv.Finish` runs at all (SCENARIO-08/09).
- `Test_finish_json_refusal_is_unchanged_mem` (pre-SCENARIO-09) stays valid but weak alone: it
  refuses on `ErrNoSuchStep` before `state` is read, pinning only the error document's key
  *shape*. SCENARIO-09's state-diverged pair plus its handoff-missing control arm is what
  proves D5 against a refusal that would otherwise have had entries to report.

## Open debts

- **Mid-write gap — unowned, dies unless re-opened.** Drops are computed before the write; if
  the state rename lands and a later write in the same call fails, R14a suppresses the rows,
  and a same-argument retry then diffs the new state against itself and reports nothing — one
  entry can vanish unremarked. Rare; not closed by this feature (specification.md's *Decisions*).
