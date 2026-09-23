---
id: SCENARIO-05
status: done
---

# SCENARIO-05: doctor checks bound planner/implementer preload the skill (roles-skill)

## Scenario

```gherkin
Scenario: SCENARIO-05 doctor checks bound planner/implementer preload the skill (roles-skill)
  WARN per agent lacking it (with omitClaudeMd wording when set), OK when all do,
  SKIP when neither role is bound and resolved, "not verified" for other-plugin bindings
```

User-visible contract (Surface & Copy, verbatim): `brief doctor [--json]` gains one row,
`roles-skill`, appended after `roles` (path = the nearest `.brief.yaml`, `""` when none).
Exit codes unchanged: every roles-skill case is SKIP/WARN/OK, never ERROR (Rule 7), so it
never moves the exit code off 0. Rows go to stdout as today; `--json` gains one `checks[]`
entry, `counts` keeps its shape.

- SKIP `no planner or implementer bound`, Fix nil — no config, unparseable config, or neither
  planner nor implementer is bound **and** resolved.
- WARN `<role>: <name> does not preload brief-workflow` per lacking agent, joined `; `, in
  planner-then-implementer order; an agent with `omitClaudeMd: true` appends
  ` and omits CLAUDE.md, so it never sees brief's instructions`. Fix
  `add "brief-workflow" to the "skills:" list of each agent named, or run 'brief init --edit-agents' for those in the repository`.
- OK `planner, implementer preload brief-workflow`, plus `; not verified: <role>` per
  other-plugin (`<plugin>:<name>`, plugin ≠ brief) binding, planner-then-implementer.

`doctorLong`: the clause from `— plus whether each role` through `nowhere in the repository.`
is replaced by the ruled sentence; `It never reads a feature's own content…` stays.

Test files touched: `internal/platform/agentfile/agentfile_test.go`,
`internal/doctor/host_test.go`, `internal/doctor/doctor_test.go`,
`internal/cli/doctor_internal_test.go`, `internal/cli/help_test.go`.

## Implementation Plan

### Red
- [x] Step 1: `agentfile_test.go` `Test_find_decodes_skills_and_omit_claude_md` — block list, flow list (`[brief-workflow]`) and `omitClaudeMd: true` surface on `Definition.Frontmatter.Skills`/`.OmitClaudeMd`; fails: the fields do not exist
- [x] Step 2: `agentfile_test.go` `Test_find_still_resolves_an_agent_whose_skills_or_omit_claude_md_is_malformed` — table: scalar `skills: brief-workflow`, mapping `skills:`, non-bool `omitClaudeMd: yes please`; each is still found, with nil Skills / false OmitClaudeMd. **Green on arrival** today (the key is ignored); it exists as the guard for the loose decode — say so, do not manufacture a red
- [x] Step 3: `agentfile_test.go` `Test_load_decodes_one_agent_files_frontmatter` — table over `agentfile.Load(path)`: block list, scalar skills (loose, no error), no frontmatter (error), unparseable YAML (error), missing file (error); fails: `Load` undefined
- [x] Step 4: `host_test.go` `Test_diagnose_classifies_roles_skill` — table (`rolesSkillCase`, same shape as `rolesCase`), asserting severity, detail, fix and path of the `roles-skill` row; fails: no such row. Cases:
  - no config anywhere → SKIP, Path `""`, Fix nil
  - unparseable `.brief.yaml` → SKIP, Fix nil
  - planner and implementer unbound (reviewer bound) → SKIP
  - planner and implementer bound but not found → SKIP
  - both other-plugin (`acme:x`) → SKIP (neither resolved)
  - bare names, block-list `skills:` → OK
  - bare names, flow-list `skills: [brief-workflow]` → OK
  - `brief:planner`/`brief:implementer` against rendered plugin agents → OK
  - `brief:implementer` overridden by `.claude/agents/implementer.md` with no frontmatter → WARN `implementer: brief:implementer does not preload brief-workflow` (proves the override file, not the plugin render, is read)
  - planner resolved with skill, implementer `acme:impl` → OK `…; not verified: implementer`
  - planner lacks, implementer lacks + `omitClaudeMd: true` → WARN, two entries joined `; `, suffix only on the implementer's
  - `omitClaudeMd: yes please` + lacking skill → WARN without suffix
  - reviewer bound to an agent lacking the skill, planner/implementer have it → OK (reviewer excluded)
  - user-level-only planner with the skill (fake home) → OK
  - **scalar-skills guard**: implementer is the only bound role, its file has `skills: brief-workflow` (scalar) → roles detail exactly `planner unbound; reviewer unbound` (no `implementer: … not found`) **and** roles-skill WARN (both asserted in this case — a strict decode adds the "not found" problem to roles and flips roles-skill to SKIP)
  - **duplicate guard**: implementer name defined twice under `.claude/agents`, the definition that sorts **second** lacks the skill → WARN once for implementer
  - planner resolved with skill, implementer unbound → OK with the fixed ruled string
- [x] Step 5: `doctor_internal_test.go` `Test_doctor_reports_roles_skill_ok_after_init_with_agents` — `run` `init --host claude-code --with-agents` then `doctor` (with `doctorFakeSeams`) in the same wd; assert stdout carries the exact `OK  roles-skill  .brief.yaml  planner, implementer preload brief-workflow` row and exit 0; fails: row missing
- [x] Step 6: `help_test.go` `Test_doctor_help_names_role_resolution_and_the_brief_workflow_skill` — `normalizeWhitespace` Contains the ruled doctorLong sentence **and** NotContains `reading "~/.claude/agents"`; fails: old clause present, new sentence absent

