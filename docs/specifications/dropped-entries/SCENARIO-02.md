---
id: SCENARIO-02
status: done
---

# SCENARIO-02: A dropped open debt is reported under its own rule

## Scenario

```gherkin
Scenario: SCENARIO-02 — A dropped open debt is reported under its own rule
  Given an Open debts entry "- D — unowned — dies unless re-opened" with no tag
  When finish drops it
  Then its row reads "dropped from Open debts, untagged: D — unowned — dies unless re-opened"
  And its rule is dropped-debt
```

User-visible contract: unchanged from SCENARIO-01 — same row shape
(`WARN  <state-rel>:<line>  dropped from <heading>, untagged: <excerpt>`), same stderr
suffix, exit 0. This scenario proves it against `## Open debts` specifically, with the
em-dash "unowned — dies unless re-opened" wording through the excerpt path.

Rule classification (`dropRuleFor`) and the `untagged` rendering were built and tested at
Server level by SCENARIO-01 (`internal/scaffold/finish_dropped_test.go`
`Test_finish_classifies_an_open_debts_drop_as_dropped_debt` already pins
`DropRuleDebt` + empty `Tag` for this exact fixture text). No CLI-level test exercises the
`## Open debts` heading or this text today — that end-to-end proof, plus Product Verdict
item 4's doc tag, is this scenario's actual content. Expect the new test green on arrival;
do not manufacture a red.

Text mode never renders `Rule` (only `heading`/`tag`/`excerpt`), so "its rule is
dropped-debt" cannot be re-proven through stdout here — it stays proven at Server level
until SCENARIO-04 wires `dropped_entries` into JSON, where `"rule":"dropped-debt"` becomes
the first CLI-level proof of that clause (see Handoff).

## Implementation Plan

### Red

- [x] Step 1: `internal/cli/finish_dropped_internal_test.go` `Test_finish_reports_an_open_debts_drop_as_dropped_debt_mem` — through `cli.Run` (`newMemFinishFixtureWithState` + `runFinishMem`), an old state with `## Open debts` holding a kept entry and the untagged `- D — unowned — dies unless re-opened`, new body drops only the latter. Assert stdout is exactly `WARN  <state-rel>:<line>  dropped from Open debts, untagged: D — unowned — dies unless re-opened\n`, `err` is nil, and stderr contains `(dropped 1 entry, listed on stdout)`. Expected **green on arrival**: SCENARIO-01 built `dropRuleFor`, `dropDetail`'s untagged branch and the heading-display path generically; this step is the first thing to drive them through `## Open debts` and the em-dash text end-to-end.

### Green

- No production edit anticipated. If Step 1 reddens, the fix belongs in `dropDetail` (untagged branch) or `dropRuleFor` (`internal/scaffold/dropped.go`) — report which, and why SCENARIO-01's classification was wrong, rather than patching around it.

### Sweep

- [x] Step 2: `docs/specifications/brief/specification.md` line 239 — append the trailing `(SCENARIO-XX)` tag to the Default profile's Open debts line, matching the other three state-heading lines above it (Product Verdict item 4). Confirmed by grep the only place this format string appears outside `docs/specifications/dropped-entries/` (sweep)
- [x] Step 3: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

### Verify

- [x] Step 4: full verification per `.claude/rules/agent-briefs.md` from start commit `dbcc543`. Mutation: in `dropDetail` (`internal/cli/finish_dropped.go`), make the untagged branch always render `"tagged " + d.Tag` (dropping the `d.Tag != ""` guard) → `Test_finish_reports_an_open_debts_drop_as_dropped_debt_mem` goes red (`tagged : D …` instead of `untagged: D …`). Restore and diff byte-identical. (Do not repeat SCENARIO-01's `dropRuleFor` mutation — text mode cannot observe `Rule`, so it cannot redden anything new here.)

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- The Open debts row/rule path needed no new production code — SCENARIO-01's `dropRuleFor`
  and `dropDetail` already handle any configured heading and any text generically; SCENARIO-02
  only adds the missing CLI-level proof for `## Open debts` specifically.
- The Default profile's Open debts line in `docs/specifications/brief/specification.md`
  (line 239) now carries the same trailing `(SCENARIO-XX)` placeholder as the other three
  state-heading lines — Product Verdict item 4, closed here.

**Left unbuilt** — named so nobody assumes it exists:
- `dropped_entries` in `finishDocument`/`jsonFieldsParagraph`, including the JSON `rule`
  field — SCENARIO-04. That element is the first place `"rule":"dropped-debt"` is provable
  end-to-end through the CLI; SCENARIO-04's fixture set must include an Open-debts drop, or
  the Gherkin's rule clause never gets a JSON-level proof.
- `finishLong`'s drop-reporting `--help` paragraph — SCENARIO-10.
- Continuation lines/indented sub-items, the empty/duplicate heading guard, refusal/no-row
  assertions — SCENARIO-07/08/09, unchanged from SCENARIO-01's handoff.

**Traps** — things that look right and are not:
- Text-mode rows never carry `Rule` — don't plan or expect a stdout assertion that shows
  `dropped-debt`/`dropped-entry`; that only surfaces once SCENARIO-04 wires the JSON field.
- A drop fixture must keep at least one old entry under the same heading (here, a second
  Open debts entry) — a diff that ignores the new body entirely still passes an
  empty-kept-entry fixture.
