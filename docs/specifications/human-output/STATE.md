# human-output — current state

Scenarios complete: SCENARIO-01..09. Last updated by SCENARIO-09.

## Binding decisions

- JSON mode = exact `--json` token before the first `--`, **stripped** by `scanJSONFlag` in
  `run()` ahead of cobra parsing. `--json=<v>` is always a text usage error. (S01)
- `reporter` (`internal/cli/json.go`) is the one per-Run output seam, narrowed per command
  via `forCommand(cmd)`. `usageError`/`refusal(err)` render R3's error document; success
  documents are a per-command `<cmd>Document` embedding `jsonHeader` first, by value.
  `(reporter).headerFor(exitCode)` builds that header at any code; `successHeader()` is
  `headerFor(0)`. `newJSONHeader` alone derives `ok` — a success-shaped doc at a non-zero
  exit (check --json with an ERROR) must use `headerFor`, never `successHeader`. (S09)
- `files_changed`: `false` for `new`, `new feature`, `new step`, `finish`; `null` otherwise
  (`filesChangedFor`).
- `classifyRefusal(err)` order: `*config.InvalidConfigError`, `*unknownFeatureError`,
  `*scaffold.RefusalError`, `*assemble.RefusalError`, generic `errorKindFailure` —
  `*unknownFeatureError` before `*scaffold.RefusalError` is load-bearing (mutation-verified).
- **Paths: absolute in `assemble`/`scaffold`/JSON, relative in text (R6)** via
  `displayPath(wd, p)`. `check`'s text path relativizes a **copy** of each finding's `Path`
  (`displayFindings`); its JSON branch builds `checkDocument` from the un-relativized
  `GroupByFeature(findings)` directly, sitting *before* `displayFindings` runs. (S04, S08, S09)
- A step frontmatter parse failure wraps as `*stepFrontmatterError{name, err}`; `newProblem`
  names the step file, fix `run 'brief check <feature>' to list every fault`. (S04)
- `assemble.FeatureStatus.Complete()` = `Problem == nil && Total > 0 && Done == Total` —
  zero steps is NOT complete. Every `run*` writes its table/groups, then stderr, always
  before returning; `status --json`/`check --json` both branch **before** any success-path
  stderr write (R1) — an ordering violation is mutation-verified for both. `[]` never `null`
  for zero rows/groups. (S06, S07, S08, S09)
- `assemble.Finding{Rule, Severity, Path, Line, Detail, Feature, FeaturePath, InFlight}` —
  `Rule` (R8) and `Severity` (`"ERROR"`/`"WARN"`) render verbatim as `check --json`'s
  `rule`/`severity`. `GroupByFeature` order is `check --json`'s `features[]` order (Check's
  emission order, never sorted); `InFlight` is the group-header label, never severity — both
  feature-level producers (`feature-symlink`, `feature-unreadable`) hard-code `InFlight:true`
  regardless of doneness, so an `in_flight:false` fixture needs a file-level finding on a
  done feature instead. `rule`/`detail` in JSON are raw, never `flattenTabwriterField`/
  `flattenOneLine` (text-only); `Line *int` is `nil` iff `Finding.Line == 0`. (S08, S09)
- `countFindings(groups) (int, int)` is the single ERROR/WARN tally shared by
  `checkSummary`'s text line, `checkDocument.Counts` and the `errCheckFindings` exit
  decision — the three can never disagree. (S09)
- Golden policy: one exact-bytes golden pins key order (`assert.Equal`, never `JSONEq`);
  matrix/decode tests check dynamic fields against a captured value, never a production
  literal.

## Left unbuilt

- `new`/`finish`/`--version`/`help` JSON documents — S10/11/12/13; until each lands that
  command runs its **text** path under `--json`.
- `completion … --json` usage-error document — S13.
- `--json` flag row in every command's help, and status/check's "For scripts, use --json;
  the text layout may change." sentence — S14. Only `start` registers `--json` in help today.
- `new`'s stdout path and `new`/`finish` success stderr are still absolute — S10/S11.
- `assemble.Problem.Line` and `assemble.RenderJSON` — both unowned.

## Traps

- Registering `--json` on every leaf instead of stripping it centrally breaks `brief --json`.
- `displayPath` must check `filepath.IsAbs` before calling `Rel`: finish's refusal path can
  be the user's own relative `--state`/`--handoff` argument.
- A reserved-name collision (`schema`, `command`, `ok`, `exit_code`, `error`) in a future
  payload struct is silently resolved by encoding/json's equal-depth rule.
- `known:` (unknown-feature errors) lists only openable directories via `assemble.Features`
  — a symlink/unopenable dir named like a feature appears in `status`'s rows but never in
  `known:`, pre-existing, never reconciled.
- A zero-step feature with findings is `InFlight == false` → `(complete)` in `check`, even
  though `status`'s `Complete()` would say "not complete" — pre-existing, not reconciled.
- A new feature-level `Finding` producer must stamp its own `Feature`/`FeaturePath`/
  `InFlight` itself, like `symlinkFeatureFinding`/`unreadableFeatureFinding` do — it never
  passes through `checkFeatureDir`'s stamping loop (mutation-verified). `Rule` likewise must
  be set at every `Finding{...}` site; an empty one renders as `N ` in the tally. Sorting the
  tally by rule id alone drops count-priority silently (both mutation-verified).
- `errCheckFindings` is returned bare from both branches of `runCheck`; nothing in `Run`
  renders it — routing it through `out.refusal` would add an error object and break R4
  (findings are data, even at exit 1). S09's own golden fixture has no multi-line detail, so
  a `flattenOneLine`'d detail would NOT redden it (mutation ran clean, not red) — a future
  fixture with a multi-line detail must add its own guard, not assume this one proves it.

## Open debts

- `assemble.RenderJSON` (see Left unbuilt) — unowned; dies unless re-opened.
- `scaffold.noSuchFeatureRefusal`'s dead `Problem`/`Fix` fields — unowned.
