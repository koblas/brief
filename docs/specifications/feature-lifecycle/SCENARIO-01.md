---
id: SCENARIO-01
status: done
---

# SCENARIO-01: new step scaffolds the acceptance heading

## Scenario

```gherkin
Scenario: SCENARIO-01 new step scaffolds the acceptance heading
  Given a feature
  When I run "brief new step <feature>"
  Then the step file carries the configured acceptance heading, a blank line, then the configured checklist heading
  And "brief start" reports no missing-acceptance-heading shortfall for it
  And the specification and state scaffolds are byte-unchanged
```

## Notes for the implementer

`stepSkeleton` (`internal/scaffold/render.go`) is the sole producer of a step file's initial
bytes, called only from `NewStepFS`. Its only reader is `internal/assemble/assemble.go`,
which already keys `markdown.Section` off `cfg.AcceptanceHeading` / `cfg.ChecklistHeading` —
`!step.Acceptance.Found` (the shortfall this scenario stops firing on a fresh step) and
`!step.Checklist.Found` (the refusal, unaffected) need no change. Only `stepSkeleton` changes;
`specificationSkeleton` and `stateSkeleton` are separate functions in the same file, untouched,
which is what keeps Rule 1's "byte-unchanged" true — their existing golden tests
(`Test_writes_the_specification_skeleton_with_the_configured_progress_heading_and_nothing_under_it`,
`Test_writes_the_state_file_with_the_four_configured_headings_and_nothing_under_them`, both in
`internal/scaffold/scaffold_test.go`) need no edit.

**Confirmed by probe, not by grep alone**: `stepSkeleton`'s change (emit
`cfg.AcceptanceHeading` + blank line before `cfg.ChecklistHeading`) plus the `fixtureConfig`
override below were applied to a `git archive a3146fe` export and `go test ./...` run there,
unpiped. Exactly two tests broke, repo-wide (`cmd/`, `internal/cli`, `internal/scaffold`, and
every other package all otherwise green):
- `internal/scaffold/step_test.go` `Test_writes_the_step_file_with_frontmatter_a_title_and_an_empty_checklist`
- `internal/cli/start_internal_test.go` `Test_start_on_a_freshly_scaffolded_feature_names_only_the_absent_acceptance_heading_mem`
  — an **existing** command-level test that already chains `new feature` → `new step` →
  `start`, and today pins exactly the gap this scenario closes (it currently asserts `start`'s
  one stderr line contains `"## Scenario"`, i.e. the missing-heading shortfall). No other test,
  fixture, or `testdata` file pins the old skeleton.

No doc comment or help string beyond `stepSkeleton`'s own describes the scaffold's contents:
`go doc ./internal/scaffold Server.NewStep`, `Server.NewStepFS`, and `internal/cli/new.go`'s
`newStepLong` were checked and none mention acceptance/checklist headings or the file's shape.

## Implementation Plan

### Red

- [x] Step 1: `internal/scaffold/scaffold_test.go` `fixtureConfig` — add
      `cfg.AcceptanceHeading = "## Fixture Scenario"` (every other field this shared fixture
      sets is already non-default; this one is not), so the golden test below exercises a
      configured, non-default acceptance heading alongside the already-non-default checklist
      heading.
- [x] Step 2: `internal/scaffold/step_test.go`
      `Test_writes_the_step_file_with_frontmatter_a_title_and_an_empty_checklist` — rename to
      name the acceptance heading it now pins (e.g.
      `Test_writes_the_step_file_with_frontmatter_a_title_an_acceptance_heading_and_an_empty_checklist`)
      and update `want` to include `cfg.AcceptanceHeading`, a blank line, then
      `cfg.ChecklistHeading`; fails: today's output omits the acceptance heading entirely.
- [x] Step 3: `internal/cli/start_internal_test.go`
      `Test_start_on_a_freshly_scaffolded_feature_names_only_the_absent_acceptance_heading_mem`
      — rewrite to the new contract and rename (e.g.
      `Test_start_on_a_freshly_scaffolded_step_reports_no_missing_acceptance_heading_shortfall_mem`):
      keep the same `new feature` → `new step` → `start` chain against one shared `*rwfs.Mem`;
      before calling `start`, `assert.Equal` the mem-held `SCENARIO-01.md` against this spec's
      "Surface & Copy" step-scaffold block verbatim (default config, so the ruled bytes are
      pinned once under this scenario, not left to inference); then assert `start` exits 0,
      prints a non-empty brief to stdout, and its stderr does **not** contain the
      missing-heading substring (`no "## Scenario" heading found`) — assert absence of that
      specific substring, not that stderr is empty overall, since SCENARIO-02 will make a
      freshly-scaffolded step print a different ("is empty") shortfall once it lands and an
      empty-stderr assertion would then fail for the wrong reason; fails today because the
      substring is present (this is the test currently pinning the gap).
