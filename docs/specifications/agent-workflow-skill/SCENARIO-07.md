---
id: SCENARIO-07
status: done
---

# SCENARIO-07: init --edit-agents adds the skill to bound repository agents

## Scenario

Scenario: SCENARIO-07 init --edit-agents adds the skill to bound repository agents
  Applies the shape table (no key / block list / flow list / already listed / other / non-regular),
  never edits ~/.claude or brief:* bindings, honors --dry-run/--print and the writability pre-check,
  refuses (exit 2) without claude-code, says "nothing to edit" when none apply

## User-visible contract

- `brief init [--host claude-code] --edit-agents [--dry-run | --print] [--json]`
- stdout: one row per targeted agent file, kind `bound-agent`, after the `agent…` rows and
  before the `snippet` row:
  `merged <rel> (brief-workflow added to skills)` / `unchanged <rel>` /
  `kept <rel> (skills: is not a list brief can edit; add brief-workflow by hand)` /
  `kept <rel> (not a regular file)`.
- `--dry-run`: the same rows, nothing written. `--print`: `# <rel> (merge)` for each pending
  edit, body = only the inserted or rewritten line.
- stderr, exit 0, text mode only (never `--json`), when no target exists:
  `brief init: --edit-agents: no planner or implementer bound to an agent under .claude/agents; nothing to edit`
- stderr, exit 2, when the resolved host is not claude-code:
  `brief init: --edit-agents requires --host claude-code; run 'brief init --host claude-code --edit-agents'`
- An agent file whose parent directory cannot be written refuses the whole run under R10:
  `ErrUnwritable`, exit **1**, nothing written.
- A file changed between plan and write refuses `ErrConcurrentEdit`, same as the other merges.
- `agents_missing_skill` / the stderr missing-skill block list what is still missing *after*
  this run, or after the planned run under `--dry-run`/`--print`.

Existence facts came from `go doc` on `internal/setup` and `internal/platform/agentfile`, plus
targeted reads of `setup.Init`/`apply`/`writableTargets`/`printArtifacts`, `cli/init.go`
`runInit`/`initLong`, and `stepfile.SetStatus`. I did not open any earlier SCENARIO file.

## Implementation Plan

### Red
- [x] Step 1: `internal/platform/agentfile/agentfile_test.go` `Test_parse_decodes_frontmatter_from_bytes`: the new exported bytes-level decoder returns the same `Frontmatter` (loose `Skills`, quoted `"brief-workflow"` included) that `Load` returns for the same bytes. It also errors on a missing closing delimiter. Fails because `Parse` is missing.
- [x] Step 2: `internal/setup/bound_agent_internal_test.go` `Test_add_workflow_skill_edits_only_the_skills_line`: table over the full shape matrix. Each row asserts the whole expected file bytes, the returned line (with its whole terminator stripped, `\r` included) and the shape. Fails because no edit is produced. Rows:
  - **No top-level key, so insert.** LF and CRLF files. A nested `skills:` under another key (`metadata:\n  skills: [x]`). `skills:` text inside a `description: |` block scalar.
  - **Block list.** 0-, 2- and 4-space indent, and a CRLF file.
  - **Flow list.** One-line `[a]`, and `[]`.
  - **Already listed.** Membership is supplied by the caller from the decoded `Skills`, not decided by the transform: block, flow, and quoted `"brief-workflow"`.
  - **Other shapes.** Scalar, empty `skills:`, `null`, multi-line flow, flow with a trailing comment, mapping, non-scalar item.
  - **Body.** A `skills:` line after the closing `---` is untouched.
- [x] Step 3: `internal/setup/bound_agent_test.go` `Test_init_edit_agents_adds_the_skill_to_bound_project_agents`: `Init` with `EditAgents` on a bare implementer bound to `.claude/agents/developer/Agent.md`. Expects a `KindBoundAgent` `ActionMerged` row with the ruled detail, the exact edited bytes, the path in `Modified`, file mode `0o600` preserved, and `AgentsMissingSkill` empty. Fails because the file is unchanged and no row exists.
- [x] Step 4: `bound_agent_test.go` `Test_init_edit_agents_orders_bound_agent_rows`: existing kept config plus `WithAgents` plus `EditAgents`. Expects artifact kinds in the order `…skill, agent×3, bound-agent, snippet`. Fails because the row is missing.
- [x] Step 5: `bound_agent_test.go` `Test_init_edit_agents_rows_for_unchanged_and_kept_shapes`: an already-listed file gives `unchanged`; an other-shape file gives `kept` with the ruled detail, stays byte-identical, and stays in `AgentsMissingSkill`. A table across the shape matrix asserts that the path is in `AgentsMissingSkill` after the run iff it was lacking before and its row is not `merged`. Fails because the rows are missing.
- [x] Step 6: `bound_agent_test.go` `Test_init_edit_agents_leaves_non_targets_alone`: subtests below. Fails because the control arms are not edited.
  - A user-level `~/.claude/agents` definition gets no row, its bytes are unchanged, and it stays listed. Control: the same agent under the project gets merged.
  - `brief:*`, other-plugin and reviewer bindings get no row.
  - A symlinked leaf `.md` pointing at a file in another TempDir gives `kept (not a regular file)`, the target's bytes are unchanged, and the symlink is still a symlink. Control: the same bytes as a regular file get merged.
  - With `.claude/` symlinked outside the repo, the regular leaf gets no row, stays untouched and stays listed. Control: a real `.claude/` gets merged.
