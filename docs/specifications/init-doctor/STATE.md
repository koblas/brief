# init-doctor — current state

Scenarios complete: SCENARIO-01..10 — every spec scenario shipped. Fix passes 1-9: git-
boundary root scoping (R3), partial-write/`files_changed` consistency, a `*RefusalError`
type-preservation gap, a pflag placeholder bug, `uninstall`'s "removed"-line discrimination,
host-snippet WARN precedence, typed `Artifact.ForceRemovable`, wd-relative fixes, and one
shared `doctor.classifyProbeError` absent-vs-unreadable classifier for all four host checks
(`syscall.ENOTDIR` reads as absent everywhere) — but pass 9 over-corrected host-plugin's own
unreadable arm to WARN. Fix pass 10: restored host-plugin unreadable to ERROR; deleted an
unreachable "reclassify as absent" arm from both probes' own `ReadFile`-failure handling;
renamed `integrationFileState.stat` to `statFailed`; split a maintidx-flagged test; added
wd≠root, mixed-row and ReadFile-arm coverage for host-plugin/-hook/-agents.

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
  `chooseSnippetLocation` would itself pick — the first candidate present at all, in
  `host.InstructionFiles` priority order.
- **`doctor.classifyProbeError(err) (absent bool, reason string)` is the one place this
  package decides absent vs unreadable on a failed `os.Lstat`**, called by both
  `probeIntegrationFile` and `scanSnippetCandidateStates`: `fs.ErrNotExist` or
  `syscall.ENOTDIR` (darwin/linux only, devenv.nix's own targets; windows not exercised) is
  absent; anything else is unreadable, reason via `readFailureReason`. An `os.ReadFile`
  failure against a path `Lstat` itself just resolved as regular is always unreadable —
  typically the file's own mode denying read — never reclassified as absent, since only a
  race between the two calls could make that reclassification correct.
  `integrationFileState`'s field is `statFailed` (renamed from `stat`), true only when the
  `Lstat` call itself failed. Every unreadable subject renders `"not readable (<reason>)"`,
  never "missing"/"not a regular file"; severity is **ERROR** for host-plugin (unreadable is
  as fatal as missing — Claude Code cannot load the skill either way), **WARN** for
  host-hook/host-agents/host-snippet. A row mixing unreadable and genuinely-missing files
  (`integrationFileRowDetail`) names both, in separate fragments, reason/Fix from the first
  unreadable file. `notReadableFix` chmods the immediate parent dir (`u+rx`) when the `Lstat`
  itself failed, or the file (`+r`) when only `ReadFile` did, both then
  `run 'brief init --host claude-code'`, rendered `relPath(wd, ...)` against `absWd` (never
  `root`) for every host check. host-agents still SKIPs when no agent file is present at all
  (unreadable still counts as present).
- `setup.Artifact.ForceRemovable` is true exactly on an Uninstall-side `ActionKept` artifact
  `--force` would turn into `ActionRemoved` — false on every other Action, including that same
  file once force has already removed it, and on every Init-side artifact.
  `cli.uninstallNextAction` switches on it, never on Detail text.
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
  trailing usage text to stay unique.
- `os.Lstat` on a file-as-directory returns `syscall.ENOTDIR`, never matched by
  `os.IsNotExist`/`fs.ErrNotExist` directly — `doctor.classifyProbeError` and
  `internal/setup`'s own `planPluginFile`/`checkWritable` check it explicitly; other
  `internal/setup` scans do not (see Open debts).
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
  negative `wantPathNotSuffix` when a case must pin one candidate specifically.
- `Diagnose` passes `absWd`, never `root`, as `wd` into all four host checks; every
  `hostCheckCase` table runs with `wd == root`, so passing `root` instead reads identically
  there — only a subdirectory run catches it
  (`Test_diagnose_host_plugin_hook_agents_unreadable_fix_is_relative_to_wd`).
- A CLI doctor test that fakes PATH lookup with `lookPathNotFound` cannot see a host-row
  severity change: env-path's own ERROR already forces exit 1 on its own. A test asserting a
  host row's severity at the CLI boundary needs brief FOUND on PATH instead
  (`Test_doctor_reports_host_plugin_error_when_the_plugin_directory_is_unreadable`).
- Under an unsearchable `.claude`, host-plugin reads ERROR and names three plugin files even
  when none was ever installed — "unreadable counts as present" has no floor. Pinned by
  `Test_doctor_env_path_stays_error_when_the_host_snippet_directory_is_unreadable`, whose own
  fixture only ever wrote a CLAUDE.md snippet. Flagged for product-vision's final pass, not
  fixed here.
- In `hostPluginCheck`/`hostAgentsCheck`, an all-unreadable subject-file set has no default
  ERROR/WARN fallback: `missingRelPaths` skips every `unreadable` state, and
  `relPathsWithOrigin` only counts `present && regular` ones, so both come back empty.
  `integrationFileRowDetail`'s own branch is the *only* thing standing between that and the
  final `return … SeverityOK, "installed"` — delete it (or its call) and the row goes healthy,
  not merely wrong-severity. Five tests guard this arm today; keep it ahead of both other
  branches.

## Open debts
- setup never rewrites/removes an `OriginOlder` file — harmless while every older digest list
  ships empty; the first real one needs this, or doctor's fix can never clear its own WARN.
  **Unowned.**
- doctor's text rows are two-space-joined, not `tabwriter`-aligned — deferred twice now, ~12
  tests pin stdout byte-exact. **Unowned.**
- `hostPluginCheck`/`hostAgentsCheck` duplicate the same origin-check body — refactor-advisor
  MINORs, deferred rather than risk behavior change. **Unowned.**
- `internal/setup`'s `detect_test.go` classification table carries no mutation-verification
  statement at all, and in `host_test.go`, `Test_diagnose_classifies_host_hook` has only one
  (fix pass 10's own addition) and `Test_diagnose_classifies_roles` has none — every other
  table in `host_test.go` has at least one (fix passes 5-10 each added some). **Unowned.**
- `internal/doctor` (`scanSnippetCandidateStates`/`hostSnippetCheck`) and `internal/setup`
  (`scanSnippetCandidates`/`chooseSnippetLocation`) each hand-write the same snippet-candidate-
  selection rule and cannot import each other (dependency rule) — arch-reviewer suggests
  hoisting the shared predicate into `internal/platform/artifact`, which both already import.
  **Unowned.**
- `internal/setup`'s own `scanSnippetCandidates` (snippet.go) and both probes in uninstall.go
  check only `os.IsNotExist`, never `syscall.ENOTDIR` — a `.claude` that is a regular file
  makes `brief init`/`uninstall` hard-refuse instead of treating it as absent, the same shape
  fix pass 9 fixed in `internal/doctor`. Checked, not fixed. **Unowned.**
- `init`'s own refusal on an unreadable CLAUDE.md leaks a Go wrap chain and names the path
  twice; contract-conformant (a refusal, correct exit code) so not blocking, but
  product-vision's suggested copy if anyone touches it is `brief init: cannot read CLAUDE.md:
  permission denied; make it readable, or run 'brief init --host none'`. **Unowned.**
- `notReadableFix`'s `statFailed` arm names the immediate parent dir of the unreadable subject
  file; the actual unsearchable directory is usually a higher ancestor (e.g. `chmod 000
  .claude` → the fix says `chmod u+rx .claude/skills/brief/.claude-plugin`, which fails —
  `chmod u+rx .claude` is the fix that actually works). The pinned strings in host_test.go
  encode today's behavior; a real fix needs an ancestor walk, retrying `Lstat` upward until one
  resolves. **Unowned.**
- `rolesCheck`'s own `fileIsRegular` (host.go) treats any `os.Stat` error, including EACCES,
  as "not found", bypassing `classifyProbeError` entirely — under `chmod 000 .claude` the
  `roles` row says an agent is not found and suggests `brief init --with-agents`, rather than
  reporting it unreadable. **Unowned.**
- `host_test.go` is still ~1150 lines across host-plugin, host-hook, host-agents, host-snippet
  ×2 and roles — a further split by check, not just by unreadable-vs-not, is deferred.
  **Unowned.**
