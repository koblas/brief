# brief — current state

Scenarios complete: SCENARIO-01..13. Last updated by SCENARIO-13.

## Binding decisions

- Config: `.brief.yaml`, upward walk, nearest wins, missing anywhere = `config.Default()`.
  `Resolve(startDir) (Config, string, error)`, `errors.Is`-comparable `ErrInvalidConfig`. (01-04)
- `stepfile.Frontmatter.Done()` is the sole doneness authority everywhere — `start`, `finish`,
  `status` never read the progress list or a markdown heading for doneness. `SetStatus` edits
  `status:` textually, CRLF-preserving; `ErrNoStatusField` when absent. (03-05, 09)
- Handoff lives in its own file, `stepPattern.ID(n) + cfg.HandoffFileSuffix`, never spliced
  into the step file; refuses a suffix digit, a non-varying `ID`, and case-folded name
  collisions (APFS is case-insensitive). (HANDOFF-FILE)
- `markdown.Section` is the one exported section reader, fence-aware both ends. No exported
  byte-offset API — every write is whole-file. (04-06, HANDOFF-FILE)
- `finish`'s four writes (handoff → state → step → specification) converge a crash-retry from
  any point; `status: done` never lands before the handoff file exists. **Identity/no-op (R11):**
  re-finishing a done step with identical inputs writes nothing. (HANDOFF-FILE, 06)
- `RefusalError.Line` (0 = whole-path), rendered `<path>:<line>` by `cli/refusal.go`. A
  leftover `## Handoff` section is ignored, never refused. `cli.Run` takes `stdin` before
  `stdout`. (fix ×3, HANDOFF-FILE, 05-06)
- **`NewFeature` has two refusal classes, never unified:** whitespace/empty name →
  `ErrInvalidFeatureName`, exit 2, no `(no files changed)` tail; existing directory →
  `*RefusalError`/`ErrFeatureExists`, exit 1, tail present. (07, 08)
- **`status` lives on `(*assemble.Server).Status`, rendered by `assemble.RenderStatusText`** —
  `assemble` owns reading, `scaffold` owns writing; `next`/`show`/`handoff`/`state get` belong
  here too. Line: `<name> <done>/<total> <next> <blocked>\n`, no header/legend, `-` when
  `Next` is empty. `Status` returns `nil`, never `[]`, for zero features (15's `--json` needs
  `null`). A missing feature root is zero features, not an error; every other open/list
  failure, or an invalid step-file pattern, still propagates. (09, 10)
- **A malformed feature degrades into a row, it is never dropped.** `FeatureStatus.Problem
  *Problem{Path, Detail, Fix}`, nil when clean. Tolerance lives in `featureStatus`, never
  `readSteps` (shared with `Start`, which stays intolerant — SCENARIO-13's checks build on
  that). One Problem per feature, first failure wins; `runStatus` writes it inline, not via
  `renderRefusal` — exit 0 always, R18 gives the failing role to `check` (22). An id/filename
  mismatch and an empty feature directory stay conforming, not malformed. (11)
- **`brief.Step == nil` has two causes; `cli.runStart`, never `assemble`, says which.**
  `assemble.Start` returns `(Brief, nil)` for both. `runStart` branches on `Done+Open`: `> 0` →
  "feature is complete"; `== 0` → "no step files yet". Both notices use the **absolute**
  feature-directory path, exit 0, no `(no files changed)` tail. (12)
- **`Start` refuses a feature it cannot assemble around rather than return a partial Brief**,
  via `*assemble.RefusalError` (embeds `Problem`, plus `Line`/`Err`; `…Error` name because
  `errname` is on). Checks run specification → state file → step files → briefed step, first
  failure wins, before any `Brief` field is populated. Structural: specification present +
  readable + fence-closed + carrying `cfg.ProgressHeading`; state file present + readable +
  fence-closed; **briefed** step's frontmatter non-empty `id:` (presence only, never compared
  to `pattern.ID(n)` — 11's ruling stands); briefed step's checklist heading present.
  Present-but-empty stays conforming for both lists (`new step` writes an empty checklist).
  Optional, SCENARIO-14's to degrade: `cfg.AcceptanceHeading` absent, `OptionalConventions`,
  an absent state heading. Every refusal names an absolute path; `Line` set only for the two
  fence cases. `Err` is always the real error, not a synthesized sentinel: checks 1-5/7/8
  wrap `ErrMalformedFeature`, but the step-frontmatter check (readSteps, shared with
  `Status`) wraps whatever `readSteps` produced — `stepfile.ErrNoFrontmatter` for a
  missing/unclosed delimiter, a bare yaml error for bad content — so `errors.Is(err,
  ErrMalformedFeature)` misses that one case by design. `cli/refusal.go` gained a dedicated
  `*assemble.RefusalError` branch (behavior-neutral today: `Error()` already renders
  identically through the generic fallback), dropping `(no files changed)` — read refusals
  never carry it. (13)

## Left unbuilt

