# SCENARIO-09 — Handoff

## What landed

- `(*assemble.Server).Status(ctx) ([]FeatureStatus, error)` in
  `internal/assemble/status.go`: enumerates feature directories under
  `s.cfg.FeatureDirectory` with `fs.ReadDir` (relying on its documented
  byte order, no `sort.Slice`), and per feature reuses `readSteps` +
  `stepfile.Pattern.Number`/`ParseFrontmatter` — never `markdown.Section`/
  `Title`. `FeatureStatus{Name, Done, Total, Next string, Blocked int}`;
  `Next` stays `""` when there is none (renderer's job to show `-`).
  Blocked = a not-done step whose `depends-on` names at least one id that
  is not a done step's `pattern.ID(n)` — direct dependencies only, unknown
  id blocks, a done step is never counted regardless of its own
  dependencies.
- `assemble.RenderStatusText(w, rows) error` in `internal/assemble/render.go`:
  `"%s %d/%d %s %d\n"`, `-` substituted only here for an empty `Next`, no
  header, no legend.
- `internal/cli/status.go`: `runStatus` mirrors `start.go` — `flag.FlagSet`
  named `status`, `--help` prints `statusUsage` to stdout, any positional
  argument or undefined flag is a usage error, `config.Resolve` → root →
  `assemble.NewServer` → `Status` → `RenderStatusText`, refusals through
  the existing `renderRefusal`.
- `internal/cli/cli.go`: dispatch `case "status"`, `usage` const gained a
  `brief status` line, both dispatcher messages now read
  `expected one of: new, start, finish, status`.
- `internal/assemble/doc.go`: "Start is the only entry point" replaced with
  "Start and Status are the entry points", and a sentence describing what
  Status computes.
- `cmd/brief/main.go`: unchanged — confirmed by reading; `cli.Run` was
  already the single dispatch point.

## Test count

267 → 279 (+12): 6 in `internal/assemble/status_test.go`, 1 in
`internal/assemble/render_test.go` (`Test_render_status_…`), 5 in
`internal/cli/status_test.go`. `go test ./...` exit 0, `go test -race
./internal/assemble/... ./internal/cli/...` exit 0, `golangci-lint run
./...` 0 issues (after fixing one `perfsprint` finding in the new
`status_test.go` fixture helper — a string-concatenation loop rewritten as
`strings.Builder`, same pattern the rest of the package already uses).

## Mutation verification (stashed to $TMPDIR, never `git stash`)

All five, one at a time, restored and diffed byte-identical after each:

- (a) prepend a header line inside `RenderStatusText` → reddened
  `Test_status_prints_one_line_per_feature_and_nothing_else` **and**
  `Test_status_prints_one_line_for_a_single_feature` (both exact-bytes CLI
  tests, Steps 11/12).
- (b) append a legend line after the loop → reddened the same two tests.
- (c) change `-` to `""` for an empty `Next` → reddened
  `Test_render_status_writes_four_space_separated_fields_per_feature`
  (Step 9).
- (d) count a done step's own unfinished dependency as blocked (added a
  blocked-check inside the done-counting loop) → reddened
  `Test_status_counts_a_step_whose_dependency_is_unfinished_as_blocked`
  (Step 3), Blocked went 1 → 2.
- (e) drop the `!done` guard from the blocked-counting loop (merged it into
  the `Next`-picking loop unconditionally) → reddened the same test the
  same way, Blocked 1 → 2.

Note: the architect's Step 3 fixture as first drafted had no *done* step
with an unfinished dependency, so mutations (d) and (e) could not have
reddened it — both silently no-op on a done step with an empty
`depends-on`. Added a fifth step to the fixture (`STEP-05`, done, depends
on the still-open `STEP-01`) before running the mutation sweep, asserting
`Blocked` stays `1` — this is the discriminating case for "a done step is
never counted, whatever its dependencies say."

## Deviation from the plan's literal per-step red/green grain

Steps 1, 3, 5, 6, 7 were written into `status_test.go` together, then
`status.go` was written once covering all of them, rather than cycling
red→green five separate times. The coarser-grained red (compile failure
covering the whole file) was confirmed before any production code existed,
and green was confirmed after. Steps 5, 6 and 7 were green on arrival, as
the plan anticipated and permits — reported here rather than manufacturing
an artificial red for any of them.

## Green on arrival (as anticipated by the plan)

- Step 5 (`Test_status_reports_an_unknown_dependency_id_as_blocking`):
  green immediately — the same `!done[dep]` check that handles a
  known-but-open dependency already handles an unknown one, since neither
  is ever inserted into `done`.
- Step 6 (no-next-step for a completed feature / zero step files): green
  immediately — `Next` only gets set inside the not-done branch, so both
  shapes leave it at its zero value `""`.
- Step 7 (byte-order fixture `Zeta`/`alpha`/`Beta`): green immediately, as
  the plan states — `fs.ReadDir`'s documented sort is what makes this pass
  with no sort call in `Status` at all.
- Step 15 (usage-error and `--help` CLI tests): green on first run —
  `runStatus` (Step 13) and the dispatch wiring (Step 14) already covered
  them; no separate red/green cycle was needed for these three.

## Left unbuilt (unchanged from the plan, restated for the next scenario)

- Missing/unreadable feature root → propagates, exit 1, empty stdout.
  SCENARIO-10 owns turning this into empty stdout + one stderr line + exit
  0.
- A step file whose frontmatter does not parse fails the whole command; no
  feature gets a line. SCENARIO-11 owns the `!` field.
- No `--json`, no `FeatureStatus` JSON tags, no R13 truncation.
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet` — not built.

## Traps for the next scenario

- `status` cannot read `brief`'s own tree today (no `SCENARIO-NN.md` here
  carries frontmatter) — same as `start`. Confirmed again this session:
  `go run ./cmd/brief status` → `brief status: assemble: no frontmatter
  found`, exit 1.
- A feature directory with zero step files renders `0/0 - 0`, not `!` —
  conforming, not malformed. SCENARIO-11's fixture must make its feature
  malformed some other way.
- `assemble.RenderText` (the `start` renderer) prints `b.Step.ID` (the
  frontmatter `id:`); `RenderStatusText` prints `pattern.ID(n)`. They agree
  in any scaffolded tree but diverge after a hand edit — do not "unify"
  them.
- SCENARIO-21's `finish` refusal must use the same done-set-by-`pattern.ID(n)`
  rule `Status`'s blocked computation uses, or the two surfaces disagree
  about the same tree; since `scaffold` cannot import `assemble`, 21 either
  reimplements it locally or the rule moves down to
  `internal/platform/stepfile`.

## Ticked by hand, not through `brief finish`

`docs/specifications/brief/specification.md`'s SCENARIO-09 line and this
handoff were written by hand, and `STATE.md` was rewritten by hand — no
`SCENARIO-NN.md` in this tree carries YAML frontmatter (checked again this
session), so `brief finish brief SCENARIO-09 …` cannot resolve the step.
Same route SCENARIO-07 and 08 took.
