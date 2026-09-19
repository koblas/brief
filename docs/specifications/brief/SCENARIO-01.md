# SCENARIO-01: Configuration resolves from the working directory upward

## Scenario

```gherkin
Scenario: SCENARIO-01 Configuration resolves from the working directory upward  [orig: R2, OQ5]
  Given a repository with a brief config file at its root
  And a working directory several levels below that root
  When configuration is resolved
  Then the root config is found and its values are in effect
  Given a repository with no config file at all
  When configuration is resolved
  Then the shipped default profile is in effect
  And the four required state headings come from the resolved configuration, not from constants
```

## Placement and shape — the decisions this scenario sets

**Package: `internal/platform/config`.** Not `internal/config`. Every feature package
(`new`, `start`, `finish`, `status`, `check`) consumes config, and `clean-architecture`
forbids a feature package importing another feature package; its own stated escape is
"move the shared thing down to `internal/platform/*`". The "no feature knowledge"
adjective on `platform` yields to the hard dependency rule — the shipped profile is the
one piece of policy that must be visible to all of them, and duplicating the type per
feature is the only alternative. Do **not** pre-split an `fsfind`/`configfile` pair; the
upward walk is a dozen lines and has one consumer.

**Plain functions, no `Server`, no functional options.** `clean-architecture:51-72`
mandates options for a constructor that holds *injected dependencies*. This package holds
no struct and has no injectable dependency: the filesystem is the thing under test, and
faking it to test an upward filesystem walk tests the fake. `t.TempDir()` is the adapter.
A `Server` with no fields plus `WithX` helpers nobody calls is ceremony, and over-applying
the pattern is as much a finding as skipping it.

**No `Store` port in this scenario.** There is nothing to persist. Config is read-only
input; the write path (R12 atomic write) arrives with SCENARIO-05 and gets its own
platform package.

**No `cmd/brief/main.go`.** Resolution is fully exercised from the package's own tests
against `t.TempDir()`; nothing here needs a command line. SCENARIO-02 is the first
scenario that forces `cmd/brief` and `internal/cli` to exist.

**Resolution semantics** (all settled here, all inherited):

- One filename, `.brief.yaml`. No `.yml` alias — `init` writes exactly one name and
  discovery must not drift (R19). *Spec is silent on the filename; this is a choice.*
- Walk from the start directory upward to the filesystem root. **Nearest wins, no
  merging** across levels: merging makes "which value is in effect" unanswerable from one
  file, which is the R1 property.
- **No config anywhere is not an error** — the shipped profile is the answer, and
  resolution reports that it had no source.
