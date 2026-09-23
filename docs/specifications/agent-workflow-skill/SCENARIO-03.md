---
id: SCENARIO-03
status: done
---

# SCENARIO-03: doctor resolves a bound role by frontmatter name, project first

## Scenario

```gherkin
Scenario: SCENARIO-03 doctor resolves a bound role by frontmatter name, project first
  Given ".brief.yaml" binds implementer to "developer"
  And ".claude/agents/developer/Agent.md" declares "name: developer"
  When I run "brief doctor"
  Then the roles row resolves it (nested layout, any filename)
  And a project definition shadows a same-named "~/.claude/agents" agent
  And a name defined twice under .claude/agents WARNs naming both paths
  And a user-level-only resolution adds "; user-level: <role>"
```

User-visible contract (`brief doctor`, `roles` row, path `.brief.yaml`; exit codes unchanged,
the roles row is never ERROR):
- A bare binding `<name>` resolves to every `*.md` under `<root>/.claude/agents/` (recursive)
  whose frontmatter `name:` equals `<name>`. Only when there are none does it look under
  `<home>/.claude/agents/` (recursive). A file with no frontmatter, no closing delimiter, or YAML
  that fails to decode is not a candidate. The filename plays no part.
- New WARN problem, joined `; ` with the existing ones:
  `<role>: <name> defined <n> times under .claude/agents (<rel>, <rel>)`. `<rel>` is relative
  to the repository root and slash-separated. Paths are in lexical walk order. The fix text is
  the existing roles fix, unchanged.
- New OK suffix `; user-level: <role>`, written beside `; not verified: <role>`. Suffixes follow
  the roles' order in `RoleBindings` (planner, implementer, reviewer), one suffix per role.
- A `brief:<name>` binding keeps today's filename-based lookup. A `<plugin>:<name>` binding stays
  "not verified".

Existence facts, from `go doc` plus anchored greps:
- `resolveRoleBinding`, `rolesCheck` and `roleBindingResult` are in `internal/doctor/host.go`
  (~674-801).
- `stepfile.ParseFrontmatter` decodes only onto `stepfile.Frontmatter`, so it cannot be reused
  as-is.
- Nothing under `internal/platform` resolves agents today.
- `internal/setup` never resolves roles.

## Implementation Plan

### Red
- [x] Step 1: `internal/platform/agentfile/agentfile_test.go` `Test_find_matches_frontmatter_name_anywhere_under_project_agents` — a nested `developer/Agent.md` with `name: developer` comes back as one project-scope definition with its path. Fails: the package does not exist.
- [x] Step 2: `agentfile_test.go` `Test_find_ignores_a_filename_match_whose_name_differs` — `developer.md` declaring `name: someone-else` is not returned for `developer`. Fails: the package does not exist.
- [x] Step 3: `agentfile_test.go` `Test_find_skips_files_whose_frontmatter_does_not_parse` — a table of no frontmatter, unclosed delimiter, and YAML that does not decode. Each arm has a control: the same path with valid `name:` frontmatter is found. Fails: the package does not exist.
- [x] Step 4: `agentfile_test.go` `Test_find_prefers_project_definitions_over_user_level` — when both scopes define the name, only project definitions come back. The control arm drops the project file and must return the user definition with user scope. Fails: the package does not exist.
- [x] Step 5: `agentfile_test.go` `Test_find_returns_every_project_duplicate_in_lexical_order` — two project files declare the same name. Fails: the package does not exist.
- [x] Step 6: `agentfile_test.go` `Test_find_edge_cases` — a table: an empty home skips the user scope, a missing `.claude/agents` returns nothing, and a non-`.md` file with a matching name is ignored. Fails: the package does not exist.
- [x] Step 7: `internal/doctor/host_test.go` `Test_diagnose_classifies_roles` — give the two bare-name fixtures (`my-reviewer.md`, project and home) a `name: my-reviewer` frontmatter. Green on arrival today; they are updated here because otherwise Green turns them into "not found". Leave the `brief:reviewer` override fixture (`custom reviewer\n`) as it is.
- [x] Step 8: `internal/doctor/host_test.go` `Test_diagnose_roles_resolves_by_frontmatter_name` — a new table whose only assertion is exact Severity and exact `Detail` equality (no `Contains`, no conditional assertions). The cases, each failing today for the reason given:
  - nested `.claude/agents/developer/Agent.md` bound as implementer → OK `planner, implementer, reviewer bound`. Fails: WARN `implementer: developer not found`.
  - project `.claude/agents/my-reviewer.md` declaring `name: someone-else` → WARN `reviewer: my-reviewer not found`. Fails: OK through the filename.
  - project definition nested plus user definition at `<home>/.claude/agents/team/x.md` → OK with no suffix. Fails: WARN not found.
  - control arm: the same user file, no project file → OK `…bound; user-level: reviewer`. Fails: WARN not found.
  - two project files with the same name → WARN `reviewer: my-reviewer defined 2 times under .claude/agents (.claude/agents/my-reviewer.md, .claude/agents/team/r.md)`. Fails: OK.
  - planner user-level only and reviewer `other:reviewer` → OK `…bound; user-level: planner; not verified: reviewer`. Fails: WARN not found.

