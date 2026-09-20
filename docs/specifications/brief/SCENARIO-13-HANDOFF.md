# SCENARIO-13 Handoff: Start refuses a malformed feature rather than assembling half a brief

## What was built

- `internal/assemble/errors.go`: `Problem` (Path/Detail/Fix) and `newProblem`
  moved here from `status.go`, unchanged in behavior. Added
  `RefusalError` — embeds `Problem`, plus `Line int` and `Err error`, with
  `Error()` (`"<path>: <detail>; <fix>"`, or `"<path>:<line>: …"` when `Line`
  is set) and `Unwrap() error` returning `Err`. Named `…Error` because
  `errname` is enabled (`.golangci.yaml`'s `default: all`).
- `internal/assemble/assemble.go` `(*Server).Start`: now checks, in order,
  specification → state file → step files → the briefed step, first
  failure wins:
  - `checkSpecification` (new): refuses absent (own imperative — "write a
    `<file>` with a `<heading>` heading and re-run", never "make it
    readable", never `brief new feature`), unreadable (`newProblem`,
    `readClassFix`), an unclosed fence (`Line` set, `markdown.UnterminatedFence`),
    or a missing `cfg.ProgressHeading` (`markdown.Section`, wording copied
    verbatim from `scaffold.progressRefusal`/`scaffold.NewStep`'s own
    refusal text since `assemble` must not import `scaffold`).
  - `readStateFile` (renamed/reshaped from the inline block that used to
    live in `Start`): same absent/unreadable/fence shape, reusing
    `newProblem` for absent/unreadable — behavior for those two was already
    correct, only the return type changed from a bare `fmt.Errorf`-wrapped
    `ErrMalformedFeature` to a path+fix-carrying `*RefusalError` that still
    satisfies `errors.Is(err, ErrMalformedFeature)`.
  - `readSteps`'s error (frontmatter absent or unparseable, shared with
    `Status`, untouched itself) is now wrapped as
    `&RefusalError{Problem: *newProblem(featurePath, err, true), Err: err}`
    — `Err` is the **real** underlying error, not a synthesized sentinel:
    for the absent-delimiter case that still wraps `stepfile.ErrNoFrontmatter`
    (`errors.Is` unaffected); for a delimiter-present-but-invalid-YAML case
    it wraps the raw yaml decode error instead, and does **not** satisfy
    `errors.Is(err, stepfile.ErrNoFrontmatter)` — deliberately, see Traps.
    `newProblem(base=featurePath, nameable=true)` is called exactly as
    `featureStatus` already calls it, so the two known behaviors are
    inherited unchanged: a `*fs.PathError` (open/read failure) names the
    specific step file; a YAML-content failure names only the feature
    directory (`newProblem`'s pre-existing, documented limitation — not
    fixed here).
  - Two new checks on the **briefed** step only (never any other step):
    empty `fm.ID` (presence only, never compared to `pattern.ID(n)`) and
    `!Checklist.Found`. Both wrap `ErrMalformedFeature`, name the briefed
    step's absolute path via `pattern.Name(e.number)` joined onto
    `featurePath`, `Line` unset.
- `internal/assemble/status.go`: unchanged in behavior; `Problem`,
  `readClassFix` and `newProblem` deleted (moved to `errors.go`); unused
  `strings` import dropped.
- `internal/cli/refusal.go` `renderRefusal`: added an
  `*assemble.RefusalError` branch beside the existing
  `*scaffold.RefusalError` one, rendering `brief <cmd>: <path>[:<line>]:
  <detail>; <fix>` with **no** `(no files changed)` tail. This branch is
  behavior-neutral today — `RefusalError.Error()` already produces
  byte-identical text through the generic fallback branch, confirmed by
  `Test_start_refuses_a_specification_with_no_progress_heading` passing
  green *before* this branch was added. Added for the same reason
  `*scaffold.RefusalError` gets one despite also having a working
  `Error()`: decouples cli's rendering contract from the error type's own
  `Error()` format.
- `internal/assemble/doc.go`, `internal/cli/start.go` `startUsage`: state
  the refusal contract as a rule (what refuses vs. what is optional/left
  to SCENARIO-14), no scenario ids in the prose.

## Tests added (7 new; 306 → 313, 0 skips, 0 fails)

`internal/assemble/assemble_test.go`:

- `Test_refuses_a_specification_that_does_not_exist` (new, **the headline
  red the architect's plan had no line item for** — see Traps): single-
  variable change from `newFixture`'s control arm (removes `SPEC.md`).
  Asserts `ErrMalformedFeature`, absolute `SPEC.md` path, `Line == 0`, and
  `refusal.Fix` never contains `"brief new feature"`.
- `Test_refuses_a_specification_with_no_progress_heading` (Step 4): single-
  variable change from the same control arm (rewrites the body). Asserts
  the configured heading text in both `Detail` and `Fix`.
- `Test_refuses_a_specification_whose_fence_is_unterminated` (new, mirrors
  the existing state-file fence test 1:1 — the architect's plan didn't
  list this one either): asserts `Line == 1` and `"unclosed"` in `Detail`.
- `Test_refuses_the_briefed_step_when_its_frontmatter_carries_no_id`
  (Step 11): blanks only `newFixture`'s STEP-03 (the step Start would
  brief) id.
