---
id: SCENARIO-03
status: done
---
# SCENARIO-03: init writes the config and feature root, and converges

## Scenario

```gherkin
Scenario: SCENARIO-03 init writes the config and feature root, and converges
  When I run "brief init --host none" in a fresh repo
  Then .brief.yaml is created with every key present but commented, and the feature root is created
  And stdout lists "created .brief.yaml" and "created docs/specifications/", with a next action on stderr
  And a second run reports "unchanged" for both, exit 0
  And a valid existing config is "kept"; an unparseable one refuses (exit 1, no files changed); --force rewrites it from defaults
  And --dry-run prints the same report and writes nothing
```

Rules covered: R2, R3, R6 (current-output recognition only), R9 (`--dry-run` part), R11, R14.
Context read: STATE.md only; no prior SCENARIO file opened.

## User-visible contract

Command line: `brief init [--host <name>] [--dry-run] [--force] [--json]`.

- Install location: `config.Locate(wd)`; a found `.brief.yaml` → operate on that file's
  directory (the same root `resolveRoot` gives every other command); none found → `wd`.
  No git-root detection.
- `--host` omitted → `none` (no detection, no extra stderr line). Accepted hosts come from one
  ordered list, `none` only in this scenario. Any other value: usage error, exit 2,
  `brief init: unknown host "<h>"; expected one of: none; run 'brief init --host none'`.
- Stdout (text): one line per artifact, `<action> <relative path>[ (<detail>)]`, config first,
  feature root second; the feature root renders with a trailing `/`.
  Actions used here: `created`, `unchanged`, `kept`.
- Stderr next action (exactly one line, exit 0):
  - something created: `brief init: installed config and feature root; run 'brief new feature <name>'`
  - every artifact `unchanged`/`kept`: `brief init: already installed; nothing changed`
  - `--dry-run`: `brief init: dry run, no files changed; rerun without --dry-run to apply`
- Config classification (existing file, no `--force`): bytes equal the current render →
  `unchanged`; parses with zero violations but differs → `kept` (detail `edited locally`);
  unparseable, unknown key, or any `ValueError` → refusal, exit 1, no files changed, fix names
  `brief init --force`. `--force` → rewrite from defaults (`created`, detail
  `rewritten from defaults`; path listed in JSON `created` — the file is written fresh
  from brief's own render), or `unchanged`
  when bytes already equal. `--force` never refuses on the old config's content.
- Feature root: the kept config's `feature-directory` wins; under `--force` (or a fresh
  config) it is `Default().FeatureDirectory`. Existing directory → `unchanged`; missing →
  `created` (MkdirAll); exists as a non-directory → refusal, exit 1, no files changed.
- `.brief.yaml` line shapes: doc prose lines begin `# ` (hash, space); commented-out value
  lines begin `#` immediately followed by the YAML line (no space). Uncommenting = strip exactly
  one leading `#` from every line not beginning `# `. No live line at all (no `---`).
- `--dry-run` computes the identical plan (including refusals, same exit codes), prints the
  same stdout rows, writes nothing.
- `--json` (stdout only, zero stderr bytes): common header, then
  `{host, dry_run, created:[abs], modified:[abs], artifacts:[{kind, path, action, detail}]}`;
  `kind` ∈ `config` | `feature-root` here; `path` absolute; `detail` null when empty;
  `created`/`modified` never null, and both `[]` under `--dry-run`. Refusal → error document
  kind `refusal`, `files_changed` false; partial write → `files_changed` true.

Dependency rule: `internal/platform/artifact` imports `internal/platform/config` (platform →
platform), following the precedent that `config` already holds brief's schema; it carries no
command or feature logic, only bytes and digests. `internal/setup` is a new feature package
(not an extension of `scaffold`, which owns the feature-directory write path).

## Implementation Plan

