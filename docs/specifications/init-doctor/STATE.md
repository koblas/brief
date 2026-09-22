# init-doctor — current state

Scenarios complete: SCENARIO-01..10 — every spec scenario shipped. Fix pass since: closed an
install-root git-boundary MAJOR plus a set of MINOR/NIT folds (R3/R11/R12/R13 amended).

## Binding decisions
- `config.LocateWithin(dir, boundary)` is the one bounded walk; `Locate` = `LocateWithin(dir, "")`;
  `LocateInRepo` bounds it to `repo.Root(wd)` (nearest enclosing `.git`, dir or file). `init`,
  `uninstall`, `doctor` and `check --hook`'s opt-in gate all call `LocateInRepo`, never bare
  `Locate`: a `.brief.yaml` found *above* that boundary (a HOME-level config, say) is treated as
  though none existed — root = `wd`, never adopted or merged into. No enclosing repo → unbounded,
  today's plain walk. `Resolve`'s own walk (every other command) stays unbounded, untouched.
  `repo.Root` also backs `checkEnvGit` (S01-07, S10, fix pass)
- `internal/doctor`/`internal/setup` import only `internal/platform/*` + stdlib, never each other
  or `scaffold`; neither has a `Store` port. `internal/platform/writable.Probe` is the one
  write-probe both share now (dedup'd from two copies) (S02, 03, 06, 07, 09, fix pass)
- `internal/platform/artifact` renders + digests every brief-written file and owns
  `ScanSnippetMarkers`, the one marker scanner setup and doctor both call, plus `Origin`
  classification. Every `older…Digests` list ships empty, so `OriginOlder` is reachable only
  white-box. CLAUDE.md is the one parameterized artifact (`SnippetBlock(dir)`), matches a block
  for *any* dir. `host.Host` imports `artifact`, never reverse (S03, 05, 06-10)
- `init`/`uninstall` plan-then-apply, every refusal decided before the first write/removal. Apply
  order: feature root, plugin files, agent files (`WithAgents` only), CLAUDE.md, config last.
  `checkWritable` runs between planning and apply, real runs only; `--force` rewrites
  `.brief.yaml` only. `verifySnippetUnchanged` re-reads CLAUDE.md immediately before either
  writes to it and refuses (`ErrConcurrentEdit`) on any mismatch from the bytes planning read —
  the one read-modify-write guard this package runs (S03, 06-09, fix pass)
- CLAUDE.md location: a candidate already holding a recognized block wins; else first existing
  candidate; else root is created. `mergeSnippet`/`removeSnippet` are exact inverses, byte-
  identical round trip, no sidecar (S07)
- Role bindings come only from `artifact.AgentBindings()`, written only into a config `Init`
  creates or `--force`-rewrites. Plugin agents: `agents/<role>.md`, addressed `brief:<role>`; a
  project `.claude/agents/<role>.md` override wins. `roles` row resolves `brief:<name>` via
  plugin-or-project, bare `<name>` via project-or-injected-home; WARN never ERROR (S08, 09, 10)
- `check --hook <host>`: the `LocateInRepo` opt-in gate now runs *before* stdin is parsed — a
  malformed payload against an unopted-in repo is silent exit 0, like any payload; inside an
  opted-in repo it's exit 1 (`errMalformedHookPayload`, one stderr line), never usage-error's
  exit 2 — host-supplied, not user-typed. ERROR findings → exit 0, `additionalContext` on stdout
  (S05, fix pass)
- Host detection (`setup` only, never `doctor`): `InitRequest.Host == ""` → root `.claude`/
  `CLAUDE.md`, or `WithHomeDir`'s own `.claude` → `HostClaudeCode`; else `HostNone` (S09)
- **S10**: `doctor` appends `host-plugin, host-hook, host-snippet, host-agents, roles` after
  `env-path` (R13's id list closed). `filesInstalled` gates host-plugin/host-hook's SKIP;
  `integrationInstalled` (`filesInstalled` **or** a snippet block) gates env-path's ERROR arm.
  `host.originRow` decides every row's WARN-older/OK-edited/OK-current triple
- A write command's partial-write failure (`ErrPartialWrite`) returns the populated `Result`, not
  the zero value; cli's `renderPartialWrite` (text) prints `landedArtifacts(res)` — `Artifacts`
  filtered to paths `Created`/`Modified`/`Removed` actually name — before the refusal line, never
  the row for the write that failed; `--json` stays the standard error document,
  `files_changed:true`, no `artifacts` field (fix pass)

## Left unbuilt
- Line number for a value error, heading-shape rules — unowned (S01)
- Feature root in `--print` output, `host` field in `--print --json` — never printed/emitted
- Dry-run writability prediction — `--dry-run` cannot report R10 (the probe writes for real)
- Marker detection inside fenced code blocks — never handled; any exact marker line counts
- Removal of `.claude/skills/`/`.claude/` created by init — never; brief owns only `host.PluginDir`
  + the CLAUDE.md candidates `InstructionFiles` names
- A stray-plugin row (`wd/.claude/skills/brief` when wd ≠ root) — doctor cannot know Claude's
  launch dir, and no R13 id exists for it
- Agent resolution by frontmatter `name:` — roles checks filenames only
- `~/.claude/skills/brief` (user-scope plugin) — never consulted

## Traps
- `t.TempDir` has no `.git` above it — a fixture without one lands in `LocateInRepo`'s unbounded
  arm; one that DOES create `.git` (several doctor fixtures do, at `wd`) puts its own boundary at
  `wd`, so a config placed farther up becomes invisible — move `.git` up to enclose it if the test
  means to exercise a farther ancestor (S02, 04, 06, 09, fix pass)
- `--force init` over a non-default `feature-directory` leaves two feature roots, never removed
- A round-trip "byte-identical"/"nothing written" assertion passes vacuously if init wrote
  nothing — assert the control run differs first (S04, 06-09)
- A claude-code Init always creates `.claude/skills/brief/...`; Uninstall prunes empty dirs only
  down to and including `host.PluginDir` — `.claude/skills/`/`.claude/` always survive, empty
- pflag's `UnquoteUsage` recognizes one backtick span per usage string — an 80-column budget on a
  leaf's terse Usage line (S06, 08, 09)
- `os.Lstat` under a file-as-directory returns `ENOTDIR`, `os.IsNotExist` never matches it —
  `planPluginFile`/`checkWritable`'s ancestor walk check it explicitly (S09)
- `Report.Counts()` has no default arm — a row carrying anything but the four Severity constants
  silently drops out of counts, the summary and the exit code (S10)
- `newHealthyDoctorFixture` and the cli doctor goldens both pin the full 12-row order and content
  — adding/reordering a row without updating both turns an unrelated test red (S10)
- A CRLF CLAUDE.md with a block reads as "no block" (SKIP) to doctor, while init refuses it —
  deliberate difference, don't add a CRLF row (S10)
- A doctor test that leaves `WithHomeDir` unset reads the developer's real `~/.claude/agents` —
  every bare-name-role test, or one building a `Server` directly, must inject a home (S10)
- `usageFix` (cli) only recovers a "run '…'" fix from a message ending at the closing quote; R8's
  unknown-host copy trails prose after it, so it falls back to the generic invocation instead —
  use `reporter.usageErrorWithFix` there rather than broadening the shared parser (fix pass)

## Open debts
- setup's own write path never rewrites/removes an `OriginOlder` file — harmless while every
  older digest list ships empty; the first release that appends a real one must teach setup to
  rewrite/remove it, or doctor's `run 'brief init'` fix can never clear the WARN it just reported.
  **Unowned — no scenario in this spec closes it; the final review must rule on it.**
- doctor's own text rows (R13) are two-space-joined, not `tabwriter`-aligned like `status`/`check`
  — raised and deferred in the fix pass: stdout is pinned byte-exact across ~12 tests, a real
  formatting change, not a mechanical one. **Unowned.**
- `hostPluginCheck`/`hostAgentsCheck` each inline the same missing→older→edited→current body
  against `originRow`; `Diagnose`'s config-family branch is a bare `switch`, not an extracted
  helper — raised as refactor-advisor MINORs, deferred rather than risk behavior change alongside
  this pass's other fixes. **Unowned.**
