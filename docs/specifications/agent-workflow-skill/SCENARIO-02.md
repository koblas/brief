---
id: SCENARIO-02
status: done
---

# SCENARIO-02: brief's planner and implementer agents preload the skill

## Scenario

```gherkin
Scenario: SCENARIO-02 brief's planner and implementer agents preload the skill
  Given "brief init --with-agents"
  Then planner.md and implementer.md carry "skills: [brief-workflow]" and the one-line bodies
  And reviewer.md is byte-identical to today
  And a previous release's planner/implementer bytes are recognized as "older", upgraded by init, not "edited"
```

The ruled bytes are in specification.md `## Surface & Copy` → *Rewritten plugin agents*. They
use a **block list** (`skills:` / `  - brief-workflow`), not the Gherkin's flow form. Surface &
Copy wins.

User-visible contract:
- `brief init --host claude-code --with-agents`, with an older planner on disk, prints to
  stdout `merged .claude/skills/brief/agents/planner.md (updated)` and exits 0. The implementer
  row has the same shape. A current file reports `unchanged` and an edited one
  `kept … (edited locally)`, both as today.
- `--dry-run` shows the same `merged` row and writes nothing. `--print` emits the full new
  body as `# <path> (merge)`.
- If the agents dir is unwritable while an older file needs replacing, init refuses under the
  existing R10 contract (stderr, exit 1).
- The file changing between plan and apply refuses as `ErrConcurrentEdit`, with no write.
- `brief uninstall` removes an older planner/implementer without `--force` (`removed <rel>`,
  no detail).
- `brief doctor` `host-agents` shows WARN `installed by an older brief release: <rel>`, fix
  `run 'brief init --with-agents'`. That code already exists and needs no change.

The fix is **generic in setup**. `planPluginFile` and `planPluginRemoval` are the one
decision point for every plugin Kind (plugin, hook, skill, agent). Every other Kind's
older-digest list is empty, so only the agents change behavior today. The tests go through
agents only, because no other Kind can classify as older yet.

## Implementation Plan

- [x] Step 1: capture today's `artifact.AgentPlanner()`, `AgentImplementer()` and `AgentReviewer()` bytes mechanically (throwaway `%q` dump, never hand-transcribed) as test literals, before touching `agents.go` (new)
- [x] Step 2: `internal/platform/artifact/agents_test.go` `Test_planner_and_implementer_agents_render_the_ruled_bytes` — literals copied byte-for-byte from specification.md; plus `Test_reviewer_agent_bytes_are_unchanged` pinned to the Step 1 reviewer literal (red for planner/implementer)
- [x] Step 3: `internal/platform/artifact/agents_test.go` `Test_previous_release_agent_renders_classify_as_older` — `Recognize` of the Step 1 planner/implementer literals is `OriginOlder`, today's render is `OriginCurrent`, and the reviewer literal is still `OriginCurrent` (red)
- [x] Step 4: `internal/platform/artifact/agents.go` `renderAgent` — accept an optional skills list and emit a `skills:` block list after `tools`, nothing when it is empty, so reviewer bytes do not move (update)
- [x] Step 5: `internal/platform/artifact/agents.go` `AgentPlanner` / `AgentImplementer` — ruled one-line bodies plus `brief-workflow` skill, with the `brief-workflow` name as a named constant; reviewer call unchanged (green)
- [x] Step 6: `internal/platform/artifact/digest.go` `olderAgentPlannerDigests` / `olderAgentImplementerDigests` — `sha256` of unexported fixed literals holding the previous release's bytes (from Step 1), never a live render call (green)
- [x] Step 7: `internal/platform/artifact/agents_test.go` existing frontmatter/body tests — accept the `skills` key and the new bodies without weakening their no-`model`/own-invocations assertions (update)
- [x] Step 8: `internal/setup/agents_test.go` `Test_init_with_agents_upgrades_an_older_agent_file` — table over planner/implementer: older literal on disk → `ActionMerged`, detail `updated`, bytes equal the spec literal, path in `Result.Modified`. Control rows show edited stays `ActionKept` "edited locally" and current stays `ActionUnchanged` (red)
- [x] Step 9: `internal/setup/setup.go` `planPluginFile` — an `OriginOlder` classification plans `ActionMerged`, detail `updated`, carrying the planned bytes on `pluginArtifact` (new field) for the apply guard (green for planning)
- [x] Step 10: `internal/setup/setup.go` `apply` — write `ActionMerged` plugin arts after re-verifying their bytes still equal what was planned (reuse or generalize `verifySnippetUnchanged` → `ErrConcurrentEdit`), and append the path to `Result.Modified` (green)
- [x] Step 11: `internal/setup/apply_internal_test.go` `Test_apply_refuses_an_older_plugin_file_changed_since_planning` — file edited between plan and apply → `ErrConcurrentEdit`, bytes untouched. Control arm: unchanged bytes are replaced (red→green against Step 10)
- [x] Step 12: `internal/setup/writable_test.go` `Test_init_refuses_when_an_older_agent_file_is_unwritable` — older planner in a non-writable agents dir → `ErrUnwritable` refusal, nothing written (red)
- [x] Step 13: `internal/setup/writable.go` `writableTargets` — include writeArts reporting `ActionMerged` (green)
- [x] Step 14: `internal/setup/agents_test.go` `Test_init_with_agents_dry_run_and_print_show_an_older_agent_upgrade` — `--dry-run` gives the merged row and the file byte-identical afterwards; `Print` gives `PrintMerge` with the full new render body. Expected green on arrival via existing `printArtifacts`; say so if it is
- [x] Step 15: `internal/setup/agents_test.go` `Test_init_without_agents_leaves_an_older_agent_file_alone` — plain init (no `WithAgents`) does not touch an older planner. This pins existing behavior (green on arrival)
- [x] Step 16: `internal/setup/uninstall_test.go` `Test_uninstall_removes_an_older_agent_file_without_force` — older planner/implementer → `ActionRemoved`, no detail, `ForceRemovable` false, file gone. Control: an edited file is still kept without `--force` (red)
- [x] Step 17: `internal/setup/uninstall.go` `planPluginRemoval` — `OriginOlder` is removed like `OriginCurrent` (green)
- [x] Step 18: `internal/cli/init_test.go` `Test_init_with_agents_reports_an_older_agent_as_merged_updated` — `cli.Run` stdout carries `merged .claude/skills/brief/agents/planner.md (updated)`, exit 0 (green on arrival expected after Steps 9–10)
- [x] Step 19: `internal/doctor/host_test.go` `Test_diagnose_classifies_host_agents` — add the case "an older planner render": WARN `installed by an older brief release: .claude/skills/brief/agents/planner.md`, fix `run 'brief init --with-agents'`, fixture from the Step 1 literal. Green on arrival; mutation-verify by emptying `olderAgentPlannerDigests` and watching this case redden
- [x] Step 20: doc sweep. `internal/setup/doc.go` ("simply edited locally… never rewrites"); `planPluginFile`, `planPluginRemoval`, `apply`, `writableTargets` and `pluginArtifact` docs; `ActionMerged` doc (whole plugin file replaced from an older render); `artifact` agents file comment and `renderAgent` doc (skills); the `AgentPlanner`/`AgentImplementer` docs; the `internal/doctor/host.go` `originRow` doc (~line 207–216) and the `host_internal_test.go` header, which now overclaim "older unreachable" for agents but stay true for plugin/hook/skill/snippet (update)
- [x] Step 21: `go build ./...`, `go test ./...`, `go test -race ./internal/setup/... ./internal/platform/artifact/... ./internal/doctor/...`, `golangci-lint run ./...` all green → set `status: done` here and tick SCENARIO-02 in specification.md (developer only)