- [x] Step 7: `bound_agent_test.go` `Test_init_edit_agents_edits_a_shared_agent_once`: planner and implementer are both bound to `developer`. Expects exactly one `merged` row, no error, the skill inserted once, and `AgentsMissingSkill` empty. Fails on the ErrConcurrentEdit self-collision if edits are not deduped by path.
- [x] Step 8: `bound_agent_test.go` `Test_init_edit_agents_dry_run_and_print_write_nothing`: `DryRun` gives the same rows as a real run and leaves the file unchanged. `Print` gives one `PrintMerge` entry whose `Body` is the inserted line only. Under both, `AgentsMissingSkill` excludes the planned-edit path and still lists an other-shape one. Fails because the rows and print entry are missing and the list is not subtracted.
- [x] Step 9: `bound_agent_test.go` `Test_init_edit_agents_refuses_without_claude_code`: explicit `HostNone`, and detection that finds nothing, both return `ErrEditAgentsNeedHost`, and no config file is created. The detection arm must use `newServerWithHome` with an empty TempDir home. With `WithAgents` also set, `ErrAgentsNeedHost` wins. Fails because the sentinel is missing and the run succeeds.
- [x] Step 10: `bound_agent_test.go` `Test_init_edit_agents_joins_the_writability_precheck`: the agent's parent directory is `0o555`. Expects `ErrUnwritable` whose `RefusalError.Path` is that directory, with neither the config nor the agent written. Follow the chmod/skip pattern in `writable_test.go`. Fails because the run writes the config.
- [x] Step 11: `apply_internal_test.go` `Test_apply_refuses_a_bound_agent_changed_since_planning`: rewrite the file between plan and `apply`. Expects `ErrConcurrentEdit` and the concurrent bytes kept. Fails because apply overwrites.
- [x] Step 12: `missing_skill_test.go` `Test_init_reports_bound_agents_missing_the_workflow_skill`: add the S06 control arm "the same fixture with EditAgents changes the file". This proves the existing "left untouched" arm can fail. Fails because the file is unchanged.
- [x] Step 13: `internal/cli/init_internal_test.go` `Test_init_lists_bound_agents_missing_the_workflow_skill`: add an `--edit-agents` arm on the identical fixture, through the `run()` seam. The project `Agent.md` changes, `homePlanner` is byte-identical, stdout carries `merged .claude/agents/developer/Agent.md (brief-workflow added to skills)`, and the stderr block lists only the `~/.claude/agents/planner.md` line. Fails because the flag is unknown (exit 2 usage) and the file is unchanged.
- [x] Step 14: `init_internal_test.go` `Test_init_edit_agents_says_nothing_to_edit`: a config with no bare planner or implementer, and separately one whose only bare binding is user-level. Asserts the verbatim nothing-to-edit stderr line at the position this plan chose (after the `roles_to_add` block, before the missing-skill block and the next-action line; see Handoff) and exit 0. Also asserts the line is absent under `--json`. Fails because the line is missing.
- [x] Step 15: `init_internal_test.go` `Test_init_edit_agents_requires_claude_code`: `--host none --edit-agents`, and a bare temp repo with an empty home. Expects the verbatim exit-2 stderr line. Fails because the flag is unknown.
- [x] Step 16: `init_internal_test.go` `Test_init_edit_agents_dry_run_and_print`: `--dry-run` shows the same `merged` row and leaves the file unchanged. `--print` shows `# .claude/agents/developer/Agent.md (merge)` followed by the inserted line only, with no `\r` on a CRLF fixture. `--print --json` has an artifacts entry `action: merge` whose body is that line. Fails because the rows and entries are missing.
- [x] Step 17: `init_internal_test.go` `Test_init_edit_agents_json`: `artifacts` has a `kind: "bound-agent"`, `action: "merged"` row; `modified` holds the absolute path; `agents_missing_skill` excludes the edited agent. Fails because the row is missing.
- [x] Step 18: `internal/cli/help_test.go`: `init --help` carries the verbatim `--edit-agents` flag help and the full ruled `initLong` `--edit-agents` sentence. Fails because the text is missing.

