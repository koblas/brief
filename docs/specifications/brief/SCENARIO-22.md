---
id: SCENARIO-22
status: done
---

# SCENARIO-22: Check reports what the write path would now refuse

## Scenario

```gherkin
Scenario: SCENARIO-22 Check reports what the write path would now refuse  [orig: R18, :451]
  Given a feature carrying an over-cap handoff written before the caps existed
  When I run check
  Then the finding names the file, the line, the measured value and the cap
  And it is emitted in the profile's finding shape, not the refusal shape
  Given every feature conforming
  When I run check
  Then it reports nothing and succeeds
```

## Contract this scenario ships

- **Invocation:** `brief check [feature]`. No argument = every feature directory, in the same
  `fs.ReadDir` order `status` uses. One argument = that feature only; an unknown feature is a
  read refusal (exit 1, stderr, no `(no files changed)` tail). Two or more = usage, exit 2.
- **Findings go to stdout**, one per line, in the profile's finding shape:
  `[SEVERITY] <path>:<line> — <problem>`. Three spec locations describe this shape and only
  two of them bind: the profile block (`specification.md` "Findings") and **R14a** both say
  `[SEVERITY] <path>:<line> — <finding>`, and the Gherkin says "the profile's finding shape,
  **not the refusal shape**". The command table's `path:line: message` row is the loose third
  description and does not bind. Do not "correct" this back to R14a's refusal template.
- **`<problem>` is the same string the refusal would print**, produced by the one shared
  predicate. `<fix>` is deliberately **not** carried into a finding: the refusal copy ends
  "and retry", which is write-path copy with no meaning in a report.
- **Severity is closed at two values.** `ERROR` for a finding on a feature still in flight (any
  step not done, or steps that could not be read); `WARN` for a finding on a feature whose
  every step is done — R18: "`check` reports what predates the tool and **fails only on
  features still in flight**."
- **Exit codes:** 0 when there is no `ERROR` finding (including a run that prints `WARN`
  findings only); 1 when any `ERROR` finding was printed; 2 for usage. `cmd/brief` needs no
  change: `main` prints nothing of its own, so `runCheck` returns an unexported sentinel after
  printing, and `ExitCode`'s default branch maps it to 1.
- **R14a vs R18 collision, resolved here, deliberately.** R14a says findings are "(R9, exit 0
  …)" and "findings never appear on a failed run". That sentence is scoped to **`finish`'s R9
  dropped-entry findings** — its purpose is letting a script tell "finished with reported
  drops" from "refused" by exit code alone. `check`'s exit code is governed by R18, whose word
  is "fails". So `check` can and does print findings on a run that exits 1. This is the single
  inference in this plan a reviewer will challenge; the reconciliation is recorded in the
  Handoff.
- **Nothing to return** (R14, SCENARIO-10 shape): no findings → empty stdout, one stderr line,
  exit 0. A repository with no features is the same shape.
- Anything printed is relative to nothing: paths are absolute, as every other command's are.

## The one-definition rule — this scenario's whole point

`check` must report exactly what `finish` would refuse. SCENARIO-21 set the pattern (the
"blocked" rule moved into `stepfile.DependencyIndex` because `scaffold` and `assemble` cannot
import each other). Same move here:

- **Extracted** into a new `internal/platform/conform`: the four argument/body predicates now
  private to `scaffold/finish.go` — `checkArgumentCap`, `checkArgumentFence`,
  `checkArgumentHeadings`, `checkStepChecklist` — as funcs returning a neutral
  `*conform.Violation{Line, Problem, Fix, Err}`. `scaffold` renders a `Violation` into a
  `*RefusalError`; `assemble.Check` renders it into a `Finding`. A future change to a refusal's
  wording or threshold lands in one function and shows up in both.
- **Not extracted:** `checkStepDependencies`. Its rule already lives in
  `stepfile.DependencyIndex` (21); what remains is `*os.Root` scanning that `check` does its
  own way. Moving it would be a second definition of the scan, not a shared one.
- **Not extracted:** the progress-entry rule. `finish`'s band also requires the specification to
  carry an entry for the step, but the only definition of "an entry" is inside
  `scaffold.tickProgressEntry`, which finds *and rewrites* the entry in one pass. A read-only
  matcher for `check` would be exactly the second definition this scenario exists to prevent.
  Deliberately out of scope; recorded as debt.
- **Population, not predicate, is where `check` narrows.** One principle: *`check` reports a
  state `finish` would refuse, except where that state is ordinary in-progress work.* That
  removes two populations and nothing else — an **open** step's unticked checklist items, and an
  open step's known-but-unmet dependency (already counted by `status`'s `Blocked`). A **done**
  step's unticked item, a self-dependency and an unknown `depends-on` id all survive.

## Rules `check` reports (the whole owned set)