### Green
- [x] Step 7: `internal/platform/agentfile/agentfile.go` `Frontmatter` — add `Skills []string` and `OmitClaudeMd bool`; decode through one unexported raw struct whose `skills`/`omitClaudeMd` are `yaml.Node`, mapped by an unexported func: a sequence of scalars → the list, anything else → nil; a bool scalar → its value, anything else → false
- [x] Step 8: `agentfile.go` `findIn` — decode via the raw struct (Step 7), never a partial decode on error
- [x] Step 9: `agentfile.go` `Load(path string) (Frontmatter, error)` — read one file and decode through the same path; errors on unreadable file, missing frontmatter, undecodable YAML
- [x] Step 10: `internal/doctor/host.go` — roles-skill copy as named constants (SKIP detail, WARN entry format, omitClaudeMd suffix, WARN fix, OK detail), comparing against `artifact.WorkflowSkillName`, never a literal
- [x] Step 11: `host.go` `rolesSkillCheckNoConfig`, `rolesSkillCheckUnparseable(nearest)` — the two SKIP rows (Fix nil)
- [x] Step 12: `host.go` `(*Server).rolesSkillCheck(root, nearest, planner, implementer roleBinding)` — per role via `resolveRoleBinding`: bare name → every `res.defs[*].Frontmatter` (lacking if any definition lacks; suffix if any lacking one sets `OmitClaudeMd`); `brief:*` → `agentfile.Load(res.path)`, a load error counts as lacking; unverified → `not verified` suffix; unbound/unresolved → not considered; none resolved → SKIP
- [x] Step 13: `internal/doctor/doctor.go` `Diagnose` — build the roles-skill row in the same config switch as `rolesCheck` (no-config / unparseable / parsed arms), append it after `rolesCheck`
- [x] Step 14: `internal/cli/doctor.go` `doctorLong` — replace the roles clause with the ruled sentence, hand-wrapped

### Sweep
- [x] Step 15: `doctor_test.go` `Test_diagnose_reports_every_check_ok_in_a_healthy_repository` — append `"roles-skill"` to the exact ID list; `doctor_internal_test.go:177` `assert.Len(t, doc.Checks, 13)` → 14; any other exact check-ID/count pin the suite reports (sweep)
- [x] Step 16: doc comments — `agentfile` package doc + `Frontmatter`/`Load`/`Find` (loose `skills:`/`omitClaudeMd` contract), `Diagnose`'s and `internal/doctor/doc.go`'s row order, the `roleResolution.path` comment (drop "a later consumer (S05's …)" — state the rule) (sweep)
- [x] Step 17: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

### Verify
- [x] Step 18: full verification per `.claude/rules/agent-briefs.md`, `go test -race ./internal/doctor/... ./internal/platform/agentfile/... ./internal/cli/...`, then mutate individually:
  - raw `skills` field typed `[]string` instead of `yaml.Node` → Step 2's scalar row and Step 4's scalar-skills guard go red
  - raw `omitClaudeMd` typed `bool` → Step 2's non-bool row goes red
  - drop the omitClaudeMd suffix append → Step 4's two-entry WARN case goes red
  - check only `res.defs[0]` → Step 4's duplicate guard goes red
  - include the reviewer binding → Step 4's reviewer-exclusion case goes red
  - `brief:*` load error counted as satisfied → Step 4's override-without-frontmatter case goes red
  - `Test_doctor_reports_every_row_ok_when_fully_installed` must stay unedited — its NotContains WARN/SKIP now also pins roles-skill OK on brief's own renders

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `agentfile.Frontmatter` carries `Skills []string` (nil for absent or any non-sequence shape) and `OmitClaudeMd bool` (false for any non-bool) — decoded loosely via `yaml.Node` so a malformed value never drops an agent out of Rule 5 resolution. S06/S07 read these fields; never re-parse.
- `agentfile.Load(path)` is the single-file entry point for a `brief:*` binding's resolved file, sharing `findIn`'s decode; S06–S08 reuse it for any known path.
- roles-skill considers planner and implementer only (product verdict item 1). "Bound and resolved" = `roleResolved`; `roleUnverified` counts as not resolved, so two other-plugin bindings SKIP and `; not verified:` appears only beside at least one resolved role.
- A `brief:*` file with no or unparseable frontmatter counts as "does not preload" (WARN), not SKIP.
- A bare name with several project definitions WARNs once for the role if **any** definition lacks the skill (Claude Code loads duplicates in unspecified order); the omitClaudeMd suffix applies if any lacking definition sets it. User-scope resolutions are checked the same way.
- roles-skill Path follows roles (`nearest`, `""` with no config); its SKIP Fix is nil (unlike roles' SKIP).

**Left unbuilt** — named so nobody assumes it exists:
- `skills:` shape classification (no key / block list / flow list / other / non-regular) — S07's `--edit-agents` shape table; S05 exposes only the decoded list.
- `agents_missing_skill` / init's missing-skill stderr report — S06.
- The `brief:` / `<plugin>:` / bare prefix split still lives in doctor's `resolveRoleBinding` — S06/S07 must move it down.

**Traps** — things that look right and are not:
- UNRULED COPY: with one role resolved and the other unbound/unresolved, OK still prints the fixed ruled `planner, implementer preload brief-workflow` (the roles row WARNs the missing role). Owner: final product-vision.
- A duplicate fixture whose lacking definition sorts first makes a `defs[0]`-only check pass — keep the lacking one second.
- The scalar-skills agent must be the only resolved role in its fixture, or a strict decode still yields OK/WARN from the other role and the guard proves nothing.
- The Gherkin's flow form `skills: [brief-workflow]` and brief's block-list render must both OK.
