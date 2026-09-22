---
id: SCENARIO-01
status: done
---

# SCENARIO-01: Invalid config values are refused by every command

## Scenario

```gherkin
Scenario: SCENARIO-01 Invalid config values are refused by every command
  Given .brief.yaml sets handoff-cap-lines to 0 (likewise: empty heading, state-file equal to specification-file, a path separator in a file name, a step pattern without exactly one verb)
  When I run "brief status" (or start, check, finish, new)
  Then stderr is one refusal naming .brief.yaml, the key and its value, with a fix; exit 1; --json gives an error document of kind refusal
```

## Contract

Validation runs in `decodeConfig` after a successful decode, so only a found `.brief.yaml`
is checked (no config file = `Default()`, untouched). The first violation, in `Config`
field-declaration order, is returned as `*config.InvalidConfigError{Path, Err}` whose `Err`
is a new `*config.ValueError{Key, Value, Reason, Err}`. `ValueError.Error()` is
`<key> is <value>, <reason>` — strings `%q`-quoted, ints bare.

The existing renderer (`classifyRefusal`, refusal.go) needs no change; with this `Err`
the one stderr line is, e.g.:

`brief status: .brief.yaml: handoff-cap-lines is 0, must be at least 1; fix it or remove it to fall back to the shipped defaults (no files changed)`

Exit 1. `--json`: error document, `kind` `refusal`, `path` = the config path, `line` null,
`problem` = the `ValueError` text, `fix` as above. Nothing on stdout in text mode.

Rules (each its own helper, `Reason` copy in quotes):
- `handoff-cap-lines`, `state-cap-lines`, `default-output-budget-bytes` ≥ 1 — "must be at least 1".
- `progress-heading`, `checklist-heading`, `acceptance-heading`, the four `state-headings.*`
  (key rendered dotted, e.g. `state-headings.traps`): not blank after `TrimSpace` — "must not
  be empty"; all seven pairwise distinct (exact string) — "must differ from <earlier key>",
  naming the later key.
