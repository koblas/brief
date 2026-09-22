# human-output — current state

Scenarios complete: SCENARIO-01..14 (all). Last fix-mode pass closed 2 MAJOR findings:
`validFeatureArgument` (both copies) let `""` through to `topRoot.OpenRoot("")`, so
`start ""` / `new step ""` / `finish "" …` printed an opaque `openat : empty path` failure
instead of the "no feature" refusal; and `assemble.Start`/`scaffold.NewStep`/`scaffold.Finish`'s
*first* `OpenRoot` site (the configured feature root itself) had no test proving its
non-`ErrNotExist` failure stays generic rather than "no such feature" (the second site,
feature's own subdirectory, already did).

## Binding decisions

- JSON mode = exact `--json` token before the first `--`, **stripped** by `scanJSONFlag` in
  `run()` ahead of cobra parsing. `--json=<v>` is always a text usage error; its fix hint
  appends `--json` to a leaf's own real invocation, not the bare invocation — the three generic
  fallbacks stay bare.
- `reporter` (`internal/cli/json.go`) is the one per-Run output seam: `usageError`/`refusal`
  render R3's error document; success is a per-command `<cmd>Document` embedding `jsonHeader`
  first, never nil slices. `--json` always writes before any text-mode write (R1,
  mutation-verified).
- `files_changed`: `null` for a read command; for a write command, `errors.Is(err,
  scaffold.ErrPartialWrite)`, set at every write site, each mutation-verified individually.
- `classifyRefusal(err)` order: `*config.InvalidConfigError`, `*unknownFeatureError`,
  `*scaffold.RefusalError` (`ErrNoSuchStep` renders through `layoutProblemOnly`, every other
  through `layoutPathProblem`), `*assemble.RefusalError`, generic `errorKindFailure` —
  load-bearing (mutation-verified).
- Unknown step (`finish` only): `no step "<id>" in <feature>; known: <a>, <b>, …`, the same
  `known:` convention `*unknownFeatureError` carries — built once in `scaffold.Finish`, not
  duplicated in cli.
- **Paths: absolute in `assemble`/`scaffold`/JSON, relative in text (R6)** via `displayPath`,
  which also appends `:<line>` to `status`'s stderr line when `Problem.Line > 0` (parity with
  `start`'s refusal text; mutation-verified).
- `status.featureStatus` checks, per feature, in fixed order: open/list the directory,
  `s.specFault`, `s.readStateFile`, then step files — first fault wins, mutation-verified by
  reordering **and** by a fixture dropping both spec and state (the only shape that
  distinguishes the two orders). `assemble.Problem` owns `Line` directly — `*RefusalError`
  embeds `Problem` with no shadowing `Line` (Go resolves a promoted field by name in a keyed
  composite literal, so no existing literal changed), so `status.go`'s two former
  `problem.Line = refusal.Line` hand-copies are gone. `check` agrees on zero-step severity:
  `inFlight` starts `true` on no step files, not vacuous WARN.
- `assemble.Start`, `scaffold.NewStep` and `scaffold.Finish` reject a feature argument via
  `validFeatureArgument` (duplicated, `assemble`/`scaffold` must not import each other) *before*
  either `os.Root.OpenRoot` call: a traversal name, a path-separator name, or `""` refuses as
  "no such feature" without depending on `OpenRoot`'s error shape (`assemble`'s `Check` guards
  its own call with `feature != ""` first, since an empty argument there means "every feature").
  Past that guard, only `errors.Is(err, fs.ErrNotExist)` on either `OpenRoot` call (the
  configured feature root, then feature's own subdirectory — both now covered by a
  not-a-directory test) is "no such feature"; any other failure is a generic wrapped failure,
  never misreported as not-found. `scaffold.openFeatureDir` is the shared seam (`NewStep`/
  `Finish`, extracted to keep `Finish` under `maintidx`, mutation-verified on both call sites);
  `assemble.Start` inlines its own copy.
- `scaffold.knownStepIDs` sorts by `pattern.Number`, not `os.ReadDir`'s filename order
  (mutation-verified). `findStepFile` returns `ErrNoSuchStep` unwrapped on no match, any other
  error wrapped and un-refused.
- `finish --json`'s `"next"` is `status`'s own `{"id","title","path"}|null` object; text still
  names only the id. `finish`/`new step` carry additive `"modified"` (never nil). The
  frontmatter-parse-failure message is one wording everywhere: `"frontmatter does not parse:
  <underlying>"`.
- `status`'s NEXT column collapses to the id alone when `Next.Title` is empty or equals
  `Next.ID`. Golden policy: one exact-bytes golden pins key order (`assert.Equal`, never
  `JSONEq`). `--json` is a real pflag on every JSON-capable leaf; `scanJSONFlag` still strips it
  before pflag runs.

## Left unbuilt

- `brief new --json` (bare `new`) success document; `assemble.RenderJSON` — unowned.
- A shared platform helper for "next open step" — lives once in `assemble`, once in `scaffold`,
  tied only by an agreement test.
- A `blocked` flag in `finish`'s output; `--version` absent from the help index; `start --json`'s
  `shortfalls` renders `null` not `[]` when empty; new step's template lacks a `## Scenario`
  heading; config accepting `state-file == specification-file` — all unowned NITs.
- Review-label narrative ("MAJOR N", "reviewer's finding") in test comments outside this pass's
  cited scope (`internal/cli`, `internal/platform/markdown`, `internal/platform/stepfile`) was
  left as-is.

## Traps

- Registering `--json` on every leaf instead of stripping it centrally breaks `brief --json`.
- `displayPath` must check `filepath.IsAbs` before `Rel`: finish's refusal path can be the
  user's own relative `--state`/`--handoff` argument.
- `known:` lists only openable dirs / real step files, sorted by `pattern.Number` for steps —
  never `os.ReadDir`'s filename order.
- A status fixture built from step files alone needs a conforming spec + state file too, else
  `featureStatus`'s spec/state check wins the row's `Problem` first; a fixture proving check
  *order* between two faults must drop both, not just one.
- A traversal feature name ("../x") or `""` against `os.Root.OpenRoot` is **not**
  `fs.ErrNotExist` (confirmed empirically) — narrowing an open-failure mapping to
  `fs.ErrNotExist` alone requires rejecting both earlier, in `validFeatureArgument`, or it
  silently stops refusing them.

## Open debts

- Everything in "Left unbuilt" above is unowned and dies unless re-opened.
- `scaffold.noSuchFeatureRefusal`'s dead `Problem`/`Fix` fields — unowned.
- A sticky write-error for a usage/refusal/help JSON write failure (today silently discarded),
  and the write-failure/render-failure exit-code asymmetry — both unowned.
- `writeExclusive` orphaning a file when `WriteString`/`Close` fails after `OpenFile` succeeds —
  correctness MINOR, not constructible via a returned error — unowned.
- `scaffold.findStepFile`'s `ReadDir`-failure branch has no deterministic test seam in
  `scaffold` (unlike `assemble`'s `SetReadDirForTest`) — fixed but unverified by mutation —
  unowned.
