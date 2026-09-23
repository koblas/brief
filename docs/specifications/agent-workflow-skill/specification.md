# Specification: brief-workflow skill and bound-agent protocol checks

## Intent & Goal

**Primary Goal**: Any agent — brief's own or one the repository already has (e.g.
`.claude/agents/developer/Agent.md`) — can preload one brief-provided skill that teaches the
step protocol (pick up the next step, tick checklist items, close with `brief finish`, add a
step), and `brief doctor` verifies that the planner and implementer bound in `.brief.yaml`
actually preload it.

**Secondary Goals**:
- `brief doctor`'s role resolution matches Claude Code's: agents are identified by frontmatter
  `name:` anywhere under `.claude/agents/`, and a project definition shadows a user-level one.
  Today a nested layout is reported "not found" and a bare name can falsely resolve to an
  unrelated `~/.claude/agents/<name>.md`.
- init can, on explicit opt-in, add the skill to the repository's bound agents, and uninstall
  undoes that edit.

**Out of Scope**:
- `brief new step` scaffolding a `## Scenario` heading (separate fix).
- Editing any file under `~/.claude`.
- Editing a `brief:*` binding or a binding to another plugin.
- Changing the CLAUDE.md snippet, `skills/start`, `skills/finish`, or `agents/reviewer.md` bytes.
- The reviewer role preloading the skill (R15 limits the reviewer to `brief start` read-only and
  `brief check`; the skill teaches and pre-approves `finish`).

## Business Rules & Invariants

- Rule 1: Every `--host claude-code` install writes `.claude/skills/brief-workflow/SKILL.md`,
  with or without `--with-agents`. Its bytes are fixed and digest-tracked like every other
  brief artifact: unedited → kept/upgraded, edited → never rewritten, uninstall removes only an
  unedited copy.
- Rule 2: The skill is a standalone project skill (not inside the `.claude/skills/brief/`
  plugin), referenced by bare name `brief-workflow` in an agent's frontmatter `skills:`.
- Rule 3: init never edits an agent file the user authored unless `--edit-agents` is given, and
  never edits a file outside the repository.
- Rule 4: `--edit-agents` edits only the planner and implementer bindings that are bare names
  resolving to a file under `.claude/agents/**`, and only the frontmatter `skills:` line(s).
- Rule 5: Role resolution: a bare name matches frontmatter `name:` in any `*.md` under
  `<root>/.claude/agents/` recursively; only when the project defines none, under
  `~/.claude/agents/` recursively. Files whose frontmatter does not parse are not candidates.
  No filename fallback.
- Rule 6: Changed renders keep upgrade compatibility: previous planner/implementer bytes move to
  their older-digest lists and classify as "older", never "edited".
- Rule 7: Roles are reported, not enforced: every new roles/skill finding is WARN. The only new
  ERROR is `host-skill` "not a regular file".
- Rule 8: uninstall removes `brief-workflow` from bound repository agents' `skills:` unless the
  skill file itself is kept as edited.

---

## Triage Brief

**Affected surface**
- `internal/platform/artifact/plugin.go` (`Render` dispatch, skill renders),
  `agents.go` (`AgentPlanner`/`AgentImplementer`/`renderAgent`), `digest.go` (`Kind` enum,
  `…Digests`/`older…Digests`, `digestsFor`).
- `internal/platform/host/claudecode.go` — `claudeCodePluginFiles`/`claudeCodeAgentFiles`
  (`File{RelPath, Kind, Hook}`), `Plugin(withHook)`, `Agents()`.
- `internal/setup/setup.go` (`planPluginFiles`/`planAgentFiles`/`planPluginFile`),
  `uninstall.go`, `print.go`.
- `internal/doctor/host.go` — `resolveRoleBinding` (~689-725, stat of fixed paths only),
  `rolesCheck`, `hostAgentsCheck`.
- `internal/cli/init.go`, `doctor.go`, `uninstall.go`, `cli.go` — flags, long help, JSON fields.
- `internal/platform/stepfile/frontmatter.go` — `ParseFrontmatter` is a reusable YAML
  frontmatter splitter under `internal/platform` (importable from doctor/setup), but its
  `Frontmatter` struct has only `id`/`status`/`depends-on`; agent `name:`/`skills:`/
  `omitClaudeMd` need a sibling type, not new machinery.

**Prior art to follow**
- Adding a plugin file end-to-end: `claudecode.go` `File{RelPath, Kind}` entry + `Kind`
  constant + render func + digest pair + `digestsFor` case.
- `roles_to_add` in init's report is the precedent for "list what the user must add by hand".
- `--print` of the CLAUDE.md snippet (bare block only) is the precedent for printing a merge
  into a user-owned file.

