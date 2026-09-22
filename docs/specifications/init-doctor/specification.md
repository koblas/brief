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
  **Amended during SCENARIO-08 planning**: `roles.*` (`planner`, `implementer`, `reviewer`) and
  `optional-conventions` are free-form and carry no validation rule at load — any string,
  including empty (unbound); `doctor`'s `roles` row (S10) is where an unbound position is
  reported, not a load-time refusal.
- R2: **`init` always writes a commented `.brief.yaml`** — every key present but commented out,
  documenting each setting in place. It infers nothing beyond whether the feature root exists.
  The file is the repository's opt-in marker (the hook is silent without one).
- R3: **`init` converges.** Valid existing `.brief.yaml` → kept and reported; everything missing
  is installed; exit 0. Unparseable or invalid existing config → refusal, exit 1, no files
  changed. `--force` means only "rewrite `.brief.yaml` from defaults". A second run of the same
  invocation reports every artifact `unchanged`.

  **Amended in the git-boundary fix**: the install root `init`, `uninstall` and `doctor` all
  share is the nearest `.brief.yaml` walking up from the working directory (`config.Locate`'s own
  walk), but only when that config sits at or below the nearest enclosing git repository (walked
  from the working directory for a `.git` directory or file). A config found *above* that
  boundary — a `HOME`-level `.brief.yaml`, say, from a repository nested under an unrelated tree
  holding its own stray config — is treated exactly as though none existed: `init` writes a fresh
  `.brief.yaml` at the working directory rather than adopting or merging into the ancestor
  repository's own files (its `CLAUDE.md` included), and `uninstall`/`doctor` likewise never read
  or report on it. With no enclosing git repository anywhere above the working directory, there is
  no boundary to enforce and the walk is unbounded, exactly as before this rule existed.
- R4: **Claude Code integration is a skills-directory plugin** at `.claude/skills/brief/`:
  `.claude-plugin/plugin.json` (`"name": "brief"`), `skills/start/SKILL.md` (`/brief:start`),
  `skills/finish/SKILL.md` (`/brief:finish`) — both `disable-model-invocation: true` with
  `allowed-tools` scoped to `Bash(brief start *)` / `Bash(brief finish *)` and an `argument-hint`
  (**amended during SCENARIO-06 planning**, verified against code.claude.com/docs/en/skills: the
  permission pattern is `Bash(<prefix> *)`) — `hooks/hooks.json`
  (`PostToolUse`, matcher `Edit|Write|MultiEdit`, command `brief check --hook claude-code`), and,
  with `--with-agents`, `agents/{planner,implementer,reviewer}.md`. The hook is installed by
  default; `--no-hook` omits it. `brief` never edits `.claude/settings.json`,
  `.claude/commands/` or `.claude/agents/`.
