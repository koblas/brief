# human-output — current state

Scenarios complete: SCENARIO-01..14 (all). Last updated by SCENARIO-14.

## Binding decisions

- JSON mode = exact `--json` token before the first `--`, **stripped** by `scanJSONFlag` in
  `run()` ahead of cobra parsing. `--json=<v>` is always a text usage error.
- `reporter` (`internal/cli/json.go`) is the one per-Run output seam: `usageError`/`refusal`
  render R3's error document; success is a per-command `<cmd>Document` embedding `jsonHeader`
  first, by value, never nil slices (`[]` not `null`). `--json` always writes before any
  text-mode write (R1, mutation-verified).
- `files_changed`: `false` for `new`, `new feature`, `new step`, `finish`; `null` otherwise.
- `classifyRefusal(err)` order: `*config.InvalidConfigError`, `*unknownFeatureError`,
  `*scaffold.RefusalError`, `*assemble.RefusalError`, generic `errorKindFailure` — load-bearing
  (mutation-verified).
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
  always the literal `"help"`; `flags` via `cmd.LocalFlags().VisitAll`/`pflag.UnquoteUsage`,
  `InitDefaultHelpFlag()` called explicitly per entry.
- R11: `brief completion <shell> --json` (recognized shell) is a usage error, guarded only on
  `runCompletion`'s resolved-shell branch.
- **`--json` is a real pflag on every JSON-capable leaf** (`new feature`, `new step`, `start`,
  `finish`, `status`, `check`) and the help stub, via one helper `addJSONFlag` (`cli.go`)
  sharing one usage const `jsonFlagUsage` = `"print one JSON document on stdout"`.
  `leafCommand` does not call it itself — `completion`'s own call still passes `nil` (R11).
  Registration is display-only: `scanJSONFlag` still strips every `--json` before pflag runs.
- Every JSON-capable command's `Long` ends with a JSON paragraph built from one shared trio in
  `cli.go`: `jsonParagraphHeaderClause` (common-header sentence, wrapped once, reused
  verbatim), `jsonFieldsParagraph(keys...)` (that command's own top-level keys, wrapped
  separately from the header clause), `wrapWords` (mechanical greedy wrap). `startLong` also
  carries the "step is null" / "`--json` before or after `<feature>`" facts, no longer in
  `jsonFlagUsage` itself. `status`/`check` additionally end with `jsonScriptHint` ("For
  scripts, use --json; the text layout may change.") — no other command carries it. Root and
  bare `new` get neither row nor paragraph (`rootHelp`/`newHelp` unchanged). `Long` is the
  help-document `description`, so every paragraph is JSON-visible too.

## Left unbuilt

- `brief new --json` (bare `new`, no type) success document — it only ever errors.
- `assemble.Problem.Line` and `assemble.RenderJSON` — both unowned.
- A shared platform helper for the "next open step" rule — lives once in `assemble`, once in
  `scaffold`, tied only by an agreement test.
- A `blocked` flag in `finish`'s output, and a flag `shorthand` field in help entries — neither
  is in the ruled shape.
- `--version` never appears in the help index — root is not an entry.
- A JSON-mode hint in root's or `new`'s own group help — nobody owns it; neither renders a
  Flags table, so a future owner needs a mechanism other than `addJSONFlag`.

## Traps

- Registering `--json` on every leaf instead of stripping it centrally breaks `brief --json`.
- `displayPath` must check `filepath.IsAbs` before calling `Rel`: finish's refusal path can be
  the user's own relative `--state`/`--handoff` argument.
- A reserved-name collision (`schema`, `command`, `ok`, `exit_code`, `error`) in a future
  payload struct is silently resolved by encoding/json's equal-depth rule.
- `known:` lists only openable dirs. A zero-step feature with findings is `InFlight == false`
  → `(complete)` in `check`, though `status.Complete()` disagrees. Both pre-existing.
- A new feature-level `Finding` producer must stamp its own `Feature`/`FeaturePath`/
  `InFlight`/`Rule` itself, at every `Finding{...}` site (mutation-verified).
- `os.ReadDir` order is filename order — `nextOpenStep` picks the minimum by `pattern.Number`;
  a sibling with unparseable frontmatter counts as not done and can be named `next`, though
  `brief start` then refuses the whole feature.
- `root.HelpFunc()` must be captured **before** `SetHelpFunc` replaces it, or the wrapper
  recurses into itself. `HelpFunc`/`cmd.Help()` never return an error, so a write failure in
  the JSON wrapper can't become a non-zero exit — same limit as cobra's own text help.
- pflag sorts a leaf's Flags rows by name: `--json` lands between `--help` and `--state` in
  finish's table and its help-JSON `flags[]` — a future flag addition must check where pflag's
  sort puts it, not assume append order.
- A word-coverage assertion over a whole help text can pass vacuously: short keys (`step`,
  `path`, `open`, `next`) already appear elsewhere in several commands' prose, and
  `features`/`feature` are substrings of each other — search only the slice starting at
  `jsonParagraphMarker`, whole words only, when adding a future field key.
- The 80-column sweep (`Test_every_leaf_help_line_fits_in_80_columns`) checks `Long` prose
  too; pflag never wraps `Long` — hand-wrap, or route through `wrapWords`.

## Open debts

- `assemble.RenderJSON`, `brief new --json`'s success document, and a JSON-mode hint in
  root's/`new`'s own group help (see Left unbuilt) — all unowned, all die unless re-opened.
- `scaffold.noSuchFeatureRefusal`'s dead `Problem`/`Fix` fields — unowned.
