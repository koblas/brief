# human-output — current state

Scenarios complete: SCENARIO-01..14 (all). Last updated by a post-review fix round (NewFeature
write-site tests).

## Binding decisions

- JSON mode = exact `--json` token before the first `--`, **stripped** by `scanJSONFlag` in
  `run()` ahead of cobra parsing. `--json=<v>` is always a text usage error.
- `reporter` (`internal/cli/json.go`) is the one per-Run output seam: `usageError`/`refusal`
  render R3's error document; success is a per-command `<cmd>Document` embedding `jsonHeader`
  first, by value, never nil slices (`[]` not `null`). `--json` always writes before any
  text-mode write (R1, mutation-verified). `reporter.document(v)` is the one place a success
  document's write error is wrapped (`status`, `check`, `start`, `finish`, both `new` leaves).
- `files_changed`: `null` for a read command; for a write command (`new`, `new feature`,
  `new step`, `finish` — each carries `writesFilesAnnotation`, read by `filesChangedFor(cmd,
  err)`) it is what actually landed: `false` when nothing was written, `true` iff
  `errors.Is(err, scaffold.ErrPartialWrite)`. Set by `scaffold.writeFailure(..., partial bool)`
  for every write in `Finish.applyFinishWrites` after the first, by `markPartial` on `NewStep`'s
  specification write, and on both of `NewFeature`'s writes — every site mutation-verified
  individually. `NewFeature`'s `writeExclusive` has no decoy-directory seam
  (`replaceBytes`/`replaceString`'s trick); its tests force the failure through `cfg` instead: a
  `specification-file` with a path separator whose parent was never `Mkdir`'d, and a `state-file`
  equal to `specification-file` so the second `O_CREATE|O_EXCL` collides with the first write.
  `partialWriteError.Unwrap() []error` carries the sentinel without changing `Error()`'s text.
- `classifyRefusal(err)` order: `*config.InvalidConfigError`, `*unknownFeatureError`,
  `*scaffold.RefusalError`, `*assemble.RefusalError`, generic `errorKindFailure` — load-bearing
  (mutation-verified). Every refusal-matrix row's `fix` is asserted `Equal`, not just non-empty.
- **Paths: absolute in `assemble`/`scaffold`/JSON, relative in text (R6)** via
  `displayPath(wd, p)` — every command goes through it.
- `finish --json`'s `next` is `*string` — null, not omitted, on the no-op; `changed` false
  only on the no-op. `next` = lowest-numbered step whose status isn't done, **depends-on
  ignored** — duplicated in `assemble.Start` and `scaffold.nextOpenStep` (may not import each
  other), agreement pinned by test.
- Golden policy: one exact-bytes golden pins key order (`assert.Equal`, never `JSONEq`);
  decode tables check dynamic fields against a captured value, never a literal.
- `versionString(readBuildInfo)` is the one version rule (`"(devel)"` fallback), shared text
  and JSON.
- Every help document comes from one `root.SetHelpFunc` wrapper (`help_json.go`); `command` is
  always the literal `"help"`. Both the help stub's topic-acceptance check and the template's
  own row loop share one predicate, `listedForHelp(cmd)`.
- **`--json` is a real pflag on every JSON-capable leaf**, via one helper `addJSONFlag`
  (`cli.go`) sharing one usage const `jsonFlagUsage`. Registration is display-only:
  `scanJSONFlag` still strips every `--json` before pflag runs.
- Every JSON-capable command's `Long` ends with a JSON paragraph built from one shared trio in
  `cli.go`: `jsonParagraphHeaderClause`, `jsonFieldsParagraph(keys...)`, `wrapWords`.
  `status`/`check` additionally carry `jsonScriptHint`.

## Left unbuilt

- `brief new --json` (bare `new`, no type) success document — it only ever errors.
- `assemble.Problem.Line` and `assemble.RenderJSON` — both unowned.
- A shared platform helper for the "next open step" rule — lives once in `assemble`, once in
  `scaffold`, tied only by an agreement test.
- A `blocked` flag in `finish`'s output, and a flag `shorthand` field in help entries — neither
  is in the ruled shape.
- `--version` never appears in the help index — root is not an entry.
- A JSON-mode hint in root's or `new`'s own group help — nobody owns it.
- `start --json`'s `shortfalls` renders `null`, not `[]`, when empty — never normalized to R9's
  empty-slice-not-null convention the other list fields follow.

## Traps

- Registering `--json` on every leaf instead of stripping it centrally breaks `brief --json`.
- `displayPath` must check `filepath.IsAbs` before calling `Rel`: finish's refusal path can be
  the user's own relative `--state`/`--handoff` argument.
- A reserved-name collision (`schema`, `command`, `ok`, `exit_code`, `error`) in a future
  payload struct is silently resolved by encoding/json's equal-depth rule.
- `known:` lists only openable dirs. A zero-step feature with findings is `InFlight == false`
  → `(complete)` in `check`, though `status.Complete()` disagrees. Both pre-existing.
- `os.ReadDir` order is filename order — `nextOpenStep` picks the minimum by `pattern.Number`.
- pflag sorts a leaf's Flags rows by name: `--json` lands between `--help` and `--state` in
  finish's table — a future flag addition must check where pflag's sort puts it.

## Open debts

- Everything in "Left unbuilt" above is unowned and dies unless re-opened.
- `scaffold.noSuchFeatureRefusal`'s dead `Problem`/`Fix` fields — unowned.
- A sticky write-error for a usage/refusal/help JSON write failure (today silently discarded:
  `_ = writeJSONDocument(...)`) — unowned — dies unless re-opened.
- A success document's own write failure and a text-mode render failure exit with different
  codes/messages for the same underlying I/O fault — unowned asymmetry — dies unless re-opened.
- `writeExclusive` orphaning a file when `WriteString`/`Close` fails after `OpenFile` succeeds
  (`NewStep` then reports `files_changed` false) — correctness MINOR, not constructible via a
  returned error — unowned.
- `reporter.refusal`'s compose-method extraction — refactor MINOR — unowned.
