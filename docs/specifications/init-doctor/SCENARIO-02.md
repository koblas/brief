---
id: SCENARIO-02
status: done
---
# SCENARIO-02: doctor reports config, feature-root and environment health

## Scenario

```gherkin
Scenario: SCENARIO-02 doctor reports config, feature-root and environment health
  When I run "brief doctor" in a healthy repo
  Then stdout has one row per check (config-file, config-parse, config-values, config-shadow, root-dir, env-git, env-path) with OK/WARN/ERROR/SKIP, id, relative path and detail
  And stderr says "brief doctor: setup ok; run 'brief check' for feature content", exit 0
  And a missing or non-directory feature root is ERROR with a fix, exit 1; an unparseable config makes config-values SKIP
  And --json gives {counts, checks:[{id, severity, path, detail, fix}]}
```

## User-visible contract

- Command line: `brief doctor [--json]`. Takes no positional argument: `brief doctor x` is a usage
  error, exit 2, `brief doctor: too many arguments; run 'brief doctor'`.
- Stdout (text): one row per check, fixed order `config-file, config-parse, config-values…,
  config-shadow, root-dir, env-git, env-path`. Row = `<SEVERITY>  <id>  <path>  <detail>`, fields
  joined by exactly two spaces, no column padding; path relativized to wd the way `check`
  relativizes (its existing `displayPath`), `-` when a check has no path; a non-nil fix renders
  as `; fix: <fix>` appended to the detail.
- Stderr: `brief doctor: setup ok; run 'brief check' for feature content` when there is no ERROR
  and no WARN; otherwise `brief doctor: N ERROR, M WARN; this checks setup only, run 'brief check'
  for feature content`.
- Exit: 1 on any ERROR (sentinel `errDoctorFindings`, same shape as `errCheckFindings`), else 0.
  An invalid or unparseable `.brief.yaml` is **reported as rows, never refused**.
- `--json`: common header, then `counts:{error, warn, ok, skip}`, `checks:[{id, severity, path,
  detail, fix}]` — path absolute (`null` when none), fix `null` when nothing to do; zero stderr
  bytes, decided before any text rendering (mirror `runCheck`).
- `Check.Path` per id: `config-file`, `config-parse`, `config-values` → the nearest config
  (`<wd>/.brief.yaml` when none exists); `config-shadow` → the nearest config, shadowed
  ancestors named in the detail; `root-dir` → the absolute feature root; `env-git` → the `.git`
  found, none when absent; `env-path` → the PATH binary, none when not found.
- `config.Locate`'s nonexistent-startDir `ErrInvalidConfig` (unreachable from `os.Getwd`) is
  `out.refusal(err)`, exit 1 — the only refusal `doctor` emits.

## Decisions taken here

- **No config anywhere** → `config-file` WARN (path = `<wd>/.brief.yaml`, fix `brief init`);
  `config-parse`, `config-values`, `config-shadow` SKIP; `root-dir` checked against
  `Default().FeatureDirectory` relative to wd. (ERROR would make every un-inited repo exit 1.)
- **config-parse** ERROR on a YAML/unknown-key decode failure → `config-values` SKIP and
  `root-dir` SKIP (feature-directory unknown). A config that parses but has bad values still
  runs `root-dir` against its decoded `feature-directory` (that key carries no R1 rule).
- **config-values**: one ERROR row per `*config.ValueError`, detail = `ValueError.Error()`;
  exactly one OK row when there are none. Violations come from `internal/platform/config`
  (new `Inspect`), never a rule list in `doctor`.
- **Error wrapping is byte-preserved**: `Inspect` returns open/decode failures with the one
  existing `resolve config:` prefix (same `*InvalidConfigError` shape as today), and `Resolve`
  returns `Inspect`'s error, or wraps its first violation, exactly as `decodeConfig` does now —
  never a second prefix. An empty `.brief.yaml` stays valid (`io.EOF` → `Default()`, no error).
- **config-shadow**: always OK — detail names the shadowed ancestor files, or says none.
- **root-dir**: exists, is a directory, readable (open + close, never list entries), writable
  (create-temp-and-remove probe, deferred cleanup — the only write `doctor` performs; `go.mod`
  has no `golang.org/x/sys` and `syscall.Access` is unix-only). Missing → ERROR fix
  `mkdir -p <rel>`; not a directory → ERROR fix `set feature-directory in .brief.yaml`;
  unreadable and unwritable are separate ERROR details (checked in that order), both with fix
  `chmod u+rwx <rel>`. Never counts features.
- **env-git**: walk up from wd for `.git` as a directory **or a file** (worktrees). Found → OK;
  none → WARN, fix `git init`. No `git` binary.
