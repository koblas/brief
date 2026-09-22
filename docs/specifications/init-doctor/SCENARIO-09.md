---
id: SCENARIO-09
status: done
---

# SCENARIO-09: --print, host detection, and unwritable targets

## Scenario

```gherkin
Scenario: SCENARIO-09 --print, host detection, and unwritable targets
  When I run "brief init --print"
  Then stdout carries each artifact as "# <path> (create|merge)" followed by its body; nothing is written
  And the host defaults to claude-code when .claude/ or CLAUDE.md exists; otherwise init installs config and root and says no host was detected
  And an unknown host is a usage error naming claude-code and none; --dry-run with --print is a usage error
  And an unwritable target is detected before anything is written: a refusal on stderr, the --print output on stdout, exit 1, no files changed
```

Rules: R8, R9, R10, R11. Inherited context: `STATE.md` only (no prior SCENARIO file opened).

## User-visible contract

- `brief init` (no `--host`): host resolves to `claude-code` when the install root (Locate's dir,
  else wd) has a `.claude` directory or a `CLAUDE.md` entry, or `<home>/.claude` is a directory;
  else `none`. Detected-none, text mode: stdout = the usual rows (config, feature root); stderr =
  exactly one line `brief init: no agent host detected; run 'brief init --host claude-code' to
  install integration` (replaces the next-action line); exit 0. Under `--json` stderr stays empty
  and the document reports `"host":"none"`.
- stderr line priority for init success: `--dry-run` line > `--print` line > no-host-detected
  line > existing R11 next actions.
- `brief init --host bogus` → stderr `brief init: unknown host "bogus"; expected one of:
  claude-code, none; run 'brief init --print' to wire it by hand`, stdout empty, exit 2.
- `brief init --dry-run --print` (either order) → stderr `brief init: --dry-run and --print cannot
  be combined; run 'brief init --print'`, stdout empty, exit 2, checked in cli before `setup` runs.
  `--print` with `--force` / `--with-agents` / `--no-hook` / `--host` is allowed.
- `brief init --print` → nothing written. stdout: for each pending artifact in `Result.Artifacts`
  order, `# <path relative to wd> (create|merge)`, newline, body verbatim (a trailing newline
  added when the body lacks one — `SnippetBlock` has none), one empty line between artifacts, none
  after the last. stderr: `brief init: printed only, no files changed; apply the output above by
  hand, or rerun without --print`. Nothing pending → stdout empty, stderr `brief init: already
  installed; nothing changed`. Exit 0.
- `brief init --print --json` → one document: common header + `artifacts:[{path, action, body}]`,
  path absolute, action `create|merge`, body the same string as text mode; never nil (`[]`).
- Unwritable target, text mode → stderr `brief init: <rel path>: <problem>; apply the output below
  by hand (no files changed)`, stdout = exactly what `--print` would print, exit 1, tree unchanged.
  `<problem>` ∈ `not writable` | `not a directory`; `<path>` = the blocking existing ancestor
  directory (or non-directory entry), not the leaf target.
- Unwritable target, `--json` → the standard error document only (kind refusal,
  `files_changed:false`), fix `run 'brief init --print --json' and apply the artifacts by hand`;
  no artifacts in it (see Handoff).

## Implementation Plan

Setup (feature package) — Server-method tests against a `t.TempDir()` root and an injected home.