- `Test_refuses_the_briefed_step_when_its_checklist_heading_is_absent`
  (Step 13): same STEP-03, drops the checklist heading entirely.
- `Test_a_step_with_an_empty_but_present_checklist_stays_conforming` (new
  control arm for the test above): STEP-03 done, STEP-04 rewritten with an
  empty-but-present checklist becomes the briefed step; proves
  present-but-empty stays conforming (the `new step` → `start` path this
  scenario must not break).

`internal/cli/start_test.go`:

- `Test_start_refuses_a_specification_with_no_progress_heading` (Step 8):
  single-variable change from `Test_prints_the_brief_and_writes_nothing_to_stderr`'s
  control arm. Exit 1, `stdout.Len() == 0`, exactly one stderr line, no
  `(no files changed)` tail, line names the absolute specification path
  and the configured heading.

`Test_start_still_refuses_a_step_file_whose_frontmatter_does_not_parse`
(existing, **re-pointed, not merely updated** — see Traps) now uses a
step file with well-formed delimiters and invalid YAML content
(`status: [open` unterminated), and asserts `NotErrorIs(ErrNoFrontmatter)`
+ `ErrorAs(*assemble.RefusalError)` + `Detail` contains `"yaml"`, rather
than the `ErrorIs(ErrNoFrontmatter)` it used to share with the absent-
frontmatter test above it (both tests used to write the byte-identical
"no frontmatter here" fixture).

## Fixture updates (10 existing tests touched, 4 more than the plan's own count of 6)

The plan named 2 in `start_test.go` and 4 in `assemble_test.go`. The
advisor call caught 4 more the plan missed, all with the same failure mode
(a conforming spec added now, or they'd refuse once check 1 landed):

`internal/cli/start_test.go`:
- `newStartFixture` (shared by 2 tests) — added a conforming
  `specification.md`.
- `Test_start_says_there_are_no_step_files_yet_for_an_empty_feature` —
  added conforming spec.
- `Test_start_still_refuses_a_feature_with_no_state_file` — added
  conforming spec (needed so this test reaches the *state* check, not the
  spec check); tightened its stderr assertion to `Contains …/STATE.md`
  (was `NotEmpty`).
- `Test_prints_the_full_checklist_when_it_contains_a_nested_fence` — **not
  in the plan's list** — added conforming spec.
- `Test_prints_the_brief_from_a_CRLF_step_file` — **not in the plan's
  list** — added a CRLF-consistent spec, to keep the CRLF fixture a
  single-variable story.

`internal/assemble/assemble_test.go`:
- `Test_returns_an_error_when_the_state_file_is_missing`,
  `Test_refuses_a_state_file_whose_fence_is_unterminated`,
  `Test_returns_an_error_when_a_step_file_has_no_frontmatter`,
  `Test_start_still_refuses_a_step_file_whose_frontmatter_does_not_parse`
  — added conforming spec to each (per plan).
- `Test_takes_the_lowest_numbered_open_step_not_the_first_in_directory_order`,
  `Test_returns_no_next_step_when_every_step_is_done` — **not in the
  plan's list** — added conforming spec to each.

`internal/assemble/status_test.go` and `render_test.go` needed no changes:
`Status` never reads the specification (confirmed by grep — no
`SpecificationFile`/`srv.Start` reference in `status_test.go`), and
`render_test.go` builds `Brief` values directly, no file I/O.

## Mutation verification (three, one at a time, backed up to `$TMPDIR`, diffed byte-identical after restore)

- Deleted the progress-heading check in `checkSpecification`.
  `Test_start_refuses_a_specification_with_no_progress_heading` reddened
  on the **stdout-emptiness assertion specifically**: exit 0,
  `stdout.Len() == 249` (the full brief), not a compile failure — the
  discriminating proof the plan called for, since the fixture is otherwise
  well-formed.
- Deleted the empty-`fm.ID` check in `Start`'s briefed-step loop.
  `Test_refuses_the_briefed_step_when_its_frontmatter_carries_no_id`
  reddened: "Expected error with 'malformed feature' in chain but got
  nil."
- Deleted the `!Checklist.Found` check in the same loop.
  `Test_refuses_the_briefed_step_when_its_checklist_heading_is_absent`
  reddened identically.

Step 16's optional second proof (hoist `RenderText` above the error
branch, return a populated `Brief` alongside a refusal) was **skipped** —
optional per the plan, and every new refusal test already asserts
`assert.Nil(t, brief.Step)` on the returned zero-value `Brief{}`, which is
what `Start` actually returns on every refusal path (never a populated one
alongside an error).

