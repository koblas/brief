# SCENARIO-12 Handoff: A completed feature has no next step

## What was built

- `internal/cli/start.go` `runStart`: after a successful `srv.Start`, branches
  on `brief.Step == nil` and writes one of two notices to stderr, then
  returns before `assemble.RenderText` is ever called:
  - `Done+Open > 0` (every step done): `brief start: <abs feature dir>:
    feature is complete, <done> of <total> steps done; run 'brief new step
    <feature>' to add the next one`
  - `Done+Open == 0` (no step files at all): `brief start: <abs feature
    dir>: no step files yet; run 'brief new step <feature>' to scaffold the
    first one`
  Both notices name the feature directory as an absolute path
  (`filepath.Join(root, cfg.FeatureDirectory, feature)`), exit 0, write
  nothing to stdout, and carry no `(no files changed)` tail — the
  read-command rule R14a already set for `status`'s no-features notice.
- `startUsage` now states the complete/empty case, mirroring how
  `statusUsage` documents its own no-features notice.
- `internal/assemble/render.go` `RenderText`'s doc comment: replaced the
  `SCENARIO-12` reference with the durable rule — "RenderText writes nothing
  when b.Step is nil; the caller decides what to say about a feature with no
  open step." `RenderText`'s own nil-`Step` early return is untouched; it
  stays as defense-in-depth for any future caller, but `runStart` no longer
  reaches it on this path.
- `assemble.Start`'s signature and behaviour are unchanged — it still
  returns `(Brief, nil)` for a complete or empty feature, never an error.
  Nothing here reads `cfg.SpecificationFile` or `cfg.ProgressHeading`, so
  SCENARIO-13's red is untouched.

## Tests added (4 new; 302 → 306, 0 skips, 0 fails)

`internal/cli/start_test.go` (3, plus one existing test amended):

- `newStartFixture` parameterized by the frontmatter `status:` value so the
  complete and open arms differ in exactly one variable; its one caller now
  passes `"open"`.
- `Test_prints_the_brief_and_writes_nothing_to_stderr` (existing, amended):
  added `assert.NotEmpty(t, stdout.String())` — the positive-stdout half of
  the pairing that makes the new tests' "stdout is empty" claim falsifiable
  on the same probe (`stdout.String()`).
- `Test_start_says_the_feature_is_complete_when_every_step_is_done` (new):
  `newStartFixture(t, "done")` — one step, done. Pins the exact stderr line,
  empty stdout, no error. Red on arrival (stderr was empty before this
  scenario); green after the `runStart` branch landed.
- `Test_start_says_there_are_no_step_files_yet_for_an_empty_feature` (new):
  a hand-built feature directory holding only `STATE.md`, no step files.
  Pins the second notice, empty stdout, no error. Red on arrival, green
  after.
- `Test_start_still_refuses_a_feature_with_no_state_file` (new): a feature
  directory with a step file but no `STATE.md` — the pre-existing
  `ErrMalformedFeature` path, now asserted at the `cli` level for the first
  time (previously only `assemble_test.go` covered it). **Green on
  arrival** — the new nil-`Step` branch sits entirely after the
  `srv.Start` error check, so it was never at risk of swallowing this
  refusal; the test exists to make that fact checkable going forward.

`internal/cli/status_test.go` (1, new):

- `Test_status_shows_a_dash_for_a_completed_feature`: `brief status` on a
  feature whose two steps are both done prints `alpha 2/2 - 0`. **Green on
  arrival** — SCENARIO-09 already built the `-` substitution in
  `RenderStatusText`; this is the first test that exercises it end-to-end
  through the CLI rather than only at the `FeatureStatus`/`RenderStatusText`
  level.

## Mutation verification (both, one at a time, backed up to `$TMPDIR`, diffed byte-identical after restore)

- Deleted the entire `if brief.Step == nil { … }` block in `runStart`
  (still compiles: `RenderText` no-ops on a nil `Step`, so stdout stayed
  empty). `Test_start_says_the_feature_is_complete_when_every_step_is_done`
  and `Test_start_says_there_are_no_step_files_yet_for_an_empty_feature`
  both went red on the stderr assertion alone (expected notice vs. empty
  string); `Test_prints_the_brief_and_writes_nothing_to_stderr` (open-step
  control arm) stayed green throughout.
- Removed the `if next == "" { next = "-" }` substitution in
  `RenderStatusText`. `Test_status_shows_a_dash_for_a_completed_feature`
  went red (`alpha 2/2 - 0` vs. `alpha 2/2  0`), confirming the assertion is
  not vacuous even though it was green on arrival.

## Binding decisions (repeated from the plan as the record)

- `Brief.Step` stays `*Step`; nil means "no open step"; `assemble.Start`
  keeps returning `(Brief, nil)` for it, never an error — SCENARIO-15's
  `--json` will marshal that pointer directly as R14's `"step": null`.
- The notice lives in `cli.runStart`, keyed on `brief.Step == nil`, written
  inline with `fmt.Fprintf(stderr, …)` followed by `return nil` — the same
  split SCENARIO-10 used for `runStatus`'s `len(rows) == 0` notice.
  `assemble` returns data; `cli` renders copy.
- Two notices, not one: `Done+Open > 0` → "feature is complete"; `Done+Open
  == 0` → "no step files yet". A bare `brief new feature x` leaves a
  directory SCENARIO-11 already ruled conforming, and it hits the same
  nil-`Step` branch — one merged notice would call that empty feature
  "complete".
- Both notices name the feature directory as an **absolute** path, per the
  rule settled here because SCENARIO-13 renders on the same path: a message
  naming a specific on-disk object is absolute; a message naming a
  configured location that is not one object stays config-relative.
- Read notices and read refusals both drop R14a's `(no files changed)`
  tail; only write refusals carry it.

## Left unbuilt

- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`, `--json` (15),
  `assemble.Problem.Line`, `markdown.Headings`, R13 truncation — all still
  unbuilt.
- No refusal for a missing progress list — SCENARIO-13 owns it; `Start`
  still never opens `cfg.SpecificationFile`.
- No optional-convention shortfall notice — SCENARIO-14 owns it. `start` can
  emit at most one stderr line today; 14 is what makes two possible.

## Traps for the next reader

- `brief start <f> | wc -c == 0` is exit-code-conditional: `start nosuch`
  and `start` on a feature with no state file both produce 0 stdout bytes
  at **exit 1**. The scriptable "complete" test is `exit 0 && wc -c == 0`,
  never byte count alone.
- `"step": null` cannot distinguish complete from zero-steps — both are
  `Step == nil`. SCENARIO-15 must use `done`/`open` (`2`/`0` vs `0`/`0`) to
  tell them apart, or accept the conflation deliberately.
- `RenderText`'s own nil-`Step` guard is never reached from `runStart`
  anymore (it returns before calling `RenderText`); the guard is covered
  only by `assemble/render_test.go`'s
  `Test_RenderText_writes_nothing_when_there_is_no_next_step` — deleting it
  will not redden anything in `cli`.
- `brief finish <feature> <step>` takes the step **id** (`SCENARIO-01`), not
  the number, if a fixture is ever driven through the real binary instead of
  written by hand.
