---
id: SCENARIO-06
status: done
---

# SCENARIO-06: init installs the Claude Code plugin

## Scenario

```gherkin
Scenario: SCENARIO-06 init installs the Claude Code plugin
  When I run "brief init --host claude-code"
  Then .claude/skills/brief/ holds the plugin manifest, skills start and finish (/brief:start, /brief:finish), and hooks/hooks.json running "brief check --hook claude-code"
  And --no-hook installs everything except the hook
  And re-running is "unchanged"; a locally edited file is "kept"; uninstall removes the directory and keeps edited files unless --force
```

Rules: R4 (plugin layout), R6 (ownership, digests, empty-dir cleanup), R11 (rows, next action,
JSON kinds), R14 (`--no-hook` in the usage line). Inherited context: `STATE.md` only.

External verification: the architect could not fetch the docs itself (no WebFetch, curl denied).
The coordinator checked code.claude.com/docs/en/plugins-reference#skills-directory-plugins and
/skills, and the plan uses what they found:
- Skills-dir plugin = folder with `.claude-plugin/plugin.json`; loads as `brief@skills-dir`;
  skills namespaced `/brief:<skill>`; `hooks/hooks.json` in the settings.json hook format.
- `plugin.json` `version` is optional; leaving it out is fine for skills-dir plugins.
- SKILL.md keys: `name`, `description`, `disable-model-invocation`, `allowed-tools`,
  `argument-hint`, `user-invocable`; the `$ARGUMENTS` placeholder is confirmed. brief writes
  `description`, `disable-model-invocation: true`, `allowed-tools`, `argument-hint`. `name`
  (defaults to the dir name) and `user-invocable` (default true) stay out.
- `allowed-tools` is a space/comma-separated string (or YAML list) using the `Bash(<prefix> *)`
  pattern: `Bash(brief start *)` / `Bash(brief finish *)`, not the `:*` form. Spec R4 is
  amended to match.
- Project-scope skills-dir plugins load ONLY from `<session primary working dir>/.claude/skills/`,
  with no walk-up, and only after workspace trust. SKILL.md edits apply immediately;
  hooks/agents need `/reload-plugins`. So the plugin goes under the install root (the
  `config.Locate` dir, else `wd`), and the next action tells the user to start Claude Code
  from that directory. Spec R11's next-action copy is amended to match.
- Removal = delete the folder (what uninstall does); `claude plugin disable brief@skills-dir`
  also exists and brief does not call it.

## User-visible contract

`brief init --host claude-code` in a fresh repo, stdout (one row per artifact, this order):

```
created .brief.yaml
created docs/specifications/
created .claude/skills/brief/.claude-plugin/plugin.json
created .claude/skills/brief/skills/start/SKILL.md
created .claude/skills/brief/skills/finish/SKILL.md
created .claude/skills/brief/hooks/hooks.json
```

