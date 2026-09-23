# agent-workflow-skill — current state

Scenarios complete: SCENARIO-01, SCENARIO-02, SCENARIO-03. Last updated by SCENARIO-03.

## Binding decisions
- The skill is a separate `host.Host.Skills() []File` list, never an entry in `Plugin()` — `Plugin(false)` would trim it instead of the hook and break doctor's `host-plugin` check. `File.RelPath` is install-root-relative; the skill's own entry is `host.WorkflowSkillDir + "/SKILL.md"`. Setup report kind `setup.KindSkill = "skill"`; artifact kind `artifact.KindSkillWorkflow`. Row order: config, feature-root, plugin…, hook, skill, agent…, bound-agent…(S07), snippet, roles, roles-skill(S05). brief owns `host.PluginDir` and below **and** `host.WorkflowSkillDir`; prune stops at each. Skill arts ride `writeArts`. Uninstall's `--force` removes an edited SKILL.md through `planPluginRemoval` (Rule 8). (SCENARIO-01)
- Older-render upgrade is **generic in `setup`**: `planPluginFile` classifies `OriginOlder` as `ActionMerged`/"updated"; `apply` re-verifies via `verifyFileUnchanged` before writing; `planPluginRemoval` treats `OriginOlder` like `OriginCurrent`; `writableTargets` includes `ActionMerged` rows (R10). Only planner/implementer have a non-empty older-digest list today. (SCENARIO-02)
- `renderAgent` takes optional `skills []string` (block-list form, after `tools`). `artifact.WorkflowSkillName = "brief-workflow"`; reviewer never gets `skills:` (R15). (SCENARIO-02)
- Plain `brief init` (no `--with-agents`) never plans agent files, so it never upgrades an older one either — existing behavior, do not "fix". (SCENARIO-02)
- Role resolution (Rule 5) lives in `internal/platform/agentfile` (`Find(root, home, name) []Definition`) — matches Claude Code's own agent identification: recursive, frontmatter `name:` only, no filename fallback. Doctor and setup may import only `internal/platform/*`; S05–S08 reuse this resolution. `Find` returns every match from **one** scope only (project if any, else user), absolute paths, lexical order, empty slice = not found, no error — unreadable/unparseable files are skipped, not reported. `Definition{Path, Scope, Frontmatter}`; S05 adds `Skills`/`OmitClaudeMd` to `Frontmatter`, S06 maps `Scope` straight to JSON `scope` — extend the struct, never re-parse. (SCENARIO-03)
- `brief:<name>` stays filename-based, unchanged (`<root>/.claude/skills/brief/agents/<name>.md`, overridden by `<root>/.claude/agents/<name>.md`, stat-only, no frontmatter needed) — Rule 5 is scoped to bare names. `resolveRoleBinding`'s result now also carries the resolved path for `brief:*` bindings, since S05's roles-skill must read frontmatter from whichever file resolved, either shape. (SCENARIO-03)
- roles' new duplicate WARN is **project scope only** (`<rel>` root-relative, slash-separated, existing roles fix text unchanged); the new `; user-level: <role>` OK suffix sits beside `; not verified: <role>`, both ordered by `RoleBindings` (planner, implementer, reviewer), one suffix per role. (SCENARIO-03)

## Left unbuilt
- `initLong`'s remaining sentences ("init never edits an agent file of yours… `--edit-agents` …") — S06/S07. `doctorLong`'s roles clause (stale wording, "reading ~/.claude/agents…") — S05 replaces it verbatim. Both hand-wrapped: normalize whitespace before a `Contains` assertion.
- `uninstallLong` does not yet mention the skill — S08's addition.
- doctor's `host-skill` row does not exist yet — S04; `anyIntegrationFilePresent` does not see the skill, S04 decides whether `Skills()` joins.
- `roles-skill` doctor row — S05. An exported single-file parse (`agentfile.Load` or similar) for the `brief:*` path's own frontmatter, and `Frontmatter.Skills`/`Frontmatter.OmitClaudeMd` — both S05.
- The `brief:` / `<plugin>:` / bare prefix split still lives inside doctor's `resolveRoleBinding`. setup cannot import doctor, so S06/S07 must move it down (e.g. into `internal/platform/config`) or re-derive it.
- `bound-agent` (S07's new `setup.Kind`) needs a case in `setup.printArtifacts`' `exhaustive` switch over `setup.Kind`; `artifact.Render`/`digestsFor` switch over `artifact.Kind` instead, needing one only if S07 adds an artifact Kind too. R10's unwritable refusal is exit **1**, not 2 — S07's writability check must match.

## Traps
- `golangci-lint`'s `exhaustive` is on: a `switch` over `artifact.Origin` needs all three cases explicit, not just `default`. A test-body `switch` over `setup.Action` trips the same linter; use an if-chain.
- `sha256.Sum256(AgentPlanner())` in an `older…Digests` list duplicates the current digest and makes `OriginOlder` unreachable while every render-pinned test stays green — the older literal must be an independent fixed string captured before the render changes, never a live call.
- The Gherkin reads `skills: [brief-workflow]` (flow form); the ruled bytes brief's own agents render are a block list. S07's `--edit-agents` shape table must accept *both* forms on a repo's own agent file — only brief's own renders are fixed to the block form.
- yaml.v3 partially fills fields on a `*yaml.TypeError`. "Any Unmarshal error → not a candidate" (Rule 5) is pinned, so `agentfile.findIn` must not use a partial decode.
- When S05 adds `Frontmatter.Skills []string`, a scalar `skills: foo` fails the whole decode and drops the agent out of role resolution entirely (roles flips to "not found" instead of roles-skill WARNing) — decode `skills:` tolerantly (e.g. `yaml.Node`) and classify its shape separately.
- `filepath.WalkDir` does not descend into symlinked directories, but a symlinked `*.md` *file* is read through its link, so `Definition.Path` can resolve outside the repository. S07's Rule 3 ("never edits a file outside the repository") must check the resolved target, not just `Scope`.
- The `brief:reviewer` override fixture in `Test_diagnose_classifies_roles` has no frontmatter on purpose — pins the filename-based `brief:*` override, unaffected by Rule 5.
- User-level duplicate agent definitions resolve silently; nothing WARNs. UNRULED — final product-vision decides.

## Open debts
- `setup.planSnippet`/`planSnippetRemoval` still treat an `OriginOlder` snippet as "edited locally". **Unowned — dies unless re-opened**; the next snippet byte change must fix it first.
- doctor's `host-skill` row (SKIP/WARN/ERROR/OK matrix) — S04, unowned until then.
- `roles-skill` doctor row — S05, unowned until then.
- `--edit-agents`, the missing-skill stderr report and `agents_missing_skill` JSON field — S06/S07.
- `uninstallLong`'s skill-removal sentence and the bound-agent uninstall row — S08.
- UNRULED COPY: R11's `merged <rel> (updated)` row and reusing `ActionMerged` for an older-file upgrade were the developer's own call, not Surface & Copy. Owner: final product-vision, before SHIP.
- UNVERIFIED (product-vision item 8): whether a plugin agent (`.claude/skills/brief/agents/*.md`) resolves a *project* skill by bare name `brief-workflow` — no live Claude Code session to check. Verify before release — unowned; the final gate decides whether this blocks SHIP.
