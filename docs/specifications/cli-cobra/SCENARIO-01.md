---
id: SCENARIO-01
status: done
---

# SCENARIO-01: Existing command contract holds on cobra

## Scenario

```gherkin
Scenario: SCENARIO-01 Existing command contract holds on cobra
  Given the existing internal/cli suite, minus the 5 stdlib-wording assertions and the --help ordering pair
  When brief dispatches through cobra
  Then every one of those tests passes unedited, including "" names, "-" stdin values and flags in any order
```

## Scope decision: the six wording/ordering tests are updated here, not left red

`go test ./...` must be green at scenario end, and pflag's wording is fixed by the approved
business rules, so those six assertions are rewritten to the exact target strings from
SCENARIO-02/03/05/06 **before** the port. That makes them this scenario's red step (they fail
under urfave). SCENARIO-02..06 then add their own dedicated tests; they do not re-edit these.

Target strings (note two assert raw `stderr.String()` **with the trailing `\n`**, not `oneLine`):
- `run_test.go:163` (`oneLine`) — `brief new feature: unknown shorthand flag: 'x' in -x; run 'brief new feature <name>'`
- `new_step_test.go:60` (`oneLine`) — `brief new step: unknown shorthand flag: 'x' in -x; run 'brief new step <feature>'`
- `finish_test.go:454` (`oneLine`) — `brief finish: unknown flag: --bogus; run 'brief finish <feature> <step> --handoff <path> --state <path>'`
- `start_test.go:462` and `:498` (raw, trailing `\n`) — `brief start: unknown flag: --bogus; run 'brief start <feature>'\n`
- `start_test.go:479-489` flips: `start --help --bogus` becomes `ErrUsage`, empty stdout, the same stderr line as `:498`

User-visible contract after this scenario (unchanged except the above): stdout carries help and
command output only; every usage error is one stderr line; exit 0 ok / 1 refusal / 2 usage via
`cli.ExitCode` only. `cmd/brief/main.go` is **not touched** — `Run` and `ExitCode` signatures
are unchanged.

## Implementation Plan

