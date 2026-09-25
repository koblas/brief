# Specification: dropped-entries

**Status:** APPROVED 2026-09-24. Implements R9 of `docs/specifications/brief/specification.md`.

## Intent & Goal

**Primary Goal**: `brief finish` reports every state entry its replacement body removes, so no
entry — above all an "unowned — dies unless re-opened" debt — vanishes unremarked. Deletion is
accounted for, not prevented (R9). Today `finish` drops such an entry silently, exit 0.

**Out of Scope**:
- Refusing, or asking to confirm, a deletion. Removing stale entries is the mechanism (R8).
- Judging whether a removal was correct, whether an entry was merely reworded, or who owns a
  debt (R7). No parsing of "unowned" or any other English.
- A `--quiet` flag, a new severity, an own-step exemption.
- Validating a tag against the step pattern.
- Closing the mid-write gap (see *Decisions*, recorded as a debt).

**Business Rules**: see below.

## Business Rules & Invariants

- **D1 — Entry.** An entry is a list item beginning at column 0 (`- `, `* `, or `N. `) inside
  the section of one of the four configured `state-headings`, plus its continuation lines: every
  following non-blank line that is neither a column-0 list item nor a heading (indented sub-items
  belong to their parent). An entry ends at a blank line, the next column-0 item, or a heading.
  Paragraph lines are not entries. List lines inside a fenced code block are not entries.
  Sections other than the four configured headings are not scanned, in either body.
- **D2 — Identity.** An entry's identity is its whitespace-normalized text: lines joined, every
  whitespace run collapsed to one space, trimmed; CR stripped first. A `- [ ]` / `- [x]` marker
  is part of the text. The heading an entry sits under is **not** part of identity. Old and new
  entries are matched as a multiset; for a text appearing more often in the old body than in the
  new, the surplus old occurrences — the last ones in old-file order — are dropped.
- **D3 — Tag.** The trailing `(<token>)` of the normalized text, token non-empty with no
  whitespace. Reported only; never used to match, classify or filter. No tag → `untagged`.
- **D4 — Every drop is reported.** Severity always `WARN`. Rule `dropped-debt` for an entry
  under the configured `open-debts` heading, `dropped-entry` under the other three. No
  exemption for entries tagged to the step being closed.
- **D5 — Findings never on a failed run (R14a).** Any refusal or usage error prints no rows and
  its JSON error document has no `dropped_entries`. Exit 0 whenever rows print.
- **D6 — Line.** The 1-based line in the **old** state file where the entry's first line was.
- **D7 — Empty or duplicate configured heading** contributes no entries (explicit guard:
  `markdown.Section(body, "")` matches the first blank line). A missing heading in the old body
  contributes nothing. An empty old state file yields no drops. A missing old state file is
  refused by `finish` before any drop is computed (pre-existing behavior; exit 1, no rows).

---

## Triage Brief

From `triage`, against this worktree. Binding on the architect.

**Affected surface.**
- `internal/scaffold/finish.go` `FinishFS` — has the old state bytes (`stateBytes`) and the
  incoming `state` body before `applyFinishWrites`. R9 is a post-decision report, **not** part
  of the validation band; it must not disturb R14a's "first thing wrong" ordering.
- `internal/scaffold/finish.go` `FinishResult` — gains the drop list.
- `internal/cli/finish.go` `finishDocument`, `runFinish` — JSON doc and text branch; the text
  branch today prints only the stderr success line.
- `internal/platform/config/config.go` `StateHeadings.Ordered()` — the four headings.

**Prior art.**
- `internal/assemble/check.go` `Finding` and `internal/assemble/render.go` `RenderFindings`
  (`  <SEVERITY>  <path>[:<line>]  <detail>` under a group header) — the live finding shape.
- `internal/cli/check.go` `checkFindingJSON` (`severity/rule/path/line|null/detail`) — the JSON
  element `dropped_entries` extends.
- `internal/platform/markdown/section.go` (`Section`, fence-aware) and `checklist.go`
  (`checklistItemRe`, checkbox items only). No parser exists for plain list items or trailing
  tags — new.
- `scaffold` and `assemble` never import each other; anything shared lives in
  `internal/platform/*`.

**Already exists — do not re-plan:** `StateHeadings.Ordered()`, the fence-aware section scanner,
the `finishDocument` / `checkFindingJSON` JSON patterns, the `FinishFS` validation band and
write order.

