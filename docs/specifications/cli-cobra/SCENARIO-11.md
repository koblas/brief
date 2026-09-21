# SCENARIO-11: Unknown command stays one line, list built from the command tree

## Scenario

```gherkin
Scenario: SCENARIO-11 Unknown command stays one line, list built from the command tree
  Given a hidden command is registered
  When I run "brief bogus"
  Then stderr is one line naming only the visible commands, with no "Did you mean" block, and the exit code is 2
```

Rule: R7 — the `expected one of:` command list is derived from registered, non-hidden
commands, never a hard-coded literal; cobra suggestions are disabled.

## User-visible contract

- `brief bogus` → stderr exactly
  `brief: unknown command "bogus"; expected one of: new, start, finish, status, check` + `\n`,
  stdout empty, `errors.Is(err, cli.ErrUsage)` (exit 2). No `Did you mean` line.
- `brief` (no args) → `brief: no command given; expected one of: new, start, finish, status, check`, exit 2.
- `brief help bogus` → `brief help: unknown command "bogus"; expected one of: new, start, finish, status, check`, exit 2.
- All three strings are unchanged byte-for-byte from today: the pinned tests at
  `run_test.go:119,130,248,275` and `help_test.go:310-335` must pass **unedited**. That
  is the evidence the derived list equals the retired literal.
- Intended change: root help lists `finish` third (after `start`) instead of last.

## Test seam (decision)

`Run` takes no options, and a derivation is indistinguishable from the literal unless the
tree holds a command production does not register. The seam is a white-box test file,
`internal/cli/cli_internal_test.go` (`package cli`), that calls the unexported
`newRootCommand`, adds one extra visible and one extra hidden `*cobra.Command`, then drives
the **real** message paths with `SetArgs` + `ExecuteContext` and reads stderr.

Why this seam and not the alternatives:
- It exercises all three call sites through cobra's real `Execute`, so the hidden commands
  cobra adds only during `Execute` (`help` stub via `InitDefaultHelpCmd`, `__complete` /
  `__completeNoDesc` via `initCompleteCmd`) are present — a pure-func unit test on a tree
  from `newRootCommand` alone cannot see them.
- It proves each call site individually: swapping any one call site back to a literal
  reddens that row. A pure-func test would stay green under that mutation.
- No production API widening (no `WithX` option on `Run`, no exported constructor) and no
  `export_test.go` alias. `go-testing`'s white-box exception applies: the file's top
  comment must say it reaches `newRootCommand` only to register commands production does
  not have, and that everything else stays in the black-box `cli_test` files.

## Implementation Plan

- [x] Step 1: `internal/cli/cli_internal_test.go` `Test_expected_command_list_names_every_visible_registered_command` — white-box table over the three invocations (no args / `bogus` / `help bogus`), each on a `newRootCommand` tree plus an extra visible command and an extra hidden command; assert exact stderr line (extra visible name at the end, hidden name absent, no `help`/`__complete`), `ErrUsage`, empty stdout (red)
- [x] Step 2: same file `Test_expected_command_list_includes_a_command_once_it_is_not_hidden` — control arm: identical tree, the one extra command with `Hidden` false; its name appears. Differs from Step 1 in exactly that one field (red)
- [x] Step 3: `internal/cli/cli.go` — unexported derivation func: root's direct children filtered by `IsAvailableCommand`, names joined `", "`, in registration order; called at message time from `cmd.Root()`, never computed in `newRootCommand` (new)
- [x] Step 4: `internal/cli/cli.go` `runRoot` + help stub `RunE` — replace all three `expectedCommands` uses with the derivation; delete the `expectedCommands` const and its doc comment (green)
- [x] Step 5: `internal/cli/cli.go` `newRootCommand` — reorder `root.AddCommand` to `new, start, finish, status, check`; rewrite the `init()` comment and `newRootCommand`'s doc comment (both say `new, start, status, check, finish`) to state the rule: registration order is both the root-help order and the `expected one of:` order. No history, no scenario ids (update)
- [x] Step 6: `internal/cli/help_test.go` `rootHelp` — **intended golden change**: move finish's two-line wrapped row from last to third (after `start`); paste the continuation line's indentation from actual `brief --help` output, not hand-typed; update the const's doc comment if it names the order (update)
- [x] Step 7: `internal/cli/run_test.go` `Test_returns_a_usage_error_when_the_command_is_unknown` — add a near-miss row (`strat`) alongside `bogus`, asserting the exact one-line stderr and `ErrUsage`; green on arrival (suggestions are already structurally unreachable) — say so, do not manufacture a red (update)
- [x] Step 8: verify — `go build ./...`, `go test ./...` unpiped, `go test -race ./internal/cli/...`, `golangci-lint run ./...`; confirm the pinned tests listed above passed unedited, `rootHelp`/`startHelp`/`newHelp` byte-identical to output, report test count + delta
- [x] Step 9: mutation checks, one at a time, via a temp copy in `$TMPDIR` (never bare `git stash`/`pop`), restore and `diff` to prove byte-identical: (a) each of the three call sites individually back to the old literal → its Step 1 row reds; (b) drop the `IsAvailableCommand` filter → Step 1 rows and the pinned `run_test`/`help_test` literals red; (c) reverse Step 5's reorder → pinned literals red. Name each mutation and the test it reddened
- [x] Step 10: all green → update `STATE.md`, mark SCENARIO-11 done in `specification.md`

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Root commands register in `new, start, finish, status, check` order; that one order drives
  both root help and every `expected one of:` list. No separate ordering table —
  `EnableCommandSorting` stays false; adding a command means choosing its position in
  `root.AddCommand`, which moves both surfaces at once.
- The list is derived at message time from `cmd.Root()`, filtering by
  `IsAvailableCommand` — cobra adds `help`/`__complete*` (and S12's `completion`) during
  `Execute`, after `newRootCommand` returns; a construction-time derivation misses them.
- `rootHelp` golden now lists `finish` third; this was an intended change, not a regression.
- Seam: `cli_internal_test.go` (`package cli`) is the one justified white-box file; it only
  calls `newRootCommand` to add commands production lacks. No `export_test.go`, no test
  option on `Run`.

**Left unbuilt** — named so nobody assumes it exists:
- `new`'s type list in `new.go` (`expected one of: feature, step`, lines ~36/43) stays a
  literal — it is a type list, not R7's command list. Unowned.
- `completion` command — S12. It must be `Hidden` (`HiddenDefaultCmd`) so the derivation
  drops it with no name check.

**Traps** — things that look right and are not:
- Do not unify the help-template listing with the `expected one of:` derivation. R9 puts
  `completion` in root help (one line) but out of `expected one of:`; the template also
  expands `new` into `new feature`/`new step` while the list names `new`. Two derivations.
- `DisableSuggestions: true` is already on root and root's unknown-command path never
  reaches cobra's suggestion code (`Args` is set, so no `legacyArgs`). Removing it reddens
  nothing — do not claim it as mutation-verified; the "no Did you mean" proof is the exact
  one-line stderr equality.
- `exit_test.go`'s `errFixtureUsage` string (`new, start, finish`) is an `ExitCode` fixture,
  not a list pin — leave it.
