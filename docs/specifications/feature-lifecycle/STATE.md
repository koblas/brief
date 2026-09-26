# feature-lifecycle — current state

Scenarios complete: SCENARIO-01..06. All six scenarios in this feature's
`## BDD Acceptance Progress` are done. Last updated by SCENARIO-06.

## Binding decisions

- `stepSkeleton` (`internal/scaffold/render.go`) emits `cfg.AcceptanceHeading`, a blank
  line, then `cfg.ChecklistHeading` — both already-configurable fields. Do not reintroduce
  an acceptance-less skeleton. (SCENARIO-01)
- `conform.ChecklistItemCount(body []byte, heading string) (int, bool)` is the only item
  counter (Rule 4): `markdown.CountChecklistItems` and `FirstUnchecked` share one unexported
  scanner. `assemble.StartFS`'s zero-item shortfall and `scaffold.checkStepPlanned`'s
  zero-item/absent-heading refusal both call it on the *whole* step file (not the
  frontmatter-stripped body), taking the heading's line from `markdown.HeadingLine` against
  that same whole body. A second counter, or a direct call to `markdown.CountChecklistItems`
  from `assemble` or `scaffold`, breaks the one decision point. (SCENARIO-02, SCENARIO-03)
- `assemble.StartFS`'s `Brief.Shortfalls` order is fixed: acceptance, then checklist, then
  state headings; `--json` `shortfalls[]` uses the same order. (SCENARIO-02)
- `scaffold.ErrUnplannedStep` is the one sentinel for finish's two new refusals
  (`internal/scaffold/errors.go`), fired only for an open step in `checkStepChecklist`'s
  slot, after argument checks, before `checkStepDependencies`. `checkStepPlanned` must never
  grow to cover the acceptance section (Rule 5: it stays optional for finish). (SCENARIO-03)
- CLI copy (Rule 7) is ruled verbatim in the spec's `## Surface & Copy` and now shipped:
  `brief new feature`'s stderr tail, root `--help`'s appended lifecycle paragraph
  (`rootLifecycleParagraph`, `internal/cli/cli.go`, never reaching `helpIndex`/`--json`
  since root itself has no entry there), and `startLong`/`finishLong`'s amended sentences
  (`internal/cli/{start,finish}.go`). (SCENARIO-04)
- `docs/specifications/brief/specification.md` (R11, `new step` row, ~line 393 scenario)
  now records finish's unplanned-step refusal and the scaffold's empty acceptance/checklist.
  (SCENARIO-04)
- Rule 8 (shipped skill + CLAUDE.md snippet change bytes in place, no older-template entry)
  is now fully discharged: `.claude/skills/brief-workflow/SKILL.md` gained its `## Lifecycle`
  section (SCENARIO-05, `olderSkillWorkflowDigests` stays `[][32]byte{}`), and
  `internal/platform/artifact/snippet.go`'s `snippetTemplateSuffix` now carries the ruled
  "Multi-step work gets a feature" sentence, inserted before "To work on a step, run";
  `snippetTemplatePrefix`/the `{dir}` split point are unchanged, and
  `olderSnippetTemplates` stays `[]func(dir string) []byte{}` (SCENARIO-06). Neither
  `SkillWorkflow()`/`digest.go` nor `SnippetBlock`/`RecognizeSnippet`/`KindSnippet`'s
  render-recognize exclusion changed shape — only fixed prose moved.

## Left unbuilt

(none — this was the feature's last scenario)

## Traps

- `assemble.Check` (and `--hook`) must not call `ChecklistItemCount`, and
  `conform.OpenChecklistItem` must keep treating an absent heading or zero items as "never a
  violation" (Rule 6). Mutation-verified. (SCENARIO-02, SCENARIO-03)
- `stepfile.ParseFrontmatter`'s second return is the frontmatter-stripped body: feeding it
  to `markdown.HeadingLine` instead of the whole file gives a wrong-but-plausible-looking
  line. Mutation-verified against `checkStepPlanned`. (SCENARIO-03)
- `fixtureConfig()` in `internal/scaffold/scaffold_test.go` is shared by ~12 test files; any
  scaffold test that `NewStep`s and then `Finish`es needs a ticked checklist item, or it
  hits the refusal. (SCENARIO-01, SCENARIO-03)
- `cmd/brief/main.go`'s package doc comment and
  `docs/specifications/human-output/SCENARIO-10.md`'s quoted old `new feature` success line
  echo pre-SCENARIO-04 wording but are out of this feature's scope — left untouched.
  (SCENARIO-04)
- Do not derive a shipped-digest `want` hash, or a fixed-prose test's expected bytes, from a
  failing test's actual-value output — hash/transcribe the independently-derived bytes
  instead. Several docs quote or embed the skill/snippet bytes as historical prose (two spec
  docs, `internal/cli/init_internal_test.go:710`, `docs/specifications/init-doctor/
  specification.md:79`) and are correctly left untouched; none of them, nor any consumer
  test across `internal/setup`/`internal/doctor`/`internal/cli`/
  `internal/platform/artifact`, hardcodes the snippet's fixed prose or a line number derived
  from its length — all call `artifact.SnippetBlock`/`RecognizeSnippet` live.
  (SCENARIO-05, SCENARIO-06)
- `KindSnippet` stays excluded from `Render`/`Recognize`/`shipped_digest_test.go` — the
  snippet's digest story is `RecognizeSnippet`'s template-replay match, not a compiled
  sha256 list. Do not add it a row there. (SCENARIO-06)

## Open debts

(none)
