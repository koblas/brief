# init-doctor — current state

Scenarios complete: SCENARIO-01..10 — every spec scenario shipped. Fix passes 1-3: git-
boundary root scoping (R3), partial-write/`files_changed` consistency, a `*RefusalError`
type-preservation gap. Fix pass 4 (product-vision SHIP WITH CHANGES): pflag placeholder bug
on `--host`/`--hook`; `uninstall`'s "removed" line discriminates host-artifact-removed vs
config-only vs edited-kept; non-regular CLAUDE.md candidate is doctor WARN (was SKIP) and
still renders via `init --print`; agent role copy tightened; `init` names host-detection's
own signal (`Result.DetectedBy`). Fix pass 5 (re-gate): host-snippet's WARN now considers
only the one candidate `planSnippet` would choose, never a farther candidate's own shape;
`uninstall --dry-run` discriminates by the computed plan exactly like a real run instead of
naming a host install unconditionally; `planSnippetRemoval`'s kept detail no longer borrows
`planSnippet`'s install-side copy; the "N file(s) … kept" count moved off `Detail` text onto
a typed `Artifact.ForceRemovable`; the pflag-placeholder fix (fix pass 4) finally got a test.

## Binding decisions
- `config.LocateWithin(dir, boundary)` bounds a walk at `boundary`, itself still checked, its
  parent never; `LocateInRepo` bounds it to `repo.Root(wd)`. `init`/`uninstall`/`doctor`/
  `check --hook` all call `LocateInRepo`; a config above that boundary is treated as absent.
  Refuses as `*InvalidConfigError` (`errors.Is(err, ErrInvalidConfig)` holds too); callers
  type-assert the concrete type for `Path`/`Err`.
- `internal/doctor`/`internal/setup` import only `internal/platform/*`, never each other or
  `scaffold`. `writable.Probe` is the one write-probe both share.
- `internal/platform/artifact` renders + digests every brief-written file; every
  `older…Digests` list ships empty, so `OriginOlder` is reachable only white-box.
- `init`/`uninstall` plan-then-apply, every refusal decided first; apply order: feature root,
  plugin/agent files, CLAUDE.md, config last. A non-regular CLAUDE.md candidate is kept, never
  followed/written; `setup`'s install-side Detail ("…add the block by hand, see 'brief init
  --print'") and doctor's WARN both point at `--print`; the removal-side Detail
  (`planSnippetRemoval`) is the plain "not a regular file" — there is nothing to add by hand
  on a removal, and the two wordings are free to differ without breaking anything, since
  `ForceRemovable` (not Detail text) is what callers key behavior on. Doctor's host-snippet
  WARN fires only for the one candidate `planSnippet`/`chooseSnippetLocation` would itself
  pick — the first candidate present at all, regular or not, in `host.InstructionFiles`
  priority order — never a farther candidate's own shape; a regular-but-blockless first
  candidate is the ordinary SKIP `not installed`, matching what `init` would actually do.
- `setup.Artifact.ForceRemovable` is true only on an Uninstall-side `ActionKept` artifact
  `--force` can turn into `ActionRemoved` (an edited file: `planPluginRemoval`,
  `planConfigRemoval`, `planSnippetRemoval`'s own non-`OriginCurrent` arm) — false on a
  non-regular kept artifact and on every Init-side artifact, where the field is unused.
  `cli.uninstallNextAction` switches on it, not on `Detail == "edited locally"`.
- Role bindings come only from `artifact.AgentBindings()`, written only into a config `Init`
  creates/`--force`-rewrites; `roles` resolves `brief:<name>` via plugin-or-project, bare
  `<name>` via project-or-injected-home; WARN never ERROR.
- `check --hook`'s opt-in gate runs before stdin is parsed; a malformed payload inside an
  opted-in repo is exit 1, one stderr line, never usage-error's exit 2.
- Host detection (`setup` only): `Host == ""` → root `.claude`/`CLAUDE.md`, else
  `WithHomeDir`'s `.claude` → `HostClaudeCode`; else `HostNone`. `detectHost`'s third return
  (`Result.DetectedBy`: `.claude`, `CLAUDE.md`, `~/.claude`) is `""` when `Host` was explicit
  — `initNextAction`'s "(detected …)" clause and `--json`'s `detected_by` gate on it non-empty.
- `doctor` appends `host-plugin, host-hook, host-snippet, host-agents, roles` after `env-path`;
  `host.originRow` decides every row's WARN-older/OK-edited/OK-current triple.
- A partial write returns the populated `Result`, wrapped `ErrPartialWrite`; cli prints landed
  rows before the refusal; `--json` drops the `files_changed`-contradicting "(no files
  changed)" tail on that same wrap.
