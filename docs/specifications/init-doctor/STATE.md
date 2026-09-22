# init-doctor — current state

Scenarios complete: SCENARIO-01, SCENARIO-02. Last updated by SCENARIO-02.

## Binding decisions
- `config.Locate(startDir) (nearest, shadowed []string, err)` is the one walk-up;
  `config.Inspect(path) (Config, []*ValueError, err)` is the one all-violations decoder;
  `Resolve` = `Locate` + first element of `Inspect`. No second rule list or walk-up anywhere
  — `doctor`'s config-values rows and every command's refusal must never drift apart
  (SCENARIO-01, SCENARIO-02)
- A value error is `*config.InvalidConfigError{Path, Err: *config.ValueError{Key, Value,
  Reason, Err}}` — `doctor` and `init`'s "invalid existing config" refusal (S03, R3) recover
  key/value with `errors.AsType[*config.ValueError]` against this exact shape (SCENARIO-01)
- `Resolve` reports only `violations(cfg)[0]`, in `Config` field-declaration order (R14a, one
  problem per line); `doctor`'s config-values reports every element, same order, same
  function (SCENARIO-01, SCENARIO-02)
- `state-file`/`specification-file` compared case-insensitively; `feature-directory` exempt
  from the file-name/separator rule; `Default()` yields zero `violations()` — `init` (S03)
  must keep this holding (SCENARIO-01)
- `internal/doctor` imports only `internal/platform/config` + stdlib; owns its own `Severity`
  (not `assemble`'s). No `Store` — ports are `WithLookPath`/`WithExecutable`/
  `WithBinaryVersion`/`WithVersion`, filesystem used directly (SCENARIO-02)
- `runDoctor` calls `config.Locate` directly, never `resolveRoot`/`Resolve` — an invalid or
  unparseable config becomes rows, never a refusal; the only refusal `doctor` emits is
  `Locate`'s nonexistent-startDir guard (SCENARIO-02)
- Row order is fixed: config-file, config-parse, config-values, config-shadow, root-dir,
  env-git, env-path. S10 appends host-plugin, host-hook, host-snippet, host-agents, roles
  **after** env-path (SCENARIO-02)
- env-path here is OK/WARN only (not-on-PATH is WARN); S10 adds the ERROR arm (integration
  installed, not on PATH) without changing the WARN rules. `WithLookPath` is the seam S10's
  ERROR tests override (SCENARIO-02)
- `probeWritable` (create-temp-and-remove in the target dir) is the only write `doctor`
  performs; S10 reuses it for any writability question (SCENARIO-02)
- No config anywhere is WARN (fix `brief init`), not ERROR — un-inited repos exit 0
  (SCENARIO-02)
- Environment seams enter via `doctor.With*` options and `run`'s trailing `...doctor.Option`,
  appended after `runDoctor`'s own `doctor.WithVersion` — no package-level vars (SCENARIO-02)

## Left unbuilt
- Line number for a value error (no `yaml.Node` decode) — unowned (SCENARIO-01)
- Validation of `roles.*` and `optional-conventions` — S08 (SCENARIO-01)
- Heading-shape rules, absolute-path/`..` check on `feature-directory` — unowned (SCENARIO-01)
- `host-plugin`, `host-hook`, `host-snippet`, `host-agents`, `roles` rows and env-path's ERROR
  arm — S10 (SCENARIO-02)
- `brief init` itself — S03; `config-file`'s WARN fix already names it (SCENARIO-02)
- `init`/`uninstall` in the command list — S03/S04 insert **around** doctor (`…, check, init,
  doctor, uninstall`), re-breaking every "expected one of:" golden and root-help row again
  (SCENARIO-02)

## Traps
- Any CLI test that writes a `.brief.yaml` to reach a scaffold/assemble seam is refused at
  load if invalid — build config directly (`scaffold.NewServer(cfg, …)` / `fixtureConfig()`)
  instead of routing setup through `cli.Run` (SCENARIO-01)
- `ValueError.Err` must wrap stepfile errors with `%w`; `%q`-formatting a `Value` containing a
  backslash doubles it in `Error()` text (SCENARIO-01)
- A `t.TempDir` has no `.git` above it — a "healthy" fixture must create one or env-git flips
  to WARN (SCENARIO-02)
- `chmod 0o000`/`0o555` root-dir tests pass vacuously as root — skip under `os.Geteuid() == 0`
  (SCENARIO-02)
- Under `go test`, `os.Executable` is the test binary and PATH's `brief` lookup is
  environment-dependent — CLI tests must inject `doctor.With*` seams through `run`'s trailing
  variadic, never rely on the real PATH/executable (SCENARIO-02)
- `filepath.EvalSymlinks` on the found PATH binary must never leak into `Check.Path` — a
  `t.TempDir()` under a symlinked `/tmp` makes a resolved path diverge from `wd`, corrupting
  `displayPath`'s relative render; `env-path`'s `Path` is always the unresolved `found`
  (SCENARIO-02)
- Root help rows are exempt from the 80-column test (fixed-column `cmdList`); the `doctor` row
  exceeds 80 like `status`'s does — do not wrap it (SCENARIO-02)
- `doctor.Report.Counts()`'s switch has no `default` arm — S10 adding a fifth `Severity`
  without a matching `case` makes `counts.Error+Warn+OK+Skip != len(Checks)`, and nothing
  catches it (SCENARIO-02)

## Open debts
- `init`'s "invalid existing config" refusal (S03, R3) must recover key/value via
  `errors.AsType[*config.ValueError]` — S03 must close it
- `roles.*` / `optional-conventions` validation — S08 must close it, or the spec must say
  explicitly these stay unvalidated
