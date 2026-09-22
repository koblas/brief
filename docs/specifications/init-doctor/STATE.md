# init-doctor — current state

Scenarios complete: SCENARIO-01..09. Last updated by SCENARIO-09.

## Binding decisions
- `config.Locate` is the one walk-up; `config.Inspect` the one all-violations decoder;
  `Resolve` = `Locate` + first `Inspect` element. `doctor`, `init`, `uninstall`, `check --hook`
  and the plugin's own install root build on this; root = `Locate`'s dir, else `wd` (S01-07)
- `internal/doctor` and `internal/setup` import only `internal/platform/*` + stdlib, never each
  other or `scaffold`; neither has a `Store` port. `probeWritable` (S09) is a deliberate copy in
  each, not shared sideways (S02, 03, 06, 07, 09)
- `internal/platform/artifact` renders + digests every brief-written file; CLAUDE.md is the one
  parameterized artifact (`SnippetBlock(dir)`/`RecognizeSnippet`), matches a block written for
  *any* dir. `host.Host` = `{Name, HookPath, WriteHookContext, Plugin(withHook bool) []File,
  Agents() []File, InstructionFiles() []string}`; `host` imports `artifact`, never reverse
  (S03, 05, 06, 07, 08)
- `init`/`uninstall` plan-then-apply, every refusal decided before the first write/removal.
  Init apply order: feature root, plugin files, agent files (`WithAgents` only), CLAUDE.md,
  config last. **S09** inserts `checkWritable` between planning and apply, real runs only
  (`!DryRun && !Print`): a target whose nearest existing ancestor is not a directory, or is one
  that can't be written to, refuses `ErrUnwritable` before the feature root is created,
  returning `Result` (Artifacts + Print) alongside the error — the only Init error path that
  does. `--force` rewrites `.brief.yaml` only; Uninstall's own `--force` removes an edited
  CLAUDE.md/plugin/agent file (S03, 06, 07, 08, 09)
- CLAUDE.md location: a candidate already holding a recognized block wins; else first existing
  candidate; else root is created. `mergeSnippet`/`removeSnippet` are exact inverses via the
  append separator — byte-identical round trip, no sidecar (S07)
- `setup.Kind` = config | feature-root | plugin | hook | agent | snippet. `artifact.KindConfig`
  recognizes two compiled-in bodies, `ConfigFile()`/`ConfigFileWithRoles()`; `--force`'s
  "unchanged" check is an exact-bytes match against the *desired* variant, never Recognize's
  "any known render" (S08, 09)
- Role bindings come only from `artifact.AgentBindings()`, written only into a config `Init`
  creates or `--force`-rewrites in the same call; `RolesToAdd` lists only *unbound* roles (R15).
  `--with-agents` requires the *resolved* host `claude-code` (`ErrAgentsNeedHost`), checked
  after detection resolves `""` — a bare `--with-agents` follows whatever the tree detects.
  Plugin agents (code.claude.com/docs/en/sub-agents): flat `agents/<role>.md`, addressed
  `brief:<role>`; a project `.claude/agents/<role>.md` of the same name overrides it — S10's
  `roles` row must not assume `brief:<role>` runs (S08, 09)
- `check --hook <host>`: ERROR findings → exit 0, `additionalContext` on stdout; config located
  from injected `wd`, never the payload's `cwd` (S05)
- **S09 detection**: `InitRequest.Host == ""` means detect (`detectHost`, after `config.Locate`,
  before `ErrAgentsNeedHost`): root `.claude` dir, root `CLAUDE.md` entry (any type), or
  `WithHomeDir`'s home's own `.claude` dir → `HostClaudeCode`; else `HostNone`,
  `Result.NoHostDetected` true — the only way cli tells detected-none from explicit
  `--host none`. `InitRequest.Print`/`Result.Print []PrintArtifact` mirror `DryRun` (nothing
  written); `Print` = pending artifacts in `Result.Artifacts` order, feature root excluded,
  snippet body always `SnippetBlock(dir)`. cli's `run`/`newRootCommand` take `...runSeam`
  (`withDoctorOpts`, `withSetupOpts`); a bare-`init` test must go through `run` with an injected
  home (`init_internal_test.go`) — `cli.Run` reads the developer's real `~/.claude`. stderr
  priority: `--dry-run` > `--print` > R8 no-host-detected > ordinary next action. Unknown-host
  copy now points at `run 'brief init --print'`. R10 under `--json`: standard error document
  only, no `artifacts`, fix `run 'brief init --print --json' and apply the artifacts by hand`

## Left unbuilt
- Line number for a value error, heading-shape rules — unowned (S01)
- `artifact.OriginOlder`, doctor rows `host-plugin`/`host-hook`/`host-snippet`/`host-agents`/
  `roles`, env-path's ERROR arm — S10
- Detection exists only inside `setup.Init` — `uninstall`/`doctor` have none; S10's host rows
  must not assume it
- Feature root in `--print` output, `host` field in `--print --json` — never printed/emitted
- Dry-run writability prediction — `--dry-run` cannot report R10 (the probe writes for real)
- Marker detection inside fenced code blocks — never handled; any exact marker line counts
- Removal of `.claude/skills/`/`.claude/` created by init — never; brief owns only
  `host.PluginDir` + the CLAUDE.md candidates `InstructionFiles` names

## Traps
- `t.TempDir` has no `.git` above it; `chmod` tests pass vacuously as root — skip under
  `os.Geteuid() == 0` (S02, 04, 06, 09)
- `--force init` over a non-default `feature-directory` leaves two feature roots, never removed
- A round-trip "byte-identical", or "nothing written", assertion passes vacuously if init wrote
  nothing — assert the control run differs first (S04, 06, 07, 08, 09)
- A claude-code Init always creates `.claude/skills/brief/...`; Uninstall prunes empty dirs only
  down to and including `host.PluginDir` — `.claude/skills/`/`.claude/` always survive, empty
- pflag's `UnquoteUsage` recognizes one backtick span per usage string; a continuation line must
  never start with `--<word>`. A leaf's terse Usage line has an 80-column budget: `init` omits
  `[--force]` and `[--json]` from it — both stay registered and documented (S06, 08, 09)
- `~/.claude` exists for nearly every Claude Code user, so detection effectively always resolves
  `claude-code`; the no-host-detected branch is testable only with an injected empty home
  (`init_internal_test.go`) — flag for the final `product-vision` pass (S09)
- `os.Lstat` under a file-as-directory returns `ENOTDIR`, which `os.IsNotExist` never matches —
  both `planPluginFile` and `checkWritable`'s ancestor walk must check it explicitly, or
  planning/the writability check fails with a generic error before R10 can refuse (S09)
- A directory *at* a target path is `kept (not a regular file)`, exit 0 — not R10, which only
  ever refuses on an *ancestor* (S09)

## Open debts
None open — S08 closed `roles.*`/`optional-conventions` validation (R1 amended); unbound roles
are `doctor`'s (S10) to report. S09 closed `--print`, host detection and R10 — nothing deferred.
