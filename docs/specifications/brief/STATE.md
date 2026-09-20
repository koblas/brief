# brief — current state

All scenarios complete: SCENARIO-01..22. Last updated by SCENARIO-22 — the final
STATE.md this pipeline writes for the `brief` feature itself.

## Binding decisions

- Config: `.brief.yaml`, upward walk, nearest wins, missing anywhere = `config.Default()`.
  `stepfile.Frontmatter.Done()` is the sole doneness authority everywhere.
- Handoff lives in its own file (`stepPattern.ID(n) + cfg.HandoffFileSuffix`), never
  spliced into the step file. `markdown.Section`/`FirstUnchecked`/`UnterminatedFence`/
  `CountLines` are the sole readers of their facts, fence-aware, shared by every caller
  (write and read alike) so no two callers can disagree about one file.
- `scaffold` owns writing (`NewFeature`, `NewStep`, `Finish`); `assemble` owns reading
  (`Start`, `Status`, `Check`); the two never import each other. Shared rules live in
  `internal/platform/*`: `stepfile.DependencyIndex` ("blocked"), `internal/platform/conform`
  (the four write-path predicates `OverCap`/`UnterminatedFence`/`MissingHeading`/
  `OpenChecklistItem`, each returning a path-less `Violation` the caller attaches `Path` to).
- `Finish` validates in one fixed band ahead of `(refinish).verdict()`: handoff cap →
  state cap → state fence → state headings → open checklist item → unmet dependency →
  spec read → verdict. A done step over any of these reports the content defect, never
  `ErrAlreadyFinished` — except the dependency check, which exempts a done step outright.
- `Check(ctx, feature)` is the R18 backstop. `feature == ""` walks every feature dir in
  `fs.ReadDir` order; a named feature that doesn't exist is bare `ErrNoSuchFeature`
  (Start's idiom). Ten rules, `C1`-`C10`, mirroring the write-path band; ordering is
  pinned byte-exact (spec → state → steps ascending → per-step band order). `check`
  narrows the **population**, never the predicate: an open step's unticked item and an
  open step's known-but-unmet dependency are ordinary in-progress work, not findings —
  every other rule (including a handoff-cap finding on an open OR done step) is unnarrowed.
  Severity is feature-wide (`ERROR` if any step is not done or unreadable, else `WARN`),
  computed once after the whole step walk and back-filled onto every collected finding —
  never set at append time. A feature with **zero step files** is vacuously "every step
  done" and takes `WARN` even with a broken specification — deliberate (there is no step
  in flight to make it `ERROR` about), but the one severity call most likely to be
  challenged; it lives only in `Check`'s doc comment before this entry. `check` walks
  step files itself, never through `readSteps`, so one step's unparseable frontmatter
  (`C6`) never blinds the rest to `C7`-`C10`.
- Findings render as `[SEVERITY] <path>:<line> — <problem>` on stdout, no `Fix`, and a
  whole-file finding's `<line>` is literally `0` (`.../STEP-01.md:0 — ...`) — the shape
  has no optional `[:line]` bracket the way R14a's refusal template does, so `0` prints
  rather than being omitted; deliberate, not an oversight. `check` exits 1 when any
  `ERROR` printed, 0 otherwise (including a `WARN`-only run) — distinct from R14a's
  `finish`-scoped "findings never appear on a failed run".
- A cap finding's `Line` is `cap + 1`, set by the caller (`conform.OverCap` itself stays
  line-less, or SCENARIO-17/18's pinned refusal bytes would move).
- `RefusalError.Line` (0 = whole-path). A malformed feature/step degrades into a finding
  or a status row, never dropped.

## Left unbuilt

Every scenario is complete — everything below is unowned, with none left to claim it.

- R4: `DependencyIndex` transitive/cycle traversal; a dependency-cycle finding in `check`;
  `assemble.Server.Next` ordering by dependency.
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`; `status --json`; `check --json`;
  `check` over a single step; `--json` on `next`/`show`/`state get`/`handoff`;
  `--force`/`--if-state-matches` and any diff/finding output on a re-finish divergence.
- `markdown.Headings`, `markdown.ChecklistItems` (all-items listing), `HandoffPattern.Number`.
- `check` never reports: an id/filename mismatch, a leftover `## Handoff` section, a done
  step with no handoff file (`finish` exempts it, R16), or a missing progress-list entry —
  the entry's only definition lives inside `scaffold.tickProgressEntry`, which finds *and
  rewrites* it in one pass; a read-only matcher would be a second definition.
- `NewStep`/`assemble`'s read path never re-validate an on-disk feature name.
- `cfg.OptionalConventions` unconsumed; no JSON error/refusal envelope, `"feature"` key,
  or schema-version key. `FeatureStatus.Blocked` is a count with no ids attached.

## Traps

- `scaffold` is named for `new feature`/`new step` while also owning `finish`.
  `-HANDOFF.md` sorts *before* its step file (`-` < `.`).
- `atomicfile.Create`'s temp-sibling mode can wedge a later write until removed.
- `assemble.RefusalError` duplicates `scaffold.RefusalError` knowingly (`assemble` must
  not import `scaffold`); `conform.Violation` is the piece that *isn't* duplicated.
- **A plan's fixture-update list can under-count — enumerate by running the suite.** True
  again in 22, but exactly as predicted this time (drift-tested: two `run_test.go` strings).
- `markdown.Section(body, "")` returns `found == true` on the first blank line — an empty
  configured heading passes silently, inherited by `check` through the shared predicate.
- `!idx.recorded[dep]` is true for both an absent key and a recorded `false` — `Known` is
  the only way to tell a typo'd id from an open dependency; `check`'s `C8`/`C9` depend on
  it exactly as `finish`'s refusal does. An unparseable depended-on step file must be
  Recorded, not skipped, or `Known` goes false and the wrong copy renders.
- This repository's own tree fails its own `check` (exit 1: `STATE.md` over cap, every
  `SCENARIO-NN.md` planning doc lacks frontmatter, several handoffs over cap) — confirmed
  by the real binary, not adjusted away. Don't "fix" the tree to pass `check` without a
  deliberate decision to, and don't pin an exact finding count here — editing this file
  changes its own line count and heading content, which `check` measures.

## Open debts

- Heading/cap **value** validation (empty/duplicate heading, non-positive cap) — unowned.
- Invalid `step-file-pattern` refusal names the feature dir, not the config — unowned.
- `insertProgressEntry`/`tickProgressEntry` insert LF into a CRLF file; `SetStatus`
  returns `ErrNoStatusField` for a missing delimiter too — unowned MINOR.
- **Data loss:** a symlinked specification or step file is silently replaced by `finish`'s
  rename — pre-existing, real, unowned, dies unless re-opened.
- Whether `status` should *follow* a symlinked feature directory — undecided, dies. A
  symlinked feature currently gets a `! ! !` row from `status` but zero findings from
  `check` (it can't `OpenRoot` a symlink, so it's silently skipped, not surfaced) — the
  same undecided question, now also a `status`/`check` divergence.
- `config.InvalidConfigError`'s refusal keeps the `(no files changed)` tail on a read
  command (`start`), contradicting R14a — pre-existing, unowned, dies.
- `cfg.DefaultOutputBudgetBytes` (R13's output budget/truncation) unconsumed; must never
  truncate a `--json` payload if ever built — unowned, dies.
- A mid-write I/O failure leaves a half-applied, same-argument-retryable result; nothing
  proactively detects that state (it self-repairs on retry) — unowned, dies.
