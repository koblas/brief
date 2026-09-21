# SCENARIO-01: --version prints the build's stored version

## Scenario

```gherkin
Scenario Outline: SCENARIO-01 --version prints the build's stored version
  Given brief was built with stored module version "<stored>"
  When I run "brief --version"
  Then stdout is exactly "brief <stored>" followed by a newline
  And stderr is empty and the exit code is 0
  Examples:
    | stored                                          |
    | v0.3.0                                          |
    | v0.0.0-20260921145925-5709f33424c3              |
    | v0.0.0-20260921145925-5709f33424c3+dirty        |
```

User-visible contract: `brief --version` (sole argument) writes `brief <Main.Version>\n` to
stdout, nothing to stderr, `Run` returns nil, so `ExitCode` gives 0. Every other shape
(`--version=x`, trailing args, `-v`, `new`/`help`/leaf `--version`) keeps today's output in
this scenario.

## Seam choice (R3)

Build info reaches `runRoot` as a `func() (*debug.BuildInfo, bool)`, the same signature as
`debug.ReadBuildInfo`, so production passes the stdlib function itself with no adapter.
`cli.Run` keeps its exported signature. It delegates to a new unexported `run` that takes
the same arguments plus the build-info reader, and `Run` passes `debug.ReadBuildInfo`.
`run` threads the reader through `newRootCommand` into `runRoot`. The white-box tests call
`run` with a fake reader, so they go through the same `SetArgs`/`SetOut`/`ExecuteContext` path
as production. The only difference is where the build info comes from.

Rejected alternatives:
- A package-level `var readBuildInfo = debug.ReadBuildInfo` swapped by tests. `gochecknoglobals`
  is disabled, so lint would not catch it, but it is still mutable package state, and the
  skill says dependencies are injected, never reached for. It also rules out `t.Parallel`.
- Variadic `...Option` on `cli.Run`. This changes the exported signature for a dependency
  that has exactly one production value.

Version formatting stays in `internal/cli`, with no new `internal/version` package. S01 only
passes `Main.Version` through verbatim and adds the `brief ` prefix, and output formatting is
`internal/cli`'s job. The skill says "add a directory when a package earns one." S02's
`(devel)` fallback is the first real decision, and it goes in an unexported pure func in
`internal/cli` (see Handoff).

## Implementation Plan

- [x] Step 1: `internal/cli/version_internal_test.go` (`package cli`) `Test_version_flag_prints_the_stored_module_version_verbatim`: table over the three Examples rows. Each row calls `run` with a fake reader that returns `Main.Version = <stored>, true`, then asserts nil error, empty stderr, and stdout exactly `brief <stored>\n` (red: does not compile yet)
- [x] Step 2: `internal/cli/run_test.go` `Test_returns_a_usage_error_for_an_unknown_double_dash_flag_at_the_root`: repoint the arg and expected stderr from `--version` to `--bogus`, and rewrite its doc comment, which currently justifies choosing `--version` (update, stays green)
- [x] Step 3: `internal/cli/classify.go`: add `argVersionFlag` to `argKind` for exactly `arg == "--version"`, checked before the generic `--` branch. Its msg is `unknownLongFlagMessage(arg)`, so `new`/`help` can reuse it byte-for-byte. `--version=x` must still fall through to `argUnknownFlag`. Update the `argKind` and `classifyDashArg` doc comments from "four" to "five" kinds (new)
- [x] Step 4: `internal/cli/classify_internal_test.go` `Test_classifyDashArg_classifies_every_token_shape`: add a `--version` → `argVersionFlag` row with msg `unknown flag: --version`, since the table claims to document every `argKind`. Add `--version` and `--version=x` to the `FuzzClassifyDashArg` seeds (update)
- [x] Step 5: `internal/cli/new.go` `runNew`: add an `argVersionFlag` arm that reports the same message as `argUnknownFlag` (fold into that case). Required because `exhaustive` is enabled and the switch has no `default` (update)
- [x] Step 6: `internal/cli/cli.go` help stub in `newHelpCommand`: same `argVersionFlag` → unknown-flag arm as Step 5 (update)
- [x] Step 7: `internal/cli/cli.go`: extract unexported `run(ctx, wd, args, stdin, stdout, stderr, readBuildInfo)`. `Run` becomes a one-line delegate passing `debug.ReadBuildInfo`. `newRootCommand` gains the reader parameter, and the root `RunE` passes `stdout` and the reader to `runRoot`. Update `Run`'s and `newRootCommand`'s doc comments (update)
- [x] Step 8: `internal/cli/cli_internal_test.go:28` `newTreeWithExtraCommands`: pass a reader to the new `newRootCommand` parameter so it compiles. Any non-nil func works, because that helper never dispatches `--version` (update)
- [x] Step 9: `internal/cli/cli.go` `runRoot`: take `stdout` explicitly (not `cmd.OutOrStdout()`) plus the reader. Add an `argVersionFlag` arm: when `len(args) == 1`, write `brief <Main.Version>\n` to stdout and return nil. When there are trailing args, keep today's unknown-flag message for `--version` (S03 owns the new copy). Update `runRoot`'s doc comment (green)
- [x] Step 10: `internal/cli/run_test.go` `Test_version_flag_through_Run_prints_one_brief_line_to_stdout`: black-box through `cli.Run` with the real build info. Assert nil error, empty stderr, and a single stdout line with prefix `brief ` (it must not assert the version text, which is `""` or `(devel)` in a test binary and belongs to S02). This is the only test proving `Run` wires in `debug.ReadBuildInfo` (new)
- [x] Step 11: mutation checks, each stashed and restored individually. (a) `Run` passes a reader returning `nil, false`: Step 10 should go red (panic or wrong output), which shows the wiring is observed. (b) `runRoot` writes to stderr instead of stdout: Step 1 goes red. (c) Drop the `argVersionFlag` arm in `runNew` so it falls to the `argNotFlag` path: `new --version` output changes. If no test reddens, add one row to `flag_error_test.go`'s `new` table pinning `brief new --version` (S06 owns the full pinning)
- [x] Step 12: `go build ./...`, `go test ./...` (unpiped, report count and delta), `go test -race ./internal/cli/...`, `golangci-lint run ./...` all clean, then mark SCENARIO-01 done in `specification.md`