- `uninstallNextAction` discriminates by what the plan holds — dry run or real, both read the
  plan the same way, only the phrasing (promise vs report) differs: a non-config
  `ActionRemoved` names the host install; a lone config removal names "brief's config"; any
  `ForceRemovable` `ActionKept` counts toward "N file(s) … kept"; zero artifacts is "nothing
  installed"; anything else is "nothing removed".

## Left unbuilt
- Line number for a value error, heading-shape rules; dry-run writability prediction (R10)
- Marker detection inside fenced code blocks; removal of `.claude/skills/`/`.claude/`
- A stray-plugin row (`wd ≠ root`); agent resolution by frontmatter `name:`;
  `~/.claude/skills/brief` (user-scope plugin) — none consulted
- A CLAUDE.md symlink to a recognized alternate (e.g. AGENTS.md) reads WARN/kept like any
  other symlink — never resolved to check its target. **Unowned.**
- `cli.uninstallNextAction` (~65 lines, artifact-classification loop plus a 4-way dry-run/real
  switch) and its own 20-line doc comment are still one function — refactor-advisor MINOR,
  extracting a `classifyUninstallResult` helper, deferred as fix-if-cheap. **Unowned.**

## Traps
- A fixture's own `.git` sets `LocateInRepo`'s boundary at `wd` — move it up to exercise a
  farther ancestor config.
- `--force init` over a non-default `feature-directory` leaves two feature roots, never removed.
- A round-trip "nothing written" assertion passes vacuously if init wrote nothing — assert a
  control run differs first.
- pflag's `UnquoteUsage` keeps the backticked word in the rendered text too — backtick a real
  value and it becomes the table placeholder; backtick a generic word instead. Now pinned by
  `Test_host_and_hook_flags_render_a_generic_table_placeholder` (help_test.go) — mutation-
  verified per flag.
- `os.Lstat` on a file-as-directory returns `ENOTDIR`, never `os.IsNotExist` — checked
  explicitly in `planPluginFile`/`checkWritable`.
- `newHealthyDoctorFixture` and the cli doctor goldens pin the full 12-row order/content; a
  doctor test leaving `WithHomeDir` unset reads the developer's real `~/.claude/agents`.
- A leaf's Usage line and root `cmdRow` share one `cmd.Use` — cobra has no wrap point for it
  (init's own Use is 97 cols); the 80-column help test exempts it by position (the one
  content line right after the literal "Usage:" line), never by a "  brief " prefix match,
  which also caught unrelated wrapped lines.
- `setup.Artifact` has both install-side and removal-side construction sites sharing one
  struct; a test comparing a literal `setup.Artifact{...}` must include `ForceRemovable`
  whenever the artifact came from `srv.Uninstall`, or the comparison silently expects the
  zero value (false).

## Open debts
- setup never rewrites/removes an `OriginOlder` file — harmless while every older digest list
  ships empty; the first real one needs this, or doctor's fix can never clear its own WARN.
  **Unowned.**
- doctor's text rows are two-space-joined, not `tabwriter`-aligned — deferred twice now, ~12
  tests pin stdout byte-exact. **Unowned.**
- `hostPluginCheck`/`hostAgentsCheck` duplicate the same origin-check body — refactor-advisor
  MINORs, deferred rather than risk behavior change. **Unowned.**
- `host_test.go`/`detect_test.go`'s classification tables carry no mutation-verification
  statement on most cases, unlike their siblings (fix pass 5 added two that do). **Unowned.**
