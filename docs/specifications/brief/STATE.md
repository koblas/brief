# brief — current state

Scenarios complete: SCENARIO-01..20. Last updated by SCENARIO-20.

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
  participates only in the noop-vs-write split, never a divergence trigger. A done step whose
  handoff file is missing or unreadable is exempt from both refusals and writes as normal —
  `finish` is the only path to a done step (R10). The state arm never uses
  `scaffold.StateSource`; it carries the recorded state file's own absolute path. (HANDOFF-FILE, 06, 16)
- `RefusalError.Line` (0 = whole-path), rendered `<path>:<line>` by `cli/refusal.go`. A
  leftover `## Handoff` section is ignored, never refused. `cli.Run` takes `stdin` before
  `stdout`. (fix ×3, HANDOFF-FILE, 05-06)
- **`NewFeature` has two refusal classes, never unified:** whitespace/empty name →
  `ErrInvalidFeatureName`, exit 2, no `(no files changed)` tail; existing directory →
  `*RefusalError`/`ErrFeatureExists`, exit 1, tail present. (07, 08)
- **`status` lives on `(*assemble.Server).Status`, rendered by `assemble.RenderStatusText`** —
  `assemble` owns reading, `scaffold` owns writing; `next`/`show`/`handoff`/`state get` belong
  here too. `Status` returns `nil`, never `[]`, for zero features; a missing feature root is
  zero features, not an error. (09, 10)
- **A malformed feature degrades into a row, it is never dropped.** `FeatureStatus.Problem
  *Problem{Path, Detail, Fix}`, nil when clean, one per feature, first failure wins; exit 0
  always, R18 gives the failing role to `check` (22). (11)
- **`brief.Step == nil` has two causes; `cli.runStart`, never `assemble`, says which.**
  `runStart` branches on `Done+Open`: `> 0` → "feature is complete"; `== 0` → "no step files
  yet". Both notices use the **absolute** feature-directory path, exit 0. (12)
- **`Start` refuses a feature it cannot assemble around**, via `*assemble.RefusalError`.
  Checks run specification → state file → step files → briefed step, first failure wins,
  before any `Brief` field is populated. `cli/refusal.go` has a dedicated branch, dropping
  `(no files changed)`. (13)
- **An absent acceptance heading in the briefed step, or an absent state heading, degrades
  rather than refuses on `start`.** `Brief.Shortfalls []Shortfall{Path, Detail, Fix}`, nil
  when none. Fires on `Section.Found == false`, never on `Body == ""`. One stderr line per
  entry, absolute path, no tail; exit stays 0, stdout unchanged. (14)
- **`--json` is on `start` and nowhere else.** `json.Marshal` of `assemble.Brief`/`Step`/
  `Section`/`Shortfall`, lowercase tags, `omitempty` on nothing. `RenderJSON` always writes a
  document; `RenderText` writes nothing when `Step` is nil. `splitLeadingPositionals` (shared
  with `finish`) lets `--json` come before or after the feature on `start`. A refusal under
  `--json` is still a plain R14a stderr line at exit 1, no JSON envelope. (15)
- **`(*scaffold.Server).Finish` validates in one fixed band**, ahead of `(refinish).verdict()`:
  handoff cap → state cap → state fence → state headings → open checklist item, all named
  against `HandoffSource`/`StateSource`/the real step path (`HandoffSource`/`StateSource` are
  placeholders `cli/finish.go` upgrades to the real `--handoff`/`--state` path). One shared
  sentinel `ErrOverCap` for both caps; `ErrMissingStateHeading` for the heading check;
  `ErrOpenChecklistItem` (`checkStepChecklist`, naming the step file and the item's 1-based
  line in the whole file) for the checklist. Internal order is mutation-verified test by test:
  handoff cap → state cap → state fence → state headings → open checklist item → spec read →
  `verdict()` — a done step over any of these reports the argument/content defect, never
  `ErrAlreadyFinished`. (17, 18, 19, 20)
