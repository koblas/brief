# SCENARIO-22 Handoff: Check reports what the write path would now refuse

## What landed

New package `internal/platform/conform` (`doc.go`, `conform.go`,
`conform_test.go`): `Violation{Line, Problem, Fix, Err}` plus four pure
predicates — `OverCap`, `UnterminatedFence`, `MissingHeading`,
`OpenChecklistItem` — moved **verbatim** (problem/fix copy byte-identical)
out of `scaffold/finish.go`'s four private helpers, along with the four
sentinels (`ErrOverCap`, `ErrUnterminatedFence`, `ErrMissingStateHeading`,
`ErrOpenChecklistItem`). `scaffold/errors.go`'s four vars now alias
`conform`'s; `scaffold/finish.go`'s helpers became thin wrappers
(`refusalFromViolation`) that attach `Path`/`Line` to a `*Violation` and
return a `*RefusalError`. `errors.Is` and the exported surface are
unchanged; `finish_cap_test.go`, `finish_headings_test.go`,
`finish_checklist_test.go` were **green on arrival**, as predicted.

New `internal/assemble/check.go`: `Severity` (`ERROR`/`WARN`), `Finding{Severity,
Path, Line, Problem}`, `(*Server).Check(ctx, feature string) ([]Finding, error)`.
`feature == ""` walks every feature directory in `fs.ReadDir` order (dirs
only, matching `Status`); one name checks only that feature, returning bare
`ErrNoSuchFeature` (Start's own idiom, not a `*RefusalError`) when it has no
directory; a missing feature-directory root returns `(nil, nil)`. `Check`
walks step files itself — never `readSteps` — so one step's `C6` never
blinds the rest of the feature to `C7`-`C10` (mutation-verified, see
below). Severity is computed once per feature, after the whole step walk,
and back-filled onto every finding collected for it.

Ten rules: `C1` (spec, reusing `checkSpecification`) · `C2`-`C5` (state:
missing/unreadable, over cap, unterminated fence, missing heading — **not**
gated on each other, unlike `Finish`'s stop-at-first-fault band; `C4` reads
state bytes directly, never through `readStateFile`) · `C6`-`C10` per step,
ascending (frontmatter absent/unparseable; a **done** step's unticked item;
an unknown `depends-on` id; a self-dependency; the step's handoff file over
cap). Population narrowing is exactly two exemptions, both proven by a
control-arm test: an **open** step's unticked item, and an open step's
ordinary known-but-unmet dependency. Every other rule — including `C10`,
which fires on open and done steps alike — applies unnarrowed.
`RenderFindings` (`render.go`) renders `[SEVERITY] <path>:<line> — <problem>`,
one per line, no `Fix`.

New `internal/cli/check.go`: `runCheck`/`checkUsage`, dispatched from
`cli.go` (`usage`, both command-list messages, and the no-command/unknown-command
strings all updated — `run_test.go`'s two pinned strings were the only
fixture breakage, exactly as many as the suite found). Exit 0 unless an
`ERROR` finding printed (`errCheckFindings` sentinel, `ExitCode`'s default
branch); a run with zero findings, or a repo with no features, prints one
stderr line and nothing on stdout, exit 0 either way (SCENARIO-10's shape).
An unknown named feature is a one-line stderr refusal, exit 1, no
`(no files changed)` tail.

## The R14a/R18 collision — reconciled, not re-derived

R14a: refusals (exit 1) and findings (R9, exit 0, "never appear on a failed
run") are deliberately different shapes. R18: "`check` reports what
predates the tool and fails only on features still in flight." These
collide only because R14a's sentence is scoped to `finish`'s own R9
dropped-entry findings — a different report, on a different exit-0 command.
`check`'s exit code is governed by R18's word "fails", so `check` prints
findings **and** exits 1 in the same run when any is `ERROR`. This is
recorded as binding; do not re-derive it without reopening R18.

## check_drift_test.go — the proof this scenario exists for

`internal/cli/check_drift_test.go` calls `scaffold.Finish` and
`assemble.Check` directly (never through `cli.Run`) and asserts
`refusal.Problem == finding.Problem` for all four `conform` predicates.
Three pair on one fixture (same bytes recorded on disk fed back through
`--handoff`/`--state`, since the cap/fence/heading band runs ahead of the
re-finish verdict); `OpenChecklistItem` pairs across two fixtures differing
only in `status:`, since `Check` only ever reports a **done** step's open
item while `Finish` refuses an open one — still valid, because the problem
text depends on the item's own text alone.

`internal/assemble/check_test.go`'s own single-purpose `C10` test
(`Test_check_reports_an_over_cap_handoff_file`) deliberately asserts the
measured **data** (`Contains` "11", "10", "over the cap"), not the whole
sentence — pinning the same full string there would make it redden on the
identical mutation the drift test exists to catch, hiding which of the two
claims actually broke.

## Mutation verification (Step 29), each stashed via a `/tmp` copy, diffed
byte-identical after restore

- **(a)** Replaced `check`'s `conform.OverCap` call with a hand-written
  `fmt.Sprintf` carrying the same two numbers, different words (`"handoff
  line count %d is over the cap value %d"`) → `check_drift_test.go`'s
  over-cap case goes red; `check_test.go`'s own `Contains`-based C10 test
  stays green, exactly as designed.
- **(b)** Changed `conform.OverCap`'s `Problem` format string → **both**
  `scaffold`'s refusal bytes (`finish_cap_test.go`, collateral) and
  `assemble`'s finding move together, and the drift test correctly stays
  green — one definition working, not a vacuous assertion. Recording this
  so nobody later "fixes" the drift test's apparent lack of movement.
- **(c)** Removed C7's `if !ps.fm.Done() { return nil }` guard →
  `Test_check_reports_nothing_for_an_unticked_checklist_item_on_an_open_step`
  goes red (open step's item now reported).
- **(d)** Removed the C8/C9 `default: return nil` branch (made an ordinary
  known-but-unmet dependency report too) →
  `Test_check_reports_nothing_for_an_ordinary_unmet_dependency` goes red.
- **(e)** Added `continue` after the `C6` parse-error case, before the
  `C10` handoff check → `Test_check_reports_a_step_whose_frontmatter_does_not_parse_and_still_reports_its_handoff_cap`
  goes red (2 findings → 1): proves `Check` does not collapse into
  `readSteps`'s all-or-nothing stance.
- **(f)** Disabled the severity `inFlight` guard (always `WARN`) →
  `Test_check_assigns_ERROR_when_a_feature_is_still_in_flight_and_WARN_when_every_step_is_done`
  goes red at the `assemble` layer, **and** `internal/cli`'s
  `Test_check_prints_findings_on_stdout_and_exits_1_for_an_error_finding`
  goes red too (exit 0 instead of 1) — the guard is reachable at the R18
  exit-code boundary, not just inside `assemble`.
- **(g)** MOVE mutation on ordering: swapped `checkFeatureDir`'s two call
  sites so state (`C2`-`C5`) runs before spec (`C1`) →
  `Test_check_orders_findings_specification_then_state_then_steps_ascending`
  goes red (`expected[0]`/`expected[1]` swap in the diff) — a delete-either
  mutation would not have proven this; the ordering test's own load-bearing
  trap (17/18/19/21's own recorded gap) is now closed for `check` too.
- **(h)** MOVE mutation, second axis: swapped the `C3`(cap)/`C4`(fence)
  blocks inside `checkStateFindings` → the same test's `expected[1]`/`[2]`
  swap in the diff, confirming the *within-state* sub-order is pinned, not
  just spec-before-state.

## Verification

- `go build ./...`, `gofmt -l .`, `go vet` (via `golangci-lint`) — clean.
- `go test ./...` — exit 0, unpiped. **463** `--- PASS` lines (`grep -c --
  "--- PASS"`, unanchored) vs 419 before this scenario — delta **+44**,
  reconciling exactly: 10 `conform` + 19 `assemble/check_test.go` + 2
  `assemble/render_test.go` (`RenderFindings`) + 9 `cli/check_test.go` + 4
  `cli/check_drift_test.go` = 44. 0 `--- SKIP`, 0 `--- FAIL`.
- `go test -race ./internal/platform/conform/... ./internal/scaffold/...
  ./internal/assemble/... ./internal/cli/...` — clean.
- `golangci-lint run ./...` — 0 issues (fixed 3 `misspell` in
  `check_drift_test.go`'s own test names, 1 `nonamedreturns`, 2 `prealloc`,
  4 `testifylint` `require-error` in `conform_test.go`; confirmed via a
  captured-then-restored `git stash` that the pre-scenario baseline was
  also 0 issues, so all nine were introduced by this scenario).
- Fixture sweep (Step 27, by running the suite): exactly the two
  `run_test.go` pinned strings the plan named — no additional breakage,
  unlike 19/21's history.
- `main` prints nothing of its own on the `errCheckFindings` exit path — not
  merely asserted, empirically confirmed: the real binary's final run
  (below) captured stderr as exactly the one line `runCheck` itself writes.

## Real `brief check` against this repository's own tree (Step 30)

Built binary, run from the repo root with no arguments, **after** this scenario's own
`STATE.md`/`SCENARIO-22-HANDOFF.md` were written (so this is the true final count, not
the mid-scenario snapshot Step 10's `cap + 1` fix later moved the `:Line` field of).
Verbatim, final run:

- **42 findings, exit 1.** `[ERROR] .../STATE.md:81 — state is 95 lines, over the cap
  of 80` (1 — this file's own final length: distilling 21 scenarios' worth of decisions
  costs more than 80 lines even after a hard trim, an accepted, documented overage, not
  an oversight); `[ERROR] .../SCENARIO-01.md:0` through `SCENARIO-22.md:0 — frontmatter
  does not parse: no frontmatter found` (22 — one more than the architect's "21" because
  this scenario's own `SCENARIO-22.md` postdates that count); 19
  `[ERROR] .../SCENARIO-NN-HANDOFF.md:61 — handoff is <N> lines, over the cap of 60` (18
  pre-existing plus this scenario's own `SCENARIO-22-HANDOFF.md`, at 147 lines — writing
  a thorough final handoff for the last scenario of a 22-scenario feature outgrew the
  cap the same way every prior handoff did; `SCENARIO-HANDOFF-FILE.md` confirmed absent
  — its name matches neither the step pattern nor `pattern.ID(n)+"-HANDOFF.md"`). Every
  finding `ERROR`: the 22 unparsed step files make the feature read as in flight. Caps
  and tree left untouched, as instructed — this handoff documents the real number rather
  than trimming itself to dodge becoming one more line in its own report.

## Binding decisions — see rewritten STATE.md

## Left unbuilt / traps / debts — see rewritten STATE.md, all now unowned
(this is the last scenario)
