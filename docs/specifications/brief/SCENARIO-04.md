# SCENARIO-04: A start brief carries everything a step needs

## Scenario

```gherkin
Scenario: SCENARIO-04 A start brief carries everything a step needs  [orig: 01]
  Given a feature with two finished steps and three open
  And the finished steps recorded decisions and constraints
  When I start the feature
  Then I get the next step's id, title, acceptance criteria and checklist
  And I get the decisions and constraints from the finished steps
  And I get the counts "2 done, 3 open"
  And I get no other step's acceptance criteria
  # The last assertion is an absence claim and passes vacuously on a single-step fixture.
  # It requires a fixture of at least three steps, and a control arm proving the other
  # steps' acceptance criteria are readable from the same files by the same probe — so
  # that deleting the filter reddens this scenario. See .claude/rules/agent-briefs.md.
  And nothing is written to disk
```

---

## Decisions this scenario takes

### 1. `start` lives in a new feature package `internal/assemble`

`internal/scaffold` owns writing; `assemble` owns reading. It is a **feature package**, not a
platform package, because it holds feature knowledge (which step is next, what a brief
contains). It imports `internal/platform/{config,stepfile,markdown}` and the standard library
only — never `internal/scaffold`, never `internal/cli`.

**Precedent for the whole read surface.** `status`, `next`, `show`, `handoff` and `state get`
join this package as further methods on the same `Server`, for the reason `scaffold`'s own
package doc already records about `new feature`/`new step`: they compute the same
feature-directory layout, and splitting them across feature packages would force a shared types
package for nothing — and the dependency rule forbids them importing each other. `finish`
**writes**, so it is not settled here; SCENARIO-05 chooses between `internal/scaffold` and a new
package.

`NewServer(cfg config.Config, root string) *Server` — positional, mirroring
`scaffold.NewServer`. There is no optional dependency for a functional option to default; the
filesystem is reached directly through `os.Root`, exactly as `scaffold` does, because the
contracts here ("nothing is written to disk") are filesystem properties no in-memory adapter
models.

### 2. "Next" is the lowest-numbered step whose frontmatter `status:` is not `done`

- Step files are found by scanning the feature directory and keeping every entry
  `stepfile.Pattern.Number` accepts — the sole matching authority, per STATE.md.
- Order is **numeric on the step number**, explicitly *not* directory order and *not*
  lexicographic. Under `%02d` these coincide; under `%d` they do not, and one test pins it.
- `depends-on` is parsed but **ignored for ordering**. R4's transitive closure is a later phase
  (`## Phasing`, *Later phases*); list/number order is the rule until then.
- Doneness is `status:` alone (STATE.md, binding). `done` after trimming and case-folding is
  done; **every other value counts as open**, including `blocked`. Named as a revisit for
  SCENARIO-09, whose four-field line carries a *blocked count* and will need a third state.
- Counts: `Done` = steps whose status is done; `Open` = total − Done.

### 3. Section extraction: `internal/platform/markdown`, fence-aware

New platform package, because `show`, `handoff`, `finish` and `check` all need it.

- `Section(body, heading string) (string, bool)` — finds the first line equal to `heading` after
  right-trimming; returns the body **below** it, leading and trailing blank lines trimmed,
  excluding the heading line. The section ends at the next heading of the **same or higher
  level** (`##` ends a `##`; `###` does not), or at EOF. Missing heading → `("", false)`.
