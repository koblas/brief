# SCENARIO-15 Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- **`--json` is on `start` and nowhere else.** *Decisions taken* 3 and open question 8 defer
  `status`/`next`/`show`/`state get`/`handoff`; the rule of thumb for adding it later is
  "structured payload or record set, not a scalar".
- **The payload is `json.Marshal` of `assemble.Brief` — nil marshals to `null`, everywhere,
  with no normalization.** `"step": null` (R14's discriminator), `"shortfalls": null` when
  none. Any later `--json` surface inherits this rule rather than re-deciding it; emitting
  `[]` on one surface and `null` on another is the thing this forbids.
- **No field carries `omitempty`.** `done`/`open` at `0` are what separate a complete feature
  (`2`/`0`) from one with no step files (`0`/`0`) — `"step": null` alone cannot. `found` at
  `false` is what separates an absent heading from an empty body (SCENARIO-14).
- **Under `--json`, stdout is one compact document plus `\n`, or zero bytes.** All notices —
  12's complete/no-steps lines, 14's shortfall lines — stay on stderr. A refusal is still a
  plain R14a stderr line at exit 1 with **no** JSON envelope, because R14a fixes one refusal
  template and a script distinguishes refused from finished by exit code alone.
- **Exit codes are untouched by `--json`:** 0 open-step / complete / no-steps / shortfall, 1
  malformed or unknown feature, 2 usage.
- **`RenderJSON` always writes a document; `RenderText` writes nothing when `Step` is nil.**
  JSON also keeps sections `RenderText` omits for an empty body. The renderers are not
  interchangeable.
- **`start` accepts `--json` before or after the feature.** `splitLeadingPositionals` (moved
  to `cli.go`) only peels the leading run; `runStart` merges that leading run with
  `fs.Args()` via `slices.Concat` after `Parse` so a trailing `--json` is not counted as an
  extra positional. `finish` never needed this merge because its feature and step always
  precede its flags — do not backport the merge there without a reason.

**Left unbuilt** — named so nobody assumes it exists:

- `status --json`, and `--json` on `next`/`show`/`state get`/`handoff` — deferred by
  *Decisions taken* 3 / open question 8, not missed.
- R13's output budget and truncation — still unowned. `cfg.DefaultOutputBudgetBytes` (8192)
  remains unconsumed. Its eventual owner must never truncate a `--json` payload — a
  line-boundary cut yields an unparseable document; the budget has to skip or refuse a
  structured payload, not trim one.
- No JSON error/refusal envelope, no `"feature"` key in the payload, no schema-version key.
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`; 16–21's `finish` refusals; 22's
  `check`.

**Traps** — things that look right and are not:

- **The struct tags in `brief.go` are the wire contract.** Renaming a `Brief`, `Step`,
  `Section` or `Shortfall` field now silently changes user-visible JSON unless the tag is
  kept — and without tags Go emits `Done`/`Open`/`Step`/`Heading`, a different contract again.
- `brief start <f> | wc -c == 0` is **not** the completion test under `--json` — stdout is
  never empty on an exit-0 `--json` run. The structured test is `"step": null` plus
  `done`/`open`, per SCENARIO-12's trap.
- `SetEscapeHTML(false)` is deliberate; a plain `json.Marshal` would rewrite `<` and `&` in
  markdown bodies to `<`/`&`. A test pins it.
- `start --json --help` prints the usage text to stdout, not JSON. Intended, not a stdout
  purity bug.
- A feature name starting with `-` is taken as a flag by `splitLeadingPositionals` —
  pre-existing for `finish`, now shared by `start`, unowned.
- Unmarshalling **collapses** null-vs-absent and absent-vs-zero: a test that only decodes
  into a struct or map cannot prove `"step": null` is present rather than omitted, and an
  omitted `"found"` decodes to `false` either way. Those four claims (`step`, `done`/`open`,
  `shortfalls`, `found`) are asserted on raw bytes on purpose; do not "clean them up" into
  decoded comparisons.

## Session notes

- Measured baseline `brief start --json demo` gave `flag provided but not defined: -json`,
  exit 2, confirmed before any change.
- The advisor caught a real defect in the plan before implementation: `finish` uses
  `splitLeadingPositionals` and keeps only the **leading run** as its positionals (`rest,
  flagArgs := splitLeadingPositionals(args)`), because its feature and step always precede
  its flags. `start` cannot copy that pattern unmodified — for `["demo","--json"]` the
  leading run is `["demo"]`, which is correct, but for `["--json","demo"]` the leading run is
  `[]` (the first token starts with `-`), and the feature name only shows up in
  `fs.Args()` **after** `fs.Parse` consumes `--json`. Using only the leading run would
  report "no feature given" for the flag-before-feature form the measured baseline used.
  Fixed by merging both sources: `rest := slices.Concat(leading, fs.Args())`. Verified by
  hand-tracing five argument shapes before implementing (`--json` before/after the feature,
  and three "too many arguments" shapes with `--json` in each position), all now covered by
  tests.
- Steps 4–8 (assemble) and 11–17 (cli) were all green on arrival: `RenderJSON` (step 3) is a
  generic `json.Marshal`/`Encode` over tagged fields, not per-field hand logic, so once the
  tags and the encoder were right, every subsequent pinning test passed without further
  production changes. Genuine red-green cycles happened at: step 1 (compile-fail on
  `RenderJSON` undefined), step 9 (`flag provided but not defined: -json`, matching the
  measured baseline), and the three mutation-verification passes (step 19).
- Mutation verification used `cp`/`diff` against `$TMPDIR`, not `git stash` — the worktree
  reminder in this session states the stash stack is shared across worktrees and other
  sessions may pop concurrently.
- A second advisor pass caught that `Test_start_json_keeps_stdout_parseable_when_a_convention_is_missing`
  asserted `Step.Acceptance.Found == false` but never checked that the shortfall itself
  reached the JSON payload — only that it reached stderr. Added
  `require.Len(t, got.Shortfalls, 1)` plus a path assertion, then mutation-verified it by
  deleting the acceptance-shortfall `append` in `assemble.Start` (`internal/assemble/assemble.go`):
  both the new `Shortfalls` assertion and the pre-existing stderr-line assertion went red;
  restored and diffed byte-identical.

## Verification

- `go build ./...` — exit 0.
- `go test ./...` — exit 0, all packages ok.
- `go test -race ./internal/cli/... ./internal/assemble/...` — exit 0.
- `golangci-lint run ./...` — 0 issues.
- Test count: 335 passing (`grep -c "^--- PASS"`), delta **+15** from the 320 baseline,
  matching 6 new tests in `internal/assemble` + 9 new tests in `internal/cli` exactly (the
  `^` anchor excludes subtests, so the count tracks new `func Test_` definitions, not
  `t.Run` cases). 0 skips.
- `go doc ./internal/assemble RenderJSON` and `go doc ./internal/assemble Brief` read as the
  contract a caller needs, without opening the source.
