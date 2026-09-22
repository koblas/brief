---
id: SCENARIO-04
status: done
---
# SCENARIO-04: uninstall removes the config init wrote and nothing else

## Scenario

```gherkin
Scenario: SCENARIO-04 uninstall removes the config init wrote and nothing else
  Given init has run
  When I run "brief uninstall"
  Then an unedited .brief.yaml is removed; an edited one is kept and reported, and --force removes it
  And the feature root and everything under it are never removed
  And with nothing installed, stdout is empty, stderr says nothing is installed, exit 0
```

Rules: R6 (ownership, digest-only recognition, feature root never removed), R11 (verbs, stderr
next action, JSON envelope + `removed`, `writesFilesAnnotation`), R14 (command order, short,
synopsis `brief uninstall [--host <name>] [--dry-run] [--force] [--json]`).

## User-visible contract (pinned)

Command: `brief uninstall [--host <name>] [--dry-run] [--force] [--json]`. Omitted `--host`
means `none` (accepted hosts: `setup.Hosts()`). Short: `remove what init installed`.

Which config: `config.Locate(wd)`'s nearest `.brief.yaml` — the same root rule as `init`.
None found anywhere → nothing installed.

Recognition is digest-only (`artifact.Recognize(KindConfig, bytes)`); uninstall never decodes
the config (no `Inspect`, no `Resolve`, no `feature-directory` lookup) and therefore has **no
config-refusal class** — a deliberate divergence from `Init`.

- bytes equal the current render (`OriginCurrent`) → `removed .brief.yaml`, with or without `--force`
- any other bytes, including invalid or unparseable → `kept .brief.yaml (edited locally)`;
  with `--force` → `removed .brief.yaml (edited locally)`
- not a regular file (`Lstat`: directory, symlink) → `kept .brief.yaml (not a regular file)`,
  with or without `--force`
- absent → no row

Stdout: one row per brief-owned artifact, via the shared row renderer —
`<action> <relative path>[ (<detail>)]`. Uninstall emits **no feature-root row, ever**.

Stderr (text mode only, prefix `brief uninstall: `), exactly one line, first match wins:
1. dry run: `dry run, no files changed; rerun without --dry-run to apply`
2. zero artifacts: `nothing installed` (stdout empty). S06 adds a ` for <host>` arm for
   non-`none` hosts
3. at least one `removed`: `removed brief's install; the feature root and its contents were left in place`
4. otherwise (all kept): `nothing removed; run 'brief uninstall --force' to remove edited files`

Exit codes: 0 success (incl. nothing installed and all-kept); 2 usage error (unknown host,
stray positional: `brief uninstall: unknown host "<h>"; expected one of: none; run 'brief
uninstall --host none'` / `brief uninstall: too many arguments; run 'brief uninstall --host
none'`); 1 refusal/runtime failure (`Locate` on a missing wd, `os.Remove` failure).

JSON (`--json`): `{<header>, host, dry_run, created:[], modified:[], removed:[abs], artifacts:
[{kind, path(abs), action, detail|null}]}`. `created`/`modified`/`removed` never `null`;
`removed` empty under `--dry-run`; nothing installed → `artifacts: []`, no stderr line.
`files_changed` present (not null) on error documents via `writesFilesAnnotation`.

`--dry-run` computes the identical plan (rows say `removed`) and removes nothing.

## Implementation Plan

Setup (feature package) — `internal/setup`:

- [x] Step 1: `internal/setup/uninstall_test.go` `Test_uninstall_removes_an_unedited_config_and_keeps_the_feature_root` — `Init` then `Uninstall`; config gone, `Removed` = [abs config path], artifact `{config, removed}`; feature root and a file placed under it still present (red)
- [x] Step 2: `internal/setup/setup.go` — `ActionRemoved`, `UninstallRequest{Host, DryRun, Force}`, `Result.Removed` (never nil; `Init` sets it empty) (new)
- [x] Step 3: `internal/setup/uninstall.go` `(*Server).Uninstall(ctx, wd, UninstallRequest) (Result, error)` — validHost → `config.Locate` → plan config artifact (Lstat + `Recognize` only) → apply unless DryRun; artifacts assembled as a list so S06/S07 host planners append before the config, config removed last (green)
- [x] Step 4: `uninstall_test.go` `Test_uninstall_keeps_an_edited_config_and_reports_it` — byte-identical afterwards, detail `edited locally`, `Removed` empty (red→green)
- [x] Step 5: `uninstall_test.go` `Test_uninstall_force_removes_an_edited_config` (red→green)
- [x] Step 6: `uninstall_test.go` table `Test_uninstall_treats_an_unparseable_or_invalid_config_as_edited` — unparseable YAML and an R1-invalid value: kept without `--force`, removed with it; no error in any row (red→green)
- [x] Step 7: `uninstall_test.go` `Test_uninstall_with_no_config_reports_nothing_installed` — zero artifacts, all three slices empty non-nil, no error; control: an existing feature root with no config is still zero artifacts and untouched (red→green)
- [x] Step 8: `uninstall_test.go` `Test_uninstall_dry_run_plans_removal_and_removes_nothing` — row `removed`, `Removed` empty, file present byte-identical (red→green)
- [x] Step 9: `uninstall_test.go` `Test_uninstall_keeps_a_config_path_that_is_not_a_regular_file` — `.brief.yaml` as an **empty** directory (so a mutation dropping the Lstat guard would actually remove it), with and without `--force`: kept, detail `not a regular file`, directory still present (red→green)
- [x] Step 10: `uninstall_test.go` `Test_uninstall_operates_on_a_config_found_in_an_ancestor` — wd is a subdirectory; the ancestor's config is the one removed (red→green)
- [x] Step 11: `uninstall_test.go` `Test_uninstall_never_removes_either_feature_root_after_force_init` — non-default `feature-directory`, `init --force` (two roots), `uninstall --force`: both roots survive, including the empty default one (red→green)
- [x] Step 12: `uninstall_test.go` `Test_uninstall_reports_a_remove_failure_without_partial_write` — parent dir `0o555`, skip under `os.Geteuid()==0`; error returned, `!errors.Is(err, ErrPartialWrite)`, file still present (red→green)
- [x] Step 13: `uninstall_test.go` `Test_uninstall_rejects_an_unknown_host` — `errors.Is(err, ErrUnknownHost)` (red→green)
- [x] Step 14: `internal/setup/roundtrip_test.go` `Test_init_then_uninstall_leaves_the_tree_as_before_except_the_feature_root` — pre-existing unrelated files (root README, nested dir + file, a `.claude/` file); snapshot every path + bytes + mode type; init; uninstall; snapshot equals before **plus exactly the empty feature root directory** (the asymmetry: init created it, uninstall keeps it — assert its presence, not just tolerate it) (red→green)
- [x] Step 15: `internal/setup/doc.go` + `Uninstall`/`UninstallRequest`/`ActionRemoved`/`Result` doc comments — state digest-only recognition, no refusal class, feature root never removed, config removed last (update)

CLI slice — `internal/cli`:

- [x] Step 16: `internal/cli/artifact_render.go` — rename `initRow`→`artifactRow`, `initArtifactJSON`/`initArtifactsJSON`→`artifactJSON`/`artifactsJSON`, moved out of `init.go`; existing init tests stay green unchanged (refactor, no behavior change)
- [x] Step 17: `internal/cli/uninstall_test.go` `Test_uninstall_removes_the_config_init_wrote` — through `cli.Run`: init then uninstall; stdout exactly `removed .brief.yaml\n`, stderr line 3, exit nil (red)
- [x] Step 18: `internal/cli/uninstall.go` — `uninstallInvocation`, `uninstallLong` (+ `jsonFieldsParagraph("host","dry_run","created","modified","removed","artifacts")`), `uninstallDocument`, `uninstallNextAction`, `runUninstall` (green)
- [x] Step 19: `internal/cli/cli.go` `newRootCommand` — `uninstall` leaf registered **after** `doctor`, flags `--host`/`--dry-run`/`--force`/`--json` with uninstall-specific host/force usage strings, `Annotations[writesFilesAnnotation] = "true"` (green)
- [x] Step 20: `uninstall_test.go` — one test per remaining contract row: edited kept + stderr line 4; `--force` removes; nothing installed → empty stdout, stderr `brief uninstall: nothing installed`, exit nil; `--dry-run` → stderr line 1, file present; unknown host → exit-2 usage copy; stray positional → exit-2 usage copy (red→green)
- [x] Step 21: `internal/cli/uninstall_json_test.go` — `Test_uninstall_json_is_one_exact_document` (removed:[abs], created/modified `[]`), `Test_uninstall_json_nothing_installed_is_an_empty_document` (artifacts `[]`, empty stderr), `Test_uninstall_json_dry_run_reports_empty_removed`, `Test_uninstall_json_failure_reports_files_changed_false` (remove failure, non-null `files_changed`) (red→green)
- [x] Step 22: goldens — re-grep for the command list (`check, init, doctor`, and any per-leaf enumeration in `help_json_test.go`, `completion_test.go`, `help_test.go`, `cli_internal_test.go`, `run_test.go`, `json_usage_test.go`, `flag_error_test.go`) and append `uninstall`; add the root-help row and a `brief uninstall --help` golden; `uninstallLong` passes the 80-column test (update)
- [x] Step 23: `cmd/brief/main.go` — no change expected (dispatch + exit-code mapping already cover usage=2 / refusal=1); confirm only
- [x] Step 24: mutation-verify, each individually, fresh cp to `$TMPDIR` per mutation, diff-restored after: (a) `Recognize` check replaced by always-current → `Test_uninstall_keeps_an_edited_config_and_reports_it` red; (b) `--force` branch ignored → `Test_uninstall_force_removes_an_edited_config` red; (c) Lstat regular-file guard dropped → `Test_uninstall_keeps_a_config_path_that_is_not_a_regular_file` red; (d) DryRun apply skip dropped → `Test_uninstall_dry_run_plans_removal_and_removes_nothing` red; (e) `writesFilesAnnotation` removed from the uninstall leaf → `Test_uninstall_json_failure_reports_files_changed_false` red; (f) Uninstall additionally `os.Remove`s the (empty) feature root → `Test_init_then_uninstall_leaves_the_tree_as_before_except_the_feature_root` red. All six restored byte-identical (verified with `diff`)
- [x] Step 25: `go build ./...`, `go test ./...` (count + delta), `go test -race ./internal/setup/... ./internal/cli/...`, `golangci-lint run ./...` → mark SCENARIO-04 done in specification.md, rewrite STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `setup.(*Server).Uninstall(ctx, wd, UninstallRequest{Host, DryRun, Force}) (Result, error)`;
  `Result` gains `Removed []string` (never nil; `Init` always returns it empty) — S06/S07 add
  host artifacts to the same Result, S07's CLAUDE.md block removal goes in `Modified`
