# version-flag — current state

Scenarios complete: SCENARIO-01, SCENARIO-02, SCENARIO-03, SCENARIO-04. Last updated by
SCENARIO-04.

## Binding decisions

- Build info is injected as `func() (*debug.BuildInfo, bool)` — `debug.ReadBuildInfo`'s own
  signature — threaded through unexported `run(ctx, wd, args, stdin, stdout, stderr,
  readBuildInfo)` → `newRootCommand(...)` → `runRoot(cmd, args, stdout, stderr,
  readBuildInfo)`. `cli.Run` keeps its exported signature and is a one-line delegate passing
  `debug.ReadBuildInfo`. No package-level reader var. (SCENARIO-01)
- `argKind` has six values. `classifyDashArg("--version")` (exact match) returns
  `argVersionFlag`; `classifyDashArg("--version=<v>")` (any value, including empty) returns
  `argVersionFlagWithValue`. Both carry msg `unknownLongFlagMessage(arg)` =
  `unknown flag: --version` — the same wording `argUnknownFlag` would report — so `runNew`
  and the help stub fold both kinds into their existing `argUnknownFlag, argVersionFlag,
  argVersionFlagWithValue` case byte-for-byte (R6). Only root tells the two apart. (see
  Traps for the `exhaustive` lint's scope) (SCENARIO-01, SCENARIO-03, SCENARIO-04)
- Root's sole-argument arms: `argVersionFlag` with `len(args) == 1` prints
  `versionLine(readBuildInfo) + "\n"` to stdout, exit 0. Any trailing argument →
  `takesNoArgumentsMessage(args[0], "brief --version")`. `argVersionFlagWithValue` never
  checks `len(args)`: a value on `--version` always wins over a trailing argument (R8) →
  `takesNoValueMessage("--version", "brief --version")` — the literal `"--version"`, not
  `msg` (unknown-flag wording) and not `args[0]` (still carries the value). First argument
  alone decides which flag's error fires: `--help --version` reports `--help`'s own error.
  (SCENARIO-03, SCENARIO-04)
- `takesNoArgumentsMessage(flag, runHint)` and `takesNoValueMessage(flag, runHint)`
  (unexported, `internal/cli/cli.go`) render root's own `brief: '<flag>' takes no
  arguments/value; run '<runHint>'`. Root-scoped only — shared by root's `argHelpFlag`/
  `argVersionFlag` and `argHelpFlagWithValue`/`argVersionFlagWithValue` arms respectively.
  `runNew`/the help stub build their own inline copies with their own prefixes and hints and
  are untouched. (SCENARIO-03, SCENARIO-04)
- `versionLine(readBuildInfo) string` (unexported, `internal/cli/cli.go`): `"brief " +
  info.Main.Version` verbatim when `ok` and `Main.Version` non-empty; else literal
  `"brief (devel)"` (R2). No `internal/version` package. (SCENARIO-01)
- **Confirmed via a throwaway probe test (removed, not committed):** in a `go test` binary,
  `debug.ReadBuildInfo()` returns `ok=true`, `Main.Version == "(devel)"` — never `""` and
  never `ok=false`. Only the `(devel)` fallback is reachable through a black-box `cli.Run`
  test in this repo's own test binaries. (SCENARIO-01)

## Left unbuilt

- `-v` pinning as an unknown shorthand: SCENARIO-05 (untouched — already unknown-shorthand
  today, no code change expected, just a pinning test).
- Full `new`/`help`/leaf byte-identity table for `--version` and `--version=<v>`:
  SCENARIO-06. `help --version` and leaf `--version` (e.g. `start --version demo`) are
  unpinned; `new`/`help` `--version=x`/`--version=` are pinned as of SCENARIO-04.
- Root-help trailer line `Run 'brief --version' to print the installed version.`:
  SCENARIO-07 (`helpTemplate` unchanged so far).

## Traps

- `exhaustive` lint is enabled (`default: all`, no `default:` case allowed). Every switch on
  `argKind` (root, `new`, help stub) must name all six values — clean as of SCENARIO-04.
  `cli_internal_test.go:28`'s `newTreeWithExtraCommands` is a hidden `newRootCommand` caller
  whose signature includes `readBuildInfo` — a later signature change breaks it too.
- cobra's `Command.Version` / `InitDefaultVersionFlag` never fire: root has
  `DisableFlagParsing: true`. Do not set `root.Version` — it would be silently inert.
- In a `go test` binary, `Main.Version` is `"(devel)"` — black-box tests through `cli.Run`
  can assert the fallback exactly but never the pass-through case. Use the `run` seam
  (`version_internal_test.go`, `package cli`) with a fake `readBuildInfo` for any
  pass-through exact-value assertion.
- Don't refactor `runNew`/the help stub onto `takesNoArgumentsMessage`/`takesNoValueMessage`:
  their prefixes (`brief new: `, `brief help: `) and hints differ, and R6 requires their
  bytes unchanged.
- Rendering root's `--version=<v>` error from `msg` gives `'unknown flag: --version' takes
  no value`; from `args[0]` gives `'--version=x' takes no value`. Both wrong — the flag name
  is the literal `"--version"`.
- `strings.HasPrefix(arg, "--version")` without the `=` catches `--versionx`/`--version-foo`
  too. The classify table's `--versionx` control row is the only guard against this.
- The `"(devel)"` fallback row, the `--help --version` control row (R8), and the `new`/`help`
  `--version=*` rows are behavior pins green on arrival, not guard evidence — each has its
  own mutation elsewhere proving the guard exists.

## Open debts

- None open. Every remaining gap is explicitly assigned to SCENARIO-05 through SCENARIO-07,
  all still pending in `specification.md`'s `## BDD Acceptance Progress`.
