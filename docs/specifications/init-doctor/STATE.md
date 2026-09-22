# init-doctor — current state

Scenarios complete: SCENARIO-01..06. Last updated by SCENARIO-06.

## Binding decisions
- `config.Locate` is the one walk-up; `config.Inspect` the one all-violations decoder;
  `Resolve` = `Locate` + first `Inspect` element. `doctor`, `init`, `uninstall`, `check --hook`
  and the claude-code plugin's own install root all build on this — no second walk-up
  anywhere. Root = `Locate`'s dir, else `wd`; Claude Code loads a project skills-dir plugin
  only from the session's own working dir, no walk-up, so `init`'s next action names that
  root (`Result.Root`) whenever it differs from `wd` (S01-06)
- `internal/doctor` and `internal/setup` import only `internal/platform/*` + stdlib, never
  each other or `scaffold`; neither has a `Store` port (S02, 03, 06)
- `internal/platform/artifact` renders + digests every brief-written file: `ConfigFile()`,
  `PluginManifest()`, `SkillStart()`, `SkillFinish()`, `ClaudeHooks()`, `Recognize(Kind,
  body) Origin`, `Render(Kind) []byte` (written and recognized bytes are one value, so a
  digest mismatch is always real). Every render is deterministic; a change appends a digest,
  never replaces one (S03, 06)
- `internal/platform/host` owns layout: `Host{Name, HookPath, WriteHookContext,
  Plugin(withHook bool) []File}`; `File{RelPath, Kind artifact.Kind, Hook bool}` carries no
  body. `host.PluginDir = ".claude/skills/brief"`. `host` imports `artifact`, never the
  reverse; `cli` and `setup` both consume `host`, never each other (S05, 06)
- `init`/`uninstall` plan-then-apply, every refusal decided before the first write/removal.
  Init apply order: feature root, plugin files (`ActionCreated` only), config last; a
  mid-sequence failure wraps `ErrPartialWrite`. `--force` only rewrites `.brief.yaml` from
  defaults — never an edited plugin file, only `Uninstall --force` removes one (S03, 06)
- `Uninstall` recognizes every artifact (config, plugin files) by digest only, never
  decoding; unparseable/invalid/edited is "edited locally", kept unless `--force`; a
  non-regular path is "kept (not a regular file)", Lstat only, never followed. Its own
  artifact order is the reverse of `Plugin`'s install order (hook, finish, start, manifest),
  config last; `applyUninstall` then prunes `pluginPruneDirs` deepest-first (stopping at
  `host.PluginDir` inclusive) only when `os.ReadDir` is empty; `Removed`/`Created` list files
  only, never a directory (S04, 06)
- `setup.Kind` (JSON `kind`: `config`, `feature-root`, `plugin`, `hook`) is a different
  vocabulary from `artifact.Kind` (one per render/digest list). `setup.Hosts()` =
  `claude-code, none` keyed off `host.ClaudeCode`; `host.HookHosts()` stays separate. `init`'s
  default host stays `none` (S09 adds detection); `uninstall`'s default is `claude-code`
  (superset of `none`) — its stderr suffix ` for <host>` applies to every branch except
  `--host none`. `--no-hook` never plans hooks.json (no row; existing one untouched);
  `--no-hook --host none` is an accepted no-op (S06)
- `allowed-tools` uses `Bash(brief start *)` / `Bash(brief finish *)` (docs-verified, not
  `:*`); `plugin.json` carries no `version` key (S06)
- CLI: `artifact_render.go` is the one row/JSON renderer, no kind-specific branching beyond
  the feature root's trailing `/`. Command order: `new, start, finish, status, check, init,
  doctor, uninstall`. `classifyRefusal` order: `*setup.RefusalError`,
  `*config.InvalidConfigError`, `*unknownFeatureError`, `*scaffold.RefusalError`,
  `*assemble.RefusalError` (S03, 04)
- Path→feature mapping is `assemble.(*Server).FeatureContaining`: lexical `filepath.Rel`
  first, `EvalSymlinks` retry only when lexical says outside (S05)
- `check --hook <host>`: ERROR findings on the edited feature → exit 0, one
  `hookSpecificOutput.additionalContext` document on stdout, stderr empty; no ERROR is fully
  silent; config located from injected `wd`, never the payload's `cwd` (S05)

## Left unbuilt
- Line number for a value error, heading-shape rules, absolute-path/`..` check on
  `feature-directory`; validation of `roles.*`/`optional-conventions` — unowned/S08 (S01)
- `KindSnippet`/`KindAgent`, CLAUDE.md block, `Modified` population — S07
- `agents/{planner,implementer,reviewer}.md`, `--with-agents`, `roles:` config lines — S08
- `--print`, host detection, R10 writability pre-check, `initInvocation`'s hint → `--print`
  — S09
- `artifact.OriginOlder`, doctor rows `host-plugin`/`host-hook`/`host-snippet`/`host-agents`,
  env-path's ERROR arm — S10 (may want a WARN for a plugin present but not at the install
  root; doctor cannot know the launch dir, so only "stray" is checkable — S10 decides)
- `claude plugin disable brief@skills-dir` — never invoked; uninstall deletes the folder
- Removal of `.claude/skills/` or `.claude/` created by init — never; brief owns only
  `host.PluginDir` and below
- Hook payload's `cwd`, `tool_name`, `hook_event_name` — unowned

## Traps
- `t.TempDir` has no `.git` above it; `chmod 0o000`/`0o555` tests pass vacuously as root —
  skip under `os.Geteuid() == 0` (S02, 04, 06)
- `doctor.Report.Counts()`'s switch has no `default` arm — a fifth `Severity` without a
  matching `case` makes the count invariant uncaught (S02)
- `--force init` over a non-default `feature-directory` leaves two feature roots; neither
  `init` nor `uninstall` ever removes either (S03, 04)
- `filesChangedFor` only returns non-nil for `writesFilesAnnotation` commands (S03, 04)
- `t.TempDir()` on macOS resolves through a symlink (`/var` → `/private/var`) — lexical match
  first, `EvalSymlinks` only as fallback, is load-bearing (S05)
- `check --hook`'s exit 0 is both "silent" and "findings" — a silent-0 test needs a control
  arm whose stdout is the non-empty document (S05)
- Uninstall's empty-dir pruning must stop at `host.PluginDir` inclusive; one non-brief file
  one level below it (e.g. `skills/extra/`) must block pruning of every ancestor up to
  `PluginDir` (S06)
- A round-trip "byte-identical" assertion passes if init wrote nothing — the post-init
  snapshot must be shown to differ from the pre-init one (S04, 06)
- pflag's `UnquoteUsage` recognizes only one backtick-quoted span per usage string; a second
  pair shifts the flag table's column width and can push another flag's line past 80 columns
  (S06)

## Open debts
- `roles.*` / `optional-conventions` validation — S08 must close it, or the spec must say
  explicitly these stay unvalidated