### Green
- [x] Step 9: `internal/platform/stepfile/frontmatter.go` — export the delimiter split as a decoder onto a caller-supplied value (`stepfile.DecodeFrontmatter`). `ParseFrontmatter` delegates to it. This is a refactor that the existing stepfile tests keep green.
- [x] Step 10: `internal/platform/agentfile/agentfile.go` — add `Frontmatter` (only `Name` for now), `Scope` with `ScopeProject` = "project" and `ScopeUser` = "user", `Definition` (absolute `Path`, `Scope`, `Frontmatter`), and `Find(root, home, name) []Definition`. `Find` walks recursively, treats any decode error as "not a candidate", searches project first and home only when the project has none, and returns lexical order.
- [x] Step 11: `internal/doctor/host.go` `resolveRoleBinding` — the bare-name branch calls `agentfile.Find` with the injected home; a home error or "" means no user scope. The result carries the matched definitions for bare names. For `brief:*` it carries the resolved file's path, either the plugin file or the filename override, each checked by a stat as today.
- [x] Step 12: `internal/doctor/host.go` `rolesCheck` — add the duplicate-definition problem (project scope only) and the per-role `user-level:` suffix, ordered alongside `not verified:` by role.

### Sweep
- [x] Step 13: `internal/platform/agentfile/doc.go` — package doc. It states that the rule mirrors Claude Code's agent identification, that the package does no editing, and that the Scope values are the strings S06's JSON uses (sweep)
- [x] Step 14: `internal/doctor/host.go` / `doctor.go` — rewrite the doc comments on `resolveRoleBinding`, `rolesCheck` and `WithHomeDir`, plus the `Test_diagnose_classifies_roles` doc, which still describe `<name>.md` filename rules (sweep)
- [x] Step 15: fix what `go build ./... && golangci-lint run ./...` reports, including any `exhaustive` switch over `roleBindingResult` (sweep)

### Verify
- [x] Step 16: full verification per `.claude/rules/agent-briefs.md`. Report the test count and the delta, then run two separate mutations of `agentfile.Find`, restoring the file each time from a fresh `$TMPDIR` copy:
  - **Project shadows user:** search home even when the project matched, and return the user definitions first. `Test_find_prefers_project_definitions_over_user_level` and the doctor "project shadows user" case, which gains `; user-level: reviewer`, must both go red.
  - **No filename fallback:** also accept `<dir>/<name>.md` as a candidate when its frontmatter name differs. `Test_find_ignores_a_filename_match_whose_name_differs` and the doctor "filename match with a different name" case must go red.

## Handoff

**Binding decisions**. A later scenario must not contradict these without saying so:
- Bare-name resolution lives in `internal/platform/agentfile` (`Find(root, home, name) []Definition`). Doctor and setup may import only `internal/platform/*`, and S06–S08 need the same resolution doctor reports.
- `Find` returns every match from one scope only: project if any, otherwise user. Paths are absolute and in lexical walk order. An empty slice means not found. Callers derive "duplicate" and "user-level" from the slice. It returns no error: unreadable dirs and files are skipped, not reported.
- `Definition` carries `Path`, `Scope` and the whole parsed `Frontmatter`. S05 adds `Skills` and `OmitClaudeMd` to `Frontmatter`, and S06 maps `Scope` straight to JSON `scope` ("project"/"user"). Extend the struct; do not add a second parse.
- `brief:<name>` stays filename-based: `<root>/.claude/skills/brief/agents/<name>.md`, or the override `<root>/.claude/agents/<name>.md`, both stat-only with no frontmatter needed. Rule 5 is scoped to bare names, so name-matching `brief:*` would be an unruled contract change. Doctor's resolution records the resolved path for `brief:*` too, because S05's roles-skill must read frontmatter from whichever file resolved. The override file may have none; S05 decides what that means.
- The duplicate WARN is project scope only, because the copy says "under .claude/agents". `<rel>` is root-relative and slash-separated. The duplicate problem reuses the existing roles fix text.
- OK suffixes follow `RoleBindings` order, one per role (`user-level:` or `not verified:`), pinned by the mixed case in Step 8.

**Left unbuilt**. These are named so nobody assumes they exist:
- `doctorLong`'s roles clause: S05 replaces the whole clause with the ruled sentence verbatim. The text is hand-wrapped, so normalize whitespace before a `Contains` assertion. Today's wording, "resolves to an agent file, reading ~/.claude/agents…", is stale only in detail.
- An exported single-file parse (`agentfile.Load`, or similar) for the `brief:*` path's frontmatter: S05.
- The `brief:` / `<plugin>:` / bare prefix split is still inside doctor's `resolveRoleBinding`. setup cannot import doctor, so S06/S07 must move it down (e.g. into `internal/platform/config`) or re-derive it.
- `Frontmatter.Skills`, `Frontmatter.OmitClaudeMd`: S05.

**Traps**. These look right and are not:
- yaml.v3 partially fills fields on a `*yaml.TypeError`. "Any Unmarshal error → not a candidate" is the pinned rule, so do not use a partial decode.
- When S05 adds `Skills []string`, an agent with a scalar `skills: foo` fails the decode and drops out of resolution: roles flips to "not found" rather than roles-skill WARNing. Decode `skills:` tolerantly (e.g. `yaml.Node`) and classify its shape separately; S07's shape table needs that anyway.
- `filepath.WalkDir` does not descend into symlinked directories. A symlinked `*.md` file is read through its link, so `Definition.Path` can resolve outside the repository. S07's Rule 3 ("never edits a file outside the repository") must check the resolved target, not just `Scope`.
- The `brief:reviewer` override fixture in `Test_diagnose_classifies_roles` has no frontmatter on purpose. It pins the filename-based `brief:*` override.
- User-level duplicates resolve silently; nothing WARNs. UNRULED: the final product-vision decides.