**Self-hosting.** The pipeline has not closed its own steps through `brief finish` (later brief
handoffs exceed the 60-line cap), so R9 is exercised only by fixtures, not by this repo's real
state rewrites.

## Product Verdict

**SHIP WITH CHANGES** (`product-vision`, Phase 1). All accepted and folded in:

1. Report every drop; no own-step exemption — R9's "every" governs.
2. Amend the finding shape to the live `<SEVERITY>  <path>[:<line>]  <detail>` in the same
   change, in three places: R14a and the Default profile's **Findings** paragraph in
   `docs/specifications/brief/specification.md` (add: "`check` groups rows under a
   `<feature>  (in flight|complete)` header; `finish` prints them bare"), and the "Findings
   render …" binding decision in `docs/specifications/brief/STATE.md`.
3. JSON field is `dropped_entries` (not `dropped` — `truncated.dropped_bytes` already uses that
   word for a byte count), always present.
4. The Default profile's Open debts line gains a trailing tag:
   `## Open debts  <debt> — <step that must close it, or "unowned — dies unless re-opened"> (SCENARIO-XX)`,
   the tag naming the step that introduced the debt.
5. Record the mid-write gap in *Decisions* as a named, unowned debt.
6. No new severity, no `--quiet`, no refusal on drops.

## Decisions

- **Mid-write gap — unowned, dies unless re-opened.** Drops are computed before the write. If
  the state rename lands and a later write fails, R14a suppresses the rows; the same-argument
  retry compares the new state with itself and reports nothing. One entry can then vanish
  unremarked. Rare; not closed by this feature.

## Surface & Copy

Implemented verbatim.

**Text mode — rows on stdout**, one per dropped entry, in old-file line order, no group header,
no indent:

```
WARN  <state-rel>:<line>  dropped from <heading>, tagged <tag>: <excerpt>
WARN  <state-rel>:<line>  dropped from <heading>, untagged: <excerpt>
```

- `<state-rel>` — the state file's path relative to the working directory (also when
  `--state -`). `:<line>` always present (D6).
- `<heading>` — the configured heading text (e.g. `Open debts`).
- `<excerpt>` — the normalized text, cut to 80 runes, followed by `…` when cut.
- Two spaces separate severity, location and detail, as in `check`.

**Success line (stderr).** With N ≥ 1 drops, the `replaced <state-rel>` clause becomes
`replaced <state-rel> (dropped N entries, listed on stdout)`; singular `(dropped 1 entry,
listed on stdout)`. Example:

```
brief finish: f SCENARIO-07 done; wrote docs/specifications/f/SCENARIO-07-HANDOFF.md, replaced docs/specifications/f/STATE.md (dropped 2 entries, listed on stdout), ticked docs/specifications/f/specification.md; next: SCENARIO-08 — run 'brief start f'
```

With zero drops the success line is byte-identical to today's. No separate summary line.

**JSON.** New top-level field `"dropped_entries"`, last in document order (after `modified`),
always present on success, `[]` when empty. Element, keys in this order:

```json
{"severity":"WARN","rule":"dropped-debt","path":"<absolute state path>","line":42,"detail":"<same detail as the text row>","heading":"Open debts","tag":"SCENARIO-04","text":"<full normalized text, never cut>"}
```

`rule` is `dropped-debt` or `dropped-entry`; `tag` is a string or `null`. `jsonFieldsParagraph`
for `finish` lists `dropped_entries` after `modified`.

**`--help`.** Appended to `finishLong`, before the JSON paragraph:

```
Each entry under the four state headings that is missing from the new
body is listed on stdout as a WARN finding (rule dropped-debt under the
open-debts heading, dropped-entry otherwise); its line is in the file as
it was before replacement. Removal is reported, never refused; a
reworded entry counts as removed. Exit status stays 0.
```

**Exit codes.** Unchanged: 0 success (with or without rows), 1 refusal (no rows), 2 usage (no
rows).

**Outcome table.**

