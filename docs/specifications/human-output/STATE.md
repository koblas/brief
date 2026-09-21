# human-output — current state

Scenarios complete: SCENARIO-01, SCENARIO-02, SCENARIO-03. Last updated by SCENARIO-03.

## Binding decisions

- JSON mode = exact `--json` token before the first `--`, **stripped** by `scanJSONFlag` in
  `run()` (`internal/cli/cli.go`) ahead of cobra parsing; no command ever sees it in its own
  args. Start's pflag `--json` stays registered only for its help-table row; commands read
  `out.json` instead. (SCENARIO-01)
- `reporter` (`internal/cli/json.go`) is the one per-Run output seam: `{stdout, stderr,
  json, wd}`, narrowed per command via `forCommand(cmd)`. `usageError` renders a usage-error
  document; `refusal(err)` renders every other exit-1 error; `successHeader()` returns the
  `jsonHeader` every success document embeds (`newJSONHeader(commandName(r.cmd), 0)`).
- `command` = `commandName(cmd)` = `strings.TrimPrefix(cmd.CommandPath(), "brief ")` —
  `"brief"` at the root.
- Every JSON document embeds `jsonHeader` anonymously first (`schema`, `command`, `ok`,
  `exit_code`), no `data` wrapper, no `omitempty`; nullable members are pointers. Encoding is
  `writeJSONDocument` — compact, `SetEscapeHTML(false)`, buffered, trailing newline. `ok` is
  always derived from `exit_code` in `newJSONHeader`, never set independently.
- **Success documents** are a per-command `<cmd>Document` struct embedding `jsonHeader` (from
  `reporter.successHeader()`) first, then the payload by value, written with
  `writeJSONDocument` — `json.go` stays the only place a header is built. Start's
  `startDocument{jsonHeader; assemble.Brief}` is the first instance; SCENARIO-07/09/10/11/12/13
  follow this shape.
- **A successful `--json` run writes nothing to stderr (R1).** Whatever a stderr line said as
  data must already be a payload field (e.g. start's shortfalls); pure human copy is dropped,
  never moved into a `notices` field. The JSON branch runs **before** any stderr write and
  returns — one branch per command, not per-write guards. Start's discriminators:
  `step:null` + `done`/`open` (complete vs. no step files) — no extra field.
- `files_changed`: `false` for `new`, `new feature`, `new step`, `finish`; `null` otherwise
  (`filesChangedFor`).
- Usage `fix` = the line's own `"; run '<hint>'"` clause when it **ends** with one
  (`usageFix`), else a per-level fallback (`usageHint`); `path`/`line`/`problem` stay `null`.
- `--json=<v>` (any value) is **always** a text usage error, checked in `run()` via
  `scanJSONFlag`'s `hasValue`, before dispatch.
- `(reporter) refusal(err)` (`internal/cli/refusal.go`) is the one refusal/failure seam;
  `classifyRefusal(err)` is the pure classifier — text line and `--json`'s `message` are
  always the same string. `path` absolutizes against `r.wd`; `""`/`"<stdin>"` render `null`.
  `kind` = `"refusal"` for `*config.InvalidConfigError`, `*scaffold.RefusalError`,
  `*assemble.RefusalError`, bare `assemble.ErrNoSuchFeature`; `"failure"` otherwise, whose
  `fix` comes from `usageHint(cmd)`. `errCheckFindings` never renders through `refusal` (R4).
- Golden policy: one exact-bytes golden pins key order per document shape (usage, refusal,
  start success); matrix/decode tests check dynamic fields against a captured value, never a
  literal copied from production.

## Left unbuilt

- status/check/new/finish/--version/help JSON documents — SCENARIO-07/09/10/11/12/13. Until
  each lands, that command silently runs its **text** path under `--json`.
- `completion … --json` usage-error document — SCENARIO-13; today it prints the script
  regardless.
- `--json` flag row in every command's help — SCENARIO-14. Only `start` registers it; its
  `startLong` still says notices go to stderr unconditionally — also SCENARIO-14's.
- A bare not-found refusal's `path` stays `null`, `fix` stays the generic status hint —
  SCENARIO-05 names the directory and the known-features list.
- `check --json` findings payload, `status --json` payload — SCENARIO-09/07.
- Relative paths in text-mode start shortfall/notice lines (R6 text side) — SCENARIO-04/05.
- `assemble.RenderJSON` — no production caller now that start builds its own document via
  `writeJSONDocument`; removing it is a refactor, **unowned by any scenario**.

## Traps

- Registering `--json` on every leaf instead of stripping it centrally: root and `new`
  disable flag parsing, so `brief --json` would become "unknown flag: --json". A pre-scan
  that forgets the `--` boundary mishandles `status x -- --json`.
- `boolFlagParseMessage` looks dead once `--json` is stripped, but isn't: cobra's leaf
  `--help` is still a real bool flag (`status --help=x --json`).
- `root.Find` before `root.InitDefaultHelpCmd()` misresolves any `help …` target — matters
  for the pre-dispatch `--json=<v>` check in `run()`. `usageError` in `cli.go` (text-only) is
  not `reporter.usageError`; the `--json=<v>` path must keep using the text one.
- finish rewrites `scaffold.RefusalError.Path` to the user's `--state`/`--handoff` argument,
  which may be relative or `<stdin>` — every other refusal path is already absolute.
- Unmarshalling a header-carrying document into a bare `assemble.Brief` still succeeds
  (extra keys ignored) — decoding proves nothing about the header; assert header fields
  explicitly or use a golden. Embedding the payload as a pointer also flattens, but a nil
  pointer silently drops every payload key — embed by value, as `startDocument` does.
- A reserved-name collision (`schema`, `command`, `ok`, `exit_code`, `error`) in a future
  payload struct is silently resolved by encoding/json's equal-depth rule — keep payload
  field names disjoint from the header's.

## Open debts

- `assemble.RenderJSON` (see Left unbuilt) — unowned; dies unless a future scenario
  re-opens removing it.