- `specification-file`, `state-file`: a plain file name — not empty, not `.`/`..`, no `/` or
  `\` — "must be a plain file name with no path separator".
- `state-file` vs `specification-file`: `strings.EqualFold` equal → refused on `state-file`,
  "must differ from specification-file" (case-insensitive for the same reason `CompileHandoff`
  is: macOS/Windows filesystems fold case).
- `step-file-pattern`: `stepfile.Compile` error → "must be a plain file name with exactly one
  %d or %0Nd verb and no other %", `Err` wraps the stepfile error.
- `handoff-file-suffix`: `stepfile.CompileHandoff(pattern, suffix, state, spec)` error →
  "must be a file-name suffix with no path separator, digit or %, naming a file distinct from
  the step, state and specification files", `Err` wraps the stepfile error.
- `feature-directory` is exempt from the separator rule: it is a path, and `Default()` ships
  `docs/specifications`.

`config` → `stepfile` import is legal: both `internal/platform`, and `stepfile` imports no
`config` (checked with `go list`), so no cycle.

## Existing seams checked

- `internal/scaffold/scaffold_test.go:255` (`sub/SPEC.md`) and `:277` (state = spec) build
  config with `fixtureConfig()` + `scaffold.NewServer(cfg, root)` and never call `Resolve`.
  Not affected. The same goes for `internal/platform/stepfile/handoff_test.go:69` and the
  `assemble` fixtures.
- `internal/cli/new_step_test.go:115` and `internal/cli/new_json_test.go:105` write a
  `.brief.yaml` that is now refused at load. Steps 8 and 9 rework them.
- Keep the existing `stepfile.Compile`/`CompileHandoff` calls in `assemble/{assemble,status,check}.go`
  and `scaffold/{scaffold,finish}.go`. They still guard callers that build config directly.

## Implementation Plan

- [x] Step 1: `internal/platform/config/resolve_test.go` `Test_Resolve_refuses_an_invalid_config_value` — table with one row per rule (each cap at 0 and at -1, each heading blank, one duplicate-heading pair, `sub/SPEC.md`, `.`, backslash, state = spec, case-only differing state/spec, `SCENARIO-%s.md`, two-verb pattern, handoff suffix with `/`). Each row asserts `ErrInvalidConfig`, the `*ValueError` Key/Value via `errors.As`, the exact `Error()` text, and `Path` (red)
- [x] Step 2: `internal/platform/config/resolve_test.go` `Test_Resolve_keeps_the_stepfile_sentinel_reachable_for_a_bad_pattern` — `errors.Is(err, stepfile.ErrInvalidPattern)` and `ErrInvalidHandoffSuffix` still hold through the wrap (red)
- [x] Step 3: `internal/platform/config/config_test.go` `Test_the_shipped_profile_passes_validation` plus a `Resolve` control arm: a config that sets every key to a valid non-default value is accepted. This proves the rules do not refuse valid input (red → green once Step 5 lands; if it is green on arrival, say so)
- [x] Step 4: `internal/platform/config/errors.go` `ValueError` + `Error`/`Unwrap` with doc comments (new)
- [x] Step 5: `internal/platform/config/validate.go` `validate(Config) error` dispatching to one unexported helper per rule (caps, headings non-empty, headings distinct, file names, state≠spec, step pattern, handoff suffix) (green)
- [x] Step 6: `internal/platform/config/resolve.go` `decodeConfig` — call `validate` after a successful decode and wrap the result in `*InvalidConfigError{Path}`. The empty-file `io.EOF` path returns `Default()` and needs no call (update)
- [x] Step 7: `internal/platform/config/doc.go`, and the `Resolve`, `ErrInvalidConfig` and `InvalidConfigError` doc comments — state that value rules are refused at load (update)
- [x] Step 8: `internal/cli/new_step_test.go` `Test_refuses_on_one_line_for_an_invalid_step_file_pattern_from_an_ancestor_config` — delete the `new feature` setup that now refuses. Run `new step payments` from `a/b` against the ancestor config and keep the existing assertions (`ErrorIs stepfile.ErrInvalidPattern`, line contains `step-file-pattern` and `SCENARIO-%s.md`, `(no files changed)` suffix) (update)
- [x] Step 9: `internal/cli/new_json_test.go` `Test_new_feature_json_files_changed_is_true_when_the_state_write_fails` — swap the `SAME.md` seam for a `state-file` that validation accepts but the OS refuses: a single name longer than 255 bytes, which gives ENAMETOOLONG on APFS, ext4 and tmpfs. Keep the spec read-back control arm. If the state write does not fail after the spec write lands, stop and report. Do not delete the test (update)
- [x] Step 10: `internal/cli/invalid_config_test.go` `Test_every_command_refuses_an_invalid_config_value` — table over `status`, `start payments`, `check`, `finish` (with whatever positional args its leaf requires to reach `resolveRoot`; check `finish.go`), `new feature payments`, `new step payments` against `handoff-cap-lines: 0`. Assert exit 1, empty stdout, exactly one stderr line equal to the full expected copy (with the relative `.brief.yaml`), and no feature directory created (red → green via Step 6; if it is green on arrival, say so)
- [x] Step 11: `internal/cli/json_refusal_test.go` — add a `new feature invalid config value` row (and one for `finish`) to the existing refusal table: `kind` `refusal`, `path` = config path, the shipped fix (update)
- [x] Step 12: mutation-verify each helper individually, stashing each one with a unique tag: make one helper return nil and confirm its own Step 1 rows go red and nothing else. Record every mutation and the test it reddens. A mutation that breaks compilation is not evidence
- [x] Step 13: `go build ./...`, `go test ./...` (unpiped, report count + delta), `go test -race ./internal/platform/config/... ./internal/cli/...`, `golangci-lint run ./...` → set `status: done` in this file's frontmatter and check SCENARIO-01 in specification.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Value validation lives in `config.decodeConfig` (called by `Resolve`), never in `doctor` or `internal/cli`. R1 requires every command to refuse, and SCENARIO-02's `doctor` reuses the same `Resolve` error instead of re-checking.
- A value error is `*config.InvalidConfigError{Path, Err: *config.ValueError{Key, Value, Reason, Err}}`. `doctor` (S02) and `init`'s "invalid existing config" refusal (S03, R3) recover key and value with `errors.As`. Changing the shape breaks both.
- Only the first violation is reported, in `Config` field order. The one-line refusal contract (R14a) allows one problem per line.
- `state-file` vs `specification-file` is compared case-insensitively, matching `CompileHandoff`'s collision rule.
- `feature-directory` may contain separators, because `Default()` ships `docs/specifications`.
- `Default()` must pass `validate`. `init` (S03) writes a commented config whose effective values are the defaults, and `Test_the_shipped_profile_passes_validation` pins this.
- No renderer change: the refusal copy is `ValueError.Error()` passed through the existing `classifyRefusal` InvalidConfigError branch with the shipped fix.

**Left unbuilt** — named so nobody assumes it exists:
- Line number for a value error (`InvalidConfigError` has no `Line`). Decoding goes straight into the struct, with no `yaml.Node`. Owner: whoever needs `doctor` to cite lines (S02 may add it).
- Validation of `roles.*` and `optional-conventions`. Owner: S08 (`--with-agents` binds roles).
- Heading-shape rules (for example, a heading must start with `#`). Not in R1.
- Any absolute-path or `..` check on `feature-directory`.

**Traps** — things that look right and are not:
- `CompileHandoff` does not compare state-file against specification-file. It only compares the handoff name against each of them, so the state≠spec rule needs its own helper.
- Any CLI test that writes a `.brief.yaml` to reach a scaffold or assemble seam is now refused at load. Build the config directly with `scaffold.NewServer(cfg, …)` / `fixtureConfig()` for such seams.
- Wrap stepfile errors with `%w` inside `ValueError.Err`, or `errors.Is(err, stepfile.ErrInvalidPattern)` in `new_step_test.go` stops holding.
- This step file's handoff for `brief finish` is `SCENARIO-01-HANDOFF.md` beside it (`HandoffFileSuffix` `-HANDOFF.md`), not only this `## Handoff` section.
