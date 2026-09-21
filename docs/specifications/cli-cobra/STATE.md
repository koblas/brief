# cli-cobra — current state

Scenarios complete: SCENARIO-01..05. Last updated by SCENARIO-05.

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
  `"brief "`, invocation = `cmd.Annotations["invocation"]`. pflag's own wording passes
  through verbatim, mutation-verified per moving part by S02–S05. S06 asserts against this
  same output; do not re-derive it per command. (SCENARIO-01..05)
- `flag_error_test.go` (`cli_test` package) holds one flag-parse-error table per scenario,
  using `run_test.go`'s `oneLine` helper; expected stderr is always a literal, never built
  from a production `invocation` constant. Tables so far: S02 long-flag, S03 shorthand
  (per-leaf singles + `-xy`/`-hx` clusters), S04 single-dash-long-flag rejection (9 rows) +
  `--x` control arm (8 rows, `finish` mutates its own fixture), S05 missing-value (4 rows),
  empty-`=`-value (2 rows), next-flag-consumed (2 rows). (SCENARIO-02..05)
- Single-dash long flags (`-json`, `-handoff`, `-state`, `-help`) parse as pflag shorthand
  clusters and are rejected through R14 — approved change (R4), S04. `-help`/`-handoff` start
  with `h` (cobra's auto help shorthand): pflag consumes it first, reports only the residual
  cluster (`'e' in -elp`, `'a' in -andoff`). `-json`/`-state` have no defined first letter, so
  the whole word is quoted. No leaf may add shorthand `j`/`s`/`a`/`e`, or a long name that
  defines those letters, without updating S04's rows. (SCENARIO-04)
- Missing-value errors (`flag needs an argument: --handoff`) use the R14 frame verbatim.
  `--handoff=`/`--state=` are **not** flag-parse errors — pflag accepts the empty value, so
  they reach `runFinish`'s own `--handoff is required`/`--state is required` guard instead.
  Moving required-flag checking into cobra (`MarkFlagRequired`) changes that wording and
  must update S05's two `=` rows. (SCENARIO-05)
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
- `--help` next to an undefined flag, either order — S06.
- Assertion that a rejected `finish -handoff`/`-state` writes nothing to disk (S04) or a
  value-taking flag row for a leaf other than `finish` (S05) — neither exists today; no
  scenario owns either.
- `start demo --json=` → Go-internal `strconv.ParseBool` wording reaches the user through
  R14 (observed, exit 2). No scenario covers it; flagged for the final product-vision pass.
  (SCENARIO-05)

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
- `finish demo SCENARIO-01 --handoff --state s.md` reports `too many arguments`, not a
  missing-value error: pflag takes `--state` as `--handoff`'s value, leaving `s.md` a third
  positional. With no trailing token, `statePath` is left unset and the result is `--state
  is required` instead. pflag's `parseLongArg` takes `NoOptDefVal` before "consume the next
  token", and only `--flag=value` bypasses it — mutating one flag's `NoOptDefVal` therefore
  reddens every space-separated occurrence of that flag in the suite, including rows using
  it only as unrelated setup; expect that spillover in a future mutation-verification of a
  value-taking flag rather than reading it as a cascade. (SCENARIO-05)

## Open debts

- Coverage duplicated between older SCENARIO-01 tests and `flag_error_test.go`'s tables,
  kept because each old test also carries SCENARIO-01 coverage the shared table doesn't:
  `run_test.go`'s `..._when_a_flag_is_not_defined` (`new feature -x p`) and
  `new_step_test.go`'s `..._for_step` (`new step -x p`) duplicate two S03 shorthand rows;
  `run_test.go`'s `..._for_the_subcommand` (`new feature --help`) duplicates S04's control-arm
  `--help` row (nil error/empty stderr — a subset of what it already asserts). Unowned — leave
  to the reviewer pass or a later cleanup; dies unless re-opened.
