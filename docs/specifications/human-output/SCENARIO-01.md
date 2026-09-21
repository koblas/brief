# SCENARIO-01: --json turns usage errors into one JSON document

## Scenario

```gherkin
Scenario: SCENARIO-01 --json turns usage errors into one JSON document
  When I run "brief status --json --bogus", "brief bogus --json" or "brief --json"
  Then stdout is one JSON document with schema 1, the command, ok false, exit_code 2,
    and error.kind "usage" whose message is the text-mode line
  And stderr is empty and the exit code is 2
  And "brief status --json=x" is a text usage error "'--json' takes no value", and a --json after "--" is a positional
```

Rules: R1, R2, R3, R5. First scenario of the feature: no STATE.md, no prior handoff.
Existence facts came from `go doc ./internal/cli`, `go doc ./internal/assemble RenderJSON`,
and anchored greps on `usageError`, `newFlagErrorFunc`, `runRoot`, `classifyDashArg`,
`leafCommand` and the `--json` test rows. No Glob needed.

## User-visible contract (this scenario)

- **JSON mode** is on iff an exact `--json` token appears in argv before the first `--`.
  `run()` scans for it before cobra sees argv, and **strips every such token**. It leaves the
  `--` token and everything after it untouched.
- **Usage error in JSON mode:** stdout gets exactly one compact JSON document plus a newline,
  and stderr gets zero bytes. Exit 2.
  Document: `{"schema":1,"command":"<cmd>","ok":false,"exit_code":2,"error":{"kind":"usage","message":"<exact text-mode line, no newline>","path":null,"line":null,"problem":null,"fix":"run '<help hint>'","files_changed":<false|null>}}`.
  - `command` is `strings.TrimPrefix(cmd.CommandPath(), "brief ")`. It is `"brief"` for
    root-level errors, and `"start"`, `"new"`, `"new feature"`, `"help"` or `"completion"`
    for the others.
  - `fix` names the same action as `message`. When the text line ends in `; run '<hint>'`,
    `fix` is exactly `run '<hint>'`, taken from the line, so `message` ends in `"; " + fix`.
    The leaf hints are `brief start <feature>`, `brief status`, and so on, and the stub hint is
    `brief help <command>`.
    When the line has no run hint (for example "no command given; expected one of: …", or
    finish's "- may be given for at most one …"), `fix` falls back:
    - a leaf with an invocation annotation gets `run '<invocation>'`;
    - `new` gets `run 'brief new --help'`;
    - `help` gets `run 'brief help <command>'`;
    - the root gets `run 'brief --help'`.
  - `files_changed` is `false` for the write commands (`new`, `new feature`, `new step`,
    `finish`) and `null` for everything else.
- **`--json=<v>` before `--`** (any value, including empty, including `true`) is a **text**
  usage error, even when a bare `--json` is also present. The error goes to stderr, stdout
  stays empty, and the exit code is 2.
  - The hint mirrors the existing per-level `--help=x` copy. Leaves:
    `brief <path>: '--json' takes no value; run '<invocation annotation>'`, for example
    `brief status: '--json' takes no value; run 'brief status'`.
  - `new`: `brief new: '--json' takes no value; run 'brief new --help'`.
  - The help stub: `brief help: '--json' takes no value; run 'brief help <command>'`.
  - The root: `brief: '--json' takes no value; run 'brief --help'`, which is
    `takesNoValueMessage`.
  - The command is resolved with `root.Find(strippedArgs)`, the same lookup cobra's own
    dispatch uses.
  - This check runs before dispatch, so it wins over every other usage error on the line.
- `--json` after `--` is a positional. `brief status x -- --json` gives a text
  "too many arguments" error on stderr. `-json` (single dash) is not the token and keeps its
  existing unknown-shorthand text error.
- **Unchanged in this scenario, and stated so nobody "fixes" it early:**
  - Refusals under `--json` stay plain stderr text, exit 1, empty stdout (S02 changes this).
  - `start --json` success prints today's bare `assemble.Brief` JSON with no header (S03).
  - `status`/`check`/`finish`/`new */--version/help/completion` given `--json` now run their
    text path and exit as they would without it. They no longer answer "unknown flag: --json".
    S07/S09/S10/S11/S12/S13 own those documents. `brief --version --json` therefore already
    prints the text version line, exit 0.

## Implementation Plan

- [x] Step 1: `internal/cli/json_usage_test.go` `Test_json_mode_renders_a_usage_error_as_one_document` — golden bytes for `status --json --bogus` (exact bytes, key order pinned) (red)
- [x] Step 2: `internal/cli/json_usage_test.go` `Test_json_mode_usage_error_message_is_the_text_mode_line` — table across every usage path. Rows, as `argv → command, files_changed`: `--json` → brief,null; `bogus --json` → brief,null; `--json --bogus` → brief,null; `--help=x --json` → brief,null; `--version extra --json` → brief,null; `new --json` → new,false; `new bogus --json` → new,false; `new -x --json` → new,false; `help bogus --json` → help,null; `help -x --json` → help,null; `status --json --bogus` → status,null (status registers no `--json`); `check --bogus --json` → check,null; `start --bogus --json demo` → start,null; `status a --json` → status,null; `start --json` → start,null; `finish --json` → finish,false; `new feature --json` → new feature,false; `new step --json` → new step,false; `completion --json` → completion,null; `completion nosh --json` → completion,null; `status --help=x --json` → status,null (a leaf bool-flag error through `boolFlagParseMessage` under JSON mode). Compare `error.message` against the no-`--json` stderr **with its single trailing newline trimmed**; never add `\n` to the message to make them match. Relation assert for `fix`: when the text line contains `; run '`, assert `strings.HasSuffix(message, "; "+fix)`. Pin the fallback `fix` as an explicit literal on the rows without a run hint (`--json`, `bogus --json`, `new --json`, `new bogus --json`, `completion nosh --json`, and any others the developer finds). Each row runs argv with and without `--json` and asserts: stderr empty; exit 2; header `schema/command/ok/exit_code`; `error.kind == "usage"`; `error.message` equals the text-mode stderr line from the no-`--json` run; exact key set of document and `error` via `map[string]json.RawMessage`, with `path/line/problem` present as `null`; `files_changed` per row (red)

  Implemented as three tables, not one, since a single table mixing all three assertion shapes would need a per-row branch, which the project's table-testing rules forbid: `Test_json_mode_usage_error_message_is_the_text_mode_line` (relation assert on rows whose line ends in its own "; run '<hint>'" clause), `Test_json_mode_usage_error_fix_falls_back_when_its_message_has_no_run_hint` (explicit literal `fix` on the six rows with no such clause at all), and `Test_json_mode_usage_error_fix_stops_at_the_quote_when_the_line_has_trailing_prose` — a third shape found during review: `new feature`'s empty-name/whitespace-name lines carry a "; run '<hint>'" clause followed by more prose, so `usageFix`'s own `HasSuffix(msg, "'")` guard fails and it falls back rather than slicing the trailing text as the hint; the fallback happens to agree with the clause because "new feature" is a leaf and its invocation literal is what the clause itself names.
- [x] Step 3: `internal/cli/json_usage_test.go` `Test_json_after_double_dash_is_a_positional` — `status x -- --json` gives text on stderr and empty stdout, with control arm `status x --json --` giving a JSON document on stdout; also root `-- --json` gives text (red)
- [x] Step 4: `internal/cli/json_usage_test.go` `Test_json_with_a_value_is_a_text_usage_error` — `status --json=x`, `status --json=`, `start --json=true demo`, `new --json=x`, `help --json=x`, root `--json=x`, `status --json --json=x` all give the per-level text line on stderr, empty stdout, exit 2. Control: `status -- --json=x` gives the text "too many arguments" line, not the takes-no-value line (red)
- [x] Step 5: `internal/cli/json.go` — `schemaVersion` const, `errorKindUsage` const, `jsonHeader` (embedded, header-first), `newJSONHeader(command, exitCode)` deriving `ok`, `jsonError` (pointer fields for nullable members, no `omitempty`), `errorDocument`, `writeJSONDocument(w, v)` (buffered, compact, `SetEscapeHTML(false)`, trailing newline, same as `RenderJSON`) (new)
- [x] Step 6: `internal/cli/json.go` `scanJSONFlag(args)` — exact-token scan up to the first `--`. Returns stripped args, JSON mode, and whether a `--json=` token was seen (new)
- [x] Step 7: `internal/cli/json.go` `reporter` — per-Run value `{stdout, stderr, json, command}`, `(reporter).forCommand(*cobra.Command)`, `(reporter).usageError(msg)`. In JSON mode it writes the error document to stdout, otherwise it writes the line to stderr. Both modes return the same `ErrUsage`-wrapped error. Helpers `commandName(cmd)`, `usageFix(msg, cmd)` (run-hint suffix of the line, else the per-level fallback), `filesChangedFor(command)` (new)

  Implemented with a `cmd *cobra.Command` field (set by `forCommand`) rather than a plain `command` string, so `usageFix`/`commandName`/`filesChangedFor` can be called lazily from inside `usageError` at render time — `command` alone can't be known until `forCommand` runs, and `usageFix` needs the command's own invocation annotation, not just its rendered name.
- [x] Step 8: `internal/cli/cli.go` `run` — call `scanJSONFlag` before `SetArgs`. On a `--json=` token, resolve via `root.Find` and return the text takes-no-value usage error before `ExecuteContext`. Build the base `reporter` and pass it to `newRootCommand` (green for Steps 3–4)

  `root.InitDefaultHelpCmd()` is called explicitly right after `newRootCommand` returns: cobra's `ExecuteC` registers the help stub as a real child command lazily (its own `InitDefaultHelpCmd` call, which normally runs just before `Find`/dispatch), so calling `root.Find` any earlier — as this early-return branch must — would see a tree with no "help" child yet and misresolve `help --json=x`'s target to root. The call is idempotent; `ExecuteContext`'s own later call to it is a harmless no-op re-add.
- [x] Step 9: `internal/cli/cli.go` — replace `usageError(stderr, msg)` with `out.usageError(msg)`. `newRootCommand`, `runRoot`, `newHelpCommand` and `newFlagErrorFunc` take the reporter. Each RunE closure and the FlagErrorFunc call `forCommand(cmd)`. Start's closure passes `out.json` in place of `GetBool("json")`. Keep `fs.Bool("json", …)` registered, because help and the `start [--json]` use line still read it (update)
- [x] Step 10: `internal/cli/{start,status,check,finish,new,completion}.go` — `run*` take a `reporter` in place of `stdout, stderr`. Usage errors go through it. Refusal and text paths keep using `out.stdout`/`out.stderr` unchanged (green for Steps 1–2)
- [x] Step 11: `internal/cli/cli.go` `boolFlagParseMessage` — **keep it**. Cobra auto-registers `-h/--help` as a bool on every leaf, so `status --help=x` still reaches it. Update only its doc comment, which cites `--json=maybe`, to name `--help=<v>` on a leaf as the live case (update)
- [x] Step 12: `internal/cli/flag_error_test.go` — rewrite only the `--json=maybe` / `--json=` rows to the takes-no-value text (`brief start: '--json' takes no value; run 'brief start <feature>'`). Fix the comment block above them. Leave the `-json` shorthand row and every `--help=` row as they are (update)

  Also renamed the test function (`Test_json_flag_with_a_value_never_reaches_the_bool_flag_rewrite`): with both its rows rewritten, nothing in it demonstrates `boolFlagParseMessage`'s bool-value rewrite any more — that proof now lives in Step 2's `status --help=x --json` row, the first case where a leaf's real bool flag (`--help`) still reaches pflag once `--json` is intercepted ahead of it. Its former positive-case reference in `cli_internal_test.go`'s `Test_bool_flag_rewrite_does_not_apply_to_a_non_bool_flag` comment was repointed there too.
- [x] Step 13: `internal/cli/start_test.go` `Test_start_json_still_reports_usage_errors` — rewrite: both arms now expect one JSON document on stdout, empty stderr, exit 2. Leave `Test_start_json_writes_no_bytes_to_stdout_when_it_refuses` untouched; S02 owns it (update)
- [x] Step 14: `internal/cli/doc.go` + `newRootCommand`/`run` doc comments — state the JSON-mode rule, stripping, the `--` boundary and the takes-no-value rule as contract. No scenario ids (update)
- [x] Step 15: mutation verification, one at a time, stashed. Required list is in Handoff *Traps*; report which test each one reddens

  All six run individually via a `cp`-backed copy of `internal/cli/json.go` (never a bare `git stash`), diffed byte-identical to the original after each restore:
  - (a) scan ignores `--`: reddened exactly `Test_json_after_double_dash_is_a_positional`'s two `--`-boundary subtests (`a positional --json after -- stays text`, `root -- --json keeps -- itself as the unknown command`); the third subtest (bare `--json` before `--`) stayed green, as it doesn't depend on the boundary.
  - (b) scan never sets JSON mode: reddened the golden test plus every row of both Step 2 tables (21 subtests) — every one expected valid JSON and got the plain unknown-flag text instead.
  - (c) `HasPrefix("--json")` in place of exact match: reddened 7 of 8 `Test_json_with_a_value_is_a_text_usage_error` rows; the `status -- --json=x` control row stayed green (unaffected — it never carries a bare `--json` token).
  - (d) detect without stripping: reddened the golden test (`Test_json_mode_renders_a_usage_error_as_one_document`) — the document's `error.message` named `--json` as the unknown flag instead of `--bogus`.
  - (e) `usageError`'s JSON branch writes to stderr instead of stdout: reddened every JSON-mode test across the package (stdout empty / stderr non-empty assertions and JSON-decode failures throughout `json_usage_test.go` and `start_test.go`).
  - (f) the `--json=` check removed: reddened 7 of 8 `Test_json_with_a_value_is_a_text_usage_error` rows (each fell through to pflag's own unknown-flag wording); the `status -- --json=x` control row stayed green.
- [x] Step 16: `go build ./...`, `go test ./...` (unpiped, report count and delta), `go test -race ./internal/cli/...`, `golangci-lint run ./...` → mark SCENARIO-01 done in specification.md

  `go build ./...` clean. `go test ./...` green across all 8 packages, exit 0. `internal/cli`: 460 subtests passed, 0 skipped, 0 failed (`go test -v ./internal/cli/...`). Baseline top-level `Test` funcs in the four touched pre-existing test files (`flag_error_test.go`, `help_test.go`, `run_test.go`, `start_test.go`) was 88 (18+13+28+29 at `HEAD`); all 88 remain (6 rows across 3 funcs rewritten for R5, 0 removed), plus 8 new top-level functions in the new `json_usage_test.go` — net +8 top-level test functions, consistent with "existing behavior pinned, new behavior added" rather than a silent drop. `go test -race ./internal/cli/...` green. `golangci-lint run ./...`: 0 issues.

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- JSON mode = exact `--json` before first `--`, detected and **stripped** in `run()` by
  `scanJSONFlag`. No command ever sees `--json` in its args. Start's pflag `--json` stays
  registered for help only; its value is never read. S12's "relaxed sole-argument" rule and
  `brief --json` → "no command given" both depend on stripping.
- `reporter` (internal/cli/json.go) is the one per-Run output seam, created in `run()` and
  narrowed per command with `forCommand(cmd)`. No package state. S02 adds `refusal` /
  `failure` rendering as reporter methods beside `usageError`. Every later success document
  goes through `writeJSONDocument` with an embedded `jsonHeader`.
- `command` = `TrimPrefix(cmd.CommandPath(), "brief ")`, so it is `"brief"` at the root. S12
  (`--version`) and S13 (`help`) documents must use the same derivation.
- Payload structs embed `jsonHeader` anonymously, so the header comes first and the fields
  stay flat (R2). No `data` wrapper, no `omitempty`, and nullable members are pointers.
  Encoding is compact, one line, `SetEscapeHTML(false)` and buffered.
- `ok` is derived from `exit_code` inside `newJSONHeader`. Never set it separately.
- `files_changed`: `false` for `new`, `new feature`, `new step`, `finish`, and `null`
  otherwise (`filesChangedFor`) and `usageFix(msg, cmd)`.
- Usage `fix` names the same action as `message`: it is the line's own `; run '<hint>'`
  suffix, else a per-level fallback (the leaf's invocation annotation, `brief new --help`,
  `brief help <command>`, `brief --help`). `path`, `line` and `problem` are `null` for usage
  errors. A `fix` that differs from the hint in `message` is a defect.
- `--json=<v>` is always a text error (R5), with precedence over every other usage error, and
  the command is resolved via `root.Find`.
- Golden policy: one exact-bytes golden per document shape pins key order. Matrix tests
  compare structurally, and `error.message` is always checked against the no-`--json` run's
  stderr, never against a literal copied from production.

**Left unbuilt** — named so nobody assumes it exists:
- refusal/failure error documents (`errorKindRefusal`, `errorKindFailure`, reporter method
  for `renderRefusal`) — S02.
- `jsonHeader` on start's success document — S03 (start still emits bare `assemble.Brief`).
- JSON documents for status/check/new/finish/--version/help — S07/S09/S10/S11/S12/S13.
  Until then those commands silently run their text path under `--json`.
- `completion … --json` usage-error document — S13. Until then it prints the script.
- `--json` flag row in every command's help — S14. Only `start` registers it today.

**Traps** — things that look right and are not:
- Registering `--json` on every leaf instead of stripping. Root and `new` disable flag
  parsing, so `brief --json` would become "unknown flag: --json" and not "no command given".
- A pre-scan that forgets the `--` boundary, or strips the `--` itself, turns
  `status x -- --json` into JSON mode or breaks positional handling in pflag.
- `--handoff --json` is now a missing-value flag error, because `--json` is stripped before
  pflag can take it as `--handoff`'s value. That is R5-correct; do not "fix" it.
- `boolFlagParseMessage` looks dead once `--json` is stripped, but it is not. Cobra's
  auto-registered leaf `--help` is a bool flag, and `status --help=x` goes through it.
- `Test_start_json_writes_no_bytes_to_stdout_when_it_refuses` stays green unchanged. It is
  S02's to rewrite, not this scenario's.
- Required mutations, stashed and run individually:
  (a) the scan ignores `--` → Step 3 red;
  (b) the scan never sets JSON mode → Steps 1–2 red;
  (c) `HasPrefix("--json")` in place of exact match → Step 4 red;
  (d) detect without stripping → Step 1 golden red (`--json` becomes the unknown flag);
  (e) `usageError` JSON branch writes to stderr → stderr-empty assertions red;
  (f) `--json=` check removed → Step 4 red.
