# brief — current state

Scenarios complete: SCENARIO-01..21. Last updated by SCENARIO-21.

## Binding decisions

- Config: `.brief.yaml`, upward walk, nearest wins, missing anywhere = `config.Default()`.
  `Resolve(startDir) (Config, string, error)`, `errors.Is`-comparable `ErrInvalidConfig`. (01-04)
- `stepfile.Frontmatter.Done()` is the sole doneness authority everywhere. `SetStatus` edits
  `status:` textually, CRLF-preserving; `ErrNoStatusField` when absent. (03-05, 09)
- Handoff lives in its own file, `stepPattern.ID(n) + cfg.HandoffFileSuffix`, never spliced
  into the step file. `markdown.Section` is the one exported section reader, fence-aware both
  ends; no exported byte-offset API. (04-06, HANDOFF-FILE)
- `finish`'s four writes (handoff → state → step → specification) converge a crash-retry from
  any point. **One predicate, `(scaffold.refinish).verdict()`, decides every re-finish of a
  done step**: no-op (R11), or `ErrAlreadyFinished` naming the divergent file (handoff checked
  first). A done step whose handoff file is missing/unreadable is exempt, writing as normal.
  (HANDOFF-FILE, 06, 16)
- `RefusalError.Line` (0 = whole-path), rendered `<path>:<line>` by `cli/refusal.go`.
  `cli.Run` takes `stdin` before `stdout`. (fix ×3, HANDOFF-FILE, 05-06)
- `NewFeature` has two refusal classes: whitespace/empty name → `ErrInvalidFeatureName`
  (exit 2, no tail); existing directory → `*RefusalError`/`ErrFeatureExists` (exit 1, tail). (07, 08)
- `status` lives on `(*assemble.Server).Status`, rendered by `assemble.RenderStatusText`;
  `assemble` owns reading, `scaffold` owns writing. Zero features → `nil`, not `[]`. (09, 10)
- A malformed feature degrades into a row, never dropped: `FeatureStatus.Problem`, first
  failure wins, exit 0 always. `check` (22) owns the failing role (R18). (11)
- `brief.Step == nil` has two causes; `cli.runStart` branches on `Done+Open` to say which,
  absolute feature-directory path, exit 0. (12)
- `Start` refuses via `*assemble.RefusalError`: specification → state file → step files →
  briefed step, first failure wins. `cli/refusal.go` drops `(no files changed)` for this type. (13)
- An absent acceptance/state heading in `start` degrades to `Brief.Shortfalls`, never refuses;
  fires on `Section.Found == false`, not body emptiness. (14)
- `--json` is on `start` only; `RenderJSON` always writes, `RenderText` writes nothing when
  `Step` is nil. A refusal under `--json` is still a plain stderr line, no envelope. (15)
- **`Finish` validates in one fixed band**, ahead of `(refinish).verdict()`: handoff cap →
  state cap → state fence → state headings → open checklist item → **unmet dependency** →
  spec read → `verdict()`. A done step over any of these reports the content defect, never
  `ErrAlreadyFinished` — **except** the dependency check, which `FirstUnmet` itself exempts a
  done step from (09's own clause), keeping R11's no-op reachable when a dependency is
  reopened by hand after the step is done. (17, 18, 19, 20, 21)
- `markdown.FirstUnchecked(body, heading)` is the sole checklist-item scanner, distinct from
  `render.go`'s own `checklistItemRe` (deliberately unwidened — `tickProgressEntry` would
  misreport a tick on `[X]`). (20)
- **`stepfile.DependencyIndex` (`Record`/`FirstUnmet`/`Known`) is the single definition of
  "blocked"**: `assemble.featureStatus`'s blocked count and `scaffold.Finish`'s refusal both
  call it — `scaffold` can't import `assemble`, so the rule lives in `platform`. Keyed by
  `pattern.ID(n)`, never `fm.ID`. Direct dependencies only; R4's transitive closure unbuilt. (21)

## Left unbuilt

- R4: `DependencyIndex` transitive/cycle traversal, `assemble.Server.Next` ordering by
  dependency — no owner, later phase. (21)
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`; `status --json`; `--json` on
  `next`/`show`/`state get`/`handoff` — deferred (15). `--force`/`--if-state-matches` and any
  diff/finding output on a re-finish divergence (R9) — deferred (16), no owner.
- `markdown.Headings`, `markdown.ChecklistItems` (all-items listing) — owner: `check` (22).
- `HandoffPattern.Number` — owner: `check` (22) and the R6 synthesis.
- `NewStep`/`assemble`'s read path do not re-validate an on-disk feature name (07).
- `status` does not adopt 13's checks — owner: `check` (22), reusing `checkSpecification`.
- `cfg.OptionalConventions` unconsumed. No JSON error/refusal envelope, `"feature"` key, or
  schema-version key. (14, 15)
- `ErrMissingStateHeading` has no `check` (22) counterpart for pre-19 state files yet.
- `check` (22) must report, unowned until then: a leftover `## Handoff` section, an id/filename
  mismatch, a pre-19 state body missing a heading, an open checklist item, a self-dependency,
  an unknown `depends-on` id, a dependency cycle, and a malformed depends-on sibling reported
  as "is not finished" rather than named directly (20, 21).
