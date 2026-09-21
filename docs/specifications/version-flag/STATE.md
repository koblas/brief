# version-flag — current state

Scenarios complete: SCENARIO-01..06. Last updated by SCENARIO-06.

## Binding decisions

- Build info is injected as `func() (*debug.BuildInfo, bool)` — `debug.ReadBuildInfo`'s own
  signature — threaded through unexported `run(...)` → `newRootCommand(...)` →
  `runRoot(cmd, args, stdout, stderr, readBuildInfo)`. `cli.Run` keeps its exported
  signature, delegating to `debug.ReadBuildInfo` in one line; no package-level reader var.
  In a `go test` binary, `debug.ReadBuildInfo()` always returns `ok=true,
  Main.Version == "(devel)"`, so only the fallback is reachable through black-box `cli.Run`
  tests; use the `run` seam (`version_internal_test.go`, `package cli`) with a fake
  `readBuildInfo` for a pass-through exact-value assertion. (SCENARIO-01)
- `argKind` has six values. `classifyDashArg("--version")` returns `argVersionFlag`;
  `classifyDashArg("--version=<v>")` (any value, incl. empty) returns
  `argVersionFlagWithValue`. Both carry msg `unknownLongFlagMessage(arg)` =
  `unknown flag: --version`, so `runNew`/the help stub fold both into their existing
  `argUnknownFlag, argVersionFlag, argVersionFlagWithValue` case byte-for-byte (R6). Only
  root tells the two apart. (SCENARIO-01, SCENARIO-03, SCENARIO-04)
- Root's sole-argument arms: `argVersionFlag` with `len(args) == 1` prints
  `versionLine(readBuildInfo) + "\n"`, exit 0; a trailing argument →
  `takesNoArgumentsMessage(args[0], "brief --version")`. `argVersionFlagWithValue` never
  checks `len(args)`: a value always wins over a trailing argument (R8) →
  `takesNoValueMessage("--version", "brief --version")` — the literal `"--version"`, not
  `msg` or `args[0]`. First argument alone decides which flag's error fires: `--help
  --version` reports `--help`'s own error. (SCENARIO-03, SCENARIO-04)
- `takesNoArgumentsMessage(flag, runHint)`/`takesNoValueMessage(flag, runHint)` (unexported,
  `cli.go`) render root's own `brief: '<flag>' takes no arguments/value; run '<runHint>'`.
  Root-scoped only. `runNew`/the help stub keep their own inline copies with their own
  prefixes/hints, untouched. (SCENARIO-03, SCENARIO-04)
- `versionLine(readBuildInfo) string` (unexported, `cli.go`): `"brief " + info.Main.Version`
  verbatim when `ok` and non-empty; else literal `"brief (devel)"` (R2). No
  `internal/version` package. (SCENARIO-01)
- `-v` (and clusters `-v=x`, `-vh`, `-hv`) is not a `--version` alias at any site —
  `classifyDashArg` sends it through `argUnknownFlag`/`unknownShortFlagMessage`, unchanged;
  reserved for a future `--verbose` (R5). Pinned in the cross-site table in
  `flag_error_test.go`. No single mutation guards it: a "`-v` prefix → `argVersionFlag`"
  mutation reaches `-v`/`-v=x`/`-vh` but never `-hv`; widening `isAllH` to accept `'v'` is
  the only mutation that reaches `-hv`. A future R5 guard needs both proofs. (SCENARIO-05)
- R6, --version outside root: every leaf's `--version` is an ordinary pflag unknown-flag
  error — no leaf registers a `version` flag, no persistent flags exist in `internal/cli`,
  so only the `start` rows are mutation-proven, the rest behavior pins. Pinned in
  `Test_reports_the_version_flag_as_unknown_outside_the_root` (`flag_error_test.go`); `new
  --version` stays standalone, `new`/`help` `--version=*` stay in the cross-site table.
  (SCENARIO-06)

## Left unbuilt

- Root-help trailer `Run 'brief --version' to print the installed version.`: SCENARIO-07
  (`helpTemplate` unchanged so far).
- Leaf `-v` (e.g. `brief start -v`): unpinned, goes through real pflag not
  `classifyDashArg`, so R5's risk doesn't reach it. Owner: none (out of scope).

## Traps

- `exhaustive` lint (`default: all`, no `default:` case). Every switch on `argKind` (root,
  `new`, help stub) must name all six values — clean as of SCENARIO-05.
  `cli_internal_test.go:28`'s `newTreeWithExtraCommands` is a hidden `newRootCommand` caller
  whose signature includes `readBuildInfo` — a later signature change breaks it too.
- cobra's `Command.Version`/`InitDefaultVersionFlag` never fire: root has
  `DisableFlagParsing: true`. Don't set `root.Version` — it would be silently inert.
- Don't refactor `runNew`/the help stub onto `takesNoArgumentsMessage`/`takesNoValueMessage`:
  their prefixes/hints differ, and R6 requires their bytes unchanged.
- Rendering root's `--version=<v>` error from `msg` gives `'unknown flag: --version' takes
  no value`; from `args[0]` gives `'--version=x' takes no value`. Both wrong — the flag name
  is the literal `"--version"`.
- `strings.HasPrefix(arg, "--version")` without the `=` catches `--versionx`/`--version-foo`
  too; the classify table's `--versionx` row is the only guard against this.
- The `"(devel)"` fallback row, the `--help --version` row (R8), `new`/`help`
  `--version=*` rows, and the six non-`start` leaf `--version` rows (SCENARIO-06) are all
  behavior pins green on arrival, not guard evidence — no guard in this package sits behind
  them to break.
- `brief help start --version` does not reach a leaf: the help stub treats the joined
  remainder as a command name (`unknown command "start --version"`). Not usable as a leaf
  row. (SCENARIO-06)

## Open debts

- None open. The remaining gap (root-help trailer) is assigned to SCENARIO-07, still
  pending in `specification.md`'s `## BDD Acceptance Progress`.
