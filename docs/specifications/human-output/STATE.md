# human-output — current state

Scenarios complete: SCENARIO-01..07. Last updated by SCENARIO-07.

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
- `classifyRefusal(err)` order: `*config.InvalidConfigError`, `*unknownFeatureError`,
  `*scaffold.RefusalError`, `*assemble.RefusalError`, generic `errorKindFailure`.
  Not-found is enriched at the call site (`enrichUnknownFeature`), never in
  `classifyRefusal`; `*unknownFeatureError` before `*scaffold.RefusalError` is load-bearing
  (mutation-verified M7, S05).
- **Paths: absolute in `assemble`/`scaffold`/JSON, relative in text (R6)** via
  `internal/cli/refusal.go`'s `displayPath(wd, p)`. (S04)
- A step-file frontmatter parse failure wraps as `*stepFrontmatterError{name, err}`; `newProblem`
  names the step file, fix `run 'brief check <feature>' to list every fault`. (S04)
- `assemble.FeatureStatus`: `Next *NextStep{ID,Title,Path}` (nil = no open step), `Path`
  absolute feature dir set on **every** row including Problem rows. `Next.Title` is
  `markdown.Title` after frontmatter; `Next.Path` = feature dir joined with
  `pattern.Name(n)`. `(FeatureStatus).Complete()` = `Problem == nil && Total > 0 &&
  Done == Total` — the one definition of "complete"; zero steps is NOT complete. (S06)
- `assemble.RenderStatusText` renders the FEATURE/DONE/BLOCKED/NEXT table via
  `text/tabwriter` (2-space pad, NEXT unpadded/last, header omitted for zero rows per R9).
  `runStatus` writes the table first (flushed), then one stderr line per malformed row,
  then always `statusSummary(rows)`. (S06)
- `status --json`'s success document is `statusDocument` in `internal/cli/status.go` (same
  placement as `startDocument`), branched on `out.json` **before** the zero-rows notice and
  any malformed-row/summary stderr write: zero stderr bytes on every success path (R1/R4).
  Malformed rows' `done`/`total`/`blocked` are `*int` nil in `statusFeatureJSON` only —
  `assemble.FeatureStatus` keeps plain ints. `features` is `[]` (never `null`) on zero rows;
  `next.title`/`problem.detail`/`fix` are raw; `problem.line` is always `null` (key present
  for an additive future `assemble.Problem.Line`). `features[].name` is the
  machine-readable "known features" list, narrower than `known:` (see Traps). (S07)
- Golden policy: one exact-bytes golden pins key order (`assert.Equal`, never `JSONEq` —
  testifylint's `encoded-compare` doesn't fire when the literal is built into a `want :=`
  variable first); matrix/decode tests check dynamic fields against a captured value, never
  a literal copied from production.

## Left unbuilt

- status/check/new/finish/--version/help JSON documents — S09/10/11/12/13 (status done,
  S07); until each lands that command runs its **text** path under `--json`.
- `completion … --json` usage-error document — S13.
- `--json` flag row in every command's help — S14. Only `start` registers it.
- `check`'s own text/JSON paths (`RenderFindings`) are still absolute, carry no stable
  `rule` id (R8), and print `:0` for a whole-file finding — S08.
- `new`'s stdout path and `new`/`finish` success stderr are still absolute — S10/S11.
- status help's "For scripts, use --json; the text layout may change." sentence, and a
  structured `known` array in the JSON error object — S14; a `--json` caller needing the
  known list uses `status --json`'s `features[].name` (S07).
- `assemble.Problem.Line` — unowned; no Problem source yields a line today.
- `assemble.RenderJSON` — no production caller; removing it is unowned.

## Traps

- Registering `--json` on every leaf instead of stripping it centrally breaks `brief --json`.
- `displayPath` must check `filepath.IsAbs` before calling `Rel`: finish's refusal path can
  be the user's own relative `--state`/`--handoff` argument.
- A reserved-name collision (`schema`, `command`, `ok`, `exit_code`, `error`) in a future
  payload struct is silently resolved by encoding/json's equal-depth rule.
- `known:` (unknown-feature errors) lists only openable directories via `assemble.Features`
  — a symlink or unopenable directory named like a feature appears in `status`'s table and
  `status --json`'s `features[]` but never in `known:`, pre-existing, never reconciled.
- `flattenTabwriterField`/`flattenOneLine` are text-only; a JSON payload must never reuse
  them — JSON carries the raw title/detail/fix even though a tab/newline would corrupt the
  text table unless flattened (mutation-verified, S07).
- Defining "complete" as `Next == nil` instead of via `Complete()` misreads a zero-step
  feature as complete (mutation-verified, S07).
- A `--json` success branch placed after any stderr-writing code lets that write happen in
  JSON mode too, breaking R1's zero-stderr guarantee — it must run before a success path's
  first possible stderr write (mutation-verified, S07).

## Open debts

- `assemble.RenderJSON` (see Left unbuilt) — unowned; dies unless re-opened.
- `scaffold.noSuchFeatureRefusal`'s dead `Problem`/`Fix` fields — unowned.
