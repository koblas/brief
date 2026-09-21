# human-output — current state

Scenarios complete: SCENARIO-01..10. Last updated by SCENARIO-10.

## Binding decisions

- JSON mode = exact `--json` token before the first `--`, **stripped** by `scanJSONFlag` in
  `run()` ahead of cobra parsing. `--json=<v>` is always a text usage error. (S01)
- `reporter` (`internal/cli/json.go`) is the one per-Run output seam, narrowed per command
  via `forCommand(cmd)`. `usageError`/`refusal(err)` render R3's error document; success
  documents are a per-command `<cmd>Document` embedding `jsonHeader` first, by value.
  `(reporter).headerFor(exitCode)` builds that header at any code; `successHeader()` is
  `headerFor(0)`. `newJSONHeader` alone derives `ok` — a success-shaped doc at a non-zero
  exit (check --json with an ERROR) must use `headerFor`, never `successHeader`. (S09)
- `files_changed` (`filesChangedFor`): `false` for `new`, `new feature`, `new step`,
  `finish`; `null` otherwise.
- `classifyRefusal(err)` order: `*config.InvalidConfigError`, `*unknownFeatureError`,
  `*scaffold.RefusalError`, `*assemble.RefusalError`, generic `errorKindFailure` —
  `*unknownFeatureError` before `*scaffold.RefusalError` is load-bearing (mutation-verified).
- **Paths: absolute in `assemble`/`scaffold`/JSON, relative in text (R6)** via
  `displayPath(wd, p)` — every command, including `new.go`, goes through it; no private
  `filepath.Rel` copies. `check`'s text path relativizes a **copy** of each finding's `Path`;
  its JSON branch builds `checkDocument` from the un-relativized findings directly. (S04, S08,
  S09, S10)
- Every `run*` writes its full success payload (table/groups/stdout-path), then stderr,
  always before returning; every `--json` branch runs **before** any success-path
  stdout/stderr write (R1) — mutation-verified for `status`, `check`, `new feature`, `new
  step`. Slices in JSON are never nil, so zero rows render `[]`, not `null`. (S06-S10)
- `assemble.Finding`/`GroupByFeature`/`countFindings` carry `check`'s rule/severity/tally
  contract (Check's own emission order, `InFlight` hard-coded per feature-level producer,
  one shared ERROR/WARN tally). (S08, S09)
- `scaffold.NewFeature`/`NewStep` return `Result{Feature, Step, Path, Created}` instead of a
  bare path. `Step` is `""` for `NewFeature`; the id exists only inside scaffold
  (`pattern.ID(next)`) — cli must never recompile `step-file-pattern` to recover it.
  `Created` lists exactly the files each call **wrote into existence**, in write order —
  `NewFeature`: `[spec, state]`; `NewStep`: `[step file]` only, never the specification it
  merely modifies. `Path` is always the single path the text-mode contract prints. (S10)
- `new feature`/`new step --json`: `newDocument{jsonHeader, Feature, Step *string, Path,
  Created}` — `Step` `nil` for `new feature` (only `new.go` enforces this; `scaffold.Result`
  carries no such guarantee), the id for `new step`; `path`/`created[]` are `res.Path`/
  `res.Created` verbatim (already absolute). Text-mode success stderr (one line, after
  stdout): `brief new feature: created <name> (<spec rel>, <state rel>); add a step with
  'brief new step <name>'` / `brief new step: created <id> in <feature>; fill in its
  acceptance criteria and checklist, then 'brief start <feature>'`. S14 must not describe
  different copy. (S10)
- Golden policy: one exact-bytes golden pins key order (`assert.Equal`, never `JSONEq`);
  matrix/decode tests check dynamic fields against a captured value, never a production
  literal. A step id can't be pinned as a literal — capture it from the created file's name,
  or from a sibling fresh feature's text-mode run (both are their feature's first step).

## Left unbuilt

- `finish`/`--version`/`help` JSON documents — S11/12/13; until each lands that command runs
  its **text** path under `--json`. `finish`'s success stderr is still absolute — S11.
- `brief new --json` (bare `new`, no type) success document — it only ever errors; nothing to
  build.
- `completion … --json` usage-error document — S13.
- `--json` flag row in every command's help, and status/check's "For scripts, use --json;
  the text layout may change." sentence — S14. Only `start` registers `--json` in help today.
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
  `InFlight` itself — it never passes through `checkFeatureDir`'s stamping loop
  (mutation-verified). `Rule` likewise must be set at every `Finding{...}` site; sorting the
  tally by rule id alone drops count-priority silently (both mutation-verified).
- `errCheckFindings` is returned bare from both branches of `runCheck`; routing it through
  `out.refusal` would add an error object and break R4 (findings are data, even at exit 1).
- On macOS `t.TempDir()` sits under a symlinked `/var`; build expected absolute paths from
  the same `wd` passed to `cli.Run`, never from `filepath.EvalSymlinks`.

## Open debts

- `assemble.RenderJSON` (see Left unbuilt) — unowned; dies unless re-opened.
- `scaffold.noSuchFeatureRefusal`'s dead `Problem`/`Fix` fields — unowned.