- Differing-inputs refusal (16), caps (17/18), state-body heading check (19), open-checklist
  refusal (20), unfinished-`depends-on` refusal (21).
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`, `--json` (15),
  `markdown.Headings` + checklist parser (20/22), R13 truncation, R9's diff/finding output,
  `FinishResult`. No optional-convention shortfall notice — 14 owns it; `start` emits at
  most one stderr line until then.
- `scaffold.HandoffSource` / `cli/finish.go`'s `--handoff` source upgrade — deleted; 17 re-adds.
  `HandoffPattern.Number` — not built; `check` (22) and the R6 synthesis need it.
- `NewStep` and `assemble`'s read path (incl. `Status`) do not re-validate an on-disk feature
  name — `NewFeature`'s creation path is the sole choke point, deliberately. (07)
- `status` does not adopt 13's checks — a feature with no specification is still a normal row
  to `status`, exit 1 to `start`. Owner: `check` (22), reusing `assemble.checkSpecification`.

## Traps

- `scaffold` is named for `new feature`/`new step` while also owning `finish` — unowned rename
  debt. `tickProgressEntry` mutates a whole `[ ]`→`[x]` line, including one inside an
  already-ticked title. Pre-existing, unowned.
- `root = wd` unless `.brief.yaml` found — route through a helper call boundary or gosec's
  `G703` flags it as traversal. `-HANDOFF.md` sorts *before* its step file (`-` < `.`).
- `atomicfile.Create`'s temp-sibling mode can wedge a later write with `permission denied`
  until the sibling is removed by hand. Pre-existing, unowned.
- A whitespace-carrying feature name is a legal directory name everywhere `brief` targets;
  `validateFeatureName` is the only guard — a future exit-2 refusal must branch in `cli`
  before `renderRefusal`. (07)
- **`NewFeature`'s already-exists refusal is guarded twice** (`root.Mkdir` then
  `writeExclusive`'s `O_EXCL`) — mutation-verify individually, never both at once. (08)
- `assemble.RenderText` prints the frontmatter `id:`; `RenderStatusText` prints
  `pattern.ID(n)` — equal in any scaffolded tree, diverge after a hand edit; don't unify. (09)
- **`newProblem`'s non-`*fs.PathError` branch cannot name a specific file, only its `base`
  argument.** `Status`'s `featureStatus` passes `base = feature dir` and pins that exact
  Path in a test, so `Start`'s new id/checklist checks (13) hand-build their own
  `RefusalError` instead of calling `newProblem` — reusing it there would need `base` to be
  both the step file (for `Path`) and the feature dir (for the "brief new step <feature>"
  fix text), which conflict. `newProblem` stays exactly as `Status` needs it. (11, 13)
- **`assemble.RefusalError` duplicates `scaffold.RefusalError`, knowingly** — `assemble`
  must not import `scaffold`. The progress-heading refusal text is duplicated between
  `checkSpecification` and `scaffold.progressRefusal`/`NewStep` — keep the strings identical
  by hand. (11, 13)
- `brief start <f> | wc -c == 0` is exit-code-conditional — a missing/malformed feature also
  gives 0 stdout bytes, at **exit 1**; the scriptable "complete" test is `exit 0 && wc -c ==
  0`. `"step": null` alone can't distinguish complete from zero-steps — 15 needs `done`/`open`
  (`2`/`0` vs `0`/`0`) too. (12)
- **A plan's fixture-update list can under-count.** 13's plan named 6 fixtures needing a
  conforming specification; 4 more needed one and weren't listed — caught only by grepping
  every test writing `cfg.StateFile` with no nearby `cfg.SpecificationFile` write. (13)

## Open debts

- Heading/cap value validation — unowned until SCENARIO-19.
- A mid-write I/O failure leaves a half-applied result a same-argument retry repairs; `check`
  (22) is the not-yet-built proactive detector. Invalid `step-file-pattern` refusal names the
  feature dir, not the config — unowned.
- `insertProgressEntry`/`tickProgressEntry` insert LF into a CRLF file; `SetStatus` returns
  `ErrNoStatusField` for a missing delimiter too. All unowned MINOR.
- `check` (22) must report a leftover `## Handoff` section, and an id/filename mismatch, as
  findings — unowned until then.
- **Data loss:** a symlinked specification or step file is silently replaced by `finish`'s
  rename. Pre-existing, real, unowned — needs its own refusal scenario — dies unless re-opened.
- **SCENARIO-21's `finish` refusal must reuse `status`'s done-set-by-`pattern.ID(n)` rule**, or
  the two surfaces disagree about the same tree; `scaffold` can't import `assemble`, so 21
  reimplements it or the rule moves to `internal/platform/stepfile`. Unowned until 21.
- Whether `status` should *follow* a symlinked feature directory — undecided, unowned, dies
  unless re-opened by its own scenario. (11)
- The `config.InvalidConfigError` refusal still carries the `(no files changed)` tail on a
  read command (`start`), contradicting R14a's read-refusal rule — shared with every write
  command, so fixing it needs `renderRefusal` to know read from write. Pre-existing, unowned.

## Crossover note

An identical re-finish being a true no-op means the crossover's "mark SCENARIO-01 through 06
done" step is safely re-runnable. `SCENARIO-01.md`…`-12.md` and their `-HANDOFF.md` files still
carry no frontmatter — the crossover owns adding it, and **must write `id:` as well as
`status:`**: SCENARIO-13's check 7 (empty `id:` on the briefed step) means a crossover that
adds only `status:` leaves `brief start brief` refused for a new reason once the frontmatter
exists at all. Until then, checks 1-5 already pass on this repo's own tree (its
`specification.md` has the progress heading and balanced fences; `STATE.md` has no fences), so
SCENARIO-13 adds no new lockout — `brief start brief` still fails with `no frontmatter found`,
unchanged from before this scenario. `status` still prints `brief ! ! !` for it.
