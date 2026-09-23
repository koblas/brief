---
id: SCENARIO-06
status: done
---

# SCENARIO-06: init reports bound agents missing the skill

## Scenario

Scenario: SCENARIO-06 init reports bound agents missing the skill
  Given a bound implementer agent without the skill
  When I run "brief init" without --edit-agents
  Then stderr lists the agent path and role, the file is untouched, exit 0
  And --json carries "agents_missing_skill" (always present, never null)

## User-visible contract

- `brief init [--host claude-code] [--dry-run]`: stdout carries the artifact rows, unchanged.
  Stderr order is fixed: the `roles_to_add` block (when present), then the missing-skill block,
  then the next-action line. The missing-skill block is printed only when at least one agent is
  listed. Its header is exactly
  `brief init: bound agents do not preload the brief-workflow skill; add "brief-workflow" to the "skills:" list in each, or rerun with --edit-agents:`.
  After the header comes one line per lacking agent file:
  - project scope: `  <displayPath(wd, path)> (<role>)`
  - user scope: `  ~/<home-relative slash path> (<role>; user-level, edit by hand)`
- Exit code is 0. No agent file is read for writing, and none is touched.
- `--dry-run`: the same block, and nothing is written.
- `--print` (text or `--json`): no block and no field, following the `roles_to_add` precedent.
- `--json`: `agents_missing_skill` comes right after `roles_to_add`. It is always present, and
  it is `[]` when nothing is listed. Each item is `{"role","agent","path"(absolute),"scope"("project"|"user")}`.
- `--host none` (explicit or detected): the list is always empty, because there is no skill to
  preload.
- Refusal paths (unknown host, invalid config, unwritable, partial write) print no block.

The home directory is found once, through `s.homeDir` (`setup.WithHomeDir`). If it returns an
error, the home is treated as `""`, the same rule doctor uses.

## Implementation Plan

### Red
- [x] Step 1: `internal/platform/agentfile/agentfile_test.go` `Test_resolve_binding_classifies_by_prefix` — table of `""` / `brief:x` (override beats plugin path; neither → unresolved) / `other:x` / bare-name resolved / bare-name unresolved, asserting `Kind`, `State` and `Path`. Fails: `ResolveBinding` and the kind/state constants do not exist.
- [x] Step 2: `internal/platform/agentfile/agentfile_test.go` `Test_lacking_skill_returns_each_definition_without_it` — cases: bare-name duplicates where one has the skill and one does not (only the second is returned); a `brief:*` file without the skill (one Definition, Scope project); a `brief:*` path with no frontmatter (one Definition, zero Frontmatter); a binding that is unbound, unresolved or unverified (nil). Fails: `(Binding).LackingSkill` does not exist.
- [x] Step 3: `internal/setup/missing_skill_test.go` `Test_init_reports_bound_agents_missing_the_workflow_skill` — table through `Server.Init` with home injected via `WithHomeDir(tempdir)`. Rows:
  - bare project agent lacking the skill: listed as {Role, Agent, Path, Scope project}, and its bytes are byte-identical afterwards.
  - the same agent with `skills: [brief-workflow]`: not listed (control).
  - user-scope-only agent: listed with Scope user and a home-relative `ScopeRelPath`.
  - `brief:implementer` whose `.claude/agents/implementer.md` override lacks the skill: not listed. Its control arm is a bare `implementer` binding to identical file content, which is listed, and the two rows differ only in the binding value. The file **must** carry frontmatter `name: implementer`. Without it the bare arm is unresolved, which is also "not listed", and mutation (b) proves nothing.
  - unbound, unresolved, or `other:x`: not listed.
  - reviewer bound to a lacking agent: not listed.
  - two project duplicates for one name: each lacking file listed, in lexical order.
  - planner and implementer both bound to the same lacking agent: two items, same path, roles planner then implementer.
  - `DryRun`: listed, and nothing is written.
  - `Host: HostNone` with a lacking bound agent: empty, non-nil.
  - `Force` rewriting an invalid config that bound a lacking agent: empty, because the report follows `planConfig`'s post-run config.

  Fails: `Result.AgentsMissingSkill` does not exist, so nothing is listed.
