# init-doctor — current state

Scenarios complete: SCENARIO-01..08. Last updated by SCENARIO-08.

## Binding decisions
- `config.Locate` is the one walk-up; `config.Inspect` the one all-violations decoder;
  `Resolve` = `Locate` + first `Inspect` element. `doctor`, `init`, `uninstall`, `check --hook`
  and the plugin's own install root all build on this. Root = `Locate`'s dir, else `wd`;
  `init`'s next action names root (`Result.Root`) when it differs from `wd` (S01-07)
- `internal/doctor` and `internal/setup` import only `internal/platform/*` + stdlib, never
  each other or `scaffold`; neither has a `Store` port (S02, 03, 06, 07)
- `internal/platform/artifact` renders + digests every brief-written file. CLAUDE.md is the
  one parameterized artifact (`SnippetBlock(dir)`/`RecognizeSnippet`, off `Render`/`Recognize`
  /`digestsFor`); matches a block written for *any* dir, reports which one (S03, 06, 07)
- `internal/platform/host.Host` = `{Name, HookPath, WriteHookContext, Plugin(withHook bool)
  []File, Agents() []File, InstructionFiles() []string}`. `Agents()` (S08) is separate from
  `Plugin`: `Init` only plans it under `InitRequest.WithAgents`, but `Uninstall` always plans
  agent removal — `UninstallRequest` carries no `WithAgents` of its own. `host` imports
  `artifact`, never reverse (S05, 06, 07, 08)
- `init`/`uninstall` plan-then-apply, every refusal decided before the first write/removal.
  Init apply order: feature root, plugin files, agent files (`WithAgents` only), CLAUDE.md
  block, config last. Uninstall: block first, agent files reversed (reviewer, implementer,
  planner), plugin files reversed, config last. `--force` rewrites `.brief.yaml` (the bound
  variant under `WithAgents`) and removes an edited CLAUDE.md/plugin/agent file under
  Uninstall — Init never rewrites an edited artifact otherwise (S03, 06, 07, 08)
- CLAUDE.md location: a candidate already holding a recognized block wins; else first
  existing candidate; else root is created. Two full candidates refuses naming
  `.claude/CLAUDE.md`; CRLF refuses only the resolved candidate. `mergeSnippet`/
  `removeSnippet` are exact inverses via the append separator — byte-identical round trip, no
  sidecar; an emptied CLAUDE.md is deleted on Uninstall, pre-existing-empty included (S07)
- `setup.Kind` = config | feature-root | plugin | hook | agent | snippet; `Action` includes
  `ActionMerged`; `Result.Modified` populated by merges/strips; `Result.RolesToAdd []string`
  (S08) is Init's own hint, never nil, populated even under `--dry-run`
- `artifact.KindConfig` recognizes **two** compiled-in bodies: `ConfigFile()` (plain) and
  `ConfigFileWithRoles()` (roles live, S08) — `Render(KindConfig)` still returns the plain
  one; `configDigests` holds both. `planConfig`'s `ActionUnchanged` branch always decodes via
  `decodeCurrentConfig` rather than assuming `config.Default()`. `--force`'s "unchanged" check
  (`configFileCurrent`) is an exact-bytes match against the *desired* variant, never
  `Recognize`'s "any known render" — S09's `--print` must emit whichever variant a real run
  would write
- Role bindings come only from `artifact.AgentBindings()` (`brief:planner`,
  `brief:implementer`, `brief:reviewer`); written only into a config `Init` creates or
  `--force`-rewrites in the same call — an existing/kept config is never edited. `RolesToAdd`
  lists only *unbound* roles (R15: never shadows an existing binding); doctor's `roles` row
  (S10) uses the same unbound vocabulary. `--with-agents` requires the *resolved* host to be
  `claude-code` (`setup.ErrAgentsNeedHost`, mapped to a usage error like `ErrUnknownHost`) —
  S09's host detection changes the outcome of a bare `init --with-agents`
- Agent frontmatter (verified, code.claude.com/docs/en/sub-agents): plugin agents are flat
  `agents/<role>.md` files, addressed `brief:<role>`; `name`/`description` required, `tools`
  comma-separated optional, no `model` key. A project `.claude/agents/<role>.md` of the same
  name overrides the plugin agent — S10's `roles` row must not assume `brief:<role>` runs
- `check --hook <host>`: ERROR findings → exit 0, `additionalContext` on stdout; config
  located from injected `wd`, never the payload's `cwd` (S05)

## Left unbuilt
- Line number for a value error, heading-shape rules — unowned (S01)
- `--print`, host detection, R10 writability pre-check — S09; `--print` must emit
  `ConfigFileWithRoles()` when a run would write the bound variant
- `artifact.OriginOlder` (all kinds, agents included), doctor rows `host-plugin`/`host-hook`/
  `host-snippet`/`host-agents`/`roles`, env-path's ERROR arm — S10
- Marker detection inside fenced code blocks — never handled; any exact marker line counts
- Removal of `.claude/skills/`/`.claude/` created by init — never; brief owns only
  `host.PluginDir` + the CLAUDE.md candidates `InstructionFiles` names

## Traps
- `t.TempDir` has no `.git` above it; `chmod` tests pass vacuously as root — skip under
  `os.Geteuid() == 0` (S02, 04, 06)
- `--force init` over a non-default `feature-directory` leaves two feature roots, never
  removed (S03, 04)
- `t.TempDir()` on macOS resolves through a symlink — lexical match first, `EvalSymlinks`
  fallback (S05)
- A round-trip "byte-identical" assertion passes if init wrote nothing — assert the post-init
  snapshot differs first (S04, 06, 07, 08)
- A claude-code Init always creates `.claude/skills/brief/...`; Uninstall prunes empty dirs
  only down to and including `host.PluginDir` (`agents/` included since S08) —
  `.claude/skills/`/`.claude/` always survive, empty (S06, 07, 08)
- pflag's `UnquoteUsage` recognizes only one backtick-quoted span per usage string; a flag
  usage string's own continuation line must never start with `--<word>` — a text-table
  scraper (this repo's own help-JSON agreement test) misreads it as a second flag row. A
  leaf's terse Usage line has an 80-column budget too: `init`'s own omits `[--json]` (matches
  `check`'s/`new`'s precedent) rather than wrap — the flag stays registered and documented
  (S06, 08)

## Open debts
None open — S08 closed `roles.*`/`optional-conventions` validation by declaring both
free-form and unvalidated at load (R1 amended); unbound roles are `doctor`'s (S10) to report.