Per feature: `C1` specification missing / unreadable / unterminated fence / no progress heading
(reuse `checkSpecification`) · `C2` state file missing or unreadable · `C3` state over
`state-cap-lines` · `C4` state unterminated fence · `C5` state missing a required heading.

Per step file, ascending: `C6` frontmatter absent or unparseable · `C7` **done** step with an
unticked checklist item · `C8` `depends-on` id that names no step file · `C9` self-dependency ·
`C10` the step's handoff file exists and is over `handoff-cap-lines` (**the Gherkin's case**).

A **missing** handoff file is not a finding: `finish` exempts it (16), so it is not something
the write path would refuse.

## Implementation Plan

### Phase A — one definition (extraction; no user-visible change)

- [x] Step 1: `internal/platform/conform/conform_test.go` — unit tests for the four predicates:
      exactly-cap passes and cap+1 fails; unclosed fence reports line and delimiter; only the
      **first** missing heading is reported; an absent checklist heading and an item-less
      checklist both yield no violation (red)
- [x] Step 2: `internal/platform/conform/doc.go` + `conform.go` — `Violation` plus `OverCap`,
      `UnterminatedFence`, `MissingHeading`, `OpenChecklistItem`; problem/fix copy moved
      **verbatim** from `scaffold/finish.go`'s four helpers so no refusal byte changes (new)
- [x] Step 3: `internal/platform/conform/conform.go` — the four sentinels (`ErrOverCap`,
      `ErrUnterminatedFence`, `ErrMissingStateHeading`, `ErrOpenChecklistItem`) move here (new)
- [x] Step 4: `internal/scaffold/errors.go` — those four become aliases of `conform`'s, so
      `errors.Is` and the exported surface are unchanged; doc comments state the rule, not the
      move (update)
- [x] Step 5: `internal/scaffold/finish.go` — the four helpers become thin `conform.*` calls
      converting `*Violation` into a `*RefusalError` with the call site's `Path`/`Line`; the
      band, its order and its doc comment are untouched (update — `finish_cap_test.go`,
      `finish_headings_test.go`, `finish_checklist_test.go` are **green on arrival**; report
      that, do not manufacture a red)
- [x] Step 6: `internal/scaffold/doc.go` — the sentinel enumeration names the `conform`-owned
      ones (update)

### Phase B — the read-side band

- [x] Step 7: `internal/assemble/check_test.go`
      `Test_check_reports_an_over_cap_handoff_file` — `(*Server).Check` against a fixture whose
      recorded handoff exceeds the cap; assert path, line, measured value and cap (red)
- [x] Step 8: `internal/assemble/check.go` — `Finding`, `Severity`, and
      `(*Server).Check(ctx, feature string) ([]Finding, error)`; `C10` only (green).
      The feature walk mirrors `Status`'s (`os.OpenRoot` → `fs.ReadDir`) but the **step walk
      does not use `readSteps`**: `readSteps` returns on the first unreadable or unparseable
      step file (`assemble.go:249-277`), which would collapse the whole feature and skip
      `C7`–`C10` for every other step in it. `check` walks entries itself —
      `pattern.Number(e.Name())` → read → `ParseFrontmatter` → on failure record `C6` and
      **continue**, still running `C10` for that step, since the handoff cap does not depend on
      frontmatter. Steps are visited in numeric order, as `readSteps` sorts them
- [x] Step 9: `internal/assemble/render.go` `RenderFindings` — `[SEVERITY] <path>:<line> —
      <problem>`, one per line; the renderer stays dumb, `Check` decides severity and line
      (green)
- [x] Step 10: `internal/assemble/check.go` — a cap finding's `Line` is `cap + 1`, the first
      line over the cap, set at the call site with its one-line justification: the shared
      predicate stays line-less because giving it a `Line` would change SCENARIO-17/18's pinned
      refusal bytes, which is out of scope (green)
- [x] Step 11: `check_test.go` + `check.go` — `C3` state over `state-cap-lines` (red → green)
- [x] Step 12: `check_test.go` + `check.go` — `C4` state unterminated fence. Read the state
      bytes directly rather than through `readStateFile`, whose own refusal copy would be a
      second definition of the same fault (red → green)
- [x] Step 13: `check_test.go` + `check.go` — `C5` state missing a required heading (red → green)
- [x] Step 14: `check_test.go` + `check.go` — `C2` state file missing or unreadable, via
      `newProblem` (red → green)
- [x] Step 15: `check_test.go` + `check.go` — `C1` specification faults, reusing
      `(*Server).checkSpecification` and converting its `*RefusalError` into findings
      (red → green)
- [x] Step 16: `check_test.go` + `check.go` — `C6` a step file whose frontmatter is absent or
      does not parse becomes a finding, never an error: one malformed step must not blind
      `check` to the rest of the feature, the same stance `Status` takes (red → green)
