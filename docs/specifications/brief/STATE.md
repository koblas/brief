# brief — current state

Scenarios complete: SCENARIO-01, SCENARIO-02, SCENARIO-03. Last updated by SCENARIO-03.

## Binding decisions

- Config (`internal/platform/config`): filename exactly `.brief.yaml`, upward walk, nearest wins,
  no merging, missing anywhere = `config.Default()`. Consumers take `Config` as a parameter; only
  `Resolve`/`decodeConfig` call `Default()`. Now carries `ChecklistHeading` (default `"##
  Implementation Plan"`) too. `Resolve(startDir) (Config, string, error)`; error is
  `errors.Is`-comparable to `ErrInvalidConfig`; `errors.AsType[*config.InvalidConfigError]` reaches
  `Path`/`Err`. (01/02/03)
- **`internal/platform/stepfile` is the sole naming/matching authority** every reader
  (`start`/`status`/`next`/`check`/`finish`) must use — never re-derive a looser parse. `Compile`
  refuses (`ErrInvalidPattern`): zero or 2+ integer verbs, any other `%`, empty/`.`/`..`, or a path
  separator. `Pattern.Name(n)`, `ID(n)` (name minus extension), `Number(filename) (int, bool)`
  **matches strictly by round trip** — `Sprintf(pattern, n) == filename` exactly;
  `SCENARIO-7.md` under `%02d` is not a step file (near-miss → future `check`/22 finding). (03)
- **`atomicfile.WriteFile(root *os.Root, name, data, perm)`** — pulled forward from SCENARIO-05
  (`## Phasing` authorized it pre-crossover; 05 *uses* this, doesn't reinvent it). Deterministic
  `.<name>.brief-tmp` sibling, `O_TRUNC` not `O_EXCL`, target's mode preserved via
  `Lstat`+`IsRegular` (load-bearing — without it a directory target fails at temp creation, not at
  `Rename`), temp removed on failure, **no fsync**. (03)
- **`status:` in step frontmatter is the sole authoritative doneness field** — never the
  progress-list checkbox (a projection `finish`/05 keeps in sync) or handoff content. (03)
- **Next step number = `max(existing numbers) + 1`**, scanned via `stepfile.Number`, never from
  the progress list (mutation-verified against count+1). Numbers are never reused — 05's progress
  marking and 21's `depends-on` depend on ids surviving a deleted step file. (03)
- **Handoff anchor = the bare `cfg.HandoffHeading` line, nothing under it.** 13's trichotomy:
  *absent* = no matching line; *present-and-empty* = line exists, no non-blank line before
  EOF/next-heading; *present-and-filled* = otherwise. `finish` (05) replaces the body wholesale.
- `internal/scaffold`: `NewServer(cfg, root) *Server`; `NewFeature`/`NewStep(ctx, feature)
  (string, error)` — no `Store` port, every write real-filesystem (mtime identity, temp-file
  absence, byte-identity-after-refusal are filesystem properties no in-memory adapter models).
  **`NewStep` validation order (R14a):** pattern compiles → feature dir opens
  (`topRoot.OpenRoot(feature)` is both traversal guard and existence check — `../escaped` →
  `ErrNoSuchFeature`) → spec reads → progress heading found (checked via a throwaway
  `insertProgressEntry(spec, heading, "")` before anything writes) → number computed → written.
  **Write order: step file before spec** — a spec-write failure after leaves a visible orphan step
  file, not a dangling progress entry. (02/03)
- **`scaffold.RefusalError{Path, Problem, Fix, Err}`**: `Error()` = `<path>: <problem>; <fix>`,
  `Unwrap()` → `Err`. `cli/refusal.go`'s `renderRefusal` adds an
  `errors.AsType[*scaffold.RefusalError]` branch (before the flatten fallback) rendering `brief
  <cmd>: <path>: <problem>; <fix> (no files changed)`. Sentinels: `ErrNoSuchFeature`,
  `ErrMalformedFeature`, `ErrNoProgressHeading`, `stepfile.ErrInvalidPattern`. (03)
- `internal/cli`: `brief new step <feature>` added beside `brief new feature <name>`; usage copy
  reads `expected one of: feature, step`; `brief new step --help` is a third help level. Go 1.27's
  `errors.AsType[T]` is preferred over `errors.As` + `var target T` (`modernize` lint). (03)

## Left unbuilt

- Reading a step file back — no frontmatter/checklist/handoff-block parser yet. `stepfile` only
  names files; SCENARIO-04/05 own opening and parsing one.
- `scaffold.ErrFeatureExists`/duplicate-feature refusal — SCENARIO-08. Feature-name validation
  (path separators, `..`) — SCENARIO-07; `os.Root` already refuses traversal for both commands,
  exit 1 either way, not yet exit 2.
- `RefusalError.Line` (R14a's `:<line>`) — SCENARIO-20. Known-feature enumeration in the
  unknown-feature refusal (R14) — SCENARIO-09.
- Heading/cap/`checklist-heading` **value** validation, global config, `--config` override,
  `feature-directory: ""` — unowned since SCENARIO-01.

## Traps

- **Avoid `t.Chdir`/`os.Getwd` in tests** (`EvalSymlinks` breaks macOS fixtures) — build paths via
  `filepath.Join`/`Rel` on strings the test already holds.
- **`yaml.Decoder.Decode` on a zero-byte file returns `io.EOF`**, not nil — `decodeConfig`'s guard
  must stay.
- **No rollback-on-failure cleanup** before SCENARIO-08's existence check lands — would delete a
  real pre-existing feature/step on a failed retry.
- **`writeExclusive`'s `O_EXCL` is unreachable by construction** in `NewFeature`/`NewStep` (their
  guarantees make the target name always free) — keep it, don't test it until 08 relaxes that.
- **`atomicfile.WriteFile` replaces a `0o400` target** (rename ignores target mode) — a sanity
  fact, not a contract; don't pin it as a test.
- A fixture config field equal to `config.Default()` makes its threading test vacuous.
  `scaffold_test.go`'s `fixtureConfig()` pins `StepFilePattern = "STEP-%02d.md"` — seed step
  filenames in terms of *that*, not the shipped `SCENARIO-*`, or numbering tests prove nothing.
- **No `.brief.yaml` in this repo**, so `root = wd`: both commands work from the repo root only.

## Open debts

- Heading/cap value validation — unowned until SCENARIO-19.
- Half-scaffolded feature dir or orphan step file on a mid-write I/O failure; nothing removes
  either. `check` (SCENARIO-22) is the natural detector for an orphan step. Unowned.
- Invalid-`step-file-pattern` refusal names the feature directory, not the `.brief.yaml` carrying
  the bad value — `scaffold` isn't given the config source path. Unowned.
- `insertProgressEntry` inserts an LF line into a CRLF file unmodified elsewhere. Unowned.
