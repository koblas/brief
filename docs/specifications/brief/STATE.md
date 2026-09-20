# brief — current state

Scenarios complete: SCENARIO-01..17. Last updated by SCENARIO-17.

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
  (R11: identical handoff+state+spec-tick writes nothing, mtime preserved), or a refusal
  wrapping `scaffold.ErrAlreadyFinished` naming the specific divergent file (recorded
  handoff file, or `cfg.StateFile`, handoff checked first). `specTicked` participates only
  in the noop-vs-write split, never as a divergence trigger — an un-ticked progress entry
  means "half-applied write to repair", not "different inputs". A done step whose handoff
  file is missing or unreadable is exempt from both refusals and writes as normal — `finish`
  is the only path to a done step (R10), so refusing there is a dead end. The state arm
  never uses `scaffold.StateSource`; it carries the recorded state file's own absolute path,
  since `cli/finish.go`'s placeholder upgrade would repoint it at the caller's `--state`
  input instead. (HANDOFF-FILE, 06, 16)
- `RefusalError.Line` (0 = whole-path), rendered `<path>:<line>` by `cli/refusal.go`. A
  leftover `## Handoff` section is ignored, never refused. `cli.Run` takes `stdin` before
  `stdout`. (fix ×3, HANDOFF-FILE, 05-06)
- **`NewFeature` has two refusal classes, never unified:** whitespace/empty name →
  `ErrInvalidFeatureName`, exit 2, no `(no files changed)` tail; existing directory →
  `*RefusalError`/`ErrFeatureExists`, exit 1, tail present. (07, 08)
- **`status` lives on `(*assemble.Server).Status`, rendered by `assemble.RenderStatusText`** —
  `assemble` owns reading, `scaffold` owns writing; `next`/`show`/`handoff`/`state get` belong
  here too. Line: `<name> <done>/<total> <next> <blocked>\n`, no header/legend, `-` when
  `Next` is empty. `Status` returns `nil`, never `[]`, for zero features. A missing feature
  root is zero features, not an error; every other open/list failure, or an invalid step-file
  pattern, still propagates. (09, 10)
- **A malformed feature degrades into a row, it is never dropped.** `FeatureStatus.Problem
  *Problem{Path, Detail, Fix}`, nil when clean. Tolerance lives in `featureStatus`, never
  `readSteps` (shared with `Start`, which stays intolerant). One Problem per feature, first
  failure wins; `runStatus` writes it inline, not via `renderRefusal` — exit 0 always, R18
  gives the failing role to `check` (22). An id/filename mismatch and an empty feature
  directory stay conforming, not malformed. (11)
- **`brief.Step == nil` has two causes; `cli.runStart`, never `assemble`, says which.**
  `assemble.Start` returns `(Brief, nil)` for both. `runStart` branches on `Done+Open`: `> 0` →
  "feature is complete"; `== 0` → "no step files yet". Both notices use the **absolute**
  feature-directory path, exit 0, no `(no files changed)` tail. (12)
- **`Start` refuses a feature it cannot assemble around**, via `*assemble.RefusalError`
  (embeds `Problem`, plus `Line`/`Err`). Checks run specification → state file → step files
  → briefed step, first failure wins, before any `Brief` field is populated: specification
  present + readable + fence-closed + carrying `cfg.ProgressHeading`; state file present +
  readable + fence-closed; briefed step's `id:` non-empty (presence only); briefed step's
  checklist heading present. Present-but-empty stays conforming for both lists. `Err` is
  always the real error: checks 1-5/7/8 wrap `ErrMalformedFeature`, but the step-frontmatter
  check wraps whatever `readSteps` produced (`stepfile.ErrNoFrontmatter` or a bare yaml
  error), so `errors.Is(err, ErrMalformedFeature)` misses that one case by design.
  `cli/refusal.go` has a dedicated `*assemble.RefusalError` branch, dropping
  `(no files changed)` — read refusals never carry it. (13)
- **An absent acceptance heading in the briefed step, or an absent state heading, degrades
  rather than refuses.** `Brief.Shortfalls []Shortfall{Path, Detail, Fix}`, nil when none,
  appended by `Start` after the 13-era checks: acceptance first, then
  `cfg.StateHeadings.Ordered()` — the same order `RenderText` renders in. Fires on
  `Section.Found == false`, never on `Body == ""` (13 ruled present-but-empty conforming;
  `stateSkeleton` writes all four state headings bare, so `Found` is the only working
  discriminator). `cli.runStart` writes one `brief start: <abs path>: <detail>; <fix>` line
  per entry to stderr, absolute path, no `(no files changed)` tail, before the stdout brief;
  exit stays 0 and stdout is byte-identical to before (`render.go` untouched).
  `cfg.OptionalConventions` stays unconsumed — no vocabulary defines an entry, and no
  Gherkin needs one; the conventions in effect are exactly `cfg.AcceptanceHeading` and the
  four `cfg.StateHeadings`. (14)