- [x] Step 4: `internal/cli/init_internal_test.go` `Test_init_lists_bound_agents_missing_the_workflow_skill` — `run(... "init","--host","claude-code" ...)` with a home seam holding a user-level `planner` agent and a project `.claude/agents/developer/Agent.md` bound as implementer. Asserts the exact stderr: header, then the `.claude/agents/developer/Agent.md (implementer)` line, then the `~/.claude/agents/planner.md (planner; user-level, edit by hand)` line, then the next-action line. Also asserts exit nil and the agent file byte-identical. A sub-case with `--dry-run` asserts the same block. Fails: stderr has no block.
- [x] Step 5: `internal/cli/init_internal_test.go` `Test_init_prints_roles_to_add_before_the_missing_skill_block` — `--with-agents` over a kept config binding only `implementer: developer` (lacking). Asserts exact stderr in the order `roles_to_add` block, missing-skill block, next-action line. Fails: the block is absent. After green, this pins the order.
- [x] Step 6: `internal/cli/init_json_test.go` `Test_init_json_is_one_exact_document` — extend the expected tail to `…"roles_to_add":[],"agents_missing_skill":[]}`. Fails: the field is absent (this is the never-null pin).
- [x] Step 7: `internal/cli/init_internal_test.go` `Test_init_json_lists_agents_missing_the_skill` — `--json` with the Step 4 fixture. Decodes `agents_missing_skill` as items with exactly the keys `role`, `agent`, `path` (absolute) and `scope` in that order, with values `project`/`user`, and asserts stderr is empty. Fails: the field is absent.
- [x] Step 8: `internal/cli/init_json_test.go` `Test_init_print_json_is_one_exact_document` — add `NotContains "agents_missing_skill"`. This is green on arrival, because the print document never had the field. Keep it as a regression pin and say so.
- [x] Step 9: `internal/cli/help_test.go` `Test_init_help_names_the_brief_workflow_skill` — also assert the whitespace-normalized sentence `init never edits an agent file of yours by default; stderr instead lists each planner or implementer bound in ".brief.yaml" whose agent lacks it.` and that the help names `agents_missing_skill`. Fails: both are absent.

### Green
- [x] Step 10: `internal/platform/agentfile/binding.go` — add these exported names:
  - `BindingKind`, with `BindingBare`, `BindingBrief` and `BindingPlugin`.
  - `BindingState`, with `BindingUnbound`, `BindingResolved`, `BindingUnresolved` and `BindingUnverified`.
  - `Binding{Kind, State, Path, Defs}`.
  - `ResolveBinding(root, home, value string) Binding`, which moves doctor's `resolveRoleBinding` logic here, including the `brief:*` override/plugin-path rule via `host.PluginDir` and an unexported `fileIsRegular`.
  - `(Binding) LackingSkill(skill string) []Definition`: this is the one decision point. For a `brief:*` binding it synthesizes a Definition from `Load`, with zero Frontmatter when `Load` fails.
- [x] Step 11: `internal/doctor/host.go` — `resolveRoleBinding` becomes a wrapper that resolves home and calls `agentfile.ResolveBinding`. `roleResolution` and the `role*` states give way to `agentfile.Binding`/`BindingState`. `roleLacksSkill` derives from `LackingSkill`: non-empty means lacking, and omitClaudeMd is true when any returned Definition sets it. Delete doctor's `fileIsRegular`. `rolesCheck`/`rolesSkillCheck` behaviour stays the same, and the existing doctor tests guard that.
- [x] Step 12: `internal/setup/missing_skill.go` — add `MissingSkillAgent{Role, Agent, Path, Scope agentfile.Scope, ScopeRelPath}` and the unexported `agentsMissingSkill(root, home string, roles config.RoleBindings) []MissingSkillAgent`. It walks planner, then implementer, keeps only `Kind == agentfile.BindingBare` resolved bindings, and emits one item per `LackingSkill(artifact.WorkflowSkillName)` Definition. The result is never nil.
- [x] Step 13: `internal/setup/setup.go` — add `Result.AgentsMissingSkill`. `Init` fills it from `planConfig`'s `cfg.Roles` only when the resolved host is `HostClaudeCode`, and fills it before the DryRun/Print return so both carry it. It is empty in every other case, and `Uninstall`'s Result sets it to empty and non-nil.
- [x] Step 14: `internal/cli/init.go` — add the `missingSkillJSON{Role, Agent, Path, Scope}` DTO and `initDocument.AgentsMissingSkill` (`json:"agents_missing_skill"`), placed after `RolesToAdd`. Add `renderAgentsMissingSkill(w, wd, agents)`, called in `runInit` right after the `roles_to_add` block and before the NoHostDetected/next-action lines. Add the `initLong` sentence after the "…frontmatter "skills:"." sentence, and append `"agents_missing_skill"` to its `jsonFieldsParagraph`.

