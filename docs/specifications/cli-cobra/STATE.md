# cli-cobra — current state

Scenarios complete: SCENARIO-01..13, plus six fix passes: `/run-reviewers` findings
(correctness + test, both MAJOR), product-vision's final SHIP WITH CHANGES verdict
(4 MAJOR), a correctness pass on token-classification edge cases, a structural pass
replacing three per-site classification ladders with one shared `classifyDashArg`
(`classify.go`), a test pass fixing a multibyte parity bug in `unknownShortFlagMessage`,
then a product-vision fix pass giving the help stub a sole-argument `-h`/`--help`/`-hh`
case (own usage, not an error). Last updated by that sixth pass.

## Binding decisions

- Command tree rebuilt per `Run` in `newRootCommand`; no package-level `*cobra.Command`.
  Root/`new` use `DisableFlagParsing`+`ArbitraryArgs`+`RunE`; every leaf uses
  `ArbitraryArgs` and does its own arg-count check. (SCENARIO-01)
- One root `SetFlagErrorFunc` frame (`newFlagErrorFunc`, `cli.go`): `boolFlagParseMessage`
  rewrite when the failing flag is bool-typed, else `err.Error()`, always
  `flattenOneLine`d, named with `CommandPath()` + `Annotations[invocationAnnotation]`.
  (SCENARIO-01..06, 12; fix passes)
- `-h`/`--help` routes to `cmd.Help()` as the **sole** argument at root (`runRoot`), `new`
  (`runNew`), and the help stub (`newHelpCommand`) — the stub's case calls
  `cmd.InitDefaultHelpFlag()` then `cmd.Help()` on itself (Use `help [command]`,
  `DisableFlagsInUseLine: true`, `helpShort`/`helpLong` consts), rendering through the
  same leaf branch of `helpTemplate` every other `--help` does; pinned by the `helpHelp`
  golden (`help_test.go`). Alongside another argument, all three still report "takes no
  arguments". All three share one `classifyDashArg(args[0])` call (`classify.go`), full
  contract documented on that function: `argNotFlag`/`argHelpFlag`/
  `argHelpFlagWithValue`/`argUnknownFlag`, no precondition on input
  (`FuzzClassifyDashArg`). `runRoot`/`runNew` never see `"help"` itself as `args[0]` —
  cobra's stub intercepts it first. (SCENARIO-02, 03, 10; fix passes)
- `resolveRoot(wd)` (`cli.go`) is the one place calling `config.Resolve`; every
  config-touching command calls its own `renderRefusal(stderr, "<command>", err)`. Every
  leaf's invocation string is a named constant; test literals stay hand-typed. (fix pass)
- Help renders through one `helpTemplate` (`fmt.Sprintf`-built). `new` carries
  `commandNounAnnotation: "type"`, root falls back to `"command"`. A resolved topic in
  `newHelpCommand`: `cmd.Root().Find(args)`, `InitDefaultHelpFlag()`, `Help()` —
  byte-identical to `<path…> --help`. `rootHelp`/`startHelp`/`newHelp`/`finishHelp`/
  `helpHelp` goldens guard the template. (SCENARIO-01, 07-10, 12; fix passes)
- `handoffFlagUsage`/`stateFlagUsage`/`jsonFlagUsage` wrap via embedded newlines. Every
  leaf's `--help` line, including the help stub's own, stays ≤ 80 columns
  (`Test_every_leaf_help_line_fits_in_80_columns`); root/`new`'s `cmdList` rows are a
  separate fixed-column layout not held to that bound.
- `expectedCommandList(cmd)` is the single source for every "expected one of:" list, in
  registration order (`new, start, finish, status, check`; `completion` excluded —
  `Hidden`, carries `listedInHelpAnnotation` instead). (SCENARIO-07, 11, 12)
- `completion`: brief's own leaf, dispatch off an ordered `[]completionShell` table,
  generated against `cmd.Root()`. (SCENARIO-12, 13)
- `unknownShortFlagMessage`'s quoted rune mirrors pflag's own double-widen quirk for
  multi-byte UTF-8 lead bytes byte-for-byte rather than decoding the real rune. (fix pass)

## Left unbuilt

- `new`'s type list stays a literal; `brief new help` stays `unknown type "help"`.
- Feature-name completion (`ValidArgsFunction`) — ADR-002 follow-up, out of scope.
- No golden of a generated completion script's full bytes — cobra owns them. "Did you
  mean" for a near-miss shell name, and trimming the shell argument, not built.
- A rejected `finish -handoff`/`-state` writing nothing to disk stays unasserted.

## Traps

- Cobra always registers hidden `__complete`/`__completeNoDescriptions` and a real hidden
  `help` child regardless of `DisableDefaultCmd` — filter by `IsAvailableCommand` (or
  `listedInHelpAnnotation`), never by name. `__complete` writes to stderr on every run.
- `new` is `DisableFlagParsing`; `new --help feature` does NOT reach `feature`'s help —
  `Find` takes `feature` as `--help`'s value first.
- `helpTemplate`'s `define` blocks encode newlines via `-}}`/`{{-` trim markers — prove
  goldens byte-identical after any template edit; other `%` in the body needs `%%`. Test
  literals for user-facing stderr/stdout are the contract, never built from a const or
  `fmt.Sprintf` in production **or in test**.
- Cobra's `Gen*Completion*` functions read whichever `*cobra.Command` they're called on —
  call on `cmd.Root()`, never the leaf.
- `classifyDashArg` only backs root/`new`/help's hand-inspected `args[0]`. A leaf's own
  flags (`status -hh=x`, `new feature -hh`) go through real `pflag.Parse` — out of scope.
- `gosmopolitan` (golangci-lint) flags any Han-script rune in a string literal anywhere,
  `_test.go` included — use a non-Han multibyte fixture instead (e.g. Cyrillic `Ж`).
- A command's Use with no `DisableFlagsInUseLine: true` gets an auto-appended `" [flags]"`
  from cobra's `UseLine()` once it has an available flag (e.g. after
  `InitDefaultHelpFlag()`) — the help stub sets that field explicitly to avoid it.

## Open debts

- Coverage still duplicated across older SCENARIO-01/03/04/06 leaf tables in
  `flag_error_test.go` versus the cross-site classification table. Unowned.
- Accepted as-is by product-vision, no code change: every leaf accepts `--help` with extra
  trailing args (cobra's own default; only root/`new`/`help` enforce strictness);
  `completion`'s write-failure silent exit 1 is a package-wide pattern, not unique to it.
- Backlog, not this feature: `brief --version` is a new surface needing its own pipeline
  pass (product-vision used it only as an example unknown-flag literal).
- Product-vision's final review named four items optional, not required for this MAJOR:
  `-hx`'s quoted residual vs. the literal typed token in a multi-flag cluster; leaf
  `--help=<v>` (pflag's own wording) vs. root/`new`/help's `--help=<v>` ("takes no value");
  `brief -- start` not treated as pflag's end-of-flags terminator ahead of a command name;
  the "run '...'" finish-hint length on long invocations not shortened. All four: unowned
  — die unless re-opened.
