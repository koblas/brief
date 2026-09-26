# feature-lifecycle — current state

Scenarios complete: SCENARIO-01..04. Last updated by SCENARIO-04.

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
  slot, after argument checks, before `checkStepDependencies`. Deliberately asymmetric with
  `checkStepChecklist`'s own `ErrOpenChecklistItem`, which still fires on a done-step
  re-finish. `checkStepPlanned` must never grow to cover the acceptance section (Rule 5:
  it stays optional for finish). (SCENARIO-03)
- CLI copy (Rule 7) is ruled verbatim in the spec's `## Surface & Copy` and now shipped:
  `brief new feature`'s stderr tail is "; write its specification, then add each step with
  'brief new step <name>'"; root `--help`'s `Long` is `rootShort + "\n\n" +
  rootLifecycleParagraph` (new const, `internal/cli/cli.go`) — `rootShort` itself and its
  doc comment are untouched, and `helpIndex` never emits an entry for root itself
  (`internal/cli/help_json.go`), so this paragraph never reaches `--json`; a later change to
  `helpIndex` that starts walking root must re-check this. `startLong`'s shortfall sentence
  and `finishLong`'s appended refusal sentence (continuing its first paragraph, no blank
  line before it) are both in `internal/cli/{start,finish}.go`, hand-wrapped to the file's
  existing ~74-column prose width. (SCENARIO-04)
- `docs/specifications/brief/specification.md` now says (R11, the `new step` command-table
  row, and the SCENARIO-03-analogue scenario ~line 393) that finish refuses an absent-or-
  empty checklist on an open step, and that `new step`'s scaffold carries an empty
  acceptance section as well as an empty checklist. (SCENARIO-04)

## Left unbuilt

- `.claude/skills/brief-workflow/SKILL.md`'s Lifecycle section — SCENARIO-05.
- The CLAUDE.md snippet's "Multi-step work gets a feature" sentence
  (`internal/platform/artifact/snippet.go`) — SCENARIO-06.

## Traps

- `assemble.Check` (and `--hook`) must not call `ChecklistItemCount`, and
  `conform.OpenChecklistItem` must keep treating an absent heading or zero items as "never a
  violation" (Rule 6). Mutation-verified: making `OpenChecklistItem` flag absent/zero
  reddens only `check`'s done rows. (SCENARIO-02, SCENARIO-03)
- `stepfile.ParseFrontmatter`'s second return is the frontmatter-stripped body: feeding it
  to `markdown.HeadingLine` instead of the whole file gives a wrong-but-plausible-looking
  line. Mutation-verified against `checkStepPlanned`. (SCENARIO-03)
- `fixtureConfig()` in `internal/scaffold/scaffold_test.go` is shared by ~12 test files; any
  scaffold test that `NewStep`s and then `Finish`es needs a ticked checklist item, or it
  hits the refusal. (SCENARIO-01, SCENARIO-03)
- `cmd/brief/main.go`'s package doc comment and
  `docs/specifications/human-output/SCENARIO-10.md`'s quoted old `new feature` success line
  both echo pre-SCENARIO-04 wording but are out of this feature's scope — left untouched.
  (SCENARIO-04)

## Open debts

(none)
