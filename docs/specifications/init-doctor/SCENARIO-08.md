---
id: SCENARIO-08
status: done
---

# SCENARIO-08: --with-agents scaffolds three role agents and binds them

## Scenario

```gherkin
Scenario: SCENARIO-08 --with-agents scaffolds three role agents and binds them
  When I run "brief init --host claude-code --with-agents" in a fresh repo
  Then agents planner, implementer and reviewer are written under the plugin directory, and the new .brief.yaml binds roles to brief:planner, brief:implementer and brief:reviewer
  And an existing config is never edited; stderr says which roles lines to add
```

Rules in play: R4 (agents live in the plugin dir), R6 (ownership, edited kept), R7 (roles),
R11 (output, `kind` = `agent`), brief R15 as amended (three positions, reviewer = calls into
the tool only).

## User-visible contract

- `brief init --host claude-code --with-agents` (fresh repo) → stdout rows in this order:
  config, feature root, manifest, start skill, finish skill, hook, `agents/planner.md`,
  `agents/implementer.md`, `agents/reviewer.md`, CLAUDE.md block; all `created`. Exit 0.
  The new `.brief.yaml` has every key commented except `roles:` and its three children,
  which are live: `planner: brief:planner`, `implementer: brief:implementer`,
  `reviewer: brief:reviewer` (doc `# ` lines stay above each).
- Existing config (kept, unchanged, or an unrecognized-but-valid one) → never edited. Stderr,
  before the next-action line: `brief init: <rel .brief.yaml> was not edited; to bind brief's
  agents, add these lines to it:` then one line each: `roles:` and `  <role>: brief:<role>` for
  every role that is **unbound** (empty) in the kept config. A role already bound to anything
  (brief's agent or the adopter's own) is never listed — R15 "never shadows an existing
  agent". No unbound role → no hint at all. Printed under `--dry-run` too.
- `--json`: stderr stays empty (existing `init_json_test.go` assertions hold). The document
  gains `roles_to_add: [<the same lines>]`, always present, `[]` when nothing to add.
  Rows carry `kind: "agent"`.
- `--force --with-agents`: the rewritten config is the bound variant (init authored it this
  run); `unchanged` only when the existing bytes equal *exactly* the bound variant.
- Existing `agents/<role>.md` whose bytes match no known render → `kept (edited locally)`,
  `--force` included; not a regular file → `kept (not a regular file)`.
