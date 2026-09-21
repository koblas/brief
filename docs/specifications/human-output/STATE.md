# human-output — current state

Scenarios complete: SCENARIO-01..05. Last updated by SCENARIO-05.

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
  `*assemble.RefusalError`, generic `errorKindFailure` (fix from `usageHint(cmd)`). No
  bare-sentinel branch remains — every not-found arrives pre-enriched. `errCheckFindings`
  never renders through `refusal` (R4).
- **Not-found is enriched at the call site, not in `classifyRefusal`.**
  `start.go`/`check.go`/`finish.go`/`new.go` (new step) each call
  `enrichUnknownFeature(ctx, cfg, root, feature, err)` before `out.refusal(...)`: recognizes
  `scaffold.ErrNoSuchFeature`/`assemble.ErrNoSuchFeature` anywhere in `err`'s chain, lists
  the feature directory via new `assemble.(*Server).Features` (real dirs only, `fs.ReadDir`
  order, `(nil,nil)` on a missing dir), wraps into `*unknownFeatureError{name, dir, known,
  err}` — `err` unchanged, `errors.Is` still reaches either sentinel. A listing failure
  returns in its own error's place, never rendered as `known: none`. Text: `<problem> in
  <dir>; known: <a>, <b>` or `known: none; run 'brief new feature <name>' to create it`,
  tail `(no files changed)` only when the wrapped sentinel is `scaffold.ErrNoSuchFeature`.
  New `refusalClassification.layout` field selects `textLine`'s shape. (S05)
- **Paths: absolute in `assemble`/`scaffold`/JSON, relative in text (R6)** via
  `internal/cli/refusal.go`'s `displayPath(wd, p)`. (S04)
- A step-file frontmatter parse failure wraps as `*stepFrontmatterError{name, err}`
  (`internal/assemble/errors.go`); `newProblem` names the step file, fix
  `run 'brief check <feature>' to list every fault`. A read failure (`*fs.PathError`) keeps
  `readClassFix` instead. (S04)
- Golden policy: one exact-bytes golden pins key order per document shape; matrix/decode
  tests check dynamic fields against a captured value, never a literal copied from
  production.

## Left unbuilt

- status/check/new/finish/--version/help JSON documents — S07/09/10/11/12/13; until each
  lands that command runs its **text** path under `--json`.
- `completion … --json` usage-error document — S13; today it prints the script regardless.
- `--json` flag row in every command's help — S14. Only `start` registers it.
- `check`'s own text/JSON paths (`RenderFindings`) are still absolute, and carry no stable
  `rule` id (R8) — S08.
- `status`'s stderr frame is still `brief status: <rel path>: <detail>; <fix>` — S06 changes
  it to name the feature, add the table and summary line.
- `new`'s stdout path and `new`/`finish` success stderr are still absolute — S10/S11.
- A structured `known` array in the JSON error object — not built; a `--json` caller needing
  the list uses `status --json` (S07).
- `assemble.RenderJSON` — no production caller now that start builds its own document;
  removing it is a refactor, **unowned by any scenario**.

## Traps

- Registering `--json` on every leaf instead of stripping it centrally breaks `brief --json`.
- `displayPath` must check `filepath.IsAbs` before calling `Rel`: finish's refusal path can
  be the user's own relative `--state`/`--handoff` argument.
- A reserved-name collision (`schema`, `command`, `ok`, `exit_code`, `error`) in a future
  payload struct is silently resolved by encoding/json's equal-depth rule.
- **`classifyRefusal` must test `*unknownFeatureError` before `*scaffold.RefusalError`.**
  `noSuchFeatureRefusal` (scaffold) returns a `*scaffold.RefusalError` wrapping
  `scaffold.ErrNoSuchFeature`, and `enrichUnknownFeature` wraps that whole error unchanged.
  Checking the scaffold type first lets `errors.AsType` find the *inner* type through
  `Unwrap` and silently keep the old path-less copy for `new step`/`finish` while
  `start`/`check` looked fixed. Mutation-verified (M7): reversing the order reddens exactly
  `new step`/`finish`.
- `scaffold.noSuchFeatureRefusal`'s own `Problem`/`Fix` copy is never rendered any more —
  `*unknownFeatureError` always wins first; editing it in `internal/scaffold/scaffold.go`
  changes nothing user-visible.
- `known:` lists only openable directories via `assemble.Features` — a symlink named like a
  feature appears in `brief status` (Problem row) but never in `known:`. S06/S07 must not
  "reconcile" the two lists into one.
- A directory `OpenRoot` cannot open (permission denied, symlink escape) still maps to
  not-found by `Start`/`NewStep`/`Finish`, so its own name then appears in `known:` —
  contradictory but pre-existing, unowned.

## Open debts

- `assemble.RenderJSON` (see Left unbuilt) — unowned; dies unless re-opened.
- `scaffold.noSuchFeatureRefusal`'s dead `Problem`/`Fix` fields (see Traps) — unowned; a
  future pass touching `internal/scaffold/scaffold.go` should consider deleting them.
