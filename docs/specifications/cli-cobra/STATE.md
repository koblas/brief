# cli-cobra — current state

Scenarios complete: SCENARIO-01..08. Last updated by SCENARIO-08.

## Binding decisions

- Command tree rebuilt per `Run` in `newRootCommand`; no package-level `*cobra.Command` —
  cobra stores parse state on the command itself. (SCENARIO-01)
- Root/`new` use `DisableFlagParsing`+`ArbitraryArgs`+`RunE` (`runRoot`/`runNew`); every leaf
  uses `ArbitraryArgs` and does its own arg-count check — keeps unknown command/type on
  brief's own error, not cobra's dispatch. (SCENARIO-01)
- R14 frame is one root `SetFlagErrorFunc` (path = `CommandPath()` minus `"brief "`,
  invocation = `Annotations["invocation"]`); pflag's wording passes through verbatim,
  mutation-verified per moving part by S02–S06 — do not re-derive it per command. `-h` rows
  scoped to `start` only; a leaf adding its own `-h`/`help` flag must add its own rows. An
  undefined flag beats `--help`/`-h` in either order — structural (`ParseFlags` stops at the
  first bad token before any help check); no pre-dispatch "help wins" args scan. (SCENARIO-01..06)
- `flag_error_test.go` holds one flag-parse-error table per scenario (`oneLine` helper,
  literal stderr, never built from the `invocation` constant) — see the file for rows; add
  new flag-error cases as new table rows, not new test functions. (SCENARIO-02..06)
- Single-dash long flags (`-json`/`-handoff`/`-state`/`-help`) parse as shorthand clusters,
  rejected via R14. No leaf may add shorthand `j`/`s`/`a`/`e`, or a long name defining those
  letters, without updating S04's rows. (SCENARIO-04)
- `--handoff=`/`--state=` are **not** flag-parse errors (pflag accepts the empty value) —
  they reach `runFinish`'s own `--handoff/--state is required` guard instead.
  `MarkFlagRequired` would change that wording; S05's `=` rows must move with it. (SCENARIO-05)
- Help renders through one `helpTemplate` set on root via `SetHelpTemplate`, never
  `SetHelpFunc` (short-circuits the template on any ancestor). The hidden `help` stub
  (replaces cobra's default, keeping `os.Exit` out of `internal/cli`) resolves its topic via
  `cmd.Root().Find(args)`, calls `target.InitDefaultHelpFlag()` (`Find` skips the flag cobra
  normally adds during dispatch; omitting the call drops `target`'s own `-h, --help` row),
  then `target.Help()` — byte-identical to `<path…> --help` for all six leaves, pinned in
  `help_test.go`. Unresolved topic → `target == root`, root help. (SCENARIO-01, SCENARIO-07,
  SCENARIO-08)
- `Long` is prose only; flag paragraphs live in each flag's usage string (finish's carry a
  backquoted `` `path` `` so the table reads `--handoff path`, not `string`). `Use` carries
  arg syntax (leaves set `DisableFlagsInUseLine`); `Annotations[invocation]` is a
  hand-written literal independent of `Use`/`UseLine()`. `-h, --help` is cobra's default;
  no leaf registers its own help flag. (SCENARIO-07)
- Root listing = each available command's `.UseLine` + `Short`, in registration order
  (`new feature, new step, start, status, check, finish`; `cobra.EnableCommandSorting =
  false` at package init, never per-`Run`), `new` expanded to its children in `new`'s place,
  an overlong `UseLine` (finish) wrapped to its own line with `Short` at the same column,
  trailer `Run 'brief <command> --help' for details.` — pinned byte-exact in `help_test.go`.
  Start's row now shows `[--json]` (from `.UseLine`) — flagged for the final product-vision
  pass. (SCENARIO-07)
- `Run`'s `root.SetArgs` always gets a fresh, non-nil copy of `args` (cobra reads
  `os.Args[1:]` when `c.args == nil`). Every `RunE` reads `context.Context` via
  `cmd.Context()`, never a threaded param — `ExecuteContext(ctx)` guarantees it first. (SCENARIO-01)

## Left unbuilt

- `help bogus` usage error (exit 2, `expected one of: …`) — S09. Today renders root help,
  exit 0: `Find` never fails on this `ArbitraryArgs`-everywhere tree, so S09 must key off
  residual post-`Find` args, not a `Find` error. Same rule covers `help <cmd> <extra>`/
  `help <cmd> --flag` (e.g. `help start extra`, `help start --json`, `help new bogus`) and
  `help help` — all currently render some leaf's own help; none pinned. (SCENARIO-08)
