# cli-cobra — current state

Scenarios complete: SCENARIO-01..13, plus five fix passes: `/run-reviewers` findings
(correctness + test, both MAJOR), product-vision's final SHIP WITH CHANGES verdict
(4 MAJOR findings), a correctness pass on token-classification edge cases, a structural
fix pass that replaced the three per-site classification ladders with one shared
`classifyDashArg` (`classify.go`), then a test pass that fixed a genuine multibyte
parity bug in `unknownShortFlagMessage` and trimmed duplicated coverage in
`classify_internal_test.go`/`flag_error_test.go`. Last updated by that fifth pass.

## Binding decisions

- Command tree rebuilt per `Run` in `newRootCommand`; no package-level `*cobra.Command`.
  Root/`new` use `DisableFlagParsing`+`ArbitraryArgs`+`RunE`; every leaf uses
  `ArbitraryArgs` and does its own arg-count check. (SCENARIO-01)
- One root `SetFlagErrorFunc` frame, built by `newFlagErrorFunc(stderr)` (path =
  `CommandPath()` minus `"brief "`, invocation = `Annotations["invocation"]`): computes
  `msg` via `boolFlagParseMessage` first (checks `.Value.Type() == "bool"`, so a non-bool
  flag's own pflag wording passes through unrewritten —
  `Test_bool_flag_rewrite_does_not_apply_to_a_non_bool_flag`, cli_internal_test.go), else
  `err.Error()` — either way `flattenOneLine`d once at the end, uniformly.
  (SCENARIO-01..06, 12; fix passes)
- `-h`/`--help` routes to `cmd.Help()` only as the **sole** argument, at root (`runRoot`)
  and under `new` (`runNew`) — never at the help stub (`newHelpCommand`; it has no
  sole-argument case, so `help -hh` is itself an error). All three call
  `classifyDashArg(args[0])` (`classify.go`) once and switch on its `argKind`: `argNotFlag`
  (plain word, `""`, `"-"`, `"--"` — falls through to the caller's own unknown-command/type
  wording); `argHelpFlag` (`"--help"`, or any single-dash cluster made entirely of `'h'` —
  `"-hh"`, `"-hhh"`, ... — pflag's own shorthand parser accepts these too, since `'h'` is
  the only no-value shorthand this tree registers); `argHelpFlagWithValue` (`"--help=<v>"`,
  or any all-`'h'` single-dash cluster followed by `"="` — `"-h=<v>"`, `"-hh=<v>"`,
  `"-hh="` — msg is the flag part as typed, value stripped); `argUnknownFlag` (msg is the
  pflag-shaped, already-`flattenOneLine`d message — bad double-dash syntax
  `""`/`-`/`=` after `--` → `"bad flag syntax: <arg>"`; a single-dash cluster skips any
  leading run of `'h'` before quoting the first residual char and cluster — `"-hx"`/`"-hhx"`
  → `"in -x"`). No precondition on input — `FuzzClassifyDashArg`
  (classify_internal_test.go) asserts no panic and a one-line `argUnknownFlag` message over
  arbitrary strings. Neither `runRoot` nor `runNew` ever sees `"help"` itself as `args[0]` —
  cobra's `help` stub intercepts that first. (SCENARIO-02, 03, 10; fix passes)
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
- `unknownShortFlagMessage`'s quoted rune is `rune(string(residual[0])[0])`, not
  `rune(residual[0])`: pflag's `NotExistError` widens the failing byte to a rune,
  UTF-8-encodes it, then widens *that* encoding's own first byte to a rune again — the
  identity for ASCII, but every multi-byte UTF-8 lead byte collapses to one constant,
  `'Ã'` (U+00C3). The single-widen version only coincidentally matched pflag for lead
  byte `0xC3` (Latin-1 Supplement, e.g. `é`); it diverged for every other lead byte
  until this pass. Pinned by
  `Test_quotes_a_multibyte_unknown_shorthand_flag_byte_identically_across_leaf_root_and_new`
  (flag_error_test.go). (fix pass)

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
- `classifyDashArg` only backs root/`new`/help's hand-inspected `args[0]`. A leaf's own
  flags (`status -hh=x`, `new feature -hh`) still go through real `pflag.Parse` and
  `newFlagErrorFunc`, never `classifyDashArg` — leaf-level `-hh`/`-hh=x` is pflag's own
  behavior, out of scope for this package.
- `gosmopolitan` (golangci-lint, `default: all`, no test-path exclusion) flags any
  Han-script rune in a string literal anywhere, `_test.go` included — a non-Latin-1
  multibyte test fixture should use a non-Han script instead (e.g. Cyrillic `Ж`).

## Open debts

- Coverage still duplicated across older SCENARIO-01/03/04/06 leaf tables in
  `flag_error_test.go` versus the cross-site classification table; this pass trimmed
  `classify_internal_test.go`'s white-box table (19 rows to 4 anchors) and deleted the
  black-box table it fully superseded, but the leaf-level duplication remains.
  Unowned — dies unless re-opened.
- Accepted as-is by product-vision, no code change: every leaf accepts `--help` with extra
  trailing args (cobra's own default; only root/`new`/`help` enforce strictness);
  `completion`'s write-failure silent exit 1 is a package-wide pattern, not unique to it.
- Backlog, not this feature: `brief --version` is a new surface needing its own pipeline
  pass (product-vision used it only as an example unknown-flag literal in this fix).