### Green
- [x] Step 19: `internal/platform/agentfile/agentfile.go` `Parse(body []byte) (Frontmatter, error)`: exported thin wrapper over `decode`, the bytes-level twin of `Load`.
- [x] Step 20: `internal/setup/errors.go` `ErrEditAgentsNeedHost`: new sentinel beside `ErrAgentsNeedHost`.
- [x] Step 21: `internal/setup/setup.go` `InitRequest.EditAgents`, `KindBoundAgent = "bound-agent"`: request field and Kind constant.
- [x] Step 22: `internal/setup/bound_agent.go` `addWorkflowSkill`: pure, unexported frontmatter transform over the ruled shape table. It takes the bytes plus caller-supplied "already listed" membership. It splits on exactly `stepfile.DecodeFrontmatter`'s `\n---` prefix cut (following `stepfile.SetStatus`'s precedent) and gives an inserted line the terminator (`\r` kept) of the line it follows. Only a top-level `skills` key counts. It returns the edited bytes, the inserted or rewritten line (terminator stripped) and the shape.
- [x] Step 23: `bound_agent.go` `planBoundAgents` and a `boundAgentArtifact` type (Artifact plus existing bytes, edited bytes, line, perm). It walks planner then implementer through `agentfile.ResolveBinding`, keeping `BindingBare` only and each `Definition` with `ScopeProject`. It dedupes by path (first role wins). Lstat on a non-regular leaf gives `kept (not a regular file)`. A regular leaf gets no row when `filepath.Rel(EvalSymlinks(root), EvalSymlinks(path))` starts with `..`. Otherwise it reads the file once, decides membership with `agentfile.Parse` on that byte slice, and passes the same slice to `addWorkflowSkill` and as `existing`.
- [x] Step 24: `bound_agent.go` `writeBoundAgent`: atomic replace that keeps the file's Lstat'd permission bits, never `writePluginFile`'s fixed `0o644`.
- [x] Step 25: `setup.go` `(*Server).Init`: refuse `ErrEditAgentsNeedHost` after detection and after the `ErrAgentsNeedHost` check. Call `planBoundAgents` under claude-code plus `EditAgents`. Append bound-agent rows after the agent rows and before the snippet. Subtract merged paths from `agentsMissingSkill`'s result before the DryRun/Print return.
- [x] Step 26: `setup.go` `apply`: write bound-agent merges after `writeArts` and before the snippet. Each is guarded by `verifyFileUnchanged(path, true, existing, "brief init --edit-agents")` and appends to `Modified`, with the usual partial-write wrapping.
- [x] Step 27: `internal/setup/writable.go` `writableTargets`: include the path of every `ActionMerged` bound-agent row.
- [x] Step 28: `internal/setup/print.go` `printArtifacts`: add a `KindBoundAgent` case with `PrintMerge` and the line only. `kept` or `unchanged` bound-agent rows are never printed.
- [x] Step 29: `internal/cli/cli.go`: add the `edit-agents` flag with the verbatim ruled help (an `editAgentsFlagUsage` const beside `withAgentsFlagUsage`), add `[--edit-agents]` to the `init` synopsis after `[--with-agents]`, and pass the value to `runInit`.
- [x] Step 30: `internal/cli/init.go` `runInit`: take the `editAgents` param into `InitRequest`, map `ErrEditAgentsNeedHost` to `usageError` with the verbatim line, and emit the nothing-to-edit line when `editAgents` is set and `res.Artifacts` holds no `KindBoundAgent` row. That line is text mode only, including `--dry-run`/`--print`, never `--json`. Its place is after the `roles_to_add` block and before the missing-skill block, the unwritten line and the next-action line.
- [x] Step 31: `init.go` `initLong`: append the ruled sentence `--edit-agents adds it to those agents' "skills:" lists, for agent files under ".claude/agents/" only; one under "~/.claude" is always left for you to edit.` after `…whose agent lacks it.`

### Sweep
- [x] Step 32: fix what `go build ./... && golangci-lint run ./...` reports, including the `exhaustive` switches over `setup.Kind` (sweep).
- [x] Step 33: `internal/setup/doc.go`, `InitRequest`/`Result`/`Kind` doc comments, and the `runInit` doc: describe `EditAgents`, the `bound-agent` row, its place in write order, and the post-run missing-skill list (sweep).
- [x] Step 34: `internal/platform/agentfile/doc.go`: name `Parse`, and replace "a future setup edit rewrites…" with the present-tense fact that setup's `--edit-agents` edits a resolved project file (sweep).
- [x] Step 35: bump any exact-count or golden help/completion/help-json assertion that the new flag moves (sweep).