- **`--json` is on `start` and nowhere else** — `status --json` and `--json` on
  `next`/`show`/`state get`/`handoff` are deferred, not missed (see *Left unbuilt*). The
  payload is `json.Marshal` of `assemble.Brief`/`Step`/`Section`/`Shortfall`, every field
  tagged lowercase, **`omitempty` on nothing**: nil marshals to `null` uniformly
  (`"step":null`, `"shortfalls":null`), and `done`/`open` survive at `0` because that is
  what separates a complete feature (`2`/`0`) from one with no step files (`0`/`0`) —
  `"step":null` alone can't. `assemble.RenderJSON` buffers through `json.NewEncoder` with
  `SetEscapeHTML(false)`, one `Write`, and **always** writes a document — unlike
  `RenderText`, which writes nothing when `Step` is nil. `cli.runStart` gates only the
  renderer choice on the flag; the shortfall/complete/no-steps stderr notices are written
  unconditionally, same as before `--json` existed. `start` accepts `--json` before or after
  the feature: `splitLeadingPositionals` (moved to `cli.go`, shared with `finish`) gives the
  leading run, and `runStart` merges it with `fs.Parse`'s own `fs.Args()` via
  `slices.Concat` — `finish` does **not** need this merge, because its feature and step
  always precede its flags; do not backport it there without a reason. A refusal under
  `--json` is still a plain R14a stderr line at exit 1, no JSON envelope — `Start` fails
  before anything is rendered, so stdout is zero bytes by construction. (15)
- **Length caps are measured in lines, by `markdown.CountLines`** (`strings.Count(body,
  "\n")` plus 1 for a non-empty body with no trailing newline; `""` is 0) — lives in
  `platform/markdown`, not `scaffold`, since `check` (22) needs the same counter from the
  read side and `assemble` may not import `scaffold`. `(*scaffold.Server).Finish` checks the
  handoff argument against `cfg.HandoffCapLines` via `checkArgumentCap`, a generic helper
  parameterised on label/source/limit so SCENARIO-18 reuses it verbatim for the state
  argument. The check runs in `Finish`'s argument band, immediately before
  `checkArgumentFence`, so it pre-empts `(refinish).verdict()`: an over-cap handoff on an
  already-done step reports `ErrOverCap`, never `ErrAlreadyFinished` — one shared sentinel for
  both caps, `RefusalError.Path` (`HandoffSource`/`StateSource`) says which body. Boundary is
  `count > cap` (exactly-at-cap accepted), mutation-verified both ways. `cli/finish.go`'s
  `StateSource`-to-real-path swap is now a `switch` with a second `HandoffSource` branch. (17)

## Left unbuilt