## Handoff

**Binding decisions** (a later scenario must not contradict these without saying so):
- Older plugin files are upgraded **generically** in `setup.planPluginFile` (`OriginOlder` →
  `ActionMerged`, detail `updated`) and removed without `--force` in `planPluginRemoval`. This
  satisfies Rules 1 and 6 for every Kind, including the skill (S04's `host-skill` older row
  assumes init fixes it).
- Plugin-file upgrades go into `Result.Modified`, join `writableTargets` (R10) and are guarded
  by an `ErrConcurrentEdit` re-read before the write. A later writer of a user file
  (S07 `--edit-agents`, S08 uninstall edits) should reuse the same guard.
- `renderAgent` takes an optional skills list, emitted as a block list after `tools`. An empty
  list emits nothing, which keeps reviewer bytes fixed (out of scope per spec).
- Older-digest entries are `sha256` of fixed byte literals, never of a live render.
- Plain `brief init` (no `--with-agents`) never plans agent files, so it never upgrades an older
  one. This is existing behavior, and doctor's fix already says `--with-agents`. Do not "fix" it.

**Copy not ruled**, needs a product-vision look at the final gate:
- The `(updated)` detail and the `merged` action for a whole-file replace. They mirror the
  snippet's `merged CLAUDE.md (block updated)` and `ActionMerged`'s own doc. No new Action
  constant.

**Left unbuilt** (named so nobody assumes it exists):
- Role resolver (S03), `host-skill` row (S04), `roles-skill` row (S05), missing-skill report and
  `agents_missing_skill` (S06), `--edit-agents` (S07), uninstall agent edits and
  `uninstallLong` (S08).
- `setup.planSnippet` (`snippet.go` ~297) and `planSnippetRemoval` (~359) still treat an
  `OriginOlder` snippet as "edited locally", although `ActionMerged`'s doc says older blocks are
  replaced. This is out of scope (snippet bytes are unchanged, and its older list is empty so
  the case is unreachable). The next snippet byte change must fix it first.

**Traps** (things that look right and are not):
- `sha256.Sum256(AgentPlanner())` in `olderAgentPlannerDigests` duplicates the current digest.
  The older arm becomes unreachable and every render-pinned test stays green. Only the
  independent Step 1 literal fixtures catch it.
- The previous bytes exist nowhere once `agents.go` changes. Capture them first (Step 1).
- The Gherkin reads `skills: [brief-workflow]`, but the ruled bytes are a block list. S07's shape
  table must accept both, and brief's own agents use the block form.
- `apply` and `writableTargets` look only at `ActionCreated`. A planned `ActionMerged` row with
  no apply change reports `merged` and writes nothing.
- `exhaustive` lint: no new Kind is added here, but a test-body `switch` over `setup.Action`
  would trip it. Use an if-chain.

**Open debts**:
- UNVERIFIED (product-vision item 8): whether a plugin agent (`.claude/skills/brief/agents/*.md`)
  resolves a *project* skill by bare name `brief-workflow`. It cannot be checked without a live
  Claude Code session. The bodies keep inline `brief start`/`brief finish`/`brief new step`
  invocations, so the agents still work if it does not. Verify in a live session before
  release.
