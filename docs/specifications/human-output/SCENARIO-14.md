# SCENARIO-14: Every command's help advertises --json

## Scenario

```gherkin
Scenario: SCENARIO-14 Every command's help advertises --json
  When I run any "<cmd> --help"
  Then it lists "--json  print one JSON document on stdout" and the command's top-level JSON fields
  And status and check help say "For scripts, use --json; the text layout may change."
```

## User-visible contract

- Leaves that get the row: `new feature`, `new step`, `start`, `finish`, `status`,
  `check`, and the help stub (`brief help -h`, because `brief help --json` has a success
  document). Each `<cmd> --help` Flags table carries one row, sorted by name among the other
  rows: `--json   print one JSON document on stdout`. Exit 0, stdout only, stderr empty.
- Each of those commands' `Long` ends with one JSON paragraph, hand-wrapped to 80 columns
  or less. It names the common header once, the same way every time (`schema`, `command`,
  `ok`, `exit_code`; on a usage error or refusal an `error` object carries the failure). It
  then names that command's own top-level keys in document order:
  - `start`: `done`, `open`, `step`, `inherited`, `shortfalls`. This paragraph also takes the
    two facts the old flag usage carried: `step` is null when no step is open, and `--json`
    may come before or after `<feature>`.
  - `finish`: `feature`, `step`, `changed`, `handoff_path`, `state_path`, `next`
  - `new feature` / `new step`: `feature`, `step`, `path`, `created`
  - `status`: `features`
  - `check`: `counts`, `features`
  - help stub: `commands`
- `statusLong` and `checkLong` end with exactly
  `For scripts, use --json; the text layout may change.` No other command carries it.
- No `--json` row and no JSON paragraph: `completion` (R11: `completion <shell> --json` is a
  usage error), root (`brief --help`), and bare `new` (`brief new --help`). See Handoff.
- The paragraph is part of `Long`, so `help --json`'s `description` field grows with it. The
  new flag also shows up in every affected entry's `flags[]`, because S13 reads flags from
  pflag.
- Use lines do not change. `start [--json] <feature>` stays as it is, so `rootHelp` and the
  help index `usage` fields keep their current values.

## Implementation Plan

- [x] Step 1: `internal/cli/help_test.go` `Test_every_command_help_lists_the_json_flag_row` — table over the seven commands above. Assert the Flags section contains a `--json` row whose usage is the ruled line; this is whitespace-tolerant between name and usage. The `completion --help` control arm must NOT contain `--json` (red)
- [x] Step 2: `internal/cli/help_test.go` `Test_every_command_help_names_its_json_documents_top_level_fields` — for each command, run a real `--json` invocation (fixtures `newStartFixture`, `newFinishCLIFixture`, `newStatusJSONFixture`, `newCheckJSONFixture`; `new feature` then `new step` in the same `t.TempDir()`, because `new step` needs the feature; `help --json`). Each row states its expected exit: `check --json` exits 1 whenever its fixture yields an ERROR finding (R4), so do not `require.NoError` uniformly. Collect top-level keys with `jsonKeys`, drop the header keys, then slice the `--help` text from the builder's constant opening marker (Step 7) to the end. Assert each remaining key appears as a whole word inside that slice. The key list comes from the live document and is never a literal (red)
- [x] Step 3: `internal/cli/help_test.go` `Test_status_and_check_help_say_the_text_layout_may_change` — `status` and `check` contain the ruled sentence. The control arm: `start` and `finish` help do not (red)
- [x] Step 4: `internal/cli/cli.go` `jsonFlagUsage` — replace the paragraph const with the one ruled line, and add one registration helper that every JSON-capable command calls. `leafCommand` itself does NOT register it, because `completion` goes through `leafCommand` (update)
- [x] Step 5: `internal/cli/cli.go` `newRootCommand` — call the helper from the `addFlags` of `new feature`, `new step`, `status` and `check`, which change from nil to a func, and of `finish` and `start`. `completion` keeps nil (update)
- [x] Step 6: `internal/cli/cli.go` `newHelpCommand` — register the same flag on the help stub's FlagSet. The stub keeps `DisableFlagParsing`, so this is for display only (update)
- [x] Step 7: `internal/cli/cli.go` — one shared builder for the JSON paragraph, with the header clause written once and each caller passing its own key list. The paragraph opens with a constant sentence, a named const, which Step 2 uses as its slice marker. No regex guessing. Update doc comments on `jsonFlagUsage` and `handoffFlagUsage`, which currently cite `jsonFlagUsage` as the wrapping precedent (new)
- [x] Step 8: `startLong` (`start.go`), `finishLong` (`finish.go`), `statusLong` (`status.go`), `checkLong` (`check.go`), `newFeatureLong` / `newStepLong` (`new.go`), `helpLong` (`cli.go`) — add each command's JSON paragraph. `startLong` takes over the "step is null" and "before or after `<feature>`" facts. `statusLong` and `checkLong` also get the "For scripts" sentence. Steps 1–3 go green (green)
- [x] Step 9: `internal/cli/start.go` `runStart` doc comment — keep the "registered only for its help row; `scanJSONFlag` strips it" note, and widen it to every leaf (update)
- [x] Step 10: `internal/cli/help_test.go` `startHelp` golden — the `--json` row shrinks to the ruled line and the prose grows by the paragraph (update)
- [x] Step 11: `internal/cli/help_test.go` `finishHelp` golden — rows become `--handoff`, `-h, --help`, `--json`, `--state` (pflag sorts them), plus the paragraph. Fix its doc comment, which mentions `jsonFlagUsage` wrapping (update)
- [x] Step 12: `internal/cli/help_test.go` `helpHelp` golden — `--json` row and paragraph (update)
- [x] Step 13: `internal/cli/help_json_test.go` `Test_help_finish_json_is_the_exact_document` — add `{"name":"json","type":"bool","usage":"print one JSON document on stdout"}` between `help` and `state`, and use the new `description` (update)
- [x] Step 14: `internal/cli/help_json_test.go` `Test_help_json_lists_every_listed_command_and_new_itself` — `status.Flags` becomes length 2 (`help`, `json`). Keep the `help` row equality on `[0]` and add the `json` row on `[1]` (update)
- [x] Step 15: `Test_every_leaf_help_line_fits_in_80_columns` is a gate, not noise. It now covers seven new hand-wrapped paragraphs and must stay green. Confirmed green on arrival, no manufactured red: `Test_help_json_entries_agree_with_each_commands_text_help`, `Test_every_command_help_has_a_usage_line_and_a_flag_table`, `Test_help_topic_prints_the_same_bytes_as_the_command_help_flag`, `rootHelp` and `newHelp` all stayed green unmodified. `Test_help_json_spellings_produce_identical_documents` pins no start `usage`/`description` literal (only `doc.Commands[0].Usage == "brief new"` for the "new" group), so it needed no update
- [x] Step 16: mutation-verified each case individually, backing up the touched file to `$TMPDIR` and diffing byte-identical after restore (never bare `git stash`). (a) removed `addJSONFlag` from `status`'s `leafCommand` call: only `Test_every_command_help_lists_the_json_flag_row/status` went red, every other subtest (including `completion_carries_no_--json_row`) stayed green. (b) added `addJSONFlag` to `completion`'s `leafCommand` call: only `Test_every_command_help_lists_the_json_flag_row/completion_carries_no_--json_row` went red. (c) dropped `"shortfalls"` from `startLong`'s `jsonFieldsParagraph` call: only `Test_every_command_help_names_its_json_documents_top_level_fields/start` went red. (d) removed `+ " " + jsonScriptHint` from `checkLong`: only `Test_status_and_check_help_say_the_text_layout_may_change/check` went red. (e) appended `jsonScriptHint` to `startLong`: only `Test_status_and_check_help_say_the_text_layout_may_change/start` went red
- [x] Step 17: `go build ./...` clean; `go test ./...` (unpiped) — all 8 tested packages `ok`, `internal/cli` 0 skips; `go test -race ./internal/cli/...` — `ok`; `golangci-lint run ./...` — `0 issues`. `internal/cli` went from (pre-scenario baseline, not separately captured) to 219 passing `--- PASS` lines under `-v`, +22 from this scenario's three new top-level tests (3 parents + 19 subtests). SCENARIO-14 marked done in `specification.md`