- **env-path**: `lookPath("brief")` not found → WARN (ERROR is S10's arm). Found and the same
  file as `executable()` (both through `filepath.EvalSymlinks`, then `os.SameFile`) → OK.
  Different file → read its embedded module version with `debug/buildinfo.ReadFile` (reads,
  never executes) and compare to the running `versionString`: equal and neither is `(devel)`
  → OK; different, `(devel)`, or unreadable → WARN, fix names the PATH binary. Never runs
  `brief --version`.
- **Package**: new `internal/doctor`, imports `internal/platform/config` + stdlib only. Own
  `Severity` type (OK/WARN/ERROR/SKIP) — not `assemble`'s. **No `Store`**: ports are the
  environment seams `WithLookPath`, `WithExecutable`, `WithBinaryVersion`, `WithVersion`
  (defaults: `exec.LookPath`, `os.Executable`, a `buildinfo.ReadFile` adapter, `(devel)`);
  the filesystem is used directly.
- **CLI seam**: unexported `run` gains a trailing variadic `...doctor.Option`, appended after
  the options `runDoctor` builds itself (`doctor.WithVersion(versionString(readBuildInfo))`),
  so every existing `run`/`Run` call site compiles unchanged. `cmd/brief/main.go` is unchanged.
- Help: Use `doctor`, invocation const `brief doctor [--json]`, Short (R14, verbatim)
  `check brief's setup: config, feature root, host integration`; Long ends with
  `jsonFieldsParagraph("counts", "checks")` + `jsonScriptHint`. Read-only: **no**
  `writesFilesAnnotation`.

## Implementation Plan

- [x] Step 1: `internal/platform/config/resolve_test.go` `Test_inspect_reports_every_invalid_value_in_field_declaration_order` — a config with several bad values yields one `*ValueError` per value, in `Config` order, plus the decoded `Config`; table arms for a clean config (no violations), an empty file (valid, defaults), and an unparseable file (`*InvalidConfigError`, `resolve config:` prefix once) (red)
- [x] Step 2: `internal/platform/config/validate.go` — split into unexported `violations(cfg) []*ValueError` (all rules, declaration order; handoff-suffix rule skipped when the step pattern itself failed) and `validate` returning its first element (update)
- [x] Step 3: `internal/platform/config/resolve.go` `Inspect(path string) (Config, []*ValueError, error)` — decode without discarding the Config; decode failure keeps Resolve's `*InvalidConfigError` shape (new → green)
- [x] Step 4: `internal/platform/config/resolve_test.go` `Test_locate_returns_the_nearest_config_and_the_ancestors_it_shadows` — nearest-first; none found → empty; nonexistent startDir → `ErrInvalidConfig` (red)
- [x] Step 5: `internal/platform/config/resolve.go` `Locate(startDir string) (nearest string, shadowed []string, err error)` — the one walk-up; `Resolve` rewritten on `Locate` + `Inspect` (first violation) (new/update → green)
- [x] Step 6: `internal/platform/config/resolve_test.go` `Test_resolve_refuses_with_the_first_violation_inspect_reports` — agreement arm (new); the byte-level proof is `internal/cli/invalid_config_test.go` and every other SCENARIO-01 test passing **unmodified** — the agreement arm alone cannot catch a doubled prefix, since both sides move together
- [x] Step 7: `internal/platform/config/doc.go` — package doc names `Locate`/`Inspect` and the one-rule-set property (update)
- [x] Step 8: `internal/doctor/doctor_test.go` `Test_diagnose_…` table through `(*Server).Diagnose` with fake seams + `t.TempDir` trees: healthy (fixture creates `.git`), no config, unparseable config, multiple bad values, shadowed ancestors, root missing, root is a file, root unreadable (chmod 0o000), root unwritable (chmod 0o555) — both chmod arms skip under euid 0 and restore mode in `t.Cleanup`, `.git` as a file, no `.git`, brief not on PATH, same file, different file same version, different version, unreadable version (red)
- [x] Step 9: `internal/doctor/doc.go` — package doc: setup-only, never reads feature contents (new)
- [x] Step 10: `internal/doctor/doctor.go` — `Severity` + constants, `Check{ID, Severity, Path, Detail, Fix *string}`, `Report` + `Counts()`, `Server`, `Option`, `WithLookPath`/`WithExecutable`/`WithBinaryVersion`/`WithVersion`, `NewServer` with production defaults, `(*Server).Diagnose(ctx, wd) Report` (new)
- [x] Step 11: `internal/doctor/checks.go` — one unexported func per check id, composed by `Diagnose` in the fixed order (green)
- [x] Step 12: `internal/doctor/probe.go` — writability probe (create temp, close, remove, deferred) + `buildinfo.ReadFile` version adapter (green)
- [x] Step 13: `internal/doctor/doctor_test.go` `Test_diagnose_leaves_the_feature_root_byte_identical` — directory listing before/after, with a control arm showing the probe did run (counts the root as writable) (new)
- [x] Step 14: `internal/cli/doctor_internal_test.go` — through `run(..., doctor.With…)`: healthy → rows golden + `setup ok` stderr + exit 0; root missing → ERROR row with fix + `N ERROR, M WARN` summary + exit 1; unparseable config → config-parse ERROR, config-values SKIP, not a refusal; invalid values → one row each, exit 1; `--json` document golden (absolute paths, `null` fix/path, counts); `--json` writes no stderr; `brief doctor extra` → usage error exit 2 (red)
- [x] Step 15: `internal/cli/doctor.go` — `doctorLong`, `doctorInvocation`, `doctorDocument`/`doctorCheckJSON`/`doctorCountsJSON`, `errDoctorFindings`, `runDoctor` (does **not** call `resolveRoot`), text row renderer + summary (green)
- [x] Step 16: `internal/cli/cli.go` — register `leafCommand("doctor", …, addJSONFlag, …)` in `root.AddCommand` after `check`; thread `run`'s variadic `...doctor.Option` through `newRootCommand` to `runDoctor`; update the `init()` order comment to R14's full list (update)
- [x] Step 17: goldens — `expected one of: new, start, finish, status, check` → `…, check, doctor` in `completion_test.go`, `json_usage_test.go`, `run_test.go`, `help_test.go`, `cli_internal_test.go`, `flag_error_test.go`; `help_test.go` `rootHelp` gains the `brief doctor` row after `check`; `help_json_test.go:81` and `:240` name lists gain `"doctor"` (update). This list came from a grep; Step 20's full-suite run is the real enumeration — fix any further golden it reddens (e.g. a completion-script golden) the same way
- [x] Step 18: help tables — add `doctor` to `Test_every_command_help_has_a_usage_line_and_a_flag_table` (`help_test.go:228`, `:269`), `Test_every_leaf_help_line_fits_in_80_columns`, the JSON-fields-paragraph test and `help_json_test.go:161` (update)
- [x] Step 19: mutation-verify, one at a time, stashed: (a) break one rule in `violations` → S01 refusal test and doctor config-values row test both red; (b) reorder two rules → field-order test red; (c) drop the `.git`-as-file arm → worktree test red; (d) make the probe skip removal → byte-identical test red; (e) return OK for not-on-PATH → env-path test red; (e2) return OK for unreadable root → unreadable arm red; (e3) return OK for unwritable root → unwritable arm red; (f) make `runDoctor` call `resolveRoot` → unparseable-config CLI test red; (g) ERROR not setting `errDoctorFindings` → exit-1 test red
- [x] Step 20: `go build ./...`, `go test ./...` (unpiped, report count + delta), `go test -race ./internal/doctor/... ./internal/cli/... ./internal/platform/config/...`, `golangci-lint run ./...` → mark SCENARIO-02 done in specification.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `config.Locate` is the only walk-up and `config.Inspect` the only all-violations entry;
  `Resolve` = `Locate` + first of `Inspect` — closes STATE's "doctor must reuse Resolve's
  shape" debt; a second rule list anywhere lets S01's refusal and doctor's rows drift.
- `runDoctor` never calls `resolveRoot`/`Resolve` — an invalid config must become rows, not a
  refusal (R13 "one row per bad value").
- `internal/doctor` owns its `Severity`; it imports only `internal/platform/config` + stdlib —
  S10's host rows must arrive via a port the wiring supplies, not by importing a feature
  package (the S06 host adapter included).
- Row order is fixed; S10 appends `host-plugin, host-hook, host-snippet, host-agents, roles`
  **after** `env-path`.
- env-path severity here is OK/WARN only; "different version" is judged by path identity
  first, then `debug/buildinfo.ReadFile` — S10 adds the ERROR arm (integration installed, not
  on PATH) without changing the WARN rules.
- Seams: `WithLookPath` is the one S10's "not on PATH → ERROR" tests override;
  `WithBinaryVersion`/`WithVersion` only answer "same version?" and stay as-is.
- `Check.Path` per id is fixed (see User-visible contract); S10's host rows use the artifact's
  absolute path, none when not installed.
- The root-dir write probe is the only write `doctor` performs and cleans up after itself;
  S10 reuses the same helper for any writability question.
- No config anywhere is WARN (fix `brief init`), not ERROR — un-inited repos exit 0.
- Environment seams enter via `doctor.With*` options and `run`'s trailing `...doctor.Option`;
  no package-level vars.

**Left unbuilt** — named so nobody assumes it exists:
- `host-plugin`, `host-hook`, `host-snippet`, `host-agents`, `roles` rows and env-path ERROR —
  S10.
- `brief init` itself — S03; the `config-file` fix already names it.
- `init` and `uninstall` in the command list — S03/S04 insert them **around** doctor
  (`…, check, init, doctor, uninstall`), re-breaking every golden in Step 17/18 again.
- Line numbers on config-values rows (no `yaml.Node` decode) — unowned.

**Traps** — things that look right and are not:
- `validate`'s handoff-suffix rule needs the compiled step pattern; in all-violations mode a
  bad pattern must skip that rule, not cascade a bogus second row.
- A `t.TempDir` has no `.git` above it — the healthy fixture must create one or env-git flips
  to WARN and the `setup ok` golden fails.
- `chmod 0o555` tests pass vacuously as root — skip under euid 0.
- Under `go test`, `os.Executable` is the test binary and `(devel)` is the version — CLI tests
  must inject seams through `run`, never rely on the real PATH.
- Root help rows are exempt from the 80-column test (fixed-column cmdList); the doctor row
  exceeds 80 like `status`'s does — do not wrap it.