- **Lines inside a fenced code block (``` or ~~~) are never headings.** This is load-bearing
  here, not hypothetical: this repo's acceptance criteria are fenced gherkin blocks containing
  `#` comment lines, so a non-fence-aware scanner truncates every brief mid-section.
- `Title(body string) (string, bool)` — the text of the first level-1 (`# `) heading,
  fence-aware.
- No AST-and-re-render library, per the triage brief's binding note.

### 4. **FLAG — "acceptance criteria" is a new *optional* configured heading; `new step` is NOT changed**

The scaffold from SCENARIO-03 writes frontmatter, `# <id>`, `## Implementation Plan`,
`## Handoff` — and no acceptance section. The Gherkin asks for acceptance criteria as a
distinct thing, so one of three had to be true. **Decision: a new config key
`acceptance-heading`, default `"## Scenario"`, read when present and not written by `new
step`.** Evidence:

- **R2's malformed list** — progress list, step files, checklist, handoff anchor, state file —
  does **not** include acceptance criteria. Making it structural would contradict R2; making it
  optional is exactly what R2's "conventions marked optional degrade instead" describes, and
  `Config.OptionalConventions` is otherwise dead.
- **SCENARIO-03's own Gherkin enumerates what the scaffold writes** and omits it. Amending
  `stepSkeleton` would re-open a shipped scenario.
- **SCENARIO-02: "no prose has been written into any of them."** Acceptance criteria is prose;
  the *planner* writes it (R15, `## First run` item 4). The tool writes structure only.
- **The default is lifted from this repo**, per `## Default profile`: every `SCENARIO-XX.md`
  here carries the Gherkin under `## Scenario`. The shipped default therefore fits this tree
  with no config, which is the stated design goal.
- `## Implementation Plan` is **not** doing double duty: it is the checklist the Gherkin asks
  for separately, and SCENARIO-20 parses its `- [ ]` items.

Consequences, stated so they are not absorbed silently: a feature scaffolded purely by `brief
new step` has no acceptance section until the planner writes one, and `start` then emits the
brief without it. **Naming the shortfall on stderr is SCENARIO-14's**, not this scenario's.
Opt-in semantics: the section is extracted **whenever the heading is present**, regardless of
`optional-conventions`; that list governs only whether *absence* is reported — SCENARIO-14's
business.

*If you want the scaffold to write a bare `## Scenario` heading instead, that is a change to
shipped SCENARIO-03 (`stepSkeleton`, its test, and the R2 malformed list) and this plan must be
revised before Step 1.*

### 5. Frontmatter parsing goes into `internal/platform/stepfile`

`stepfile.Frontmatter{ID, Status, DependsOn}`, `ParseFrontmatter(body []byte) (Frontmatter,
[]byte, error)` returning the remaining body, `ErrNoFrontmatter`, and `Frontmatter.Done()`.
It lives in `stepfile` rather than in `assemble` because `finish` (05), `status` (09) and
`check` (22) all read the same fields and no feature package may import another. `yaml.v3` is
already a direct dependency.

### 6. A missing state file is refused; SCENARIO-13 owns the copy

R2 lists the state file as structural and R10 says `start` refuses a malformed feature rather
than assembling half a brief — "a brief that looks complete while silently omitting inherited
constraints is the worst output this tool could produce." So `start` **returns an error and no
partial brief**. This scenario pins only *that* (`errors.Is(err, assemble.ErrMalformedFeature)`
and empty stdout); it pins **no user-facing copy**. **SCENARIO-13 owns** the R14a read-refusal
template, `RefusalError.Path/Problem/Fix`, and the missing-handoff-anchor case. Until then
`start`'s errors render through `renderRefusal`'s existing flatten fallback as
`brief start: <cause>`, exit 1.

R6's synthesis-from-handoffs fallback is a later phase and is **not** built: `start` reads the
state file only.

A state file present but missing one of the four configured headings renders as an empty
section here (see 7). Validating the state body's headings is SCENARIO-19, on the write path.

### 7. Output shape — **the specification is silent; this is the architect's choice**

`## Product Verdict` fixes no literal stdout for `start`; it fixes only the surrounding rules
(payload to stdout, diagnostics to stderr, R14's nothing-to-return contract). Pinned here:

```
STEP-03 — 2 done, 3 open

# STEP-03 Assemble the brief

## Fixture Scenario

<acceptance body, verbatim>

## Fixture Checklist

<checklist body, verbatim>

## Decisions Fixture

<state body, verbatim>

... the remaining state sections, in config order ...
```

- First line is `<id> — <done> done, <open> open`: unconditional, carries the id the Gherkin
  asks for, and never duplicates an id the title already contains.
- Every heading is the **configured text, verbatim** — the payload is markdown the caller can
  paste, not a re-rendered format.
- `Brief.Inherited` carries **all four state sections, empty body included**, so SCENARIO-15's
  JSON encoder gets stable keys. The **text renderer** skips a section with an empty body; the
  omission is a rendering rule, not a loading one.
- `RenderText` lives in `internal/assemble/render.go`, not `internal/cli`, for the reason
  `scaffold/render.go` already sets: the markdown body *is* the product's payload. `internal/cli`
  keeps exit codes, usage copy, stderr diagnostics, and — later — the `--json` choice, which
  encodes the same `Brief` struct rather than re-parsing this text.
- `Brief.Step` is a **pointer**, nil meaning no next step — SCENARIO-15's `"step": null`
  discriminator and SCENARIO-12's empty stdout. This scenario pins only `Step == nil` and that
  `RenderText` writes nothing for it. **SCENARIO-12 owns** the stderr "feature is complete"
  line and the exit-0 command contract; **SCENARIO-15** owns `--json`.

### 8. Deliberately not built

`--json`; the complete-feature command contract; the malformed-feature refusal copy; the
optional-convention shortfall; `finish`; `status`; `check`; R13 truncation (`Brief` carries no
budget field); dependency ordering; the R6 synthesis fallback; known-feature enumeration in the
unknown-feature error (SCENARIO-09, per STATE.md).

## User-visible contract

```
brief start <feature>
```

| Case | stdout | stderr | exit |
| --- | --- | --- | --- |
| happy path | the brief above | empty | 0 |
| `brief start` (no feature) | empty | `brief start: no feature given; run 'brief start <feature>'` | 2 |
| `brief start a b` | empty | `brief start: too many arguments; run 'brief start <feature>'` | 2 |
| `brief start --bogus f` | empty | `brief start: flag provided but not defined: -bogus; run 'brief start <feature>'` | 2 |
| `brief start --help` | `startUsage` | empty | 0 |
| unknown feature | empty | one flattened line, copy not pinned | 1 |
| feature with no state file | empty | one flattened line, copy not pinned | 1 |
| step file with unparseable frontmatter | empty | one flattened line, copy not pinned | 1 |
| feature name escaping the feature root | empty | one flattened line, copy not pinned | 1 |

`cmd/brief/main.go` needs **no change** — it already calls `cli.Run` and `cli.ExitCode`.

## Fixture

One helper, `newFixture(t)`, writing under `t.TempDir()` using a `fixtureConfig()` whose every
read field differs from `config.Default()` (mirror `scaffold_test.go`'s): `FeatureDirectory
"specs"`, `SpecificationFile "SPEC.md"`, `StateFile "NOTES.md"`, `StepFilePattern
"STEP-%02d.md"`, `ProgressHeading "## Progress"`, `ChecklistHeading "## Fixture Checklist"`,
`HandoffHeading "## Fixture Handoff"`, `AcceptanceHeading "## Fixture Scenario"`, and
`scaffold_test.go`'s four fixture `StateHeadings`.

Feature `demo` with **five** step files:

- `STEP-01.md`, `STEP-02.md` — `status: done`; each carries `## Fixture Scenario` with a unique
  marker (`ACCEPTANCE-01`, `ACCEPTANCE-02`) **inside a fenced gherkin block containing a `#`
  comment line**, a checklist, and `## Fixture Handoff` with a unique marker
  (`HANDOFF-ONLY-01`, `HANDOFF-ONLY-02`) appearing nowhere else.
- `STEP-03.md` — `status: open`, title `# STEP-03 Assemble the brief`, acceptance marker
  `ACCEPTANCE-03` (fenced, with a `#` comment line), checklist markers `CHECKLIST-03-A` and
  `CHECKLIST-03-B`.
- `STEP-04.md`, `STEP-05.md` — `status: open`, markers `ACCEPTANCE-04`, `ACCEPTANCE-05`,
  same shape.
- `NOTES.md` — the four fixture state headings, each with a non-empty body carrying a unique
  marker (`STATE-DECISION-A (STEP-01)`, `STATE-DECISION-B (STEP-02)`, `STATE-UNBUILT-A`,
  `STATE-TRAP-A`, `STATE-DEBT-A`).
- `SPEC.md` — title plus `## Progress` with five entries.

Next step is **STEP-03**: neither the first nor the last file, so a "take the first" or "take
the last" mutation reddens. Counts 2/3 are asymmetric, so a done/open inversion reddens.

**Write the frontmatter byte-for-byte as `scaffold.stepSkeleton` emits it** (`---\nid: …\nstatus:
…\ndepends-on: []\n---\n`). Nothing in the build binds writer to reader; the fixture is the only
thing that proves `ParseFrontmatter` reads what `new step` writes.

## Implementation Plan

- [x] Step 1: `internal/platform/config/config_test.go` `Test_the_default_profile_names_the_acceptance_heading` — pins `"## Scenario"` (red)
- [x] Step 2: `internal/platform/config/config.go` — add `AcceptanceHeading string \`yaml:"acceptance-heading"\`` + its `Default()` value; extend the `Config` doc comment (green)
- [x] Step 3: `internal/platform/markdown/section_test.go` — `Section`: body under the heading; ends at the next same-or-higher heading; includes deeper subheadings; **a `#` line inside a fenced block is not a heading**; unknown heading → `("", false)`; leading/trailing blank lines trimmed (red)
- [x] Step 4: `internal/platform/markdown/doc.go` + `section.go` `Section` (green)
- [x] Step 5: `internal/platform/markdown/section_test.go` `Title` cases — first `# ` heading; fence-aware; none → `("", false)` (red)
- [x] Step 6: `internal/platform/markdown/section.go` `Title` (green)
- [x] Step 7: `internal/platform/stepfile/frontmatter_test.go` — parses `id`/`status`/`depends-on`; returns the remaining body; no leading `---` → `ErrNoFrontmatter`; malformed YAML → error; `Done()` true for `done`/` Done ` and false for `open`/`blocked`/`""` (red)
- [x] Step 8: `internal/platform/stepfile/frontmatter.go` `Frontmatter`, `ParseFrontmatter`, `ErrNoFrontmatter`, `Done`; widen `stepfile`'s `doc.go` from naming to "naming + machine fields" (green)
- [x] Step 9: `internal/assemble/assemble_test.go` — `fixtureConfig()`, `newFixture(t)` per **## Fixture**, plus `Test_returns_the_lowest_numbered_open_step_s_id_and_title` (red)
- [x] Step 10: `internal/assemble/doc.go`, `brief.go` (`Brief`, `Step`, `Section`), `errors.go` (`ErrNoSuchFeature`, `ErrMalformedFeature`), `assemble.go` (`Server`, `NewServer`, `Start` via `os.Root`) (green). Two conventions the developer must not guess: **(a)** `Start` runs `stepfile.ParseFrontmatter` first and feeds `markdown.Title`/`markdown.Section` the **returned rest**, never the whole file, so a `#` inside a YAML value can never be read as a heading; **(b)** `wrapcheck` is on — wrap each foreign error **once** as `fmt.Errorf("assemble: %w", err)` at the boundary, mirroring `scaffold`, and return the package's own sentinels bare
- [x] Step 11: `assemble_test.go` `Test_returns_the_next_step_s_acceptance_criteria_and_checklist` — `ACCEPTANCE-03` and both `CHECKLIST-03-*` markers present, fenced `#` comment line included (green on arrival — Step 10's extraction already covers it)
- [x] Step 12: `assemble_test.go` `Test_carries_every_state_file_section_as_inherited_context` — all four fixture headings and all five `STATE-*` markers (green on arrival)
- [x] Step 13: `assemble_test.go` `Test_reports_two_done_and_three_open` — `Done == 2`, `Open == 3` (green on arrival)
- [x] Step 14: `assemble_test.go` `Test_reads_inherited_context_from_the_state_file_not_from_the_step_handoffs` — no `HANDOFF-ONLY-*` marker anywhere in the brief (green on arrival)
- [x] Step 15: `assemble_test.go` `Test_carries_no_other_step_s_acceptance_criteria` — **the absence claim**: `ACCEPTANCE-01/02/04/05` absent from the whole rendered brief while `ACCEPTANCE-03` is present (green on arrival; mutation-verified in Step 28)
- [x] Step 16: `assemble_test.go` `Test_every_step_file_in_the_fixture_carries_acceptance_criteria_the_same_probe_reads` — **the control arm**: calls `markdown.Section` on each of the five fixture step files with `cfg.AcceptanceHeading` and asserts each returns `found == true` and its own `ACCEPTANCE-0N` marker, and likewise `HANDOFF-ONLY-01/02` under `cfg.HandoffHeading`. Proves Steps 14 and 15 are non-vacuous (green on arrival)
- [x] Step 17: `assemble_test.go` `Test_takes_the_lowest_numbered_open_step_not_the_first_in_directory_order` — a second fixture on `StepFilePattern "STEP-%d.md"` with steps 2 (done), 9 (open), 10 (open); next must be `STEP-9`, which lexicographic order gets wrong (green on arrival)
- [x] Step 18: `assemble_test.go` `Test_returns_no_next_step_when_every_step_is_done` — `Step == nil`, counts still filled. Nil-safety only; SCENARIO-12 owns the stderr line, exit 0 and `--json` null (green on arrival)
- [x] Step 19: `assemble_test.go` — error matrix: `Test_returns_an_error_when_the_feature_does_not_exist` (`ErrNoSuchFeature`), `Test_returns_an_error_when_the_state_file_is_missing` (`ErrMalformedFeature`, no partial brief), `Test_returns_an_error_when_a_step_file_has_no_frontmatter`, `Test_returns_an_error_when_the_feature_name_escapes_the_feature_root` (mirror `scaffold`'s `../escaped` test). No copy pinned (green on arrival)
- [x] Step 20: `internal/assemble/render_test.go` `Test_RenderText_...` — the Step 7 layout; state sections in config order; `Test_RenderText_skips_a_state_section_with_an_empty_body`; `Test_RenderText_writes_nothing_when_there_is_no_next_step` (red). Refactored `Step.Acceptance`/`Checklist` from bare strings to `Section{Heading, Body}` so `RenderText(w, b Brief)` (no `cfg` parameter, per Step 21's pinned signature) has the configured heading text to render verbatim without threading config through it; existing Step 11/14/15 assertions updated to `.Body`, still green throughout
- [x] Step 21: `internal/assemble/render.go` `RenderText(w io.Writer, b Brief) error` (green)
- [x] Step 22: `internal/assemble/assemble_test.go` `Test_writes_nothing_to_disk` — sha256+mode+mtime sweep of every path under the `t.TempDir()` **root** (not just the feature dir), before and after `Start`; assert the maps are `Equal` (green on arrival — `Start` never opens a file for writing)
- [x] Step 23: `assemble_test.go` `Test_the_disk_sweep_sees_a_write` — **control arm for Step 22**: snapshot, `os.WriteFile` a file into the tree, snapshot again, assert the two differ. Confirms the sweep is non-vacuous
- [x] Step 24: `internal/cli/start_test.go` — command slice through `cli.Run` against a default-profile fixture in `t.TempDir()`: `Test_prints_the_brief_and_writes_nothing_to_stderr`; usage-error variants for no feature / too many args / unknown flag; `Test_prints_the_start_usage_for_help`; `Test_returns_an_error_for_an_unknown_feature_on_start` (exit 1, stdout empty) (red)
- [x] Step 25: `internal/cli/start.go` — `startUsage` ("brief start reads; it never writes." — six words on read-only, per *Decisions taken* 2), `runStart`: parse flags, `config.Resolve(wd)`, `root = filepath.Dir(source)` or `wd`, `assemble.NewServer(cfg, root).Start`, `assemble.RenderText(stdout, brief)`, errors through `renderRefusal(stderr, "start", err)` (green)
- [x] Step 26: `internal/cli/cli.go` — dispatch `case "start"`; updated the `usage` const with the `brief start <feature>` line; updated **both** `expected one of: new` strings to `new, start` (update)
- [x] Step 27: `internal/cli/run_test.go:58`, `:69` and `internal/cli/exit_test.go:15` — updated the three pinned copies of `expected one of: new` to `new, start` (update)
- [x] Step 28: **mutation verification**, one at a time, each backed up to `$TMPDIR` and diffed byte-identical after restore (`git stash` was not needed since each mutation was a targeted `Edit`/restore-from-copy pair): (a) made `Start` append *every* step's acceptance section instead of only the selected step's — `Test_returns_the_lowest_numbered_open_step_s_id_and_title` (Step 11) and `Test_reports_two_done_and_three_open` (Step 13) stayed green; `Test_carries_no_other_step_s_acceptance_criteria`'s four negatives (Step 15) went **red**, exactly as predicted. (b) deleted the fence-tracking block from `markdown.Section`'s end-of-section loop — `Test_returns_the_next_step_s_acceptance_criteria_and_checklist` (Step 11) went **red**, truncated at the fenced `#` comment line. Discovered en route: this mutation only reddens because `headingLevelOf` tolerates leading whitespace before `#` — a prerequisite fix, itself TDD'd (`Test_Section_ends_at_a_heading_line_with_leading_whitespace`, red then green). A post-hoc review then found the first version of that fix unbounded the tolerance to arbitrary indentation, which would silently truncate a checklist at any deeply-indented `#` continuation line (a real R10-class content-loss risk, not hypothetical — nothing in the Step 9 fixture exercised it since every checklist item is single-line). Bounded to CommonMark's 0–3 leading spaces instead, with its own red→green test (`Test_Section_does_not_treat_a_hash_line_indented_four_or_more_spaces_as_a_heading`); re-ran mutation (b) after the bound and it still reddens Step 11, since the fixture's Gherkin comment is 2-space-indented, inside the bound. (c) inverted `Frontmatter.Done`'s equality — `Test_reports_two_done_and_three_open` (Step 13) went **red** with `3 done, 2 open`. Every restore verified byte-identical by `diff`
- [x] Step 29: `go build ./...` (exit 0), `go test ./...` unpiped from the repo root (exit 0, 126 tests total vs. 76 before this scenario — +50, after the post-review indentation-bound test also landed), `go test -race` on `./internal/assemble/... ./internal/platform/markdown/... ./internal/platform/stepfile/... ./internal/cli/...` (all ok), `golangci-lint run ./...` (0 issues, after fixing one gosec G122 in a test-only `filepath.WalkDir` helper by switching it to `os.Root`-scoped reads, two `modernize` hits, and three `perfsprint` string-concat-in-loop hits); also tightened two error-matrix assertions (Step 19) from bare `require.Error` to `require.ErrorIs` against `stepfile.ErrNoFrontmatter` and `assemble.ErrNoSuchFeature` respectively, per review; `go doc ./internal/assemble` and `go doc ./internal/platform/markdown` read as contracts
- [x] Step 30: mark SCENARIO-04 done in `specification.md`'s `## BDD Acceptance Progress`, and rewrite `docs/specifications/brief/STATE.md` folding in the Handoff below
