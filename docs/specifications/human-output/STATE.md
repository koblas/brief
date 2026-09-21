# human-output — current state

Scenarios complete: SCENARIO-01. Last updated by SCENARIO-01.

## Binding decisions

- JSON mode = exact `--json` token anywhere before the first `--`, detected and **stripped**
  by `scanJSONFlag` in `run()` (`internal/cli/cli.go`), before cobra ever parses anything. No
  command ever sees `--json` in its own args. Start's pflag `--json` bool flag stays
  registered only so its help-table row still renders; its value is never read — commands
  read `out.json` instead. (SCENARIO-01)
- `reporter` (`internal/cli/json.go`) is the one per-Run output seam: built once in `run()`
  with `{stdout, stderr, json}`, narrowed per command via `forCommand(cmd)` which sets its
  `cmd` field. `usageError(msg)` is its only method today; a later scenario adds
  `refusal`/`failure` rendering beside it. No package-level state.
- `command` (the JSON header field and every fallback-fix lookup) = `commandName(cmd)` =
  `strings.TrimPrefix(cmd.CommandPath(), "brief ")` — `"brief"` at the root. Every later
  document (start, status, check, finish, new, --version, help) must derive it the same way.
- Every JSON document embeds `jsonHeader` anonymously first (`schema`, `command`, `ok`,
  `exit_code`), no `data` wrapper, no `omitempty`; nullable members are pointers. Encoding is
  `writeJSONDocument` — compact, `SetEscapeHTML(false)`, buffered, one trailing newline.
  `ok` is always derived from `exit_code` in `newJSONHeader`, never set independently.
- `files_changed`: `false` for `new`, `new feature`, `new step`, `finish`; `null` otherwise
  (`filesChangedFor`).
- Usage `fix` = the line's own `"; run '<hint>'"` clause when the line **ends** with one
  (`usageFix` requires both `strings.LastIndex(msg, "; run '")` and `HasSuffix(msg, "'")`),
  else a per-level fallback (`usageHint`: leaf's own invocation annotation, `"brief new
  --help"`, `"brief help <command>"`, or `"brief --help"`). A line that carries the clause
  followed by more prose (`new feature`'s empty/whitespace-name lines) also falls back — it
  agrees with the clause only because the leaf's own invocation happens to be what the clause
  names, not because `usageFix` parses the clause out of trailing text. `path`/`line`/`problem`
  stay `null` for a usage error.
- `--json=<v>` (any value, including empty) is **always** a text usage error, checked in
  `run()` via `scanJSONFlag`'s `hasValue` return — before `ExecuteContext`, so it wins over
  every other usage error on the line, even a bare `--json` alongside it. `run()` calls
  `root.InitDefaultHelpCmd()` right after building `root` (needed so `root.Find` can resolve
  `"help"` as a real child before cobra's own dispatch would normally register it) — do not
  remove this call, it is not dead code.
- Golden policy: one exact-bytes golden pins key order; matrix tests compare structurally and
  always check `error.message` against the same argv's no-`--json` stderr, never a literal
  copied from production.

## Left unbuilt

- Refusal/failure error documents (`errorKindRefusal`, `errorKindFailure`, a `reporter`
  method beside `usageError`) — SCENARIO-02. Refusals under `--json` still print plain stderr
  text today, exit 1, empty stdout.
- `jsonHeader` on a success document — SCENARIO-03 (start still emits bare `assemble.Brief`
  JSON with no header). status/check/new/finish/--version/help JSON documents —
  SCENARIO-07/09/10/11/12/13. Until each lands, that command silently runs its **text** path
  under `--json` (this is intentional, not a gap to "fix" early) — confirmed for `help` and
  `--version` by `Test_json_stripped_from_help_topic_arguments_runs_the_ordinary_help_path`
  and `Test_version_with_json_relaxes_the_sole_argument_rule` (`json_usage_test.go`).
- `completion … --json` usage-error document — SCENARIO-13. Until then it prints the script
  regardless of `--json` (stripped, ignored).
- `--json` flag row in every command's help — SCENARIO-14. Only `start` registers it today.

## Traps

- Registering `--json` on every leaf instead of stripping it centrally: root and `new`
  disable flag parsing, so `brief --json` would become "unknown flag: --json" instead of "no
  command given".
- A pre-scan that forgets the `--` boundary, or strips `--` itself, turns
  `status x -- --json` into JSON mode or breaks positional handling in pflag.
- `boolFlagParseMessage` looks dead once `--json` is stripped, but it is not: cobra's
  auto-registered leaf `--help` is still a real bool flag, so `status --help=x` still reaches
  it (proof moved to `json_usage_test.go`'s `status --help=x --json` row).
- Calling `root.Find` before `root.InitDefaultHelpCmd()` misresolves any `help …` target
  (falls back to root) — only matters for the pre-dispatch `--json=<v>` check in `run()`,
  since `ExecuteContext`'s own dispatch already calls `InitDefaultHelpCmd` first.
- `Test_start_json_writes_no_bytes_to_stdout_when_it_refuses` (`start_test.go`) is
  deliberately untouched — SCENARIO-02 owns rewriting refusal rendering, not this one.

## Open debts

- None unowned. Every "Left unbuilt" item above already names the scenario that closes it.
