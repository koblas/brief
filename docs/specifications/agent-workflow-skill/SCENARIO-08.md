---
id: SCENARIO-08
status: done
---

# SCENARIO-08: uninstall removes brief-workflow from bound agents

## Scenario

```gherkin
Scenario: SCENARIO-08 uninstall removes brief-workflow from bound agents
  Removes the entry (and a now-empty skills: line); keeps entries when SKILL.md itself is kept as edited
```

## User-visible contract

- Command: `brief uninstall [--host claude-code] [--dry-run] [--force] [--json]`. There is no new flag.
- stdout row (text): `removed <rel> (brief-workflow from skills)`. The kind is `bound-agent`. It prints one row per edited agent file, and a file shared by planner and implementer gets one row.
- Row order is the reverse of init: snippet, bound-agent…, agent…, skill, plugin…, config. Among the bound-agent rows, implementer comes before planner (strict reversal of init's planner-first walk).
- `--json`: the row is `{"kind":"bound-agent","action":"removed","detail":"brief-workflow from skills","path":<abs>}`. The absolute path goes in `modified`, never in `removed`. Under `--dry-run`, `modified` is empty and the row is still present.
- stderr next-action: unchanged logic. A bound-agent `removed` row counts as a host artifact removed, so the stderr line reads `removed brief's claude-code install; …`.
- Exit codes: 0 on success. A concurrent edit between plan and write returns the existing `ErrConcurrentEdit` refusal, with rerun text `brief uninstall`. A write failure after an earlier removal falls under the existing `ErrPartialWrite` contract.
- `uninstallLong` gains this sentence verbatim, placed right after the `…unless --force.` sentence and before `The feature root…`:
  `It also removes "brief-workflow" from the "skills:" list of the planner and implementer agents bound in ".brief.yaml", repository files only, unless the skill file itself is kept.`

## Implementation Plan

### Red
- [x] Step 1: `internal/setup/bound_agent_internal_test.go` `Test_remove_workflow_skill_edits_only_the_skills_line` — table test for the edit function, mirroring S07's add table. It covers: the inserted `skills: [brief-workflow]` line dropped; the block-list item removed; the last block item removed, which drops the `skills:` key line too; flow `[a, brief-workflow]` → `[a]`; brief-workflow first or middle in a flow list; `[brief-workflow]` line dropped; CRLF preserved; a body `skills:` after the closing `---` never touched; a quoted, commented or multi-line value → unremovable. Fails because the returned bytes still carry the brief-workflow entry (stub `removeWorkflowSkill` first so the table compiles).
- [x] Step 2: `internal/setup/uninstall_bound_agent_test.go` (black-box, `WithHomeDir` injected) `Test_uninstall_removes_brief_workflow_from_bound_project_agents` — covers the no-key, block and flow fixtures. The row is `removed`/`bound-agent`/`brief-workflow from skills`. The file still exists with the expected bytes and its permission bits intact, and its path is in `Modified` and not in `Removed`. Fails because no row is planned and the entry stays.
- [x] Step 3: same file, `Test_uninstall_orders_bound_agent_rows_after_the_snippet` — checks the snippet row, then implementer, then planner, then the agent rows. Fails because the rows are missing.
- [x] Step 4: same file, `Test_uninstall_keeps_bound_agent_entries_while_the_skill_is_kept` — with SKILL.md edited, the entry stays and there is no bound-agent row. With SKILL.md replaced by a directory, the same happens under `--force`. Control arm: the same fixture under `--force` with an edited (regular) SKILL.md gives a `removed` row and the entry gone. With SKILL.md absent, the entry is removed. Fails because the control arm has no row yet.
- [x] Step 5: same file, `Test_uninstall_leaves_non_targets_alone` — a table test. The baseline fixture is a bare-name project agent listing brief-workflow in a flow list, which gets a `removed` row. Each case changes exactly one variable from the baseline and asserts no bound-agent row and byte-identical agent bytes:
  - binding → `brief:planner`
  - agent defined only under the injected home. That home must live **inside** root (e.g. `root/home`), so the escape check cannot mask a missing `ScopeProject` filter.
  - leaf → symlink to a regular file
  - `.claude` → symlink to a directory outside root (DryRun, per the S07 trap)
  - value → quoted `"brief-workflow"`
  - skill not listed
  - `.brief.yaml` → invalid YAML (also asserts no refusal)
  - `--host none`

  Fails because the baseline row is absent.
- [x] Step 6: same file, `Test_uninstall_edits_a_shared_bound_agent_once` and `Test_uninstall_dry_run_plans_bound_agent_rows_and_writes_nothing`. Fails because no rows are planned.
- [x] Step 7: `internal/setup/apply_internal_test.go` (white-box, following that file's existing `ErrConcurrentEdit` precedent) `Test_apply_uninstall_refuses_a_bound_agent_edited_after_planning` — plan, change the agent's bytes, then call `applyUninstall`. Expects `ErrConcurrentEdit` and the agent's new bytes kept. Control arm: unchanged bytes → the edit lands. Fails because no `ErrConcurrentEdit` is returned and the control's bytes still list the skill (widen `applyUninstall`'s signature first so the test compiles).
- [x] Step 8: `internal/setup/roundtrip_test.go` `Test_init_edit_agents_then_uninstall_leaves_bound_agents_byte_identical` — the fixture is a pre-existing `.brief.yaml` with bare-name roles (edited, so kept) plus agents covering no key, a non-empty block list and a canonical `skills: [a]`. It runs `snapshotTree` before and after `Init{EditAgents}` followed by `Uninstall`. Fails because the agents still list brief-workflow.
- [x] Step 9: `internal/cli/uninstall_internal_test.go` (the `run()` seam plus `withSetupOpts(WithHomeDir)`) `Test_uninstall_reports_the_bound_agent_row` — checks the exact stdout row text in its position and the stderr next-action. The `--json` subtest checks the row fields, `modified` holding the absolute path and `removed` excluding it. The `--dry-run` subtest checks the same row, an empty `modified`, and the file unchanged. Fails because stdout lacks the `removed … (brief-workflow from skills)` row and `modified` is empty. Partial-write rendering needs no change, because `landedArtifacts` already reads `Result.Modified`.
- [x] Step 10: `internal/cli/help_test.go` — pin the `uninstallLong` sentence (whitespace-normalized, same precedent as the initLong pin at ~line 947). Fails because the sentence is absent.

### Green
- [x] Step 11: `internal/setup/bound_agent.go` — extract the target selection from `planBoundAgents` into one unexported selector: bare-name bindings only, `ScopeProject`, deduped by path, the Lstat regular/non-regular split, and the escaping path skipped. `planBoundAgents` calls it unchanged in behaviour, and the S07 tests are the safety net.
- [x] Step 12: `internal/setup/bound_agent.go` `removeWorkflowSkill` — the inverse of `addWorkflowSkill`. It reuses `findTopLevelSkillsKey`, `boundAgentBlockListItems` and `boundAgentTerminator`, and returns the edited bytes and a removable/unremovable result. "Listed" is caller-decided via `agentfile.Parse` over the same bytes.
- [x] Step 13: `internal/setup/bound_agent.go` `planBoundAgentRemovals` — uses the Step 11 selector. A non-regular or unremovable file gives no row, a removable one gives an `ActionRemoved` row carrying `existing`, `edited` and `perm`. The output order is the reverse of the selector's order.
- [x] Step 14: `internal/setup/uninstall.go` `(*Server).Uninstall`:
  - claude-code only: read roles via `config.Inspect(nearest)`. When `nearest == ""` or on any error, there are no bound rows. Resolve home through `s.homeDir`, falling back to `""` on error.
  - Plan the skill before the bound rows, and plan bound rows only when no `KindSkill` row is `ActionKept`.
  - Insert the bound rows after the snippet and before the agent rows.
  - Thread the bound-agent artifacts into `applyUninstall`.
- [x] Step 15: `internal/setup/uninstall.go` `applyUninstall` — skip `KindBoundAgent` in the `os.Remove` loop. After the snippet and before the other removals, run `verifyFileUnchanged(…, "brief uninstall")`, then `writeBoundAgent`, and append to `res.Modified`. Apply the existing `markPartial` handling.
- [x] Step 16: `internal/cli/uninstall.go` `runUninstall`, `internal/cli/cli.go` — accept `...setup.Option` and pass `rs.setupOpts` through to `setup.NewServer`.
- [x] Step 17: `internal/cli/uninstall.go` `uninstallLong` — add the ruled sentence verbatim at the position named in the contract above.

### Sweep
- [x] Step 18: `internal/setup/doc.go` — fix the Uninstall row-order sentence to "snippet, bound-agent…, agent…, skill, plugin…, config". Add the Uninstall bound-agent rule in one paragraph: the skill-kept gate, and no row for non-regular, unremovable or escaping files (sweep)
- [x] Step 19: `internal/cli/uninstall.go` `uninstallDocument` doc comment — drop the "modified always empty (uninstall never writes)" claim, which is already false because of the snippet (sweep)
- [x] Step 20: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

### Verify
- [x] Step 21: full verification per `.claude/rules/agent-briefs.md`, with `go test -race ./internal/setup/... ./internal/cli/...`. Mutate each guard individually, restoring from a fresh `$TMPDIR` copy each time:
  - (a) Remove the `KindBoundAgent` skip in `applyUninstall`'s remove loop. `Test_uninstall_removes_brief_workflow_from_bound_project_agents` must go red because the file is deleted.
  - (b) Invert or remove the skill-kept gate in `Uninstall`. `Test_uninstall_keeps_bound_agent_entries_while_the_skill_is_kept` must go red.
  - (c) In `removeWorkflowSkill`, keep an emptied flow line as `skills: []` instead of dropping it. `Test_init_edit_agents_then_uninstall_leaves_bound_agents_byte_identical` must go red on the no-key agent.
  - (d) Drop the `ScopeProject` filter in the shared selector. `Test_uninstall_leaves_non_targets_alone` must go red on its home-inside-root case. If it stays green, stop and report: the escape check is masking the filter.

## Handoff

**Binding decisions** (a later change must not contradict these without saying so):
- Uninstall and `--edit-agents` share one target selector: bare-name, `ScopeProject`, deduped, Lstat split, escape skip. Rule 3/4's boundary must be decided in one place, or the round trip diverges.
- The skill-kept gate is any `KindSkill` row with `Action == ActionKept`: edited without `--force`, or not a regular file even with `--force`. An absent SKILL.md or a force-removed one lets the entries be removed. This is Rule 8 plus uninstallLong's "unless the skill file itself is kept".
- The bound-agent uninstall row is `ActionRemoved` but the file is rewritten, not deleted. Its path goes to `Result.Modified`, never to `Removed`. `applyUninstall`'s remove loop must skip `KindBoundAgent`.
- A shape uninstall can't edit gets no row: a non-regular leaf, an escaping path, a quoted, commented or multi-line value, or an entry not listed. Ruled copy has only the `removed` row.
- There is no writability pre-check in Uninstall. None exists for any uninstall artifact, and `checkWritable`'s Fix copy ("apply the output below by hand") has no referent without `--print`. A failure falls under the plain-error or `ErrPartialWrite` contract. The concurrent-edit guard (`verifyFileUnchanged`, "brief uninstall") is kept.
- An invalid or missing `.brief.yaml` gives no bound rows, never a refusal, because uninstall's config removal stays digest-only.
- Emptying a block list drops the `skills:` key line, the same "drop when empty" rule as the flow list.

**Left unbuilt:**
- There is no `kept` row or copy for a bound agent uninstall can't edit. That belongs to final product-vision.

**Traps:**
- The `os.Remove` loop in `applyUninstall` removes every `ActionRemoved` row. Without the `KindBoundAgent` skip, it deletes the user's agent file.
- The skill is planned after the agents loop today. The bound-agent decision needs the skill row, but the bound rows sit before the agent rows. Plan first, then insert at the index.
- R16's byte identity holds only for no key, a non-empty block list, and canonical `skills: [a]`. An original `skills: []` loses its line (ruled drop), and `skills:[a]` or `skills:  [a]` comes back as `skills: [a]`, because `editFlowList` rewrites the prefix. Do not "fix" either without a ruling.
- `runUninstall` ignored `rs.setupOpts` before Step 16. A cli uninstall test that binds a bare name must use the `run()` seam with an injected home.
- A home directory under `t.TempDir()` sits outside root, so the escape check alone already excludes it. Any "`ScopeProject` excludes user scope" test must put home inside root, or it cannot fail.
- `Test_uninstall_treats_an_unparseable_or_invalid_config_as_edited` must stay green. Decode failures in the new `config.Inspect` call must never surface.
- UNRULED, owner final product-vision: (1) no row for bound agents uninstall can't edit; (2) no uninstall writability pre-check; (3) the `skills: []` round-trip loss; (4) the implementer-before-planner bound row order.
