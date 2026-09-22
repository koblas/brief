---
id: SCENARIO-10
status: done
depends-on: []
---

# SCENARIO-10: "new" has its own help

## Scenario

```gherkin
Scenario: SCENARIO-10 "new" has its own help
  When I run "brief new --help" or "brief help new"
  Then stdout lists "new feature" and "new step", stderr is empty, and the exit code is 0
```

## User-visible contract

- `brief new --help`, `brief new -h`, `brief help new` → stdout is exactly `newHelp` (below),
  stderr empty, nil error, exit 0. All three byte-identical (R8).
- `brief new --help <anything>` (e.g. `new --help widget`, `new --help -x`) → the help form is
  accepted only as the **sole** argument; otherwise it falls through to runNew's existing
  copy naming the first argument as typed: `brief new: unknown type "--help"; expected one of:
  feature, step` (or `"-h"` for `new -h widget`), stdout empty, exit 2.
  Same strictness as S06 (undefined flag beats `--help`) and S09 (trailing extra rejected).
- Unchanged, pinned by existing tests that must pass unedited: `brief new` →
  `brief new: no type given; …` (run_test.go:133); `brief new widget x` → unknown type
  `"widget"` (run_test.go:144); `brief new -x` → unknown type `"-x"` (run_test.go:281). All
  one line on stderr, exit 2.

Golden (hand-write it in the test; the two rows are copied from `rootHelp`'s rows, same
`cmdRow` padding — never paste a captured run):

```
Scaffolds a new feature, or the next step of an existing feature.

Usage:
  brief new feature <name>         scaffold a new feature's specification and state file
  brief new step <feature>         scaffold the next step file and its progress entry

Run 'brief new <command> --help' for details.
```

No `Flags:` table and no `brief new [flags]` Usage line: `new` renders root's group shape,
not the leaf shape. Its own `UseLine` is rendered nowhere, so `DisableFlagsInUseLine` on
`newCmd` is unnecessary and not planned.

## Design decisions (this plan only)

- **Dispatch:** `newCmd` keeps `DisableFlagParsing`. `runNew` gains the `*cobra.Command`
  (mirroring `runRoot`) and, as its first check, routes an argument list of exactly `-h` or
  exactly `--help` to `cmd.Help()`. Everything else reaches today's two branches untouched.
  Rejected: removing `DisableFlagParsing` (pflag would report `new -x` as `unknown shorthand
  flag` through R14, breaking run_test.go:289); a pre-dispatch argv scan (contradicts S06's
  "structural, no pre-dispatch scan").
- **Why both paths render the same bytes:** cobra's `execute()` calls `InitDefaultHelpFlag()`
  before `ParseFlags` (which early-returns under `DisableFlagParsing`), and the help stub calls
  it explicitly — and the group shape prints no flag table anyway.
- **Template:** the single `helpTemplate` stays single-sourced. `{{if .HasParent}}` no longer
  partitions the cases (`new` is the first command that has a parent AND available
  subcommands). Extract the current root body (Long, `Usage:` rows with the nested-children
  expansion, trailer) into a `{{define "cmdList"}}` alongside `cmdRow`, render it for any
  command with available subcommands, and keep the leaf body for the rest.
- **Trailer:** becomes `Run '{{.CommandPath}} <command> --help' for details.` — root's
  `CommandPath()` is `brief`, so `rootHelp` is unchanged; under `new` it reads
  `brief new <command>`.
- **`newLong`** const in `new.go` beside `newFeatureLong`/`newStepLong`, set as `newCmd`'s
  `Long`; without it the group body's leading `{{.Long}}` emits a bare blank line. No
  `Short` on `newCmd` — root expands new's children in its place, so a Short would render
  nowhere and be untestable.

## Implementation Plan

