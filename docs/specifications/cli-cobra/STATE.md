# cli-cobra — current state

Scenarios complete: SCENARIO-01..03. Last updated by SCENARIO-03.

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
  `unknown flag: --bogus`, `unknown shorthand flag: 'x' in -x`, `flag needs an argument: --x`.
  S02 mutation-verified all four moving parts of this frame independently: the path
  expression, the per-command `Annotations` lookup, the `usageError(...)` wrap, and one
  leaf's `invocation` argument. S03 mutation-verified the same frame holds for shorthand
  clusters (registering a colliding `x` shorthand on `new feature` alone reddened exactly
  that leaf's three shorthand rows, not the other five leaves). S04..S06 assert against this
  same output; do not re-derive it differently per command. (SCENARIO-01, SCENARIO-02,
  SCENARIO-03)
- `flag_error_test.go` (`cli_test` package) is the home for flag-parse error tables, one
  table per scenario, using the `oneLine` helper from `run_test.go`. Each row's expected
  stderr is a literal string, never built from a production `invocation` constant. S02 added
  the long-flag table; S03 added the shorthand table (`new feature`, `start`, `finish`,
  `status`, `check`, `new step` single-shorthand rows plus `-xy`/`-hx` cluster rows on `new
  feature`). S04/S05 add their own tables (or rows) there rather than scattering assertions
  per command file. (SCENARIO-02, SCENARIO-03)
- Help is one root `SetHelpFunc` printing `cmd.Long` verbatim; each `*Usage` constant sits in
  `Long` unchanged. cobra's own help template is never invoked — S07 is what introduces one.
  (SCENARIO-01)
- cobra's default `help` command is replaced via `SetHelpCommand` with a hidden stub
  (`RunE` prints the root `usage` constant, ignoring its topic) — this is not a topic-aware
  help yet, just a swap-in that keeps `os.Exit` out of `internal/cli`. S08/S09 build real
  topic handling on top of it. (SCENARIO-01)
- `Run`'s `root.SetArgs` always receives a **fresh, non-nil copy** of `args`, never `args`
  itself — cobra reads `os.Args[1:]` when `c.args == nil`. (SCENARIO-01)
- Every `RunE` closure gets its `context.Context` via `cmd.Context()`, never as a threaded
  parameter — `//nolint:contextcheck`'d at the `newRootCommand(...)` call in `Run`.
  `ExecuteContext(ctx)` guarantees `cmd.Context()` is correct before any `RunE` runs.
  (SCENARIO-01)

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
- Single-dash long flag (`-json`, `-help`) and missing-value (`flag needs an argument: --x`)
  flag-error tables — owned by S04 and S05 respectively, added to `flag_error_test.go`
  alongside S02's long-flag and S03's shorthand tables.

## Traps

- Cobra always registers hidden `__complete`/`__completeNoDescriptions` commands in
  `ExecuteC`, regardless of `CompletionOptions.DisableDefaultCmd` (that option only affects
  the visible `completion` command). S11's derived list must filter `Hidden`
  (`IsAvailableCommand`), not just exclude `completion` by name.
- Cobra's *default* help command (if `SetHelpCommand` were ever removed) calls
  `cobra.CheckErr` → `os.Exit(1)` on an unknown topic — but only when `Command.Find` returns
  a nil command or a non-nil error. Every command in this tree sets `Args:
  cobra.ArbitraryArgs`, so `Find` never errors and never returns nil here; verified by
  deleting `SetHelpCommand` entirely and confirming the suite stayed green. The `os.Exit`
  path is real, confirmed by reading cobra's source, not by a reddened test — a future
  scenario that makes `Find` fallible must re-check this before relying on `SetHelpCommand`
  alone.
- `start_test.go:462` and `:498` compare raw `stderr.String()` with a trailing `\n`; the rest
  of the suite (including `flag_error_test.go`) uses the `oneLine` helper (which trims it).
  Don't "fix" one style into the other without checking both are intentional per-test.
- pflag flag values are read with the error discarded (`jsonOut, _ :=
  cmd.Flags().GetBool("json")`) — safe only because the flag is registered on the same
  command immediately above. Any new leaf flag must follow the same
  register-then-read-on-that-command pattern or the discarded error becomes a real bug.
- `-help`/`-json` (single-dash long flags) now parse as pflag shorthand clusters, not long
  flags, and are rejected — approved change (R4), owned by S04. No test yet proves the exact
  wording.
- `new` is `DisableFlagParsing`; `new --bogus` / `new -x` are handled by `runNew`, not the
  `FlagErrorFunc`, and have a different message than the `new feature`/`new step` leaves,
  which do go through the leaf `FlagErrorFunc` frame. Do not add root/`new`-level rows to the
  shorthand or long-flag tables. (SCENARIO-02, SCENARIO-03)
- A shorthand cluster with a defined letter consumed first quotes only the **residual**
  cluster, not the whole one: `-hx` (defined `-h`, undefined `x`) reports `in -x`, while an
  all-undefined cluster `-xy` reports the whole thing, `in -xy`. Writing the residual case by
  analogy with the all-undefined case gets the quoted text wrong. `-h` is presently the only
  defined shorthand anywhere in the tree (cobra's auto help flag); a future leaf shorthand
  only collides with S03's `x`/`y` probe letters if it picks those same letters. (SCENARIO-03)

## Open debts

- `run_test.go`'s `Test_returns_a_usage_error_when_a_flag_is_not_defined` (`new feature -x
  p`) and `new_step_test.go`'s `Test_returns_a_usage_error_when_a_flag_is_not_defined_for_step`
  (`new step -x p`) now duplicate two rows of S03's shorthand table in
  `flag_error_test.go`. Deliberately not deleted — removing them would also remove
  SCENARIO-01 coverage they carry. Unowned — leave to the reviewer pass or a later cleanup;
  dies unless re-opened.