- **The state-heading check is presence-only, any order, empty sections valid** —
  `markdown.Section(...).found == false`, identical to 14's read-side rule. Order is
  deliberately unenforced (`assemble.stateSections` reads by name). One refusal line names the
  first missing heading; `Finish` stops at the first fault, unlike 14's degrade path. (19)
- **`markdown.FirstUnchecked(body, heading)` is the sole checklist-item scanner** — grammar
  `^\s*- \[[ xX]\]`, hyphen bullet only, any leading whitespace, `x`/`X` ticked; fence-aware,
  same-or-higher-level stop via unexported `sectionSpan` (shared with `sectionRange`). Zero
  items, an absent heading, and every item ticked are all `found == false`, indistinguishable
  to a caller. `render.go`'s own `checklistItemRe` (`[ x]`) stays separate and un-widened,
  deliberately: `tickProgressEntry`'s `strings.Replace(line, "[ ]", "[x]", 1)` would misreport
  a tick on `[X]` if unified. Both this check and the state-heading check narrow R11 further: a
  done step's re-finish is a no-op only when its checklist is also complete. (20)

## Left unbuilt

- Unfinished-`depends-on` refusal (21) — must reuse 20's `checkStepChecklist` position rule
  and insert its own check *after* it, so a step both un-ticked and blocked reports the
  checklist item first. `--force`/`--if-state-matches` and any diff/finding output on a
  re-finish divergence (R9) — deferred by 16, no owner. No un-finish/un-done verb planned.
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`, `status --json` and `--json` on
  `next`/`show`/`state get`/`handoff` — deferred, not missed (15). `markdown.Headings` and
  `markdown.ChecklistItems` (an all-items listing, past the single-item `FirstUnchecked`) —
  owner: `check` (22). R9's diff/finding output, `FinishResult`.
- `HandoffPattern.Number` — not built; `check` (22) and the R6 synthesis need it.
- `NewStep` and `assemble`'s read path (incl. `Status`) do not re-validate an on-disk feature
  name — `NewFeature`'s creation path is the sole choke point, deliberately. (07)
- `status` does not adopt 13's checks. Owner: `check` (22), reusing `assemble.checkSpecification`.
- `cfg.OptionalConventions` consumption — no owner. No JSON error/refusal envelope, no
  `"feature"` key, no schema-version key. (14, 15)
- `ErrMissingStateHeading` has no `check` (22) counterpart yet; R18's backstop for pre-19
  state files is still unwritten.
- Nothing reports a `* [ ]` or `- []` line as an open item — a hand-written step using those
  bullets finishes silently. Owner: `check` (22) as a finding, or nobody. (20)

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
- `brief start <f> | wc -c == 0` is exit-code-conditional; under `--json`, stdout is **never**
  empty on an exit-0 run — the completion test is `"step":null` plus `done`/`open`, not byte
  count. (12, 15)
- **A plan's fixture-update list can under-count — enumerate by running the suite, never by
  reading one file.** 19's plan named six fixtures needing a heading-carrying state body; the
  suite surfaced three more (`internal/scaffold/finish_test.go`'s trivial `"s"` state bodies).
  Two of 19's six were the dangerous kind: they still failed, but with the wrong sentinel
  (`ErrMissingStateHeading` instead of `ErrAlreadyFinished`) rather than a build break. (13, 14, 19)
- **13's refusal and 14's shortfall can share a `Detail` string** — always also assert exit
  code and stdout non-emptiness; use `Section.Found`, not body emptiness. 19's own refusal
  copy (`state is missing the %q section`) is deliberately unlike 14's (`no %q heading
  found`), for the same reason. (14, 19)
- **The struct tags in `brief.go` are the wire contract.** `SetEscapeHTML(false)` is
  deliberate. Unmarshalling collapses null-vs-absent — assert on raw bytes, not decoded
  values. (15)
- **This pipeline's own fix-mode re-runs of `finish` on an already-done step hit 16's
  refusal** whenever the regenerated handoff or STATE.md body differs from what is recorded.
  Resolution: read the recorded file and edit it directly, or re-run with the recorded body. (16)
- **A byte-snapshot alone can't prove a refusal happened before any write** — pair it with the
  `pinnedModTime`/`pinModTimes`/`modTimes` probe. Assert `Equal` on the whole refusal line,
  never `Contains`, where pre-refusal code could print similar text on the same inputs. (16)
- **`cli/finish.go`'s placeholder swap only upgrades a `RefusalError.Path` it recognizes** —
  `StateSource` and `HandoffSource` both have branches; any future placeholder needs its own. (17, 18)
- **A "reports X first" test is vacuous without the swap/reverse mutation** — true for the cap
  band's ordering tests (17, 18) and for 19's `Ordered()`-position tests alike; the one-sided
  test alone also passes with the *other* thing deleted, or with only index 0 ever checked.
- **`markdown.Section(body, "")` returns `found == true`** — `findHeading` matches the first
  blank line, so a repository configuring an empty state heading (`traps: ""`) would pass 19's
  check silently. Not fixed; the concrete hole under the heading-value-validation debt below. (19)
- **This repository's own `STATE.md` is now over the 80-line cap `brief` enforces** —
  `brief finish brief <step>` refuses on this feature's own state body (R18/19: also over the
  heading-band's line-cap ordering, harmless since the tool is not self-hosted). `check` (22)
  gets the reporting role, per R18.

## Open debts

- Heading/cap **value** validation (an empty or duplicate configured heading, a non-positive
  cap) — unowned; SCENARIO-19 validates the argument against the schema, not the schema
  itself.
- **This repository's handoff files (29–373 lines) still individually exceed the 60-line
  default handoff cap** in places — harmless today for the same self-hosting reason above.
  Owner: `check` (22), per R18. (17)
- A mid-write I/O failure leaves a half-applied result a same-argument retry repairs; `check`
  (22) is the not-yet-built proactive detector. Invalid `step-file-pattern` refusal names the
  feature dir, not the config — unowned.
- `insertProgressEntry`/`tickProgressEntry` insert LF into a CRLF file; `SetStatus` returns
  `ErrNoStatusField` for a missing delimiter too. All unowned MINOR.
- `check` (22) must report a leftover `## Handoff` section, an id/filename mismatch, a
  pre-19 state body missing a heading, and an open checklist item outside of `finish`
  refusing on it, as findings — unowned until then.
