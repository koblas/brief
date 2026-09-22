# init-doctor — current state

Scenarios complete: SCENARIO-01..10 — every spec scenario shipped. Fix passes 1-3: git-
boundary root scoping (R3), partial-write/`files_changed` consistency, a `*RefusalError`
type-preservation gap. Fix pass 4: pflag placeholder bug on `--host`/`--hook`; `uninstall`'s
"removed" line discriminates host-artifact-removed vs config-only vs edited-kept; non-regular
CLAUDE.md candidate is doctor WARN (was SKIP); `init` names host-detection's own signal
(`Result.DetectedBy`). Fix pass 5: host-snippet's WARN considers only the one candidate
`planSnippet` would choose; `uninstall --dry-run` discriminates by the computed plan like a
real run; the "N file(s) … kept" count moved onto typed `Artifact.ForceRemovable`. Fix pass
6: three untested fix-pass-5 arms pinned and mutation-verified (`notRegularDetail`'s
empty-kind fold, `uninstall --dry-run`'s edited-kept and plain "nothing removed" arms);
host-snippet's SKIP row Path now names the real first-present candidate, not a hardcoded
root path; the pflag-placeholder test's control values now carry enough trailing usage text
to stay unique; `ForceRemovable` is false on every force-removed artifact across all three
producers, rule stated on `Artifact`'s own doc comment. Fix pass 7: an existing-but-unreadable
CLAUDE.md candidate is its own host-snippet WARN, distinct from "not installed" (an
unverifiable absence claim) and from the not-a-regular-file WARN; the reviewer gate's PASS
WITH CHANGES closed on this alone, plus three cheap MINOR folds. Fix pass 8: host-snippet's
own "not readable" WARN fix is rendered relative to `wd`, not the install root, matching the
row's own Path (M1); a stat failure other than "not found" — an Lstat that cannot even reach
the candidate, not just a `ReadFile` on one it already reached — is `unreadable`/`present`,
never folded into absence, in both `scanSnippetCandidateStates` and `probeIntegrationFile`
(M2), so an inaccessible `.claude/` can no longer downgrade `env-path` from ERROR to WARN or
flip doctor's own exit code from 1 to 0.

## Binding decisions
- `config.LocateWithin(dir, boundary)` bounds a walk at `boundary`, itself still checked, its
  parent never; `LocateInRepo` bounds it to `repo.Root(wd)`, called by `init`/`uninstall`/
  `doctor`/`check --hook`; a config above that boundary is treated as absent. Refuses as
  `*InvalidConfigError` (`errors.Is(err, ErrInvalidConfig)` holds too).
- `internal/doctor`/`internal/setup` import only `internal/platform/*`, never each other or
  `scaffold`. `writable.Probe` is the one write-probe both share.
- `internal/platform/artifact` renders + digests every brief-written file; every
  `older…Digests` list ships empty, so `OriginOlder` is reachable only white-box.
- `init`/`uninstall` plan-then-apply, every refusal decided first; apply order: feature root,
  plugin/agent files, CLAUDE.md, config last. A non-regular CLAUDE.md candidate is kept, never
  followed/written; `setup`'s install-side Detail and doctor's WARN both point at `--print`;
  `planSnippetRemoval`'s own removal-side Detail is the plain "not a regular file" — the two
  wordings differ freely since `ForceRemovable`, not Detail text, is what callers key on.
  Doctor's host-snippet WARN/SKIP fire only for the one candidate `planSnippet`/
  `chooseSnippetLocation` would itself pick — the first candidate present at all, regular or
  not, readable or not, in `host.InstructionFiles` priority order, never a farther
  candidate's own shape; SKIP's own Path names that same first-present candidate (falls back
  to the first candidate in priority order only when none is present at all). A candidate
  `os.Lstat` cannot even reach, or a present, regular one `os.ReadFile` cannot read
  (`snippetCandidateState.unreadable`, distinct from `present=false`; `os.IsNotExist` on
  either call is still the plain absent shape) is its own WARN — "not readable (`<reason>`);
  cannot check for brief block", `<reason>` from `readFailureReason` (a wrapped
  `*fs.PathError`'s own inner error, always non-nil since `os.Lstat`/`os.ReadFile` only ever
  fail with one) — never the "not installed" a genuinely missing candidate gets, since a stat
  or read failure other than "not found" proves nothing about absence; the block-wins
  carve-out still overrides it exactly like `notRegular`. Its Fix (`notReadableFix`) is
  rendered `relPath(wd, ...)` — the same `wd` `Diagnose` was called with, never `root` — since
  the row's own Path is later rendered relative to `wd` too (`cli.doctorRow`'s `displayPath`);
  `probeIntegrationFile` (host-plugin/-hook/-agents, and `anyIntegrationFilePresent`'s own
  "installed" gate) makes the identical absent-vs-unreadable call on the same `os.Lstat`.
