# cli-cobra — current state

Scenarios complete: SCENARIO-01..13, plus three fix passes: `/run-reviewers` findings
(correctness + test, both MAJOR), product-vision's final SHIP WITH CHANGES verdict
(4 MAJOR findings), then a correctness pass on `isFlagLike`/`unknownFlagMessage`'s
token-classification edge cases. Last updated by that third fix pass.

## Binding decisions

- Command tree rebuilt per `Run` in `newRootCommand`; no package-level `*cobra.Command`.
  Root/`new` use `DisableFlagParsing`+`ArbitraryArgs`+`RunE`; every leaf uses
  `ArbitraryArgs` and does its own arg-count check. (SCENARIO-01)
- One root `SetFlagErrorFunc` frame (path = `CommandPath()` minus `"brief "`, invocation =
  `Annotations["invocation"]`) computes `msg` once: `boolFlagParseMessage` first (checks
  `.Value.Type() == "bool"`, so a non-bool flag's own pflag wording passes through
  unrewritten — `Test_bool_flag_rewrite_does_not_apply_to_a_non_bool_flag`,
  cli_internal_test.go), else `flattenOneLine(err.Error())`. (SCENARIO-01..06, 12; fix
  passes)
- `-h`/`--help` routes to `cmd.Help()` only as the **sole** argument, at root (`runRoot`),
  under `new` (`runNew`), and — unconditionally — the help stub (`newHelpCommand`). At all
  three, `args[0]` is classified in order (`cli.go`): `helpFlagWithValue` (`"--help=<v>"`/
  `"-h=<v>"`, any value — `'<flag>' takes no value; run '...'`, flag echoed as typed, even
  a value pflag's own bool parser would accept) → `isHelpFlag` (`"--help"`, or any
  all-`'h'` shorthand cluster — `"-hh"` parses without error under pflag too, since `'h'`
  is the only no-value shorthand this tree registers) → `isFlagLike` (`-`-prefixed,
  `len > 1`, **excluding** `"--"` — pflag's terminator, not a flag, falls through to plain
  unknown-command/type wording) → `unknownFlagMessage` (owns its own `flattenOneLine`;
  callers never re-wrap it): bad double-dash syntax (`""`/`-`/`=` after `--`) is
  `"bad flag syntax: <arg>"`; a single-dash cluster skips any leading run of `'h'` (not
  just one) before quoting the first residual char and cluster — `"-hx"` → `"in -x"`.
  Neither `runRoot` nor `runNew` ever sees `"help"` itself as `args[0]` — cobra's `help`
  stub intercepts that first. (SCENARIO-02, 03, 10; fix passes)
- `resolveRoot(wd)` (`cli.go`) is the one place calling `config.Resolve`; every
  config-touching command still calls its own `renderRefusal(stderr, "<command>", err)`.
  Every leaf's invocation string is a named constant used at `leafCommand` registration and
  every `usageError`; test literals stay hand-typed, never built from these. (fix pass)
- Help renders through one `helpTemplate` (`fmt.Sprintf`-built; `%[1]q`/`%[2]q` =
  `listedInHelpAnnotation`/`commandNounAnnotation`, `%[3]d`/`%[4]d` = named consts
  `cmdRowUseWidth`(33)/`cmdRowContinuationWidth`(35)). `new` carries
  `commandNounAnnotation: "type"`, root falls back to `"command"`.
  `newHelpCommand(stderr)`: `cmd.Root().Find(args)`, `InitDefaultHelpFlag()`, `Help()` —
  byte-identical to `<path…> --help`. `rootHelp`/`startHelp`/`newHelp`/`finishHelp`
  goldens guard the template. (SCENARIO-01, 07, 08, 09, 10, 12; fix passes)
- `handoffFlagUsage`/`stateFlagUsage` (`cli.go`) wrap via embedded newlines, the same
  convention `jsonFlagUsage` uses. Every leaf's `--help` line stays ≤ 80 columns
  (`Test_every_leaf_help_line_fits_in_80_columns`); root/`new`'s `cmdList` rows are a
  separate, fixed-column-padded layout not held to that bound.
- `expectedCommandList(cmd)` is the single source for every "expected one of:" list, in
  registration order (`new, start, finish, status, check`; `completion` excluded —
  `Hidden`, carries `listedInHelpAnnotation` instead). (SCENARIO-07, 11, 12)
- `completion`: brief's own leaf, dispatch off an ordered `[]completionShell` table,
  generated against `cmd.Root()`. (SCENARIO-12, 13)

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
