# human-output — current state

Scenarios complete: SCENARIO-01, SCENARIO-02. Last updated by SCENARIO-02.

## Binding decisions

- JSON mode = exact `--json` token anywhere before the first `--`, detected and **stripped**
  by `scanJSONFlag` in `run()` (`internal/cli/cli.go`), before cobra ever parses anything. No
  command ever sees `--json` in its own args. Start's pflag `--json` bool flag stays
  registered only so its help-table row still renders; its value is never read — commands
  read `out.json` instead. (SCENARIO-01)
- `reporter` (`internal/cli/json.go`) is the one per-Run output seam: built once in `run()`
  with `{stdout, stderr, json, wd}`, narrowed per command via `forCommand(cmd)` which sets its
  `cmd` field. `usageError(msg)` renders a usage-error document; `refusal(err)` renders every
  other exit-1 error. No package-level state. (SCENARIO-01/02)
- `command` (the JSON header field and every fallback-fix lookup) = `commandName(cmd)` =
  `strings.TrimPrefix(cmd.CommandPath(), "brief ")` — `"brief"` at the root.
- Every JSON document embeds `jsonHeader` anonymously first (`schema`, `command`, `ok`,
  `exit_code`), no `data` wrapper, no `omitempty`; nullable members are pointers. Encoding is
  `writeJSONDocument` — compact, `SetEscapeHTML(false)`, buffered, one trailing newline.
  `ok` is always derived from `exit_code` in `newJSONHeader`, never set independently.
- `files_changed`: `false` for `new`, `new feature`, `new step`, `finish` (regardless of
  which check inside that command failed); `null` otherwise (`filesChangedFor`).
- Usage `fix` = the line's own `"; run '<hint>'"` clause when the line **ends** with one
  (`usageFix`), else a per-level fallback (`usageHint`). `path`/`line`/`problem` stay `null`
  for a usage error.
- `--json=<v>` (any value, including empty) is **always** a text usage error, checked in
  `run()` via `scanJSONFlag`'s `hasValue` return, before dispatch.
- `(reporter) refusal(err) error` (`internal/cli/refusal.go`) is the one refusal/failure
  seam; `renderRefusal` is gone. `classifyRefusal(err)` is the pure classifier (kind, path,
  line, problem, fix, tail) every command's `out.refusal(err)` call renders through — text
  line and `--json`'s `message` are always the same string, built once by
  `refusalClassification.textLine()`. `error.path` absolutizes against `r.wd` when relative;
  `""` and `"<stdin>"` both render `null`. `kind` = `"refusal"` for
  `*config.InvalidConfigError`, `*scaffold.RefusalError`, `*assemble.RefusalError`, and bare
  `assemble.ErrNoSuchFeature` (assemble returns it bare; scaffold always wraps its own,
  distinct, `ErrNoSuchFeature` in a `*scaffold.RefusalError`); `"failure"` for anything else,
  whose `--json` `fix` is filled by `reporter.refusal` from `usageHint(cmd)`, never by the
  classifier. `errCheckFindings` never renders through `refusal` — `check --json` with
  findings stays exactly its text-mode output (R4). (SCENARIO-02)
- Golden policy: one exact-bytes golden pins key order per document shape (usage, refusal);
  matrix tests compare structurally and always check dynamic fields (OS error text, messages)
  against a captured value, never a literal copied from production.

## Left unbuilt

- `jsonHeader` on a success document — SCENARIO-03 (start still emits bare `assemble.Brief`
  JSON with no header). status/check/new/finish/--version/help JSON documents —
  SCENARIO-07/09/10/11/12/13. Until each lands, that command silently runs its **text** path
  under `--json` (intentional, not a gap to "fix" early).
- `completion … --json` usage-error document — SCENARIO-13. Until then it prints the script
  regardless of `--json` (stripped, ignored).
- `--json` flag row in every command's help — SCENARIO-14. Only `start` registers it today.
- A bare not-found refusal's `path` (the feature directory) stays `null` and its `fix` stays
  the generic `"run 'brief status' …"` — SCENARIO-05 owns naming the directory and offering
  the known-features list instead.
- `check --json` findings payload, `status --json` payload — SCENARIO-09/07.

## Traps

- Registering `--json` on every leaf instead of stripping it centrally: root and `new`
  disable flag parsing, so `brief --json` would become "unknown flag: --json".
- A pre-scan that forgets the `--` boundary, or strips `--` itself, mishandles
  `status x -- --json`.
- `boolFlagParseMessage` looks dead once `--json` is stripped, but it is not: cobra's
  auto-registered leaf `--help` is still a real bool flag (`status --help=x --json`).
- Calling `root.Find` before `root.InitDefaultHelpCmd()` misresolves any `help …` target
  (falls back to root) — only matters for the pre-dispatch `--json=<v>` check in `run()`,
  since `ExecuteContext`'s own dispatch already calls `InitDefaultHelpCmd` first.
- `classifyRefusal`'s typed checks run before the bare-sentinel check by design — mutation
  testing found this order not currently load-bearing (scaffold's own `ErrNoSuchFeature` is a
  distinct sentinel value from assemble's, so `errors.Is` never confuses the two); kept for
  documentation clarity. Re-verify if a future scenario makes scaffold wrap assemble's sentinel.
- finish rewrites `scaffold.RefusalError.Path` to the user's `--state`/`--handoff` argument,
  which may be relative or `<stdin>` — every other refusal path is already absolute.
- `usageError` in `cli.go` (package func, text-only) is not `reporter.usageError`; the
  `--json=<v>` path must keep using the text one.
- A `problem ⊂ message` assertion only holds where the text line carries the problem
  verbatim (every refusal shape does, via `flattenOneLine`); a generic failure's `--json`
  `fix` is never in `message` — it is a `--json`-only fallback.

## Open debts

- None unowned. Every "Left unbuilt" item above already names the scenario that closes it.