- `setup.Artifact.ForceRemovable` is true exactly on an Uninstall-side `ActionKept` artifact
  `--force` would turn into `ActionRemoved` (an edited file: `planPluginRemoval`,
  `planConfigRemoval`, `planSnippetRemoval`'s own non-`OriginCurrent` arm) — false on every
  other Action, including that same file once force has already removed it, and on every
  Init-side artifact. `cli.uninstallNextAction` switches on it, never on Detail text.
- Role bindings come only from `artifact.AgentBindings()`, written only into a config `Init`
  creates/`--force`-rewrites; `roles` resolves `brief:<name>` via plugin-or-project, bare
  `<name>` via project-or-injected-home; WARN never ERROR.
- `check --hook`'s opt-in gate runs before stdin is parsed; a malformed payload inside an
  opted-in repo is exit 1, one stderr line, never usage-error's exit 2.
- Host detection (`setup` only): `Host == ""` → root `.claude`/`CLAUDE.md`, else
  `WithHomeDir`'s `.claude` → `HostClaudeCode`; else `HostNone`. `detectHost`'s third return
  (`Result.DetectedBy`) is `""` when `Host` was explicit; `initNextAction`'s "(detected …)"
  and `--json`'s `detected_by` gate on it non-empty.
- `doctor` appends `host-plugin, host-hook, host-snippet, host-agents, roles` after `env-path`;
  `host.originRow` decides every row's WARN-older/OK-edited/OK-current triple.
- A partial write returns the populated `Result`, wrapped `ErrPartialWrite`; cli prints landed
  rows before the refusal; `--json` drops the contradicting "(no files changed)" tail.
- `uninstallNextAction` discriminates by what the plan holds — dry run or real read the plan
  the same way, only the phrasing (promise vs report) differs: a non-config `ActionRemoved`
  names the host install; a lone config removal names "brief's config"; any `ForceRemovable`
  `ActionKept` counts toward "N file(s) … kept"; zero artifacts is "nothing installed";
  anything else is "nothing removed".

## Left unbuilt
- Line number for a value error, heading-shape rules; dry-run writability prediction (R10)
- Marker detection inside fenced code blocks; removal of `.claude/skills/`/`.claude/`
- A stray-plugin row (`wd ≠ root`); agent resolution by frontmatter `name:`;
  `~/.claude/skills/brief` (user-scope plugin) — none consulted
- A CLAUDE.md symlink to a recognized alternate reads WARN/kept like any other symlink —
  never resolved to check its target. **Unowned.**
- `cli.uninstallNextAction` (~65 lines) is still one function — refactor-advisor MINOR,
  extracting a `classifyUninstallResult` helper, deferred as fix-if-cheap. **Unowned.**

## Traps
- A fixture's own `.git` sets `LocateInRepo`'s boundary at `wd` — move it up to exercise a
  farther ancestor config.
- `--force init` over a non-default `feature-directory` leaves two feature roots, never removed.
- A round-trip "nothing written" assertion passes vacuously if init wrote nothing — assert a
  control run differs first.
- pflag's `UnquoteUsage` keeps the backticked word in the rendered text too — backtick a real
  value and it becomes the table placeholder; backtick a generic word instead. Pinned by
  `Test_host_and_hook_flags_render_a_generic_table_placeholder` (help_test.go), mutation-
  verified per flag; a control asserting the reverted placeholder is absent needs enough
  trailing usage text to stay unique — a bare "--host none" also occurs, unrelated to this
  bug, in `--no-hook`'s own "(no effect with --host none)" parenthetical.
- `os.Lstat` on a file-as-directory returns `ENOTDIR`, never `os.IsNotExist` — checked
  explicitly in `planPluginFile`/`checkWritable`.
- `newHealthyDoctorFixture` and the cli doctor goldens pin the full 12-row order/content; a
  doctor test leaving `WithHomeDir` unset reads the developer's real `~/.claude/agents`.
- A leaf's Usage line and root `cmdRow` share one `cmd.Use` — the 80-column help test exempts
  it by position, never by a "  brief " prefix match, which also caught unrelated wrapped
  lines.
- `setup.Artifact` has install-side and removal-side construction sites sharing one struct; a
  literal `setup.Artifact{...}` comparison must include `ForceRemovable` whenever the artifact
  came from `srv.Uninstall`, or it silently expects false.
- A `hostCheckCase.wantPathSuffix` shorter than the full disambiguating suffix (e.g. bare
  "CLAUDE.md") can match either candidate's own path — use the full relative suffix, or the
  negative `wantPathNotSuffix` when a case must pin the root candidate specifically.
- A `Check.Fix`'s own shell command must be rendered `relPath(wd, ...)`, the same `wd`
  `Diagnose` was called with — `checkRootDir` already did; `hostSnippetCheck`'s own
  `notReadableFix` did not until fix pass 8, and every fixture in `host_test.go` sets `wd ==
  root`, so the bug was invisible to every existing case there; only a fixture calling
  `Diagnose` from a subdirectory (`root-dir`'s own coverage never does either) catches it.

## Open debts
- setup never rewrites/removes an `OriginOlder` file — harmless while every older digest list
  ships empty; the first real one needs this, or doctor's fix can never clear its own WARN.
  **Unowned.**
- doctor's text rows are two-space-joined, not `tabwriter`-aligned — deferred twice now, ~12
  tests pin stdout byte-exact. **Unowned.**
- `hostPluginCheck`/`hostAgentsCheck` duplicate the same origin-check body — refactor-advisor
  MINORs, deferred rather than risk behavior change. **Unowned.**
- `host_test.go`/`detect_test.go`'s classification tables mostly carry no mutation-verification
  statement, unlike their siblings (fix passes 5, 6, 7 and 8 each added a couple that do). **Unowned.**
- `internal/doctor` (`scanSnippetCandidateStates`/`hostSnippetCheck`) and `internal/setup`
  (`scanSnippetCandidates`/`chooseSnippetLocation`) each hand-write the same snippet-candidate-
  selection rule and cannot import each other (dependency rule) — arch-reviewer suggests
  hoisting the shared predicate into `internal/platform/artifact`, which both already import.
  **Unowned.**
- `init`'s own refusal on an unreadable CLAUDE.md leaks a Go wrap chain and names the path
  twice — `brief init: setup: read /abs/CLAUDE.md: open /abs/CLAUDE.md: permission denied`;
  contract-conformant (a refusal, correct exit code) so not blocking, but product-vision's
  suggested copy if anyone touches it is `brief init: cannot read CLAUDE.md: permission
  denied; make it readable, or run 'brief init --host none'`. **Unowned.**
