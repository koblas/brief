# brief — current state

Scenarios complete: SCENARIO-01. Last updated by SCENARIO-01.

## Binding decisions

- Config lives at `internal/platform/config` — every feature package consumes it; a feature package may not import another feature package, and `platform` is the skill's escape hatch for exactly this. (SCENARIO-01)
- **Consumers take a `Config` as a parameter. Nothing outside `config.Resolve`/`config.decodeConfig` calls `config.Default()`.** SCENARIO-19 refuses on a missing state heading and must read that heading from the `Config` it was handed; a direct `config.Default()` call there kills the config-driven property while SCENARIO-01's own tests stay green — that is exactly what the Step 10 mutation proved falsifiable. (SCENARIO-01)
- The four state headings are `config.StateHeadings` — four named fields (`BindingDecisions`, `LeftUnbuilt`, `Traps`, `OpenDebts`), never a slice — plus `Ordered() []string` for iteration. R2: config retitles a section, never drops one. (SCENARIO-01)
- Config filename is exactly `.brief.yaml`, no `.yml` alias. Resolution: upward walk from a start directory to the filesystem root, **nearest wins, no merging**; missing config anywhere is the shipped profile (`config.Default()`), not an error. A found file is decoded onto `Default()` (overlay: omitted key keeps its shipped value) with `yaml.Decoder.KnownFields(true)` (unknown key refused). (SCENARIO-01)
- `Resolve(startDir string) (Config, string, error)` — second return is the source path, `""` meaning shipped defaults in effect; error is `errors.Is`-comparable to `config.ErrInvalidConfig` for a malformed/unknown-key file or a non-existent `startDir`. Both classification points wrap `%w` twice (`ErrInvalidConfig` then the cause) per `wrapcheck`. (SCENARIO-01)
- An **empty `.brief.yaml`** is treated as the shipped profile in effect (not refused) — the file is still reported as the source. Chosen over refusing, because `init` writing a comments-only file must not become a hard failure. (SCENARIO-01)
- `Resolve` stats `startDir` at entry and refuses a non-existent one as `ErrInvalidConfig`; `filepath.Abs` alone does not stat, so without this guard a mistyped path silently picks up an ancestor's config. (SCENARIO-01)
- Default output budget is **8192 bytes** (`Config.DefaultOutputBudgetBytes`) — the spec left this open; picked as a round, generous-but-bounded figure for a `start` payload. Any later scenario changing it is a deliberate revision, not a rediscovery. (SCENARIO-01)
- Step-file name pattern (`Config.StepFilePattern`, default `"SCENARIO-%02d.md"`) is stored as an **opaque string** — SCENARIO-01 does not expand or interpret it. SCENARIO-03 owns pattern interpretation. (SCENARIO-01)
- The whole schema (`Roles`, `OptionalConventions`) is modeled now though unused — a post-crossover schema change would be a migration of `brief`'s own tree. (SCENARIO-01)
- Tests are external `package config_test`; `testpackage` lint rule is on.

## Left unbuilt

- `cmd/brief/main.go`, `internal/cli` — do not exist yet. SCENARIO-02 owns both, including the R14 error→exit-code mapping. (SCENARIO-01)
- R14a refusal *copy* for `ErrInvalidConfig` — `Resolve` returns a plain wrapped error (`resolve config: <path>: invalid brief config: <cause>`), not the `brief <command>: <path>: <problem>; <next action>` template. SCENARIO-02 owns rendering it at the `cmd/brief` boundary, alongside the exit-code mapping — the two are separate obligations and only the mapping was previously named here. (SCENARIO-01)
- Global config (`brief init --global`) and the project/global conflict `--show` reports — not built. `Resolve`'s upward walk is the *project* path only and will happily treat a `$HOME/.brief.yaml` as the project's own. Whoever builds `--global` owns the distinction. (SCENARIO-01)
- `--config <path>` override — not built; `Resolve` takes only a start directory.
- Validation of heading/cap **values** — an empty heading string or a non-positive cap in a config file decodes without error today. SCENARIO-19 ("names the missing heading") must define what "missing" means for `""` — unowned until SCENARIO-19 closes it.
- The atomic write primitive (R12) — SCENARIO-05 owns it, in its own platform package. Nothing here writes anything.

## Traps

- **`filepath.EvalSymlinks` breaks every fixture on macOS** — `t.TempDir()` returns `/var/folders/...`, a symlink to `/private/var/...`. Use `filepath.Abs` only; never canonicalize a path a test constructed. Also avoid `t.Chdir` + `os.Getwd` for the same reason — compute a relative path via `filepath.Rel(realCwd, target)` without chdir if a relative-path test is needed. (SCENARIO-01)
- **`yaml.Decoder.Decode` on a zero-byte file returns `io.EOF`** where `yaml.Unmarshal` would return nil — `KnownFields(true)` forces the `Decoder` API. `config.decodeConfig` guards this explicitly; a future rewrite of that guard must preserve it or an `init`-written comments-only config becomes a hard failure again. (SCENARIO-01)
- **Overlay is leaf-scalar only** — a present *collection* key (a list, a nested struct decoded as a map) replaces the default wholesale rather than merging per-element. Harmless today (the four headings are scalars); will bite `OptionalConventions`/`Roles` once those get non-empty defaults.
- `go get`/`go mod tidy` need network; sandbox denies by default — rerun with the sandbox disabled. Also: `go mod tidy` removes an added dependency's `require` line again if no `.go` file imports it yet — add the import first, tidy after.

## Open debts

- Heading/cap value validation (empty heading, non-positive cap, **or two headings configured to the same text**) — unowned until SCENARIO-19; dies unless re-opened there.
