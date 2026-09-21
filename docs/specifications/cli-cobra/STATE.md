# cli-cobra — current state

Scenarios complete: SCENARIO-01..06. Last updated by SCENARIO-06.

## Binding decisions

- Command tree rebuilt per `Run` in `newRootCommand`; no package-level `*cobra.Command` —
  cobra stores parse state on the command itself. (SCENARIO-01)
- Root/`new` use `DisableFlagParsing`+`ArbitraryArgs`+`RunE` (`runRoot`/`runNew`); every leaf
  uses `ArbitraryArgs` and does its own arg-count check — keeps unknown command/type on
  brief's own error, not cobra's dispatch. (SCENARIO-01)
- R14 frame is one root `SetFlagErrorFunc` (path = `CommandPath()` minus `"brief "`,
  invocation = `Annotations["invocation"]`); pflag's wording passes through verbatim,
  mutation-verified per moving part by S02–S06 — do not re-derive it per command. (SCENARIO-01..06)
- `flag_error_test.go` holds one flag-parse-error table per scenario (`oneLine` helper,
  literal stderr, never built from the `invocation` constant): S02 long-flag, S03 shorthand
  (+ `-xy`/`-hx` clusters), S04 single-dash-long + `--x` control, S05 missing-value /
  empty-`=` / next-flag-consumed, S06 `--help`/`-h` next to an undefined flag either order
  (14 rows) + `-h`-alone control. (SCENARIO-02..06)
- An undefined flag beats `--help`/`-h` in either order — structural (pflag's `ParseFlags`
  stops at the first bad token before any help check runs), not brief code; no future help
  work may add a pre-dispatch "help wins" scan of args. `-h` rows scoped to `start` only
  (`InitDefaultHelpFlag` identical per leaf); a leaf with its own `-h`/`help` flag must add
  its own rows. (SCENARIO-06)
- Single-dash long flags (`-json`/`-handoff`/`-state`/`-help`) parse as shorthand clusters,
  rejected via R14 (approved, R4). `-help`/`-handoff` share the leading `h` with cobra's auto
  help shorthand, so only the residual cluster is quoted (`'e' in -elp`); `-json`/`-state`
  quote the whole word. No leaf may add shorthand `j`/`s`/`a`/`e`, or a long name that
  defines those letters, without updating S04's rows. (SCENARIO-04)
- Missing-value errors use the R14 frame verbatim. `--handoff=`/`--state=` are **not**
  flag-parse errors (pflag accepts the empty value) — they reach `runFinish`'s own
  `--handoff is required`/`--state is required` guard instead. `MarkFlagRequired` would
  change that wording; S05's two `=` rows must move with it. (SCENARIO-05)
- Help is one root `SetHelpFunc` printing `cmd.Long` verbatim (each `*Usage` constant sits in
  `Long` unchanged; cobra's own template unused until S07). cobra's default `help` command is
  replaced via `SetHelpCommand` with a hidden stub (prints root `usage`, ignores topic) —
  keeps `os.Exit` out of `internal/cli`; S08/S09 add real topic handling. (SCENARIO-01)
- `Run`'s `root.SetArgs` always gets a fresh, non-nil copy of `args` (cobra reads
  `os.Args[1:]` when `c.args == nil`). Every `RunE` reads `context.Context` via
  `cmd.Context()`, never a threaded param (`//nolint:contextcheck`'d at the
  `newRootCommand(...)` call) — `ExecuteContext(ctx)` guarantees it's correct first. (SCENARIO-01)

## Left unbuilt

- Real `help` command (`help <cmd…>` = `<cmd…> --help`, `help bogus` = usage error) — S08/S09.
- Generated help template / `Short` / flag usage strings — S07; today's help is the `*Usage`
  constant printed as-is.
- `new` help listing `new feature`/`new step` — S10.
- Tree-derived `expected one of:` list — S11; `runRoot` keeps a hard-coded literal until then.
- `completion` command (`CompletionOptions.DisableDefaultCmd` flips back on) — S12/S13.
- Assertion that a rejected `finish -handoff`/`-state` writes nothing to disk (S04), or a
  value-taking flag row for a leaf other than `finish` (S05) — unowned.
- `start demo --json=` → Go-internal `strconv.ParseBool` wording reaches the user through R14
  (observed, exit 2). Unowned; flagged for the final product-vision pass. (SCENARIO-05)

## Traps

- Cobra always registers hidden `__complete`/`__completeNoDescriptions` in `ExecuteC`
  regardless of `CompletionOptions.DisableDefaultCmd`. S11's derived list must filter
  `Hidden` (`IsAvailableCommand`), not just exclude `completion` by name.
- Cobra's *default* help command calls `cobra.CheckErr` → `os.Exit(1)` on an unknown topic,
  only when `Command.Find` returns nil/errors — every command here sets `ArbitraryArgs`, so
  `Find` never does (read from cobra's source, not a reddened test). A scenario that makes
  `Find` fallible must re-check this before relying on `SetHelpCommand` alone.
- `start_test.go:462,498` compare raw `stderr.String()` with a trailing `\n`; the rest of the
  suite (incl. `flag_error_test.go`) uses `oneLine` (trims it) — don't unify without checking
  both are intentional.
- pflag flag values are read with the error discarded (e.g. `GetBool("json")`) — safe only
  because the flag is registered on that same command immediately above. Any new leaf flag
  must follow register-then-read-on-that-command.
- `new` is `DisableFlagParsing`; `new --bogus`/`new -x` go through `runNew`, not
  `FlagErrorFunc`, with different wording than `new feature`/`new step`. Don't add
  root/`new`-level rows to the shorthand or long-flag tables. (SCENARIO-02, SCENARIO-03)
- A shorthand cluster with a defined letter consumed first quotes only the **residual**
  cluster (`-hx` -> `in -x`); an all-undefined cluster quotes the whole thing (`-xy` -> `in
  -xy`). `-h` is the only defined shorthand today; a future leaf shorthand only collides with
  S03's probes if it picks `x`/`y`. (SCENARIO-03)
- `finish ... --handoff --state s.md` reports `too many arguments`, not missing-value: pflag
  takes `--state` as `--handoff`'s value, leaving `s.md` a third positional (no trailing
  token instead leaves `statePath` unset, giving `--state is required`). Mutating one flag's
  `NoOptDefVal` reddens every space-separated occurrence of that flag in the suite — expect
  that spillover rather than reading it as a cascade. (SCENARIO-05)
- The obvious "help wins" guard, `if cmd.Flags().Changed("help") { return nil }` inside
  `SetFlagErrorFunc`, reddens only the help-FIRST order (pflag stops at the first bad token,
  so in `--bogus --help` the help flag is never parsed) and spills into S03's `-hx` and S04's
  `-help`/`-handoff` rows, whose leading `h` sets the help flag. Don't reach for this guard to
  detect "help was requested". (SCENARIO-06)

## Open debts

- Coverage duplicated between older SCENARIO-01 tests and `flag_error_test.go`'s tables:
  `run_test.go`'s `..._when_a_flag_is_not_defined` and `new_step_test.go`'s `..._for_step`
  duplicate two S03 shorthand rows; `run_test.go`'s `..._for_the_subcommand` duplicates S04's
  control-arm `--help` row; `start_test.go`'s help/undefined-flag-order pair duplicates S06's
  two `start --help` rows (kept: it asserts the raw trailing `\n` that `oneLine` trims).
  Unowned — leave to the reviewer pass or a later cleanup; dies unless re-opened.
