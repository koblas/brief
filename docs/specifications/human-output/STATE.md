# human-output — current state

Scenarios complete: SCENARIO-01..11. Last updated by SCENARIO-11.

## Binding decisions

- JSON mode = exact `--json` token before the first `--`, **stripped** by `scanJSONFlag` in
  `run()` ahead of cobra parsing. `--json=<v>` is always a text usage error. (S01)
- `reporter` (`internal/cli/json.go`) is the one per-Run output seam. `usageError`/
  `refusal(err)` render R3's error document; success documents are a per-command
  `<cmd>Document` embedding `jsonHeader` first, by value. `successHeader()` is
  `headerFor(0)`; a success doc at non-zero exit (check --json with an ERROR) uses
  `headerFor` directly. (S09)
- `files_changed` (`filesChangedFor`): `false` for `new`, `new feature`, `new step`,
  `finish`; `null` otherwise.
- `classifyRefusal(err)` order: `*config.InvalidConfigError`, `*unknownFeatureError`,
  `*scaffold.RefusalError`, `*assemble.RefusalError`, generic `errorKindFailure` —
  `*unknownFeatureError` before `*scaffold.RefusalError` is load-bearing (mutation-verified).
- **Paths: absolute in `assemble`/`scaffold`/JSON, relative in text (R6)** via
  `displayPath(wd, p)` — every command goes through it, no private `filepath.Rel` copies.
  (S04, S08-S11)
- Every `run*` writes its full success payload, then stderr, before returning; `--json`
  always runs **before** any text-mode write (R1, mutation-verified). JSON slices are never
  nil — `[]`, not `null`. (S06-S11)
- `scaffold.NewFeature`/`NewStep` return `Result{Feature, Step, Path, Created}`; `scaffold.
  Finish` returns a separate `FinishResult{Feature, Step, Changed, HandoffPath, StatePath,
  Next}` — same "absolute, verbatim into JSON" convention, different type. (S10, S11)
- Text-mode success stderr, one line after stdout: `new feature`/`new step` — "created <name>
  (<spec rel>, <state rel>); add a step with…" / "created <id> in <feature>; fill in its
  acceptance criteria…, then 'brief start <feature>'". `finish` — "<f> <s> done; wrote
  <handoff rel>, replaced <state rel>; next: <id> — run 'brief start <f>'" / "…; <f> is
  complete" / no-op: "<f> <s> already done with identical inputs; nothing written". (S10, S11)
- `finish --json`: `{header, feature, step, changed, handoff_path, state_path, next}`.
  `next` is `*string` — null, not omitted, even on the no-op, where the *text* line omits it
  entirely; deliberate asymmetry, do not harmonize. `changed` false only on the no-op.
  `state_path` is the replaced state file, never the `--state` input source. (S11)
- `next` = lowest-numbered step file (`stepfile.Pattern.Number`) whose frontmatter status
  isn't done, finished step counted done, **depends-on ignored** (a blocked step can be
  next) — `assemble.Start`'s rule, duplicated in `scaffold.nextOpenStep` since assemble and
  scaffold may not import each other. `Test_finish_next_agrees_with_start` (internal/cli)
  pins the two in agreement. Computed in `Finish`'s validation phase, before any write, from
  the `[]os.DirEntry` `findStepFile` already read — no second `ReadDir`, no error return.
  (S11)
- Golden policy: one exact-bytes golden pins key order (`assert.Equal`, never `JSONEq`);
  decode tables check dynamic fields (ids, generated paths) against a captured value, never a
  literal.

## Left unbuilt

- `--version`/`help`/`completion --json` documents — S12/S13; those commands run their
  **text** path under `--json` until then.
- `brief new --json` (bare `new`, no type) success document — it only ever errors.
- `--json` help row on every command, and status/check's "For scripts, use --json…" sentence
  — S14. Only `start` registers `--json` in help today.
- `assemble.Problem.Line` and `assemble.RenderJSON` — both unowned.
- A shared platform helper for the "next open step" rule is unbuilt — it lives once in
  `assemble`, once in `scaffold`, tied only by the S11 agreement test.
- A `blocked` flag in `finish`'s output does not exist; the ruled JSON shape has none.

## Traps

- Registering `--json` on every leaf instead of stripping it centrally breaks `brief --json`.
- `displayPath` must check `filepath.IsAbs` before calling `Rel`: finish's refusal path can
  be the user's own relative `--state`/`--handoff` argument.
- A reserved-name collision (`schema`, `command`, `ok`, `exit_code`, `error`) in a future
  payload struct is silently resolved by encoding/json's equal-depth rule.
- `known:` lists only openable dirs (`assemble.Features`) — an unopenable dir appears in
  `status` but never `known:`. A zero-step feature with findings is `InFlight == false` →
  `(complete)` in `check`, though `status.Complete()` says otherwise. Both pre-existing.
- A new feature-level `Finding` producer must stamp its own `Feature`/`FeaturePath`/
  `InFlight`/`Rule` itself, at every `Finding{...}` site (mutation-verified).
- On macOS `t.TempDir()` sits under a symlinked `/var`; build expected absolute paths from
  the same `wd` passed to `cli.Run`, never `filepath.EvalSymlinks`.
- `os.ReadDir` order is filename order — `nextOpenStep` picks the minimum by
  `pattern.Number` itself; a sibling with unparseable frontmatter counts as not done and can
  be named `next`, even though `brief start` then refuses the whole feature — truthful, but
  the two disagree on that one tree.
- `scaffold.Finish` sits right at golangci-lint's `maintidx` budget (its four writes are
  already extracted into `applyFinishWrites` for this reason); a future addition should
  extract another helper rather than inline more branches into it.

## Open debts

- `assemble.RenderJSON` (see Left unbuilt) — unowned; dies unless re-opened.
- `scaffold.noSuchFeatureRefusal`'s dead `Problem`/`Fix` fields — unowned.
