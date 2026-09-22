# init-doctor — current state

Scenarios complete: SCENARIO-01..07. Last updated by SCENARIO-07.

## Binding decisions
- `config.Locate` is the one walk-up; `config.Inspect` the one all-violations decoder;
  `Resolve` = `Locate` + first `Inspect` element. `doctor`, `init`, `uninstall`, `check --hook`
  and the plugin's own install root all build on this — no second walk-up anywhere. Root =
  `Locate`'s dir, else `wd`; `init`'s next action names root (`Result.Root`) when it differs
  from `wd` (S01-07)
- `internal/doctor` and `internal/setup` import only `internal/platform/*` + stdlib, never
  each other or `scaffold`; neither has a `Store` port (S02, 03, 06, 07)
- `internal/platform/artifact` renders + digests every brief-written file. The CLAUDE.md
  block is the one parameterized artifact: `SnippetBlock(dir)`/`RecognizeSnippet(block)
  SnippetMatch` are its own render/recognize, deliberately off `Render`/`Recognize`/
  `digestsFor` (explicit `KindSnippet` cases return nil, satisfying `exhaustive`).
  `RecognizeSnippet` is config-independent: matches a block written for *any* dir, reports
  which one — caller compares to its own configured dir for unchanged vs "block updated" (S03,
  06, 07)
- `internal/platform/host.Host` = `{Name, HookPath, WriteHookContext, Plugin(withHook bool)
  []File, InstructionFiles() []string}`. `InstructionFiles()` = `["CLAUDE.md",
  ".claude/CLAUDE.md"]`, no filesystem access. `host` imports `artifact`, never reverse (S05,
  06, 07)
- `init`/`uninstall` plan-then-apply, every refusal decided before the first write/removal.
  Init apply order: feature root, plugin files (created only), CLAUDE.md block (created or
  merged), config last. Uninstall plans/applies the block first, then plugin files reverse
  order, config last. `--force` rewrites `.brief.yaml` from defaults and removes an edited
  CLAUDE.md block/plugin file under Uninstall — Init never rewrites an edited artifact,
  `--force` included (S03, 06, 07)
- CLAUDE.md location (`internal/setup/snippet.go`): a candidate already holding a recognized
  block wins; else first existing candidate; else root is created —
  `scanSnippetCandidates`+`chooseSnippetLocation`, shared by Init/Uninstall. Two candidates
  each holding a block refuses naming `.claude/CLAUDE.md` (root preferred) + begin line. A
  marker defect refuses per-file, checked on *both* candidates. CRLF refuses only the one
  candidate resolved to act on (`crlfRefusal`, post-location) — deliberate, so an untouched
  Windows-authored CLAUDE.md never blocks a run aimed at the other candidate (S07)
- `mergeSnippet`/`removeSnippet` are exact inverses via the append separator (ends-in-`\n`
  existing → `\n`+block+`\n`; no trailing `\n` → `\n\n`+block, nothing after) — makes the
  round trip byte-identical with no sidecar. An emptied CLAUDE.md is deleted on Uninstall
  (only "brief created it" signal), so a pre-existing empty file is deleted too, never
  restored (S07)
- `setup.Kind` gained `snippet`; `setup.Action` gained `ActionMerged`; `Result.Modified` is
  now populated (Init's merge, Uninstall's strip). `--no-hook` never affects the snippet — it
  is never a `host.File` (S06, 07)
- `initNextAction` treats both `ActionCreated` and `ActionMerged` as "changed" — a merge-only
  run must never say "already installed; nothing changed" (S03, 04, 07)
- `check --hook <host>`: ERROR findings → exit 0, `additionalContext` on stdout; config
  located from injected `wd`, never the payload's `cwd` (S05)

## Left unbuilt
- Line number for a value error, heading-shape rules, `roles.*`/`optional-conventions`
  validation — unowned/S08 (S01)
- `KindAgent`, `agents/{planner,implementer,reviewer}.md`, `--with-agents`, `roles:` config
  lines — S08
- `--print`, host detection, R10 writability pre-check — S09
- `artifact.OriginOlder` (snippet + plugin), doctor rows `host-plugin`/`host-hook`/
  `host-snippet`/`host-agents`, env-path's ERROR arm — S10
- Marker detection inside fenced code blocks — never handled; any exact marker line counts
- Removal of `.claude/skills/`/`.claude/` created by init — never; brief owns only
  `host.PluginDir` + the CLAUDE.md candidates `InstructionFiles` names

## Traps
- `t.TempDir` has no `.git` above it; `chmod` tests pass vacuously as root — skip under
  `os.Geteuid() == 0` (S02, 04, 06)
- `--force init` over a non-default `feature-directory` leaves two feature roots, never
  removed (S03, 04)
- `t.TempDir()` on macOS resolves through a symlink — lexical match first, `EvalSymlinks`
  fallback (S05)
- A round-trip "byte-identical" assertion passes if init wrote nothing — assert the post-init
  snapshot differs first (S04, 06, 07)
- A claude-code Init always creates `.claude/skills/brief/...`; Uninstall prunes empty dirs
  only down to and including `host.PluginDir` — `.claude/skills/`/`.claude/` always survive,
  empty. A tree-snapshot round-trip test must expect both even when the fixture never
  pre-created them (S06, 07)
- `mergeSnippet`/`removeSnippet` round-tripping proves they are mutual inverses, not that
  either matches the *specified* separator bytes — a mutant changing the encoding
  consistently on both sides still round-trips. Only an exact-byte assertion on
  `mergeSnippet`'s output catches that; keep both (S07)
- pflag's `UnquoteUsage` recognizes only one backtick-quoted span per usage string (S06)

## Open debts
- `roles.*` / `optional-conventions` validation — S08 must close it, or the spec must say
  explicitly these stay unvalidated
