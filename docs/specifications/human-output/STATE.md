# human-output — current state

Scenarios complete: SCENARIO-01..08. Last updated by SCENARIO-08.

## Binding decisions

- JSON mode = exact `--json` token before the first `--`, **stripped** by `scanJSONFlag` in
  `run()` ahead of cobra parsing. `--json=<v>` is always a text usage error. (S01)
- `reporter` (`internal/cli/json.go`) is the one per-Run output seam, narrowed per command
  via `forCommand(cmd)`. `usageError`/`refusal(err)` render R3's error document; success
  documents are a per-command `<cmd>Document` embedding `jsonHeader` first, by value.
- `files_changed`: `false` for `new`, `new feature`, `new step`, `finish`; `null` otherwise
  (`filesChangedFor`).
- `classifyRefusal(err)` order: `*config.InvalidConfigError`, `*unknownFeatureError`,
  `*scaffold.RefusalError`, `*assemble.RefusalError`, generic `errorKindFailure` —
  `*unknownFeatureError` before `*scaffold.RefusalError` is load-bearing (mutation-verified).
- **Paths: absolute in `assemble`/`scaffold`/JSON, relative in text (R6)** via
  `displayPath(wd, p)`. `check`'s `runCheck` relativizes a **copy** of each finding's
  `Path`; `assemble.Check`/`GroupByFeature`/`RenderFindings` stay absolute — S09's
  `check --json` must build from the un-relativized findings. (S04, S08)
- A step frontmatter parse failure wraps as `*stepFrontmatterError{name, err}`; `newProblem`
  names the step file, fix `run 'brief check <feature>' to list every fault`. (S04)
- `assemble.FeatureStatus.Complete()` = `Problem == nil && Total > 0 && Done == Total` —
  zero steps is NOT complete. `RenderStatusText`/`RenderFindings` share
  `flattenTabwriterField`; each `run*` writes its table/groups, then stderr, always before
  returning. `status --json` branches **before** any success-path stderr write (R1); `[]`
  never `null` for zero rows. (S06, S07, S08)
- `assemble.Finding` carries `Rule`, `Severity`, `Path`, `Line`, `Detail`, `Feature`,
  `FeaturePath`, `InFlight`. Rule ids (R8 wire strings — never rename): `feature-symlink`,
  `feature-unreadable`, `spec-missing`, `spec-unreadable`, `state-missing`,
  `state-unreadable`, `steps-unlistable`, `step-unreadable`, `frontmatter`, `fence`,
  `heading`, `state-cap`, `handoff-cap`, `checklist`, `depends-on`. `fence`/`heading` each
  cover spec and state; `depends-on` covers self and dangling — the path disambiguates.
  `assemble.Check` keeps `([]Finding, error)`; grouping is `assemble.GroupByFeature` →
  `[]FeatureFindings{Name, Path, InFlight, Findings}`, which S09 reuses for `features[]`.
  Group header label is `InFlight`, **never severity**: both feature-level producers
  hard-code `InFlight: true` regardless of their feature's own doneness. (S08)
- `check`'s text rows omit the rule id (only in the stderr tally, and from S09 in JSON).
  Summary: ERROR/WARN never pluralized; `feature`/`features` pluralized on K (groups with
  findings); rule tally ordered count desc then rule id asc (never map order). The
  `ERRORs block finish…` clause fires only when an ERROR exists; the narrowing clause only
  when no feature arg was given and K > 1; joined `; <c1> — <c2>` when both apply. (S08)
- Golden policy: one exact-bytes golden pins key order (`assert.Equal`, never `JSONEq`);
  matrix/decode tests check dynamic fields against a captured value, never a production
  literal.

## Left unbuilt

- `new`/`finish`/`--version`/`help` JSON documents — S10/11/12/13; until each lands that
  command runs its **text** path under `--json`.
- `check`'s own `checkDocument`/`--json` payload (`counts{error,warn}`,
  `features[]{name,path,in_flight,findings[]}`) — S09, reusing `GroupByFeature` and the
  un-relativized findings.
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
- `flattenTabwriterField`/`flattenOneLine` are text-only; S09's `check --json` must carry
  the raw `detail`, not `RenderFindings`'s flattened one (mutation-verified).
- A zero-step feature with findings is `InFlight == false` → `(complete)` in `check`, even
  though `status`'s `Complete()` would say "not complete" — pre-existing, the two commands'
  definitions are not reconciled.
- A `--json` success branch placed after any stderr write breaks R1 (mutation-verified).
- A new feature-level `Finding` producer must stamp its own `Feature`/`FeaturePath`/
  `InFlight` itself, like `symlinkFeatureFinding`/`unreadableFeatureFinding` do — it never
  passes through `checkFeatureDir`'s stamping loop (mutation-verified: dropping the stamp
  there reddened exactly its own test). `Rule` likewise must be set at every `Finding{...}`
  site; an empty one renders as `N ` in the tally. Sorting the tally by rule id alone drops
  count-priority silently (both mutation-verified).

## Open debts

- `assemble.RenderJSON` (see Left unbuilt) — unowned; dies unless re-opened.
- `scaffold.noSuchFeatureRefusal`'s dead `Problem`/`Fix` fields — unowned.