- [x] Step 1: `internal/platform/artifact/artifact_test.go` `Test_config_file_resolves_to_the_shipped_defaults` — `config.Inspect` of the written render equals `config.Default()` with zero violations; plus `Test_uncommenting_the_config_file_yields_the_shipped_defaults` (strip one leading `#` from every line not beginning `# `, Inspect again, still `Default()`) and `Test_every_config_key_is_documented` (red)
- [x] Step 2: `internal/platform/artifact/doc.go` — package doc: renders the files brief writes into a repository and recognizes them by digest; imports `internal/platform/config` + stdlib + yaml only (new)
- [x] Step 3: `internal/platform/artifact/config.go` `ConfigFile() []byte` — marshal `config.Default()` (2-space indent), prepend a `# ` doc line per key, prefix every YAML line with `#` (no space); deterministic bytes, no version/date (green)
- [x] Step 4: `internal/platform/artifact/artifact_test.go` `Test_the_current_config_render_is_a_known_digest` — sha256 of `ConfigFile()` is in the compiled-in registry; plus `Recognize` table: current bytes → current, other bytes → edited (red)
- [x] Step 5: `internal/platform/artifact/digest.go` — `Kind`/`Origin` types, `configDigests` list (current render only), `Recognize(kind, body) Origin` returning current / edited — no `older` arm until a real old digest exists (S10 adds it) (green)
- [x] Step 6: `internal/setup/setup_test.go` `Test_init_creates_the_config_and_feature_root_in_a_fresh_repo` — `Server.Init` against a `t.TempDir` (red)
- [x] Step 7: `internal/setup/doc.go` — package doc: plan-then-apply install writes; imports `internal/platform/{config,artifact,atomicfile}` + stdlib only, never `scaffold`/`doctor`; no Store port (filesystem is the contract, as in scaffold) (new)
- [x] Step 8: `internal/setup/setup.go` — `Server`, `Option`, `NewServer`, `InitRequest{Host, DryRun, Force}`, `Result{Host, DryRun, Artifacts, Created, Modified}`, `Artifact{Kind, Path, Action, Detail}`, `Action`/`Kind` constants, `Hosts()` ordered list (`none`), `ErrUnknownHost`, `ErrPartialWrite` (new)
- [x] Step 9: `internal/setup/setup.go` `(*Server).Init` — Locate → classify config (Inspect + `artifact.Recognize`) → classify feature root → full plan built and every refusal decided before any write → apply unless DryRun: feature root first, `.brief.yaml` last (atomicfile); a failure after the first write wraps `ErrPartialWrite` (green)
- [x] Step 10: `internal/setup/setup_test.go` — converge matrix, one test each: second run all `unchanged`; valid differing config `kept` + root from its `feature-directory`; unparseable config refused, tree byte-identical; invalid value refused with `*RefusalError` carrying key and value recovered via `errors.AsType[*config.ValueError]`; `--force` over invalid config rewrites to `ConfigFile()`; `--force` over current bytes `unchanged`; feature root is a file → refused, no config written; `--force` with `.brief.yaml` as a directory → root created, error wraps `ErrPartialWrite`; DryRun returns the same artifacts and writes nothing; unknown host → `ErrUnknownHost`; config found in an ancestor → operates there (red → green)
- [x] Step 11: `internal/setup/errors.go` `RefusalError{Path, Problem, Fix, Err}` — config refusal problem built from the ValueError's key/value (or the decode error), fix `fix it, or run 'brief init --force' to rewrite it from defaults` (green)
- [x] Step 12: `internal/cli/init_test.go` `Test_init_reports_created_then_unchanged` — command slice through `cli.Run`: exact stdout rows + stderr next action, then the rerun (red)
- [x] Step 13: `internal/cli/init.go` — `initLong` (with `jsonFieldsParagraph("host", "dry_run", "created", "modified", "artifacts")`), `initInvocation`, `runInit`: flag values → `setup.InitRequest`, `ErrUnknownHost` → usage error, other errors → `out.refusal`, text rows via `displayPath`, stderr line selection (green)
- [x] Step 14: `internal/cli/refusal.go` `classifyRefusal` — `*setup.RefusalError` branch placed before the `*config.InvalidConfigError` branch, with the no-files-changed tail (update)
- [x] Step 15: `internal/cli/json.go` `filesChangedFor` — also true for `setup.ErrPartialWrite`; update its doc comment's command list (update)
- [x] Step 16: `internal/cli/init.go` `initDocument` / `initArtifactJSON` — R11 envelope, non-nil slices, absolute paths, detail null when empty (green)
- [x] Step 17: `internal/cli/init_test.go` / `init_json_test.go` — CLI matrix: kept; unparseable refusal (exit 1, stderr line, tree byte-identical, with a control arm on the happy path showing the same probe sees the created files); invalid value refusal names key, value and `brief init --force`; `--force`; `--dry-run` rows + dry-run stderr + nothing on disk; unknown host exit 2 (use `bogus`, never `claude-code`); stray positional → usage error; `--json` success; `--dry-run --json`; refusal `--json` with `files_changed` false (red → green)
- [x] Step 18: `internal/cli/cli.go` `newRootCommand` — register `init` leaf between `check` and `doctor`, short `install brief's config and agent-host integration`, flags `--host`, `--dry-run`, `--force`, `--json`, `writesFilesAnnotation`; update the `init()` comment's order list (update)
- [x] Step 19: goldens — every `expected one of: new, start, finish, status, check, doctor` → `…, check, init, doctor` (cli_internal, completion, exit, flag_error, help, json_usage, run tests), root-help row, help-json command list; `initLong` within 80 columns (update)
- [x] Step 20: `internal/cli/doc.go` — add `init` wherever the command set is enumerated (update)
- [x] Step 21: mutation-verify, each individually and stashed per agent-briefs: (a) skip the Inspect violation check → the invalid-value refusal tests go red; (b) drop the `ErrPartialWrite` wrap → `Test_force_over_a_directory_named_brief_yaml_reports_a_partial_write` goes red (fixture: `.brief.yaml` is a directory, `--force`, no feature root yet — root creation lands, the config rename fails; no chmod, no euid skip); (c) make `Recognize` always return edited → the rerun-`unchanged` tests go red; (d) drop the DryRun guard → the dry-run nothing-written tests go red. Name each mutation and the test it reddened

  Verified (fresh copy to $TMPDIR before each, restored + diffed byte-identical after):
  - (a) `internal/setup/setup.go` `planConfig`: `if len(violations) > 0` → `if false && len(violations) > 0` — reddened `internal/setup` `Test_an_invalid_config_value_refuses_naming_the_key_and_value` and `internal/cli` `Test_init_refuses_an_invalid_config_value_naming_the_key_value_and_force_fix`.
  - (b) `internal/setup/setup.go` `apply`: dropped the `if wroteSomething { return Result{}, markPartial(err) }` branch, returning the write error unwrapped — reddened `Test_force_with_the_config_path_as_a_directory_reports_a_partial_write` (this scenario's own name for the plan's `Test_force_over_a_directory_named_brief_yaml_reports_a_partial_write`).
  - (c) `internal/platform/artifact/digest.go` `Recognize`: changed the digest-match branch to `return OriginEdited` (matching `digestsFor` still called, so the mutation stays compiling and surgical) — reddened `internal/setup` `Test_a_second_run_reports_every_artifact_unchanged` and `internal/cli` `Test_init_reports_created_then_unchanged`.
  - (d) `internal/setup/setup.go` `Init`: `if req.DryRun` → `if false && req.DryRun` — reddened `Test_dry_run_returns_the_plan_and_writes_nothing` and `internal/cli` `Test_init_dry_run_prints_the_plan_and_writes_nothing`.
