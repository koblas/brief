# human-output — current state

Scenarios complete: SCENARIO-01..06. Last updated by SCENARIO-06.

## Binding decisions

- JSON mode = exact `--json` token before the first `--`, **stripped** by `scanJSONFlag` in
  `run()` ahead of cobra parsing. Start's pflag `--json` stays registered only for its help
  row; commands read `out.json`. `--json=<v>` is always a text usage error. (S01)
- `reporter` (`internal/cli/json.go`) is the one per-Run output seam, narrowed per command
  via `forCommand(cmd)`. `usageError` renders a usage-error document; `refusal(err)` renders
  every other exit-1 error; success documents are a per-command `<cmd>Document` embedding
  `jsonHeader` first, payload **by value**, via `writeJSONDocument`.
- `files_changed`: `false` for `new`, `new feature`, `new step`, `finish`; `null` otherwise
  (`filesChangedFor`).
- `classifyRefusal(err)` is the pure classifier text line and `--json` `message` share.
  Order: `*config.InvalidConfigError`, `*unknownFeatureError`, `*scaffold.RefusalError`,
  `*assemble.RefusalError`, generic `errorKindFailure`. Not-found is enriched at the call
  site (`enrichUnknownFeature` in `start.go`/`check.go`/`finish.go`/`new.go`), never in
  `classifyRefusal` itself, and `*unknownFeatureError` **must** be checked before
  `*scaffold.RefusalError` (mutation-verified M7, S05) or `new step`/`finish` silently keep
  the old path-less copy.
- **Paths: absolute in `assemble`/`scaffold`/JSON, relative in text (R6)** via
  `internal/cli/refusal.go`'s `displayPath(wd, p)`. (S04)
- A step-file frontmatter parse failure wraps as `*stepFrontmatterError{name, err}`; `newProblem`
  names the step file, fix `run 'brief check <feature>' to list every fault`. (S04)
- `assemble.FeatureStatus`: `Next *NextStep{ID,Title,Path}` (nil = no open step), `Path`
  absolute feature dir set on **every** row including Problem rows. `Next.Title` is
  `markdown.Title` of the step body after frontmatter (empty when no `# ` heading — the
  status table then shows the id alone); `Next.Path` = feature dir joined with
  `pattern.Name(n)`, the same join `Start` uses. `(FeatureStatus).Complete()` =
  `Problem == nil && Total > 0 && Done == Total` is the one definition of "complete" — a
  zero-step feature is NOT complete (DONE `0/0`, NEXT `-`, counted **in progress**). (S06)
- `assemble.RenderStatusText` renders the FEATURE/DONE/BLOCKED/NEXT table via
  `text/tabwriter` (2-space pad, NEXT unpadded/last, header omitted for zero rows per R9);
  it carries no paths and knows nothing of `wd`. `internal/cli/status.go`'s `runStatus`
  writes the table to stdout first (flushed), then one stderr line per malformed row —
  `brief status: <feature>: <rel path>: <detail>; <fix>` — then always (when rows exist)
  `unexported statusSummary(rows)`: `brief status: N features: A in progress, B complete,
  C malformed` (`1 feature` singular, all three buckets always printed). (S06)
- Golden policy: one exact-bytes golden pins key order per document shape; matrix/decode
  tests check dynamic fields against a captured value, never a literal copied from
  production.

## Left unbuilt

- status/check/new/finish/--version/help JSON documents — S07/09/10/11/12/13; until each
  lands that command runs its **text** path under `--json`.
- `completion … --json` usage-error document — S13; today it prints the script regardless.
- `--json` flag row in every command's help — S14. Only `start` registers it.
- `check`'s own text/JSON paths (`RenderFindings`) are still absolute, carry no stable
  `rule` id (R8), and print `:0` for a whole-file finding — S08.
- `new`'s stdout path and `new`/`finish` success stderr are still absolute — S10/S11.
- `status --json` document (`statusDocument`), `Problem.Line` (still absent — S07's
  `problem.line` needs it or `null`), status help's "For scripts, use --json; the text
  layout may change." sentence, and a structured `known` array in the JSON error object —
  all S07/S14; a `--json` caller needing the known list uses `status --json` (S07).
- `assemble.RenderJSON` — no production caller now that start builds its own document;
  removing it is a refactor, **unowned by any scenario**.

## Traps

- Registering `--json` on every leaf instead of stripping it centrally breaks `brief --json`.
- `displayPath` must check `filepath.IsAbs` before calling `Rel`: finish's refusal path can
  be the user's own relative `--state`/`--handoff` argument.
- A reserved-name collision (`schema`, `command`, `ok`, `exit_code`, `error`) in a future
  payload struct is silently resolved by encoding/json's equal-depth rule.
- `known:` (unknown-feature errors) lists only openable directories via `assemble.Features`
  — a symlink or an unopenable directory named like a feature appears in `brief status`'s
  table but never in `known:`, contradictory but pre-existing. S07 must not "reconcile" the
  two lists into one.
- `text/tabwriter` pads only tab-terminated cells: NEXT (last column) must never be
  followed by a tab, or every line gains trailing padding; a tab/newline inside a feature
  name or step title corrupts the table's columns unless flattened first
  (`flattenTabwriterField`). S07's JSON payload must not reuse this flattening — JSON needs
  the raw title.
- Defining "complete" as `Next == nil` (instead of via `Complete()`) makes a zero-step
  feature misread as complete; S07's `complete` bool must call `Complete()`, not recompute.

## Open debts

- `assemble.RenderJSON` (see Left unbuilt) — unowned; dies unless re-opened.
- `scaffold.noSuchFeatureRefusal`'s dead `Problem`/`Fix` fields — unowned; a future pass
  touching `internal/scaffold/scaffold.go` should consider deleting them.