- Resolution reports **where the config came from**; an absent source means shipped
  defaults. (`init --show` and R14a refusal copy both need this. The exact return shape is
  the developer's.)
- **Overlay**: unmarshal onto `Default()`, so a config that sets one key keeps the
  shipped value for every other. The spec's "Everything has a default" is this.
- **`KnownFields(true)`**: an unknown key is refused, not ignored. Consequence, recorded
  deliberately: a config written for a later `brief` fails on an older binary.
- `filepath.Abs` on the start directory, **never `filepath.EvalSymlinks`** — see *Traps*.
- Errors: one exported sentinel for an unusable config (`ErrInvalidConfig`), reached via
  `errors.Is`. Exit-code mapping (R14: 1 = validation failure) is `cmd/brief`'s job and
  does not exist yet.

**Shipped profile values** — from `## Default profile` and `## Configuration`; the
developer transcribes these, they are not re-derived:

| Key | Default |
| --- | --- |
| feature-directory root | `docs/specifications` |
| step-file name pattern | `SCENARIO-%02d.md` — *pattern interpretation is SCENARIO-03's; S01 only stores the string* |
| progress heading | `## BDD Acceptance Progress` |
| handoff heading | `## Handoff` |
| state headings (4, ordered) | `## Binding decisions`, `## Left unbuilt`, `## Traps`, `## Open debts` |
| handoff cap | 60 lines |
| state cap | 80 lines |
| default output budget | *spec is silent — pick a byte figure and record it in the handoff* |
| optional conventions in use | empty |
| role bindings (planner, implementer) | empty |

The four state headings are **four named struct fields, not a slice** — R2 says names are
configuration and structure is not, so config may retitle a section but never drop one.
Give them an ordered accessor so SCENARIO-19 can iterate without re-hardcoding the order.
Model the whole schema now (roles and optional conventions included, unused): *Bootstrap
hazards* makes a post-crossover schema change a migration of `brief`'s own tree.

**Tests are external (`package config_test`).** `testpackage` is on in `.golangci.yaml`
and its only exclusion names a path that does not exist here. The entire surface of this
scenario is exported, so black-box costs nothing.

Verification commands per `.claude/rules/agent-briefs.md`. `go get` needs network — expect
the sandbox to deny it and re-run with the sandbox disabled.

## Implementation Plan

Red for any single step is `go test ./internal/platform/config/ -run '^<TestName>$' -v`;
green is `go test ./...` unpiped. **Several steps below are green on arrival by
construction** — once Step 6 decodes YAML generically onto `Default()`, a config that sets
a key overrides it with no further code, so there is nothing to add and no red to observe.
Say so per the standing brief; do not manufacture a red. The falsifiability of Decision 4
rests entirely on the Step 10 mutation, which is therefore not optional polish.

- [x] Step 1: `go.mod` — `go get gopkg.in/yaml.v3` and `go get github.com/stretchr/testify`, then `go mod tidy`; prove with `go build ./...` (new)
- [x] Step 2: `internal/platform/config/config_test.go` `Test_Default_carries_the_four_shipped_state_headings` — asserts `Default()` returns the four heading texts from the table above, in order (red: package does not exist). `go test ./internal/platform/config/` (new)
- [x] Step 3: `internal/platform/config/doc.go` — package doc: what config owns, that the shipped profile is the default, that resolution is upward and nearest-wins (new)
- [x] Step 4: `internal/platform/config/config.go` — `Config` and its nested types, the four-field state-headings struct plus its ordered accessor, `Default()`, and the exported `ErrInvalidConfig` sentinel (declared here so Step 13's red is an assertion failure, not a compile error) (green)
- [x] Step 5: `resolve_test.go` `Test_Resolve_finds_the_config_at_the_repository_root_from_three_levels_below` — fixture writes `.brief.yaml` at a `t.TempDir()` root and resolves from **three** levels below; asserts a value set by that file is in effect and that the reported source is the root file (red). *Three levels, not one: a one-level fixture passes against a walk that only checks the parent* (new)
- [x] Step 6: `internal/platform/config/resolve.go` `Resolve` — upward walk, `filepath.Abs` on the start dir, first hit wins, stop at filesystem root, YAML decoded onto `Default()` with `KnownFields(true)` (green)
- [x] Step 7: `resolve_test.go` `Test_Resolve_falls_back_to_the_shipped_profile_when_no_config_file_exists` — resolves from a directory tree containing no `.brief.yaml`; asserts no error, the shipped values are in effect, and the reported source is absent (red/green — if green on arrival, say so and say why)
- [x] Step 8: `resolve_test.go` `Test_Resolve_reads_the_four_state_headings_from_the_config_file` — **the falsifiable arm**: config file present, all four state headings set to text that differs from the shipped defaults; asserts the resolved headings are the fixture's (green on arrival — the generic decode already does this; falsifiability comes from Step 10)
- [x] Step 9: `resolve_test.go` `Test_Resolve_keeps_the_shipped_state_headings_when_the_config_omits_them` — **the control arm, differing from Step 8 in exactly one variable**: same config file present, headings key omitted; asserts the shipped defaults are in effect. Together with Step 8 this is what makes "not from constants" falsifiable; the Step 7 no-config arm differs in two variables and is not the discriminator (green on arrival)
- [x] Step 10: **mutation check — this step carries the whole Decision-4 burden, do not skip it.** The generic decode has no per-field heading read to break, so the mutation is **additive and must still compile**: stash `resolve.go` per the standing brief, add a line after the decode re-assigning the four state headings from `Default()`, run Step 8's test alone and observe RED (Step 9 must stay green — that is what proves the two arms differ in one variable), pop, `diff` byte-identical. Report the mutation and which test reddened (verify)
- [x] Step 11: `resolve_test.go` `Test_Resolve_prefers_the_nearest_config_when_two_exist` — configs at the root and at an intermediate level setting the same key differently; asserts the nearer value wins and that the farther file's other keys are **not** merged in (green on arrival)
- [x] Step 12: `resolve_test.go` `Test_Resolve_keeps_shipped_defaults_for_keys_the_config_omits` — a config setting only one cap; asserts every other key still holds its shipped value (overlay semantics) (green on arrival)
- [x] Step 13: `resolve_test.go` `Test_Resolve_refuses_a_config_with_malformed_yaml` — asserts `errors.Is(err, ErrInvalidConfig)` and that the message names the config path (red)
- [x] Step 14: `resolve.go` — classify the decode failure as `ErrInvalidConfig` and wrap once with `%w` and the path, per `wrapcheck` (green)
- [x] Step 15: `resolve_test.go` `Test_Resolve_refuses_a_config_carrying_an_unknown_key` — asserts `ErrInvalidConfig` and that the message names the offending key (`KnownFields`) (green on arrival once Step 14 lands)
- [x] Step 16: `resolve_test.go` `Test_Resolve_treats_an_empty_config_file_as_the_shipped_profile` — a zero-byte `.brief.yaml`. **`KnownFields(true)` forces the `yaml.Decoder` API, and `Decode` on an empty file returns `io.EOF` where `yaml.Unmarshal` would have returned nil** — so without a guard an empty config is a hard failure, contradicting "Everything has a default". Assert the shipped profile is in effect with the file reported as the source. *If the developer would rather refuse an empty config explicitly, that is defensible — but it must be a named decision in the handoff, not an `io.EOF` leaking out* (red)
- [x] Step 17: `resolve_test.go` `Test_Resolve_refuses_a_start_directory_that_does_not_exist` — the guard is an **explicit stat at entry**; `filepath.Abs` does not stat, so a walk from a non-existent directory otherwise finds a config in an existing parent. Fixture must place a valid `.brief.yaml` in a parent of the non-existent directory, so the test discriminates "stat guard present" from "walk found nothing". *Spec is silent; chosen because a mistyped path silently yielding the shipped profile is the silently-wrong class this design exists to avoid* (red)
- [x] Step 18: `resolve_test.go` `Test_Resolve_accepts_a_relative_start_directory` — asserts the same resolution as its absolute form (green on arrival)
- [x] Step 19: `go doc ./internal/platform/config` — read the exported surface as a caller will; every exported symbol states its contract and names `ErrInvalidConfig` (update)
- [x] Step 20: `go build ./...`, `go test ./...` unpiped, `go test -race ./internal/platform/config/...`, `golangci-lint run ./...` all clean; report the exact test count → mark SCENARIO-01 done in `specification.md`
- [x] Step 21: `docs/specifications/brief/STATE.md` — create it from this scenario's Handoff (it does not exist yet; this is the first scenario)
