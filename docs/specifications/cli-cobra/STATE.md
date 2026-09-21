# cli-cobra — current state

Scenarios complete: SCENARIO-01..04. Last updated by SCENARIO-04.

## Binding decisions

- Command tree is built per `Run` in `newRootCommand` (`internal/cli/cli.go`); no
  package-level `*cobra.Command` — cobra stores parse state/flag values on the command
  itself, so a shared tree leaks between `Run` calls. (SCENARIO-01)
- Root and `new` use `DisableFlagParsing` + `Args: cobra.ArbitraryArgs` + `RunE`
  (`runRoot`/`runNew`, unchanged pre-cobra) — keeps `brief bogus`, `brief -x`,
  `brief new widget`, `brief new -x` on brief's one-line errors instead of cobra's own
  dispatch. Every leaf sets `Args: cobra.ArbitraryArgs`; every `run*` does its own
  argument-count checking. (SCENARIO-01)
- R14's invocation frame is one root `SetFlagErrorFunc`: path = `cmd.CommandPath()` minus
  `"brief "`, invocation = `cmd.Annotations["invocation"]` (set by `leafCommand`'s
  `invocation` param). pflag's own wording passes through verbatim: `unknown flag: --bogus`,
  `unknown shorthand flag: 'x' in -x`, `flag needs an argument: --x`. Mutation-verified per
  moving part (S02: path expression, Annotations lookup, `usageError` wrap, `invocation` arg;
  S03: shorthand clusters, one leaf at a time; S04: single-dash spellings are
  letter-sensitive, not generic-failure-sensitive). S05/S06 assert against this same output;
  do not re-derive it per command. (SCENARIO-01..04)
- `flag_error_test.go` (`cli_test` package) holds one flag-parse-error table per scenario,
  using `run_test.go`'s `oneLine` helper; expected stderr is always a literal, never built
  from a production `invocation` constant. Tables so far: S02 long-flag, S03 shorthand
  (`new feature`/`start`/`finish`/`status`/`check`/`new step` singles + `-xy`/`-hx` clusters
  on `new feature`), S04 single-dash-long-flag rejection (9 rows) + its `--x` control arm (8
  rows, fixture built inside each control subtest — `finish` mutates it). S05/S06 add their
  own tables here too. (SCENARIO-02..04)
- Single-dash long flags (`-json`, `-handoff`, `-state`, `-help`) parse as pflag shorthand
  clusters and are rejected through R14 — approved change (R4), S04. `-help`/`-handoff` start
  with `h` (cobra's auto help shorthand): pflag consumes it first, reports only the residual
  cluster (`'e' in -elp`, `'a' in -andoff`). `-json`/`-state` have no defined first letter, so
  the whole word is quoted. No leaf may add shorthand `j`/`s`/`a`/`e`, or a long name that
  defines those letters, without updating S04's rows. (SCENARIO-04)
- Help is one root `SetHelpFunc` printing `cmd.Long` verbatim; each `*Usage` constant sits in
  `Long` unchanged. cobra's own help template is never invoked — S07 introduces one.
  (SCENARIO-01)
- cobra's default `help` command is replaced via `SetHelpCommand` with a hidden stub
  (prints root `usage`, ignores topic) — not topic-aware yet; keeps `os.Exit` out of
  `internal/cli`. S08/S09 build real topic handling on it. (SCENARIO-01)
- `Run`'s `root.SetArgs` always gets a **fresh, non-nil copy** of `args` — cobra reads
  `os.Args[1:]` when `c.args == nil`. (SCENARIO-01)
- Every `RunE` closure reads `context.Context` via `cmd.Context()`, never a threaded param —
  `//nolint:contextcheck`'d at the `newRootCommand(...)` call in `Run`. `ExecuteContext(ctx)`
  guarantees `cmd.Context()` is correct before any `RunE` runs. (SCENARIO-01)

## Left unbuilt

- Real `help` command (`help <cmd…>` = `<cmd…> --help`, `help bogus` = usage error) — S08/S09.
- Generated help template / `Short` / flag usage strings — S07; today's help is the `*Usage`
  constant printed as-is.
- `new` help listing `new feature`/`new step` — S10.
- Tree-derived `expected one of:` list — S11; `runRoot` keeps a hard-coded literal until then.
- `completion` command (`CompletionOptions.DisableDefaultCmd` flips back on) — S12/S13.
- Missing-value flag-error table (`flag needs an argument: --handoff`) — S05.
- `--help` next to an undefined flag, either order — S06.
- Assertion that a rejected `finish -handoff`/`-state` writes nothing to disk — not in S04's
  contract; rejection happens in flag parsing before `runFinish`, no test pins it.

## Traps

- Cobra always registers hidden `__complete`/`__completeNoDescriptions` in `ExecuteC`
  regardless of `CompletionOptions.DisableDefaultCmd` (only hides the visible `completion`
  command). S11's derived list must filter `Hidden` (`IsAvailableCommand`), not just exclude
  `completion` by name.
- Cobra's *default* help command calls `cobra.CheckErr` → `os.Exit(1)` on an unknown topic,
  but only when `Command.Find` returns nil or errors — every command here sets
  `Args: cobra.ArbitraryArgs`, so `Find` never does, confirmed by reading cobra's source
  (not a reddened test). A scenario that makes `Find` fallible must re-check this before
  relying on `SetHelpCommand` alone.
- `start_test.go:462,498` compare raw `stderr.String()` with a trailing `\n`; the rest of the
  suite (including `flag_error_test.go`) uses `oneLine` (trims it) — don't unify the styles
  without checking both are intentional.
- pflag flag values are read with the error discarded (e.g. `cmd.Flags().GetBool("json")`) —
  safe only because the flag is registered on the same command immediately above. Any new
  leaf flag must follow register-then-read-on-that-command or the discarded error becomes real.
- `new` is `DisableFlagParsing`; `new --bogus`/`new -x` go through `runNew`, not
  `FlagErrorFunc`, with different wording than `new feature`/`new step`. Don't add
  root/`new`-level rows to the shorthand or long-flag tables. (SCENARIO-02, SCENARIO-03)
- A shorthand cluster with a defined letter consumed first quotes only the **residual**
  cluster (`-hx` -> `in -x`); an all-undefined cluster quotes the whole thing (`-xy` ->
  `in -xy`). `-h` is presently the only defined shorthand anywhere in the tree; a future leaf
  shorthand only collides with S03's `x`/`y` probes if it picks those letters. (SCENARIO-03)

## Open debts

- `run_test.go`'s `Test_returns_a_usage_error_when_a_flag_is_not_defined` (`new feature -x p`)
  and `new_step_test.go`'s `..._for_step` (`new step -x p`) duplicate two rows of S03's
  shorthand table in `flag_error_test.go`. Deliberately not deleted — they also carry
  SCENARIO-01 coverage. Unowned — leave to the reviewer pass or a later cleanup; dies unless
  re-opened.