**Already exists — do not re-plan**
- The digest older/current mechanism (`digest.go`) already classifies a changed render as
  "older" once previous bytes are moved to the `older…Digests` list; every `older…Digests`
  list ships empty today.
- `stepfile.ParseFrontmatter` as the frontmatter splitter.
- The writability pre-check (R10) and `--dry-run`/`--print` plumbing in setup.

**Genuinely new**
- A `KindSkillWorkflow` artifact and its host file entry.
- A recursive, frontmatter-`name:`-based agent resolver shared by doctor and setup.
- Surgical frontmatter `skills:` edit / removal on user agent files.
- `host-skill` and `roles-skill` doctor rows.

## Product Verdict

**SHIP WITH CHANGES** — changes accepted and folded in:
1. Reviewer is excluded: `agents/reviewer.md` keeps its bytes; `--edit-agents` and
   `roles-skill` consider planner and implementer only.
2. Flag is `--edit-agents` (not `--wire-agents` — collides with `--with-agents` on tab and in
   history; this is the only flag that edits files brief does not own).
3. uninstall undoes the edit (R16 round trip).
4. CLAUDE.md snippet unchanged.
5. `omitClaudeMd: true` only adds wording to a missing-skill WARN, never WARNs alone.
6. `allowed-tools` is a YAML list scoped to protocol verbs; `user-invocable: false`.
7. Missing `host-skill` beside an installed plugin is WARN, not ERROR.
8. **Architect must verify live** before SCENARIO-02 lands whether a plugin agent
   (`.claude/skills/brief/agents/*.md`) resolves a project skill by bare name; the Claude Code
   docs do not say. The one-line bodies keep the `start`/`finish` invocations inline so the
   agents still work if it does not (a missing preloaded skill is a warning, not an error).