- [x] Step 17: `check_test.go` + `check.go` — `C7` an unticked checklist item on a **done**
      step. Pair it with a control arm: the same item on an **open** step yields no finding.
      Pass the **whole step file** to the predicate, never `stepEntry.rest` — `FirstUnchecked`
      counts from the top of what it is given, so the remainder short-changes every line by the
      frontmatter's length, and Step 28 cannot catch it because it asserts the `<problem>`
      segment, not the line. The fixture's step file must carry a **multi-line frontmatter
      block** and the test must assert the exact line number, or the defect hides (red → green)
- [x] Step 18: `check_test.go` + `check.go` — `C8` a `depends-on` id `DependencyIndex.Known`
      reports nothing under, with `finish`'s "names no step file" copy. Control arm: an
      ordinary open, unmet dependency yields no finding (red → green)
- [x] Step 19: `check_test.go` + `check.go` — `C9` a step declaring its own id in `depends-on`;
      check-specific copy, since `finish` folds this into its "is not finished" branch
      (red → green)
- [x] Step 20: `check_test.go` — severity: a feature with an open step yields `ERROR`, the same
      defect on a feature whose every step is done yields `WARN`. One variable differs between
      the arms (red → green)
- [x] Step 21: `check_test.go` — ordering is byte-exact across one fixture carrying several
      findings: features in `ReadDir` order, within a feature specification → state → step
      files ascending, within a step the band's own order (red → green)
- [x] Step 22: `check_test.go` — a conforming feature yields **no findings** (Gherkin case 2),
      and `Check("")` over a repository with no feature directory returns `nil, nil`
      (red → green)
- [x] Step 23: `internal/assemble/doc.go` — third clause for `Check`: the backstop that reports
      what the write path would now refuse (update)

### Phase C — the command surface

- [x] Step 24: `internal/cli/check_test.go` — slice tests through `cli.Run`: findings land on
      stdout in the profile shape; a clean repository prints nothing on stdout, one line on
      stderr, exit 0; an `ERROR` finding exits 1 while a `WARN`-only run exits 0; an unknown
      feature is a one-line refusal with **no** `(no files changed)` tail; two positionals and
      an undefined flag are usage errors (`ErrUsage`, exit 2) (red)
- [x] Step 25: `internal/cli/check.go` — `runCheck` plus `checkUsage`; resolve config the way
      `runStatus` does, render through `assemble.RenderFindings`, print the summary stderr line,
      and return the unexported findings sentinel when any `ERROR` was printed (green)
- [x] Step 26: `internal/cli/cli.go` — dispatch `check`, and update **all four** places that
      enumerate the commands: the `usage` const's command list, its "Run 'brief … --help'"
      sentence, the no-command message and the unknown-command message. Enumerate the fixtures
      this breaks **by running the suite**, not from this list (update)
- [x] Step 27: `internal/cli/run_test.go` and any other pinned-copy test the suite names —
      update the pinned strings (update)

### Phase D — anti-drift, mutations, closing

- [x] Step 28: `internal/cli/check_drift_test.go` — the test that matters: for each of the four
      shared predicates, `brief finish` refuses and `brief check` reports the same defect, and
      the `<problem>` segment of the two lines is **identical**. This is the proof the two
      cannot drift. `OverCap`/`UnterminatedFence`/`MissingHeading` pair on **one** fixture —
      pass the defective bytes as `--handoff`/`--state` while the same bytes sit on disk, since
      the cap band runs ahead of the refinish verdict. `OpenChecklistItem` cannot: `finish`
      refuses an open step and `check` reports a done one, so it pairs across **two fixtures
      differing only in `status:`** — still a valid copy-equality assertion, because the problem
      text depends on the item text alone (red → green)
- [x] Step 29: mutation-verify each rule individually, stashing every mutation per the standing
      brief. Two that are load-bearing and must be named in the report: (a) replace `check`'s
      `conform.OverCap` call with a hand-written `fmt.Sprintf` carrying the same numbers in
      different words → Step 28 goes red while Step 7's shape test stays green; (b) change the
      shared predicate's copy → **both** outputs change and Step 28 correctly stays green, which
      is one definition working, not a vacuous assertion. Record (b) so nobody later "fixes" it
- [x] Step 30: run `brief check` from this repository's own root with the built binary and
      record the measured output verbatim in the handoff — do **not** change the caps and do
      **not** edit the tree to make it pass. Expected from today's tree: `HandoffCapLines` 60,
      `StateCapLines` 80; 19 handoff files measure over 60 by `wc -l` (29–373), but one of them
      is `SCENARIO-HANDOFF-FILE.md`, which matches no step's handoff name, so `check` reports
      **18**; `STATE.md` is 129 lines against 80; all 21 `SCENARIO-NN.md` files still carry no
      frontmatter, so each is a `C6` finding and the feature reads as in flight → `ERROR`,
      exit 1. Read every number from the tool, not from `wc -l` — `markdown.CountLines` need
      not agree with `wc -l` on a file's last line (no file here is near the boundary; the
      closest is 57 against 60)
