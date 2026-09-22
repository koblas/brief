---
id: SCENARIO-07
status: done
---

# SCENARIO-07: Command help keeps its prose inside generated structure

## Scenario

```gherkin
Scenario: SCENARIO-07 Command help keeps its prose inside generated structure
  When I run "brief start --help"
  Then stdout has a Usage line, the existing start prose verbatim, and a flag table listing --json
  And there is no "Global Flags:" or "Additional help topics:" block, stderr is empty, and the exit code is 0
```

Rule R6; Product Verdict item 4. Context read: `STATE.md` only (no prior SCENARIO file
opened). Existence facts from `internal/cli/cli.go:20-217`, the six `*Usage` constants, and
cobra v1.10.2 / pflag v1.0.9 source (`HelpFunc`, `HelpTemplate`, `UseLine`,
`InitDefaultHelpFlag`, pflag `wrap`).

## User-visible contract

- `brief <leaf path> --help` / `-h` → stdout, stderr empty, exit 0, nil error. Layout, in order:
  `Usage:` + one indented line = `UseLine()` (from `Use`, no `[flags]` suffix); blank line;
  the command's prose (its `Long`) verbatim; blank line; `Flags:` + pflag's generated table
  (`LocalFlags().FlagUsages()`). Nothing else — no `Global Flags:`, `Additional help topics:`,
  `Available Commands:`, or cobra `Use "... --help" for more` trailer.
- `-h, --help` row: cobra's own default, `help for <name>` (e.g. `help for start`,
  `help for feature`). Chosen deliberately — registering our own help flag would suppress
  `InitDefaultHelpFlag` and void STATE's "`-h` identical per leaf" premise behind S03/S04/S06.
- `Use` carries today's argument syntax: `start [--json] <feature>`, `status`,
  `check [feature]`, `finish <feature> <step> --handoff <path> --state <path>`,
  `feature <name>`, `step <feature>`. So each generated Usage line equals today's
  hand-written one.
- Flag usage strings carry today's flag paragraphs (start `--json`; finish `--handoff`,
  `--state`), wording unchanged, embedded newlines allowed. finish's two strings name their
  value with a backquoted `path`, so the table reads `--handoff path` / `--state path`, not
  `string`. pflag re-indents embedded newlines to the table's description column — that
  indentation is accepted as generated, not treated as a prose change.
- Root help (`brief --help`, `brief -h`, `brief help`) → stdout, exit 0, byte-identical across
  the three. Layout: root `Long` (the one-sentence description line); blank line; `Usage:`;
  one line per leaf, the leaf's `.UseLine` padded to a fixed description column, then that
  leaf's `Short`. Because the row is `.UseLine`, start's row becomes
  `brief start [--json] <feature>` (today: `brief start <feature>`) — a deliberate, visible
  change, consistent with finish's row already showing its flags; the description column
  widens to fit it. Flag it for the final product-vision pass — `new` expands to its two children (`new feature`, `new step`), so today's
  six entries all survive; an entry too long for the column (finish) puts its `Short` on the
  next line at that same column (single column for both lines — today's off-by-one between
  the finish continuation and the other rows is dropped); blank line;
  `Run 'brief <command> --help' for details.`
