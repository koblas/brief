---
id: SCENARIO-13
status: done
depends-on: []
---

# SCENARIO-13: help --json is a machine-readable command index

## Scenario

```gherkin
Scenario: SCENARIO-13 help --json is a machine-readable command index
  When I run "brief help --json" or "brief --help --json"
  Then the document lists commands[] with name, usage, summary, description, flags[]
  And "brief help start --json" and "brief start --help --json" return the same document filtered to start
  And "brief completion bash --json" is a usage error document
```

## User-visible contract

- Payload (product-vision ruling): `{schema, command, ok, exit_code, commands:[{name, usage,
  summary, description, flags:[{name, type, usage}]}]}` on stdout, stderr empty, exit 0.
- `command` header is the literal `"help"` for every help document, whatever spelling asked:
  `command` is what scripts dispatch on for payload shape, and `"start"` already names
  `start --json`'s brief document — a help document must not share that value.
- Full index: `brief help --json`, `brief --help --json`, `brief -h --json`, `brief --json --help`.
  `commands[]` = depth-first, registration order: `new`, `new feature`, `new step`, `start`,
  `finish`, `status`, `check`, `completion`. Root and the `help` stub are not entries. This is
  root-help row order with `new` itself inserted ahead of its children (root help shows only
  the children's rows).
- Filtered (exactly one entry, the command whose help was asked for):
  `help start --json` ≡ `start --help --json` ≡ `start -h --json`;
  `help new --json` ≡ `new --help --json` → one entry, `new` itself (not its children);
  `help new feature --json` ≡ `new feature --help --json`;
  `help completion --json` ≡ `completion --help --json`;
  `help -h --json` → one entry, `help` (the stub's own Use/Short/Long) — the filter is "the
  command asked about", not the index-membership predicate.
- Entry fields: `name` = `commandName(cmd)` (`"new feature"`); `usage` = `cmd.UseLine()`
  (`"brief start [--json] <feature>"`, `"brief new"` — see Step 5a); `summary` = `Short`; `description` = `Long`
  verbatim (embedded newlines kept); `flags` = `cmd.LocalFlags()` via `VisitAll` (the same set
  `helpTemplate` renders), hidden flags skipped, `-h/--help` included (`InitDefaultHelpFlag`
  called on every emitted command); flag `name` = pflag `Name` without dashes, `type` =
  `Value.Type()` (`"bool"`, `"string"`), `usage` = `pflag.UnquoteUsage`'s usage (backquote
  varname markers stripped, newlines kept). `commands` and `flags` never `null`.
- Unknown topic under `--json` (`help bogus --json`) is already a usage-error document
  (`json_usage_test.go` row `help bogus --json`) — green on arrival, no step.
- R11: `brief completion <valid shell> --json` → usage-error document, exit 2, stderr empty,
  `error.message` = `brief completion: completion prints a shell script; --json does not apply`,
  `error.fix` = `run 'brief completion <bash|zsh|fish|powershell>'` (usageFix fallback),
  `files_changed` null, and **zero script bytes on stdout**. `completion --json` (no shell)
  and `completion nosh --json` keep their S01 text-mode messages (guard sits on the
  resolved-shell branch only).
- Text mode is byte-identical to today for every help spelling.

## Implementation Plan

Tests go through `cli.Run` (black-box `cli_test`) in a new `internal/cli/help_json_test.go`.
All code stays in `internal/cli` — this is delivery-layer metadata about the cobra tree
(precedent: `versionDocument`); no `internal/help` feature package.

- [x] Step 1: `internal/cli/cli.go` `newRootCommand` — before building on it, confirm `root.HelpFunc()` captured before `SetHelpFunc` is cobra's default closure rendering the `*Command` it is handed (existing help goldens in `help_test.go` are the oracle); if not, stop and report (update)
- [x] Step 2: `help_json_test.go` `Test_help_json_lists_every_listed_command_and_new_itself` — full index via `help --json`: header (`command` "help", exit 0), names in the order above; control arm: hidden `completion` IS present while `help` and `brief` are absent (red)
- [x] Step 3: `help_json_test.go` `Test_help_json_entries_agree_with_each_commands_text_help` — per leaf entry (`new` renders the group template, which has no single `Usage:` line or flag table; its entry is checked by name/usage `brief new` in Step 6), `usage` equals the `Usage:` line and `flags[].name` equals the flag-table rows of that command's own text `--help` captured in the same test (never literals); every `summary`/`description` non-empty, `flags` non-null (red)
- [x] Step 4: `internal/cli/help_json.go` — `helpDocument{jsonHeader; Commands}`, `helpCommandJSON`, `helpFlagJSON`, the index builder (membership = `IsAvailableCommand() || listedInHelpAnnotation`, recursive, root excluded) and the one-entry builder, header from `newJSONHeader("help", 0)`; per entry the builder calls `InitDefaultHelpFlag()` BEFORE reading `UseLine()`/`LocalFlags()` (new)
- [x] Step 5: `internal/cli/cli.go` `newRootCommand` — `root.SetHelpFunc` wrapper: JSON mode → write root's full index when cmd is root, else the one-entry document for cmd; text mode → captured default func (green)
- [x] Step 5a: `internal/cli/cli.go` `newCmd` — set `DisableFlagsInUseLine: true` (every leaf and the stub already have it) so `new`'s `usage` is `brief new`, not `brief new [flags]` once its help flag exists; no text-help effect, since `new`'s UseLine renders nowhere in text (update)
- [x] Step 6: `help_json_test.go` `Test_help_json_spellings_produce_identical_documents` — table of spelling groups (full index ×4; start ×3; new ×2; new feature ×2; completion ×2): byte-equal stdout within each group, empty stderr, exit 0, `command` == `"help"` in every group; `new` group yields exactly one entry named `new` with usage `brief new`; `help -h --json` yields one entry named `help` (green on arrival after Step 5 — confirmed)
- [x] Step 7: `help_json_test.go` `Test_help_finish_json_is_the_exact_document` — exact-bytes golden (`assert.Equal`, literal) for `help finish --json`; chosen over `start` because it pins string type, backquote stripping (`path`), multi-line usage and the help-flag row in one document (green on arrival, confirmed)
- [x] Step 8: `json_usage_test.go` — delete `Test_json_stripped_from_help_topic_arguments_runs_the_ordinary_help_path` (it pins the pre-S13 text path under `--json`); keep `help --json start` as a row in Step 6's start group (update)
- [x] Step 9: `help_test.go` — every text-mode help test passes unchanged; no edits made
- [x] Step 10: `help_json_test.go` `Test_completion_with_a_shell_under_json_is_a_usage_error_document` — rows for each of bash/zsh/fish/powershell: exit 2, `ErrUsage`, stderr empty, stdout is exactly one usage-error document with the message/fix above (no script bytes precede it); control arm: same shell without `--json` writes a non-empty script, exit 0 (red)
- [x] Step 11: `internal/cli/completion.go` `runCompletion` — on the resolved-shell branch, `out.json` → `out.usageError(<R11 line>)` before `s.gen`; named as `completionJSONUnsupportedMessage` (green)
- [x] Step 12: `internal/cli/cli.go` / `doc.go` / `completion.go` doc comments — stated the help-document rule and R11 on `newRootCommand`, `newHelpCommand`, `runCompletion`, and `doc.go`'s package comment (update)
- [x] Step 13: mutation-verify (green), one at a time, stashed via cp/diff per agent-briefs: (a) membership predicate narrowed to `IsAvailableCommand()` only → Step 2 red on `completion` (confirmed); (b) drop `InitDefaultHelpFlag` from the entry builder → neither Step 3 nor Step 6 caught this (both reach the entry via a path cobra/newHelpCommand already calls `InitDefaultHelpFlag` on); strengthened Step 2 with a "status" full-index entry's own flags assertion, which does catch it (confirmed red, then restored green); (c) raw `flag.Usage` instead of `UnquoteUsage` → Step 7 red (confirmed); (d) header built from `commandName(c)` instead of the "help" literal → Step 6 `command` assertion red (confirmed); (g) drop Step 5a → Step 6 `new` usage red (confirmed); (e) R11 guard moved to the top of `runCompletion` → S01 rows `completion --json` / `completion nosh --json` red (confirmed); (f) R11 guard removed → Step 10 red (confirmed)
- [x] Step 14 (green): `go build ./...`, `go test ./...` (unpiped, 216 top-level tests in `internal/cli`, 0 skip, 0 fail — up from 211 before this scenario: +6 new in `help_json_test.go`, -1 deleted pre-S13 pin in `json_usage_test.go`), `go test -race ./internal/cli/...`, `golangci-lint run ./...` (0 issues) → marked SCENARIO-13 done in specification.md, rewrote STATE.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Help documents come from one `root.SetHelpFunc` wrapper, not per-call-site branches — root
  `--help`, the help stub, `runNew`'s sole-help arm and every leaf `--help` all reach
  `cmd.Help()`/HelpFunc; four branches would drift and the ≡ tests would catch only some.
- Help document `command` is the literal `"help"`, built with `newJSONHeader("help", 0)`, not
  from the described command — scripts dispatch on `command` for payload shape, and
  `"start"` already names start's brief document. A deliberate exception to S09's
  header-from-`r.cmd` convention (the wrapper's base reporter has `cmd == nil` anyway).
- Index membership = `IsAvailableCommand() || listedInHelpAnnotation` (the root-help row
  predicate); root and the `help` stub are never index entries; `new` IS an entry.
- Filter = exactly the command asked about, even when it is not index-eligible (`help -h
  --json` → one `help` entry). Membership and filter are two different predicates.
- `flags[]` derives from `cmd.LocalFlags()` — the set `helpTemplate` renders — so the table and
  the index cannot disagree. `-h/--help` is listed; `InitDefaultHelpFlag` runs per entry.
- R11 line `brief completion: completion prints a shell script; --json does not apply` is the
  product-vision literal behind the house `brief <cmd>: ` prefix — interpreted, not verbatim;
  the final product-vision pass may reword it.
- R11 guard sits on `runCompletion`'s resolved-shell branch only; S01 rows `completion --json`
  and `completion nosh --json` keep their text-mode messages.

**Left unbuilt** — named so nobody assumes it exists:
- `--version` never appears in the index: root is not an entry (it has `DisableFlagParsing`;
  `--help`/`--version` are hand-classified, not pflags).
- `--json` rows in `flags[]` for commands other than `start` — S14. S14 must register `--json`
  as a real pflag per command (`fs.Bool("json", false, …)`, as `start` does), NOT by editing
  `helpTemplate`; the index then picks it up automatically. The STATE.md trap about
  registering `--json` on every leaf concerns reading its value / root's `DisableFlagParsing`
  — pre-dispatch stripping makes leaf registration safe (start proves it). `root` and `new`
  (flag parsing disabled) need their help rows some other way.
- A flag `shorthand` field — not in the ruled shape.

**Traps** — things that look right and are not:
- Capture `root.HelpFunc()` before `SetHelpFunc`; calling `root.HelpFunc()` inside the wrapper
  recurses forever.
- HelpFunc returns nothing and `cmd.Help()` always returns nil, so a `writeJSONDocument` error in
  help mode cannot surface as a non-zero exit — same as cobra's own text help today.
- Cobra only calls `InitDefaultHelpFlag` on the executed command; skipping it in the builder
  makes the full index lack `help` rows the filtered document has.
- Raw `flag.Usage` carries pflag's backquote varname markup (`` `path` ``); use `UnquoteUsage`.
- `new` was the only command without `DisableFlagsInUseLine`; its `UseLine()` gains ` [flags]`
  the moment `InitDefaultHelpFlag` runs — order-dependent until Step 5a.
- `commandName(r.cmd)` differs per spelling (`help`/`brief`/`start`) — never use it for the
  help header.