- [x] Step 31: verification per the standing brief, then mark SCENARIO-22 done in
      `specification.md`

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- Findings print as `[SEVERITY] <path>:<line> — <problem>` on **stdout** — the profile block and
  R14a both specify that shape and the Gherkin demands "not the refusal shape"; the command
  table's `path:line: message` row is the non-binding third description.
- Severity has exactly two values, `ERROR` (feature still in flight) and `WARN` (every step
  done) — R18: "`check` reports what predates the tool and fails only on features still in
  flight." Adding a third value breaks the exit-code rule below.
- `brief check` exits 1 when any `ERROR` finding printed, 0 otherwise — **this is the R14a/R18
  collision, resolved deliberately.** R14a's "findings … exit 0 … never appear on a failed run"
  is scoped to `finish`'s R9 dropped-entry findings, whose purpose is telling "finished with
  drops" from "refused" by exit code. `check`'s exit code is R18's "fails". Do not reconcile it
  the other way without reopening R18.
- The four write-path body predicates live in `internal/platform/conform` and are called by both
  `scaffold.Finish` and `assemble.Check` — `scaffold` and `assemble` cannot import each other,
  the same constraint that put `DependencyIndex` in `stepfile` (21). `scaffold.ErrOverCap`,
  `ErrUnterminatedFence`, `ErrMissingStateHeading` and `ErrOpenChecklistItem` are now aliases of
  `conform`'s; `errors.Is` still works and the exported surface is unchanged.
- A finding carries the predicate's `<problem>` and **not** its `<fix>` — the fix copy ends "and
  retry", which is write-path copy.
- `check` narrows the **population**, never the predicate: an open step's unticked items and an
  open step's known-but-unmet dependency are ordinary in-progress work and are not findings. Any
  new rule must state which population it applies to.
- A cap finding's line is `cap + 1` and is set by `assemble.Check`, not by the predicate: giving
  `conform.OverCap` a `Line` would change SCENARIO-17/18's pinned refusal bytes.
- A **missing** handoff file is not a finding — `finish` exempts it (16), so it is not something
  the write path would refuse.

**Left unbuilt** — named so nobody assumes it exists, and this is the last scenario, so every
item below is **debt with no owner**:

- The progress-entry rule: `check` does not verify that the specification carries a progress
  entry per step, because the only definition of "an entry" is inside
  `scaffold.tickProgressEntry`, which finds and rewrites in one pass. Deliberate.
- An id/filename mismatch finding (`fm.ID` vs `pattern.ID(n)`) and a leftover `## Handoff`
  section finding — dropped from this scope: neither has a `finish` counterpart, and a
  check-only rule undercuts the one-definition story. Earlier handoffs assigned both to `check`.
- A done step with **no** handoff file; a dependency **cycle** finding; `markdown.Headings`,
  `markdown.ChecklistItems`, `HandoffPattern.Number`; `check --json`; `check` over a single
  step. Everything STATE.md already lists unowned stays unowned.

**Traps** — things that look right and are not:

- **`assemble.readSteps` is all-or-nothing** — it returns on the first unreadable or
  unparseable step file, which is why `Status` collapses a malformed feature into one row.
  Reusing it in `check` silently drops `C7`–`C10` for every other step in that feature.
- `stepEntry.rest` is the post-frontmatter remainder. Feeding it to `conform.OpenChecklistItem`
  produces line numbers short by the frontmatter's length, and the drift test cannot see it.
- `SCENARIO-HANDOFF-FILE.md` (373 lines) matches neither the step pattern nor
  `pattern.ID(n) + "-HANDOFF.md"`, so `check` never sees it. It is not a bug; it is invisible.
- `markdown.Section(body, "")` returns `found == true` on the first blank line, so an empty
  configured heading passes `C5` silently — inherited from 19, still not fixed, now inherited by
  `check` through the same shared predicate.
- `assemble.readStateFile` has its own refusal copy for an unterminated fence. Using it for `C4`
  would produce a second definition of a fault `conform` already owns — read the bytes directly.
- A "reports X" test on a `check` rule is vacuous without a **control arm that differs in one
  variable** — the open-vs-done step for `C7`, the unknown-vs-unmet dependency for `C8`.
- A plan's fixture-update list under-counts. Step 26's four `cli.go` sites and Step 27's pinned
  tests must be enumerated by running the suite, not from this file.
- Running `check` on this repository is expected to exit 1 with dozens of findings. That is the
  product telling the truth about its own tree, not a failing build — do not adjust the caps or
  the tree to silence it.