- **Data loss:** a symlinked specification or step file is silently replaced by `finish`'s
  rename. Pre-existing, real, unowned — needs its own refusal scenario — dies unless re-opened.
- **21's `finish` refusal must reuse `status`'s done-set-by-`pattern.ID(n)` rule**, or the two
  surfaces disagree; `scaffold` can't import `assemble`, so 21 reimplements it or the rule
  moves to `internal/platform/stepfile`. Unowned until 21.
- Whether `status` should *follow* a symlinked feature directory — undecided, unowned. (11)
- The `config.InvalidConfigError` refusal still carries the `(no files changed)` tail on a
  read command (`start`), contradicting R14a. Pre-existing, unowned.
- **R13's output budget and truncation is unowned, and must never truncate a `--json`
  payload.** `cfg.DefaultOutputBudgetBytes` (8192) remains unconsumed. (15)

## Crossover note

An identical re-finish being a true no-op means the crossover's "mark SCENARIO-01 through 06
done" step is safely re-runnable **only when re-run with the same handoff/state bytes**, and,
as of 19, only when that state body also carries all four configured headings — a
regenerated body now hits 16's refusal, or 19's, instead of silently overwriting.
`SCENARIO-01.md`…`-12.md` and their `-HANDOFF.md` files still carry no frontmatter — the
crossover owns adding it, and **must write `id:` as well as `status:`**. Until then, checks
1-5 already pass on this repo's own tree, so 13 adds no new lockout — `brief start brief`
still fails with `no frontmatter found`, unchanged. `status` still prints `brief ! ! !` for it.
