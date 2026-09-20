# brief — current state

Scenarios complete: SCENARIO-01..18. Last updated by SCENARIO-18.

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
  any point; `status: done` never lands before the handoff file exists. **One predicate,
  `(scaffold.refinish).verdict()`, decides every re-finish of a done step** — identity/no-op
  (R11), or a refusal wrapping `scaffold.ErrAlreadyFinished` naming the specific divergent
  file (recorded handoff file, or `cfg.StateFile`, handoff checked first). `specTicked`
  participates only in the noop-vs-write split, never as a divergence trigger. A done step
  whose handoff file is missing or unreadable is exempt from both refusals and writes as
  normal — `finish` is the only path to a done step (R10). The state arm never uses
  `scaffold.StateSource`; it carries the recorded state file's own absolute path. (HANDOFF-FILE, 06, 16)
- `RefusalError.Line` (0 = whole-path), rendered `<path>:<line>` by `cli/refusal.go`. A
  leftover `## Handoff` section is ignored, never refused. `cli.Run` takes `stdin` before
  `stdout`. (fix ×3, HANDOFF-FILE, 05-06)
- **`NewFeature` has two refusal classes, never unified:** whitespace/empty name →
  `ErrInvalidFeatureName`, exit 2, no `(no files changed)` tail; existing directory →
  `*RefusalError`/`ErrFeatureExists`, exit 1, tail present. (07, 08)
- **`status` lives on `(*assemble.Server).Status`, rendered by `assemble.RenderStatusText`** —
  `assemble` owns reading, `scaffold` owns writing; `next`/`show`/`handoff`/`state get` belong
  here too. Line: `<name> <done>/<total> <next> <blocked>\n`. `Status` returns `nil`, never
  `[]`, for zero features; a missing feature root is zero features, not an error. (09, 10)
- **A malformed feature degrades into a row, it is never dropped.** `FeatureStatus.Problem
  *Problem{Path, Detail, Fix}`, nil when clean, one per feature, first failure wins; exit 0
  always, R18 gives the failing role to `check` (22). (11)
- **`brief.Step == nil` has two causes; `cli.runStart`, never `assemble`, says which.**
  `runStart` branches on `Done+Open`: `> 0` → "feature is complete"; `== 0` → "no step files
  yet". Both notices use the **absolute** feature-directory path, exit 0. (12)
- **`Start` refuses a feature it cannot assemble around**, via `*assemble.RefusalError`
  (embeds `Problem`, plus `Line`/`Err`). Checks run specification → state file → step files
  → briefed step, first failure wins, before any `Brief` field is populated. `cli/refusal.go`
  has a dedicated `*assemble.RefusalError` branch, dropping `(no files changed)`. (13)
- **An absent acceptance heading in the briefed step, or an absent state heading, degrades
  rather than refuses.** `Brief.Shortfalls []Shortfall{Path, Detail, Fix}`, nil when none.
  Fires on `Section.Found == false`, never on `Body == ""`. One stderr line per entry,
  absolute path, no tail; exit stays 0, stdout unchanged. (14)
- **`--json` is on `start` and nowhere else.** `json.Marshal` of `assemble.Brief`/`Step`/
  `Section`/`Shortfall`, lowercase tags, **`omitempty` on nothing**. `assemble.RenderJSON`
  always writes a document; `RenderText` writes nothing when `Step` is nil.
  `splitLeadingPositionals` (in `cli.go`, shared with `finish`) lets `--json` come before or
  after the feature on `start`; `finish` does **not** need this merge. A refusal under
  `--json` is still a plain R14a stderr line at exit 1, no JSON envelope. (15)
- **Length caps are measured in lines, by `markdown.CountLines`** (lives in `platform/markdown`,
  not `scaffold`, since `check` (22) needs the same counter from the read side and `assemble`
  may not import `scaffold`). `(*scaffold.Server).Finish` now checks **both** arguments via
  the generic `checkArgumentCap(body, source, label, limit)` helper, in one adjacent "cap
  band": handoff cap, then state cap, then `checkArgumentFence(state, …)` — in that fixed
  order, ahead of `(refinish).verdict()`. Three consequences, each mutation-verified: handoff
  reported first when both bodies are over; state cap reported before the state's unclosed
  fence; an over-cap body (either) on an already-done step reports `ErrOverCap`, never
  `ErrAlreadyFinished`. One shared sentinel `ErrOverCap` for both caps; `RefusalError.Path`
  (`HandoffSource`/`StateSource`) says which body. Boundary is `count > cap` (exactly-at-cap
  accepted). `cli/finish.go`'s placeholder-to-real-path swap is a `switch` with a branch per
  source. (17, 18)

## Left unbuilt

