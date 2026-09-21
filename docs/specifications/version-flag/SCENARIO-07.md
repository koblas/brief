# SCENARIO-07: Root help points to --version

## Scenario

```gherkin
Scenario: SCENARIO-07 Root help points to --version
  When I run "brief --help"
  Then its last line is "Run 'brief --version' to print the installed version."
  And it comes right after "Run 'brief <command> --help' for details."
```

Rule R7: root help gains exactly one trailer line after
`Run 'brief <command> --help' for details.` — `Run 'brief --version' to print the installed version.`

## User-visible contract

- `brief --help`, `brief -h`, `brief -hh`, `brief help` → stdout = `rootHelp` whose last two
  lines are `Run 'brief <command> --help' for details.` then
  `Run 'brief --version' to print the installed version.`; stderr empty; exit 0.
- `brief new --help` / `new -h` / `help new` → stdout = `newHelp` byte-identical to today
  (footer `Run 'brief new <type> --help' for details.` stays last); stderr empty; exit 0.
- Every leaf `--help` (`startHelp`, `finishHelp`, the 80-column sweep, the help-flag/topic
  equivalence table) → byte-identical to today.
- No new failure classes: this scenario only changes the bytes of a successful render.

## What exists (no step needed)

- `helpTemplate` (`internal/cli/cli.go:108`) — one template for the whole tree; the
  `"cmdList"` block is shared by root AND `new`. There is no root-only branch today; the
  trailer must be conditioned inside `cmdList` on the command being the root.
- `-hh` at root already renders the same bytes as `--help`
  (`Test_prints_help_for_the_hh_cluster_alone`, `flag_error_test.go:742`, live comparison).
- `newHelp`, `startHelp`, `finishHelp` goldens and
  `Test_every_leaf_help_line_fits_in_80_columns` (`help_test.go`) — unchanged by this plan;
  they are the guards.

## Implementation Plan

- [x] Step 1: `internal/cli/help_test.go` `rootHelp` — append the R7 trailer line as the golden's last line, after the `for details.` line; update the const's doc comment to name both trailer lines (red — `Test_prints_the_root_help_with_one_line_per_command` fails on all rows)
- [x] Step 2: `internal/cli/help_test.go` `Test_prints_the_root_help_with_one_line_per_command` — add a `-hh` row so every root-help entry point is pinned against the literal golden, not only by live comparison; refresh its doc comment (update, red with Step 1)
- [x] Step 3: `internal/cli/cli.go` `helpTemplate` `"cmdList"` block — emit the `--version` trailer line after the `for details.` line gated on `not .HasParent` (cobra `*Command.HasParent`; no `AddTemplateFunc`), text `Run '{{.CommandPath}} --version' to print the installed version.` so it matches the sibling trailer's style; use trim markers so the two trailers are byte-adjacent (no blank line between) and the render ends in exactly one `\n` — at root and at `new` (green)
- [x] Step 4: `internal/cli/cli.go` `helpTemplate` doc comment — state that root's group body ends with the `--version` pointer and that `new`'s and every leaf's do not (update)
- [x] Step 5: confirm unchanged, without editing them: `newHelp` golden (`Test_prints_new_help_listing_its_two_types`), `startHelp`/`finishHelp` goldens, `Test_help_topic_prints_the_same_bytes_as_the_command_help_flag`, `Test_prints_help_for_the_hh_cluster_alone`, `Test_every_leaf_help_line_fits_in_80_columns` all green on arrival. The other live root-help captures (`run_test.go` `Test_prints_usage_to_stdout_when_help_is_requested_for_the_binary`, `Test_prints_root_usage_and_a_nil_error_for_brief_help`, `Test_still_prints_root_help_for_a_bare_help_flag`) assert only `NotEmpty` — checked while planning, unaffected (verify)
- [x] Step 6: mutation A — stash `cli.go`, drop the root-only condition so the trailer renders for every `cmdList` command → `Test_prints_new_help_listing_its_two_types` must go red (all three rows); restore, prove byte-identical (verify)
- [x] Step 7: mutation B — stash `cli.go`, move the trailer to the template's tail outside both branches → `Test_prints_start_help_as_usage_line_prose_and_flag_table` (startHelp golden) is the claimed red — report that test by name, not "the suite reds" (rootHelp/newHelp/finishHelp also redden); restore, prove byte-identical (verify)
- [x] Step 8: `go build ./...`, `go test ./...` (unpiped, report count + delta — expect +1 subtest for the `-hh` row), `go test -race ./internal/cli/...`, `golangci-lint run ./...` → mark SCENARIO-07 done in `specification.md`; rewrite `STATE.md` (remove the "Left unbuilt: root-help trailer" and "Open debts" entries)

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- The `--version` trailer lives inside `helpTemplate`'s shared `"cmdList"` block, gated on
  `not .HasParent` — `cmdList` also renders `new`, and R7/R6 require `newHelp`
  byte-identical (its footer `Run 'brief new <type> --help' for details.` stays last).
  Rejected: `eq .CommandPath "brief"` (hardcodes the binary name) and keying on the absence
  of `commandNounAnnotation` (breaks when a future group command omits the annotation).
- The trailer is root help's last line, immediately after `Run 'brief <command> --help' for
  details.` — the Gherkin asserts both "last line" and adjacency; `rootHelp` pins both.
- No package-global template func is added; the template keeps to cobra's built-in funcs,
  `*Command` methods and text/template builtins, as its doc comment already promises.

**Left unbuilt** — named so nobody assumes it exists:
- No `--version` pointer in `new`'s help, any leaf's help, or the help stub's own `-h`
  render — deliberate (R7 is root-only).
- `brief` with no arguments stays a usage error on stderr (`no command given; expected one
  of: ...`), not root help — it does not gain the trailer.

**Traps** — things that look right and are not:
- Appending the line after `{{template "cmdList" .}}` or at the template tail: the first
  leaks into `newHelp`, the second into every leaf golden. Mutations A and B (Steps 6-7) are
  the proofs; both must be run individually.
- An untrimmed `{{if}}`/`{{end}}` around the line adds a blank line between the trailers or
  after the last one (and can add a stray `\n` to `newHelp`); a golden diff only says bytes
  differ, so check trim markers first.
- Golden comments: `rootHelp`'s doc says "and the trailer" (singular) and `newHelp`'s doc
  contrasts itself with root's trailer — keep both accurate after the change.
- `Test_every_leaf_help_line_fits_in_80_columns` excludes root and `new` by design; it
  proves nothing about the new line. The line is 53 columns; `rootHelp` pins it exactly.
- `-hh` at root was only pinned by live equivalence to `--help`; a mutation that alters
  both identically passes it. Step 2's golden row closes that for root.
