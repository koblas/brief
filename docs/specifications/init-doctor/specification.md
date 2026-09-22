# Specification: `brief init`, `brief uninstall`, `brief doctor`

## Intent & Goal

**Primary Goal**: One command sets a repository up for `brief` — config, feature root, and
Claude Code integration — `brief doctor` reports whether that setup is healthy, and
`brief uninstall` removes exactly what `init` added.

**Out of Scope**: `--global` installs and the project/global `scope-conflict` check (follow-up
feature); native integration for hosts other than Claude Code (they get `--print`); `init --show`
(dropped — `doctor` reports installed state); inferring config values from existing markdown
(R3 forbids structure-from-English); `doctor` reading feature contents (that is `check`);
review policy or persona content in the reviewer agent.

**User decisions (binding)**: full spec surface minus `--global` and `--show`; Claude Code
first; hook on by default with `--no-hook`; ownership by naming convention — realized as the
namespaced plugin directory `.claude/skills/brief/` plus a marked `CLAUDE.md` block; three
roles (planner, implementer, reviewer); re-running `init` converges and keeps a valid existing
config, `--force` only rewrites `.brief.yaml` from defaults; `doctor` checks config health,
feature root (accessible, not counted), environment, host integration — setup only.

## Business Rules & Invariants

- R1: **Config values are validated at load** (`config.Resolve`), so every command refuses an
  invalid config, not only `doctor`: caps > 0; headings non-empty and distinct; step-file
  pattern has exactly one `%d`/`%0Nd` verb; `state-file` ≠ `specification-file`; no path
  separator in file-name values. Refusal names `.brief.yaml`, the key and its value, and a fix.
- R2: **`init` always writes a commented `.brief.yaml`** — every key present but commented out,
  documenting each setting in place. It infers nothing beyond whether the feature root exists.
  The file is the repository's opt-in marker (the hook is silent without one).
- R3: **`init` converges.** Valid existing `.brief.yaml` → kept and reported; everything missing
  is installed; exit 0. Unparseable or invalid existing config → refusal, exit 1, no files
  changed. `--force` means only "rewrite `.brief.yaml` from defaults". A second run of the same
  invocation reports every artifact `unchanged`.
- R4: **Claude Code integration is a skills-directory plugin** at `.claude/skills/brief/`:
  `.claude-plugin/plugin.json` (`"name": "brief"`), `skills/start/SKILL.md` (`/brief:start`),
  `skills/finish/SKILL.md` (`/brief:finish`) — both `disable-model-invocation: true` with
  `allowed-tools` scoped to `Bash(brief start:*)` / `Bash(brief finish:*)` — `hooks/hooks.json`
  (`PostToolUse`, matcher `Edit|Write|MultiEdit`, command `brief check --hook claude-code`), and,
  with `--with-agents`, `agents/{planner,implementer,reviewer}.md`. The hook is installed by
  default; `--no-hook` omits it. `brief` never edits `.claude/settings.json`,
  `.claude/commands/` or `.claude/agents/`.
- R5: **The only edit inside an existing file is the `CLAUDE.md` block** delimited by the lines
  `<!-- brief:begin -->` and `<!-- brief:end -->`. Location: repo-root `CLAUDE.md`, falling back
  to `.claude/CLAUDE.md` only when that exists and the root file does not; `init` says which.
  Re-running replaces the marked span; uninstall removes it and leaves the rest byte-identical.
  A lone marker refuses, naming file and line (no files changed). Snippet content (R17 thin):

  ```
  <!-- brief:begin -->
  ## brief
  Features under `<feature-directory>/` are tracked by `brief`. To work on a step, run
  `brief start <feature>` and work from its output rather than reading the specification
  or earlier steps whole. Close the step with
  `brief finish <feature> <step> --handoff <path> --state <path>` — never write a handoff
  or tick the progress list by hand. `brief --help` for the rest.
  <!-- brief:end -->
  ```
- R6: **Ownership and edits.** brief owns the plugin directory and the marked block. A file is
  "brief-written and unedited" when its bytes equal what this binary would write, or any
  earlier release's output (digests compiled into the binary — no manifest, no sidecar).
  Otherwise it is "edited locally": `init` keeps it (`kept`), `uninstall` keeps it unless
  `--force`. Directories left empty are removed; a directory still holding non-brief files is
  kept. The feature root and anything under it are never removed.
