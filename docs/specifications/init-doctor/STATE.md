# init-doctor — current state

Scenarios complete: SCENARIO-01. Last updated by SCENARIO-01.

## Binding decisions
- Value validation lives in `config.decodeConfig` (called by `Resolve`), never in `doctor` or
  `internal/cli` — R1 requires every command to refuse; `doctor` (S02) must reuse the same
  `Resolve` error rather than re-checking (SCENARIO-01)
- A value error is `*config.InvalidConfigError{Path, Err: *config.ValueError{Key, Value,
  Reason, Err}}` — `doctor` (S02) and `init`'s "invalid existing config" refusal (S03, R3)
  recover key/value with `errors.As` against this exact shape (SCENARIO-01)
- Only the first violation is reported, in `Config` field-declaration order (one problem per
  line, R14a) (SCENARIO-01)
- `state-file` vs `specification-file` is compared case-insensitively (`strings.EqualFold`),
  matching `CompileHandoff`'s own collision rule (SCENARIO-01)
- `feature-directory` is exempt from the file-name/separator rule — it is a path, and
  `Default()` ships `docs/specifications` (SCENARIO-01)
- `Default()` passes `validate`; `init` (S03) writes a commented config whose effective
  values are the defaults, and this must keep holding (SCENARIO-01)
- No refusal-renderer change: the stderr/JSON copy is `ValueError.Error()` flowing through
  the existing `classifyRefusal` `InvalidConfigError` branch (SCENARIO-01)

## Left unbuilt
- Line number for a value error (`InvalidConfigError` has no `Line`; decoding goes straight
  into the struct, no `yaml.Node`) — owner: `doctor` (S02) if it needs to cite lines
  (SCENARIO-01)
- Validation of `roles.*` and `optional-conventions` — owner: S08 (`--with-agents` binds
  roles) (SCENARIO-01)
- Heading-shape rules (e.g. a heading must start with `#`) — not in R1, unowned
  (SCENARIO-01)
- Any absolute-path or `..` check on `feature-directory` — unowned (SCENARIO-01)

## Traps
- `stepfile.CompileHandoff` does not compare state-file against specification-file itself —
  the state≠spec rule needed its own helper in `validate.go` (SCENARIO-01)
- Any CLI test that writes a `.brief.yaml` to reach a scaffold/assemble seam is now refused
  at load if that config is invalid; build config directly via `scaffold.NewServer(cfg, …)`
  / `fixtureConfig()` for such seams instead of routing setup through `cli.Run` (SCENARIO-01)
- `ValueError.Err` must wrap stepfile errors with `%w`, or
  `errors.Is(err, stepfile.ErrInvalidPattern)` / `ErrInvalidHandoffSuffix` stops holding
  through `InvalidConfigError` → `ValueError` (SCENARIO-01)
- `%q`-formatting a string `Value` containing a literal backslash doubles it in the rendered
  `Error()` text — a golden string with a raw backslash must account for that (SCENARIO-01)

## Open debts
- `doctor` (S02) must reuse `config.Resolve`'s own error/shape rather than re-validating —
  S02 must close it
- `init`'s "invalid existing config" refusal (S03, R3) must recover key/value via `errors.As`
  on `*config.ValueError` — S03 must close it
- `roles.*` / `optional-conventions` validation — S08 must close it, or the spec must say
  explicitly these stay unvalidated
