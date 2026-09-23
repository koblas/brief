---
id: SCENARIO-01
status: done
---

# SCENARIO-01: init installs the brief-workflow skill on every claude-code install

## Scenario

```gherkin
Scenario: SCENARIO-01 init installs the brief-workflow skill on every claude-code install
  Given a repository with ".claude/" and no brief install
  When I run "brief init" (with or without --with-agents)
  Then ".claude/skills/brief-workflow/SKILL.md" is created with the ruled bytes
  And a re-run reports it "unchanged", an edited copy is kept, "--host none" writes no skill,
      --dry-run/--print show it, and uninstall removes an unedited copy
```

User-visible contract (all exit 0 except the unwritable row, which is exit 1 — pinned by
`Test_init_refuses_an_unwritable_target_and_prints_the_manual_output`):
- `brief init --host claude-code` stdout gains `created .claude/skills/brief-workflow/SKILL.md`
  after the `hooks.json` row and before the agent rows / `CLAUDE.md` row. Row order: config,
  feature-root, plugin…, hook, **skill**, agent…, snippet. `--json` row `kind` is `"skill"`.
- `--no-hook` and `--with-agents` both still write it. `--host none` produces no skill row and no file.
- Re-run → `unchanged`. Edited copy → `kept … (edited locally)`, even under `--force`.
  Non-regular path → `kept … (not a regular file)`. Unwritable → the existing R10 refusal, exit 1,
  nothing written.
- `--dry-run` → same row, nothing written. `--print` → `# <path> (create)` + the ruled bytes.
- An existing install without the skill (every current adopter): rerun creates only the skill row,
  and stderr is the ordinary `installed for claude-code; …` line (`initNextAction` is kind-generic).
- `brief uninstall` removes an unedited copy and prunes `.claude/skills/brief-workflow/` when empty
  (never `.claude/skills/`). An edited copy is `kept (edited locally)` unless `--force`.
  `--host none` leaves it. Uninstall order: snippet, agents (reversed), skill, hook, plugin (reversed), config.

## Implementation Plan

Verification per `.claude/rules/agent-briefs.md`. `golangci-lint` runs with `default: all`, so
`exhaustive` is on: every `switch` over `artifact.Kind` / `setup.Kind` needs the new case.