- State-body heading check (19), open-checklist refusal (20), unfinished-`depends-on` refusal
  (21). `--force`/`--if-state-matches` and any diff/finding output on a re-finish divergence
  (R9) — deferred by 16, no owner. No un-finish/un-done verb planned.
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`, `status --json` and `--json` on
  `next`/`show`/`state get`/`handoff` — deferred, not missed (15). `markdown.Headings` +
  checklist parser (20/22), R9's diff/finding output, `FinishResult`.
- `HandoffPattern.Number` — not built; `check` (22) and the R6 synthesis need it.
- `NewStep` and `assemble`'s read path (incl. `Status`) do not re-validate an on-disk feature
  name — `NewFeature`'s creation path is the sole choke point, deliberately. (07)
- `status` does not adopt 13's checks. Owner: `check` (22), reusing `assemble.checkSpecification`.
- `cfg.OptionalConventions` consumption — no owner. No JSON error/refusal envelope, no
  `"feature"` key, no schema-version key. (14, 15)
- Validation of cap *values* (0 or negative accepted as configured) — unowned.

## Traps

- `scaffold` is named for `new feature`/`new step` while also owning `finish` — unowned rename
  debt. `tickProgressEntry` mutates a whole `[ ]`→`[x]` line, including one inside an
  already-ticked title. Pre-existing, unowned.
- `root = wd` unless `.brief.yaml` found — route through a helper call boundary or gosec's
  `G703` flags it as traversal. `-HANDOFF.md` sorts *before* its step file (`-` < `.`).
- `atomicfile.Create`'s temp-sibling mode can wedge a later write with `permission denied`
  until the sibling is removed by hand. Pre-existing, unowned.
- A whitespace-carrying feature name is a legal directory name everywhere `brief` targets;
  `validateFeatureName` is the only guard. (07)
- **`NewFeature`'s already-exists refusal is guarded twice** (`root.Mkdir` then
  `writeExclusive`'s `O_EXCL`) — mutation-verify individually, never both at once. (08)
- **`assemble.RefusalError` duplicates `scaffold.RefusalError`, knowingly** — `assemble` must
  not import `scaffold`. Progress-heading refusal text duplicated between
  `checkSpecification` and `scaffold.progressRefusal`/`NewStep` — keep identical by hand. (11, 13)
- `brief start <f> | wc -c == 0` is exit-code-conditional — a missing/malformed feature also
  gives 0 stdout bytes, at **exit 1**. Under `--json`, stdout is **never** empty on an exit-0
  run: the completion test is `"step":null` plus `done`/`open`, not byte count. (12, 15)
- **A plan's fixture-update list can under-count** — enumerate, never estimate. (13, 14)
- **13's refusal and 14's shortfall can share a `Detail` string** — always also assert exit
  code and stdout non-emptiness; use `Section.Found`, not body emptiness. (14)
- **The struct tags in `brief.go` are the wire contract.** `SetEscapeHTML(false)` is
  deliberate. Unmarshalling collapses null-vs-absent — assert on raw bytes, not decoded
  values. (15)
- **This pipeline's own fix-mode re-runs of `finish` on an already-done step hit SCENARIO-16's
  refusal** whenever the regenerated handoff or STATE.md body differs from what is recorded.
  Resolution: read the recorded file and edit it directly, or re-run with the recorded body. (16)
- **A byte-snapshot alone can't prove a refusal happened before any write** — pair it with the
  `pinnedModTime`/`pinModTimes`/`modTimes` probe. `Contains` on refusal text is unsafe where
  pre-refusal code printed similar text on the same inputs — assert `Equal` on the whole line. (16)
- **`cli/finish.go`'s placeholder swap only upgrades a `RefusalError.Path` it recognizes** —
  both `StateSource` and `HandoffSource` have branches today; any future placeholder needs
  its own. (17, 18)
- **`Test_reports_the_handoff_cap_first_when_both_bodies_are_over_their_caps` (and its state
  twin, if one is ever added) passes with the *other* cap check deleted** — the only real
  proof either cap is checked first is the mutation that swaps the two call sites. Re-run
  that mutation, not just the suite, on any future edit to the cap band. (17, 18)
- **The cap tests' bodies are bare line-text with no state headings** — they finish
  successfully today because nothing validates the state body's structure.
  **SCENARIO-19 will refuse them.** 19 owns giving every at-cap/accepting state fixture the
  four configured headings; its heading check must run **after** `checkArgumentFence`, not
  inside the cap band — a body that fails the fence scan cannot have its headings read. (18)
- **This repository's own `STATE.md` is now over the 80-line cap `brief` enforces** —
  `brief finish brief <step>` refuses on this feature's own state body as of SCENARIO-18.
  Present tense, not prospective. Harmless in practice (the tool is not self-hosted;
  `brief start brief` still fails on missing frontmatter; pipeline agents write `STATE.md` by
  hand). R18 gives the reporting role to `check` (22). (18)

## Open debts

- Heading/cap value validation — unowned until SCENARIO-19.
- **This repository's handoff files (29–373 lines) still individually exceed the 60-line
  default handoff cap** in places — harmless today for the same self-hosting reason above.
  Owner: `check` (22), per R18. (17)
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
  the two surfaces disagree; `scaffold` can't import `assemble`, so 21 reimplements it or the
  rule moves to `internal/platform/stepfile`. Unowned until 21.
- Whether `status` should *follow* a symlinked feature directory — undecided, unowned. (11)
- The `config.InvalidConfigError` refusal still carries the `(no files changed)` tail on a
  read command (`start`), contradicting R14a. Pre-existing, unowned.
- **R13's output budget and truncation is unowned, and must never truncate a `--json`
  payload.** `cfg.DefaultOutputBudgetBytes` (8192) remains unconsumed. (15)

## Crossover note

An identical re-finish being a true no-op means the crossover's "mark SCENARIO-01 through 06
done" step is safely re-runnable **only when re-run with the same handoff/state bytes** — a
regenerated body now hits SCENARIO-16's refusal instead of silently overwriting (see Traps).
`SCENARIO-01.md`…`-12.md` and their `-HANDOFF.md` files still carry no frontmatter — the
crossover owns adding it, and **must write `id:` as well as `status:`**. Until then, checks
1-5 already pass on this repo's own tree, so SCENARIO-13 adds no new lockout — `brief start
brief` still fails with `no frontmatter found`, unchanged. `status` still prints
`brief ! ! !` for it.
