# init-doctor — current state

Scenarios complete: SCENARIO-01..05. Last updated by SCENARIO-05.

## Binding decisions
- `config.Locate` is the one walk-up; `config.Inspect` is the one all-violations decoder;
  `Resolve` = `Locate` + first element of `Inspect`. `doctor`, `init`, `uninstall` and
  `check --hook` all build on this — no second rule list or walk-up anywhere (S01-03, 05)
- A value error is `*config.InvalidConfigError` wrapping `*config.ValueError`, recovered via
  `errors.AsType[*config.ValueError]` (SCENARIO-01, SCENARIO-03)
- `internal/doctor` and `internal/setup` import only `internal/platform/*` + stdlib — never
  each other, `scaffold`, or one another; neither has a `Store` port; both call
  `config.Locate` directly, never `resolveRoot` (SCENARIO-02, SCENARIO-03)
- Rendered artifacts and their digest registry live in `internal/platform/artifact`
  (`ConfigFile()`, `Recognize(Kind, body) Origin`); deterministic renders, a changed one
  appends a new digest, old ones stay (S10's "older release" origin) (SCENARIO-03)
- `init` plan-then-apply: every refusal decided before the first write; apply order is
  feature root then `.brief.yaml`; a failure after the first write wraps
  `setup.ErrPartialWrite`; `--force` only rewrites `.brief.yaml` from `config.Default()`
- `setup.(*Server).Uninstall` recognizes artifacts by digest only (`artifact.Recognize`),
  never by decoding; unparseable/invalid config bytes are "edited locally", kept unless
  `--force`; a non-regular `.brief.yaml` is always "kept (not a regular file)" (SCENARIO-04)
- Root rule for `init`, `uninstall`, and `check --hook`'s opt-in gate = `config.Locate`'s
  nearest: a subdirectory operates on the ancestor's config; no config anywhere is WARN for
  `doctor`, fully silent exit 0 for `check --hook` (never a WARN row) (S02, 03, 04, 05)
- `classifyRefusal` checks `*setup.RefusalError`, then `*config.InvalidConfigError`, then
  `*unknownFeatureError`, then `*scaffold.RefusalError`, then `*assemble.RefusalError`, in
  that order (SCENARIO-03)
- CLI: `internal/cli/artifact_render.go` holds the one row/JSON renderer `init`/`uninstall`
  use. Command order: `new, start, finish, status, check, init, doctor, uninstall` —
  registration order in `newRootCommand`, mirrored in every "expected one of:" list (S03, 04)
- Host adapters live in `internal/platform/host`: `Host{Name, HookPath, WriteHookContext}`,
  `ErrMalformedPayload`, `HookHosts()`, `Lookup(name)`. `internal/cli` (hook parsing/output)
  and `internal/setup` (artifact rendering, S06) both consume it; a feature package never
  imports another. `host.HookHosts()` is separate from `setup.Hosts()` — `claude-code` is
  NOT yet in `setup.Hosts()`; S06 must reconcile the two (SCENARIO-05)
- Path→feature mapping is `assemble.(*Server).FeatureContaining(path) (string, bool)`:
  lexical `filepath.Rel` against `root/FeatureDirectory` first, `EvalSymlinks` retry on both
  sides only when lexical says outside, first component must be a real directory with
  something beneath it; `cli` never computes this itself (SCENARIO-05)
- `check --hook <host>` (R12 amended, commit 1b12f18): ERROR findings on the edited feature
  → exit 0, one `hookSpecificOutput.additionalContext` document on stdout via
  `host.WriteHookContext`, stderr empty; no ERROR is fully silent; stdout never carries
  `check`'s own findings table. `--hook` excludes a positional feature and `--json`; a
  malformed payload is a usage error, exit 2. Config is located from the injected `wd`,
  never the payload's `cwd` (SCENARIO-05)
- `checkInvocation` stays the generic flag-error hint; `checkHookInvocation` ("brief check
  --hook claude-code") is `--hook`'s own concrete hint, same split as `init`/`uninstall`

## Left unbuilt
- Line number for a value error, heading-shape rules, absolute-path/`..` check on
  `feature-directory` — unowned (SCENARIO-01)
- Validation of `roles.*` and `optional-conventions` — S08 (SCENARIO-01)
- `host-plugin`, `host-hook`, `host-snippet`, `host-agents`, `roles` doctor rows, env-path's
  ERROR arm, and `artifact.Recognize`'s `older` origin — S10; uninstall must remove on
  `OriginOlder` too once it exists (SCENARIO-02, SCENARIO-03, SCENARIO-04)
- Host `claude-code`, kinds `plugin`/`hook`/`agent`, `--no-hook`, `hooks/hooks.json`
  rendering, adding `claude-code` to `setup.Hosts()`, ` for <host>` suffix on uninstall's
  "nothing installed", empty-plugin-dir cleanup (must not sweep the feature root) — S06;
  `snippet`, `merged`, CLAUDE.md block removal into `Modified` — S07; `--with-agents`,
  `roles` lines in the config — S08
- Uninstall with omitted `--host` removing every host's integration — S06/S09 decide
- `--print`, host detection, `--dry-run`/`--print` exclusivity, R10 writability pre-check — S09
- Hook payload's `cwd`, `tool_name`, `hook_event_name` — unowned; add only if a later
  scenario needs them

## Traps
- A `t.TempDir` has no `.git` above it — a "healthy" doctor fixture must create one;
  `chmod 0o000`/`0o555` tests pass vacuously as root — skip under `os.Geteuid() == 0` (S02, 04)
- `doctor.Report.Counts()`'s switch has no `default` arm — S10 adding a fifth `Severity`
  without a matching `case` makes the count invariant uncaught (SCENARIO-02)
- `--force init` over a non-default `feature-directory` leaves two feature roots on disk;
  neither `init` nor `uninstall` ever removes either (SCENARIO-03, SCENARIO-04)
- `filesChangedFor` only returns non-nil for commands carrying `writesFilesAnnotation` —
  forgetting it makes `files_changed` null instead of `false`/`true` (SCENARIO-03, 04)
- R6's "directories left empty are removed" never reaches the feature root; a not-a-regular-
  file fixture for uninstall must use an **empty** directory (SCENARIO-04)
- `hostFlagUsage`/`forceFlagUsage` (init.go) are init-worded — uninstall owns its own
  `uninstallHostFlagUsage`/`uninstallForceFlagUsage` (SCENARIO-04)
- `t.TempDir()` on macOS resolves through a symlink (`/var` → `/private/var`); matching
  lexically first, retrying with `EvalSymlinks` only when lexical says outside, is load
  bearing — resolving symlinks first would launder a symlinked feature directory, which must
  instead be rejected (SCENARIO-05)
- `check --hook`'s exit 0 is both the "silent" and the "findings" outcome — a silent-0 test
  asserting only the exit code proves nothing; it needs a control arm whose stdout is the
  non-empty `hookSpecificOutput` document, differing by exactly one variable (SCENARIO-05)

## Open debts
- `roles.*` / `optional-conventions` validation — S08 must close it, or the spec must say
  explicitly these stay unvalidated
