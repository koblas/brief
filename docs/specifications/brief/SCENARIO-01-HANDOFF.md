**Binding decisions** — a later scenario must not contradict these without saying so:

- Config lives at `internal/platform/config` — every feature package consumes it and a feature package may not import another feature package; `platform` is the skill's own escape hatch for exactly this.
- **Consumers take a `Config` as a parameter. Nothing outside the resolve path calls `Default()`.** SCENARIO-19 refuses on a missing state heading and must read that heading from the `Config` it was handed; a direct `config.Default()` call there kills the config-driven property while S01's tests stay green.
- The four state headings are four named struct fields with an ordered accessor, never a slice — R2: config retitles a section, it never drops one.
- Filename is `.brief.yaml`, exactly one name, no `.yml` alias — `init` (R19) writes it and discovery must not drift. Spec was silent; this was chosen.
- Resolution is upward, **nearest wins, no merging**, stop at filesystem root; a missing config is the shipped profile and is **not** an error.
- Overlay: a config file is decoded onto `Default()`, so an omitted key keeps its shipped value.
- `KnownFields(true)` — an unknown key is refused. Consequence: a config written for a later `brief` fails on an older binary. Reversing this is a behavior change, not a tweak.
- The whole schema (roles, optional conventions) is modeled now though unused — *Bootstrap hazards* makes a post-crossover schema change a migration of `brief`'s own tree.
- Tests are external `package config_test`; `testpackage` is on and its only exclusion names a nonexistent path.

**Left unbuilt** — named so nobody assumes it exists:

- `cmd/brief/main.go`, `internal/cli` — do not exist. SCENARIO-02 owns both, including the R14 error→exit-code mapping.
- Global config (`brief init --global`) and the project/global conflict `--show` reports — **not built**. The upward walk is the *project* path only and will happily pick up a `$HOME/.brief.yaml` as if it were the project's. Whoever builds `--global` owns that distinction.
- `--config <path>` override — not built; `Resolve` takes a start directory only.
- Step-file **pattern interpretation** (expanding the name pattern to a filename) — SCENARIO-03 owns it. S01 stores the string and nothing more.
- Validation of heading *values*: an empty heading string or a non-positive cap in a config file is accepted today. SCENARIO-19 "names the missing heading" is undefined if a heading is `""` — SCENARIO-19 owns closing this.
- The atomic write primitive (R12) — SCENARIO-05 owns it, in its own platform package.

**Traps** — things that look right and are not:

- **`filepath.EvalSymlinks` breaks every fixture on macOS.** `t.TempDir()` returns `/var/folders/...`, a symlink to `/private/var/...`. If the resolver canonicalizes, the reported source path will not equal the path the test constructed and the walk tests fail for a reason unrelated to the walk. Use `filepath.Abs` only. For the same reason, pass the start directory explicitly — do **not** use `t.Chdir` plus `os.Getwd`, which can hand back the resolved path.
- **Overlay is leaf-scalar only.** Unmarshalling onto `Default()` leaves unset scalars alone, but a *present collection key replaces the default collection wholesale*. Harmless for the four headings; it will bite on optional conventions and role bindings once those have non-empty defaults.
- **An empty `.brief.yaml` returns `io.EOF`.** `KnownFields(true)` forces the `yaml.Decoder` API, and `Decode` on a zero-byte file errors where `yaml.Unmarshal` would have returned nil and left the defaults intact. Unguarded, that makes an empty config a hard failure — and `init` writing a commented-only file is exactly the case that hits it.
- **`filepath.Abs` does not stat.** A resolver with no entry guard walks happily from a non-existent directory and finds a config in an existing parent, so a "bad start dir" test with no config above it passes vacuously.
- A no-config fixture is **not** the control arm for "headings come from config" — it differs from the override fixture in two variables (file presence and heading text). The control arm is a config file present with the headings key omitted.
- `go get` needs network; the sandbox denies it. Re-run with the sandbox disabled rather than concluding the module is broken.