## Handoff

**Binding decisions:**
- Build info is injected as `func() (*debug.BuildInfo, bool)` through unexported `run` → `newRootCommand` → `runRoot`. `cli.Run` keeps its signature and passes `debug.ReadBuildInfo`. S02 drives `run` with fakes for `(devel)`, `""`, and `nil, false`. No package-level reader var.
- `runRoot` takes `stdout` as an explicit parameter. The white-box path never calls `cmd.OutOrStdout()`, and S02–S04 add arms that use the same writer.
- `classifyDashArg("--version")` returns `argVersionFlag` with msg `unknownLongFlagMessage("--version")`, matched on the exact string only. `new` and the help stub report that msg exactly as `argUnknownFlag` does, which is what R6/S06 depend on.
- Version text stays in `internal/cli`, with no `internal/version` package. S02's fallback goes in an unexported pure func in `internal/cli` (e.g. `versionLine(readBuildInfo) string`), called from `runRoot`'s `argVersionFlag` arm.

**Left unbuilt:**
- `(devel)` / empty / `ok == false` fallback: S02. Today an empty `Main.Version` prints `brief \n`, and `nil, false` must not be dereferenced; S02 owns the guard.
- Exact-value assertion of Step 10's through-`Run` test (`brief (devel)\n`): S02 tightens it.
- `'--version' takes no arguments` copy for trailing args: S03. S01 keeps today's unknown-flag message there.
- `'--version' takes no value` for `--version=` / `--version=x`: S04. These still classify as `argUnknownFlag`.
- `-v` pinning: S05. Full `new`/`help`/leaf byte-identity tables: S06. Root-help trailer line in `helpTemplate`: S07.

**Traps:**
- `exhaustive` is enabled (`default: all`). All three `classifyDashArg` switches end in a bodyless `case argNotFlag:` with no `default`, so a new kind breaks lint at every site until each has an arm.
- `cli_internal_test.go:28` is a hidden `newRootCommand` caller and stops compiling when the signature changes.
- cobra's `Command.Version` / `InitDefaultVersionFlag` never fire, because root has `DisableFlagParsing: true`. Do not set `root.Version`.
- In a `go test` binary, `Main.Version` is not the module's real version, so black-box tests through `cli.Run` cannot assert a specific version string. Use the `run` seam for exact values.
- Checking `--version` after the `--` prefix branch would send it to `argUnknownFlag`. The exact-match check must come first, and it must not use a `HasPrefix` that swallows `--version=x`.