- Root listing order is today's: `new feature`, `new step`, `start`, `status`, `check`,
  `finish`. Achieved by turning off cobra's alphabetical sort — `cobra.EnableCommandSorting =
  false`, set once at package init in `internal/cli` (never inside `Run`/`newRootCommand`:
  a per-call write races parallel tests' reads) — and registering root's children in that
  order (`new, start, status, check, finish`). `new`'s children stay `feature, step`.
- `Short`s are today's listing descriptions, verbatim.
- Unchanged, deferred: `brief help <topic>` still prints root help (S08/S09);
  `brief new --help` still a `runNew` usage error (S10).

## Implementation Plan

- [x] Step 1: `internal/cli/help_test.go` `Test_prints_start_help_as_usage_line_prose_and_flag_table` — `cli.Run(start --help)`; stdout equals a hand-written literal golden (Usage line, full start prose, Flags table with `-h, --help` and the `--json` paragraph); stderr empty; nil error (red)
- [x] Step 2: `internal/cli/help_test.go` `Test_prints_the_root_help_with_one_line_per_command` — `brief --help` stdout equals a hand-written literal golden per the contract above; table asserts `brief -h` and `brief help` are byte-identical to it (red)
- [x] Step 3: `internal/cli/help_test.go` `Test_every_command_help_has_a_usage_line_and_a_flag_table` — table over all six leaves: stdout begins `Usage:\n  brief <path>`, contains `Flags:` and `-h, --help`, contains none of `Global Flags:` / `Additional help topics:` / `Available Commands:`; stderr empty; nil error (red)
- [x] Step 4: `internal/cli/help_test.go` `Test_prints_finish_flag_prose_in_its_flag_table` — finish `--help` stdout contains the `--handoff path` and `--state path` rows and the `COMPLETE replacement body` prose (red)
- [x] Step 5: `internal/cli/cli.go` `helpTemplate` — one template const, branching on `{{if .HasParent}}`: leaf branch = Usage/UseLine, trimmed Long, Flags/FlagUsages; root branch = Long, listing over available commands (descending one level into `new`), trailer line; only cobra's built-in template funcs + `text/template` builtins — no `cobra.AddTemplateFunc` (package-global) (new)
- [x] Step 6: `internal/cli/cli.go` `newRootCommand` — delete `root.SetHelpFunc(...)` (it short-circuits any template: `HelpFunc()` returns it before reaching the template) and set `root.SetHelpTemplate(helpTemplate)`, inherited by every child via `HelpTemplate()`'s parent walk; root `Long` becomes the description sentence only; delete the `usage` const (update)
- [x] Step 7: `internal/cli/cli.go` `runRoot` + root `RunE` + hidden help stub — thread the `*cobra.Command`; the `-h`/`--help`/`help` branch and the stub both render via `cmd.Help()` / `cmd.Root().Help()` so every root-help path is one render (update)
- [x] Step 8: `internal/cli/cli.go` `leafCommand` — add a `short` parameter, set `DisableFlagsInUseLine: true`; each call site passes `Use` with its argument syntax and its `Short`; `invocation` annotation arguments unchanged literals (update)
- [x] Step 8a: `internal/cli/cli.go` — package-init `cobra.EnableCommandSorting = false` (doc comment: root help lists commands in workflow order) and reorder `root.AddCommand` to `new, start, status, check, finish` (update)
- [x] Step 9: `internal/cli/cli.go` flag registrations — `--json`, `--handoff`, `--state` usage strings take the prose of today's flag paragraphs verbatim (finish's with backquoted `path`) (update)
- [x] Step 10: `start.go`, `finish.go`, `status.go`, `check.go`, `new.go` — rename each `*Usage` const to `*Long` and strip it to prose only (drop the hand-written `Usage:` block and flag paragraphs; every other byte verbatim); doc comments say "is ... 's help prose"; fix the stale `startUsage` mention in `start_test.go`'s `Test_start_json_help_prints_usage_not_json` comment (green)
- [x] Step 11: golden reconciliation — run Steps 1-2; a diff that is only pflag's column/indentation for multi-line flag usage or `FlagUsages` alignment is accepted into the literal; a diff missing or altering a prose word is a code bug, fix the code (green)
- [x] Step 12: pin-safety — the must-pass-unedited pins stay untouched and green: `check_test.go:212` (`brief check` — generated Usage line), `finish_test.go:503` (finish opening prose line), `status_test.go:340-341` (`!`, `exits 0`), `start_test.go:473,487,731` (`brief start reads; it never writes.` — prose above the moved `--json` paragraph); S06's `Test_prints_help_for_the_h_shorthand_alone` `-h`≡`--help` control; every S02-S06 row in `flag_error_test.go` (invocation annotations unchanged). Any of these reddening means prose or an annotation moved (verify)
- [x] Step 13: mutation-verify individually (stash with a unique `-m` tag, capture SHA, `git stash apply <sha>`, drop by tag — never bare stash/pop): (a) restore a `SetHelpFunc` printing `cmd.Long` → Step 1/3 red; (b) remove `DisableFlagsInUseLine` → Step 1 red (`[flags]`); (c) drop one sentence from `--json`'s usage string → Step 1 red; (d) swap one leaf's `Short` → Step 2 red; (d2) remove the `EnableCommandSorting` init → Step 2 red (alphabetical order); (e) drop the `IsAvailableCommand` filter from the root listing → Step 2 red (hidden `help`/`__complete*` listed). Report which test each reddened (verify)
- [x] Step 14: `go build ./...`, `go test ./...` (unpiped; report count + delta), `go test -race ./internal/cli/...`, `golangci-lint run ./...` → mark SCENARIO-07 done in `specification.md`, fold the Handoff into `STATE.md` (replace STATE's "Help is one root `SetHelpFunc` printing `cmd.Long` verbatim" decision and the "Generated help template / `Short` / flag usage strings — S07" Left-unbuilt line)

Note on negative assertions (Step 3): with no persistent flags and no non-runnable children,
cobra's default template would not emit `Global Flags:` / `Additional help topics:` on a leaf
either — those `NotContains` are documentation, not proof. The proof is the byte-exact
goldens (Steps 1-2) and mutations (a)-(e). Do not claim more in test comments.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Help is one `helpTemplate` set on root via `SetHelpTemplate`, inherited by all children; no
  `SetHelpFunc` anywhere — a HelpFunc on any ancestor silently bypasses the template.
- Every help path renders through `cmd.Help()` (root: `runRoot`'s help branch and the hidden
  stub call it too) — S08's `help <cmd…>` byte-identity is `target.Help()`, nothing more.
- `Use` carries arg syntax; leaves set `DisableFlagsInUseLine`. `Annotations[invocation]`
  stays a hand-written literal, never derived from `UseLine()` — S02-S06 pin
  `run 'brief start <feature>'` (no `[--json]`), which `UseLine()` would change.
- `-h, --help` row is cobra's default `help for <name>`; no leaf registers its own help flag
  (S03/S04/S06 rows assume `InitDefaultHelpFlag` per leaf).
- Root listing = each leaf's `.UseLine` + `Short`, in registration order (`new feature, new
  step, start, status, check, finish`; `cobra.EnableCommandSorting = false` at package init),
  `new` expanded to its children, finish wrapped at the same column, trailer
  `Run 'brief <command> --help' for details.` Pinned byte-exact in `help_test.go`. Start's
  row now shows `[--json]` (from `.UseLine`) — flagged for final product-vision.
- `Long` = prose only; flag paragraphs live in flag usage strings. The six Contains pins
  depend on that split.

**Left unbuilt** — named so nobody assumes it exists:
- `brief help <topic>` rendering the topic — S08 (stub still prints root help).
- `brief help bogus` usage error — S09.
- `new`'s own help (`brief new --help` still a `runNew` usage error; `new` has no `Short`) — S10.
- Tree-derived `expected one of:` in `runRoot` — S11 (literal unchanged here).

**Traps** — things that look right and are not:
- With sorting off, S11's tree-derived `expected one of:` list comes out in registration
  order `new, start, status, check, finish` — today's literal is `new, start, finish, status,
  check`. S11 must pin whichever it ships; reordering `AddCommand` also moves root help.
- `EnableCommandSorting` is a cobra package global: set it once at init, never per `Run`.
- pflag `FlagUsages()` (wrap width 0) re-indents embedded newlines in a usage string to the
  description column — golden indentation is pflag's, not the constant's.
- Hidden `help` stub and cobra's `__complete*` exist in `Commands()`; the root listing must
  filter on `IsAvailableCommand`, not by name.
- `cobra.AddTemplateFunc` mutates a package global — use only built-in template funcs.