- [x] Step 1: `internal/cli/help_test.go` — run the existing `rootHelp`/`startHelp` goldens and record the package test count before touching anything (baseline)
- [x] Step 2: `internal/cli/help_test.go` `newHelp` const + `Test_prints_new_help_listing_its_two_types` — table over `new --help`, `new -h`, `help new`; each asserts nil error, empty stderr, stdout equal to the `newHelp` literal (red: today `new --help` is an unknown-type usage error, `help new` renders the bare leaf shape)
- [x] Step 3: `internal/cli/help_test.go` `Test_help_topic_prints_the_same_bytes_as_the_command_help_flag` — add a `new` row (red: `new --help` returns a usage error today)
- [x] Step 4: `internal/cli/help_test.go` `Test_new_help_flag_is_help_only_as_the_sole_argument` — table: `new --help widget`, `new --help -x`, `new -h widget`, `new --help feature` (stays on `new` — see Traps) → exact one-line stderr literal, empty stdout, `ErrUsage`; plus `new -x --help` → `unknown type "-x"` (the other order) (green on arrival today — say so; it becomes load-bearing after step 6)
- [x] Step 5: `internal/cli/cli.go` `helpTemplate` — extract the root body into a `cmdList` define, select it by available subcommands instead of `HasParent`, trailer via `CommandPath`; re-run step 1's goldens byte-identical BEFORE continuing (refactor under green; whitespace-trim markers are the failure mode)
- [x] Step 6: `internal/cli/new.go` `runNew` — take the command; sole `-h`/`--help` → `cmd.Help()`; doc comment states the sole-argument rule (update)
- [x] Step 7: `internal/cli/new.go` `newLong` — new's one-sentence help prose const (new)
- [x] Step 8: `internal/cli/cli.go` `newRootCommand` — set `newCmd`'s `Long: newLong`, pass `cmd` to `runNew`; update the `newRootCommand` and `helpTemplate` doc comments to state the group/leaf rule and new's own help routing, no history (green)
- [x] Step 9: mutation checks, individually, via a WIP commit or `git stash push -u -m "s10-mutation-<n>"` + `git stash apply <sha>` (never bare stash/pop): (a) group selector back to `HasParent` → step 2 red, `rootHelp`/`startHelp` green; (b) delete the help intercept in `runNew` → step 2's `new --help`/`new -h` rows red, run_test.go:281 (`new -x`) green; (c) loosen the intercept to `args[0]` only → step 4's `--help widget` rows red. Restore and diff byte-identical after each
- [x] Step 10: `go build ./...`, `go test ./...` (unpiped, report count + delta), `go test -race ./internal/cli/...`, `golangci-lint run ./...`, `go doc ./internal/cli` reads cleanly
- [x] Step 11: all tests green → mark SCENARIO-10 done in specification.md; rewrite STATE.md (drop the "new's own help" Left-unbuilt entry)

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `helpTemplate` has two shapes: a `cmdList` group body for any command with available
  subcommands (root, `new`) and the leaf body for the rest — S11's tree-derived list and
  S12's `completion` row touch the same group body; `rootHelp` and `newHelp` goldens both
  guard it.
- Group trailer is `Run '{{.CommandPath}} <command> --help' for details.` — root's reads
  `brief <command>`, new's `brief new <command>`.
- `new` keeps `DisableFlagParsing` and owns its `-h`/`--help` routing inside `runNew`, as the
  **sole** argument only — S06/S09 strictness; run_test.go:281 (`new -x` → unknown type) depends
  on flag parsing staying off. `newCmd` has no `invocation` annotation; R14 never fires for it.
- `new`'s own `UseLine` is rendered nowhere (root expands its children; its group body skips
  it), so `newCmd` needs neither `DisableFlagsInUseLine` nor `Short`.
- Registration order now moves three literals together: `rootHelp`, `newHelp` (feature before
  step), and S09's stderr lists.

**Left unbuilt** — named so nobody assumes it exists:
- `new` row in `Test_every_command_help_has_a_usage_line_and_a_flag_table` — deliberately
  absent: the group shape has no `Usage:\n  brief new` prefix and no Flags table.
- Per-level help errors for `new` — still S09's one format string + `expectedCommands`.
- `brief new help` (help as a type) — still `unknown type "help"`; not a help alias.
- `<command>` vs `<type>` wording in new's trailer (runNew's errors say "type") — flag for the
  final product-vision pass.

**Traps** — things that look right and are not:
- `{{- if .HasParent}}Usage:` glues `Usage:` to the action; moving bodies into a `define` shifts
  newlines silently — prove `rootHelp`/`startHelp` byte-identical before any new-specific edit.
- `new --help feature` does NOT route to `feature`'s help (observed on the built binary): at
  `Find` time `new` has no `help` flag registered yet, so `stripFlags` takes `feature` as
  `--help`'s value and dispatch stays on `new` → `unknown type "--help"`, exit 2. Only
  `new feature --help` reaches feature's help.
- `new --help` reaching `RunE` is because `ParseFlags` early-returns under
  `DisableFlagParsing`; cobra's own help-flag check then sees `help=false`. Turning parsing on
  "to get help for free" breaks the `-x` unknown-type contract.
