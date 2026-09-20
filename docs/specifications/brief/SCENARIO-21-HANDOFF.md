# SCENARIO-21 Handoff: A step with an unfinished dependency cannot be finished

## What landed

`internal/platform/stepfile` gained a new pure type, `DependencyIndex`
(`depends.go`): `NewDependencyIndex()`, `Record(id string, fm Frontmatter)`
(stores every step's doneness, keyed by whatever id the caller records it
under — never `fm.ID`), `Known(id string) bool`, and
`FirstUnmet(fm Frontmatter) (string, bool)` (the first declared dependency
that is not a Recorded done step; short-circuits `("", false)` when
`fm.Done()`). This is now the **single** definition of "blocked" —
`internal/assemble/status.go`'s `featureStatus` was refactored in this same
scenario to build one per feature and call `FirstUnmet` instead of its old
inline done-map/blocked-loop pair; `internal/assemble/status_test.go` was
untouched and green on arrival, proving the refactor bit-for-bit preserved
09/11's behaviour.

`(*scaffold.Server).Finish` calls a new `checkStepDependencies(root,
pattern, fm, step, stepPath) error`, placed immediately after
`checkStepChecklist` and before the specification read — SCENARIO-20's
pinned "checklist reports first" holds, and this scenario's own dependency
check now reports ahead of the spec read. It returns `nil` fast when
`fm.DependsOn` is empty (no directory scan at all), otherwise scans the
already-open feature `*os.Root` for every entry `pattern.Number`
recognizes, records each into a fresh `DependencyIndex` — a sibling that
cannot be read or whose frontmatter does not parse is Recorded from a zero
`Frontmatter` (via a small `siblingFrontmatter` helper) rather than
skipped, per Decision 5 — then renders `FirstUnmet`'s result through
`Known` into one of two refusal copies. `checkStepDependencies` returns a
single `error`, not `(*RefusalError, error)`: golangci-lint's `nilnil`
checker flagged the two-nil-return shape on first pass, and collapsing to
one return also dropped `Finish`'s cyclomatic complexity back under the
`maintidx` threshold the extra branch had pushed it over — both are `go
build`/lint facts, not stylistic choices.

New sentinel `ErrUnmetDependency` in `internal/scaffold/errors.go`.
`RefusalError.Path` is always the real step file being finished (`Line:
0`); no `internal/cli` change was needed or made — confirmed by a CLI
slice test passing on arrival, the same proof SCENARIO-20 used.

## Refusal copy

Unmet but known (including self-dependency, which prints its own id on
both sides of the line):

```
brief finish: <abs step path>: step "STEP-02" depends on "STEP-01", which is not finished; finish STEP-01 first, or remove it from this step's depends-on, and retry (no files changed)
```

Unknown id:

```
brief finish: <abs step path>: step "STEP-02" depends on "STEP-99", which names no step file; correct the id in this step's depends-on, or remove it, and retry (no files changed)
```

## Tests added (19: 8 stepfile + 10 scaffold + 1 cli)

`internal/platform/stepfile/depends_test.go` (new file, 8 tests):
satisfied-done-dependency, recorded-not-done-is-Known-and-unmet,
never-recorded-is-unmet-and-not-Known, zero-Frontmatter-sibling-is-
Known-and-unmet, done-dependant-exempt-even-with-unmet-dep,
empty-DependsOn-never-unmet, first-of-two-unmet-in-declaration-order, and
the index-keys-on-the-recorded-id-not-`fm.ID` pin (file 2's frontmatter
says `id: WRONG-02`, depended on as `STEP-02`, resolves).

`internal/scaffold/finish_depends_test.go` (new file, 10 tests):

- `Test_finish_refuses_a_step_whose_dependency_is_not_finished` — the core
  case; whole `*RefusalError` (path, `Line == 0`, `err.Error()` equal),
  `errors.Is(ErrUnmetDependency)`, both nothing-lands probes
  (`snapshotTree` byte-identity and `pinModTimes`/`modTimes`).
