# cli-cobra — current state

Scenarios complete: SCENARIO-01..13, plus two fix passes: `/run-reviewers` findings
(correctness + test, both MAJOR), then product-vision's final SHIP WITH CHANGES verdict
(4 MAJOR findings). Last updated by that second fix pass.

## Binding decisions

- Command tree rebuilt per `Run` in `newRootCommand`; no package-level `*cobra.Command`.
  Root/`new` use `DisableFlagParsing`+`ArbitraryArgs`+`RunE`; every leaf uses
  `ArbitraryArgs` and does its own arg-count check. (SCENARIO-01)
- One root `SetFlagErrorFunc` frame (path = `CommandPath()` minus `"brief "`, invocation =
  `Annotations["invocation"]`) runs pflag's error text through `flattenOneLine` first.
  `boolFlagParseMessage` intercepts `*pflag.InvalidValueError` for any bool-typed flag
  first and reports `invalid value %q for --%s (want true or false, or no value)` instead
  of pflag's raw `strconv.ParseBool` wording — generic, not hard-coded to `--json`.
  (SCENARIO-01..06, 12; fix passes)
- `-h`/`--help` routes to `cmd.Help()` only as the **sole** argument, at root (`runRoot`),
  under `new` (`runNew`), and — unconditionally, no sole-argument exception — the help stub
  (`newHelpCommand`). Any other dash-prefixed `args[0]` at those three sites is shared
  wording from `isHelpFlag`/`isFlagLike`/`unknownFlagMessage` (`cli.go`): `-h`/`--help`
  alongside another argument reports `'<flag>' takes no arguments; run 'brief help
  <command|new <type>>'`; any other dash-prefixed token (`--version`, `-x`, `-xy`) gets
  pflag's own unknown-flag/unknown-shorthand wording, pointing at `brief <command|new
  <type>> --help`. `isFlagLike` requires `len(arg) > 1`, so a bare `-` stays a plain
  unknown command/type, matching pflag's own `parseArgs`. Neither `runRoot` nor `runNew`
  ever sees `"help"` itself as `args[0]` — cobra's own `help` stub intercepts that as a
  subcommand dispatch first. (SCENARIO-02, 03, 10; fix passes)
- `resolveRoot(wd)` (`cli.go`) is the one place calling `config.Resolve`; every
  config-touching command still calls its own `renderRefusal(stderr, "<command>", err)`.
  Every leaf's invocation string is a named constant used at `leafCommand` registration and
  every `usageError`; test literals stay hand-typed, never built from these. (fix pass)
- Help renders through one `helpTemplate` (`fmt.Sprintf`-built, two `%q` slots:
  `listedInHelpAnnotation`, `commandNounAnnotation`). The trailer reads `Run '<path>
  <{{or (index .Annotations "commandNoun") "command"}}> --help' for details.` — `new`
  carries `commandNounAnnotation: "type"` (used by both the trailer and the wording bucket
  above), root carries none and falls back to `"command"`. `newHelpCommand(stderr)`:
  `cmd.Root().Find(args)`, `InitDefaultHelpFlag()`, `Help()` — byte-identical to `<path…>
  --help`. `rootHelp`/`startHelp`/`newHelp`/`finishHelp` goldens guard the template.
  (SCENARIO-01, 07, 08, 09, 10, 12; fix pass)
- `handoffFlagUsage`/`stateFlagUsage` (`cli.go`) wrap via embedded newlines, the same
  convention `jsonFlagUsage` uses: `FlagUsages` re-indents them to the description column
  (`maxlen+2` = 23 for `finish`'s flag set). Every leaf's `--help` line stays ≤ 80 columns;
  root/`new`'s `cmdList` rows are a separate, fixed-column-padded layout not held to that
  bound.
- `expectedCommandList(cmd)` is the single source for every "expected one of:" list.
  Registration order is `new, start, finish, status, check, completion`; `completion` is
  `Hidden` with `listedInHelpAnnotation`. (SCENARIO-07, 11, 12)
- `completion`: brief's own leaf, dispatch off an ordered `[]completionShell` table,
  generated against `cmd.Root()`. `new` carries a `Short` for completion descriptions even
  though root help never renders it (always expands to `new`'s children's rows instead).
  (SCENARIO-12, 13)

## Left unbuilt

- `new`'s type list (`expected one of: feature, step`) stays a literal.
- Per-level help errors for `new` — `brief new help` stays `unknown type "help"`.
- Feature-name completion (`ValidArgsFunction`) — ADR-002 follow-up, out of scope.
- No golden of a generated completion script's full bytes — cobra owns them.
- "Did you mean" for a near-miss shell name, and trimming the shell argument — not built.
- A rejected `finish -handoff`/`-state` writing nothing to disk stays unasserted.

## Traps

- Cobra always registers hidden `__complete`/`__completeNoDescriptions` and a real hidden
  `help` child regardless of `DisableDefaultCmd` — filter by `IsAvailableCommand` (or
  `listedInHelpAnnotation`), never by name. `__complete` writes to stderr on every run.
- `new` is `DisableFlagParsing`; every `new --bogus`/`-x`/`--help` goes through `runNew`,
  never `FlagErrorFunc` or cobra's help check. `new --help feature` does NOT reach
  `feature`'s help — `Find` takes `feature` as `--help`'s value first.
- `helpTemplate`'s `define` blocks encode newlines via `-}}`/`{{-` trim markers — prove the
  goldens byte-identical after any template edit; any other `%` in the body needs `%%`.
- Test literals for user-facing stderr/stdout are the contract, never built from a const or
  `fmt.Sprintf` in production **or in test**.
- Cobra's `Gen*Completion*` functions read whichever `*cobra.Command` they're called on —
  call on `cmd.Root()`, never the leaf.

## Open debts

- Coverage duplicated between older SCENARIO-01 tests and `flag_error_test.go`'s tables.
  Unowned — dies unless re-opened.
- Accepted as-is by product-vision, no code change: every leaf accepts `--help` with extra
  trailing args (cobra's own default; only root/`new`/`help` enforce strictness);
  `completion`'s write-failure silent exit 1 is a package-wide pattern, not unique to it.
- Backlog, not this feature: `brief --version` is a new surface needing its own pipeline
  pass (product-vision used it only as an example unknown-flag literal in this fix).
