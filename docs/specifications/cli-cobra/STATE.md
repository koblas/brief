# cli-cobra — current state

Scenarios complete: SCENARIO-01..09. Last updated by SCENARIO-09.

## Binding decisions

- Command tree rebuilt per `Run` in `newRootCommand`; no package-level `*cobra.Command` —
  cobra stores parse state on the command itself. Root/`new` use
  `DisableFlagParsing`+`ArbitraryArgs`+`RunE`; every leaf uses `ArbitraryArgs` and does its
  own arg-count check — unknown command/type stays brief's own error, not cobra's dispatch.
  (SCENARIO-01)
- R14 frame is one root `SetFlagErrorFunc` (path = `CommandPath()` minus `"brief "`,
  invocation = `Annotations["invocation"]`); pflag's wording passes through verbatim. `-h`
  rows scoped to `start` only. An undefined flag beats `--help`/`-h` in either order —
  structural (`ParseFlags` stops at the first bad token first), no pre-dispatch scan.
  `flag_error_test.go` holds one table (`oneLine` helper, literal stderr, never built from
  the `invocation` constant) — add cases as rows, not functions. Single-dash long flags parse
  as shorthand clusters and are rejected via R14. `--handoff=`/`--state=` are **not**
  flag-parse errors; they reach `runFinish`'s own required-flag guard. (SCENARIO-01..06)
- Help renders through one `helpTemplate` set on root via `SetHelpTemplate` (never
  `SetHelpFunc`, which short-circuits the template on any ancestor). The hidden `help` stub
  (replaces cobra's default, keeping `os.Exit` out of `internal/cli`) resolves its topic via
  `cmd.Root().Find(args)`, calls `target.InitDefaultHelpFlag()` (`Find` skips the flag cobra
  normally adds during dispatch), then `target.Help()` — byte-identical to `<path…> --help`
  for all six leaves. (SCENARIO-01, SCENARIO-07, SCENARIO-08)
- Help topic accepted iff `Find`'s residual is empty AND (target is root OR
  `target.IsAvailableCommand()`); else `brief help: unknown command "<topic>"; expected one
  of: <list>`, exit 2 — `<topic>` is every topic arg **as typed**, joined by spaces, never
  the residual alone. Rejects a trailing extra positional/flag, a flag ahead of the topic
  (`Find` stops at root, treating it as the topic), and any hidden command incl. `help`
  itself. The list is one const, `expectedCommands` in `cli.go`
  (`"new, start, finish, status, check"`), used by `runRoot` (×2) and the help stub — S11
  replaces it with a tree derivation; all three call sites move together. (SCENARIO-09)

## Left unbuilt

- `new`'s own help (`brief new --help` still a usage error; `brief help new` renders `new`'s
  bare template — observed `"Usage:\n  brief new [flags]\n\n\n\nFlags:\n  -h, --help   help
  for new\n"`, because `newCmd` doesn't set `DisableFlagsInUseLine` and has no `Short`/`Long`)
  — S10.
- Tree-derived `expected one of:` list — S11.
- `completion` command (`DisableDefaultCmd` flips back on) — S12/S13. If S12 wants
  `help completion` to render despite `Hidden: true`, it must carve that out of S09's
  hidden-target rule explicitly; `completion` must still stay out of the list.
- Per-level help errors for `new` — deliberately not built; S09's one format string + one
  list covers every topic instead.
- Assertion that a rejected `finish -handoff`/`-state` writes nothing to disk (S04), or a
  value-taking flag row for a leaf other than `finish` (S05) — unowned.
- `start demo --json=` → Go-internal `strconv.ParseBool` wording reaches the user through
  R14. Unowned; flagged for the final product-vision pass. (SCENARIO-05)

## Traps

- Cobra always registers hidden `__complete`/`__completeNoDescriptions`, and `help` is a real
  hidden child too. Any listing built from the tree must filter `IsAvailableCommand`, not
  exclude by name.
- `Find`'s residual is unstripped (flags included) and `Find` never errors on this
  `ArbitraryArgs`-everywhere tree — the help stub keys off `len(residual) > 0`, never a
  `Find` error; keying off the error would accept `help start --json`.
  `Find(["--json","start"])` → root, residual is the whole slice: a leading flag before the
  topic never resolves it.
- `runRoot`'s `case "help"` is unreachable from argv — `Find(["help"])` always returns the
  hidden stub first. Don't delete the branch on the strength of the `rootHelp` golden alone —
  it proves the output, not this path.
- `new` is `DisableFlagParsing`; `new --bogus`/`new -x` go through `runNew`, not
  `FlagErrorFunc`. A shorthand cluster with a defined letter consumed first quotes only the
  residual cluster; `-h` is the only defined shorthand today. (SCENARIO-02, SCENARIO-03)
- `finish ... --handoff --state s.md` reports `too many arguments`: pflag takes `--state` as
  `--handoff`'s value. Mutating one flag's `NoOptDefVal` reddens every space-separated
  occurrence of that flag. A `Changed("help")` "help wins" guard in `SetFlagErrorFunc`
  reddens only the help-FIRST order and spills into S03/S04's rows — don't add one.
  (SCENARIO-05, SCENARIO-06)
- With sorting off, a tree-derived list comes out in registration order `new, start, status,
  check, finish` — today's `expectedCommands` literal is `new, start, finish, status, check`.
  S11 must pick which order it ships; reordering `root.AddCommand` also moves the root help
  listing and S09's stderr literals. `EnableCommandSorting` is a cobra package global: set
  once at init, never per `Run`. (SCENARIO-07, SCENARIO-09)
- pflag `FlagUsages()` (wrap width 0) re-indents a usage string's embedded newlines to the
  description column — golden indentation is pflag's, not the constant's.
  `cobra.AddTemplateFunc`/`AddTemplateFuncs` mutate a package global; `helpTemplate` uses
  only cobra's pre-registered funcs plus `text/template` builtins. (SCENARIO-07)
- Test literals for user-facing stderr/stdout are the contract: never build an expected
  string from a const or `fmt.Sprintf` in the test. (SCENARIO-07, SCENARIO-09)

## Open debts

- Coverage duplicated between older SCENARIO-01 tests and `flag_error_test.go`'s tables (two
  S03 shorthand rows; S04's control-arm `--help` row; S06's two `start --help` rows —
  `start_test.go:462,498` — kept because they assert the raw trailing `\n` that `oneLine`
  trims). Unowned — dies unless re-opened.
