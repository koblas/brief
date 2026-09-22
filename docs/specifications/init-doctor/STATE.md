# init-doctor — current state

Scenarios complete: SCENARIO-01..04. Last updated by SCENARIO-04.

## Binding decisions
- `config.Locate` is the one walk-up; `config.Inspect` is the one all-violations decoder;
  `Resolve` = `Locate` + first element of `Inspect`. No second rule list or walk-up anywhere:
  `doctor`, `init` and `uninstall` all build on this (SCENARIO-01..03)
- A value error is `*config.InvalidConfigError{Path, Err: *config.ValueError{Key, Value,
  Reason, Err}}`, recovered via `errors.AsType[*config.ValueError]` (SCENARIO-01, SCENARIO-03)
- `internal/doctor` and `internal/setup` each import only `internal/platform/*` + stdlib —
  never each other, `scaffold`, or one another. Neither has a `Store` port. Both call
  `config.Locate` directly, never `resolveRoot` — `doctor` turns an invalid config into rows,
  `init` classifies it itself via `Inspect` so `--force` can rewrite it, `uninstall` never
  decodes at all (below) (SCENARIO-02, SCENARIO-03)
- Rendered artifacts and their digest registry live in `internal/platform/artifact`
  (`ConfigFile()`, `Recognize(Kind, body) Origin`). Every render is deterministic; a changed
  render appends a new digest, old ones stay (S10's "older release" origin) (SCENARIO-03)
- `init` plan-then-apply: every refusal decided before the first write; apply order is
  feature root then `.brief.yaml`; a failure after the first write wraps `setup.ErrPartialWrite`.
  `--force` only ever rewrites `.brief.yaml` from `config.Default()`, never reads old content to
  decide refusal (SCENARIO-03)
- `setup.(*Server).Uninstall(ctx, wd, UninstallRequest{Host, DryRun, Force}) (Result, error)`;
  `Result` gained `Removed []string` (never nil; `Init` always empty). Artifacts is a list,
  config last — S06/S07 prepend host artifacts ahead of it so a partial uninstall never removes
  the repo's opt-in marker while something else still stands; S07's CLAUDE.md removal goes in
  `Modified` (SCENARIO-04)
- Uninstall recognition is digest-only (`artifact.Recognize`) — never decodes the config, no
  refusal class of its own: unparseable/R1-invalid bytes are "edited locally", kept unless
  `--force`. A non-regular `.brief.yaml` (`os.Lstat`: dir, symlink) is always "kept (not a
  regular file)", even under `--force`; brief never calls `RemoveAll`. Zero artifacts ⇔
  "nothing installed"; the feature root is never an Uninstall artifact (SCENARIO-04)
- Removal order in `applyUninstall`: list order, config last; a failure after ≥1 artifact
  already removed wraps `ErrPartialWrite`, a single-artifact failure does not (SCENARIO-04)
- Root rule for both `init` and `uninstall` = `config.Locate`'s nearest: running in a
  subdirectory operates on the ancestor's config (SCENARIO-03, SCENARIO-04)
- `classifyRefusal` checks `*setup.RefusalError` before `*config.InvalidConfigError` (init's
  own refusal wraps one as `Err`) (SCENARIO-03)
- CLI: `internal/cli/artifact_render.go` holds the one row/JSON renderer both `init` and
  `uninstall` use (`artifactRow`, `artifactsJSON`) — never forked. `initDocument` never gains
  `removed`; `uninstallDocument` carries `host, dry_run, created, modified, removed, artifacts`
  (`created`/`modified` always empty) (SCENARIO-04)
- Command order: `new, start, finish, status, check, init, doctor, uninstall` — registration
  order in `newRootCommand`, mirrored in every "expected one of:" list and root help
  (SCENARIO-03, SCENARIO-04)
- No config anywhere is WARN (fix `brief init`), not ERROR — un-inited repos exit 0
  (SCENARIO-02)

## Left unbuilt
- Line number for a value error, heading-shape rules, absolute-path/`..` check on
  `feature-directory` — unowned (SCENARIO-01)
- Validation of `roles.*` and `optional-conventions` — S08 (SCENARIO-01)
- `host-plugin`, `host-hook`, `host-snippet`, `host-agents`, `roles` doctor rows, env-path's
  ERROR arm, and `artifact.Recognize`'s `older` origin — S10; uninstall must remove on
  `OriginOlder` too once it exists (SCENARIO-02, SCENARIO-03, SCENARIO-04)
- Host `claude-code`, kinds `plugin`/`hook`/`agent`, `--no-hook`, ` for <host>` suffix on
  uninstall's "nothing installed", empty-plugin-dir cleanup (must not sweep the feature root,
  see Traps) — S06; `snippet`, `merged`, CLAUDE.md block removal into `Modified` — S07;
  `--with-agents`, `roles` lines in the config — S08
- Uninstall with omitted `--host` removing every host's integration — S06/S09 decide
- `--print`, host detection, `--dry-run`/`--print` exclusivity, R10 writability pre-check — S09

## Traps
- A `t.TempDir` has no `.git` above it — a "healthy" doctor fixture must create one;
  `chmod 0o000`/`0o555` tests pass vacuously as root — skip under `os.Geteuid() == 0`
  (SCENARIO-02, SCENARIO-04)
- `doctor.Report.Counts()`'s switch has no `default` arm — S10 adding a fifth `Severity`
  without a matching `case` makes the count invariant uncaught (SCENARIO-02)
- `--force init` over a non-default `feature-directory` leaves two feature roots on disk;
  neither `init` nor `uninstall` (even `--force`) ever removes either (SCENARIO-03, SCENARIO-04)
- `filesChangedFor` only returns non-nil for commands carrying `writesFilesAnnotation` —
  forgetting it on a new write command makes `files_changed` null instead of `false`/`true`
  (SCENARIO-03, SCENARIO-04)
- R6's "directories left empty are removed" never reaches the feature root — its last
  sentence carves it out; S06's own directory cleanup must not sweep it (SCENARIO-04)
- A not-a-regular-file fixture for uninstall must use an **empty** directory: `os.Remove`
  fails on a non-empty one regardless, so a dropped `Lstat` guard would still error and prove
  nothing (SCENARIO-04)
- `hostFlagUsage`/`forceFlagUsage` (init.go) are init-worded — uninstall owns its own
  `uninstallHostFlagUsage`/`uninstallForceFlagUsage`; don't reuse init's copy on a future
  host-aware command without checking the wording fits (SCENARIO-04)

## Open debts
- `roles.*` / `optional-conventions` validation — S08 must close it, or the spec must say
  explicitly these stay unvalidated