- `new`'s own help (`brief new --help` still a `runNew` usage error; `brief help new` now
  renders `new`'s bare template output — `"Usage:\n  brief new\n\n\n\nFlags:\n"`, no
  `Short`/`Long`, empty flag table since `new` is `DisableFlagParsing`; `new` has no `Short`,
  so no root-listing row of its own) — S10. Not an S10 regression: S08's correct `Find`
  resolution is what exposes the bare render.
- Tree-derived `expected one of:` list — S11; `runRoot` keeps a hard-coded literal until then.
- `completion` command (`CompletionOptions.DisableDefaultCmd` flips back on) — S12/S13.
- Assertion that a rejected `finish -handoff`/`-state` writes nothing to disk (S04), or a
  value-taking flag row for a leaf other than `finish` (S05) — unowned.
- `start demo --json=` → Go-internal `strconv.ParseBool` wording reaches the user through R14
  (observed, exit 2). Unowned; flagged for the final product-vision pass. (SCENARIO-05)

## Traps

- Cobra always registers hidden `__complete`/`__completeNoDescriptions` in `ExecuteC`
  regardless of `CompletionOptions.DisableDefaultCmd`, and the hidden `help` stub is a real
  child too. Any listing built from the tree (S07's root help, S11's future
  `expected one of:`) must filter `IsAvailableCommand`, not exclude by name.
- Cobra's *default* help command calls `cobra.CheckErr` → `os.Exit(1)` on an unknown topic,
  only when `Find` returns nil/errors — every command here sets `ArbitraryArgs`, so `Find`
  never does; see *Left unbuilt* for the residual-args rule this forces on S09.
  `Find(["--json","start"])` → root (residual `["--json","start"]`): a leading flag before
  the topic does not resolve it.
- `runRoot`'s `case "help"` is unreachable from argv — `Find(["help"])` always returns the
  hidden stub first. Its `-h`/`--help` cases are live (root is `DisableFlagParsing`). The
  `rootHelp` golden proves the *output*, not this path; don't delete the branch on that basis.
- `start_test.go:462,498` compare raw `stderr.String()` (trailing `\n`); the rest of the
  suite uses `oneLine` (trims it) — don't unify without checking both are intentional.
- pflag flag values are read with the error discarded (e.g. `GetBool("json")`) — safe only
  because the flag is registered on that same command immediately above.
- `new` is `DisableFlagParsing`; `new --bogus`/`new -x` go through `runNew`, not
  `FlagErrorFunc`, with different wording. Don't add root/`new`-level rows to the shorthand
  or long-flag tables. (SCENARIO-02, SCENARIO-03)
- A shorthand cluster with a defined letter consumed first quotes only the **residual**
  cluster; an all-undefined cluster quotes the whole thing. `-h` is the only defined
  shorthand today. (SCENARIO-03)
- `finish ... --handoff --state s.md` reports `too many arguments`: pflag takes `--state` as
  `--handoff`'s value. Mutating one flag's `NoOptDefVal` reddens every space-separated
  occurrence of that flag in the suite. (SCENARIO-05)
- A `Changed("help")` "help wins" guard in `SetFlagErrorFunc` reddens only the help-FIRST
  order and spills into S03/S04's rows whose leading `h` sets the help flag. Don't add it. (SCENARIO-06)
- With sorting off, a tree-derived list comes out in registration order `new, start, status,
  check, finish` — today's `expected one of:` literal is `new, start, finish, status, check`.
  S11 must pin whichever order it ships; reordering `root.AddCommand` also moves the root
  help listing. `EnableCommandSorting` is a cobra package global: set once at init, never per
  `Run`. (SCENARIO-07)
- pflag `FlagUsages()` (wrap width 0) re-indents a usage string's embedded newlines to the
  description column — golden indentation is pflag's, not the constant's. (SCENARIO-07)
- `cobra.AddTemplateFunc`/`AddTemplateFuncs` mutate a package global; `helpTemplate` uses
  only cobra's pre-registered funcs (`rpad`, `trim`, `trimTrailingWhitespaces`) plus
  `text/template` builtins. (SCENARIO-07)

## Open debts

- Coverage duplicated between older SCENARIO-01 tests and `flag_error_test.go`'s tables (two
  S03 shorthand rows in `run_test.go`/`new_step_test.go`; S04's control-arm `--help` row in
  `run_test.go`; S06's two `start --help` rows in `start_test.go`, kept because it asserts
  the raw trailing `\n` that `oneLine` trims). Unowned — dies unless re-opened.
