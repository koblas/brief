---
id: SCENARIO-21
status: done
depends-on: []
---

# SCENARIO-21: A step with an unfinished dependency cannot be finished

## Scenario

```gherkin
Scenario: SCENARIO-21 A step with an unfinished dependency cannot be finished  [orig: 06e]
  Given a step whose frontmatter declares depends-on another step
  And that step is unfinished
  When I finish it
  Then the refusal names the dependency
  And nothing is written
  # Refusing on a dependency is one frontmatter read and one done-check. ORDERING by
  # dependencies — R4's transitive closure, "next returns the first step whose dependencies
  # are done" — is a later phase and is not in scope here.
```

## Measured baseline (today, before this scenario)

Binary built from this worktree, feature scaffolded with `brief new feature demo` +
`brief new step demo` ×2, `SCENARIO-02` hand-edited to `depends-on: [SCENARIO-01]` while
`SCENARIO-01` stays `status: open`:

- `brief finish demo SCENARIO-02 --handoff … --state …` → **exit 0**, stdout empty, stderr
  `brief finish: SCENARIO-02 is done`. Files changed: `SCENARIO-02-HANDOFF.md` created,
  `SCENARIO-02.md` frontmatter `status: open`→`done`, `specification.md` entry ticked
  (`STATE.md` unchanged only because the supplied body was byte-identical). There is no
  dependency check on the write path at all.
- `depends-on: [SCENARIO-99]` (id naming no step file) → **exit 0**, finishes.
- `depends-on: [SCENARIO-02]` on `SCENARIO-02` itself → **exit 0**, finishes.
- `depends-on: []` → exit 0, finishes (the scaffolded shape; must keep working).
- `brief status` on the same trees already reports **blocked 1** in all three blocking cases
  (`demo 0/2 SCENARIO-01 1`) — SCENARIO-09's rule already calls every one of them blocked.
  So `status` and `finish` disagree on the same tree today; this scenario closes that.
- Re-finishing the now-done `SCENARIO-02` with the same inputs while its dependency is still
  open is a true no-op (exit 0, all five files byte-identical). That must stay true — see
  Decision 4.

## Decisions this plan commits to

1. **One definition of blocked, shared, in `internal/platform/stepfile`.** SCENARIO-09's rule
   lives inline in `assemble.featureStatus` (two loops: a done set keyed by
   `pattern.ID(n)`, then "any `depends-on` id not in it"). `scaffold` must not import
   `assemble` (sibling feature packages), so the **rule** — the keying, the predicate, and
   the never-blocked-when-done guard — moves down to `stepfile` as a new pure type
   `DependencyIndex`; `assemble.featureStatus` is refactored onto it in the same scenario.
   The **traversal** does not move: `assemble.readSteps` keeps its error→`Problem`
   degradation (09/11 behaviour), and `scaffold.Finish` scans the feature directory it has
   already opened.
