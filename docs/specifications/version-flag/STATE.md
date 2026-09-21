# version-flag — current state

Scenarios complete: SCENARIO-01, SCENARIO-02. Last updated by SCENARIO-02.

## Binding decisions

- Build info is injected as `func() (*debug.BuildInfo, bool)` — `debug.ReadBuildInfo`'s own
  signature — threaded through unexported `run(ctx, wd, args, stdin, stdout, stderr,
  readBuildInfo)` → `newRootCommand(wd, stdin, stdout, stderr, readBuildInfo)` → `runRoot(cmd,
  args, stdout, stderr, readBuildInfo)`. `cli.Run` keeps its exported signature and is a
  one-line delegate passing `debug.ReadBuildInfo`. No package-level reader var. (SCENARIO-01)
- `runRoot` takes `stdout` as an explicit parameter (no longer `cmd.OutOrStdout()`). Any new
  `runRoot` arm that writes to stdout uses that same parameter. (SCENARIO-01)
- `classifyDashArg("--version")` (exact match only) returns `argVersionFlag`, msg
  `unknownLongFlagMessage("--version")`. `--version=x` still falls through to
  `argUnknownFlag` (SCENARIO-04's rule). `new` (`runNew`) and the help stub
  (`newHelpCommand`) fold `argVersionFlag` into their `argUnknownFlag` arm byte-for-byte —
  `--version` reports exactly what an unrecognized flag would there (R6).
- `runRoot`'s `argVersionFlag` arm: `len(args) == 1` writes `versionLine(readBuildInfo) +
  "\n"` to stdout and returns nil; any trailing arg falls to today's unknown-flag message
  (SCENARIO-03 replaces this branch with the "takes no arguments" copy).
- `versionLine(readBuildInfo) string` (unexported, `internal/cli/cli.go`): returns `"brief "
  + info.Main.Version` verbatim when `ok` and `Main.Version` is non-empty; returns the
  literal `"brief (devel)"` when `ok == false` (checked before `info` is ever dereferenced)
  or `Main.Version == ""` — asking for the version never fails (R2). The fallback text is an
  inline literal, not a constant, so a mutation of it stays falsifiable. No
  `internal/version` package — version text stays in `internal/cli`.
- **Confirmed via a throwaway probe test (removed, not committed):** in a `go test` binary,
  `debug.ReadBuildInfo()` returns `ok=true`, `Main.Version == "(devel)"` — never `""` and
  never `ok=false`. Only the `(devel)` fallback is reachable through a black-box `cli.Run`
  test in this repo's own test binaries;
  `Test_version_flag_through_Run_prints_one_brief_line_to_stdout` asserts the exact literal
  `"brief (devel)\n"` on that basis.

## Left unbuilt

- `argVersionFlag` + trailing-arg copy `'--version' takes no arguments; run 'brief
  --version'`: SCENARIO-03 (currently reports today's unknown-flag wording instead).
- `--version=x` / `--version=` → `'--version' takes no value; run 'brief --version'`:
  SCENARIO-04 (currently classifies as plain `argUnknownFlag`, unknown-flag wording).
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
  clean as of SCENARIO-02, but a future kind addition breaks all three sites again until each
  gets an arm. `cli_internal_test.go:28`'s `newTreeWithExtraCommands` is a hidden
  `newRootCommand` caller whose signature includes `readBuildInfo`.
- cobra's `Command.Version` / `InitDefaultVersionFlag` never fire: root has `DisableFlagParsing:
  true`. Do not set `root.Version` — it would be silently inert.
- In a `go test` binary, `Main.Version` is `"(devel)"` — black-box tests through `cli.Run`
  can assert the fallback exactly but never the pass-through case. Use the `run` seam
  (`version_internal_test.go`, `package cli`) with a fake `readBuildInfo` for any pass-through
  exact-value assertion.
- The `"(devel)"` row in `versionLine`'s fallback table cannot redden under any guard
  mutation — pass-through and fallback produce the same bytes for that one input. It is a
  behavior pin, not guard evidence; only the `""`, `(nil,false)` and `ok=false`-with-info
  rows prove the guards.
- A guard written as `info == nil` instead of `!ok` would pass the `(nil,false)` row but fail
  the `ok=false`-with-info row — that pairing is why both rows exist in the table.
- `Test_version_flag_through_Run_prints_one_brief_line_to_stdout` now passes for a degenerate
  reader too (`(nil,false)` yields the same `brief (devel)\n`), so a wiring regression (e.g.
  `Run` stops passing `debug.ReadBuildInfo`) is only caught by that test asserting a
  distinctive non-`(devel)` version — the mutation this scenario ran to verify it.

## Open debts

- None open. Every remaining gap is explicitly assigned to SCENARIO-03 through SCENARIO-07,
  all still pending in `specification.md`'s `## BDD Acceptance Progress`.
