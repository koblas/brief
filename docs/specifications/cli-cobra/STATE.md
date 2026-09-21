# cli-cobra — current state

Scenarios complete: SCENARIO-01..10. Last updated by SCENARIO-10.

## Binding decisions

- Command tree rebuilt per `Run` in `newRootCommand`; no package-level `*cobra.Command` —
  cobra stores parse state on the command itself. Root/`new` use
  `DisableFlagParsing`+`ArbitraryArgs`+`RunE`; every leaf uses `ArbitraryArgs` and does its
  own arg-count check. (SCENARIO-01)
- R14 frame is one root `SetFlagErrorFunc` (path = `CommandPath()` minus `"brief "`,
  invocation = `Annotations["invocation"]`); pflag's wording passes through verbatim. An
  undefined flag beats `--help`/`-h` in either order — structural, no pre-dispatch scan.
  Single-dash long flags are shorthand clusters, rejected via R14; `--handoff=`/`--state=`
  reach `runFinish`'s own required-flag guard instead. (SCENARIO-01..06)
- Help renders through one `helpTemplate` set on root via `SetHelpTemplate` (never
  `SetHelpFunc`). Two shapes: a `cmdList` group body for any command with available
  subcommands (root, `new` — S11/S12 extend the same body), selected by
  `HasAvailableSubCommands` not `HasParent`, and a leaf body for the rest. Group trailer is
  `Run '{{.CommandPath}} <command> --help' for details.` A hidden `help` stub (replaces
  cobra's default, keeping `os.Exit` out of `internal/cli`) resolves via
  `cmd.Root().Find(args)`, calls `target.InitDefaultHelpFlag()`, then `target.Help()` —
  byte-identical to `<path…> --help` for every command. `rootHelp`/`startHelp`/`newHelp`
  goldens guard it. Topic accepted iff `Find`'s residual is empty AND (target is root OR
  `IsAvailableCommand()`); else `brief help: unknown command "<topic>"; expected one of:
  <list>`, exit 2, `<topic>` every arg **as typed** joined by spaces. `expectedCommands` in
  `cli.go` — S11 replaces it with a tree derivation; all call sites move together.
  (SCENARIO-01, 07, 08, 09, 10)
- `new` keeps `DisableFlagParsing` and owns its `-h`/`--help` routing inside `runNew`, as the
  **sole** argument only — S06/S09 strictness; run_test.go:281 (`new -x`) depends on flag
  parsing staying off. `newCmd` has no `invocation` annotation; R14 never fires for it.
  `new`'s own `UseLine` renders nowhere, so it needs neither `DisableFlagsInUseLine` nor
  `Short`. (SCENARIO-02, 03, 10)

## Left unbuilt

- Tree-derived `expected one of:` list — S11. `completion` command (`DisableDefaultCmd`
  flips back on) — S12/S13; if S12 wants `help completion` to render despite `Hidden: true`
  it must carve that out of S09's hidden-target rule explicitly, and `completion` must stay
  out of the list.
- Per-level help errors for `new` — deliberately not built (S09's one format string covers
  every topic; `brief new help` stays `unknown type "help"`, not a help alias); nor is a
  `new` row in `Test_every_command_help_has_a_usage_line_and_a_flag_table` — its group shape
  has no `Usage:` prefix or Flags table.
- Unowned, flagged for the final product-vision pass: `<command>` vs `<type>` wording between
  `new`'s errors and the group trailer; `start demo --json=`'s Go-internal `strconv.ParseBool`
  wording reaching the user through R14 (S05); a rejected `finish -handoff`/`-state` writing
  nothing to disk unasserted (S04); no value-taking flag row for a leaf other than `finish`
  (S05).

## Traps

- Cobra always registers hidden `__complete`/`__completeNoDescriptions`, and `help` is a real
  hidden child too — filter listings by `IsAvailableCommand`, not by name. `Find`'s residual
  is unstripped and `Find` never errors on this `ArbitraryArgs`-everywhere tree — the help
  stub keys off `len(residual) > 0`, never a `Find` error. `runRoot`'s `case "help"` is
  unreachable from argv (`Find(["help"])` always returns the hidden stub first) — don't
  delete it on the strength of `rootHelp` alone.
- `new` is `DisableFlagParsing`; `new --bogus`/`-x`/`--help` all go through `runNew`, not
  `FlagErrorFunc` or cobra's help check. `new --help feature` does NOT reach `feature`'s
  help: at `Find` time `new` has no `help` flag registered yet, so `stripFlags` takes
  `feature` as `--help`'s value, and dispatch stays on `new`. Only `new feature --help`
  reaches it. (SCENARIO-02, 03, 10)
- `helpTemplate`'s `define` blocks encode newlines via `-}}`/`{{-` trim markers; adding or
  moving a `define`, or changing which branch a command falls into, shifts whitespace
  silently — prove `rootHelp`/`startHelp`/`newHelp` byte-identical after any template edit.
  `finish ... --handoff --state s.md` reports `too many arguments`: pflag takes `--state` as
  `--handoff`'s value; a `Changed("help")` guard in `SetFlagErrorFunc` would spill into
  S03/S04's rows — don't add one. (SCENARIO-05, 06, 07, 10)
- With sorting off, a tree-derived list comes in registration order `new, start, status,
  check, finish` — today's `expectedCommands` literal is `new, start, finish, status, check`;
  S11 must pick an order, and reordering `root.AddCommand` also moves root help and S09's
  stderr literals. `EnableCommandSorting` is a cobra package global, set once at init. pflag
  `FlagUsages()` re-indents a usage string's embedded newlines to the description column —
  golden indentation is pflag's, not the constant's; test literals for user-facing
  stderr/stdout are the contract, never built from a const or `fmt.Sprintf`. (SCENARIO-07, 09)

## Open debts

- Coverage duplicated between older SCENARIO-01 tests and `flag_error_test.go`'s tables (two
  S03 shorthand rows; S04's control-arm `--help` row; S06's `start_test.go:462,498` rows,
  kept because they assert the raw trailing `\n` `oneLine` trims). Unowned — dies unless
  re-opened.
