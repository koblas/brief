# human-output — current state

Scenarios complete: SCENARIO-01..04. Last updated by SCENARIO-04.

## Binding decisions

- JSON mode = exact `--json` token before the first `--`, **stripped** by `scanJSONFlag` in
  `run()` (`internal/cli/cli.go`) ahead of cobra parsing. Start's pflag `--json` stays
  registered only for its help-table row; commands read `out.json` instead. `--json=<v>` is
  always a text usage error, checked before dispatch. (S01)
- `reporter` (`internal/cli/json.go`) is the one per-Run output seam: `{stdout, stderr, json,
  wd}`, narrowed per command via `forCommand(cmd)`. `command` = `commandName(cmd)` =
  `cmd.CommandPath()` minus `"brief "`. `usageError` renders a usage-error document;
  `refusal(err)` renders every other exit-1 error; `successHeader()` builds the `jsonHeader`
  every success document embeds first (`schema`, `command`, `ok`, `exit_code`; no `data`
  wrapper, no `omitempty`, nullable members are pointers), via `writeJSONDocument` (compact,
  `SetEscapeHTML(false)`, buffered, trailing newline).
- **Success documents** are a per-command `<cmd>Document` embedding `jsonHeader` first, then
  the payload **by value** (a pointer payload lets a nil silently drop every key). Start's
  `startDocument{jsonHeader; assemble.Brief}` is the first instance; S07/09/10/11/12/13 follow
  it. A successful `--json` run writes nothing to stderr (R1): the JSON branch runs before
  any stderr write and returns.
- `files_changed`: `false` for `new`, `new feature`, `new step`, `finish`; `null` (read
  commands) otherwise (`filesChangedFor`).
- `(reporter) refusal(err)` is the one refusal/failure seam; `classifyRefusal(err)` is the
  pure classifier — text line and `--json`'s `message` are always the same string. `kind` =
  `"refusal"` for `*config.InvalidConfigError`, `*scaffold.RefusalError`,
  `*assemble.RefusalError`, bare `assemble.ErrNoSuchFeature`; `"failure"` otherwise, whose
  `fix` comes from `usageHint(cmd)`. `errCheckFindings` never renders through `refusal` (R4).
- **Paths: absolute in `assemble`/`scaffold` and every JSON field, relative in every
  text-mode line (R6).** `internal/cli/refusal.go`'s `displayPath(wd, p)` is the one
  text-side relativizer: `""`/`"<stdin>"`/an already-relative `p` pass through unchanged, an
  absolute `p` goes through `filepath.Rel(wd, p)` with an absolute fallback on error, and a
  `..` chain is accepted on purpose (a subdirectory below the config root, or finish's own
  out-of-tree `--state`/`--handoff` temp file, both produce one). `textLine(wd)` and
  `status.go`/`start.go`'s stderr lines render through it; `jsonPath`/`jsonLine` stay
  absolute — refusal `message` relative, JSON `path` absolute is intended. (S04)
- **A step-file frontmatter parse failure** (no frontmatter, unclosed delimiter, bad YAML) is
  wrapped by `readSteps` in the unexported `*stepFrontmatterError{name, err}`
  (`internal/assemble/errors.go`; `Unwrap` keeps `errors.Is(ErrNoFrontmatter)` reachable).
  `newProblem`'s branch for it: `Path` = the step file, `Detail` = the wrapped error's own
  message, `Fix` = `run 'brief check <feature>' to list every fault` — `<feature>` from the
  feature dir, never the step file's name. A read failure (`*fs.PathError`) keeps
  `readClassFix`. `newProblem`'s 3rd, generic-fallback branch is unreachable from any current
  caller and is deliberately not red-tested. (S04)
- Golden policy: one exact-bytes golden pins key order per document shape (usage, refusal,
  start success); matrix/decode tests check dynamic fields against a captured value, never a
  literal copied from production.

## Left unbuilt

- status/check/new/finish/--version/help JSON documents — S07/09/10/11/12/13. Until each
  lands, that command silently runs its **text** path under `--json`.
- `completion … --json` usage-error document — S13; today it prints the script regardless.
- `--json` flag row in every command's help — S14. Only `start` registers it.
- A bare not-found refusal's `path` stays `null`, `fix` stays the generic status hint — S05
  names the directory and the known-features list.
- `check`'s own text/JSON paths (`RenderFindings`) are still absolute — S08 relativizes them
  through `displayPath`.
- `status`'s stderr frame is still `brief status: <rel path>: <detail>; <fix>` — S06 changes
  it to `brief status: <feature>: <rel path>: …` with the table and summary line.
- `new`'s stdout path and `new`/`finish` success stderr are still absolute — S10/S11.
- `assemble.RenderJSON` — no production caller now that start builds its own document via
  `writeJSONDocument`; removing it is a refactor, **unowned by any scenario**.

## Traps

- Registering `--json` on every leaf instead of stripping it centrally: root and `new`
  disable flag parsing, so `brief --json` would become "unknown flag: --json". A pre-scan
  that forgets the `--` boundary mishandles `status x -- --json`.
- Wrapping a frontmatter parse failure as an `*fs.PathError` looks like it would name the
  file for free; it routes into `newProblem`'s read-class branch and silently changes `Fix`
  to `readClassFix`.
- `displayPath` must check `filepath.IsAbs` before calling `Rel`: finish's refusal path can
  be the user's own relative `--state`/`--handoff` argument. A "no `..`" guard on its output
  would break the subdirectory contract, which needs one.
- Unmarshalling into a bare payload type (e.g. `assemble.Brief`) still succeeds with extra
  keys ignored — decoding proves nothing about the header; assert header fields explicitly.
- A reserved-name collision (`schema`, `command`, `ok`, `exit_code`, `error`) in a future
  payload struct is silently resolved by encoding/json's equal-depth rule — keep payload
  field names disjoint from the header's.

## Open debts

- `assemble.RenderJSON` (see Left unbuilt) — unowned; dies unless a future scenario
  re-opens removing it.