| Input | Rows / `dropped_entries` | Exit |
| --- | --- | --- |
| Entries removed | one row each | 0 |
| Nothing removed (whitespace reflowed) | none; `[]`; success line unchanged | 0 |
| Entry moved between two state headings | none | 0 |
| Entry moved to a non-state section | a row | 0 |
| Reworded or re-tagged | a row (old text) | 0 |
| Checkbox toggled inside state | a row | 0 |
| Old file lacks a heading | that section contributes nothing | 0 |
| Empty old state file | none | 0 |
| No old state file | refused before diffing (pre-existing); no rows | 1 |
| Old file CRLF | CR stripped before normalizing; line numbers unchanged | 0 |
| List line inside a fence | none | 0 |
| Paragraph line under a heading | none | 0 |
| Configured heading empty or duplicated | none for that heading | 0 |
| R11 identical re-finish | none; `[]`; existing no-op line | 0 |
| Any refusal | none; error doc has no `dropped_entries` | 1 |
| Usage error | none | 2 |
| Failure after the state rename landed | none (recorded gap) | 1 |

---

## Scenarios (Gherkin)

```gherkin
Scenario: SCENARIO-01 — A dropped state entry is reported, not refused
  Given a feature whose STATE.md lists "- X (SCENARIO-02)" under Traps at line 17
  When I finish SCENARIO-03 with a state body that omits it
  Then the step is done and the exit code is 0
  And stdout is "WARN  <state-rel>:17  dropped from Traps, tagged SCENARIO-02: X (SCENARIO-02)"
  And the stderr success line says "(dropped 1 entry, listed on stdout)"

Scenario: SCENARIO-02 — A dropped open debt is reported under its own rule
  Given an Open debts entry "- D — unowned — dies unless re-opened" with no tag
  When finish drops it
  Then its row reads "dropped from Open debts, untagged: D — unowned — dies unless re-opened"
  And its rule is dropped-debt

Scenario: SCENARIO-03 — A finish that removes nothing prints what it prints today
  Given a state body keeping every entry, with whitespace reflowed
  When I finish
  Then stdout is empty and the success line is byte-identical to today's

Scenario: SCENARIO-04 — --json lists dropped entries in a structured field
  When I finish with --json and entries are dropped
  Then "dropped_entries" is the last field and each element carries
       severity, rule, path, line, detail, heading, tag, text in that order
  And with no drops "dropped_entries" is []

Scenario: SCENARIO-05 — Moving an entry between state headings is not a drop; moving it out is
  Given an entry under Traps
  When the new body carries it under Binding decisions
  Then no row is printed
  But when the new body carries it only under a non-state heading
  Then a row is printed for it

Scenario: SCENARIO-06 — A reworded, re-tagged or re-ticked entry is reported as dropped
  Given an entry whose text, tag, or checkbox marker changes in the new body
  When I finish
  Then a row reports the old text

Scenario: SCENARIO-07 — A multi-line entry is one entry; a duplicate dropped once is reported once
  Given an entry with continuation lines and indented sub-items
  When the new body keeps it reflowed onto one line
  Then no row is printed
  And given the same text twice in the old body and once in the new
  Then one row is printed, at the later occurrence's line

Scenario: SCENARIO-08 — Only list items under the configured state headings are entries
  Given list lines inside a fence, paragraph lines, list items under other sections,
        an empty or duplicated configured heading, and CRLF line endings in the old body
  When the new body omits them
  Then no false row is printed and line numbers stay correct for CRLF

Scenario: SCENARIO-09 — A refused finish and an identical re-finish report no drops
  Given a finish that would drop entries but is refused
  Then no row is printed, the exit code is 1, and the error document has no dropped_entries
  And given an identical re-finish of a done step
  Then no row is printed and "dropped_entries" is []

Scenario: SCENARIO-10 — finish --help documents drop reporting
  When I run brief finish --help
  Then it contains the ruled drop-reporting paragraph before the JSON paragraph
  And the JSON paragraph lists dropped_entries after modified
```

---

## BDD Acceptance Progress

- [x] SCENARIO-01: A dropped state entry is reported, not refused
- [x] SCENARIO-02: A dropped open debt is reported under its own rule
- [x] SCENARIO-03: A finish that removes nothing prints what it prints today
- [x] SCENARIO-04: --json lists dropped entries in a structured field
- [x] SCENARIO-05: Moving an entry between state headings is not a drop; moving it out is
- [x] SCENARIO-06: A reworded, re-tagged or re-ticked entry is reported as dropped
- [x] SCENARIO-07: A multi-line entry is one entry; a duplicate dropped once is reported once
- [x] SCENARIO-08: Only list items under the configured state headings are entries
- [x] SCENARIO-09: A refused finish and an identical re-finish report no drops
- [ ] SCENARIO-10: finish --help documents drop reporting