- Without `--with-agents`: no agent rows, nothing written, an existing brief-installed
  `agents/` left untouched (mirrors `--no-hook`'s "no plan, no row").
- `--with-agents` with resolved host `none` → usage error, exit 2, no files changed:
  `brief init: --with-agents requires --host claude-code; run 'brief init --host claude-code --with-agents'`.
  Checked on the **resolved** host (so today a bare `brief init --with-agents` is this error;
  S09's detection may resolve it to claude-code and make it succeed — intended).
- `brief uninstall [--host claude-code]` always plans agent removal (no flag): recognized
  agent files `removed`, edited kept unless `--force`, `agents/` pruned when empty. The bound
  config variant is recognized and removed like the plain one.

## Decisions taken in this plan

- Agent frontmatter keys `name`, `description`, `tools` (comma-separated tool names),
  `model`: locally evidenced by this repo's own `.claude/agents/*/Agent.md`. The external doc
  (code.claude.com/docs/en/sub-agents) could NOT be fetched while planning (permission
  denied twice) — Step 0 verifies it, including the plugin-agent namespacing the binding
  values `brief:<role>` depend on. Pinned shape (adjust only if Step 0 contradicts it):
  `name: <role>`, a one-line `description`; no `model` key (inherit the session's model);
  `tools`: planner `Read, Grep, Glob, Bash, Edit, Write` (fills plan files), implementer no
  `tools` key (inherits all), reviewer `Read, Grep, Glob, Bash` (no Edit/Write — read-only).
  Bodies thin, only calls into the tool: planner — turn the specification into steps with
  `brief new step <feature>`, fill each plan, never write code; implementer — `brief start
  <feature>`, work from its output, close with `brief finish <feature> <step> --handoff
  <path> --state <path>`; reviewer — `brief start <feature>` read-only, `brief check
  <feature>`, report; no review policy or persona.
- Bound config variant = a second render (`artifact.ConfigFileWithRoles`) whose digest is
  **appended to `configDigests`**; `Render(KindConfig)` still returns the plain `ConfigFile()`.
  Recognition at `planConfig`, `planConfigRemoval` and re-run needs no second Kind. The
  `--force` "unchanged" test becomes an exact-bytes compare against the variant this run
  writes (not `Recognize`), else `--force --with-agents` over a plain render would report
  `unchanged` and bind nothing.
- The binding values live in `artifact` (`AgentBindings() config.RoleBindings`) — artifact
  already owns the plugin name `brief`, and `host` imports `artifact`, never the reverse.
- `host.Host` gains `Agents() []File` (three files, `PluginDir + "/agents/<role>.md"`); the
  `Plugin(withHook bool)` signature stays (STATE binding). Init plans agents after the plugin
  files, reusing `planPluginFile`; Uninstall plans them first among plugin files (reverse
  write order), reusing `planPluginRemoval`.
- Adding `RoleBindings.Reviewer` changes `config.Default()` → `ConfigFile()` bytes → its one
  digest. The pre-S08 render is **not** appended (brief is unreleased; the old render is not
  reproducible from code). A `.brief.yaml` written by a pre-S08 build of this branch now
  reads `edited locally`.
- Open debt closed by declining: `roles.*` values are free-form (any string, empty = unbound)
  and `optional-conventions` stays unvalidated at load; `doctor`'s `roles` row (S10) reports
  unbound positions. Stated in R1 by Step 16.
- Flag-combination rule lives in `setup` as a sentinel (`ErrAgentsNeedHost`), mapped to a
  usage error by `cli` exactly like `ErrUnknownHost`.

## Implementation Plan

- [x] Step 0: verify against code.claude.com/docs/en/sub-agents and /plugins-reference, three separate checks: (i) file layout — does a plugin's `agents/` take `<name>.md` (flat) or `<name>/Agent.md`? (this repo's own `.claude/agents/<name>/Agent.md` is a project-local layout, not evidence for plugins); (ii) frontmatter keys (`name`, `description`, `tools` format, `model` optional/inherit); (iii) a plugin agent is addressed as `brief:<name>`. If the fetch is blocked, the parent's pin wins — flat `agents/<role>.md`, `brief:<role>` — say so in the handoff and keep the pinned shape above; record the result as an "amended during SCENARIO-08 planning" note in R4/R7 (verify)
  **Result (developer, SCENARIO-08):** confirmed externally (code.claude.com/docs/en/sub-agents): a plugin's agents are single flat files `agents/<name>.md` — not `<name>/Agent.md`; that nested shape is a project-local convention (`.claude/agents/<name>/Agent.md`), never plugin layout. Frontmatter: `name` (required, lowercase+hyphens, no `:`) and `description` (required); `tools` optional, comma-separated string or YAML list; `model` optional (sonnet/opus/haiku/fable/full id/inherit) — omitted here so the session's own model is inherited. A plugin agent is addressed `<plugin>:<path>`, so `agents/planner.md` in plugin "brief" is `brief:planner`, matching `artifact.AgentBindings()`. Plugin agents ignore hooks/mcpServers/permissionMode. A project `.claude/agents/planner.md` of the same name overrides a plugin agent — relevant to S10's `roles` doctor check, which must not assume `brief:planner` is what actually runs if the repository also defines its own `.claude/agents/planner.md`. The pinned shape (flat `agents/<role>.md`, `brief:<role>`, no `model` key) stands unchanged.
- [x] Step 1: `internal/platform/config/config_test.go` `Test_a_config_binding_the_reviewer_role_decodes` — `roles.reviewer` decodes, no violation (red: `KnownFields(true)` rejects it)
- [x] Step 2: `internal/platform/config/config.go` `RoleBindings.Reviewer` (`yaml:"reviewer"`) + doc comment "three positions" (green)
- [x] Step 3: `internal/platform/artifact/artifact_test.go` `Test_every_config_key_is_documented` goes red on `reviewer` → `configFieldDocs["reviewer"]` in `artifact/config.go` (red → green)
  **Note (developer, SCENARIO-08):** the test's own discriminator (`strings.HasPrefix(line, "# ")`) was fooled by YAML's 2-space indent under `roles:` — an indented value line (`#  reviewer: ""`) coincidentally starts with `# ` and was treated as an already-satisfied doc line, so it never actually went red on the missing entry. Replaced the discriminator with `looksLikeKeyValue` (mirrors `configFileLineKey`'s own key-char rule from the test side) so a value line is identified structurally, not by prefix. Verified red before the doc-entry fix, green after.
- [x] Step 4: `internal/platform/artifact/agents_test.go` `Test_agent_files_are_thin_and_limited_to_brief_calls` — table over the three renders: frontmatter keys per the pin, reviewer's `tools` carries neither Edit nor Write, each body names its brief invocations, planner body names `brief new step` (red)
  **Note (developer, SCENARIO-08):** split into four focused tests rather than one mixed-assertion table (`go-testing`'s "one assertion shape per table" rule): frontmatter name/description/no-model (table, 3 roles), planner+reviewer tools (table, exact string), implementer's absent tools key (single test), and body invocations (table, 5 cases, one `Contains` each).
- [x] Step 5: `internal/platform/artifact/agents.go` — `AgentPlanner`/`AgentImplementer`/`AgentReviewer` renders + `KindAgentPlanner`/`KindAgentImplementer`/`KindAgentReviewer`; add the three arms to `Render`'s switch in `plugin.go` (a missing arm returns nil → init writes zero-byte agent files) (new)
- [x] Step 6: `internal/platform/artifact/digest.go` — three agent digest lists + `digestsFor` cases; extend `Test_recognize_classifies_each_plugin_file_against_its_own_kind` with the agent kinds (cross-kind never matches) (red → green)
- [x] Step 7: `internal/platform/artifact/artifact_test.go` `Test_the_bound_config_file_binds_only_the_roles` — as written decodes to `config.Default()` with `Roles == AgentBindings()` and no violations; every non-`roles` line stays commented; `Recognize(KindConfig, …)` is `OriginCurrent` for both variants; `Render(KindConfig)` still equals `ConfigFile()` (red)
- [x] Step 8: `internal/platform/artifact/config.go` `ConfigFileWithRoles`, `AgentBindings`; `digest.go` append its digest to `configDigests`; correct `doc.go`, `Render`, `Kind`, `configDigests` doc comments (Render no longer the only recognized body for `KindConfig`) (green)
- [x] Step 9: `internal/platform/host/claudecode_test.go` `Test_claude_code_lists_three_agent_files` — paths under `PluginDir/agents/`, kinds, order planner, implementer, reviewer (red) → `host.Host.Agents()` on the interface + `claudeCode` (green)
- [x] Step 10: `internal/setup/agents_test.go` — Server-level tests against a `t.TempDir` repo (red): `Test_init_with_agents_writes_three_agents_and_a_config_binding_them` (rows, order, kind `agent`, config bytes == `ConfigFileWithRoles()`, `RolesToAdd` empty); `Test_rerunning_init_with_agents_reports_every_agent_unchanged`; `Test_init_keeps_an_unrecognized_agent_file_even_under_force`; `Test_init_with_agents_never_edits_an_existing_config` (table: plain render → unchanged, edited-valid → kept; bytes identical after; `RolesToAdd` lists header + all three); `Test_roles_to_add_lists_only_unbound_roles` (one role bound to the adopter's agent, one to brief's → only the third listed; all bound → empty); `Test_roles_to_add_lines_parse_into_brief_bindings` (control arm for the hint: append `RolesToAdd` to a config with no `roles:` key, `config.Inspect` it → `Roles == artifact.AgentBindings()`, no violations — proves the advice works, and whether values render quoted); `Test_init_with_agents_on_a_bound_config_reports_it_unchanged_with_nothing_to_add`; `Test_force_init_with_agents_rewrites_a_plain_config_to_the_bound_variant`; `Test_init_without_agents_leaves_installed_agents_alone` (control arm: same fixture with `WithAgents` reports three rows); `Test_init_with_agents_for_host_none_refuses_and_writes_nothing` (`ErrAgentsNeedHost`, tree unchanged); `Test_init_with_agents_dry_run_writes_nothing_but_reports_roles_to_add`
- [x] Step 11: `internal/setup/setup.go` — `InitRequest.WithAgents`, `Kind` `KindAgent`, `Result.RolesToAdd` (never nil), `ErrAgentsNeedHost` in `errors.go`; `Init` plans `h.Agents()` via `planPluginFile` into the plugin list; `planConfig` writes/returns the variant and the *decoded* roles of a kept config AND of the `ActionUnchanged` (recognized) branch, which today returns `config.Default()` — mandatory, or a bound file reads as unbound; `configFileCurrent` compares exact bytes of the variant; `rolesToAdd` helper; update `Init`/`Result`/package doc for the new order (green)
  **Note (developer, SCENARIO-08):** went green on first attempt (no unexpected red) — production changes matched the plan's design exactly: `planConfig` now always decodes via `decodeCurrentConfig` on the `ActionUnchanged` path instead of returning `config.Default()`, and `configFileCurrent` compares exact bytes against the desired variant (not `Recognize`) so `--force --with-agents` correctly distinguishes "already bound" from "recognized but plain".
- [x] Step 12: `internal/setup/uninstall_test.go` `Test_uninstall_removes_agents_and_prunes_the_agents_directory`, `Test_uninstall_keeps_an_edited_agent_unless_forced`, `Test_uninstall_removes_a_bound_config`; `internal/setup/roundtrip_test.go` `Test_init_with_agents_then_uninstall_leaves_the_tree_as_before` (assert post-init snapshot differs first; `.claude/skills/` and `.claude/` survive empty) (red)
  **Note (developer, SCENARIO-08):** `Test_uninstall_removes_a_bound_config` and the roundtrip test went green on arrival — both exercise the `ConfigFileWithRoles` digest and agent-removal wiring already built correctly in Steps 8/11/13; kept as regression coverage (mutation (c)/(f) in Step 17 prove they are not vacuous).
- [x] Step 13: `internal/setup/uninstall.go` — plan `h.Agents()` reversed ahead of `h.Plugin(true)` reversed, kind `KindAgent`; `pluginPruneDirs` gains `host.PluginDir/agents` before `host.PluginDir`; doc updates (green)
- [x] Step 14: `internal/cli/init_test.go` (red): `Test_init_with_agents_installs_three_agents_and_binds_roles` (stdout rows, exit 0); `Test_init_with_agents_over_an_existing_config_prints_the_roles_lines_to_add` (exact stderr copy, hint before next action, config bytes unchanged); `Test_init_with_agents_and_host_none_is_a_usage_error` (exact copy, exit 2, tree unchanged; also bare `init --with-agents`); `internal/cli/init_json_test.go` `Test_init_with_agents_json_reports_agent_rows_and_roles_to_add` (kind `agent`, `roles_to_add` array, stderr empty); existing JSON tests assert `roles_to_add: []`; `help_test.go` golden usage gains `[--with-agents]`; update any `help_json_test.go`/help golden pinning init's usage or long text (it lists `init` at line 162)
- [x] Step 15: `internal/cli/cli.go` — `--with-agents` flag + `withAgentsFlagUsage`, usage string `init [--host <name>] [--no-hook] [--with-agents] [--dry-run] [--force] [--json]`; `internal/cli/init.go` — pass `WithAgents`, map `ErrAgentsNeedHost` to the usage error, render the hint to stderr in text mode, `initDocument.RolesToAdd` (`roles_to_add`), `initLong` prose + `jsonFieldsParagraph` field (green)
  **Note (developer, SCENARIO-08):** the plan's literal Use string (with `[--json]` on the end) overflows `Test_every_leaf_help_line_fits_in_80_columns` (87 > 80 columns) — a plan defect, fixed by dropping `[--json]` from init's own Use line only, matching existing precedent (`check`'s and `new`'s Use lines already omit already-registered `--json`); the flag itself stays fully registered, documented in the Flags table and in `initLong`'s JSON paragraph. Separately, `withAgentsFlagUsage`'s first-draft second line started with the literal token `--host`, which `Test_help_json_entries_agree_with_each_commands_text_help`'s own flag-table scraper misread as a second flag row (caught by that test, not by me first) — reworded to avoid a continuation line starting with `--`.
- [x] Step 16: `docs/specifications/init-doctor/specification.md` — amend R1 (roles.* and optional-conventions free-form, unvalidated at load), R7 (unbound-only hint, exact stderr copy, `--force` writes the bound variant, host-none usage error on the resolved host, no-flag leaves agents alone), R11 (`roles_to_add` in init's JSON) (update)
- [x] Step 17: mutation verification — copy each file to `$TMPDIR` before mutating and `cp` it back, then `diff` to prove byte-identical; **never `git stash`** (the stash stack is shared across worktrees). One guard at a time, each surgical and still compiling: (a) drop the `ErrAgentsNeedHost` check → host-none tests red; (b) plan agents unconditionally → `Test_init_without_agents_leaves_installed_agents_alone` red; (c) remove the `ConfigFileWithRoles` digest from `configDigests` → `Test_uninstall_removes_a_bound_config` + rerun-unchanged red; (d) make `rolesToAdd` list bound roles too → `Test_roles_to_add_lists_only_unbound_roles` red; (e) revert `configFileCurrent` to `Recognize` → force-rewrite test red; (f) drop `agents` from `pluginPruneDirs` → uninstall prune test red; (g) give the reviewer render `Edit` → agents render test red. Report which test each mutation reddened
  **Report (developer, SCENARIO-08):** all 7 ran, each `cp`-backed up to `$TMPDIR`, restored via `cp` back (never `git stash`), `diff` confirmed byte-identical after each restore.
  (a) `internal/setup/setup.go`, dropped the `WithAgents`/`ErrAgentsNeedHost` guard → reddened `Test_init_with_agents_for_host_none_refuses_and_writes_nothing` (setup) and both subtests of `Test_init_with_agents_and_host_none_is_a_usage_error` (cli).
  (b) `internal/setup/setup.go`, made `planAgentFiles` run unconditionally → reddened `Test_init_without_agents_leaves_installed_agents_alone`.
  (c) `internal/platform/artifact/digest.go`, dropped `ConfigFileWithRoles()`'s digest from `configDigests` → reddened `Test_uninstall_removes_a_bound_config` and `Test_rerunning_init_with_agents_reports_every_agent_unchanged`.
  (d) `internal/setup/setup.go`, made `rolesToAdd`'s loop append unconditionally → reddened both subtests of `Test_roles_to_add_lists_only_unbound_roles`.
  (e) `internal/setup/setup.go`, reverted `configFileCurrent` to `artifact.Recognize(...)==OriginCurrent` → reddened `Test_force_init_with_agents_rewrites_a_plain_config_to_the_bound_variant`.
  (f) `internal/setup/uninstall.go`, dropped `agents/` from `pluginPruneDirs` → reddened `Test_uninstall_removes_agents_and_prunes_the_agents_directory`.
  (g) `internal/platform/artifact/agents.go`, added `Edit` to `agentReviewerTools` → reddened exactly the `reviewer` subtest of `Test_planner_and_reviewer_agents_scope_their_own_tools` (the `planner` subtest stayed green, proving the two cases discriminate independently).
  (h, added post-review — advisor caught this one, the plan's own Step 11 called it mandatory and no mutation had proven it): `internal/setup/setup.go`, replaced `decodeCurrentConfig(nearest)` in `planConfig`'s non-force `ActionUnchanged` branch with `config.Default()` → reddened `Test_init_with_agents_on_a_bound_config_reports_it_unchanged_with_nothing_to_add` (a fully-bound config reported all three roles as still needing to be added). The force-branch call to `decodeCurrentConfig` is separately exercised by `Test_force_over_the_current_render_reports_unchanged` (plain variant) and `Test_init_keeps_an_unrecognized_agent_file_even_under_force` (bound variant, `Force: true, WithAgents: true`) — both already existing before this pass, confirmed by inspection rather than a further mutation.
- [x] Step 18: `go build ./...`, `go test ./...` (unpiped, report count + delta), `go test -race ./internal/setup/... ./internal/cli/...`, `golangci-lint run ./...` all green → mark SCENARIO-08 done in specification.md
  **Report (developer, SCENARIO-08):** `go build ./...` clean. `go test ./...` unpiped: exit 0, 12 packages ok (STATE.md's own package count), 0 `--- FAIL`, 0 `--- SKIP`, 814 `--- PASS` (verbose count) — up from the pre-scenario baseline by the tests this scenario added (config: +1, artifact: +2 files/~16 cases, host: +1, setup: two new files/~20 cases, cli: +6). `go test -race` on `internal/setup`, `internal/cli`, `internal/platform/artifact`, `internal/platform/host`, `internal/platform/config` all green. `golangci-lint run ./...`: one `prealloc` finding on the `append(pluginArts, agentArts...)` call in `Init`, fixed by building a properly-preallocated `writeArts` slice instead of appending onto the reused `pluginArts`; 0 issues after.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `KindConfig` recognizes two bodies: `ConfigFile()` (plain) and `ConfigFileWithRoles()`
  (roles live); `Render(KindConfig)` is the plain one. S09's `--print` must emit whichever
  variant the run would write; S10's `host-agents`/`roles`/`OriginOlder` build on the two-digest list
- Binding values come only from `artifact.AgentBindings()` (`brief:planner`,
  `brief:implementer`, `brief:reviewer`) — doctor's `roles` row compares against it
- `host.Host.Agents() []File` is separate from `Plugin(withHook bool)` — Uninstall has no
  `--with-agents` and must always plan agent removal
- Init order: config, feature root, manifest, start, finish, hook, planner, implementer,
  reviewer, snippet. Uninstall: snippet, reviewer, implementer, planner, hook, finish, start,
  manifest, config
- `--with-agents` + resolved host `none` = usage error via `setup.ErrAgentsNeedHost`, checked
  on the resolved host — S09's detection changes the outcome of bare `init --with-agents`
- Existing config never edited by `--with-agents`; the hint lists only *unbound* roles — a
  deliberate reading of R7 (the whole block would tell an adopter to overwrite their own
  binding); S10's `roles` row uses the same unbound vocabulary; JSON carries it in `roles_to_add`, stderr stays empty
- `--force --with-agents` rewrites to the bound variant; `--force` "unchanged" = exact bytes
- `roles.*` free-form, `optional-conventions` unvalidated at load — debt closed by decision;
  unbound roles are doctor's (S10) to report
- Pre-S08 `ConfigFile` digest not kept (unreleased; `Reviewer` field changed the render)

**Left unbuilt** — named so nobody assumes it exists:
- doctor rows `host-agents`, `roles` — S10
- `--print` output for agents and the bound config — S09
- `artifact.OriginOlder` for agent kinds — S10

**Traps** — things that look right and are not:
- `planConfig`'s recognized-file branch returned `config.Default()`; for a bound file that
  reports roles unbound and prints a bogus hint — decode the real roles
- `configFileCurrent` via `Recognize` treats the plain render as current under
  `--force --with-agents` → no bindings written
- A "no agent rows without the flag" test passes if agents are never planned at all —
  needs the `--with-agents` control arm on the same fixture
- Frontmatter/namespacing were not externally verified at planning time (Step 0)
- Adding `Reviewer` reddens any test pinning `ConfigFile()` bytes or digest — expected
