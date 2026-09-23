# agent-workflow-skill — current state

Scenarios complete: SCENARIO-01. Last updated by SCENARIO-01.

## Binding decisions
- The skill is a separate `host.Host.Skills() []File` list, never an entry in
  `Plugin()` — putting it in `Plugin` would make `Plugin(false)` trim the skill
  instead of the hook and break doctor's future `host-plugin` check. (SCENARIO-01)
- `File.RelPath` is install-root-relative; the skill's own entry is
  `host.WorkflowSkillDir + "/SKILL.md"`, `WorkflowSkillDir = ".claude/skills/brief-workflow"`.
  Later scenarios (doctor's `host-skill` row, `--edit-agents`) read the path from here. (SCENARIO-01)
- Setup report kind `setup.KindSkill = "skill"`; artifact kind `artifact.KindSkillWorkflow`,
  render `artifact.SkillWorkflow()`. Row order: config, feature-root, plugin…, hook, skill,
  agent…, snippet. A later scenario inserting `bound-agent…` rows places them between
  agent and snippet. (SCENARIO-01)
- Skill arts ride `writeArts` (drives `apply`, `writableTargets`, `--print` bodies) —
  anything planned only into `artifacts` reports `created` and writes nothing. (SCENARIO-01)
- `olderSkillWorkflowDigests` ships empty; a future byte change to the skill moves today's
  digest there (Rule 6). (SCENARIO-01)
- brief owns `host.PluginDir` and below **and** `host.WorkflowSkillDir`; prune stops at
  each, never `.claude/skills/` itself. (SCENARIO-01)
- Uninstall's skill removal rides `planPluginRemoval`: `--force` removes an edited
  SKILL.md, same contract as every other plugin/agent file. (SCENARIO-01)

## Left unbuilt
- `initLong`'s remaining sentences ("init never edits an agent file of yours…
  `--edit-agents` …") — owned by S06 (missing-skill report) / S07 (`--edit-agents` itself).
- `uninstallLong` does not yet mention the skill; its ruled addition is S08's.
- doctor's `host-skill` row does not exist yet — S04. `anyIntegrationFilePresent`
  (`internal/doctor/host.go`) does not see the skill; S04 decides whether `Skills()` joins
  that union.
- Planner/implementer plugin agents do not yet carry `skills: [brief-workflow]` — S02.
  Role resolver (frontmatter `name:`) — S03. `roles-skill` doctor row — S05.
- `artifact.OriginOlder` handling in `setup.planPluginFile`/`planPluginRemoval` is still
  absent for every Kind, skill included — S02 builds it. S01 is unaffected: the skill's
  own older-digest list ships empty, so no existing install can classify as "older" yet.

## Traps
- `golangci-lint`'s `exhaustive` is on: `artifact.Render`, `artifact.digestsFor`,
  `setup.printArtifacts` each switch over a Kind and need the new case, or lint fails
  while tests stay green. A test-body `switch` over `setup.Kind` trips the same linter —
  use an if-chain in test helpers instead.
- Pinning the SKILL.md test against `artifact.SkillWorkflow()` or `Render(...)` proves
  nothing; the literal must come from specification.md, byte-for-byte (em dash, exact
  wrapping, trailing newline).
- `initLong` is hand-wrapped; a verbatim `Contains` on a ruled sentence fails — normalize
  whitespace in the assertion first.
- The R10 unwritable refusal is exit **1**, not 2 (2 is usage errors).

## Open debts
- doctor's `host-skill` row (SKIP/WARN/ERROR/OK matrix) — S04, unowned until then.
- `roles-skill` doctor row and the frontmatter role resolver — S03/S05.
- `--edit-agents`, the missing-skill stderr report and `agents_missing_skill` JSON field —
  S06/S07.
- `uninstallLong`'s skill-removal sentence and the bound-agent uninstall row — S08.