## Handoff

**Binding decisions:**
- `--json` is registered as a real pflag on `new feature`, `new step`, `start`, `finish`,
  `status`, `check` and the help stub, all through one helper that uses one usage const
  (`print one JSON document on stdout`). If a future command writes its own wording, the help
  index `usage` values silently drift apart.
- Registration only affects display. `scanJSONFlag` still strips every `--json` before
  pflag runs, and no command reads `cmd.Flags().GetBool("json")`. The STATE trap "registering
  on every leaf instead of stripping centrally breaks `brief --json`" still holds: stripping
  remains the mechanism.
- `leafCommand` does not register `--json` itself. `completion` is built through it, and R11
  forbids advertising `--json` there.
- Root and bare `new` get no row and no paragraph. Neither renders a Flags table (the
  `cmdList` branch). Bare `new --json` has no success document, and root's only one is
  `--version`'s, so neither can meet the "top-level fields" clause. The `rootHelp`/`newHelp`
  goldens are unchanged.
- Field paragraphs live in each command's `Long`, from one shared builder, with header keys
  named once in the same wording everywhere. Only top-level keys are named. Because `Long` is
  the help-document `description`, the paragraph is also JSON-visible.
- The "For scripts, use --json; the text layout may change." sentence is on `status` and
  `check` only.
- Use lines are unchanged. `start [--json] <feature>` is the only one that mentions `--json`.

**Left unbuilt:**
- A JSON-mode hint in root or `new` group help — nobody owns it.
- `brief new --json` success document (still left unbuilt from S13).

**Traps:**
- pflag sorts Flags rows by name, so `--json` goes between `--help` and `--state` in finish's
  table and in its help-JSON `flags[]`. Don't append it at the end.
- Short keys (`step`, `path`, `open`, `next`) already appear as words in several commands'
  prose. A field-coverage assertion over the whole help text passes without any change.
  Search the JSON paragraph only, and match whole words.
- `features` contains `feature` as a substring. Match whole words, or `new`'s `feature` key
  can be satisfied by a paragraph that only says `features`.
- The 80-column sweep checks `Long` prose too. The new paragraphs need hand-wrapping, and
  pflag does not wrap `Long`.