- R7: **Roles.** With `--with-agents`, three agents: planner, implementer, reviewer — each thin
  and limited to calls into the tool (the reviewer: `brief start <feature>` read-only and
  `brief check <feature>`; no review policy). Bindings (`roles: {planner: brief:planner, …}`)
  are written only into a `.brief.yaml` that `init` creates in the same run; an existing config
  is never edited — stderr says which lines to add. An existing `agents/<role>.md` inside the
  plugin that matches no known output is the adopter's customisation: `kept`.
- R8: **Host selection.** `--host claude-code|none`. Default: `claude-code` when the repo has
  `.claude/` or `CLAUDE.md`, or `~/.claude` exists; otherwise config + root only, with stderr
  `brief init: no agent host detected; run 'brief init --host claude-code' to install
  integration`, exit 0. Unknown host: usage error, exit 2:
  `brief init: unknown host "<h>"; expected one of: claude-code, none; run 'brief init --print' to wire it by hand`.
- R9: **`--dry-run`** prints the same report and writes nothing. **`--print`** writes nothing
  and emits each artifact on stdout as `# <relative path> (create|merge)` then its body, blank
  line between artifacts (snippet body = the marked block); `--print --json` gives
  `{artifacts:[{path, action, body}]}`. `--dry-run` with `--print`: usage error, exit 2
  (`brief init: --dry-run and --print cannot be combined; run 'brief init --print'`).
- R10: **Unwritable target (spec R16)**: every target is checked before any write; on failure
  stderr refusal `brief init: <path>: <problem>; apply the output below by hand (no files
  changed)`, the `--print` output on stdout, exit 1.
- R11: **Output.** init/uninstall stdout: one line per artifact, verb from the closed set
  `created | merged | unchanged | kept | removed`, relative path, optional parenthetical detail;
  next action on stderr (`brief init: installed for claude-code; run /reload-plugins, then 'brief
  new feature <name>'`; re-run: `brief init: already installed; nothing changed`; dry run:
  `brief init: dry run, no files changed; rerun without --dry-run to apply`). JSON (common
  envelope): `{host, dry_run, created:[abs], modified:[abs], artifacts:[{kind, path, action,
  detail}]}`, `kind` ∈ config | feature-root | plugin | hook | snippet | agent; uninstall adds
  `removed:[abs]`. init and uninstall carry `writesFilesAnnotation`. Nothing installed on
  uninstall: empty stdout, `brief uninstall: nothing installed for claude-code`, exit 0.
- R12: **`check --hook <host>`** reads the host hook payload on stdin, takes the path from
  `tool_input.file_path`, and checks only the feature containing it. No `.brief.yaml` found, or
  a path outside the feature root: silent, exit 0. Otherwise `check`'s exit codes; the first
  stderr line stands alone: `brief check: <feature dir rel>: N ERROR findings; run 'brief check
  <feature>'`. Payload parsing sits behind an internal host interface.
- R13: **`doctor`** is setup only — never reads feature contents. Stdout: one row per check,
  `<SEVERITY>  <id>  <rel path>  <detail>` with SEVERITY ∈ OK | WARN | ERROR | SKIP. Stderr
  summary: `brief doctor: setup ok; run 'brief check' for feature content` or
  `brief doctor: N ERROR, M WARN; this checks setup only, run 'brief check' for feature
  content`. Exit 1 on any ERROR, else 0. JSON: `{counts:{error, warn, ok, skip},
  checks:[{id, severity, path, detail, fix}]}` — path absolute, fix null when nothing to do.
  Stable ids: config-file, config-parse, config-values (one row per bad value), config-shadow
  (ancestor configs shadowed — OK with detail), root-dir (exists, is a directory, readable and
  writable — not counted), env-git, env-path (ERROR when integration is installed but `brief`
  is not on PATH; WARN when the PATH `brief` is a different version), host-plugin, host-hook,
  host-snippet, host-agents, roles. SKIP when a check could not run (e.g. config-values when the
  config does not parse) or its subject is not installed. Older-release files → WARN with fix
  `brief init`; locally edited → OK, detail `edited locally`.
- R14: **Command surface & order.** Root help and every "expected one of:" list:
  `new, start, finish, status, check, init, doctor, uninstall`. Shorts: init `install brief's
  config and agent-host integration`; doctor `check brief's setup: config, feature root, host
  integration`; uninstall `remove what init installed`.

  ```
  brief init [--host <name>] [--no-hook] [--with-agents] [--dry-run | --print] [--force] [--json]
  brief uninstall [--host <name>] [--dry-run] [--force] [--json]
  brief doctor [--json]
  brief check [feature] [--hook <host>]
  ```

