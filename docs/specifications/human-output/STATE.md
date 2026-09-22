# human-output — current state

Scenarios complete: SCENARIO-01..14 (all). Last updated by a fix-mode pass: 3 MAJOR findings
(status/check parity with start's own refusals, finish `next` shape parity with status,
unknown-step refusal copy) plus 6 cheap-optional fixes.

## Binding decisions

- JSON mode = exact `--json` token before the first `--`, **stripped** by `scanJSONFlag` in
  `run()` ahead of cobra parsing. `--json=<v>` is always a text usage error; its fix hint
  appends `--json` to a leaf's own real invocation (`brief status --json`), not the bare
  invocation — the three generic fallbacks (`brief --help`, `brief new --help`,
  `brief help <command>`) stay bare.
- `reporter` (`internal/cli/json.go`) is the one per-Run output seam: `usageError`/`refusal`
  render R3's error document; success is a per-command `<cmd>Document` embedding `jsonHeader`
  first, by value, never nil slices (`[]` not `null`). `--json` always writes before any
  text-mode write (R1, mutation-verified). `reporter.document(v)` wraps a success document's own
  write error.
- `files_changed`: `null` for a read command; for a write command it is `errors.Is(err,
  scaffold.ErrPartialWrite)`, set at every write site, each mutation-verified individually.
- `classifyRefusal(err)` order: `*config.InvalidConfigError`, `*unknownFeatureError`,
  `*scaffold.RefusalError` (`errors.Is(refusal.Err, scaffold.ErrNoSuchStep)` renders through
  `layoutProblemOnly` — `"<problem>; <fix><tail>"`, path in `--json` only — every other
  `*scaffold.RefusalError` keeps `layoutPathProblem`), `*assemble.RefusalError`, generic
  `errorKindFailure` — load-bearing (mutation-verified).
- Unknown step (`finish` only): `no step "<id>" in <feature>; known: <a>, <b>, …`, the same
  `known:` convention `*unknownFeatureError` carries — built once in `scaffold.Finish`
  (`knownStepIDs`/`knownStepsFix`), not duplicated in cli. Empty: `known: none; run 'brief new
  step <feature>' to create one`.
- **Paths: absolute in `assemble`/`scaffold`/JSON, relative in text (R6)** via `displayPath`.
- `status.featureStatus` (now a `*Server` method) checks, per feature: open/list the directory,
  `s.specFault`, `s.readStateFile` (both reused from `Start`), then step files — the same faults
  `brief start` refuses over now degrade a status row into `Problem`. `check` agrees on
  zero-step severity too: `checkStepFindings`'s `inFlight` starts `true` when a feature has no
  step files (`Total > 0`, matching `Status.Complete()`), not vacuous WARN.
- `finish --json`'s `"next"` is `status`'s own `{"id","title","path"}|null` object
  (`scaffold.FinishNext`, via `nextOpenStep` + `stepTitleFromFile`); text still names only the
  id. `finish`/`new step` also carry additive `"modified"` (never nil): `finish` =
  `[state, step, spec]` on write, `[]` on the R11 no-op, `handoff_path` never included; `new
  step` = `[spec]`; `new feature` = `[]`.
- The frontmatter-parse-failure message is one wording everywhere (`assemble.newProblem`,
  `assemble.checkStepFindings`, `scaffold.Finish`): `"frontmatter does not parse: <underlying>"`.
- `status`'s NEXT column collapses to the id alone when `Next.Title` is empty **or equals
  `Next.ID`**. `check`'s "no findings" names the feature when given one.
- Golden policy: one exact-bytes golden pins key order (`assert.Equal`, never `JSONEq`);
  decode tables check dynamic fields against a captured value, never a literal.
  `versionString(readBuildInfo)` is the one version rule (`"(devel)"` fallback), shared text/JSON.
- Every help document comes from one `root.SetHelpFunc` wrapper; `command` is always `"help"`.
  **`--json` is a real pflag on every JSON-capable leaf** (`addJSONFlag`); `scanJSONFlag` still
  strips every `--json` before pflag runs (display-only registration). Every JSON-capable
  command's `Long` ends with a JSON paragraph from one shared trio in `cli.go`; `status`/`check`
  additionally carry `jsonScriptHint`.

## Left unbuilt

- `brief new --json` (bare `new`, no type) success document — it only ever errors.
- `assemble.Problem.Line` and `assemble.RenderJSON` — both unowned.
- A shared platform helper for "next open step" — lives once in `assemble`, once in `scaffold`,
  tied only by an agreement test (now also checks `next.path` against `status`'s own).
- A `blocked` flag in `finish`'s output, and a flag `shorthand` field in help entries.
- `--version` never appears in the help index; a JSON-mode hint in root's/`new`'s group help.
- `start --json`'s `shortfalls` renders `null`, not `[]`, when empty.
- New step's template lacks a `## Scenario` heading (scaffold bug, separate); start's own
  missing-state-file fix wording ("make it readable"); help `--json` has no json flag for bare
  `new`; a completion-message stutter; config accepting `state-file == specification-file`;
  `applyFinishWrites`'s param count / `landed`-`partial` naming (NITs) — all unowned.

## Traps

- Registering `--json` on every leaf instead of stripping it centrally breaks `brief --json`.
- `displayPath` must check `filepath.IsAbs` before `Rel`: finish's refusal path can be the
  user's own relative `--state`/`--handoff` argument.
- A reserved-name collision (`schema`, `command`, `ok`, `exit_code`, `error`) in a future payload
  struct is silently resolved by encoding/json's equal-depth rule.
- `known:` (unknown-feature/unknown-step) lists only openable dirs / real step files.
- `os.ReadDir` order is filename order — `nextOpenStep` picks the minimum by `pattern.Number`.
- pflag sorts a leaf's Flags rows by name: `--json` lands between `--help` and `--state` in
  finish's table.
- Every status fixture built from step files alone now needs a conforming spec + state file
  too, or `featureStatus`'s spec/state check wins the row's `Problem` for the wrong reason —
  `writeConformingFeature`/`writeConformingFeatureFiles` exist in both test packages for this.

## Open debts

- Everything in "Left unbuilt" above is unowned and dies unless re-opened.
- `scaffold.noSuchFeatureRefusal`'s dead `Problem`/`Fix` fields — unowned.
- A sticky write-error for a usage/refusal/help JSON write failure (today silently discarded) —
  unowned — dies unless re-opened.
- A success document's own write failure and a text-mode render failure exit with different
  codes/messages for the same underlying I/O fault — unowned asymmetry — dies unless re-opened.
- `writeExclusive` orphaning a file when `WriteString`/`Close` fails after `OpenFile` succeeds —
  correctness MINOR, not constructible via a returned error — unowned.
- `reporter.refusal`'s compose-method extraction — refactor MINOR — unowned.
