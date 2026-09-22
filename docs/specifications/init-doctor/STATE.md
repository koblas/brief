# init-doctor — current state

Scenarios complete: SCENARIO-01..10 — every spec scenario shipped. Fix passes 1-3: git-
boundary root scoping (R3), partial-write/`files_changed` consistency, a `*RefusalError`
type-preservation gap. Fix pass 4: pflag placeholder bug; `uninstall`'s "removed" line
discriminates host-artifact-removed/config-only/edited-kept; non-regular CLAUDE.md candidate
is doctor WARN. Fix pass 5: host-snippet's WARN considers only `planSnippet`'s own chosen
candidate; `uninstall --dry-run` reads the computed plan; "N file(s) … kept" moved onto typed
`Artifact.ForceRemovable`. Fix pass 6: mutation-verified three fix-pass-5 arms; SKIP row Path
names the real first-present candidate; `ForceRemovable` false on every force-removed
artifact. Fix pass 7: an unreadable CLAUDE.md candidate is its own host-snippet WARN. Fix pass
8: host-snippet's fix renders relative to `wd`; an unreachable candidate is
`unreadable`/`present`, never absence. Fix pass 9: replaced the per-check absent-vs-unreadable
logic (patched twice, wrong twice) with one classifier, `doctor.classifyProbeError`, used by
both `probeIntegrationFile` and `scanSnippetCandidateStates`; `syscall.ENOTDIR` (a path
component that is a regular file) now reads as absent everywhere, fixing host-plugin/-hook/
-agents/-snippet all wrongly reporting "incomplete"/"not a regular file"/"missing" instead of
SKIP when `.claude` itself is a plain file; host-plugin/-hook/-agents gained the same
unreadable-vs-missing split host-snippet already had.

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
  package decides absent vs unreadable**, called by both `probeIntegrationFile` and
  `scanSnippetCandidateStates` for every failed `os.Lstat`/`os.ReadFile`: `fs.ErrNotExist` or
  `syscall.ENOTDIR` (darwin/linux only, devenv.nix's own targets; windows not exercised) is
  absent; anything else is unreadable, reason via `readFailureReason`. A ReadFile failure the
  classifier itself reads as absent folds into the plain-missing shape, no separate TOCTOU
  branch. Every host row renders WARN "not readable (`<reason>`)" for an unreadable subject,
  never "missing"/"not a regular file"; a row mixing unreadable and genuinely-missing files
  (`integrationFileRowDetail`) names both in separate fragments, reason/Fix from the first
  unreadable file (untested against a genuinely mixed cause — none observed in practice).
  `notReadableFix` chmods the immediate parent dir (`u+rx`) when the Lstat itself failed, or
  the file (`+r`) when only the ReadFile did, both then `run 'brief init --host claude-code'`,
  rendered `relPath(wd, ...)`, the same `wd` `Diagnose` was called with. host-agents still
  SKIPs when no agent file is present at all (unreadable still counts as present).
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

## Open debts
- setup never rewrites/removes an `OriginOlder` file — harmless while every older digest list
  ships empty; the first real one needs this, or doctor's fix can never clear its own WARN.
  **Unowned.**
- doctor's text rows are two-space-joined, not `tabwriter`-aligned — deferred twice now, ~12
  tests pin stdout byte-exact. **Unowned.**
- `hostPluginCheck`/`hostAgentsCheck` duplicate the same origin-check body — refactor-advisor
  MINORs, deferred rather than risk behavior change. **Unowned.**
- `host_test.go`/`detect_test.go`'s classification tables mostly carry no mutation-verification
  statement, unlike their siblings (fix passes 5-9 each added some that do). **Unowned.**
- `internal/doctor` (`scanSnippetCandidateStates`/`hostSnippetCheck`) and `internal/setup`
  (`scanSnippetCandidates`/`chooseSnippetLocation`) each hand-write the same snippet-candidate-
  selection rule and cannot import each other (dependency rule) — arch-reviewer suggests
  hoisting the shared predicate into `internal/platform/artifact`, which both already import.
  **Unowned.**
- `internal/setup`'s own `scanSnippetCandidates` (snippet.go) and both probes in uninstall.go
  check only `os.IsNotExist`, never `syscall.ENOTDIR`, unlike `planPluginFile`/`checkWritable`
  in the same package — a `.claude` that is a regular file makes `brief init`/`uninstall` hard-
  refuse ("setup: lstat …: not a directory") instead of treating it as absent, the same shape
  fix pass 9 just fixed in `internal/doctor`. Checked, not fixed — out of scope for this pass.
  **Unowned.**
- `init`'s own refusal on an unreadable CLAUDE.md leaks a Go wrap chain and names the path
  twice — `brief init: setup: read /abs/CLAUDE.md: open /abs/CLAUDE.md: permission denied`;
  contract-conformant (a refusal, correct exit code) so not blocking, but product-vision's
  suggested copy if anyone touches it is `brief init: cannot read CLAUDE.md: permission
  denied; make it readable, or run 'brief init --host none'`. **Unowned.**