---

## Triage Brief

**Affected surface.** `internal/cli/cli.go` `newRootCommand` (registration order drives root help
and `expectedCommandList`); golden strings `"expected one of: new, start, finish, status,
check"` appear in 6 files under `internal/cli` (e.g. `help_test.go:329-349`) — all change.
`internal/platform/config/{config.go,resolve.go}` (schema, `Default()`, `Resolve` walk-up,
`decodeConfig` with `KnownFields(true)` and no value validation). `internal/assemble/check.go`
(content-only checks — `doctor` must not duplicate). JSON/refusal plumbing: `internal/cli/json.go`
(`jsonHeader`, `jsonError`, `filesChangedFor`, `writesFilesAnnotation` at `cli.go:172-178`),
`internal/cli/refusal.go` (`reporter.refusal`, `InvalidConfigError` rendering).

**Prior art.** `leafCommand` registration; `check`/`status` shape for read-only `doctor`;
`new feature`/`finish` shape (plus `writesFilesAnnotation`, `scaffold.Result` created/modified)
for `init`/`uninstall`; `check`'s Rule/Severity vocabulary for doctor severities.

**Already exists — do not re-plan.** `config.Resolve`/`config.Default`; the `--json` envelope
and R14a refusal renderer; `leafCommand` + help template + 80-column wrap discipline;
`versionString` (`cli.go:718`) for env-path version comparison; `stepfile.Compile` (pattern
validation).

**Must be built.** Config value validation; a package for init/uninstall write logic
(scaffold-shaped) and one for doctor's read logic; a Claude Code host adapter behind an
internal interface (artifact rendering, hook payload parsing); compiled-in digests of emitted
artifacts; three new leaves and `check --hook`.

**Verified externally.** Claude Code skills-directory plugins: a folder with
`.claude-plugin/plugin.json` under a skills directory loads as `<name>@skills-dir` with no
marketplace; plugin skills are namespaced `/<plugin>:<skill>`; `commands/` is legacy, use
`skills/<name>/SKILL.md`; `hooks/hooks.json` uses the settings.json hook format; changes apply
with `/reload-plugins`; project-scope plugins require workspace trust.
(code.claude.com/docs/en/plugins, /plugins-reference.)

## Product Verdict

**SHIP WITH CHANGES** (`product-vision`, pre-design). Accepted (user confirmed 1–3 and 7):
1. Plugin directory instead of editing `.claude/settings.json`/`commands`/`agents` — makes the
   spec's byte-identical round trip achievable and ownership a directory boundary.
2. Drop `init --show`; `doctor` reports installed state.
3. `init` converges and keeps a valid config (R16), `--force` only rewrites config.
4. Three roles need a `roles.reviewer` binding and an R15 amendment; reviewer limited to calls
   into the tool.
5. Config value validation lives in `config.Resolve` (scenario 01), not only `doctor`.
6. The hook needs scoping: `check --hook claude-code` reads the edited path from the payload.
7. `--global` deferred to a follow-up.

Spec amendments applied to `docs/specifications/brief/specification.md`: `doctor` in the command
table; `--show` removed; R15 amended (three positions; unbound roles reported by `doctor`);
Init/First run amended (plugin directory, commented config always written, `--global`
deferred); open questions 1–5 marked settled.

---

## Scenarios (Gherkin)

