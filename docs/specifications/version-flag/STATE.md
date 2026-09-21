# version-flag — current state

Scenarios complete: SCENARIO-01. Last updated by SCENARIO-01.

## Binding decisions

- Build info is injected as `func() (*debug.BuildInfo, bool)` — `debug.ReadBuildInfo`'s own
  signature — threaded through unexported `run(ctx, wd, args, stdin, stdout, stderr,
  readBuildInfo)` → `newRootCommand(wd, stdin, stdout, stderr, readBuildInfo)` → `runRoot(cmd,
  args, stdout, stderr, readBuildInfo)`. `cli.Run` keeps its exported signature and is now a
  one-line delegate passing `debug.ReadBuildInfo`. No package-level reader var. (SCENARIO-01)
- `runRoot` takes `stdout` as an explicit parameter (no longer `cmd.OutOrStdout()`). Any new
  `runRoot` arm that writes to stdout uses that same parameter. (SCENARIO-01)
- `classifyDashArg("--version")` (exact match only, checked before the generic `--` branch)
  returns `argVersionFlag`, msg `unknownLongFlagMessage("--version")` = `"unknown flag:
  --version"`. `--version=x` still falls through to `argUnknownFlag` (a different rule: S04's
  "takes no value"). `new` (`runNew`) and the help stub (`newHelpCommand`) both fold
  `argVersionFlag` into their `argUnknownFlag` arm byte-for-byte — `--version` reports exactly
  what an unrecognized flag would there, since it is root-only (R6).
- `runRoot`'s `argVersionFlag` arm: `len(args) == 1` writes `versionLine(readBuildInfo) +
  "\n"` to stdout and returns nil; any trailing arg falls to today's unknown-flag message
  (`"brief: unknown flag: --version; run 'brief <command> --help'"` — S03 replaces this branch
  with the "takes no arguments" copy).
- `versionLine(readBuildInfo) string` (unexported, in `internal/cli/cli.go`) renders `"brief "
  + info.Main.Version` with **no fallback guard** — `info` is dereferenced unconditionally.
  Version text stays in `internal/cli`; no `internal/version` package.
- **Confirmed via a throwaway probe test (removed, not committed):** in a `go test` binary,
  `debug.ReadBuildInfo()` returns `ok=true` and `Main.Version == "(devel)"` — never `""` and
  never `ok=false`. This is why S02's three fallback cases (`(devel)`, empty, `ok==false`) all
  still need real fakes through the `run` seam; only `(devel)` is reachable through a black-box
  `cli.Run` test in this repo's own test binaries.

## Left unbuilt

- `versionLine`'s fallback guard for `Main.Version == "(devel)"`, `== ""`, and `ok == false` —
  the first two currently print the literal string; the third is a **reachable nil-pointer
  panic** (`info` is dereferenced unconditionally) if `readBuildInfo` ever returns `(nil,
  false)`. Nothing today prevents `Run`'s production reader from doing that. SCENARIO-02 must
  add the guard. Until it does, SCENARIO-01's mutation check 11(a) relies on that same panic
  being observable (it wires `Run` to a reader returning `nil, false` and expects
  `Test_version_flag_through_Run_prints_one_brief_line_to_stdout` to redden) — once S02 adds
  the guard, that check's expected failure mode changes from "panic" to "prints the fallback
  text", and 11(a) should be re-run against the new behavior, not left pointing at a mutation
  that no longer panics. S02 also tightens
  `Test_version_flag_through_Run_prints_one_brief_line_to_stdout`'s exact-value assertion.
- `argVersionFlag` + trailing-arg copy `'--version' takes no arguments; run 'brief --version'`:
  SCENARIO-03 (currently reports today's unknown-flag wording instead).
- `--version=x` / `--version=` → `'--version' takes no value; run 'brief --version'`:
  SCENARIO-04 (currently classifies as plain `argUnknownFlag`, unknown-flag wording).
- `-v` pinning as an unknown shorthand: SCENARIO-05 (untouched — already unknown-shorthand
  today, no code change expected, just a pinning test).
- Full `new`/`help`/leaf byte-identity table for `--version`: SCENARIO-06. SCENARIO-01 added
  exactly one guard row (`flag_error_test.go`'s `Test_new_reports_the_version_flag_as_unknown`);
  `help --version` and leaf `--version` (e.g. `start --version demo`) are unpinned.
- Root-help trailer line `Run 'brief --version' to print the installed version.`: SCENARIO-07
  (`helpTemplate` unchanged so far).

## Traps

- `exhaustive` lint is enabled (`default: all`, no `default:` case allowed). Every
  `classifyDashArg` switch (root, `new`, help stub) must carry an arm for every `argKind,`
  including any new one — verified clean by `golangci-lint run ./...` as of SCENARIO-01, but a
  future kind addition breaks all three sites again until each gets an arm.
  `cli_internal_test.go:28`'s `newTreeWithExtraCommands` is a hidden `newRootCommand` caller —
  its signature now includes `readBuildInfo`, and it stops compiling if that parameter list
  changes again.
- cobra's `Command.Version` / `InitDefaultVersionFlag` never fire: root has `DisableFlagParsing:
  true`. Do not set `root.Version` — it would be silently inert.
- In a `go test` binary, `Main.Version` is `"(devel)"` (confirmed above), never the real module
  version — black-box tests through `cli.Run` cannot assert an exact version string. Use the
  `run` seam (`version_internal_test.go`, `package cli`) with a fake `readBuildInfo` for any
  exact-value assertion.
- `versionLine` panics on `readBuildInfo` returning `(nil, false)` — a real bug, not a design
  choice; see the "Left unbuilt" entry above for exactly how SCENARIO-02 must close it and
  what mutation check 11(a) currently depends on.

## Open debts

- `versionLine`'s nil-pointer panic on `readBuildInfo` returning `(nil, false)` — a reachable
  production panic, not covered by any guard today — SCENARIO-02 must close it (see "Left
  unbuilt" and "Traps" above for the exact mechanics and the mutation-check dependency).
- Every other gap above is explicitly assigned to SCENARIO-03 through SCENARIO-07, all still
  pending in `specification.md`'s `## BDD Acceptance Progress`.
