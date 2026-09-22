# init-doctor — current state

Scenarios complete: SCENARIO-01..10 — every spec scenario shipped. Fix passes 1-3: git-
boundary root scoping (R3), partial-write/`files_changed` consistency, a `*RefusalError`
type-preservation gap. Fix pass 4 (product-vision SHIP WITH CHANGES): pflag placeholder bug
on `--host`/`--hook`; `uninstall`'s "removed" line discriminates host-artifact-removed vs
config-only vs edited-kept; non-regular CLAUDE.md candidate is doctor WARN (was SKIP) and
still renders via `init --print`; agent role copy tightened; `init` names host-detection's
own signal (`Result.DetectedBy`); several fix-text/"skipped:"-prefix folds.

## Binding decisions
- `config.LocateWithin(dir, boundary)` bounds a walk at `boundary`, itself still checked, its
  parent never; `LocateInRepo` bounds it to `repo.Root(wd)`. `init`/`uninstall`/`doctor`/
  `check --hook` all call `LocateInRepo`; a config above that boundary is treated as absent.
- `internal/doctor`/`internal/setup` import only `internal/platform/*`, never each other or
  `scaffold`. `writable.Probe` is the one write-probe both share.
- `internal/platform/artifact` renders + digests every brief-written file; every
  `older…Digests` list ships empty (fix-pass-4's own agent-copy change stayed off it too,
  since the old bytes were never released), so `OriginOlder` is reachable only white-box.
- `init`/`uninstall` plan-then-apply, every refusal decided first; apply order: feature root,
  plugin/agent files, CLAUDE.md, config last. A non-regular CLAUDE.md candidate is kept, never
  followed/written; `setup`'s Detail and doctor's WARN both point at `brief init --print` —
  `printArtifacts` special-cases that one `ActionKept` shape (`notRegular`) to still emit the
  block's bytes. Doctor's WARN fires only once no candidate holds a real block.
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
- `uninstallNextAction` discriminates by what was removed, not requested host: a non-config
  `ActionRemoved` names the host install; a lone config removal names "brief's config"
  instead. Only Detail `"edited locally"` counts toward "N file(s) … kept" — `"not a regular
  file"` never does, since `--force` can't remove one either.

## Left unbuilt
- Line number for a value error, heading-shape rules; dry-run writability prediction (R10)
- Marker detection inside fenced code blocks; removal of `.claude/skills/`/`.claude/`
- A stray-plugin row (`wd ≠ root`); agent resolution by frontmatter `name:`;
  `~/.claude/skills/brief` (user-scope plugin) — none consulted
- A CLAUDE.md symlink to a recognized alternate (e.g. AGENTS.md) reads WARN/kept like any
  other symlink — never resolved to check its target. **Unowned.**

## Traps
- A fixture's own `.git` sets `LocateInRepo`'s boundary at `wd` — move it up to exercise a
  farther ancestor config.
- `--force init` over a non-default `feature-directory` leaves two feature roots, never removed.
- A round-trip "nothing written" assertion passes vacuously if init wrote nothing — assert a
  control run differs first.
- pflag's `UnquoteUsage` keeps the backticked word in the rendered text too — backtick a real
  value and it becomes the table placeholder; backtick a generic word instead.
- `os.Lstat` on a file-as-directory returns `ENOTDIR`, never `os.IsNotExist` — checked
  explicitly in `planPluginFile`/`checkWritable`.
- `newHealthyDoctorFixture` and the cli doctor goldens pin the full 12-row order/content; a
  doctor test leaving `WithHomeDir` unset reads the developer's real `~/.claude/agents`.
- A leaf's Usage line and root `cmdRow` share one `cmd.Use` — cobra has no wrap point for it,
  so the 80-column help test exempts lines starting `"  brief "` (init's own Use is 97 cols).

## Open debts
- setup never rewrites/removes an `OriginOlder` file — harmless while every older digest list
  ships empty; the first real one needs this, or doctor's fix can never clear its own WARN.
  **Unowned.**
- doctor's text rows are two-space-joined, not `tabwriter`-aligned — deferred twice now, ~12
  tests pin stdout byte-exact. **Unowned.**
- `hostPluginCheck`/`hostAgentsCheck` duplicate the same origin-check body — refactor-advisor
  MINORs, deferred rather than risk behavior change. **Unowned.**