```gherkin
Scenario: SCENARIO-01 Invalid config values are refused by every command
  Given .brief.yaml sets handoff-cap-lines to 0 (likewise: empty heading, state-file equal to specification-file, a path separator in a file name, a step pattern without exactly one verb)
  When I run "brief status" (or start, check, finish, new)
  Then stderr is one refusal naming .brief.yaml, the key and its value, with a fix; exit 1; --json gives an error document of kind refusal

Scenario: SCENARIO-02 doctor reports config, feature-root and environment health
  When I run "brief doctor" in a healthy repo
  Then stdout has one row per check (config-file, config-parse, config-values, config-shadow, root-dir, env-git, env-path) with OK/WARN/ERROR/SKIP, id, relative path and detail
  And stderr says "brief doctor: setup ok; run 'brief check' for feature content", exit 0
  And a missing or non-directory feature root is ERROR with a fix, exit 1; an unparseable config makes config-values SKIP
  And --json gives {counts, checks:[{id, severity, path, detail, fix}]}

Scenario: SCENARIO-03 init writes the config and feature root, and converges
  When I run "brief init --host none" in a fresh repo
  Then .brief.yaml is created with every key present but commented, and the feature root is created
  And stdout lists "created .brief.yaml" and "created docs/specifications/", with a next action on stderr
  And a second run reports "unchanged" for both, exit 0
  And a valid existing config is "kept"; an unparseable one refuses (exit 1, no files changed); --force rewrites it from defaults
  And --dry-run prints the same report and writes nothing

Scenario: SCENARIO-04 uninstall removes the config init wrote and nothing else
  Given init has run
  When I run "brief uninstall"
  Then an unedited .brief.yaml is removed; an edited one is kept and reported, and --force removes it
  And the feature root and everything under it are never removed
  And with nothing installed, stdout is empty, stderr says nothing is installed, exit 0

Scenario: SCENARIO-05 check --hook scopes a Claude Code hook call to the edited feature
  Given a PostToolUse payload on stdin whose tool_input.file_path is inside feature "auth"
  When I run "brief check --hook claude-code"
  Then only "auth" is checked; exit codes follow check's rules, and the first stderr line stands alone as a summary with the next command
  And a path outside the feature root, or no .brief.yaml found, is silent with exit 0

Scenario: SCENARIO-06 init installs the Claude Code plugin
  When I run "brief init --host claude-code"
  Then .claude/skills/brief/ holds the plugin manifest, skills start and finish (/brief:start, /brief:finish), and hooks/hooks.json running "brief check --hook claude-code"
  And --no-hook installs everything except the hook
  And re-running is "unchanged"; a locally edited file is "kept"; uninstall removes the directory and keeps edited files unless --force

Scenario: SCENARIO-07 init adds the CLAUDE.md instruction block
  When I run "brief init --host claude-code"
  Then CLAUDE.md gains the snippet between <!-- brief:begin --> and <!-- brief:end -->, naming the configured feature directory
  And re-running replaces only the marked span; uninstall removes it, leaving the rest byte-identical
  And a lone begin or end marker refuses naming the file and line (no files changed)

Scenario: SCENARIO-08 --with-agents scaffolds three role agents and binds them
  When I run "brief init --host claude-code --with-agents" in a fresh repo
  Then agents planner, implementer and reviewer are written under the plugin directory, and the new .brief.yaml binds roles to brief:planner, brief:implementer and brief:reviewer
  And an existing config is never edited; stderr says which roles lines to add

Scenario: SCENARIO-09 --print, host detection, and unwritable targets
  When I run "brief init --print"
  Then stdout carries each artifact as "# <path> (create|merge)" followed by its body; nothing is written
  And the host defaults to claude-code when .claude/ or CLAUDE.md exists; otherwise init installs config and root and says no host was detected
  And an unknown host is a usage error naming claude-code and none; --dry-run with --print is a usage error
  And an unwritable target is detected before anything is written: a refusal on stderr, the --print output on stdout, exit 1, no files changed

Scenario: SCENARIO-10 doctor reports Claude Code integration health
  Given init ran with --host claude-code
  When I run "brief doctor"
  Then rows host-plugin, host-hook, host-snippet, host-agents and roles report OK, or WARN when installed by an older version, or "edited locally", or SKIP when not installed, each with a fix
  And env-path is ERROR when the integration is installed but brief is not on PATH
```

---

## BDD Acceptance Progress

- [x] SCENARIO-01: Invalid config values are refused by every command
- [ ] SCENARIO-02: doctor reports config, feature-root and environment health
- [ ] SCENARIO-03: init writes the config and feature root, and converges
- [ ] SCENARIO-04: uninstall removes the config init wrote and nothing else
- [ ] SCENARIO-05: check --hook scopes a Claude Code hook call to the edited feature
- [ ] SCENARIO-06: init installs the Claude Code plugin
- [ ] SCENARIO-07: init adds the CLAUDE.md instruction block
- [ ] SCENARIO-08: --with-agents scaffolds three role agents and binds them
- [ ] SCENARIO-09: --print, host detection, and unwritable targets
- [ ] SCENARIO-10: doctor reports Claude Code integration health