- State cap (18, `cfg.StateCapLines` unconsumed), state-body heading check (19),
  open-checklist refusal (20), unfinished-`depends-on` refusal (21). `--force`/
  `--if-state-matches` and any diff/finding output on a re-finish divergence (R9) — deferred
  by 16, no owner. No un-finish/un-done verb planned; the documented escape from 16's refusal
  is to edit the recorded file directly.
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`, `status --json` and `--json` on
  `next`/`show`/`state get`/`handoff` (deferred by *Decisions taken* 3 / open question 8, not
  missed — 15), `markdown.Headings` + checklist parser (20/22), R9's diff/finding output,
  `FinishResult`.
- `HandoffPattern.Number` — not built; `check` (22) and the R6 synthesis need it.
- `NewStep` and `assemble`'s read path (incl. `Status`) do not re-validate an on-disk feature
  name — `NewFeature`'s creation path is the sole choke point, deliberately. (07)
- `status` does not adopt 13's checks — a feature with no specification is still a normal row
  to `status`, exit 1 to `start`. Owner: `check` (22), reusing `assemble.checkSpecification`.
- `cfg.OptionalConventions` consumption, and any convention-name vocabulary — no owner. An
  empty configured heading as an off-switch (`acceptance-heading: ""`) is not built either:
  `markdown.Section` would still match the first blank line, so `Found` stays true. (14)
- No JSON error/refusal envelope, no `"feature"` key in the JSON payload, no schema-version
  key. Top-level `usage` in `cli.go` still lists `finish`'s flags inline but not `start
  --json` — deliberately untouched, out of 15's scope. (15)

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
  argument.** `Start`'s id/checklist checks hand-build their own `RefusalError` instead of
  calling `newProblem` for this reason — `newProblem` stays exactly as `Status` needs it.
  (11, 13)
- **`assemble.RefusalError` duplicates `scaffold.RefusalError`, knowingly** — `assemble`
  must not import `scaffold`. The progress-heading refusal text is duplicated between
  `checkSpecification` and `scaffold.progressRefusal`/`NewStep` — keep the strings identical
  by hand. (11, 13)
- `brief start <f> | wc -c == 0` is exit-code-conditional — a missing/malformed feature also
  gives 0 stdout bytes, at **exit 1**; the scriptable "complete" test is `exit 0 && wc -c ==
  0`. Under `--json`, stdout is **never** empty on an exit-0 run: the structured completion
  test is `"step":null` plus `done`/`open` (`2`/`0` vs `0`/`0`), not byte count. (12, 15)
- **A plan's fixture-update list can under-count.** 13's plan named 6 fixtures needing a
  conforming specification; 4 more needed one and weren't listed — caught only by grepping
  every test writing `cfg.StateFile` with no nearby `cfg.SpecificationFile` write. The method
  — enumerate, never estimate — is what to repeat. (13, 14)
- **13's refusal and 14's shortfall can share a `Detail` string** (`no "## X" heading
  found`), so a test asserting only `Contains` on that text cannot tell them apart — always
  also assert exit code and stdout non-emptiness. `RenderText`'s omit-on-empty-body rule
  means stdout itself can't distinguish absent from present-but-empty either; use
  `Section.Found`. A freshly scaffolded feature (`new feature` + `new step`) is not
  shortfall-free: `stepSkeleton` writes no acceptance heading, so `start` on it now prints
  one stderr line — intended, not a bug in `stepSkeleton`. (14)
- **The struct tags in `brief.go` are the wire contract.** Renaming a `Brief`/`Step`/
  `Section`/`Shortfall` field silently changes user-visible JSON unless the tag moves with
  it; without a tag Go falls back to the bare Go field name. `SetEscapeHTML(false)` is
  deliberate — a plain `json.Marshal` rewrites `<`/`&` in markdown bodies to
  `<`/`&`. `start --json --help` prints usage text, not JSON — intended.
  Unmarshalling **collapses** null-vs-absent and absent-vs-zero, so the four load-bearing
  claims (`step`, `done`/`open`, `shortfalls`, `found`) are asserted on raw bytes, not
  decoded values — do not "clean up" those assertions into unmarshal-and-compare. (15)
- **This pipeline's own fix-mode re-runs of `finish` on an already-done step now hit
  SCENARIO-16's refusal** whenever the regenerated handoff or STATE.md body differs from
  what is recorded — previously a silent overwrite. The crossover note below stays true only
  for its literal identical-inputs claim. Resolution: read the recorded file and edit it
  directly, or re-run with the recorded body. (16)
- **A byte-snapshot alone can't prove a refusal happened before any write** — a refusal taken
  after a byte-identical rewrite would still pass `snapshotTree`. Pair it with the
  `pinnedModTime`/`pinModTimes`/`modTimes` probe in `finish_idempotent_test.go` for that
  claim; a `Contains` assertion on refusal text is also unsafe where the pre-refusal code
  printed similar text on the same inputs — assert `Equal` on the whole line/string. (16)
- **`cli/finish.go`'s placeholder swap only upgrades a `RefusalError.Path` it recognizes** —
  adding a new placeholder (17's `HandoffSource`) without a matching `case` renders the raw
  `<handoff>` literal; any future placeholder needs its own branch there too.

## Open debts

- Heading/cap value validation — unowned until SCENARIO-19.
- **This repository's own artifacts already exceed both length caps** — handoff files run
  29–373 lines against the 60-line default cap 17 now enforces, and `STATE.md` is over 200
  against the 80-line cap 18 will enforce. Harmless today (`brief` is not self-hosted —
  `brief start brief` still fails on missing frontmatter), but `brief finish brief <step>`
  will refuse on its own bodies once 18 lands. Owner: `check` (22), per R18. (17)
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
  read command (`start`), contradicting R14a's read-refusal rule. Pre-existing, unowned.
- **R13's output budget and truncation is unowned, and must never truncate a `--json`
  payload** — a line-boundary cut yields an unparseable document, so the eventual owner has
  to skip or refuse a structured payload rather than trim one. `cfg.DefaultOutputBudgetBytes`
  (8192) remains unconsumed. (15)

## Crossover note

An identical re-finish being a true no-op means the crossover's "mark SCENARIO-01 through 06
done" step is safely re-runnable **only when re-run with the same handoff/state bytes** — a
regenerated body now hits SCENARIO-16's refusal instead of silently overwriting (see Traps).
`SCENARIO-01.md`…`-12.md` and their `-HANDOFF.md` files still
carry no frontmatter — the crossover owns adding it, and **must write `id:` as well as
`status:`**: SCENARIO-13's check 7 (empty `id:` on the briefed step) means a crossover that
adds only `status:` leaves `brief start brief` refused for a new reason once the frontmatter
exists at all. Until then, checks 1-5 already pass on this repo's own tree, so SCENARIO-13
adds no new lockout — `brief start brief` still fails with `no frontmatter found`, unchanged
from before this scenario. `status` still prints `brief ! ! !` for it.