## Binding decisions (repeated from the plan as the record)

- Check order: specification → state file → step files → briefed step,
  first failure wins. Never reordered.
- All eight refusals name an absolute path; `Line` is set only for the two
  unclosed-fence cases.
- Read refusals carry no `(no files changed)` tail.
- The `id:` check is presence-only, on the briefed step only, never
  compared to `pattern.ID(n)`.
- Present-but-empty stays conforming for both the progress list (unchanged
  from before this scenario) and the checklist (proved by the new control
  arm above).
- `RefusalError.Err` is always the real error the refusal concerns — never
  a synthesized/umbrella sentinel. Concretely: the frontmatter-refusal's
  `Err` is whatever `readSteps` actually produced, so
  `errors.Is(err, stepfile.ErrNoFrontmatter)` is true only when the real
  cause was the missing/unclosed delimiter, and false for a genuine YAML
  content error.

## Left unbuilt

- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`, `--json` (15),
  `markdown.Headings` + checklist parser (20/22), R13 truncation, R9's
  diff/finding output, `FinishResult`.
- No optional-convention shortfall notice — SCENARIO-14 owns it: absent
  `cfg.AcceptanceHeading`, anything in `cfg.OptionalConventions`, an absent
  state heading in an otherwise-readable state file. `start` can emit at
  most one stderr line until then.
- A refusal for a missing `status:` key — deliberate, unowned:
  `Frontmatter.Done()` defaults it to "open" and `finish` refuses with
  `ErrNoStatusField` at the write path (R18).
- `ErrNoSuchFeature` still does not list known features (R14). Unowned.
- `status` does not adopt 13's checks — a feature with no specification is
  still a normal row to `status` and exit 1 to `start`. Owner: `check`
  (22), reusing `checkSpecification`.
- The `config.InvalidConfigError` tail inconsistency (still renders the
  tail on a read command) — pre-existing, unowned.

## Traps for the next reader

- **The architect's plan named 6 fixtures needing a conforming spec; 4
  more needed one and weren't listed** (see Fixture updates above) — a
  fresh read of the plan alone would have left 4 tests broken. Caught by
  an advisor consult before writing production code; grep the whole test
  file for `cfg.StateFile`/`"STATE.md"` writes with no matching
  `cfg.SpecificationFile` write nearby before touching `Start` again.
- **Two names for one check.** The plan's own text ("Wording for the
  step-file case is already in `assemble.newProblem`; reuse it as-is")
  cannot be satisfied literally for the id/checklist checks (7, 8): those
  need the **briefed step's own path**, but `newProblem`'s non-`*fs.PathError`
  branch can only name `base` (whatever the caller passes), and `base`
  must stay the **feature directory** for the frontmatter-parse-failure
  branch (`Status`'s `Test_status_marks_a_feature_whose_step_frontmatter_does_not_parse`
  pins `Problem.Path == deltaDir` exactly). Checks 7/8 are therefore
  hand-built in `Start`, not routed through `newProblem`; check 6 (readSteps)
  is routed through it unchanged.
- **`RefusalError.Err` is not always `ErrMalformedFeature`.** Only checks
  1–5, 7 and 8 wrap it. Check 6 (step frontmatter) wraps whatever
  `readSteps` actually produced, which may be `stepfile.ErrNoFrontmatter`,
  a bare parse error, or (for a read failure) an `*fs.PathError`. A future
  caller that does `errors.Is(err, ErrMalformedFeature)` expecting it to
  catch *every* Start refusal will miss check 6's YAML-content case —
  this is deliberate (see Binding decisions), not an oversight, but it is
  a real asymmetry.
- **The generic `renderRefusal` fallback already renders `*assemble.RefusalError`
  correctly** — the dedicated branch added in this scenario is currently
  behavior-neutral (confirmed: the cli-level red test passed before the
  branch existed). If a future change to `RefusalError.Error()`'s format
  and the dedicated branch's own formatting ever diverge, they will now
  silently disagree depending on whether `errors.AsType` matches before
  the generic branch — keep the two in sync by hand, same as
  `*scaffold.RefusalError` already requires.
- `newProblem`'s non-`*fs.PathError` branch still cannot name a specific
  step file when the frontmatter itself (not its file open) fails to
  parse — pre-existing, documented in `newProblem`'s own comment, not
  fixed by this scenario. `Start`'s check 6 inherits this exactly as
  `Status` already has it.
- This repo's own `docs/specifications/brief/specification.md` and
  `STATE.md` were **not modified structurally** by this scenario (only the
  progress-list tick and this file). `brief start brief` still fails with
  `no frontmatter found` (`SCENARIO-01.md`…`-12.md` carry no `id:`), for
  the same reason as before — checks 1–5 all pass on this repo's own tree.
  **The crossover must write `id:` as well as `status:`** into every
  `SCENARIO-XX.md` frontmatter, or check 7 (empty id) refuses `brief start
  brief` for a new reason once the frontmatter exists at all.