### Verify
- [x] Step 36: full verification per `.claude/rules/agent-briefs.md`, with `go test -race ./internal/setup/... ./internal/cli/... ./internal/platform/agentfile/...` and the test count delta reported. Mutate each guard individually and name the test that goes red:
  1. Drop the `ScopeProject` filter in `planBoundAgents`: the user-level subtest of `Test_init_edit_agents_leaves_non_targets_alone` goes red.
  2. Drop the leaf Lstat non-regular check: the symlinked-leaf subtest goes red because the outside target changes.
  3. Drop the `EvalSymlinks`/`Rel` containment check: the `.claude`-symlinked-outside subtest goes red.
  4. Normalize the inserted line's terminator to `\n` in `addWorkflowSkill`: the CRLF rows of `Test_add_workflow_skill_edits_only_the_skills_line` go red.

## Handoff

**Binding decisions:**
- **Targets.** Only bare-name planner and implementer bindings (`agentfile.BindingBare`) count. Every `Definition` with `ScopeProject` is a target, including duplicates. The same filter drives S06's report and S08's removal, which must mirror it.
- **Dedupe.** Edits are deduped by path, and the first role wins (planner). Two merges of one file would collide on `verifyFileUnchanged` against brief's own write.
- **Symlinked leaf.** A leaf `.md` that Lstat reports as non-regular (a symlink) is `kept <rel> (not a regular file)` and is never followed. The `atomicfile` rename would replace the link with a regular file.
- **Ancestor escape (UNRULED).** A regular leaf whose resolved path leaves the resolved root (a `.claude/` symlinked outside the repo) gets no row, is not edited, and stays in `agents_missing_skill` (Rule 3). No ruled row fits. Owner: final product-vision.
- **Membership.** "Already listed" is decided by `agentfile.Parse` (new, exported) over the same bytes that are edited and guarded as `existing`, the same loose decode `LackingSkill` sees. Text only decides shape, so the rows and the post-run `agents_missing_skill` cannot disagree.
- **Post-run missing list.** `AgentsMissingSkill` is S06's list minus every `merged` bound-agent path, computed before the DryRun/Print return.
- **`--print`.** Only pending (`merged`) edits print, as `PrintMerge`. The body is the inserted or rewritten line only. `kept` and `unchanged` rows never print.
- **Nothing-to-edit trigger.** Derived in cli: `--edit-agents` is set and there is no `KindBoundAgent` artifact. There is no Result field. It is text mode only (including `--dry-run`/`--print`), never `--json`.
- **Stderr placement (UNRULED).** Order is roles_to_add, nothing-to-edit, missing-skill block, then the next-action or unwritten line. Owner: final product-vision.
- **Host check order.** `ErrAgentsNeedHost` is checked before `ErrEditAgentsNeedHost`, and both come after host detection.
- **Synopsis.** `[--edit-agents]` in the `init` synopsis follows the `[--with-agents]` precedent. It is not new copy.
- **Concurrent-edit copy (UNRULED).** The refusal's rerun command is `brief init --edit-agents`. Owner: final product-vision.

**Left unbuilt:**
- The inverse of `addWorkflowSkill` (remove the entry, and drop a line left as `[brief-workflow]` or `[]`), the `removed <rel> (brief-workflow from skills)` row, and `uninstallLong`'s sentence belong to S08. Put the removal beside `addWorkflowSkill` in `internal/setup/bound_agent.go` and reuse `planBoundAgents`'s target filter.
- Uninstall plans no `KindBoundAgent` rows yet (S08).

**Traps:**
- On macOS, `t.TempDir()` sits under `/var`, which is a symlink to `/private/var`. Resolve both the path and the root with `EvalSymlinks`, or every fixture reads as "escaped" and the happy path fails for the wrong reason.
- `writePluginFile` and `writeSnippetFile` hardcode `0o644`, and the S06 fixtures write `0o600`. Reusing either writer silently widens a user's file permissions.
- `stepfile.DecodeFrontmatter` accepts CRLF, so a CRLF agent does reach the editor. The inserted line must carry `\r` (see `SetStatus`). The `--print` body strips it, because `renderPrint` appends `\n`.
- The decoder's closing delimiter is the prefix cut `\n---`, not a whole `---` line. The transform must cut the same way, or a `----` line splits differently in the two places.
- Containment must use `filepath.Rel` and reject a leading `..`, never a string-prefix test: `/repo` is a prefix of `/repo2`.
- `checkWritable` probes the parent directory, never the leaf. A read-only leaf in a writable directory still gets replaced by the rename. That matches R10 as ruled; do not add a leaf check.
- `cli.Run` exported tests cannot inject home. Every `--edit-agents` cli test uses `init_internal_test.go`'s `run()` seam with `withSetupOpts(setup.WithHomeDir(...))`.