### Sweep
- [x] Step 15: fix what `go build ./... && golangci-lint run ./...` reports (sweep)
- [x] Step 16: `internal/platform/agentfile/doc.go` — widen "only locates and decodes" to cover binding classification and the lacks-skill decision (sweep)
- [x] Step 17: `internal/setup/doc.go` + `Result` doc — add `internal/platform/agentfile` to the import list, and document `AgentsMissingSkill` (claude-code only, bare-name planner/implementer, post-run config, never nil, computed under DryRun/Print) (sweep)
- [x] Step 18: `internal/cli/*init*_test.go`, `internal/setup/*_test.go` — grep for existing claude-code init runs that bind a bare role absent from the project. Init now calls `agentfile.Find` for those, so a test without an injected home reads the real `~/.claude/agents`. Inject home, or confirm the test does not pin stderr or the whole `Result`. Also bump any whole-`Result` `assert.Equal` literal for the new non-nil empty `AgentsMissingSkill`, because neither build nor lint catches that (sweep)
- [x] Step 19: `internal/doctor/host.go` comments and `internal/cli/init.go` `initDocument` doc — drop references to the moved `resolveRoleBinding` internals, and name `agents_missing_skill` (sweep)

### Verify
- [x] Step 20: full verification per `.claude/rules/agent-briefs.md`. Report the test-count delta. Run these mutations one at a time, and restore each byte-identically:
  - (a) invert the has-skill test inside `agentfile.(Binding).LackingSkill`. Both `Test_diagnose_classifies_roles_skill` and `Test_init_reports_bound_agents_missing_the_workflow_skill` must go red, which proves there is a single decision point.
  - (b) drop the `Kind == agentfile.BindingBare` filter in `agentsMissingSkill`. The `brief:*` exclusion row of `Test_init_reports_bound_agents_missing_the_workflow_skill` must go red while its bare-name control stays green.
  - (c) swap the `roles_to_add` and missing-skill blocks in `runInit`. `Test_init_prints_roles_to_add_before_the_missing_skill_block` must go red.

## Handoff

**Binding decisions:**
- Binding classification and the lacks-skill decision live in `internal/platform/agentfile`: `ResolveBinding`, `Binding{Kind, State, Path, Defs}`, and `(Binding).LackingSkill(skill) []Definition`. Doctor's roles/roles-skill and setup's report both call them, because setup cannot import doctor. Any change to "lacks the skill" goes only in `LackingSkill`.
- init's report covers **bare-name planner/implementer bindings only** (`Kind == BindingBare`). It never covers `brief:*`, other-plugin, or the reviewer (R15). Three things force this: the header advises `--edit-agents`, which never edits `brief:*` (Rule 4); JSON `scope` is exactly `Definition.Scope`; and plain init never upgrades an older plugin agent, so listing it would advise hand-editing a brief-owned file. Doctor's roles-skill still WARNs on `brief:*`, so the two commands intentionally cover different sets.
- Setup filters on the explicit `Kind`, never on `len(Defs) > 0`, because `LackingSkill` synthesizes a Definition for `brief:*`. S07's `--edit-agents` target set uses the same `BindingBare` filter.
- The report comes from `planConfig`'s returned `cfg.Roles`, which is the post-run config, never a re-read of disk. There is one item per lacking Definition per role, planner before implementer, and within a role in Find's lexical order. Duplicate paths across roles are listed twice, because JSON `role` is a single string.
- The report is computed only when the host is claude-code. It is empty and non-nil otherwise.
- Stderr order: `roles_to_add` block, then the missing-skill block, then the next-action/no-host line. JSON `agents_missing_skill` follows `roles_to_add`. Neither is rendered under `--print`.
- Display: project paths use `displayPath(wd, Path)` (wd-relative, like every init row). User paths are `"~/" + ScopeRelPath`. `ScopeRelPath` is not serialized, and the JSON DTO `missingSkillJSON` is separate from `setup.MissingSkillAgent`.
- `initLong` now carries the "init never edits an agent file of yours by default…" sentence. The `--edit-agents …` sentence still waits for S07.

**Left unbuilt:**
- `--edit-agents` flag, `bound-agent` kind/rows, and the shape table: S07.
- The `initLong` sentence `--edit-agents adds it to those agents' "skills:" lists, …`: S07.
- Uninstall's skill-entry removal and `uninstallLong` addition: S08.

**Traps:**
- The ruled header already says "or rerun with --edit-agents", even though the flag lands in S07. This is ruled copy, so do not trim it.
- In S06, "file untouched" is unfalsifiable because no agent write path exists yet. S07 must add the control arm, where the same fixture with `--edit-agents` changes the file.
- S07 must compute the report *after* planning its edits and subtract the agents it will edit, including under `--dry-run` ("after the planned run").
- Every new setup/cli test must inject home (`WithHomeDir` / a `withSetupOpts` seam). Otherwise a developer's real `~/.claude/agents/<name>.md` flips the unresolved and user-scope rows. `cli.Run` exported tests cannot inject home, so use `init_internal_test.go`.
- UNRULED: init's bare-name-only report deliberately differs from doctor's roles-skill, which also WARNs on `brief:*`. Recorded as an open debt for final product-vision.
- `agentfile` now imports `internal/platform/host` for `PluginDir`. Neither `host` nor `artifact` may ever import `agentfile`, or an import cycle results.
