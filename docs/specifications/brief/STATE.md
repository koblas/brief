# brief — current state

Scenarios complete: SCENARIO-01..11. Last updated by SCENARIO-11.

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
  here too. Line: `<name> <done>/<total> <next> <blocked>\n`, no header/legend. `<next>` =
  `pattern.ID(n)` of the lowest not-done step, ignoring `depends-on` like `Start`. **Blocked**
  = not-done step with ≥1 unresolved *direct* `depends-on` id (unknown id blocks, done step
  never blocked). Feature order is `fs.ReadDir` byte order, not re-sorted. `Status` returns
  `nil`, never `[]`, for zero features — 15's `--json` depends on that marshaling to `null`. (09)
- **A missing feature root is zero features, not an error** — `Status` returns `(nil, nil)`
  only via `errors.Is(err, fs.ErrNotExist)` on `os.OpenRoot`; every other top-level open/list
  failure, or an invalid step-file pattern, still propagates. `cli.runStatus` keys the notice
  on `len(rows) == 0`: `brief status: no features found in <cfg.FeatureDirectory>; run 'brief
  new feature <name>' to create one` to stderr, exit 0. (10)
- **A malformed feature degrades into a row, it is never dropped.** `FeatureStatus.Problem
  *Problem{Path, Detail, Fix}`, nil when clean; set means `Done`/`Total`/`Next`/`Blocked` stay
  zero. Renders `<name> ! ! !` — four single-token fields, never a single-field marker (`-`
  and `0/0` already mean something real). Tolerance lives in `featureStatus`, never
  `readSteps` (shared with `Start`; SCENARIO-13's red needs `readSteps` to stay intolerant).
  One Problem per feature, first failure wins. `runStatus` writes `brief status: <absolute
  path>: <detail>; <fix>\n` per malformed row, inline, not via `renderRefusal` — exit 0
  always, R18 gives the failing role to `check` (22). `Status`'s entry filter is three-way:
  directory → read; symlink → `!` row named after it, never followed/resolved (target
  irrelevant); everything else → skipped, no row (10's decision, unchanged — "not a directory
  → mark" would wrongly catch a stray `README.md`). An id/filename mismatch and an empty
  feature directory stay conforming, not malformed. (11)

## Left unbuilt

- Differing-inputs refusal (16), caps (17/18), state-body heading check (19), open-checklist
  refusal (20), unfinished-`depends-on` refusal (21).
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`, `--json` (15), complete-feature
  stderr+exit-0 (12), `markdown.Headings` + checklist parser (20/22), R13 truncation, R9's
  diff/finding output, `FinishResult`, `Problem.Line` (13 adds one if its refusal needs it).
- `scaffold.HandoffSource` / `cli/finish.go`'s `--handoff` source upgrade — deleted; 17 re-adds.
  `HandoffPattern.Number` — not built; `check` (22) and the R6 synthesis need it.
- `NewStep` and `assemble`'s read path (incl. `Status`) do not re-validate an on-disk feature
  name — `NewFeature`'s creation path is the sole choke point, deliberately. (07)

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
- **`stepfile.ParseFrontmatter`'s errors name no file** — `newProblem`'s `nameable` param
  exists because of this: only an OS-level `*fs.PathError` (open/read failure) names its own
  file via `.Path`; a parse failure gets only the feature-directory path. (11)
- **`assemble.Problem` duplicates `scaffold.RefusalError` minus `Line`, knowingly** —
  `assemble` must not import `scaffold`. Field names don't line up: `RefusalError.Problem` is
  the text field, `Problem.Detail` is. (11)

## Open debts

- Heading/cap value validation — unowned until SCENARIO-19.
- A mid-write I/O failure leaves a half-applied result a same-argument retry repairs; `check`
  (22) is the not-yet-built proactive detector. Invalid `step-file-pattern` refusal names the
  feature dir, not the config — unowned.
- `insertProgressEntry`/`tickProgressEntry` insert LF into a CRLF file; `SetStatus` returns
  `ErrNoStatusField` for a missing delimiter too. All unowned MINOR.
- `assemble`'s sentinels duplicate `scaffold`'s (never a `*RefusalError`) — SCENARIO-13 closes.
- `check` (22) must report a leftover `## Handoff` section, and an id/filename mismatch, as
  findings — unowned until then.
- **Data loss:** a symlinked specification or step file is silently replaced by `finish`'s
  rename. Pre-existing, real, unowned — needs its own refusal scenario — dies unless re-opened.
- **SCENARIO-21's `finish` refusal must reuse `status`'s done-set-by-`pattern.ID(n)` rule**, or
  the two surfaces disagree about the same tree; `scaffold` can't import `assemble`, so 21
  reimplements it or the rule moves to `internal/platform/stepfile`. Unowned until 21.
- Whether `status` should *follow* a symlinked feature directory — undecided, unowned, dies
  unless re-opened by its own scenario. (11)

## Crossover note

An identical re-finish being a true no-op means the crossover's "mark SCENARIO-01 through 06
done" step is safely re-runnable. `SCENARIO-01.md`…`-11.md` and their `-HANDOFF.md` files still
carry no frontmatter — the crossover owns adding it. Until then, `start` refuses `brief`'s own
feature directory (`ErrMalformedFeature`) while `status` prints `brief ! ! !` for it (11
changed status's failure mode from exit 1 to a marked row; start is untouched).