2. **Direct dependencies only; no transitive closure.** The scenario's own comment in
   `specification.md` ("ORDERING by dependencies — R4's transitive closure … is a later
   phase and is not in scope here") and *Decisions taken* 7 separate the refusal from R4's
   ordering. R4 stays unbuilt.
3. **An id naming no step file blocks** (09's clause), and so does a self-dependency — both
   are simply "not a done step's `pattern.ID(n)`". One sentinel, two copy branches: an
   unmet-but-known dep ("is not finished", fix: finish it) and an unknown id ("names no step
   file", fix: correct the id). A self-dependency takes the first branch, printing its own
   id twice on one line; it is then permanently unfinishable through `brief finish` until the
   frontmatter is edited, which is correct — the tool refuses rather than writes, and the fix
   is named in the refusal. Reporting a self-dependency *proactively*, and reporting an
   unknown id outside the write path, belong to `check` (22).
4. **A done step is never dependency-refused.** `FirstUnmet` short-circuits on `fm.Done()`,
   which is 09's own "done steps are never blocked" clause, so a re-finish of a done step
   whose dependency was later re-opened stays R11's no-op (or 16's `ErrAlreadyFinished` on
   divergent inputs). This is a **deliberate exception** to the band pattern 17/18/19/20
   established ("a done step over any of these reports the content defect, never
   `ErrAlreadyFinished`") — it is the price of reusing 09's rule verbatim, and it is what
   keeps the measured no-op above reachable.
5. **A step file that cannot be read or whose frontmatter does not parse is recorded as a
   known, not-done step** — a zero `Frontmatter` whose `Done()` is false — because
   `stepfile.Frontmatter.Done()` is the sole doneness authority (STATE.md) and a file with no
   parseable doneness records none. It therefore blocks if depended on, reported with the "is
   not finished" copy; recording it (rather than skipping it) is what keeps `Known` true so the
   refusal does not claim a file that plainly exists "names no step file". `finish` grows
   **no** new malformed-sibling refusal; `status` already degrades that feature to `!` and
   `check` (22) owns the precise finding.
6. **The scan is skipped entirely when `len(fm.DependsOn) == 0`** — the scaffolded shape.
   No sibling is read, so an unrelated broken step file cannot affect an ordinary finish.
7. **Placement: immediately after `checkStepChecklist`, before the specification read** —
   SCENARIO-20 reports first when both apply, as STATE.md's *Left unbuilt* entry pins.
8. **`RefusalError.Line` is 0 (whole-path form).** `ParseFrontmatter` returns no line
   numbers and the fault is a dependency's state, not a position in this file.

## User-visible contract

Command: `brief finish <feature> <step> --handoff <path> --state <path>`

- Unmet known dependency → exit **1**, stdout **empty**, one stderr line:
  `brief finish: <abs path>/STEP-02.md: step "STEP-02" depends on "STEP-01", which is not finished; finish STEP-01 first, or remove it from this step's depends-on, and retry (no files changed)`
- Unknown dependency id → exit **1**, stdout empty, one stderr line:
  `brief finish: <abs path>/STEP-02.md: step "STEP-02" depends on "STEP-99", which names no step file; correct the id in this step's depends-on, or remove it, and retry (no files changed)`
- Empty `depends-on`, or every dependency done → unchanged: exit 0, stdout empty, stderr
  `brief finish: <step> is done`.
- Wrapped sentinel: `scaffold.ErrUnmetDependency` (both branches), inside `*RefusalError`
  rendered by the existing `cli/refusal.go` path. No `cli/finish.go` change is expected —
  the refusal carries a real step-file path, not a `HandoffSource`/`StateSource` placeholder.

## Implementation Plan

Every refusal test in Steps 5–14 carries **both** nothing-lands probes — `snapshotTree` byte
identity over the whole feature directory and the `pinModTimes`/`modTimes` pin — even where the
step line does not repeat it. Step 5 owns the nothing-lands proof for this scenario; Step 15
owns the exit-code and stdout/stderr contract. Every new exported symbol carries a doc comment
per the `clean-architecture` skill.

- [x] Step 1: `internal/platform/stepfile/depends_test.go` — `DependencyIndex` unit tests (red):
      a recorded done step meets its dependant; a recorded **not-done** step is `Known` but
      unmet; an id no step was recorded under is unmet **and** not `Known`; a step recorded from
      a zero `Frontmatter` (the unparseable-sibling case) is `Known` and unmet; a `Frontmatter`
      whose `Done()` is true is never unmet even with an unmet dep; empty `DependsOn` is never
      unmet; two unmet deps report the **first in `depends-on` declaration order**;
      **the index keys on `pattern.ID(n)`, not `fm.ID`** (fixture: file number 2 whose
      frontmatter says `id: WRONG-02`, depended on as `STEP-02`, resolves) (new)
- [x] Step 2: `internal/platform/stepfile/depends.go` — `DependencyIndex` + `NewDependencyIndex`,
      pointer receivers throughout: `Record` stores **every** step it is given, done or not,
      keyed by `pattern.ID(n)`, carrying that step's doneness; `FirstUnmet` reports nothing for a
      done `Frontmatter`, otherwise the first declared dependency that is not a recorded done
      step (an id never recorded and a recorded not-done step are both unmet); `Known` reports
      whether an id was recorded at all (green)
- [x] Step 3: `internal/platform/stepfile/doc.go` — package doc gains the dependency rule and
      names its two callers (`status`'s blocked count, `finish`'s refusal) (update)
- [x] Step 4: `internal/assemble/status.go` `featureStatus` — replace the inline done-map and
      blocked loop with `DependencyIndex`; SCENARIO-09/11's status tests are green on arrival —
      say so, do not manufacture a red (update)
- [x] Step 5: `internal/scaffold/finish_depends_test.go` `Test_finish_refuses_a_step_whose_dependency_is_not_finished`
      — Server-method test on a fixture whose STEP-02 declares `depends-on: [STEP-01]` with
      STEP-01 `status: open`; assert `ErrorIs` the new sentinel, `Equal` on the whole refusal
      line, `refusal.Line == 0`, plus **both** nothing-lands probes (`snapshotTree` byte
      identity and the `pinModTimes`/`modTimes` mtime pin over every fixture file) (red)
- [x] Step 6: `internal/scaffold/errors.go` — `ErrUnmetDependency` sentinel with a doc comment
      stating both branches and the done-step exemption (new)
- [x] Step 7: `internal/scaffold/finish.go` `checkStepDependencies` — returns nil immediately
      when `len(fm.DependsOn) == 0`; otherwise scans the already-open feature `*os.Root` for
      entries `pattern.Number` recognizes, records each readable+parseable one into a
      `stepfile.DependencyIndex` — a read or parse failure records that id from a zero
      `Frontmatter` rather than skipping it, per Decision 5 — then renders `FirstUnmet`'s result
      through `Known` into the unmet-vs-unknown copy; call site goes **immediately after**
      `checkStepChecklist` and before the specification read (green)
- [x] Step 8: `internal/scaffold/finish_depends_test.go` `Test_finish_refuses_a_dependency_id_that_names_no_step_file`
      — the unknown-id copy branch, same two nothing-lands probes (new)
- [x] Step 8a: `internal/scaffold/finish_depends_test.go` `Test_finish_refuses_a_dependency_whose_step_file_does_not_parse`
      — the depended-on sibling exists but its frontmatter is garbage: the **"is not finished"**
      line, never the "names no step file" line and never `ErrMalformedFeature`, and never a
      pass. This is what makes Decision 5 observable (new)
- [x] Step 9: `internal/scaffold/finish_depends_test.go` `Test_finish_refuses_a_step_that_depends_on_itself`
      — pins the self-dependency taking the "is not finished" branch with its own id twice, so
      the copy is a deliberate pinned decision rather than an accident (new)
- [x] Step 10: `internal/scaffold/finish_depends_test.go` `Test_finish_accepts_a_step_with_no_declared_dependencies`
      — control arm: a target with `depends-on: []` in a feature that also holds a **not-done**
      sibling still finishes, and the four writes actually land (asserted on disk, so it cannot
      pass on a silent no-op) (new)
- [x] Step 11: `internal/scaffold/finish_depends_test.go` `Test_finish_accepts_a_step_whose_dependency_is_done`
      — the other side of the boundary: dependency recorded `status: done` → the finish lands
      (new)
- [x] Step 12: `internal/scaffold/finish_depends_test.go` `Test_finish_reports_an_open_checklist_item_before_an_unfinished_dependency`
      — both-apply ordering (SCENARIO-20 first), paired with the reverse mutation named below,
      or the test is vacuous (new)
- [x] Step 13: `internal/scaffold/finish_depends_test.go` `Test_finish_reports_an_unfinished_dependency_before_the_specification_read`
      — a blocked step in a feature whose progress entry for it is missing reports the
      dependency, not `ErrNoProgressEntry` (new)
- [x] Step 14: `internal/scaffold/finish_depends_test.go` — the done-step pair: a **done** step
      with an unfinished dependency re-finished with the recorded inputs is still a no-op
      (mtimes pinned), and with divergent inputs is still `ErrAlreadyFinished` — Decision 4 (new)
- [x] Step 15: `internal/cli/finish_test.go` — command-slice test through `cli.Run`: exit code
      1, **stdout empty**, the exact single stderr line including the `(no files changed)` tail,
      and the tree byte-identical afterwards (new)
- [x] Step 16: `internal/scaffold/finish.go` — extend `Finish`'s doc comment so the enumerated
      check band names the dependency check in its real position, its skip-when-empty rule, its
      ignore-unparseable-sibling rule, and the done-step exemption (update)
- [x] Step 17: full-suite fixture sweep — run `go test ./...` from the repo root and repair
      **whatever it surfaces**, not an enumerated list: STATE.md's trap records that 19's plan
      named six fixtures and the suite found nine, and two failed with the wrong sentinel rather
      than a build break. Existing `depends-on: [STEP-01]` fixtures point at a `status: done`
      STEP-01 and should pass; treat that as a prediction to verify, not a fact (update)
- [x] Step 18: individual mutation verification, one at a time, each stashed per the standing
      brief. Say which mutation reddened which test:
      (a) invert `FirstUnmet`'s unmet condition → Step 5's refusal test **and** `assemble`'s
      blocked-count test go red, proving the rule is genuinely shared;
      (b) delete `FirstUnmet`'s done short-circuit → **both** of Step 14's tests go red (the
      no-op returns `ErrUnmetDependency` instead of nil, the divergent one returns it instead
      of `ErrAlreadyFinished`) and nothing else — existing done-step fixtures depend on a
      `status: done` STEP-01, so they are unaffected;
      (c) move the `checkStepDependencies` call above `checkStepChecklist` → Step 12 goes red
      (deleting either check would not prove the order; moving the call site does);
      (d) delete the empty-`depends-on` early return → **nothing is expected to go red**: it is
      a pure optimization with no behavioural contract, since `FirstUnmet` reports nothing for
      an empty list anyway. Report the green and do not claim Step 10 covers it;
      (e) invert the `Known` condition in the copy branch → Step 5 renders the unknown-id line
      and Step 8 renders the not-finished line, both red and separately attributable because
      each pins its whole line with `Equal`
- [x] Step 19: verification per `.claude/rules/agent-briefs.md` (`go build`, unpiped
      `go test ./...` with the exact count and delta, `go test -race` on
      `./internal/scaffold/... ./internal/assemble/... ./internal/platform/stepfile/... ./internal/cli/...`,
      `golangci-lint run ./...`), then mark SCENARIO-21 done in `specification.md`

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- `stepfile.DependencyIndex` is the **single** definition of "blocked" — `assemble.featureStatus`'s
  blocked count (09) and `scaffold.Finish`'s refusal (21) both call `FirstUnmet`; `scaffold`
  cannot import `assemble`, so the rule had to land in platform. A second definition makes
  `status` and `finish` disagree about the same tree, which they measurably did before this.
- `Record` stores **every** step as `ids[pattern.ID(n)] = fm.Done()`, not just done ones —
  `Known(id)` is what separates "exists but open" from "no such step file", and only the
  unknown-id refusal copy consumes it. Filtering on `Done()` inside `Record` breaks that copy
  silently; `assemble` would never notice.
- Keying is `pattern.ID(n)`, never `fm.ID` — `scaffold.findStepFile` resolves `brief finish`'s
  step argument the same way, so a step file whose frontmatter `id:` disagrees with its filename
  still resolves consistently across `finish`, `status` and `start`.
- **A done step is never dependency-refused** (`FirstUnmet` short-circuits on `fm.Done()`),
  a deliberate exception to 17/18/19/20's "a done step over any band check reports the content
  defect, never `ErrAlreadyFinished`" pattern. It is 09's own clause, and it is what keeps R11's
  no-op re-finish reachable on a tree whose dependency was re-opened by hand.
- Direct dependencies only. R4's transitive closure and `next`-by-dependency ordering stay
  unbuilt, per the scenario's own comment in `specification.md`.
- The dependency check sits **after** `checkStepChecklist` and **before** the specification
  read. A step both un-ticked and blocked reports the checklist item (20's pinned constraint).
- The scan is skipped when `depends-on` is empty, so the scaffolded step shape reads no sibling.

**Left unbuilt** — named so nobody assumes it exists:

- `stepfile.DependencyIndex` transitive/cycle traversal, `assemble.Server.Next` ordering by
  dependencies (R4) — no owner; R4 is a later phase.
- A malformed-sibling refusal on `finish`. A depended-on step file that cannot be read or whose
  frontmatter does not parse is reported as "is not finished". Owner: `check` (22).
- A self-dependency finding, an unknown `depends-on` id finding, and a dependency cycle finding
  **outside** the write path — owner: `check` (22).
- `FeatureStatus.Blocked` is still a count with no ids attached, and no `--json` on `status`.

**Traps** — things that look right and are not:

- `!ids[dep]` is true for both an absent key and a recorded `false`. That is the intended rule,
  but it means a typo'd dep id and an open dep are indistinguishable inside `FirstUnmet` — the
  copy branch depends on `Known`, so removing the `Known` call silently collapses two messages
  into one and only an `Equal` on the whole refusal line catches it.
- An unparseable depended-on step file must be **recorded** as a known not-done step, not
  skipped. Skipping it makes `Known` false and the refusal then claims a file sitting right
  there "names no step file" — the wrong message, and the one `check` (22) inherits.
- The existing `newFinishFixture` already ships `STEP-02` with `depends-on: [STEP-01]` and
  `STEP-01` `status: done` — the happy-path fixture is *already* dependency-satisfied, so a
  broken check can pass the whole existing suite. Step 5's fixture must re-open STEP-01.
- A "reports X first" test passes with the *other* check deleted; mutation (c) — moving the call
  site, not deleting it — is the one that proves the order.
- A byte snapshot alone does not prove a refusal happened before any write; pair it with the
  `pinModTimes`/`modTimes` probe, as every other refusal test in `finish_*_test.go` does.
- Refactoring `assemble.featureStatus` produces no red on arrival. That is expected and must be
  reported as such, not papered over with a manufactured failing test.