- [x] Step 4: `internal/cli/start_internal_test.go`, new
      `Test_start_on_a_scaffolded_step_with_its_acceptance_heading_removed_names_the_missing_heading_mem`
      — control arm for Step 3, same chain against a second `*rwfs.Mem`, differing in exactly
      one variable: after `new step`, read the generated step file back, `require` it contains
      `cfg.AcceptanceHeading` (proving there is something to strip), rewrite it with that
      heading line and its following blank line removed, `require` the rewritten body differs
      from the original, then call `start` and assert its stderr **does** contain
      `no "## Scenario" heading found`; fails today because, before Step 5 lands, the scaffold
      never had the heading to strip in the first place, so the `require` that it was present
      fails.

### Green

- [x] Step 5: `internal/scaffold/render.go` `stepSkeleton` — emit, after the title heading and
      a blank line: `cfg.AcceptanceHeading`, a blank line, `cfg.ChecklistHeading`, file ending
      in a single trailing newline.

### Sweep

- [x] Step 6: fix what `go build ./... && golangci-lint run ./...` reports (sweep)
- [x] Step 7: `internal/scaffold/render.go` `stepSkeleton` doc comment — update the sentence
      describing the rendered file to name the acceptance heading it now includes; the comment
      is already near its 1–2 line budget for an unexported func, so the fix must stay the same
      length or shorter, never longer (sweep)

### Verify

- [x] Step 8: full verification per `.claude/rules/agent-briefs.md`; mutate `stepSkeleton` to
      drop the `cfg.AcceptanceHeading` emission (revert to the old format string) →
      Step 2's renamed test and Step 3's renamed test both go red; restore and diff to confirm
      byte-identical.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `stepSkeleton` now emits `cfg.AcceptanceHeading`, a blank line, then `cfg.ChecklistHeading` —
  both already-configurable fields. SCENARIO-02's empty-acceptance and zero-item-checklist
  shortfalls both assume the heading exists on a fresh step; do not reintroduce an
  acceptance-less skeleton.
- `specificationSkeleton` and `stateSkeleton` are untouched by this scenario. SCENARIO-04's
  amendment to `docs/specifications/brief/specification.md`'s "new step" command-table row and
  its approved scenario (~line 393) must describe the scaffold this scenario now produces
  (this spec's "Surface & Copy" block), which Step 3 pins byte-for-byte under the default
  config.

**Left unbuilt** — named so nobody assumes it exists:
- The "acceptance section present but whitespace-only" shortfall and the "checklist present,
  0 items" shortfall (Rule 2), in `internal/assemble` — owned by SCENARIO-02. After this
  scenario, a freshly scaffolded step's acceptance section is heading-only (empty body);
  `assemble.go`'s `!step.Acceptance.Found` check only tests heading presence, so no shortfall
  fires for it yet. SCENARIO-02 makes it fire.
- `brief finish`'s new checklist-heading/zero-items refusal (Rule 3) and the shared
  item-counting function in `conform` (Rule 4) — SCENARIO-03.
- Help text, the root `--help` paragraph, and the `docs/specifications/brief/specification.md`
  R11/new-step amendments (Rule 7) — SCENARIO-04.
- Skill and CLAUDE.md snippet changes (Rule 8) — SCENARIO-05/06.

**Traps**:
- `fixtureConfig()` in `internal/scaffold/scaffold_test.go` is shared by ~12 test files in the
  package; a `git archive a3146fe` probe with the Step 1 override applied confirmed only
  `step_test.go`'s golden-byte assertion reads `cfg.AcceptanceHeading` today — every other
  consumer either builds its own literal step body by hand or only `assert.Contains`s. A
  future scaffold test adding its own exact-byte step assertion must account for the
  acceptance heading now being present.
- The command-level `start` test (Step 3) must not assert an empty stderr — SCENARIO-02 adds a
  real shortfall line to that same fixture's output once it lands, and an empty-stderr
  assertion would then fail for the wrong reason.
- The pre-existing `start_internal_test.go` test pinning this exact gap
  (`Test_start_on_a_freshly_scaffolded_feature_names_only_the_absent_acceptance_heading_mem`)
  was missed on a text-grep pass for `heading found` / `AcceptanceHeading`, because its own
  assertion only checks the bare substring `"## Scenario"`, not the shortfall's fuller detail
  text — neither pattern matches it. Find skeleton pins by probing (apply the change to a
  `git archive` export, run `go test ./...` there, read the failures), not by grepping test
  bodies for the new copy.
- `internal/cli/start_internal_test.go`'s chain test was renamed/repurposed, not added fresh —
  do not assume it is new when reading `git log` on that name later.
