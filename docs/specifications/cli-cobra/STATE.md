# cli-cobra — current state

Scenarios complete: SCENARIO-01. Last updated by SCENARIO-01.

## Binding decisions

- Command tree is built per `Run` in `newRootCommand` (`internal/cli/cli.go`); no
  package-level `*cobra.Command` — cobra stores parse state and flag values on the command,
  so a shared tree leaks between `Run` calls. (SCENARIO-01)
- Root and `new` use `DisableFlagParsing` + `Args: cobra.ArbitraryArgs` + `RunE`
  (`runRoot`/`runNew`, unchanged from the pre-cobra code) — this is what keeps `brief bogus`,
  `brief -x`, `brief new widget`, `brief new -x` on brief's one-line errors instead of
  cobra's own dispatch. Every leaf also sets `Args: cobra.ArbitraryArgs`; every `run*`
  function does its own argument-count checking. (SCENARIO-01)
- R14's invocation frame comes from one root `SetFlagErrorFunc`: path =
  `cmd.CommandPath()` minus the `"brief "` prefix, invocation = `cmd.Annotations["invocation"]`
  (set by `leafCommand`'s `invocation` param). pflag's own wording passes through verbatim:
  `unknown flag: --x`, `unknown shorthand flag: 'x' in -x`, `flag needs an argument: --x`.
  S02..S06 assert against this output; do not re-derive it differently per command.
  (SCENARIO-01)
- Help is one root `SetHelpFunc` printing `cmd.Long` verbatim; each `*Usage` constant sits in
  `Long` unchanged. cobra's own help template is never invoked — S07 is what introduces one.
  (SCENARIO-01)
- cobra's default `help` command is replaced via `SetHelpCommand` with a hidden stub
  (`RunE` prints the root `usage` constant, ignoring its topic) — this is not a topic-aware
  help yet, just a swap-in that keeps `os.Exit` out of `internal/cli`. S08/S09 build real
  topic handling on top of it. (SCENARIO-01)
- `Run`'s `root.SetArgs` always receives a **fresh, non-nil copy** of `args`, never `args`
  itself — cobra reads `os.Args[1:]` when `c.args == nil`. Mutation-verified: passing `args`
  directly reddens `Test_returns_a_usage_error_when_args_are_nil` (cobra parses the test
  binary's own `-test.*` flags as brief's argv). (SCENARIO-01)
- Every `RunE` closure gets its `context.Context` via `cmd.Context()`, never as a threaded
  parameter — `newRootCommand` and `leafCommand` have no `ctx` parameter. This is
  `//nolint:contextcheck`'d at the `newRootCommand(...)` call in `Run`; do not remove the
  nolint or "fix" it by threading ctx through `newRootCommand` — `ExecuteContext(ctx)`
  guarantees `cmd.Context()` is correct before any `RunE` runs. (SCENARIO-01)

## Left unbuilt

- Real `help` command (`help <cmd…>` = `<cmd…> --help`, `help bogus` = usage error) — owned
  by S08/S09; today's stub prints root usage for any topic, ignoring it.
- Generated help template / `Short` / flag usage strings — owned by S07; today's help is the
  `*Usage` constant printed as-is, no `Usage:` line or flag table from cobra.
- `new` help listing `new feature`/`new step` — owned by S10.
- Tree-derived `expected one of:` list — owned by S11; the literal in `runRoot` stays a
  hard-coded string until then.
- `completion` command (`CompletionOptions.DisableDefaultCmd` flips back on) — owned by
  S12/S13.

## Traps

- Cobra always registers hidden `__complete`/`__completeNoDescriptions` commands in
  `ExecuteC`, regardless of `CompletionOptions.DisableDefaultCmd` (that option only affects
  the visible `completion` command). S11's derived list must filter `Hidden`
  (`IsAvailableCommand`), not just exclude `completion` by name.
- Cobra's *default* help command (if `SetHelpCommand` were ever removed) calls
  `cobra.CheckErr` → `os.Exit(1)` on an unknown topic — but only when `Command.Find` returns
  a nil command or a non-nil error. Every command in this tree sets `Args:
  cobra.ArbitraryArgs`, so `Find` never errors and never returns nil here; verified by
  deleting `SetHelpCommand` entirely and confirming the suite stayed green (it fell through
  to `cmd.Help()` on the root, which coincidentally reproduces today's text via
  `SetHelpFunc`). The `os.Exit` path is real, confirmed by reading cobra's source, not by a
  reddened test — a future scenario that makes `Find` fallible (e.g. a real `Args` validator
  somewhere) must re-check this before relying on `SetHelpCommand` alone.
- `start_test.go:462` and `:498` compare raw `stderr.String()` with a trailing `\n`; the rest
  of the suite uses the `oneLine` helper (which trims it). Don't "fix" one style into the
  other without checking both are intentional per-test.
- pflag flag values are read with the error discarded (`jsonOut, _ :=
  cmd.Flags().GetBool("json")`) — safe only because the flag is registered on the same
  command immediately above. Any new leaf flag must follow the same
  register-then-read-on-that-command pattern or the discarded error becomes a real bug.
- `-help`/`-json` (single-dash long flags) now parse as pflag shorthand clusters, not long
  flags, and are rejected — approved change (R4), pinned by S04. No SCENARIO-01 test uses
  them, so nothing here proves the exact wording yet.

## Open debts

None — every item under "Left unbuilt" is owned by a named scenario (S07-S13) already
listed in `specification.md`'s BDD Acceptance Progress. Nothing here is unowned.
