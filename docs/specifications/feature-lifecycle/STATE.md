# feature-lifecycle — current state

Scenarios complete: SCENARIO-01..06. All six scenarios are done; fix pass 1 (post
`/run-reviewers`) landed on top. Last updated by fix pass 1.

## Binding decisions

- `stepSkeleton` (`internal/scaffold/render.go`) emits `cfg.AcceptanceHeading`, a blank
  line, then `cfg.ChecklistHeading` — both already-configurable fields. Do not reintroduce
  an acceptance-less skeleton. (SCENARIO-01)
- `markdown.CountChecklistItems` is the one item counter (Rule 4) — `conform.ChecklistItemCount`
  was a pass-through and is deleted; `assemble.StartFS` and `scaffold.checkStepPlanned` both
  call it directly, on the step file's post-frontmatter body only (never the whole file — see
  Traps). (SCENARIO-02, SCENARIO-03, fix pass 1)
- Every heading/checklist scan a step file undergoes — `assemble.StartFS`/`stepFromEntry`,
  `assemble.checkStepChecklistFinding`, `scaffold.checkStepChecklist`, `scaffold.checkStepPlanned`
  — reads only the byte suffix after `stepfile.ParseFrontmatter`'s frontmatter (`rest`/`e.rest`/
  `ps.rest`), never the whole file. A reported `Line` is that rest-relative line plus
  `frontmatterLines := strings.Count(string(whole[:len(whole)-len(rest)]), "\n")`, which maps it
  back to the whole file's own numbering. This is the one fix for both a frontmatter YAML
  comment byte-identical to a heading, and a frontmatter block-scalar line that looks like an
  opened, never-closed fence to a scanner naive of YAML syntax. (fix pass 1)
- `assemble.StartFS`'s `Brief.Shortfalls` order is fixed: acceptance, then checklist, then
  state headings; `--json` `shortfalls[]` uses the same order. (SCENARIO-02)
- `scaffold.ErrUnplannedStep` is the one sentinel for finish's two new refusals
  (`internal/scaffold/errors.go`), fired only for an open step in `checkStepChecklist`'s
  slot, after argument checks, before `checkStepDependencies` — order is mutation-verified
  (`Test_finish_reports_an_unplanned_checklist_before_an_unfinished_dependency`).
  `checkStepPlanned` must never grow to cover the acceptance section (Rule 5: it stays
  optional for finish). (SCENARIO-03, fix pass 1)
- CLI copy (Rule 7) is ruled verbatim in the spec's `## Surface & Copy` and shipped:
  `brief new feature`'s stderr tail, root `--help`'s appended lifecycle paragraph
  (`rootLifecycleParagraph`, `internal/cli/cli.go`), and `startLong`/`finishLong`'s amended
  sentences (`internal/cli/{start,finish}.go`). (SCENARIO-04)
- `docs/specifications/brief/specification.md` (R11, `new step` row, ~line 393 scenario)
  records finish's unplanned-step refusal and the scaffold's empty acceptance/checklist.
  (SCENARIO-04)
- Rule 8 is discharged: `.claude/skills/brief-workflow/SKILL.md`'s `## Lifecycle` section and
  `internal/platform/artifact/snippet.go`'s `snippetTemplateSuffix` carry the ruled lifecycle
  sentence; `olderSkillWorkflowDigests`/`olderSnippetTemplates` stay empty. (SCENARIO-05, -06)

## Left unbuilt

(none)

## Traps

- `assemble.Check` (and `--hook`) must not call `markdown.CountChecklistItems`, and
  `conform.OpenChecklistItem` must keep treating an absent heading or zero items as "never a
  violation" (Rule 6). Mutation-verified. (SCENARIO-02, SCENARIO-03)
- Never pass a step file's whole on-disk bytes (frontmatter included) to a heading/checklist
  scanner — it is naive of YAML and reads a comment or a block-scalar line as real markdown.
  Always split with `stepfile.ParseFrontmatter` first and scan the returned `rest`, adding the
  frontmatter's own line count back for any reported `Line`. Mutation-verified on both
  `scaffold.checkStepChecklist`/`checkStepPlanned` and `assemble.checkStepChecklistFinding`.
  (SCENARIO-03, fix pass 1)
- `fixtureConfig()` in `internal/scaffold/scaffold_test.go` is shared by ~12 test files; any
  scaffold test that `NewStep`s and then `Finish`es needs a ticked checklist item, or it
  hits the refusal. (SCENARIO-01, SCENARIO-03)
- `cmd/brief/main.go`'s package doc comment and
  `docs/specifications/human-output/SCENARIO-10.md`'s quoted old `new feature` success line
  echo pre-SCENARIO-04 wording but are out of this feature's scope — left untouched.
  (SCENARIO-04)
- Do not derive a shipped-digest `want` hash, or a fixed-prose test's expected bytes, from a
  failing test's actual-value output — hash/transcribe the independently-derived bytes
  instead. None of `internal/setup`/`internal/doctor`/`internal/cli`/`internal/platform/artifact`
  hardcodes the snippet's fixed prose or a line number derived from its length — all call
  `artifact.SnippetBlock`/`RecognizeSnippet` live. (SCENARIO-05, SCENARIO-06)
- `KindSnippet` stays excluded from `Render`/`Recognize`/`shipped_digest_test.go`. Do not add
  it a row there. (SCENARIO-06)

## Open debts

- StartFS compose-method extraction; `start`'s second parse of the checklist section;
  `checkStepPlanned`'s `RefusalError` literal dedup; `Server.Start` doc length;
  `ErrUnplannedStep` sentinel doc length (file-wide idiom in `internal/scaffold/errors.go`);
  `rootLifecycleParagraph` doc length — all raised in fix pass 1 review, judged MINOR/cosmetic,
  deferred — unowned, dies unless a later scenario reopens that surface.
- The frontmatter line offset (`strings.Count(body[:len(body)-len(rest)], "\n")`) is computed
  twice, in `scaffold.FinishFS` and `assemble` check's `parsedStep`, and leans on
  `stepfile.ParseFrontmatter` returning an unmodified content suffix. `stepfile` should own it
  (return the offset beside `rest`). Raised by arch, correctness and refactor re-gate as MINOR;
  deferred — unowned.