- [x] Step 22: `go build ./...`, `go test ./...` (unpiped; report count + delta), `go test -race ./internal/setup/... ./internal/platform/artifact/... ./internal/cli/...`, `golangci-lint run ./...` → mark SCENARIO-03 done in specification.md, set frontmatter `status: done`, rewrite STATE.md

  `go build ./...` clean. `go test ./...` unpiped, exit 0, all 11 packages `ok` (0 skips). New/changed package test counts (via `-v | grep -c "=== RUN"`, includes subtests): `internal/setup` 12 (0 → 12, new package), `internal/platform/artifact` 7 (0 → 7, new package), `internal/cli` 644 total after adding the `init` suite plus a handful of `init` rows into five existing sweep tables (kept, unparseable + control arm, invalid-value, `--force`, `--dry-run`, unknown host, stray positional, `--json` success, `--dry-run --json`, refusal `--json`). `go test -race` on all three: clean. `golangci-lint run ./...`: `0 issues` (fixed 3 `modernize`, 3 `testifylint`, 1 `wrapcheck` — the last by adding `config.Locate(` to `.golangci.yaml`'s `wrapcheck.extra-ignore-sigs`, alongside the existing `config.Resolve(` entry, since `Locate` wraps its own nonexistent-startDir refusal the same already-wrapped way).

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `init` operates on `filepath.Dir(config.Locate(wd).nearest)` when a config exists, else `wd` —
  the same root `resolveRoot` gives every other command, so `init`'s feature root is the one
  `new`/`check`/`doctor` use. No git-root rule
- `init` never calls `Resolve`/`resolveRoot` — `--force` must be able to rewrite an invalid
  config; it uses `Locate` + `Inspect` and decides refusals itself
- Rendered artifacts and the digest registry live in `internal/platform/artifact`, not in
  `internal/setup` — S04 (uninstall) and S10 (`doctor` older/edited rows) both consume
  `artifact.Recognize`, and `doctor` may not import a feature package. S06/S07/S08 add their
  renderers and digest lists there
- Every artifact render is deterministic (no version, date, map order) — otherwise a digest
  proves nothing. When a render changes, its new digest is appended; old digests stay (they
  become "older release" for R6)
- `ConfigFile()` resolves to exactly `Default()` (all lines commented) — S08's roles-bound
  variant is a second render and gets its own digests
- Plan-then-apply: every refusal is decided before the first write; feature root written
  first, `.brief.yaml` last (the opt-in marker appears only once everything else landed).
  A failure after the first write wraps `setup.ErrPartialWrite`, which `filesChangedFor` checks
  alongside `scaffold.ErrPartialWrite`
- Omitted `--host` means `none` in S03; accepted hosts come from `setup.Hosts()`. S06 appends
  `claude-code`; S09 replaces the omitted-host default with R8 detection and its stderr line
  (never fired when `--host none` was explicit), and adds `--print` to the usage-error fix
- `--force` means only "rewrite `.brief.yaml` from defaults"; reported `created` with detail
  `rewritten from defaults`, path in JSON `created` (JSON `created` = written fresh from a brief
  render; `modified` = edited in place, reserved for S07's CLAUDE.md merge). S04's `removed[]`
  sits beside these
- `.brief.yaml` prefixes: `# ` = doc prose, `#<yaml>` = commented value — S08's live roles lines
  must not break the uncomment rule
- Existing valid config not byte-equal to the current render → `kept`, detail `edited locally`
- Command order: `new, start, finish, status, check, init, doctor` (S04 appends `uninstall`)
- Closes STATE open debt: `init`'s invalid-config refusal recovers key/value via
  `errors.AsType[*config.ValueError]` inside `*config.InvalidConfigError`

**Left unbuilt** — named so nobody assumes it exists:
- `setup.(*Server).Uninstall`, `removed` action, JSON `removed` — S04
- Host `claude-code`, kinds `plugin`/`hook`/`agent`, `--no-hook` — S06; `snippet`, `merged` — S07;
  `--with-agents`, roles lines in the config — S08
- `--print`, host detection, `--dry-run`/`--print` exclusivity, R10 writability pre-check — S09
- Older-release digests and `artifact.Recognize`'s `older` origin — S10 (lists hold the current
  render only)
- The claude-code stderr copy (`installed for claude-code; run /reload-plugins, …`) — S06; the
  `--host none` line in this plan is S03's own wording

**Traps** — things that look right and are not:
- `--force` over a config with a non-default `feature-directory` leaves the old feature root in
  place and creates the default one; S04 must still never remove either
- `classifyRefusal`'s `*config.InvalidConfigError` branch has a fix telling the user to remove
  the file; if `*setup.RefusalError` wraps it and the branch order is wrong, init's refusal loses
  its `brief init --force` fix
- An all-comment `.brief.yaml` resolves via `Inspect`'s `io.EOF`-as-empty branch — a render that
  emits even one live line (e.g. a `---` marker) breaks the Default() round trip
- `filesChangedFor` only returns non-nil for commands with `writesFilesAnnotation` — forgetting
  the annotation on `init` makes `files_changed` null
- Root-help `init` row, like `doctor`'s, is exempt from the 80-column test; `initLong` is not