- [x] Step 1: `internal/platform/artifact/artifact_test.go` `Test_workflow_skill_renders_the_ruled_bytes` — pin `SkillWorkflow()` against a literal copied from specification.md's *The skill file* block, not from the render (red)
- [x] Step 2: `internal/platform/artifact/artifact_test.go` `Test_recognize_classifies_each_plugin_file_against_its_own_kind` — add current / one-byte-edited / cross-kind (`SkillStart` bytes vs `KindSkillWorkflow`) rows (red)
- [x] Step 3: `internal/platform/artifact/plugin.go` — `KindSkillWorkflow` constant + `SkillWorkflow()` render + `Render` case (green)
- [x] Step 4: `internal/platform/artifact/digest.go` — `skillWorkflowDigests` / empty `olderSkillWorkflowDigests` pair + `digestsFor` case (green)
- [x] Step 5: `internal/platform/artifact/doc.go` + `Kind` type doc — name the workflow skill among the rendered files (update)
- [x] Step 6: `internal/platform/host/claudecode_test.go` `Test_claude_code_lists_the_workflow_skill_outside_the_plugin_directory` — one `File` at `WorkflowSkillDir + "/SKILL.md"`, kind `KindSkillWorkflow`, not under `PluginDir`, not `Hook`; fresh copy per call (red)
- [x] Step 7: `internal/platform/host/host.go` `Host.Skills() []File` + `WorkflowSkillDir` const; `claudecode.go` `claudeCodeSkillFiles` + `(claudeCode) Skills()` (green)
- [x] Step 8: `internal/platform/host/doc.go` + `File` doc — Skills is a third list, installed on every claude-code install, outside PluginDir (update)
- [x] Step 9: `internal/setup/skill_test.go` `Test_init_for_claude_code_writes_the_workflow_skill_after_the_hook` — table over plain / `NoHook` / `WithAgents`: `KindSkill` row at the index after the hook (or after finish under NoHook) and before the first agent/snippet row; file bytes equal `artifact.SkillWorkflow()` (red)
- [x] Step 10: `internal/setup/skill_test.go` — the rest of the init matrix: `--host none` writes no skill row and no file (control: claude-code does); rerun `unchanged`; edited copy kept under `Force`, bytes untouched; directory at the path kept `not a regular file`; `DryRun` row present, file absent; `Print` entry `create` with the ruled body; install-without-skill upgrade creates only the skill row (red)
- [x] Step 11: `internal/setup/setup.go` — `KindSkill Kind = "skill"`; `planSkillFiles(root, h)` (via `planPluginFile`, kind `KindSkill`); skill arts inserted into both `artifacts` and `writeArts` between plugin and agent arts (green)
- [x] Step 12: `internal/setup/print.go` `printArtifacts` — add `KindSkill` to the body-by-path case (green)
- [x] Step 13: `internal/setup/writable_test.go` — an unwritable `.claude/skills/brief-workflow` parent refuses before anything is written (confirm it rides `writableTargets` via `writeArts`; green on arrival is acceptable — say so) (red or green-on-arrival)
- [x] Step 14: `internal/setup/skill_test.go` uninstall arm — unedited copy removed and `.claude/skills/brief-workflow/` pruned while a sibling `.claude/skills/other/` survives; edited copy kept `ForceRemovable` unless `Force`; `--host none` leaves it; skill row sits between the last agent row and the hook row (red)
- [x] Step 15: `internal/setup/uninstall.go` — plan `h.Skills()` removal (reversed) after agents, before plugin files; add `host.WorkflowSkillDir` to `pluginPruneDirs` (before `host.PluginDir`; never `.claude/skills`) (green)
- [x] Step 16: `internal/setup/roundtrip_test.go` — existing init→uninstall tree-equality tests must pass with the skill in the tree; mutation-check by dropping the prune entry and seeing them red (green)
- [x] Step 17: existing exact-count / exact-row assertions in `internal/setup/{plugin,agents,print,uninstall,setup_converge}_test.go` — every `require.Len` bump comes with an assertion on the new skill row at its index (update)
- [x] Step 18: `internal/setup/doc.go`, `Server.Init` / `Server.Uninstall` / `apply` / `pluginPruneDirs` / `Kind` docs — ownership is `host.PluginDir` and below plus `host.WorkflowSkillDir`; Init writes Skills on every claude-code install (update)
- [x] Step 19: `internal/cli/init_test.go` — update the exact stdout blocks (claude-code, no-hook, rerun, with-agents, print, dry-run) to carry the skill row in position; new `Test_init_over_an_install_without_the_workflow_skill_creates_only_it` (stdout one `created` row, stderr `installed for claude-code; …`, exit 0) (red→green)
- [x] Step 20: `internal/cli/init_json_test.go`, `uninstall_test.go`, `uninstall_json_test.go` — update exact rows; assert a `"kind": "skill"` row in init `--json` (update)
- [x] Step 21: `internal/cli/help_test.go` `Test_init_help_names_the_brief_workflow_skill` — whitespace-normalized contains of the ruled first sentence in `brief init --help` (red)
- [x] Step 22: `internal/cli/init.go` `initLong` — insert, hand-wrapped, directly after the `--with-agents` sentence ("…for any role still unbound."): `Every claude-code install also writes a "brief-workflow" skill under ".claude/skills/brief-workflow/", which agents preload by listing it in their frontmatter "skills:".` — this sentence only (green)
- [x] Step 23: `go build ./...`, `go test ./...` (unpiped, report count + delta), `go test -race ./internal/setup/... ./internal/platform/... ./internal/cli/...`, `golangci-lint run ./...` all green → tick SCENARIO-01 in specification.md, set `status: done`, rewrite STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- The skill is a separate `host.Host.Skills() []File` list, NOT an entry in `Plugin()` — putting it in `Plugin` makes `Plugin(false)` trim the skill instead of the hook, breaks `claudecode_test.go`'s every-file-under-`PluginDir` pin, and turns doctor's `host-plugin` (probes `Plugin(false)`) ERROR "incomplete" on every older install, dragging S04 into S01.
- `File.RelPath` is install-root-relative (not PluginDir-relative); the skill entry is `host.WorkflowSkillDir + "/SKILL.md"` with `WorkflowSkillDir = ".claude/skills/brief-workflow"`. S04's `host-skill` row and S06–S08 read the path from here.
- Setup report kind is `setup.KindSkill` = `"skill"`; artifact kind is `artifact.KindSkillWorkflow`, render `artifact.SkillWorkflow()`. Row order config, feature-root, plugin…, hook, skill, agent…, snippet — S07 inserts `bound-agent…` between agent and snippet.
- Skill arts ride `writeArts` (drives `apply`, `writableTargets`, `--print` bodies). Anything planned only into `artifacts` reports `created` and writes nothing.
- `olderSkillWorkflowDigests` ships empty; a future byte change moves today's digest there (Rule 6).
- brief owns `host.PluginDir` and below **and** `host.WorkflowSkillDir`; prune stops there, never `.claude/skills/`.
- Uninstall uses `planPluginRemoval` semantics: `--force` removes an edited SKILL.md. S08's "kept as edited" gate must read the skill's own uninstall artifact (`KindSkill`, `ActionKept`), not re-derive it.