- R5: **The only edit inside an existing file is the `CLAUDE.md` block** delimited by the lines
  `<!-- brief:begin -->` and `<!-- brief:end -->`. Location: repo-root `CLAUDE.md`, falling back
  to `.claude/CLAUDE.md` only when that exists and the root file does not; `init` says which.
  Re-running replaces the marked span; uninstall removes it and leaves the rest byte-identical.
  A lone marker refuses, naming file and line (no files changed). Snippet content (R17 thin):

  **Amended during SCENARIO-07 planning**: a candidate already holding the block wins over the
  root-first rule — Claude Code's own `/init` can create a root `CLAUDE.md` after the block
  landed in `.claude/CLAUDE.md`, and the plain root-first rule would then install a second block
  there. Two candidates each holding a block refuses, naming `.claude/CLAUDE.md` (root is the
  preferred location) and its own begin line, fix "delete that block". A chosen candidate with
  CRLF line endings refuses too, scoped to that one candidate — an untouched CLAUDE.md elsewhere
  with CRLF endings never blocks an install or removal aimed at the other one. An emptied
  CLAUDE.md is deleted on uninstall, the only "brief created it" signal this scenario keeps, so a
  pre-existing, already-empty CLAUDE.md is deleted too, not restored.

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

  **Amended during SCENARIO-08 planning**: the hint (stderr, before the next-action line) lists
  only roles still *unbound* in the kept or unchanged config — `brief init: <rel .brief.yaml> was
  not edited; to bind brief's agents, add these lines to it:` then `roles:` and one
  `  <role>: brief:<role>` line per unbound role, no lines at all when every role is already
  bound to anything (brief's own agent or the adopter's own — R15's "never shadows an existing
  agent"). `--force --with-agents` rewrites the config to the bound variant
  (`artifact.ConfigFileWithRoles`); "unchanged" under `--force` means the existing bytes already
  equal that bound variant exactly, not merely any recognized render. `--with-agents` requires the
  *resolved* host to be `claude-code`; otherwise a usage error, exit 2: `brief init: --with-agents
  requires --host claude-code; run 'brief init --host claude-code --with-agents'`. Without
  `--with-agents`, no agent row is planned at all and an already-installed `agents/` directory is
  left completely untouched — the same "no flag, no plan, no row" rule `--no-hook` already
  follows. `uninstall` carries no `--with-agents` flag of its own: it always plans the three
  agent files for removal, independent of whether the install that wrote them — or this
  `uninstall` call — ever named the flag.
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

  **Amended during SCENARIO-09 planning**: `--print`'s own artifact set is the pending
  (`ActionCreated`/`ActionMerged`) entries of `Result.Artifacts`, in that same order, the feature
  root excluded — `ActionUnchanged`/`ActionKept` never appear, with one exception (**amended in
  fix pass 4**): a CLAUDE.md candidate that is `ActionKept` because it is not a regular file
  still prints its own body as `merge`, since apply can never write there either but the block
  still needs to be added by hand. A `--force` config rewrite prints as `create` with the variant
  a real run would write. stderr ends with `brief init: printed
  only, no files changed; apply the output above by hand, or rerun without --print`; nothing
  pending prints `brief init: already installed; nothing changed` instead, stdout empty. stderr
  priority for init's own success line is `--dry-run` line > `--print` line > R8's no-host-detected
  line > the ordinary next action — each one entirely replaces the others, never combined.
- R10: **Unwritable target (spec R16)**: every target is checked before any write; on failure
  stderr refusal `brief init: <path>: <problem>; apply the output below by hand (no files
  changed)`, the `--print` output on stdout, exit 1.

  **Amended during SCENARIO-09 planning**: the writability probe runs only when applying for
  real — never under `--dry-run` or `--print`, both of which already write nothing — and only
  after every planning refusal already passed. Under `--json` the response is the standard error
  document alone (kind `refusal`, `files_changed:false`), never an `artifacts` field: R1's one
  common envelope outranks R10's "print output on stdout", so the JSON `fix` instead reads
  `run 'brief init --print --json' and apply the artifacts by hand`.
- R11: **Output.** init/uninstall stdout: one line per artifact, verb from the closed set
  `created | merged | unchanged | kept | removed`, relative path, optional parenthetical detail;
  next action on stderr (`brief init: installed for claude-code; start Claude Code in this
  directory (or run /reload-plugins in a session already here), then 'brief new feature <name>'`,
  or `... installed for claude-code in <rel>; start Claude Code in <rel> (or run /reload-plugins
  in a session already there), ...` when the install root is not the working directory —
  **amended during SCENARIO-06 planning**: Claude Code loads project skills-dir plugins only from
  the session's primary working directory, no walk-up; re-run: `brief init: already installed; nothing changed`; dry run:
  `brief init: dry run, no files changed; rerun without --dry-run to apply`). JSON (common
  envelope): `{host, dry_run, created:[abs], modified:[abs], artifacts:[{kind, path, action,
  detail}]}`, `kind` ∈ config | feature-root | plugin | hook | snippet | agent; uninstall adds
  `removed:[abs]`. init and uninstall carry `writesFilesAnnotation`. Nothing installed on
  uninstall: empty stdout, `brief uninstall: nothing installed for claude-code`, exit 0.
  **Amended during SCENARIO-08 planning**: init's JSON document gains `roles_to_add:[<lines>]`,
  always present (`[]` when nothing to add) — the same lines the stderr hint prints, one array
  element per line; stderr stays empty under `--json` exactly as every other success does.

  **Amended in the git-boundary fix**: a partial write (a write-path failure after at least one
  artifact already landed) prints the rows that actually landed — never the full plan, and never a
  row for the write that failed — on stdout before the refusal line on stderr; `--json` stays the
  standard error document, `files_changed:true`, no `artifacts` field, matching R10's own
  unwritable-target contract. `uninstall`'s own "removed" next-action line names the host right
  after "brief's" (`removed brief's claude-code install; …`) rather than trailing it with "for
  claude-code", which read awkwardly; every other next-action line keeps the trailing `for <host>`
  suffix.

  **Amended in fix pass 4, corrected in fix pass 5**: `uninstall`'s next-action line
  discriminates by what the plan actually holds, not by the requested host, dry run or not — the
  plan is fully computed before either arm returns, so both read the same four-way split. A host
  artifact (any non-config `ActionRemoved`) reports `brief uninstall: removed brief's <host>
  install; the feature root and its contents were left in place` for a real run, or `brief
  uninstall: dry run, nothing removed; rerun without --dry-run to remove brief's <host> install`
  under `--dry-run` (or "brief's install" for `none`, both arms). A lone config removal (the
  shape a default-host `uninstall` leaves after an earlier `init --host none`) reports `removed
  brief's config; …` or, under `--dry-run`, `dry run, nothing removed; rerun without --dry-run to
  remove brief's config` — never a claude-code install that was never there, dry run included:
  fix pass 4's own dry-run line named `installLabel(host)` unconditionally, the exact same false
  claim the real-run arm was fixed against in the git-boundary fix, just left standing on the
  dry-run arm. Everything kept reports `nothing removed; N file(s) edited locally were kept; run
  'brief uninstall --force' to remove them` for a real run, or, under `--dry-run`, `dry run,
  nothing removed; N file(s) edited locally would be kept; run 'brief uninstall --force' to
  remove them` — N counting only artifacts kept because they were edited, never one kept because
  it is not a regular file (`--force` cannot remove that either). Nothing installed reports
  `nothing installed for <host>` (real) or `dry run, nothing installed for <host>` (dry run,
  `withHostSuffix`, dropped for `none`); a non-empty plan with nothing removed and nothing
  force-removable reports `nothing removed for <host>` or `dry run, nothing removed for <host>`
  the same way.
  `init`'s own next-action line and JSON document additionally name host-detection's own signal
  when `--host` was not given: `brief init: installed for claude-code (detected <.claude|
  CLAUDE.md|~/.claude>; use --host none to skip); …`, and `--json` gains `detected_by` (null
  unless detected), inserted right after `host`.

  **Ruled in fix pass 5**: a non-regular CLAUDE.md candidate's own stdout row differs by
  direction — `uninstall`'s `kept <rel> (not a regular file)`, plain, never R9's own install-side
  `not a regular file; add the block by hand, see 'brief init --print'`. There is no block to
  add by hand on a removal: `planSnippetRemoval` never carries `notRegular`, and `--print` is
  `init`'s own flag, not `uninstall`'s.
- R12: **`check --hook <host>`** reads the host hook payload on stdin, takes the path from
  `tool_input.file_path`, and checks only the feature containing it. No `.brief.yaml` found, or
  a path outside the feature root: silent, exit 0. **Amended during SCENARIO-05 planning** (verified
  against code.claude.com/docs/en/hooks: a PostToolUse hook's exit 1 is shown to the user only as a
  "hook error" notice and never reaches the model; exit 2 is a blocking error; exit 0 with
  `hookSpecificOutput.additionalContext` JSON on stdout reaches the model as context): when the
  edited feature has ERROR findings, the hook path exits 0 and writes
  `{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"brief check: <feature
  dir rel>: N ERROR finding(s); run 'brief check <feature>'"}}` to stdout, nothing to stderr.
  No findings or WARN only: silent, exit 0. Payload parsing and output shaping sit behind an
  internal host interface.

  **Amended in the git-boundary fix**: the opt-in gate (no `.brief.yaml` found anywhere at or
  below the nearest enclosing git repository, R3) runs *before* the stdin payload is parsed at
  all — a malformed payload against an un-opted-in repository is silent, exit 0, exactly like a
  well-formed one, rather than being reported as a fault the repository never asked to have
  checked. Inside a repository that did opt in, a malformed payload is exit 1 (a PostToolUse
  hook's own non-blocking failure) with one stderr line, never usage-error's exit 2 — the payload
  is host-supplied, not user-typed.
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

  **Amended in the git-boundary fix**: `doctor`'s own install root shares R3's rule exactly — a
  `.brief.yaml` found above the nearest enclosing git repository is treated as though none
  existed, and the config family reports the "no config anywhere" rows rather than naming that
  ancestor file.

  **Amended in fix pass 4, corrected in fix pass 5**: a CLAUDE.md candidate that exists but is
  not a regular file (a symlink or a directory) is host-snippet WARN, detail `not a regular file
  (symlink|directory); brief block not installed`, fix `run 'brief init --print' and add the
  CLAUDE.md block by hand` — not the SKIP `not installed` row a genuinely missing candidate
  reports — unless the other candidate still holds a real block, in which case that block is
  reported exactly as it would be were both candidates regular files. Fix pass 4's own wording
  ("unless the other candidate still holds a real block") only ever ruled the both-a-block-and-a-
  non-regular-file case; it never ruled a non-regular candidate sitting *behind* a regular,
  blockless one. The WARN considers only the one candidate `planSnippet` would itself choose
  once neither candidate holds a block — the first candidate that exists at all, regular or not,
  in host.InstructionFiles' own priority order (`chooseSnippetLocation`'s own fallback rule) —
  never a later candidate's own shape. A regular root `CLAUDE.md` with no block and a directory
  at `.claude/CLAUDE.md` is the SKIP `not installed` row, naming root `CLAUDE.md`, exactly as
  `init --dry-run` would merge into that same file; the directory is never named, since brief
  would never write there either.

  **Amended in fix pass 7**: the chosen candidate existing but unreadable (any `os.ReadFile`
  failure on a present, regular file — permission denied is the practical case) is its own
  host-snippet WARN, distinct from the not-a-regular-file WARN above: detail `not readable
  (<reason>); cannot check for brief block`, `<reason>` the read error's own underlying cause
  (e.g. `permission denied`) rather than a hardcoded word, fix `chmod +r <rel path>, then run
  'brief init --host claude-code'`. Asserting `not installed` from a failed read is an
  unverifiable claim — brief does not know whether a block is present, only that it could not
  check — so this WARN replaces the SKIP a missing candidate reports, exactly as the
  not-a-regular-file WARN already does. The same block-wins carve-out applies unchanged: an
  unreadable candidate yields to the other candidate's own real block, reported exactly as it
  would be were the unreadable candidate a regular, readable file.
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

**Final verdict (`product-vision`, post-implementation, fix pass 12): SHIP WITH CHANGES.**
Findings resolved in fix pass 12: `doctor`'s unreadable-directory fix now names the actual
ancestor missing its search bit (an ancestor walk bounded at the install root), with the write
bit restored too, since a plain `chmod u+rx` on the named directory still left `brief init`
unable to write through it; `check --hook`'s help copy reworded (NIT, folded). See STATE.md for
the open debts this verdict left standing.

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
- [x] SCENARIO-02: doctor reports config, feature-root and environment health
- [x] SCENARIO-03: init writes the config and feature root, and converges
- [x] SCENARIO-04: uninstall removes the config init wrote and nothing else
- [x] SCENARIO-05: check --hook scopes a Claude Code hook call to the edited feature
- [x] SCENARIO-06: init installs the Claude Code plugin
- [x] SCENARIO-07: init adds the CLAUDE.md instruction block
- [x] SCENARIO-08: --with-agents scaffolds three role agents and binds them
- [x] SCENARIO-09: --print, host detection, and unwritable targets
- [x] SCENARIO-10: doctor reports Claude Code integration health
