# brief — current state

Scenarios complete: SCENARIO-01..10. Last updated by SCENARIO-10.

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
  only via `errors.Is(err, fs.ErrNotExist)` on `os.OpenRoot`; every other open failure (e.g.
  `ENOTDIR`) still propagates. `cli.runStatus` keys the notice on `len(rows) == 0`, writing
  `brief status: no features found in <cfg.FeatureDirectory>; run 'brief new feature <name>'
  to create one` to stderr and returning before `RenderStatusText`. Not R14a's shape — no
  `<path>:` prefix, no `(no files changed)` tail. (10)

## Left unbuilt

- Differing-inputs refusal (16), caps (17/18), state-body heading check (19), open-checklist
  refusal (20), unfinished-`depends-on` refusal (21).
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`, `--json` (15), complete-feature
  stderr+exit-0 (12), malformed-feature tolerance + `!` field (11), `markdown.Headings` +
  checklist parser (20/22), R13 truncation, R9's diff/finding output, `FinishResult`.
- `scaffold.HandoffSource` / `cli/finish.go`'s `--handoff` source upgrade — deleted; 17 re-adds.
- `HandoffPattern.Number` — not built; `check` (22) and the R6 synthesis need it.
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
- **`brief status` cannot read `brief`'s own tree today** — no `SCENARIO-NN.md` here carries
  frontmatter: exit 1, "no frontmatter found". A feature dir with zero step files is
  conforming (`0/0 - 0`); only a root with zero *directory entries* triggers "no features". (09)
- **A typo'd `feature-directory` exits 0** ("no features found in `<typo>`") — the R14 shape,
  not a bug; don't "fix" into a refusal without reopening SCENARIO-10. `errors.Is(err,
  fs.ErrNotExist)` only, never a string match (`ENOTDIR`'s message differs). Notice keys on
  `len(rows) == 0`, not an error value — **11 must emit a row for a malformed feature, not
  drop it**, or a repo whose only feature is malformed prints "no features found" and exits 0. (10)

## Open debts

- Heading/cap value validation — unowned until SCENARIO-19.
- A mid-write I/O failure leaves a half-applied result a same-argument retry repairs; `check`
  (22) is the not-yet-built proactive detector. Invalid `step-file-pattern` refusal names the
  feature dir, not the config — unowned.
- `insertProgressEntry`/`tickProgressEntry` insert LF into a CRLF file; `SetStatus` returns
  `ErrNoStatusField` for a missing delimiter too. All unowned MINOR.
- `assemble`'s sentinels duplicate `scaffold`'s (never a `*RefusalError`) — SCENARIO-13 closes.
- `check` (22) must report a leftover `## Handoff` section as a finding — unowned until then.
- **Data loss:** a symlinked specification or step file is silently replaced by `finish`'s
  rename. Pre-existing, real, unowned — needs its own refusal scenario — dies unless re-opened.
- **SCENARIO-21's `finish` refusal must reuse `status`'s done-set-by-`pattern.ID(n)` rule**, or
  the two surfaces disagree about the same tree; `scaffold` can't import `assemble`, so 21
  reimplements it or the rule moves to `internal/platform/stepfile`. Unowned until 21.

## Crossover note

An identical re-finish being a true no-op means the crossover's "mark SCENARIO-01 through 06
done" step is safely re-runnable. `SCENARIO-01.md`…`-10.md` and their `-HANDOFF.md` files still
carry no frontmatter — the crossover owns adding it, and until then neither `start` nor
`status` can read `brief`'s own feature directory (see Traps).