- Uninstall emits no feature-root artifact and never reads `feature-directory`; zero artifacts
  ⇔ "nothing installed" (empty stdout). A row for the feature root would make that
  unsatisfiable without a special case
- Recognition is digest-only (`artifact.Recognize`); uninstall never decodes config and has no
  config-refusal class. Invalid/unparseable → `kept (edited locally)`, `--force` removes
- A non-regular `.brief.yaml` (Lstat: dir, symlink) is `kept (not a regular file)` even under
  `--force`; brief never calls `RemoveAll` on anything
- Removal order: host artifacts first (S06/S07), config last — a partial uninstall stays
  "installed" and re-runnable. A failure after one removal wraps `ErrPartialWrite`; a
  single-artifact failure does not
- Root rule = `config.Locate` nearest, same as `init`: running in a subdirectory removes the
  ancestor's config. Intentional, not a bug
- CLI document types stay separate: `initDocument` never gains `removed`
  (`Test_init_json_is_one_exact_document`); `uninstallDocument` carries it
- One row/JSON renderer for both commands (`artifactRow`, `artifactsJSON`) — S06/S07 extend it,
  never fork it
- Stderr lines (prefix `brief uninstall: `): dry run / `nothing installed` / `removed brief's
  install; the feature root and its contents were left in place` / `nothing removed; run
  'brief uninstall --force' to remove edited files`
- Command order: `new, start, finish, status, check, init, doctor, uninstall`

**Left unbuilt** — named so nobody assumes it exists:
- ` for <host>` suffix on `nothing installed`, plugin/hook/agent removal, empty-plugin-dir
  cleanup — S06; `CLAUDE.md` block removal (`modified`) — S07
- Uninstall with omitted `--host` removing every host's integration — S06/S09 decide
- `artifact.OriginOlder` (older-release config counts as unedited → removed) — S10; uninstall
  must remove on `OriginOlder` too once it exists

**Traps** — things that look right and are not:
- R6's "directories left empty are removed" never reaches the feature root — R6's last sentence
  carves it out. After init+uninstall the feature root is typically empty; S06's directory
  cleanup must not sweep it
- `--force` init over a non-default `feature-directory` leaves two roots; uninstall removes
  neither
- Mirroring `planConfig` imports `Inspect` and its refusal path by accident — uninstall must
  not refuse on config content
- The not-a-regular-file test must use an **empty** directory: `os.Remove` fails on a
  non-empty one, so a dropped Lstat guard would still pass (as an error) and prove nothing
- `hostFlagUsage`/`forceFlagUsage` are init-worded ("install for", "rewrite from defaults") —
  uninstall needs its own usage strings
- Forgetting `writesFilesAnnotation` makes `files_changed` null on uninstall's error documents
