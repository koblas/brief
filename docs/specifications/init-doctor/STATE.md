# init-doctor — current state

Scenarios complete: SCENARIO-01..10 — every spec scenario shipped. Last updated by SCENARIO-10.

## Binding decisions
- `config.Locate` is the one walk-up; `config.Inspect` the one all-violations decoder;
  `Resolve` = `Locate` + first `Inspect` element. `doctor`, `init`, `uninstall`, `check --hook`
  and the host-integration root all build on this: root = `Locate`'s dir whenever a config was
  found (parseable or not), else `wd` (S01-07, S10)
- `internal/doctor` and `internal/setup` import only `internal/platform/*` + stdlib, never each
  other or `scaffold`; neither has a `Store` port. `probeWritable` is a deliberate copy in each,
  not shared sideways (S02, 03, 06, 07, 09)
- `internal/platform/artifact` renders + digests every brief-written file and owns
  `ScanSnippetMarkers`, the one marker scanner setup and doctor both call, plus `Origin`
  classification (`OriginCurrent`/`OriginOlder`/`OriginEdited` via `classify`/`digestsFor`, each
  Kind split into a current and an older digest list). Every `older…Digests`/
  `olderSnippetTemplates` list ships empty, so `OriginOlder` is reachable only white-box:
  `host.originRow`, `artifact.classify`, `artifact.recognizeSnippetWith`. CLAUDE.md is the one
  parameterized artifact (`SnippetBlock(dir)`/`RecognizeSnippet`), matches a block for *any*
  dir — `extractSnippetDir` derives each template's own prefix/suffix via a sentinel render, not
  a shared constant, so a genuine second template stays recognizable. `host.Host` imports
  `artifact`, never reverse (S03, 05, 06-10)
- `init`/`uninstall` plan-then-apply, every refusal decided before the first write/removal.
  Apply order: feature root, plugin files, agent files (`WithAgents` only), CLAUDE.md, config
  last. `checkWritable` runs between planning and apply, real runs only (`!DryRun && !Print`);
  `--force` rewrites `.brief.yaml` only; Uninstall's own `--force` removes an edited
  CLAUDE.md/plugin/agent file (S03, 06-09)
- CLAUDE.md location: a candidate already holding a recognized block wins; else first existing
  candidate; else root is created. `mergeSnippet`/`removeSnippet` are exact inverses via the
  append separator — byte-identical round trip, no sidecar (S07)
- Role bindings come only from `artifact.AgentBindings()`, written only into a config `Init`
  creates or `--force`-rewrites in the same call. Plugin agents: flat `agents/<role>.md`,
  addressed `brief:<role>`; a project `.claude/agents/<role>.md` of the same name overrides it.
  `doctor`'s `roles` row (S10) resolves `brief:<name>` via the plugin file **or** the project
  override, a bare `<name>` via the project **or** an injected home (`doctor.WithHomeDir`), and
  any other `<plugin>:<name>` as bound-but-unverified; WARN never ERROR — roles are reported,
  not enforced (R1 amendment) (S08, 09, 10)
- `check --hook <host>`: ERROR findings → exit 0, `additionalContext` on stdout; config located
  from injected `wd`, never the payload's `cwd` (S05)
- Host detection (`detectHost`, `setup` only — `doctor` never detects, always `host.ClaudeCode`):
  `InitRequest.Host == ""` → root `.claude` dir, root `CLAUDE.md` entry, or `WithHomeDir`'s
  home's own `.claude` dir → `HostClaudeCode`; else `HostNone`, `Result.NoHostDetected` true.
  `--print`/`--dry-run`, stderr priority and `--json` R10 shape unchanged since S09 (S09)
- **S10**: `doctor` appends `host-plugin, host-hook, host-snippet, host-agents, roles` after
  `env-path`, always in that order (R13's id list now closed). Two "installed" predicates:
  `filesInstalled` (any `Plugin(true)` ∪ `Agents()` file present) gates host-plugin/host-hook's
  SKIP; the broader `integrationInstalled` (`filesInstalled` **or** a snippet block found) gates
  only env-path's new ERROR arm. "Not installed" is always SKIP with the install command as Fix;
  a could-not-run SKIP (root-dir, roles behind an unparseable config) carries a nil Fix like
  every OK row. `host.originRow(origin, olderFix, suffix)` decides every row's
  WARN-older/OK-edited/OK-current triple. host-agents' own fix is always `--with-agents`, even
  for "older" — a plain `brief init` never calls `planAgentFiles`

## Left unbuilt
- Line number for a value error, heading-shape rules — unowned (S01)
- Feature root in `--print` output, `host` field in `--print --json` — never printed/emitted
- Dry-run writability prediction — `--dry-run` cannot report R10 (the probe writes for real)
- Marker detection inside fenced code blocks — never handled; any exact marker line counts
- Removal of `.claude/skills/`/`.claude/` created by init — never; brief owns only
  `host.PluginDir` + the CLAUDE.md candidates `InstructionFiles` names
- A stray-plugin row (`wd/.claude/skills/brief` when wd ≠ root): doctor cannot know Claude's
  launch dir, and no R13 id exists for it
- Agent resolution by frontmatter `name:` — roles checks filenames only
- `~/.claude/skills/brief` (user-scope plugin): never consulted

## Traps
- `t.TempDir` has no `.git` above it; `chmod` tests pass vacuously as root — skip under
  `os.Geteuid() == 0` (S02, 04, 06, 09)
- `--force init` over a non-default `feature-directory` leaves two feature roots, never removed
- A round-trip "byte-identical", or "nothing written", assertion passes vacuously if init wrote
  nothing — assert the control run differs first (S04, 06-09)
- A claude-code Init always creates `.claude/skills/brief/...`; Uninstall prunes empty dirs only
  down to and including `host.PluginDir` — `.claude/skills/`/`.claude/` always survive, empty
- pflag's `UnquoteUsage` recognizes one backtick span per usage string; a leaf's terse Usage line
  has an 80-column budget (S06, 08, 09)
- `os.Lstat` under a file-as-directory returns `ENOTDIR`, which `os.IsNotExist` never matches —
  `planPluginFile`/`checkWritable`'s ancestor walk check it explicitly (S09)
- `Report.Counts()` has no default arm — a row carrying anything but the four Severity constants
  silently drops out of counts, the summary and the exit code (S10)
- `newHealthyDoctorFixture` (internal/doctor) and the cli doctor goldens both pin the full
  12-row order and content — adding/reordering a row without updating both turns an unrelated
  test red (S10)
- A CRLF CLAUDE.md with a block reads as "no block" (SKIP) to doctor, while init refuses that
  same file — deliberate difference, don't add a CRLF row (S10)
- A doctor test that leaves `WithHomeDir` unset reads the developer's real `~/.claude/agents` —
  every test binding a bare-name role, or building a `Server` directly, must inject a home (S10)

## Open debts
- setup's own write path never rewrites/removes an `OriginOlder` file: `planPluginFile` and
  `planSnippet`/`planSnippetRemoval` still branch `== OriginCurrent`/`!= OriginCurrent`, so an
  older file is `kept "edited locally"`. Harmless while every older digest list ships empty; the
  first release that appends a real older digest must teach setup to rewrite/remove it first, or
  doctor's `run 'brief init'` fix can never clear the WARN it just reported. **Unowned — no
  scenario in this spec closes it; the final review must rule on it.**
