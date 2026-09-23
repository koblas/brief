# agent-workflow-skill — current state

Scenarios complete: SCENARIO-01, SCENARIO-02. Last updated by SCENARIO-02.

## Binding decisions
- The skill is a separate `host.Host.Skills() []File` list, never an entry in
  `Plugin()` — `Plugin(false)` would trim it instead of the hook and break doctor's
  `host-plugin` check. `File.RelPath` is install-root-relative; the skill's own entry is
  `host.WorkflowSkillDir + "/SKILL.md"`. Setup report kind `setup.KindSkill = "skill"`;
  artifact kind `artifact.KindSkillWorkflow`. Row order: config, feature-root, plugin…,
  hook, skill, agent…, snippet — a later bound-agent row (S07) goes between agent and
  snippet. brief owns `host.PluginDir` and below **and** `host.WorkflowSkillDir`; prune
  stops at each. Skill arts ride `writeArts`; anything planned only into `artifacts`
  reports but writes nothing (S07's bound-agent edits are exactly this shape). Uninstall's
  `--force` removes an edited SKILL.md through `planPluginRemoval`, same contract as every
  other plugin/agent file (Rule 8). (SCENARIO-01)
- Older-render upgrade is **generic in `setup`**, not agent-specific: `planPluginFile`
  classifies `OriginOlder` as `ActionMerged`, detail `updated`, carrying the file's
  pre-write bytes on `pluginArtifact.existing []byte`; `apply` re-verifies those bytes via
  `verifyFileUnchanged` (renamed from `verifySnippetUnchanged` — now the one shared
  read-modify-write guard for CLAUDE.md *and* any `ActionMerged` plugin/skill/agent file;
  S07/S08, writing user agent files, should reuse it) before writing, refusing
  `ErrConcurrentEdit` on drift. `planPluginRemoval` treats `OriginOlder` like
  `OriginCurrent` (removed without `--force`). `writableTargets` includes `ActionMerged`
  rows (R10). Every Kind but the planner/implementer agents still ships an empty
  older-digest list — the next Kind needs only its own literal, no setup.go change.
  `originRow`'s own `OriginOlder` arm (doctor/host.go) needed no code change; it already
  existed and just started firing. (SCENARIO-02)
- `renderAgent` takes an optional `skills []string`, emitted as a `skills:` block list
  (not the Gherkin's flow form) after `tools`, nothing when empty — reviewer bytes stay
  fixed. `artifact.WorkflowSkillName = "brief-workflow"` is the named constant
  `AgentPlanner`/`AgentImplementer` pass. Reviewer never gets `skills:` (R15: read-only
  `brief start`/`brief check` only). Planner/implementer bodies were reworded to the
  ruled Surface & Copy one-liners, not merely appended to. (SCENARIO-02)
- Plain `brief init` (no `--with-agents`) never plans agent files at all, so it never
  upgrades an older one either — existing behavior; doctor's fix already says
  `--with-agents`. Do not "fix" this. (SCENARIO-02)

## Left unbuilt
- `initLong`'s remaining sentences ("init never edits an agent file of yours…
  `--edit-agents` …") — S06 (missing-skill report) / S07 (`--edit-agents` itself).
  `initLong` is hand-wrapped: normalize whitespace before a verbatim `Contains` assertion.
- `uninstallLong` does not yet mention the skill; its ruled addition is S08's.
- doctor's `host-skill` row does not exist yet — S04. `anyIntegrationFilePresent`
  (`internal/doctor/host.go`) does not see the skill; S04 decides whether `Skills()`
  joins that union.
- Role resolver (frontmatter `name:`) — S03. `roles-skill` doctor row — S05.
- `bound-agent` (S07's new `setup.Kind`) needs a case in every `exhaustive`-checked `Kind`
  switch: `artifact.Render`, `digestsFor`, `setup.printArtifacts`. R10's unwritable
  refusal is exit **1**, not 2 — S07's agent-file writability check must match.

## Traps
- `golangci-lint`'s `exhaustive` is on: a `switch` over `artifact.Origin` needs all three
  cases (`OriginCurrent`/`OriginOlder`/`OriginEdited`) explicit, not just `default` — hit
  in both `setup.go`'s `planPluginFile` and `uninstall.go`'s removal check. A test-body
  `switch` over `setup.Action` trips the same linter; use an if-chain.
- `sha256.Sum256(AgentPlanner())` in an `older…Digests` list duplicates the current digest
  and makes the `OriginOlder` arm unreachable while every render-pinned test stays green —
  the older literal must be an independent fixed string, captured (`%q` dump) *before* the
  render changes, never a live call. Mutation-verified: emptying
  `olderAgentPlannerDigests` reddens exactly the doctor "an older planner render" case.
- The Gherkin reads `skills: [brief-workflow]`, but the ruled bytes brief's own agents
  render are a block list. S07's `--edit-agents` shape table must accept *both* forms when
  editing a repo's own agent file — only brief's own renders are fixed to the block form.
- `setup.planSnippet`/`planSnippetRemoval` still treat an `OriginOlder` snippet as "edited
  locally", though `ActionMerged`'s own doc reads as if an older block gets replaced — it
  does not; unreachable today since `olderSnippetTemplates` is empty. Out of scope here.
  **Unowned — dies unless re-opened**; the next snippet byte change must fix it first.

## Open debts
- doctor's `host-skill` row (SKIP/WARN/ERROR/OK matrix) — S04, unowned until then.
- `roles-skill` doctor row and the frontmatter role resolver — S03/S05.
- `--edit-agents`, the missing-skill stderr report and `agents_missing_skill` JSON field —
  S06/S07.
- `uninstallLong`'s skill-removal sentence and the bound-agent uninstall row — S08.
- UNRULED COPY: R11's `merged <rel> (updated)` row and reusing `ActionMerged` for an
  older-file upgrade were the developer's own call, not Surface & Copy. Owner: the final
  product-vision pass, before SHIP.
- UNVERIFIED (product-vision item 8): whether a plugin agent
  (`.claude/skills/brief/agents/*.md`) resolves a *project* skill by bare name
  `brief-workflow`. Still unchecked — no live Claude Code session available; the bodies
  keep inline `brief start`/`finish`/`new step` invocations so the agents still work if it
  does not. Verify in a live session before release — unowned; the final gate decides
  whether this blocks SHIP.