Facts established while ruling: `brief finish` itself refuses an unticked checklist item;
`brief start` does not print the state path or caps (finish's refusals name them); Claude Code
loads two agents with the same `name:` in one `.claude/agents/` tree in unspecified order.

## Surface & Copy

### The skill file

Path `.claude/skills/brief-workflow/SKILL.md`; written for every claude-code install. Setup
report kind `skill`; artifact kind `KindSkillWorkflow`. Exact bytes:

````markdown
---
description: brief's step protocol — pick up a feature's next open step with brief start, tick its checklist as items go green, close it with brief finish. Use when planning or implementing a step of a brief-tracked feature.
user-invocable: false
allowed-tools:
  - Bash(brief start *)
  - Bash(brief finish *)
  - Bash(brief new step *)
  - Bash(brief status *)
  - Bash(brief check *)
---

# brief step protocol

A feature is a directory of markdown: a specification, ordered step files, and one
state file. `brief status` lists every feature and its next open step.

1. **Start.** `brief start <feature>` prints the next open step — its id, acceptance
   criteria and checklist — and the decisions it inherits from the state file. Work
   from that output; do not read the specification or earlier handoffs whole. It
   writes nothing.
2. **Work.** As each checklist item goes green, tick it by hand in the step file:
   `- [ ]` becomes `- [x]`. This is the only bookkeeping edit you make yourself.
   `brief finish` refuses while any item is unticked.
3. **Finish.** Write two bodies to scratch files (or pass `-` for one, read from stdin):
   - the handoff: what this step decided, what it left undone, what the next step
     must know;
   - the state: a COMPLETE replacement of the feature's state file — every inherited
     section `brief start` printed, updated, not just this step's delta. Anything you
     leave out is dropped.
   Then run `brief finish <feature> <step> --handoff <path> --state <path>`. It ticks
   the progress list, marks the step done, writes the step's handoff file and replaces
   the state file — all or nothing. A refusal names what to fix (an unticked item, a
   missing state heading, a body over its line cap) and changes no files; fix it and
   run it again.
4. **Add a step.** `brief new step <feature>` scaffolds the next step file and its
   progress entry; fill in its body.

Never tick the progress list, mark a step done, write a handoff file or edit the state
file by hand: `brief finish` is the only way a step closes. Headings, file names and
caps are configured per repository, and brief's own output names the ones in force.
`brief <command> --help` covers every flag.
````

### Rewritten plugin agents

Current bytes of each move to its older-digest list. `.claude/skills/brief/agents/planner.md`:

```markdown
---
name: planner
description: Turn a feature's specification into ordered scenario plans.
tools: Read, Grep, Glob, Bash, Edit, Write
skills:
  - brief-workflow
---

Plan the feature's next scenario: run `brief new step <feature>`, then fill its plan file. Never write production or test code.
```

`.claude/skills/brief/agents/implementer.md`:

```markdown
---
name: implementer
description: Implement a feature's next open step, from brief start through brief finish.
skills:
  - brief-workflow
---

Implement the next open step: `brief start <feature>`, work it, then `brief finish <feature> <step> --handoff <path> --state <path>`.
```

`reviewer.md`, the CLAUDE.md snippet, and `skills/start`/`skills/finish` keep current bytes.

### `brief init --edit-agents`

Flag help (double quotes, never backticks — pflag turns a backticked word into a placeholder):

```go
fs.Bool("edit-agents", false, "add \"brief-workflow\" to the \"skills:\" list of the planner\nand implementer agents bound in .brief.yaml (repository files only)")
```

Targets: planner and implementer bindings that are bare names resolving to a file under
`.claude/agents/**`. Never a `brief:*` binding, another plugin's binding, or anything under
`~/.claude`.

Edit per agent file — frontmatter only, surgical line edit:

| Shape found | Edit | Row |
|---|---|---|
| no `skills:` key | insert `skills: [brief-workflow]` before the closing `---` | `merged <rel> (brief-workflow added to skills)` |
| block list | append `<indent of first item>- brief-workflow` after the last item | same |
| one-line flow list `[…]` / `[]` | append `, brief-workflow` / write `[brief-workflow]` | same |
| already listed | none | `unchanged <rel>` |
| any other shape | none | `kept <rel> (skills: is not a list brief can edit; add brief-workflow by hand)` |
| not a regular file | none | `kept <rel> (not a regular file)` |

- Agent files join the existing writability pre-check; an unwritable one refuses the whole run
  under the existing R10 contract.
- Artifact `kind` is `bound-agent`. Row order: config, feature-root, plugin…, hook, skill,
  agent…, bound-agent…, snippet.
- `--dry-run`: same rows, writes nothing.
- `--print`: each pending edit as `# <path> (merge)`; body is only the inserted or rewritten
  line (snippet's bare-block precedent).
- Nothing to edit (exit 0), stderr:
  `brief init: --edit-agents: no planner or implementer bound to an agent under .claude/agents; nothing to edit`
- Without claude-code (exit 2):
  `brief init: --edit-agents requires --host claude-code; run 'brief init --host claude-code --edit-agents'`

Missing-skill report — stderr, exit 0, with or without the flag, listing agents still missing
the skill after this run (after the planned run under `--dry-run`):

```
brief init: bound agents do not preload the brief-workflow skill; add "brief-workflow" to the "skills:" list in each, or rerun with --edit-agents:
  .claude/agents/developer/Agent.md (implementer)
  ~/.claude/agents/planner.md (planner; user-level, edit by hand)
```

`--json`: new field `agents_missing_skill`, after `roles_to_add`, always present, never null;
items `{"role", "agent", "path" (absolute), "scope" ("project"|"user")}`. Omitted from the
`--print --json` document. `jsonFieldsParagraph(...)` appends `"agents_missing_skill"`.

`initLong` addition, after the `--with-agents` sentence:

> Every claude-code install also writes a "brief-workflow" skill under ".claude/skills/brief-workflow/", which agents preload by listing it in their frontmatter "skills:". init never edits an agent file of yours by default; stderr instead lists each planner or implementer bound in ".brief.yaml" whose agent lacks it. --edit-agents adds it to those agents' "skills:" lists, for agent files under ".claude/agents/" only; one under "~/.claude" is always left for you to edit.

### `brief uninstall`

- For each bound planner or implementer agent under `.claude/agents/**` whose `skills:` contains
  `brief-workflow`: remove the entry; when that leaves exactly `skills: [brief-workflow]` or
  `skills: []`, drop the line. Row `removed <rel> (brief-workflow from skills)`, kind
  `bound-agent`.
- When `SKILL.md` is kept as edited, the agent entries are kept too (no row).
- `uninstallLong` addition:
  `It also removes "brief-workflow" from the "skills:" list of the planner and implementer agents bound in ".brief.yaml", repository files only, unless the skill file itself is kept.`

### `brief doctor`

Row order: `… host-plugin, host-hook, host-skill, host-snippet, host-agents, roles, roles-skill`.
New rows are additive in `checks[]`; `counts` keeps its shape.

`host-skill` (path `.claude/skills/brief-workflow/SKILL.md`):

| Case | Severity | Detail | Fix |
|---|---|---|---|
| no claude-code install at all | SKIP | `not installed` | `run 'brief init --host claude-code'` |
| missing while plugin installed | WARN | `not installed; bound agents cannot preload it` | `run 'brief init'` |
| unreadable | WARN | `not readable (<reason>)` | existing `notReadableFix` |
| not a regular file | ERROR | `not a regular file` | `run 'brief init'` |
| older release | WARN | `installed by an older brief release` | `run 'brief init'` |
| edited | OK | `edited locally` | — |
| current | OK | `installed` | — |

`roles` resolution (Rule 5), plus:
- new WARN problem: `<role>: <name> defined <n> times under .claude/agents (<rel>, <rel>)`
- new OK suffix: `; user-level: <role>` (beside existing `; not verified: <role>`).

`roles-skill` (path `.brief.yaml`; planner and implementer only, bound and resolved, including
`brief:*`):

| Case | Severity | Detail | Fix |
|---|---|---|---|
| no config / unparseable / neither role bound and resolved | SKIP | `no planner or implementer bound` | — |
| any lacks the skill | WARN | `<role>: <name> does not preload brief-workflow` per agent, joined `; `; when that agent sets `omitClaudeMd: true`, append ` and omits CLAUDE.md, so it never sees brief's instructions` | `add "brief-workflow" to the "skills:" list of each agent named, or run 'brief init --edit-agents' for those in the repository` |
| all satisfied | OK | `planner, implementer preload brief-workflow` (+ `; not verified: <role>` for other-plugin bindings) | — |

Exit codes unchanged: 1 only when any row is ERROR.

`doctorLong`: replace the roles clause ("plus whether each role … nowhere in the repository.")
with:

> — plus whether each role bound in ".brief.yaml" resolves to an agent, matched by its frontmatter "name:" anywhere under ".claude/agents/", then "~/.claude/agents/" when the repository defines none, and whether the bound planner and implementer preload the "brief-workflow" skill.

---

## Scenarios (Gherkin)

```gherkin
Scenario: SCENARIO-01 init installs the brief-workflow skill on every claude-code install
  Given a repository with ".claude/" and no brief install
  When I run "brief init" (with or without --with-agents)
  Then ".claude/skills/brief-workflow/SKILL.md" is created with the ruled bytes
  And a re-run reports it "unchanged", an edited copy is kept, "--host none" writes no skill,
      --dry-run/--print show it, and uninstall removes an unedited copy

Scenario: SCENARIO-02 brief's planner and implementer agents preload the skill
  Given "brief init --with-agents"
  Then planner.md and implementer.md carry "skills: [brief-workflow]" and the one-line bodies
  And reviewer.md is byte-identical to today
  And a previous release's planner/implementer bytes are recognized as "older", upgraded by init, not "edited"

Scenario: SCENARIO-03 doctor resolves a bound role by frontmatter name, project first
  Given ".brief.yaml" binds implementer to "developer"
  And ".claude/agents/developer/Agent.md" declares "name: developer"
  When I run "brief doctor"
  Then the roles row resolves it (nested layout, any filename)
  And a project definition shadows a same-named "~/.claude/agents" agent
  And a name defined twice under .claude/agents WARNs naming both paths
  And a user-level-only resolution adds "; user-level: <role>"

Scenario: SCENARIO-04 doctor reports the brief-workflow skill's health (host-skill)
  Rows for not installed (SKIP), missing beside plugin (WARN), unreadable (WARN),
  not a regular file (ERROR, exit 1), older (WARN), edited (OK), current (OK)

Scenario: SCENARIO-05 doctor checks bound planner/implementer preload the skill (roles-skill)
  WARN per agent lacking it (with omitClaudeMd wording when set), OK when all do,
  SKIP when neither role is bound and resolved, "not verified" for other-plugin bindings

Scenario: SCENARIO-06 init reports bound agents missing the skill
  Given a bound implementer agent without the skill
  When I run "brief init" without --edit-agents
  Then stderr lists the agent path and role, the file is untouched, exit 0
  And --json carries "agents_missing_skill" (always present, never null)

Scenario: SCENARIO-07 init --edit-agents adds the skill to bound repository agents
  Applies the shape table (no key / block list / flow list / already listed / other / non-regular),
  never edits ~/.claude or brief:* bindings, honors --dry-run/--print and the writability pre-check,
  refuses (exit 2) without claude-code, says "nothing to edit" when none apply

Scenario: SCENARIO-08 uninstall removes brief-workflow from bound agents
  Removes the entry (and a now-empty skills: line); keeps entries when SKILL.md itself is kept as edited
```

---

## BDD Acceptance Progress

- [x] SCENARIO-01: init installs the brief-workflow skill on every claude-code install
- [ ] SCENARIO-02: brief's planner and implementer agents preload the skill
- [ ] SCENARIO-03: doctor resolves a bound role by frontmatter name, project first
- [ ] SCENARIO-04: doctor reports the brief-workflow skill's health (host-skill)
- [ ] SCENARIO-05: doctor checks bound planner/implementer preload the skill (roles-skill)
- [ ] SCENARIO-06: init reports bound agents missing the skill
- [ ] SCENARIO-07: init --edit-agents adds the skill to bound repository agents
- [ ] SCENARIO-08: uninstall removes brief-workflow from bound agents