- [x] Step 1: `internal/cli/run_test.go`, `new_step_test.go`, `finish_test.go`, `start_test.go` — rewrite the 5 stdlib-wording assertions to the pflag strings above (red)
- [x] Step 2: `internal/cli/start_test.go` — rename `Test_prints_the_start_usage_when_help_precedes_an_undefined_flag` to a usage-error name, flip its assertions, and rewrite the comment above the pair (it currently claims "the first of --help and an undefined flag decides") to state that an undefined flag is a usage error in either order (red)
- [x] Step 3: `internal/cli/run_test.go` `Test_returns_a_usage_error_when_args_are_nil` — `cli.Run` with a nil args slice yields the no-command usage error (characterization; green on arrival under urfave, guards the cobra `os.Args` fallback)
- [x] Step 4: `internal/cli/run_test.go` — characterization tests for dispatch the port could silently change: `brief help` prints root usage to stdout (nil error); `brief help bogus` prints root usage with nil error (pins today's behavior; SCENARIO-09 rewrites this assertion); `brief -x` is `unknown command "-x"`; `brief new -x` is `unknown type "-x"` (green on arrival)
- [x] Step 5: `go.mod` / `go.sum` — `go get github.com/spf13/cobra@v1.10.2` (Bash needs `proxy.golang.org` and `sum.golang.org` in `allowed_domains`) (new)
- [x] Step 6: `internal/cli/cli.go` `newRootCommand` — rebuild the tree on `*cobra.Command`, constructed per `Run` (no package-level tree): root and `new` each get `DisableFlagParsing`, `Args: cobra.ArbitraryArgs` and a `RunE` delegating to the existing `runRoot` / `runNew`; root sets `SilenceErrors`, `SilenceUsage`, `DisableSuggestions`, `CompletionOptions.DisableDefaultCmd` (green)
- [x] Step 7: `internal/cli/cli.go` `leafCommand` — leaves carry their `*Usage` constant verbatim in `Long`, `Args: cobra.ArbitraryArgs`, their flags on `cmd.Flags()`, and the R14 invocation string in `Annotations`; `RunE` reads flag values and calls the unchanged `run*` functions (green)
- [x] Step 8: `internal/cli/cli.go` — one root-level `SetFlagErrorFunc` (inherited) wrapping the pflag error into `usageError` as `brief <path>: <pflag message>; run '<invocation>'`, path from `CommandPath()` minus the leading `brief`, invocation from the annotation (green)
- [x] Step 9: `internal/cli/cli.go` — one root-level `SetHelpFunc` (inherited) printing `cmd.Long` verbatim to stdout; root's `Long` is the existing `usage` constant (green)
- [x] Step 10: `internal/cli/cli.go` — `SetHelpCommand` with a hidden `help` stub (`DisableFlagParsing`, `ArbitraryArgs`, `RunE` printing root usage, exactly today's `runRoot` "help" behavior) so cobra's default help command, and its `os.Exit` path, is never installed (green)
- [x] Step 11: `internal/cli/cli.go` `Run` — `SetArgs` with a non-nil copy of `args`, `SetIn`/`SetOut`/`SetErr` to the passed streams, return `ExecuteContext(ctx)`'s error unwrapped-as-is (green)
- [x] Step 12: `internal/cli/cli.go` — delete `isHelpRequest` and the urfave import; rewrite `newRootCommand`/`leafCommand` doc comments as the current contract (no migration narrative) (update)
- [x] Step 13: `go mod tidy` — urfave/cli/v3 gone from `go.mod` and `go.sum` (update)
- [x] Step 14: `internal/cli/doc.go` — replace the urfave paragraph (lines 6-13) with the cobra contract: brief owns help text, flag-error copy and exit codes; cobra never prints or exits (update)
- [x] Step 15: `.golangci.yaml` — delete the `cli/v3.Command).Run(` ignore-sig and its comment block (lines 56-59); run `golangci-lint run ./...` and add `cobra.Command).ExecuteContext(` with a one-line rationale only if wrapcheck flags it (update; also added a `//nolint:contextcheck` on `Run`'s `newRootCommand` call — contextcheck flags `cmd.Context()`'s internal `context.Background()` fallback, which it cannot see is always overwritten by `ExecuteContext(ctx)` before any `RunE` runs; not anticipated by the plan, documented at the call site)
- [x] Step 16: `docs/adr/002-adopt-cobra.md` — new ADR: cobra v1.10.2 replaces urfave/cli v3, pflag GNU syntax accepted, feature-name completion (`ValidArgsFunction`) named as follow-up; `docs/adr/001-adopt-urfave-cli-v3.md` — status line only → `Superseded by ADR-002` (new / update)
- [x] Step 17: verify from repo root, unpiped: `go build ./...`, `go test ./...`, `go test -race ./internal/cli/...`, `golangci-lint run ./...`; report test count and delta (+ the Step 3/4 tests, 6 edited, 0 removed); mutation-verify Step 3 by dropping the non-nil copy in `SetArgs` (expect the nil-args test red); do not claim a mutation proof for Step 10 unless one actually reddens (with root on `ArbitraryArgs`, cobra's default help may print the same root usage, so removing the stub may stay green; report what happened) → mark SCENARIO-01 done in specification.md

**Step 17 results:**
- `go build ./...`: exit 0. `go test ./...`: exit 0, all 8 packages `ok`, 0 skips. `go test -race ./internal/cli/...`: exit 0. `golangci-lint run ./...`: `0 issues`.
- `internal/cli` test count: 111 baseline (measured against the pre-scenario tree via a stash/apply/drop round trip) → 116 now. Delta +5 = the 5 new tests from Steps 3-4 (`Test_returns_a_usage_error_when_args_are_nil`, `Test_prints_root_usage_and_a_nil_error_for_brief_help`, `Test_prints_root_usage_and_a_nil_error_for_brief_help_with_an_unknown_topic`, `Test_returns_a_usage_error_when_the_root_command_is_a_single_dash_flag`, `Test_returns_a_usage_error_when_the_new_type_is_a_single_dash_flag`). 6 edited in place (Step 1's five plus Step 2's rename), 0 removed.
- Step 3 mutation: replaced `root.SetArgs(argsCopy)` (a fresh non-nil copy) with `root.SetArgs(args)` (the raw, possibly-nil param). `Test_returns_a_usage_error_when_args_are_nil` went red: cobra's `c.args == nil` fallback read the test binary's own flags (`-test.testlogfile=...`) as `brief`'s argv, producing `unknown command "-test.testlogfile=..."` instead of the expected no-command usage error. Reverted; `diff` against a pre-mutation copy confirmed byte-identical restoration.
- Step 10 mutation: deleted the `root.SetHelpCommand(...)` block entirely (cobra installs its own default help command instead). `go test ./internal/cli/...` **stayed green** — no test reddened. Reason found by inspection: every command in the tree sets `Args: cobra.ArbitraryArgs`, so `Command.Find` never returns a non-nil error and never resolves to a nil command; cobra's default help `Run` only reaches `CheckErr` (the `os.Exit(1)` path) when `Find` errors or returns nil, which cannot happen here. For "help bogus", the default help command instead falls into its `else` branch, calls `cmd.Help()` on the command `Find` resolved (root, since "bogus" isn't a subcommand), which drives our still-registered `SetHelpFunc` and prints the same root usage — reproducing today's pinned behavior by a different path, coincidentally, not by proof of the stub being load-bearing. The `os.Exit(1)` path is real (verified by reading cobra's source, not by a reddened test) but no scenario-01 test can reach it; SCENARIO-09 is where a test exercising a genuine unknown-topic-with-Find-error case would be added. Reverted; `diff` confirmed byte-identical restoration.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- The six old-wording/ordering tests were rewritten here to the exact SCENARIO-02/03/05/06 strings — S02..S06 add dedicated tests; they do not re-edit these, and a mismatch between them is a bug.
- Command tree is built per `Run` in `newRootCommand`; no package-level `*cobra.Command` — cobra stores parse state and flag values on the command, so a shared tree leaks between Runs.
- Root and `new` use `DisableFlagParsing` + `Args: cobra.ArbitraryArgs` + `RunE` (`runRoot`/`runNew`) — this is what keeps `brief bogus`, `brief -x`, `brief new widget`, `brief new -x` on brief's one-line errors instead of `legacyArgs` / `flag.ErrHelp`. S10 (new help) and S11 (derived list) build on this shape.
- R14 frame comes from one root `SetFlagErrorFunc` using `CommandPath()` + the leaf's invocation annotation — S02..S06 assert against its output.
- Help is one root `SetHelpFunc` printing `cmd.Long`; each `*Usage` constant sits verbatim in `Long` — S07 swaps the help func/template and splits flag prose out, it does not relocate content.
- `root.SetArgs` always receives a non-nil slice — cobra reads `os.Args[1:]` when `c.args == nil`.
- `SilenceErrors`, `SilenceUsage`, `DisableSuggestions` on root; nothing in `internal/cli` calls `os.Exit` or `cobra.CheckErr`.

**Left unbuilt** — named so nobody assumes it exists:
- `isHelpRequest` — deleted, not ported.
- Real `help` command (`help <cmd…>` = `<cmd…> --help`, `help bogus` usage error) — S08/S09; today's stub prints root usage for any topic.
- Generated help template / `Short` / flag usage strings — S07.
- `new` help listing `new feature`/`new step` — S10.
- Tree-derived `expected one of:` list — S11; the literal in `runRoot` stays until then.
- `completion` command (`CompletionOptions.DisableDefaultCmd` flips) — S12/S13.

**Traps** — things that look right and are not:
- `SetArgs(nil)` makes cobra parse the test binary's own flags — only the nil-args test catches it.
- Cobra's default `help` command can reach `CheckErr` → `os.Exit(1)` (`cobra.go:238`) on an unknown topic; there is no "disable" switch — it must be replaced via `SetHelpCommand`. The `help bogus` characterization test may stay green without the stub (root `ArbitraryArgs` lets `Find` resolve to root), so it is not proof the stub is load-bearing.
- Cobra always adds hidden `__complete`/`__completeNoDesc` commands in `ExecuteC`; S11's derived list must filter `Hidden` (and `IsAvailableCommand`), not just exclude `completion`.
- `start_test.go:462/:498` compare raw stderr with trailing `\n`; the others use `oneLine`.
- A non-runnable `new` returns `flag.ErrHelp` → help on stdout with nil error: silently wrong exit 0 for `brief new widget`.
- `-help`/`-json` single-dash now parse as pflag shorthand clusters, not long flags (exact wording unverified here) — approved change; S04 pins it. No existing test uses them.