- `Test_finish_refuses_a_dependency_id_that_names_no_step_file` — the
  unknown-id branch, same two probes.
- `Test_finish_refuses_a_dependency_whose_step_file_does_not_parse` —
  Decision 5 made observable: garbage frontmatter on the depended-on
  sibling takes the "is not finished" branch, never `ErrMalformedFeature`.
- `Test_finish_refuses_a_step_that_depends_on_itself` — self-dependency,
  whole-line pin naming `STEP-02` on both sides.
- `Test_finish_accepts_a_step_with_no_declared_dependencies` — Decision 6's
  control arm: `depends-on: []` beside an unrelated not-done sibling still
  finishes; all four writes asserted on disk.
- `Test_finish_accepts_a_step_whose_dependency_is_done` — the satisfied
  side of the boundary.
- `Test_finish_reports_an_open_checklist_item_before_an_unfinished_dependency`
  — both-apply ordering, paired with mutation (c) below.
- `Test_finish_reports_an_unfinished_dependency_before_the_specification_read`.
- `Test_re_finishing_a_done_step_whose_dependency_is_open_with_recorded_inputs_stays_a_noop`
  and `..._with_divergent_inputs_is_still_ErrAlreadyFinished` — Decision
  4's pair, both against a reopened `STEP-01` on an already-finished
  fixture.

`internal/cli/finish_test.go`:

- `Test_finish_refuses_a_step_whose_dependency_is_unfinished` — through
  `cli.Run`, `Equal` on the whole stderr line, exit 1, empty stdout, and
  the four fixture files byte-identical before/after — green on arrival,
  no `cli` code change.

## Mutation verification (Step 18), each stashed/copied and restored
byte-identical, diffed to confirm

- (a) Invert `FirstUnmet`'s unmet condition (`!idx.recorded[dep]` →
  `idx.recorded[dep]`) → `Test_finish_refuses_a_step_whose_dependency_is_not_finished`
  goes red, confirming the scaffold side. On the assemble side the named
  plan test (`Test_status_counts_a_step_whose_dependency_is_unfinished_as_blocked`)
  does **not** redden under this exact mutation — it asserts only the
  `Blocked` *count*, and the mutation happens to swap which of two steps is
  wrongly counted (STEP-02 wrongly unblocked, STEP-03 wrongly blocked) while
  leaving the count at 1 by coincidence. The full `assemble` suite does
  redden, on `Test_status_reports_an_unknown_dependency_id_as_blocking`
  (an id nobody recorded is `idx.recorded[dep] == false`, so the inverted
  condition treats it as met). Recorded here for `check` (22) or a future
  pass: the named test is not, on its own, a sufficient discriminator for
  this mutation; the id-blocking test is.
- (b) Delete `FirstUnmet`'s `if fm.Done() { return "", false }`
  short-circuit → both Decision-4 tests in Step 14 go red (the no-op
  returns `ErrUnmetDependency` instead of `nil`; the divergent-input case
  returns it instead of `ErrAlreadyFinished`); nothing else in the suite
  moved.
- (c) Move `checkStepDependencies`'s call above `checkStepChecklist` →
  `Test_finish_reports_an_open_checklist_item_before_an_unfinished_dependency`
  goes red (dependency reported instead of the checklist item); nothing
  else moved.
- (d) Delete the `len(fm.DependsOn) == 0` early return → **green**, exactly
  as predicted: `FirstUnmet` already reports nothing for an empty list, so
  the early return is a pure optimization with no behavioural contract.
  Reported as green, not manufactured red.
- (e) Invert the `Known` condition in the copy-branch (`idx.Known(dep)` →
  `!idx.Known(dep)`) → both copy-branch tests redden and swap: the
  not-finished test renders the "names no step file" line, the unknown-id
  test renders the "is not finished" line — each caught by its own `Equal`
  on the whole rendered line.

