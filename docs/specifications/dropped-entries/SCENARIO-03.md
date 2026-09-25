---
id: SCENARIO-03
status: done
---

# SCENARIO-03: A finish that removes nothing prints what it prints today

## Scenario

```gherkin
Scenario: SCENARIO-03 — A finish that removes nothing prints what it prints today
  Given a state body keeping every entry, with whitespace reflowed
  When I finish
  Then stdout is empty and the success line is byte-identical to today's
```

User-visible contract: unchanged from SCENARIO-01/02 — zero drops means no stdout rows and
the stderr `replaced <state-rel>` clause carries no `(dropped N …)` suffix at all (`internal/cli/finish_dropped.go`
`dropCountSuffix(0)` already returns `""`).

This is D2's whitespace-normalization rule (`normalizeEntryText`, `internal/scaffold/dropped.go`)
first proof at CLI level. STATE.md records it as built (CR-strip and whitespace-collapse
both live in the same one-line function) but **unpinned by any assertion**: SCENARIO-03
owns the whitespace-collapse red, SCENARIO-08 owns the CR-strip red — this scenario must
not fold in a `\r\n` fixture.

**Green on arrival, named precisely:** interior doubled spaces and a tab, both collapsed by
`normalizeEntryText`'s whitespace-collapse step, and a trailing space, stripped earlier by
`markdown.entryItemText`'s `trimEOL` — plus the zero-drop success line, already exactly
`dropCountSuffix(0) == ""`. The existing zero-entry fixtures (e.g.
`Test_finishes_the_step_and_prints_nothing_to_stdout_mem`) prove none of this: their old
`STATE.md` bodies carry no column-0 list items at all, so they trivially produce zero drops
without ever calling `normalizeEntryText` on a real entry. This scenario's fixture must
carry real entries under more than one heading so the collapse itself is what's under test.
Expect the new test to pass unmodified against today's code; if it reddens, the fix belongs
in `normalizeEntryText`'s whitespace-collapse step, not elsewhere — report which and why.

**Excluded, deliberately: a wrapped line joined into one.** A logical entry spanning two
physical lines (`- foo` with a continuation line `  bar` below it) is **red today**, not
green: `markdown.Entries` ignores both an indented line and a paragraph line under an item,
so old `- foo\n  bar` scans as the single-line entry `foo`, and a new body carrying it
joined as `- foo bar` produces a false WARN row (or the same in reverse — one line in old,
wrapped in new). That is exactly SCENARIO-07's Gherkin, not this scenario's; do not fold a
two-physical-line fixture into Step 1.

## Implementation Plan

### Red

- [x] Step 1: `internal/cli/finish_dropped_internal_test.go` `Test_finish_reports_no_drops_when_the_new_body_only_reflows_whitespace_mem` — through `cli.Run` (`newMemFinishFixtureWithState` + `runFinishMem`), an old state whose `## Binding decisions`, `## Traps` and `## Open debts` sections each carry one single-physical-line entry, each followed by a blank line before the next (STATE.md's folding trap: a line sitting directly under an item must not be mistaken for a continuation fixture) — one entry with doubled interior spaces, one with a tab in place of a space, one with a trailing space on the line; the new body carries the same three entries with that whitespace collapsed to single spaces and no trailing space, wording and token order unchanged. Assert `stdout` is empty, `err` is nil, and `stderr` equals `memWantFinishCompleteLine("demo", "SCENARIO-01")` (the fixed pre-existing template, not a string built by calling `dropCountSuffix` or any other production formatter). Expected **green on arrival** — say so and why: SCENARIO-01 built the collapse and the trim generically; this step is the first thing to drive them through `cli.Run` with real entries present.

### Green

- No production edit anticipated. If Step 1 reddens, the fix belongs in `normalizeEntryText`'s whitespace-collapse step (`internal/scaffold/dropped.go`) — report which token boundary it fails to collapse, rather than patching around it.

### Sweep

- [x] Step 2: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

### Verify

- [x] Step 3: full verification per `.claude/rules/agent-briefs.md` from start commit `c7ab94f`. Mutation: in `normalizeEntryText` (`internal/scaffold/dropped.go`), replace `strings.Join(strings.Fields(strings.ReplaceAll(raw, "\r", "")), " ")` with `strings.TrimSpace(strings.ReplaceAll(raw, "\r", ""))` — CR-strip stays intact (so this mutation stays SCENARIO-03's own, not SCENARIO-08's), but runs of interior whitespace are no longer collapsed to one space → `Test_finish_reports_no_drops_when_the_new_body_only_reflows_whitespace_mem` goes red (the doubled-space and tab entries now register as false-positive drops). Restore and diff byte-identical.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Whitespace-collapse in `normalizeEntryText` and trailing-space trim in
  `markdown.entryItemText` needed no production change — SCENARIO-01 already built both
  generically; SCENARIO-03 adds the missing CLI-level proof that reflowed (not removed)
  whitespace produces zero drops, across three of the four state headings.
- Zero drops means the stderr success line is byte-identical to the pre-feature line (no
  `(dropped …)` clause at all) — the user-visible contract this scenario pins, via the
  pre-existing `memWantFinishCompleteLine` template.

**Left unbuilt** — named so nobody assumes it exists:
- Continuation-line folding across two physical lines in `markdown.Entries` — SCENARIO-07.
  It is **red today**: an old two-line entry (`- foo` + continuation `  bar`) scans as `foo`
  alone, so a new body carrying it joined as one line (`- foo bar`) produces a false WARN
  row today. SCENARIO-07 must add a CLI-level assertion for exactly this: old body with a
  two-line entry, new body with the same entry reflowed onto one line, expect empty stdout
  — the case this scenario explicitly excluded.
- CR-strip's own CLI-level proof (`\r\n` in the old body) — SCENARIO-08, deliberately not
  folded into this scenario.
- `dropped_entries` in JSON, including its `[]` empty-array shape on a zero-drop finish —
  SCENARIO-04; this scenario is text-mode only.

**Traps** — things that look right and are not:
- Fixtures whose old `STATE.md` body has no column-0 list items at all (several existing
  `_mem` tests) prove nothing about whitespace-collapse — they never call
  `normalizeEntryText` on a real entry. Don't reuse one as "proof" of this scenario's claim.
- `entryItemText` already trims trailing `" \t\r"` before `normalizeEntryText` ever runs, so
  a trailing-space-only fixture doesn't exercise the Verify mutation above — it's included
  for delta coverage, not as the thing that mutation reddens on.
- **A plain CRLF fixture likely cannot redden SCENARIO-08's own CR-strip mutation.** The
  body is split on `"\n"`, so a CRLF line ends in a bare `\r`, and `entryItemText`'s
  `trimEOL` (`strings.TrimRight(s, " \t\r")`) already strips it before `normalizeEntryText`
  ever sees the text; an interior CR is ordinary whitespace to `strings.Fields` regardless.
  Removing `ReplaceAll(raw, "\r", "")` only changes output for an interior *bare* CR
  mid-line, or a CR a continuation-line join (SCENARIO-07) leaves behind — SCENARIO-08's
  architect should read `normalizeEntryText` and `trimEOL` together before assuming a
  trailing-CRLF fixture proves anything.