stderr, exit 0. Install root is where `.brief.yaml` lands (`config.Locate`'s dir, else `wd`):
- root = wd: `brief init: installed for claude-code; start Claude Code in this directory (or run /reload-plugins in a session already here), then 'brief new feature <name>'`
- root ≠ wd (run from a subdirectory of a configured repo; `<rel>` = root relative to wd, e.g. `..`):
  `brief init: installed for claude-code in <rel>; start Claude Code in <rel> (or run /reload-plugins in a session already there), then 'brief new feature <name>'`

The plugin lands at `<root>/.claude/skills/brief/`, never under `wd` when they differ.

SKILL.md frontmatter: start = `description`, `disable-model-invocation: true`,
`allowed-tools: Bash(brief start *)`, `argument-hint: <feature>`; finish = same with
`Bash(brief finish *)` and `argument-hint: <feature> <step> --handoff <path> --state <path>`.
Body: run `brief start $ARGUMENTS` / `brief finish $ARGUMENTS` and work from its output.
`--no-hook`: same minus the `hooks/hooks.json` row. Re-run: every row `unchanged`, stderr
`brief init: already installed; nothing changed`. Edited plugin file: `kept <path> (edited
locally)`, under `--force` too. Path occupied by a non-regular file (dir, symlink): `kept <path>
(not a regular file)`. `--host none` stderr stays `installed config and feature root; run 'brief
new feature <name>'`. Unknown host (usage, exit 2): `brief init: unknown host "x"; expected one
of: claude-code, none; run 'brief init --host claude-code'` (S09 swaps the hint to `--print`).

`brief uninstall` (default host now `claude-code`): rows `removed`/`kept` for hooks.json,
finish, start, plugin.json, then `.brief.yaml` last; nothing installed → empty stdout,
`brief uninstall: nothing installed for claude-code`; `--host none` → `brief uninstall: nothing
installed` (no suffix). JSON `kind`: `plugin` for plugin.json + both SKILL.md, `hook` for
hooks.json; `host` echoes `claude-code`.

## Implementation Plan

Artifact bytes (platform, render + digest in one package):

- [x] Step 1: `internal/platform/artifact/artifact_test.go` `Test_recognize_classifies_each_plugin_file_against_its_own_kind` — table over the four new Kinds: own render → current, one-byte edit → edited, another Kind's render (start bytes as `KindSkillFinish`) → edited (red)
- [x] Step 2: `internal/platform/artifact/artifact_test.go` `Test_hooks_file_runs_brief_check_on_post_tool_use_edits` — decode `ClaudeHooks()` as JSON; `PostToolUse`, matcher `Edit|Write|MultiEdit`, `type: command`, command `brief check --hook claude-code` (red)
- [x] Step 3: `internal/platform/artifact/artifact_test.go` `Test_plugin_manifest_names_the_plugin_brief_and_carries_no_version` and `Test_skill_files_are_user_invoked_and_scoped_to_their_command` — frontmatter parsed per file: `description` non-empty, `disable-model-invocation: true`, `allowed-tools` = `Bash(brief start *)` / `Bash(brief finish *)`, `argument-hint` as in the contract, no `name`/`user-invocable`; body names `brief start $ARGUMENTS` / `brief finish $ARGUMENTS` (red)
- [x] Step 4: `internal/platform/artifact/plugin.go` — `PluginManifest()`, `SkillStart()`, `SkillFinish()`, `ClaudeHooks()` renders + Kinds `KindPluginManifest`, `KindSkillStart`, `KindSkillFinish`, `KindClaudeHooks`; `Render(Kind) []byte` with the same switch shape as `digestsFor`, so written bytes and recognized bytes are one value (new)
- [x] Step 5: `internal/platform/artifact/digest.go` — one digest list per new Kind, `digestsFor` cases; update `Kind`/`Recognize` doc comments (green)

Host layout (platform, host imports artifact; never the reverse):

- [x] Step 6: `internal/platform/host/claudecode_test.go` `Test_claude_code_plugin_layout_lists_every_file_and_drops_only_the_hook_without_it` — relative paths under `.claude/skills/brief/`, their artifact Kinds and the hook flag; `withHook=false` differs by exactly the hooks.json entry (red)
- [x] Step 7: `internal/platform/host/host.go` + `claudecode.go` — `File{RelPath, Kind artifact.Kind, Hook bool}` (no `Body` — setup writes `artifact.Render(f.Kind)`), `PluginDir` constant, `Host.Plugin(withHook bool) []File` on the interface and on the claude-code adapter; update package doc (green)

setup — init (feature package, Server-level tests against `t.TempDir`):

- [x] Step 8: `internal/setup/plugin_test.go` `Test_init_for_claude_code_writes_the_plugin_after_the_feature_root_and_before_the_config` — rows in the contract order, kinds plugin/hook, files on disk equal their renders, Created = files only, `Result.Root` = install root (red)
- [x] Step 8b: `internal/setup/plugin_test.go` `Test_init_from_a_subdirectory_installs_the_plugin_at_the_config_root` — ancestor `.brief.yaml`; plugin under the ancestor, nothing under `wd/.claude` (red)
- [x] Step 9: `internal/setup/plugin_test.go` `Test_init_with_no_hook_installs_everything_but_the_hook` — control arm is Step 8's run, differing only in `NoHook` (red)
- [x] Step 10: `internal/setup/plugin_test.go` `Test_rerunning_init_for_claude_code_reports_every_plugin_file_unchanged` (red)
- [x] Step 11: `internal/setup/plugin_test.go` `Test_init_keeps_an_edited_plugin_file_even_under_force` — edited SKILL.md kept with bytes untouched while `--force` rewrites `.brief.yaml` in the same run (the config arm is the control) (red)
- [x] Step 12: `internal/setup/plugin_test.go` `Test_init_keeps_a_plugin_path_that_is_not_a_regular_file` — directory at `SKILL.md` path, and a symlink; neither followed nor written (red)
- [x] Step 13: `internal/setup/plugin_test.go` `Test_init_dry_run_for_claude_code_writes_nothing` — same rows as the real run, `.claude/` absent afterwards (red)
- [x] Step 14: `internal/setup/plugin_test.go` `Test_a_plugin_write_failure_after_the_feature_root_is_a_partial_write_and_leaves_no_config` — read-only `.claude/skills/brief/`; skip under `os.Geteuid()==0` (red)
- [x] Step 15: `internal/setup/setup.go` — `HostClaudeCode` (= `host.ClaudeCode`), `Hosts()` returns `claude-code, none`; `InitRequest.NoHook`; `Result.Root` (absolute install root); `KindPlugin`/`KindHook`; `planPluginFiles` (Lstat, `artifact.Recognize`, never decode); `apply` writes feature root, plugin files (MkdirAll parents 0o755, files 0o644), config last; update package doc, `Result`/`Init` docs (green)

setup — uninstall:

- [x] Step 16: `internal/setup/uninstall_test.go` `Test_uninstall_for_claude_code_removes_the_unedited_plugin_and_its_empty_directories` — rows hooks, finish, start, manifest, config last; `.claude/skills/brief/` gone; `.claude/skills/` and `.claude/` still present (red)
- [x] Step 17: `internal/setup/uninstall_test.go` `Test_uninstall_keeps_an_edited_plugin_file_and_the_directories_holding_it_unless_forced` — table: no force → kept + parent dirs survive; force → removed + dirs pruned (red)
- [x] Step 17b: `internal/setup/uninstall_test.go` `Test_uninstall_after_a_no_hook_init_removes_the_three_files_and_the_directory` — `hooks/` never existed: no hook row, no error, `.claude/skills/brief/` still pruned (red)
- [x] Step 18: `internal/setup/uninstall_test.go` `Test_uninstall_keeps_a_plugin_directory_holding_a_file_brief_did_not_write` — `skills/extra/notes.md` survives with `skills/` and `brief/`; brief's own files in the same run are removed (control) (red)
- [x] Step 19: `internal/setup/uninstall_test.go` `Test_uninstall_for_host_none_leaves_the_plugin_in_place` — control arm: same tree under `claude-code` removes it (red)
- [x] Step 20: `internal/setup/roundtrip_test.go` `Test_init_then_uninstall_for_claude_code_leaves_pre_existing_claude_files_byte_identical` — snapshot (path → mode + bytes, dirs included) of a tree with `.claude/settings.json` and `.claude/skills/other/SKILL.md`; control: the post-init snapshot differs and holds brief's four files (red)
- [x] Step 21: `internal/setup/uninstall.go` — plan host files ahead of the config (hooks, finish, start, manifest); `applyUninstall` removes files, then prunes brief's own dirs deepest-first (`skills/start`, `skills/finish`, `skills`, `hooks`, `.claude-plugin`, `brief`) only when present and `os.ReadDir` is empty, via `os.Remove`, never above `PluginDir`; a missing plugin file is no row and no error; `Removed` lists files only (symmetric with `Created`); config last; update docs (green)

  Prune boundary: R6 "brief owns the plugin directory" makes `.claude/skills/brief/` the ownership line. `.claude/` and `.claude/skills/` belong to the host and may hold (or later receive) the user's own files; with no manifest brief cannot tell whether it created them, so an empty `.claude/` left behind by a round trip in a repo that had none is accepted, not a defect.

CLI surface (parsing + rendering only):

- [x] Step 22: `internal/cli/init_test.go` `Test_init_for_claude_code_installs_the_plugin_and_says_where_to_start_claude_code` + `Test_init_from_a_subdirectory_names_the_install_root_in_the_next_action` (root ≠ wd form; control is the root = wd form) + `Test_init_no_hook_omits_the_hook_row` + `Test_no_hook_with_host_none_is_accepted_and_changes_nothing` — exact stdout rows and stderr through `cli.Run`, exit 0 (red)
- [x] Step 23: `internal/cli/init_test.go:160`, `internal/cli/uninstall_test.go:123` — golden `expected one of: claude-code, none` (update)
- [x] Step 24: `internal/cli/uninstall_test.go` — `:86` golden becomes `nothing installed for claude-code` with `--host` omitted; new `--host none` case keeps `nothing installed` (update)
- [x] Step 25: `internal/cli/init_json_test.go` / `uninstall_json_test.go` — artifacts carry `kind` `plugin`/`hook`, `host` `claude-code`, `created`/`removed` list files only, never directories (red)
- [x] Step 26: `internal/cli/help_test.go` — init usage line gains `[--no-hook]`; `--no-hook` flag usage present (red)
- [x] Step 27: `internal/cli/cli.go` — register `--no-hook` (`noHookFlagUsage`), thread into `runInit`; `hostFlagUsage` / `uninstallHostFlagUsage` list `claude-code` and `none` (update)
- [x] Step 28: `internal/cli/init.go` — `initNextAction` takes the host and the install root relative to wd (from `Result.Root`); the two claude-code forms pinned above; `initLong` and `initDocument` doc mention the plugin (update)
- [x] Step 29: `internal/cli/uninstall.go` — default host `setup.HostClaudeCode`; `uninstallNextAction` takes the host and appends ` for <host>` unless `none`; `uninstallInvocation` → `brief uninstall --host claude-code` (and `initInvocation` → `brief init --host claude-code` in init.go, Step 28); `uninstallLong` names the plugin (update)
- [x] Step 30: `internal/cli/artifact_render.go` — confirm per-file rows need no change (no trailing `/` on plugin rows); update doc if the kinds list is named there (update)

Verification:

- [x] Step 31: mutation-verify, each alone, copy the file to `$TMPDIR` and restore then `diff` (not git stash — shared stash stack): (a) drop the `ReadDir`-empty guard → Step 18 red; (b) make `NoHook` a no-op → Step 9 red; (c) edited plugin file → `ActionRemoved` without force in uninstall → Step 17 red, config tests stay green; (d) init overwrites an edited plugin file under `--force` → Step 11 red; (e) prune one level past `PluginDir` → Step 16 red; (f) swap apply order to write config before plugin files → Step 14 red. Report which test each reddened.
- [x] Step 32: `go build ./...`, `go test ./...` (unpiped, exact count + delta), `go test -race ./internal/setup/... ./internal/cli/... ./internal/platform/...`, `golangci-lint run ./...`, `go doc ./internal/setup` and `./internal/platform/{artifact,host}` read as contracts → mark SCENARIO-06 done in specification.md, rewrite STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Renders and digests of plugin files live in `internal/platform/artifact` (one `artifact.Kind` per file shape, own digest list each); `internal/platform/host` owns only the layout via `Host.Plugin(withHook) []host.File` and imports `artifact`, never the reverse — digests are computed from the render in the same package (S10's `OriginOlder` appends there)
- `plugin.json` carries no `version` — renders must be deterministic so a digest distinguishes "older release" from "edited" (R6/S10); a version would change every release's bytes
- One row per file (R11): uninstall's mixed case (one file kept, others removed) and JSON's per-artifact `kind` cannot fit one aggregated row. Directories never get rows
- `setup.Kind` (R11 JSON `kind`: `plugin` = manifest + both skills, `hook` = hooks.json) is a different vocabulary from `artifact.Kind` (digest list per file)
- init row order: config, feature root, plugin.json, start, finish, hooks.json. Apply order: feature root, plugin files, `.brief.yaml` last (opt-in marker; the hook is silent until it lands). S07 snippet and S08 agents go between plugin files and config
- init `--force` never rewrites an edited plugin file (R3: `--force` = rewrite `.brief.yaml` only); uninstall `--force` does remove it (R6). The asymmetry is deliberate
- A non-regular file at a plugin path is `kept (not a regular file)` by init and uninstall alike; Lstat, never followed
- `init --no-hook` never plans hooks.json at all — no row, and an existing one is left untouched; `--no-hook` with `--host none` is an accepted no-op (S09 detection may pick none)
- Uninstall's default host is `claude-code` — its plan is a superset of `none`'s, so omitted `--host` removes everything; S09 must not switch uninstall to detection. `--host none` removes only the config. Stderr suffix ` for <host>` only when host ≠ none
- Uninstall removal order: hooks.json, finish, start, plugin.json, pruned dirs (deepest first), `.brief.yaml` last. `Created`/`Removed` list files (and init's feature root) only, never plugin directories
- setup writes `artifact.Render(kind)` — `host.File` has no body, so a written file can never disagree with its own digest list
- The plugin is installed at `<install root>/.claude/skills/brief/`, with install root = `config.Locate`'s dir, else `wd` (same root rule as the config). Claude Code loads project skills-dir plugins only from the session's primary working dir, no walk-up, so init's next action names that directory. `Result.Root` carries it
- `allowed-tools` uses `Bash(brief start *)` / `Bash(brief finish *)` (docs-verified; not `:*`). A later render change appends a digest, never replaces one
- `setup.Hosts()` = `claude-code, none` keyed off `host.ClaudeCode`; `host.HookHosts()` stays separate (check --hook's list)

**Left unbuilt** — named so nobody assumes it exists:
- `snippet`/`merged` kind, CLAUDE.md block, `Modified` population — S07
- `agents/{planner,implementer,reviewer}.md`, `--with-agents`, KindAgent — S08
- `--print`, host detection (init default stays `none`), swapping `initInvocation`'s hint to `--print`, R10 writability pre-check — S09
- `artifact.OriginOlder`, doctor rows `host-plugin`/`host-hook` — S10. S10 may want a WARN when the plugin root is not where Claude Code would load it. doctor cannot know the launch dir, so the only checkable form is "plugin exists but not at the install root" (e.g. a stray `wd/.claude/skills/brief/`). S10 decides, or skips it with a stated reason
- `claude plugin disable brief@skills-dir` — never invoked; uninstall deletes the folder (the documented removal)
- Removal of `.claude/skills/` or `.claude/` created by init — never (cannot tell who created them)

**Traps** — things that look right and are not:
- Empty-dir pruning must stop at `.claude/skills/brief/` inclusive; `os.Remove` only, emptiness checked by `os.ReadDir` first — relying on `os.Remove`'s ENOTEMPTY error is platform-shaped
- The sandbox denies writes to the worktree's own `.claude/skills` — tests must use `t.TempDir`, never the repo tree
- A plugin under a subdirectory's `.claude/skills/` does not load when Claude Code starts at the repo root, and vice versa: no walk-up. Writing under `wd` instead of the install root would look fine in a root-level test, which is why Step 8b exists
- Every CLI/setup test must pass `--host claude-code` explicitly; the default is still `none` for init
- A round-trip "byte-identical" assertion passes if init wrote nothing — the post-init snapshot must be shown to differ
- `chmod 0o555` partial-write tests pass vacuously as root — skip under `os.Geteuid()==0`