- [x] Step 1: `internal/setup/detect_test.go` `Test_init_detects_claude_code_from_the_install_root_or_home` — table over `.claude/` dir in root / `CLAUDE.md` in root / `.claude` dir in injected home / nothing (→ `HostNone`, `Result.NoHostDetected` true) / explicit `HostNone` with `.claude/` present (→ none, `NoHostDetected` false) / home func returning an error (→ clause false); detection keyed on the Locate root when wd is a subdirectory (red)
- [x] Step 2: `internal/setup/setup.go` — `WithHomeDir(func() (string, error)) Option` defaulting to `os.UserHomeDir`; `Server` gains the field (new)
- [x] Step 3: `internal/setup/detect.go` `detectHost(root, home)` + `Init` treats `req.Host == ""` as "detect" after `config.Locate`, before the `ErrAgentsNeedHost` check; `validHost` accepts `""`; `Result.NoHostDetected bool` (green)
- [x] Step 4: `internal/setup/detect_test.go` `Test_init_with_agents_follows_the_detected_host` — bare `WithAgents` with `.claude/` present installs agents; with nothing detected returns `ErrAgentsNeedHost` and the tree is unchanged (red→green on Step 3)
- [x] Step 5: `internal/setup/print_test.go` `Test_init_print_returns_pending_bodies_and_writes_nothing` — fresh root, claude-code: `Result.Print` is config (`ConfigFile()`), plugin files, CLAUDE.md = `SnippetBlock(dir)` as `create`; existing CLAUDE.md without block → `merge`; `WithAgents` fresh → config body `ConfigFileWithRoles()` + three agents; `--force` over a kept config → config `create` with the desired variant; second pass after a real Init → empty, non-nil; tree snapshot identical before/after every case (red)
- [x] Step 6: `internal/setup/setup.go` — `InitRequest.Print bool`, `PrintArtifact{Path, Action, Body}`, `PrintAction` + `PrintCreate`/`PrintMerge`, `Result.Print` (never nil); derived from the plan: `ActionCreated` file artifacts → create, `ActionMerged` → merge; feature root, unchanged, kept excluded; Print implies no writes (new)
- [x] Step 7: `internal/setup/print.go` `printArtifacts(...)` — pure derivation from the already-planned artifacts + bodies, bodies taken from the same values apply would write (green)
- [x] Step 8: `internal/setup/writable_test.go` `Test_init_refuses_an_unwritable_target_before_writing_anything` — (a) portable: `.claude/skills` is a regular file, `--host claude-code` explicit → `*RefusalError` wrapping `ErrUnwritable`, Path = `.claude/skills`, Problem `not a directory`, Fix `apply the output below by hand`, `Result.Print` populated alongside the error, tree byte-identical, no `.brief.yaml`; (b) `.claude/` chmod 0o500, skip under `os.Geteuid() == 0`, Problem `not writable`, Path = `.claude`; (c) control arm: same trees with DryRun/Print → no refusal (probe runs only when applying) (red)
- [x] Step 9: `internal/setup/setup.go` `planPluginFile` (and any sibling planner Lstat on a target path) — an `ENOTDIR` lstat means "does not exist yet" → `ActionCreated`, leaving the refusal to the pre-check (update)
- [x] Step 10: `internal/setup/writable.go` `checkWritable(targets)` + `probeWritable(dir)` (a deliberate copy of doctor's CreateTemp-and-remove probe) + `ErrUnwritable` sentinel in `errors.go` — for every target to be written, walk up to the nearest existing ancestor: non-directory → `not a directory`; probe fails → `not writable`; dedupe dirs; called after all planning refusals and before the first write, only when `!DryRun && !Print`; on failure `Init` returns the Result (Artifacts + Print) with the error (green)
- [x] Step 11: `internal/setup/doc.go` + `InitRequest`/`Result`/`Init` doc comments — detection rule, Print, ErrUnwritable and the "Result populated alongside ErrUnwritable" contract (update)
- [x] Step 12: regression — `plugin_test.go`'s "directory instead of a file → kept" and `internal/cli` `Test_init_keeps_a_plugin_path_that_is_a_directory_instead_of_a_file` still green unchanged (a directory *at* a target path is `kept`; R10 is about the parent) (green)

CLI slice.

- [x] Step 13: `internal/cli/cli.go` `run`/`newRootCommand` — replace the trailing `extraDoctorOpts ...doctor.Option` with `...runSeam` (`withDoctorOpts`, `withSetupOpts`), thread setup opts into `runInit`; update `doctor_internal_test.go`'s one call site (update)
- [x] Step 14: `internal/cli/init_internal_test.go` `Test_init_without_host_detects_the_host_from_the_tree` — via `run` + `withSetupOpts(setup.WithHomeDir(<empty tempdir>))`: nothing present → rows + the one R8 stderr line, exit 0; `CLAUDE.md` present → plugin rows + claude-code next action; `--json` detected-none → stderr empty, `"host":"none"`; bare `--with-agents` with nothing detected → existing ErrAgentsNeedHost usage error, exit 2, tree empty; with `.claude/` → installs agents (red)
- [x] Step 15: `internal/cli/init_test.go` `Test_init_with_agents_and_host_none_is_a_usage_error` — drop the `bare --with-agents` case (moved to Step 14; through `cli.Run` it would read the real home) and its "until S09" comment (update)
- [x] Step 16: `internal/cli/init.go` `runInit` — stop defaulting `""` to `HostNone`; render `NoHostDetected` per the priority above (green)
- [x] Step 17: `internal/cli/init_test.go` — unknown-host test at line ~160 asserts the R8 copy with the `--print` hint; new `Test_init_dry_run_with_print_is_a_usage_error` (both orders, `--json` gives a usage error document, tree empty) (red)
- [x] Step 18: `internal/cli/cli.go` — register `--print` (`printFlagUsage` const, single line, no `--<word>` continuation); `hostFlagUsage` gains that the default is detected (keep one backtick span — pflag `UnquoteUsage` trap); Use line `init [--host <name>] [--no-hook] [--with-agents] [--dry-run | --print] [--force]` — measure against the 80-column help test; if it overflows, omit `[--force]` from the Use line only (flag stays registered and documented, `[--json]` precedent) and record it; `internal/cli/init.go` usage-error copy for the two changes (green)
- [x] Step 19: `internal/cli/init_test.go` `Test_init_print_writes_bodies_to_stdout_and_nothing_to_disk` — fresh `--host claude-code --print` exact stdout (headers relative, bodies from `artifact.*` render funcs, snippet newline added, blank-line separation, no trailing blank), exact stderr line, exit 0, tree empty; merge case with an existing CLAUDE.md; installed tree → empty stdout + "already installed" line (red)
- [x] Step 20: `internal/cli/init_json_test.go` `Test_init_print_json_is_one_exact_document` — exact document `{schema, command, ok, exit_code, artifacts:[{path, action, body}]}`, abs paths, stderr empty (red)
- [x] Step 21: `internal/cli/init.go` — `initPrintDocument`, `printArtifactJSON`, `renderPrint(w, wd, []setup.PrintArtifact)` text renderer, `--print` branch in `runInit`, `initLong` gains `--print` prose (naming the `{path, action, body}` element shape in prose) and the R8 detection sentence; the trailing `jsonFieldsParagraph(...)` keeps listing the success document's keys only — first read `help_test.go`'s `jsonFieldsCase` agreement table (~lines 672-850) and keep its init row green; extend it with a `--print --json` row only if the table's shape allows a second document per command (green)
- [x] Step 22: `internal/cli/init_test.go` `Test_init_refuses_an_unwritable_target_and_prints_the_manual_output` — portable `.claude/skills`-is-a-file case (explicit `--host claude-code`): exact stderr refusal line, stdout byte-equal to a `--print` run on the same tree (control: `--print` run first, captured), exit 1, tree byte-identical; chmod case with root skip; `--json`: one error document, kind refusal, `files_changed:false`, the JSON fix copy, no artifacts field (red)
- [x] Step 23: `internal/cli/init.go` — on `errors.Is(err, setup.ErrUnwritable)`: text → refusal on stderr then `renderPrint(res.Print)` on stdout; JSON → refusal document with the overridden fix (green)
- [x] Step 24: `internal/cli/doc.go` + `runInit` doc comment + help/help-JSON goldens (`help_test.go`, `help_json_test.go`) for the new flag row and Use line (update)

Verification.

- [x] Step 25: mutations — copy the file to `$TMPDIR`, mutate, run the named test, observe RED, copy back, `diff` byte-identical; **never `git stash`** (the stash stack is shared across worktrees). One at a time: (a) remove the cli `--dry-run`+`--print` check → Step 17's test red; (b) make `checkWritable` return nil → Steps 8 and 22 red; (c) make `detectHost` always return `HostNone` → Steps 1, 4, 14 red; (d) make `detectHost` ignore the home clause → Step 1 home row red; (e) drop the `!DryRun && !Print` gate so the probe runs under Print → Step 8(c) control red on the portable `.claude/skills`-as-file tree (not the chmod tree — it is skipped as root, which would make this mutation vacuous); (f) emit feature root in `Result.Print` → Steps 5 and 19 red. Record each mutation and the test it reddened in the Handoff of this file.
- [x] Step 26: `docs/specifications/init-doctor/specification.md` — amend R9/R10/R11 with an "**Amended during SCENARIO-09 planning**" note carrying: the `--print` success stderr line and the nothing-pending line; the stderr priority order; the `--print` artifact set (pending body-bearing files, feature root excluded, `--force` rewrite prints as create); R10 under `--json` = error document only with the `--print --json` fix; R10 probe not run under `--dry-run`/`--print`; R8's detected-none line replacing the next action (update)
- [x] Step 27: `go build ./...`, `go test ./...` (unpiped, report count + delta), `go test -race ./internal/setup/... ./internal/cli/...`, `golangci-lint run ./...`, skip count for `internal/setup` and `internal/cli` → mark SCENARIO-09 done in specification.md; developer rewrites STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Host detection lives in `setup.Init` (root is only known after `config.Locate`); `InitRequest.Host == ""` means detect; `Result.NoHostDetected` is the only way cli distinguishes detected-none from explicit `--host none` — the R8 stderr line depends on it.
- Detection: root `.claude` directory, root `CLAUDE.md` entry (any type), or `<home>/.claude` directory. Home comes from `setup.WithHomeDir` (default `os.UserHomeDir`); a home error makes that clause false, never a refusal.
- cli seam: `run(..., readBuildInfo, ...runSeam)` with `withDoctorOpts`/`withSetupOpts`. Any cli test of a bare `init` (no `--host`) must go through `run` with an injected home in a `package cli` internal test — through `cli.Run` it reads the developer's real `~/.claude`.
- `uninstall` is unchanged: it already defaults to `claude-code` (a superset of none's plan), so `init` (detected) then `uninstall` still removes everything.
- `--print` set = pending body-bearing files only: `ActionCreated` → create, `ActionMerged` → merge; feature root, unchanged, kept excluded; a `--force` config rewrite prints as `create` with the variant a real run would write (`ConfigFileWithRoles()` under `WithAgents`). Snippet body = `SnippetBlock(dir)`, never the merged file.
- `--print --json` paths are absolute (common-envelope rule); text headers are wd-relative.
- R10 probe = CreateTemp-and-remove in the nearest existing ancestor of each target, run only when applying (`!DryRun && !Print`), after every planning refusal. `probeWritable` is a deliberate copy of `internal/doctor/probe.go` — `setup` may not import `doctor`; do not "dedupe" by importing sideways (moving it to `internal/platform` is fine).
- On `ErrUnwritable`, `Init` returns a populated `Result` (Artifacts + Print) together with the error — the only Init error path that does.
- `init --print` on a tree with a *planning* refusal (invalid `.brief.yaml`, CLAUDE.md marker defect, feature root not a directory) refuses as today and prints nothing — `ErrUnwritable` is the only error carrying printable output, even though the unknown-host copy advertises `--print` as the by-hand route.
- JSON key `artifacts` has two element shapes by document: success/dry-run `{kind, path, action, detail}`, `--print` `{path, action, body}` — the plain reading of R9; the print document carries no `host`/`dry_run`/`created`/`modified`/`roles_to_add`.
- R10 under `--json`: error document only, no artifacts — R1's one-common-envelope rule outranks R10's "print output on stdout"; the JSON fix points at `brief init --print --json`.

**Left unbuilt** — named so nobody assumes it exists:
- Feature root in `--print` output — never printed; the printed config names `feature-directory`, creating it is on the user.
- Dry-run writability prediction — `--dry-run` cannot report R10 (the probe writes); a dry run can succeed where the real run refuses.
- Detection for `uninstall` / `doctor` — none; S10's host rows must not assume `Result.NoHostDetected` exists outside Init.
- `host` field in the `--print --json` document — not emitted.
- Doctor rows `host-*`, `roles`, `artifact.OriginOlder` — S10.

**Traps** — things that look right and are not:
- `~/.claude` exists for nearly every Claude Code user, so detection effectively always resolves `claude-code`; the no-host-detected branch is almost unreachable in practice and is testable only with an injected empty home. Flag for the final `product-vision` pass: does the home clause earn its place?
- A regular file at `.claude` in a test tree trips detection as well as R10 — the unwritable tests use `.claude/skills` as the file and pass `--host claude-code` explicitly.
- `os.Lstat` under a file-as-directory returns `ENOTDIR`, which `os.IsNotExist` does not match — without Step 9 planning fails with a generic `setup: lstat` error before R10 runs.
- A directory sitting *at* a target path is the existing `kept (not a regular file)` row, exit 0 — not R10.
- chmod tests pass vacuously as root — skip under `os.Geteuid() == 0`; the portable case is the one that must always run.
- "Nothing written" for `--print` needs a before/after tree snapshot that a real Init would have changed (assert the control run differs), or it passes on a no-op.
- R10's pre-write check preempts `plugin_test.go`'s prior `Test_a_plugin_write_failure_after_the_feature_root_is_a_partial_write_and_leaves_no_config`: a chmod'd, already-existing plugin directory used to surface as `ErrPartialWrite` after the feature root landed; it now refuses `ErrUnwritable` before anything is written at all. Renamed to `Test_an_unwritable_plugin_directory_refuses_before_the_feature_root_is_created` and reasserted against the new contract — this is R10 doing exactly what it was built for, not a regression.

**Mutation verification** (copy-to-`$TMPDIR`-and-restore, never `git stash`; every mutated file diffed byte-identical against its own backup after restore):
- (a) removed `internal/cli/init.go`'s `dryRun && printOnly` usage-error check → reddened both subtests of `Test_init_dry_run_with_print_is_a_usage_error`.
- (b) made `internal/setup/writable.go`'s `checkWritable` an unconditional `return nil` → reddened `plugin_test.go`'s `Test_an_unwritable_plugin_directory_refuses_before_the_feature_root_is_created`, `writable_test.go`'s `Test_init_refuses_an_unwritable_target_before_writing_anything` (both subtests), and `internal/cli`'s `Test_init_refuses_an_unwritable_target_and_prints_the_manual_output` (both subtests).
- (c) made `detectHost` an unconditional `return HostNone, false` → reddened the "root .claude directory", "root CLAUDE.md file" and "home .claude directory" subtests of `Test_init_detects_claude_code_from_the_install_root_or_home`; the "nothing present"/"explicit host none"/"home error" subtests stayed green, as they should (same expectation either way) — confirming the table's own subtests discriminate independently.
- (d) dropped `detectHost`'s home-directory clause → reddened only the "home .claude directory, root has neither" subtest; every other subtest (which never depends on home) stayed green.
- (e) restructured `setup.Init` so the writability probe runs before the `Print` early-return (`DryRun` still short-circuits first) → reddened only the "print" subtest of `Test_the_writability_probe_never_runs_under_dry_run_or_print`; "dry run" stayed green, confirming the two gates are independent.
- (f) removed `printArtifacts`'s `KindFeatureRoot` exclusion → reddened `internal/setup`'s `Test_init_print_returns_pending_bodies_and_writes_nothing` and `Test_init_print_force_over_a_kept_config_reports_create_with_the_desired_variant`, and `internal/cli`'s `Test_init_print_writes_bodies_to_stdout_and_nothing_to_disk` (its "fresh install" subtest was strengthened from `Contains`/`HasPrefix` assertions to one exact-stdout `assert.Equal` specifically so this mutation reddened it at the cli boundary too, not only inside `setup`).
