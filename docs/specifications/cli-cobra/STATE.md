# cli-cobra — current state

Scenarios complete: SCENARIO-01..12. Last updated by SCENARIO-12.

## Binding decisions

- Command tree rebuilt per `Run` in `newRootCommand`; no package-level `*cobra.Command` —
  cobra stores parse state on the command itself. Root/`new` use
  `DisableFlagParsing`+`ArbitraryArgs`+`RunE`; every leaf uses `ArbitraryArgs` and does its
  own arg-count check. (SCENARIO-01)
- R14 frame is one root `SetFlagErrorFunc` (path = `CommandPath()` minus `"brief "`,
  invocation = `Annotations["invocation"]`); pflag's wording passes through verbatim. An
  undefined flag beats `--help`/`-h` in either order — structural, no pre-dispatch scan.
  Single-dash long flags are shorthand clusters, rejected via R14; `--handoff=`/`--state=`
  reach `runFinish`'s own required-flag guard instead. (SCENARIO-01..06, 12)
- Help renders through one `helpTemplate` (now a `var`, built with `fmt.Sprintf` so the
  `listedInHelpAnnotation` constant and the template's `index .Annotations %q` stay in
  sync) set on root via `SetHelpTemplate` (never `SetHelpFunc`). Two shapes: a `cmdList`
  group body for any command with available subcommands (root, `new`), and a leaf body for
  the rest. The `cmdList` outer row loop admits a command when `.IsAvailableCommand` OR it
  carries `listedInHelpAnnotation` — the one place besides the help stub that reads that
  annotation. A hidden `help` stub (replaces cobra's default, keeping `os.Exit` out of
  `internal/cli`) resolves via `cmd.Root().Find(args)`, calls `target.InitDefaultHelpFlag()`,
  then `target.Help()` — byte-identical to `<path…> --help` for every command. Topic
  accepted iff `Find`'s residual is empty AND (target is root, OR `IsAvailableCommand()`, OR
  carries `listedInHelpAnnotation`); else `brief help: unknown command "<topic>"; expected
  one of: <list>`, exit 2. `rootHelp`/`startHelp`/`newHelp` goldens guard the template; all
  three stayed byte-identical through the SCENARIO-12 template edit. (SCENARIO-01, 07, 08,
  09, 10, 12)
- R7: `expectedCommandList(cmd)` (`cli.go`) is the single source for every "expected one of:"
  list — `cmd.Root().Commands()` filtered by `IsAvailableCommand`, names joined `", "`, in
  registration order. `completion` is `Hidden` so it never appears here even though it is
  dispatchable — R7's filter stays `IsAvailableCommand`-only, never a name check.
  (SCENARIO-11, 12)
- Root commands register `new, start, finish, status, check, completion` — one order drives
  both root help and every "expected one of:" list; `completion` registers **last** and is
  `Hidden` with `listedInHelpAnnotation` set, so it gets a root-help row (the wrapped/finish
  shape — its UseLine is 43 cols) and a `brief help completion` topic without ever joining
  an "expected one of:" list. `EnableCommandSorting` stays false. (SCENARIO-07, 11, 12)
- `new` keeps `DisableFlagParsing` and owns its `-h`/`--help` routing inside `runNew`, as the
  **sole** argument only. `new`'s children carry no `listedInHelpAnnotation`; `newHelp` is
  unaffected by that annotation's introduction. (SCENARIO-02, 03, 10)
