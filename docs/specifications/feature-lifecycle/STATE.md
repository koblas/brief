# feature-lifecycle — current state

Scenarios complete: SCENARIO-01. Last updated by SCENARIO-01.

## Binding decisions

- `stepSkeleton` (`internal/scaffold/render.go`) emits `cfg.AcceptanceHeading`, a blank
  line, then `cfg.ChecklistHeading` — both already-configurable fields. SCENARIO-02's
  empty-acceptance and zero-item-checklist shortfalls both assume the heading exists on a
  fresh step; do not reintroduce an acceptance-less skeleton. (SCENARIO-01)
- `specificationSkeleton` and `stateSkeleton` are untouched. SCENARIO-04's amendment to
  `docs/specifications/brief/specification.md`'s "new step" command-table row and its
  approved scenario (~line 393) must describe the scaffold this scenario now produces —
  see this feature's `specification.md` "Surface & Copy" step-scaffold block, pinned
  byte-for-byte under default config by `internal/cli/start_internal_test.go`. (SCENARIO-01)

## Left unbuilt

- The "acceptance section present but whitespace-only" shortfall and the "checklist
  present, 0 items" shortfall (Rule 2), in `internal/assemble` — owned by SCENARIO-02.
  After SCENARIO-01, a freshly scaffolded step's acceptance section is heading-only (empty
  body); `assemble.go`'s `!step.Acceptance.Found` check only tests heading presence, so no
  shortfall fires for it yet.
- `brief finish`'s new checklist-heading/zero-items refusal (Rule 3) and the shared
  item-counting function in `conform` (Rule 4) — SCENARIO-03.
- Help text, the root `--help` paragraph, and the `docs/specifications/brief/specification.md`
  R11/new-step amendments (Rule 7) — SCENARIO-04.
- Skill and CLAUDE.md snippet changes (Rule 8) — SCENARIO-05/06.

## Traps

- `fixtureConfig()` in `internal/scaffold/scaffold_test.go` is shared by ~12 test files in
  the package; only `step_test.go`'s golden-byte assertion reads `cfg.AcceptanceHeading`
  today. A future scaffold test adding its own exact-byte step assertion must account for
  the acceptance heading now being present.
- The command-level `start` test pinning this scaffold must not assert an empty stderr —
  SCENARIO-02 adds a real shortfall line to that same fixture's output once it lands, and
  an empty-stderr assertion would then fail for the wrong reason.
- Find skeleton pins by probing (apply the change to a `git archive` export, run
  `go test ./...` there, read the failures), not by grepping test bodies for the new copy —
  a pre-existing test can pin an old contract without ever mentioning the new copy's text.

## Open debts

(none)
