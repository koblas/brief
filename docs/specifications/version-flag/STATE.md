# version-flag — current state

Scenarios complete: SCENARIO-01..07 — all scenarios in `specification.md` are done.

## Binding decisions

- Build info is injected as `func() (*debug.BuildInfo, bool)` — `debug.ReadBuildInfo`'s own
  signature — threaded through unexported `run(...)` → `newRootCommand(...)` →
  `runRoot(cmd, args, stdout, stderr, readBuildInfo)`. `cli.Run` keeps its exported
  signature, delegating to `debug.ReadBuildInfo` in one line; no package-level reader var.
  In a `go test` binary, `debug.ReadBuildInfo()` always returns `ok=true,
  Main.Version == "(devel)"`, so only the fallback is reachable through black-box `cli.Run`
  tests; use the `run` seam (`version_internal_test.go`, `package cli`) with a fake
  `readBuildInfo` for a pass-through exact-value assertion. (SCENARIO-01)
- `argKind` has six values. `classifyDashArg("--version")` returns `argVersionFlag`;
  `classifyDashArg("--version=<v>")` (any value, incl. empty) returns
  `argVersionFlagWithValue`. Both carry msg `unknownLongFlagMessage(arg)` =
  `unknown flag: --version`, so `runNew`/the help stub fold both into their existing
  `argUnknownFlag, argVersionFlag, argVersionFlagWithValue` case byte-for-byte (R6). Only
  root tells the two apart. (SCENARIO-01, SCENARIO-03, SCENARIO-04)
- Root's sole-argument arms: `argVersionFlag` with `len(args) == 1` prints
  `versionLine(readBuildInfo) + "\n"`, exit 0; a trailing argument →
  `takesNoArgumentsMessage(args[0], "brief --version")`. `argVersionFlagWithValue` never
  checks `len(args)`: a value always wins over a trailing argument (R8) →
  `takesNoValueMessage("--version", "brief --version")` — the literal `"--version"`, not
  `msg` or `args[0]`. First argument alone decides which flag's error fires. (SCENARIO-03,
  SCENARIO-04)
- `takesNoArgumentsMessage(flag, runHint)`/`takesNoValueMessage(flag, runHint)` (unexported,
  `cli.go`) render root's own `brief: '<flag>' takes no arguments/value; run '<runHint>'`.
  Root-scoped only; `runNew`/the help stub keep their own inline copies (R6). (SCENARIO-03,
  SCENARIO-04)
- `versionLine(readBuildInfo) string` (unexported, `cli.go`): `"brief " + info.Main.Version`
  verbatim when `ok` and non-empty; else literal `"brief (devel)"` (R2). No
  `internal/version` package. (SCENARIO-01)
- `-v` (and clusters `-v=x`, `-vh`, `-hv`) is not a `--version` alias at any site —
  `classifyDashArg` sends it through `argUnknownFlag`/`unknownShortFlagMessage`, unchanged;
  reserved for a future `--verbose` (R5). Pinned in the cross-site table in
  `flag_error_test.go`. (SCENARIO-05)
- R6, `--version` outside root: every leaf's `--version` is an ordinary pflag unknown-flag
  error — no leaf registers a `version` flag. Pinned in
  `Test_reports_the_version_flag_as_unknown_outside_the_root` (`flag_error_test.go`).
  (SCENARIO-06)
- Root help's `--version` trailer lives inside `helpTemplate`'s shared `"cmdList"` block
  (`cli.go`), gated on `{{if not .HasParent}}` — `cmdList` also renders `new`, and R6/R7
  require `newHelp` byte-identical (its own footer stays last). Rejected: `eq .CommandPath
  "brief"` (hardcodes the binary name) and keying on the absence of
  `commandNounAnnotation` (breaks when a future group command omits the annotation). The
  trailer is root help's last line, byte-adjacent to `Run 'brief <command> --help' for
  details.` (no blank line, exactly one trailing `\n`), via trim markers, not a second
  `{{if}}` around whitespace. No package-global template func added — `not` and `if` are
  text/template builtins. (SCENARIO-07)

## Left unbuilt

- Leaf `-v` (e.g. `brief start -v`): unpinned, goes through real pflag not
  `classifyDashArg`, so R5's risk doesn't reach it. Owner: none (out of scope, per
  specification's "Out of Scope").

## Traps

- `exhaustive` lint (`default: all`, no `default:` case). Every switch on `argKind` (root,
  `new`, help stub) must name all six values.
- cobra's `Command.Version`/`InitDefaultVersionFlag` never fire: root has
  `DisableFlagParsing: true`. Don't set `root.Version` — it would be silently inert.
- Don't refactor `runNew`/the help stub onto `takesNoArgumentsMessage`/`takesNoValueMessage`:
  their prefixes/hints differ, and R6 requires their bytes unchanged.
- `strings.HasPrefix(arg, "--version")` without the `=` catches `--versionx`/`--version-foo`
  too; the classify table's `--versionx` row is the only guard against this.
- `brief help start --version` does not reach a leaf: the help stub treats the joined
  remainder as a command name (`unknown command "start --version"`).
- In `helpTemplate`, appending the `--version` line after `{{template "cmdList" .}}` leaks
  it into `newHelp` (mutation A, SCENARIO-07); appending it at the template's outer tail
  leaks it into every leaf golden, `startHelp` included (mutation B, SCENARIO-07) — both
  reproduced and restored byte-identical. An untrimmed `{{if}}`/`{{end}}` around the line
  adds a blank line or a stray trailing `\n`; a golden diff only says bytes differ, so check
  trim markers first, not the text.

## Open debts

None. All seven scenarios in `specification.md`'s `## BDD Acceptance Progress` are checked.
