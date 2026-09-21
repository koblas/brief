# cli-cobra — current state

Scenarios complete: SCENARIO-01..13 (all scenarios implemented), plus one fix pass on
`/run-reviewers` findings (correctness + test, both MAJOR). Last updated by that fix pass.

## Binding decisions

- Command tree rebuilt per `Run` in `newRootCommand`; no package-level `*cobra.Command` —
  cobra stores parse state on the command itself. Root/`new` use
  `DisableFlagParsing`+`ArbitraryArgs`+`RunE`; every leaf uses `ArbitraryArgs` and does its
  own arg-count check. (SCENARIO-01)
- One root `SetFlagErrorFunc` frame (path = `CommandPath()` minus `"brief "`, invocation =
  `Annotations["invocation"]`) runs pflag's error text through `flattenOneLine` first, so a
  flag name carrying a literal newline still reports one stderr line. Undefined flag beats
  `--help`/`-h` in either order; single-dash long flags are shorthand clusters, rejected
  through this frame; `--handoff=`/`--state=` reach `runFinish`'s own required-flag guard.
  (SCENARIO-01..06, 12; fix pass)
- `-h`/`--help` routes to `cmd.Help()` only as the **sole** argument, at root (`runRoot`)
  and under `new` (`runNew`, same rule); any trailing argument is the ordinary
  unknown-command/unknown-type error naming `args[0]`. Neither sees `"help"` itself as
  `args[0]` — cobra's own `help` stub (`newHelpCommand`, factored out of `newRootCommand`)
  intercepts that as a subcommand dispatch first. `cobra.MousetrapHelpText = ""` sits beside
  `EnableCommandSorting` in `init`: cobra's Windows mousetrap check calls `os.Exit(1)`
  directly, bypassing `ExitCode`. (SCENARIO-02, 03, 10; fix pass)
- `resolveRoot(wd) (config.Config, string, error)` (`cli.go`) is the one place calling
  `config.Resolve` and computing the directory config paths are relative to; all six
  config-touching commands use it and still call their own
  `renderRefusal(stderr, "<command>", err)`. `config.Resolve` sits in wrapcheck's
  `extra-ignore-sigs`: its errors are already wrapped internally, so re-wrapping at the call
  site would double the prefix or break the `*InvalidConfigError` assertion. (fix pass)
- Every leaf's invocation string is a named constant beside `finishInvocation`/
  `completionInvocation` (`newFeatureInvocation`, `newStepInvocation`, `startInvocation`,
  `statusInvocation`, `checkInvocation`), used at `leafCommand` registration and every
  `usageError` in that file. Test literals stay hand-typed, never built from these. (fix pass)
- Help renders through one `helpTemplate` (a `var`, `fmt.Sprintf`-built so
  `listedInHelpAnnotation` and `index .Annotations %q` stay in sync), set on root via
  `SetHelpTemplate`. A `cmdList` group body for root/`new`; a leaf body for the rest —
  admits a command when `.IsAvailableCommand` OR it carries `listedInHelpAnnotation`.
  `newHelpCommand(stderr)` builds the hidden `help` stub (replaces cobra's default so no
  `os.Exit` lives in `internal/cli`): `cmd.Root().Find(args)`, `InitDefaultHelpFlag()`, then
  `Help()` — byte-identical to `<path…> --help`. Topic accepted iff `Find`'s residual is
  empty AND (target is root, `IsAvailableCommand()`, or carries `listedInHelpAnnotation`);
  else a usage error, exit 2. `rootHelp`/`startHelp`/`newHelp` goldens guard the template.
  (SCENARIO-01, 07, 08, 09, 10, 12)
- `expectedCommandList(cmd)` is the single source for every "expected one of:" list —
  `cmd.Root().Commands()` filtered by `IsAvailableCommand`, registration order, never a
  name-based filter. Registration order (`init`'s `EnableCommandSorting = false`) is
  `new, start, finish, status, check, completion`; `completion` registers last, `Hidden`
  with `listedInHelpAnnotation`, so it gets a root-help row and a help topic without ever
  joining an "expected one of:" list. (SCENARIO-07, 11, 12)
- `completion`: brief's own leaf (`CompletionOptions.DisableDefaultCmd` stays `true`),
  dispatch off an ordered `[]completionShell{name, gen}` table (bash/zsh/fish/powershell),
  generated against `cmd.Root()`. Matching is exact/case-sensitive; an empty positional is
  an unknown shell, not `no shell given` (reserved for zero positionals). (SCENARIO-12, 13)

## Left unbuilt

- `new`'s type list (`expected one of: feature, step`) stays a literal, not
  `expectedCommandList`'s list. Unowned.
- Per-level help errors for `new` — `brief new help` stays `unknown type "help"`.
- Feature-name completion (`ValidArgsFunction`) — ADR-002 follow-up, out of scope.
- No golden of a generated completion script's full bytes — cobra owns them.
- "Did you mean" for a near-miss shell name, and trimming the shell argument — not built.
- Flagged for the final product-vision pass: `<command>` vs `<type>` wording between `new`'s
  errors and the group trailer; `start demo --json=`'s raw `strconv.ParseBool` wording
  reaching the user; a rejected `finish -handoff`/`-state` writing nothing unasserted; no
  value-taking flag row for a leaf other than `finish`.

## Traps

- Cobra always registers hidden `__complete`/`__completeNoDescriptions` and a real hidden
  `help` child regardless of `DisableDefaultCmd` — filter by `IsAvailableCommand` (or
  `listedInHelpAnnotation`), never by name. `__complete` writes to stderr on every run.
- `new` is `DisableFlagParsing`; every `new --bogus`/`-x`/`--help` goes through `runNew`,
  never `FlagErrorFunc` or cobra's help check. `new --help feature` does NOT reach
  `feature`'s help — `stripFlags` takes `feature` as `--help`'s value at `Find` time.
  (SCENARIO-02, 03, 10)
- `helpTemplate`'s `define` blocks encode newlines via `-}}`/`{{-` trim markers — prove the
  three goldens byte-identical after any template edit. Any other `%` in the template body
  needs `%%` or `fmt.Sprintf` misparses it. A `Changed("help")` guard in `SetFlagErrorFunc`
  would spill into S03/S04's rows — don't add one. (SCENARIO-05, 06, 07, 10)
- Test literals for user-facing stderr/stdout are the contract, never built from a const or
  `fmt.Sprintf` in production **or in test**. (SCENARIO-07, 09, 11)
- Cobra's `GenZshCompletion`/`GenFishCompletion`/`GenBashCompletionV2`/
  `GenPowerShellCompletionWithDesc` read whichever `*cobra.Command` they're called on — call
  on `cmd.Root()`, never the leaf. `completion` is `ArbitraryArgs`; `runCompletion`'s own
  guards are the only arg-count check.

## Open debts

- Coverage duplicated between older SCENARIO-01 tests and `flag_error_test.go`'s tables (two
  S03 shorthand rows; S04's control-arm `--help` row; S06's `start_test.go:462,498` rows,
  kept because they assert the raw trailing `\n` `oneLine` trims). Unowned — dies unless
  re-opened.
