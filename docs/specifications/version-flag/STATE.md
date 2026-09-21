# version-flag — current state

Scenarios complete: SCENARIO-01, SCENARIO-02, SCENARIO-03. Last updated by SCENARIO-03.

## Binding decisions

- Build info is injected as `func() (*debug.BuildInfo, bool)` — `debug.ReadBuildInfo`'s own
  signature — threaded through unexported `run(ctx, wd, args, stdin, stdout, stderr,
  readBuildInfo)` → `newRootCommand(wd, stdin, stdout, stderr, readBuildInfo)` → `runRoot(cmd,
  args, stdout, stderr, readBuildInfo)`. `cli.Run` keeps its exported signature and is a
  one-line delegate passing `debug.ReadBuildInfo`. No package-level reader var. (SCENARIO-01)
- `classifyDashArg("--version")` (exact match only) returns `argVersionFlag`, msg
  `unknownLongFlagMessage("--version")`. `--version=x` still falls through to
  `argUnknownFlag` (SCENARIO-04's rule). `new` (`runNew`) and the help stub
  (`newHelpCommand`) fold `argVersionFlag` into their `argUnknownFlag` arm byte-for-byte —
  `--version` reports exactly what an unrecognized flag would there (R6, unchanged by
  SCENARIO-03).
- `runRoot`'s `argVersionFlag` arm: `len(args) == 1` writes `versionLine(readBuildInfo) +
  "\n"` to stdout and returns nil; any trailing argument returns
  `takesNoArgumentsMessage(args[0], "brief --version")` — `brief: '--version' takes no
  arguments; run 'brief --version'` — using `args[0]`, not `msg`. First argument alone
  decides (R8): `--help --version` still reports `--help`'s own error; `--version --help`
  reports `--version`'s.
- `takesNoArgumentsMessage(flag, runHint string) string` (unexported, `internal/cli/cli.go`)
  renders `brief: '<flag>' takes no arguments; run '<runHint>'`; shared by root's
  `argHelpFlag` (`runHint = "brief help <command>"`) and `argVersionFlag`
  (`runHint = "brief --version"`) arms only. Root-scoped — `brief: ` prefix is baked in;
  `runNew`/the help stub build their own copy inline and are untouched.
- `versionLine(readBuildInfo) string` (unexported, `internal/cli/cli.go`): returns `"brief "
  + info.Main.Version` verbatim when `ok` and `Main.Version` is non-empty; returns the
  literal `"brief (devel)"` when `ok == false` or `Main.Version == ""` (R2). No
  `internal/version` package — version text stays in `internal/cli`.
- **Confirmed via a throwaway probe test (removed, not committed):** in a `go test` binary,
  `debug.ReadBuildInfo()` returns `ok=true`, `Main.Version == "(devel)"` — never `""` and
  never `ok=false`. Only the `(devel)` fallback is reachable through a black-box `cli.Run`
  test in this repo's own test binaries.

## Left unbuilt

- `--version=x` / `--version=` → `'--version' takes no value; run 'brief --version'`:
  SCENARIO-04 (currently classifies as plain `argUnknownFlag`, unknown-flag wording).
  SCENARIO-04 should add a sibling `takesNoValueMessage(flag, runHint)` next to
  `takesNoArgumentsMessage` rather than re-inlining `Sprintf` — `--help`'s value arm
  (`argHelpFlagWithValue`) is its second user.
- `-v` pinning as an unknown shorthand: SCENARIO-05 (untouched — already unknown-shorthand
  today, no code change expected, just a pinning test).
- Full `new`/`help`/leaf byte-identity table for `--version`: SCENARIO-06. SCENARIO-01 added
  exactly one guard row; `help --version` and leaf `--version` (e.g. `start --version demo`)
  are unpinned.
- Root-help trailer line `Run 'brief --version' to print the installed version.`: SCENARIO-07
  (`helpTemplate` unchanged so far).

## Traps

- `exhaustive` lint is enabled (`default: all`, no `default:` case allowed). Every
  `classifyDashArg` switch (root, `new`, help stub) must carry an arm for every `argKind` —
  clean as of SCENARIO-03; no new `argKind` was added this scenario.
  `cli_internal_test.go:28`'s `newTreeWithExtraCommands` is a hidden `newRootCommand` caller
  whose signature includes `readBuildInfo` — a later change to that signature breaks it too.
- cobra's `Command.Version` / `InitDefaultVersionFlag` never fire: root has `DisableFlagParsing:
  true`. Do not set `root.Version` — it would be silently inert.
- In a `go test` binary, `Main.Version` is `"(devel)"` — black-box tests through `cli.Run`
  can assert the fallback exactly but never the pass-through case. Use the `run` seam
  (`version_internal_test.go`, `package cli`) with a fake `readBuildInfo` for any pass-through
  exact-value assertion.
- The `"(devel)"` row in `versionLine`'s fallback table cannot redden under any guard
  mutation — it is a behavior pin, not guard evidence.
- Don't refactor `runNew`/the help stub onto `takesNoArgumentsMessage`: their prefixes
  (`brief new: `, `brief help: `) and hints differ, and R6 requires their bytes unchanged.
- The `--help --version` control row (R8) is a behavior pin, green on arrival; no plausible
  guard mutation in the `argVersionFlag` arm can redden it — don't claim it as guard evidence
  for that arm.

## Open debts

- None open. Every remaining gap is explicitly assigned to SCENARIO-04 through SCENARIO-07,
  all still pending in `specification.md`'s `## BDD Acceptance Progress`.