**Left unbuilt** — named so nobody assumes it exists:
- initLong remainder ("init never edits an agent file of yours… --edit-agents …") — S06 (missing-skill report) / S07 (`--edit-agents`).
- `uninstallLong` does not mention the skill; its ruled addition is S08's. Open debt for the final product-vision pass — do not invent copy.
- doctor `host-skill` row — S04. `anyIntegrationFilePresent` (`internal/doctor/host.go`, `Plugin(true) ∪ Agents()`) does not see the skill, and the doctor fixture helpers (`internal/doctor/doctor_test.go:39`, `internal/cli/doctor_internal_test.go:219`) do not write it. S04 decides whether `Skills()` joins that union and updates the helpers.
- planner/implementer rewrite with `skills: [brief-workflow]` — S02; resolver — S03; `roles-skill` — S05.
- `artifact.OriginOlder` handling in setup — S02 builds it. `setup.planPluginFile` (init) reports every non-`OriginCurrent` file, older included, as `kept (edited locally)`, so it never upgrades anything. `setup.planPluginRemoval` (uninstall) keeps an older file as `ForceRemovable` unless `--force`, which contradicts Rule 1. `grep OriginOlder internal/setup` finds no handling on either path. Triage's "already exists" covers only `artifact`'s classifier. S01 is unaffected because the skill's older list ships empty.

**Traps** — things that look right and are not:
- `exhaustive` is on (`default: all`): `artifact.Render`, `artifact.digestsFor`, `setup.printArtifacts` each switch over a Kind and each needs the new case, or lint fails while tests pass.
- Pinning the SKILL.md test against `artifact.SkillWorkflow()` or `Render(...)` proves nothing. The literal comes from specification.md, byte-for-byte (em dash, trailing newline).
- `initLong` is hand-wrapped. A verbatim `Contains` on the sentence fails. Normalize whitespace in the assertion.
- Bumping `require.Len(..., 7)` to `8` without asserting the new row at its index is a vacuous update.
- The R10 unwritable refusal is exit **1**, not 2 (2 is usage errors).