Each mutation was applied to a single file, run, observed, then restored
from a `$TMPDIR` copy and `diff`-confirmed byte-identical before the next.

## Refactor note (lint, not behaviour)

First pass wrote `checkStepDependencies` as `(*RefusalError, error)`,
matching `findStepFile`'s two-value shape. `golangci-lint run ./...`
flagged it (`nilnil`) and separately flagged `Finish` itself
(`maintidx`, complexity 30 — confirmed via `git stash` that the
pre-scenario baseline was 0 issues, so both findings were introduced by
this scenario, not pre-existing). Collapsing the return to a single
`error` fixed both at once: one fewer conditional at the call site
dropped `Finish` back under the maintainability threshold. Also fixed a
`testifylint` `require-error` finding on one `assert.NotErrorIs` inside a
test that goes on to use the same `err` value via `require.ErrorAs`
immediately after. No behavioural test needed rewriting for any of this —
`go test ./...` stayed green throughout.

## Verification

- `go build ./...` — clean.
- `go vet ./...` — clean.
- `go test ./...` — exit 0, unpiped. 419 `--- PASS` lines (`grep -c --
  "--- PASS"`, unanchored) vs 400 before this scenario — delta +19,
  matching the 19 new tests exactly (8 stepfile + 10 scaffold + 1 cli). 0
  `--- SKIP`, 0 `--- FAIL`.
- `go test -race ./internal/scaffold/... ./internal/assemble/...
  ./internal/platform/stepfile/... ./internal/cli/...` — clean.
- `golangci-lint run ./...` — 0 issues (after the refactor note above).
- R11's no-op (`Test_re_finishing_a_done_step_with_the_same_inputs_leaves_every_file_byte_identical`,
  `..._preserves_every_modification_time`) and this scenario's own
  Decision-4 no-op both re-run green as the final check.
- Fixture sweep (Step 17): full suite run, zero pre-existing fixtures
  broke or needed repair — `newFinishFixture`'s STEP-01 already ships
  `status: done`, so every existing happy-path finish test stayed
  dependency-satisfied without change.

## Binding decisions

- `stepfile.DependencyIndex` is the single definition of "blocked" —
  `assemble.featureStatus`'s blocked count and `scaffold.Finish`'s refusal
  both call `FirstUnmet`. A second definition would let `status` and
  `finish` disagree about the same tree again, which they measurably did
  before this scenario.
- `Record` stores every step's doneness, keyed by whatever id the caller
  passes (`pattern.ID(n)` at every real call site, never `fm.ID`) — `Known`
  is what lets a caller distinguish "recorded but open" from "no such step
  file"; recording only done steps would silently break the unknown-id
  refusal copy.
- A done step is never dependency-refused — `FirstUnmet` short-circuits on
  `fm.Done()`, 09's own clause, reused verbatim rather than
  reimplemented — which is what keeps R11's no-op reachable when a done
  step's dependency is reopened by hand.
- Direct dependencies only; R4's transitive closure and `next`-by-dependency
  ordering stay unbuilt.
- Placement: after `checkStepChecklist`, before the specification read. A
  step both un-ticked and blocked reports the checklist item first.
- The scan is skipped entirely when `depends-on` is empty — an unrelated
  broken sibling step file can never affect an ordinary finish.
- A sibling that cannot be read or whose frontmatter does not parse is
  Recorded as a known, not-done step, never skipped — it blocks with "is
  not finished", never the wrong "names no step file" copy, and `finish`
  grows no separate malformed-sibling refusal for it.
- `checkStepDependencies` returns a plain `error`, not `(*RefusalError,
  error)` — a lint-driven shape, not a behavioural one; callers still
  recover the refusal with `errors.As`/`errors.Is` exactly as with every
  other refusal in this package.

## Left unbuilt / traps / debts — see rewritten STATE.md