- `FeatureStatus.Blocked` is still a count with no ids attached, no `--json` on `status`.

## Traps

- `scaffold` is named for `new feature`/`new step` while also owning `finish` — unowned rename
  debt. `-HANDOFF.md` sorts *before* its step file (`-` < `.`).
- `atomicfile.Create`'s temp-sibling mode can wedge a later write until the sibling is removed.
- A whitespace-carrying feature name is a legal directory name everywhere; `validateFeatureName`
  is the only guard. `NewFeature`'s exists-refusal is guarded twice (`Mkdir` then `O_EXCL`) —
  mutation-verify individually. (07, 08)
- `assemble.RefusalError` duplicates `scaffold.RefusalError`, knowingly — `assemble` must not
  import `scaffold`. (11, 13)
- `--json` on a completed `start` is never byte-empty stdout — test `"step":null`, not byte
  count. (12, 15)
- **A plan's fixture-update list can under-count — enumerate by running the suite.** 19's plan
  named six, the suite found nine; 21's plan named zero and the suite confirmed zero (STEP-01
  already ships done in the shared fixture). (13, 14, 19, 21)
- **This pipeline's own fix-mode re-runs of `finish` on a done step hit 16's refusal** whenever
  the regenerated body differs from what is recorded — read the recorded file and edit it
  directly, or re-run with the recorded body. (16)
- **A byte-snapshot alone doesn't prove a refusal happened before any write** — pair it with
  the `pinModTimes`/`modTimes` probe; assert `Equal` on the whole line, never `Contains`. (16)
- `cli/finish.go`'s placeholder swap only upgrades a `RefusalError.Path` it recognizes
  (`StateSource`/`HandoffSource`) — any future placeholder needs its own branch. (17, 18)
- **A "reports X first" test is vacuous without a call-site MOVE mutation** — deleting either
  check doesn't prove order; true for the cap band, 19's headings, and 21's dependency-vs-
  checklist ordering alike. (17, 18, 19, 21)
- `markdown.Section(body, "")` returns `found == true` on the first blank line — an empty
  configured heading would pass 19's check silently. Not fixed. (19)
- **`!idx.recorded[dep]` is true for both an absent key and a recorded `false`** — intended,
  but a typo'd id and an open dependency are indistinguishable inside `FirstUnmet` alone; the
  copy branch depends on `Known`, so only `Equal` on the whole refusal line catches a broken
  branch. A count-only assertion (e.g. `Blocked == 1`) can pass under a broken `FirstUnmet`
  even when the wrong step is being counted — proven by mutation during this scenario. (21)
- An unparseable depended-on step file must be **Recorded**, not skipped, or `Known` goes
  false and the refusal wrongly claims a file sitting right there "names no step file". (21)
- This repository's own `STATE.md` and several handoff files still exceed `brief`'s own caps —
  harmless since the tool is not self-hosted; `check` (22) gets the reporting role (R18).

## Open debts

- Heading/cap **value** validation (empty/duplicate heading, non-positive cap) — unowned. (19)
- A mid-write I/O failure leaves a half-applied result a same-argument retry repairs; `check`
  (22) is the not-yet-built proactive detector. Invalid `step-file-pattern` refusal names the
  feature dir, not the config — unowned.
- `insertProgressEntry`/`tickProgressEntry` insert LF into a CRLF file; `SetStatus` returns
  `ErrNoStatusField` for a missing delimiter too. All unowned MINOR.
- **Data loss:** a symlinked specification or step file is silently replaced by `finish`'s
  rename. Pre-existing, real, unowned — needs its own refusal scenario — dies unless re-opened.
- Whether `status` should *follow* a symlinked feature directory — undecided, unowned. (11)
- The `config.InvalidConfigError` refusal still carries the `(no files changed)` tail on a
  read command (`start`), contradicting R14a. Pre-existing, unowned.
- **R13's output budget/truncation is unowned, and must never truncate a `--json` payload.**
  `cfg.DefaultOutputBudgetBytes` (8192) remains unconsumed. (15)

## Crossover note

An identical re-finish being a true no-op means the crossover's "mark SCENARIO-01 through 06
done" step is safely re-runnable only with the same handoff/state bytes, and only when that
state body carries all four configured headings. `SCENARIO-01.md`…`-12.md` and their
`-HANDOFF.md` files still carry no frontmatter — the crossover owns adding it, including
`id:` and `status:`. Until then `brief start brief` still fails with `no frontmatter found`,
and `status` still prints `brief ! ! !` for it.
