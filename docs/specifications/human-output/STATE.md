# human-output — current state

Scenarios complete: SCENARIO-01..12. Last updated by SCENARIO-12.

## Binding decisions

- JSON mode = exact `--json` token before the first `--`, **stripped** by `scanJSONFlag` in
  `run()` ahead of cobra parsing. `--json=<v>` is always a text usage error. (S01)
- `reporter` (`internal/cli/json.go`) is the one per-Run output seam. `usageError`/
  `refusal(err)` render R3's error document; success documents are a per-command
  `<cmd>Document` embedding `jsonHeader` first, by value. `successHeader()` is `headerFor(0)`;
  a non-zero-exit success doc (check --json with an ERROR) uses `headerFor` directly. (S09)
- `files_changed` (`filesChangedFor`): `false` for `new`, `new feature`, `new step`, `finish`;
  `null` otherwise.
- `classifyRefusal(err)` order: `*config.InvalidConfigError`, `*unknownFeatureError`,
  `*scaffold.RefusalError`, `*assemble.RefusalError`, generic `errorKindFailure` —
  `*unknownFeatureError` before `*scaffold.RefusalError` is load-bearing (mutation-verified).
- **Paths: absolute in `assemble`/`scaffold`/JSON, relative in text (R6)** via
  `displayPath(wd, p)` — every command goes through it, no private `filepath.Rel` copies.
  (S04, S08-S11)
- Every `run*` writes its full success payload, then stderr, before returning; `--json` always
  runs **before** any text-mode write (R1, mutation-verified). JSON slices are never nil —
  `[]`, not `null`. (S06-S11)
- `scaffold.NewFeature`/`NewStep` return `Result{Feature, Step, Path, Created}`; `scaffold.
  Finish` returns `FinishResult{Feature, Step, Changed, HandoffPath, StatePath, Next}` — same
  "absolute, verbatim into JSON" convention, different type. (S10, S11)
- Text-mode success stderr, one line after stdout, per command; see `new.go`/`finish.go`
  doc comments for the exact wording — no-op finish prints only "already done with identical
  inputs; nothing written". (S10, S11)
- `finish --json`: `{header, feature, step, changed, handoff_path, state_path, next}`. `next`
  is `*string` — null, not omitted, even on the no-op, where the *text* line omits it entirely
  (deliberate asymmetry). `changed` false only on the no-op; `state_path` is the replaced file,
  never the `--state` input source. (S11)
- `next` = lowest-numbered step file whose status isn't done, **depends-on ignored** —
  `assemble.Start`'s rule, duplicated in `scaffold.nextOpenStep` (they may not import each
  other); `Test_finish_next_agrees_with_start` pins the two in agreement. Computed before any
  write, from the `[]os.DirEntry` already read — no second `ReadDir`. (S11)
- Golden policy: one exact-bytes golden pins key order (`assert.Equal`, never `JSONEq`);
  decode tables check dynamic fields against a captured value, never a literal.
- `versionString(readBuildInfo)` is the one version rule; `versionLine` = `"brief " +
  versionString`, and `--version --json`'s `versionDocument{header, version}` field is the
  bare `versionString` — never disagree on the `(devel)` fallback. Always exits 0. The
  sole-argument relaxation is `scanJSONFlag` stripping, not a special case in `runRoot`:
  `runRoot` still requires `len(args) == 1` after stripping, so any other trailing token is
  still "takes no arguments"; a value on `--version` still wins over a trailing argument. (S12)

## Left unbuilt

- `help`/`completion --json` documents — S13; those commands run their **text** path under
  `--json` until then.
- `brief new --json` (bare `new`, no type) success document — it only ever errors.
- `--json` help row on every command, and status/check's "For scripts, use --json…" sentence
  — S14. Only `start` registers `--json` in help today.
- `assemble.Problem.Line` and `assemble.RenderJSON` — both unowned.
- A shared platform helper for the "next open step" rule is unbuilt — it lives once in
  `assemble`, once in `scaffold`, tied only by the S11 agreement test.
- A `blocked` flag in `finish`'s output does not exist; the ruled JSON shape has none.

## Traps

- Registering `--json` on every leaf instead of stripping it centrally breaks `brief --json`.
- `displayPath` must check `filepath.IsAbs` before calling `Rel`: finish's refusal path can be
  the user's own relative `--state`/`--handoff` argument.
- A reserved-name collision (`schema`, `command`, `ok`, `exit_code`, `error`) in a future
  payload struct is silently resolved by encoding/json's equal-depth rule.
- `known:` lists only openable dirs — an unopenable dir appears in `status` but never
  `known:`. A zero-step feature with findings is `InFlight == false` → `(complete)` in
  `check`, though `status.Complete()` says otherwise. Both pre-existing.
- A new feature-level `Finding` producer must stamp its own `Feature`/`FeaturePath`/
  `InFlight`/`Rule` itself, at every `Finding{...}` site (mutation-verified).
- On macOS `t.TempDir()` sits under a symlinked `/var`; build expected absolute paths from the
  same `wd` passed to `cli.Run`, never `filepath.EvalSymlinks`.
- `os.ReadDir` order is filename order — `nextOpenStep` picks the minimum by `pattern.Number`
  itself; a sibling with unparseable frontmatter counts as not done and can be named `next`,
  even though `brief start` then refuses the whole feature — truthful, but the two disagree.
- `scaffold.Finish` sits right at golangci-lint's `maintidx` budget (already extracted into
  `applyFinishWrites`); a future addition should extract another helper, not inline more.
- A go test binary's own `debug.ReadBuildInfo` reports `(devel)`, byte-identical to
  `versionString`'s fallback — a black-box `cli.Run` test cannot tell pass-through from
  fallback. Pin either only through unexported `run` with a fake reader, or against a value
  captured from `--version` in the same binary, never a literal. (S12)

## Open debts

- `assemble.RenderJSON` (see Left unbuilt) — unowned; dies unless re-opened.
- `scaffold.noSuchFeatureRefusal`'s dead `Problem`/`Fix` fields — unowned.