- `completion` (`internal/cli/completion.go`): brief's own leaf, not cobra's default —
  `CompletionOptions.DisableDefaultCmd` stays `true`. Dispatch is a small ordered
  `[]completionShell{name, gen}` table (bash, zsh, fish, powershell); any shell-list message
  (arg-count errors' invocation string aside) derives from it. `runCompletion` generates
  against `cmd.Root()`, never the leaf, so the script names "brief". Arg-count errors reuse
  R2's `start` shapes (`no shell given` / `too many arguments`). An unmatched single shell
  name is SCENARIO-13's to pin; the miss branch is already implemented. (SCENARIO-12)

## Left unbuilt

- `new`'s type list in `new.go` (`expected one of: feature, step`, lines ~36/43) stays a
  literal — it is a type list, not R7's command list. Unowned.
- Per-level help errors for `new` — deliberately not built; `brief new help` stays `unknown
  type "help"`, not a help alias.
- A test pinning `brief completion tcsh` →
  `brief completion: unknown shell "tcsh"; expected one of: bash, zsh, fish, powershell`,
  exit 2 — SCENARIO-13. The branch exists in `runCompletion`; S13 adds the row and
  mutation-verifies it.
- Feature-name completion (`ValidArgsFunction`) — named follow-up in ADR-002, out of scope.
- No golden of any generated completion script's full bytes — deliberately; cobra owns them.
  `completion_test.go` asserts only each shell's header-line marker.
- Unowned, flagged for the final product-vision pass: `<command>` vs `<type>` wording between
  `new`'s errors and the group trailer; `start demo --json=`'s Go-internal `strconv.ParseBool`
  wording reaching the user through R14 (S05); a rejected `finish -handoff`/`-state` writing
  nothing to disk unasserted (S04); no value-taking flag row for a leaf other than `finish`
  (S05).

## Traps

- Cobra always registers hidden `__complete`/`__completeNoDescriptions` regardless of
  `CompletionOptions.DisableDefaultCmd`, and `help` is a real hidden child too — filter
  listings by `IsAvailableCommand` (or `listedInHelpAnnotation` where that's the deliberate
  exception), not by name. `__complete` writes "Completion ended with directive: …" to
  stderr on every run — asserting stderr empty on it is wrong.
- `new` is `DisableFlagParsing`; `new --bogus`/`-x`/`--help` all go through `runNew`, not
  `FlagErrorFunc` or cobra's help check. `new --help feature` does NOT reach `feature`'s
  help — `stripFlags` takes `feature` as `--help`'s value at `Find` time. Only
  `new feature --help` reaches it. (SCENARIO-02, 03, 10)
- `helpTemplate`'s `define` blocks encode newlines via `-}}`/`{{-` trim markers; adding or
  moving a `define`, or changing which branch a command falls into, shifts whitespace
  silently — prove `rootHelp`/`startHelp`/`newHelp` byte-identical after any template edit.
  `helpTemplate` is a `var` built with `fmt.Sprintf(..., listedInHelpAnnotation)`: any other
  `%` in the template body needs escaping to `%%` or Sprintf misparses it. `finish ... --handoff
  --state s.md` reports `too many arguments`: pflag takes `--state` as `--handoff`'s value; a
  `Changed("help")` guard in `SetFlagErrorFunc` would spill into S03/S04's rows — don't add
  one. (SCENARIO-05, 06, 07, 10)
- `Commands()` returns registration order only because `EnableCommandSorting` is false (a
  cobra package global set once at init). Test literals for user-facing stderr/stdout are the
  contract, never built from a const or `fmt.Sprintf` in production **or in test**. Each table
  row carries its own full literal. (SCENARIO-07, 09, 11)
- Cobra's `GenZshCompletion`/`GenFishCompletion`/`GenBashCompletionV2`/
  `GenPowerShellCompletionWithDesc` read the tree from whichever `*cobra.Command` they're
  called on — call them on `cmd.Root()`, never the leaf, or the script names the wrong
  program.
- `completion` is `ArbitraryArgs` (via `leafCommand`) — cobra never counts its args;
  `runCompletion`'s own guards are the only ones.

## Open debts

- Coverage duplicated between older SCENARIO-01 tests and `flag_error_test.go`'s tables (two
  S03 shorthand rows; S04's control-arm `--help` row; S06's `start_test.go:462,498` rows,
  kept because they assert the raw trailing `\n` `oneLine` trims). Unowned — dies unless
  re-opened.
