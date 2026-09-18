# SCENARIO-02: New feature scaffolds a conforming feature

## Scenario

```gherkin
Scenario: SCENARIO-02 New feature scaffolds a conforming feature  [orig: 03a]
  Given a project using the shipped default profile
  When I create a new feature
  Then a specification skeleton, an empty progress list and a state file exist
  And no prose has been written into any of them
```

## Decisions this plan takes (the spec is silent on each)

1. **Feature package is `internal/scaffold`, and it owns `new step` too (SCENARIO-03).**
   Both commands compute the same feature-directory layout and both touch the progress
   list; a feature package may not import another feature package, so splitting them
   would force a shared platform types package for nothing.

2. **No `Store` port.** The scaffold writes through the real filesystem. R1 makes files
   the source of truth — there is no second backend, now or later ("no database, no
   cache, no sidecar index, no daemon"). More decisively, the contracts this feature
   ships are byte-level filesystem properties: mtime identity on re-finish (S06), no temp
   file left behind (S05), every file byte-identical after a refusal (S08). An in-memory
   adapter models none of them, so every test that matters runs against the real FS
   anyway and the second adapter is ceremony no assertion trusts. The seam kept instead
   is a **pure renderer** (`specificationSkeleton`/`stateSkeleton`, unexported, exercised
   through `NewFeature`); the port that *is* warranted later is the atomic-write
   primitive, and that is a platform package owned by SCENARIO-05, not a `Store`.

3. **Hand-rolled dispatcher over `flag.FlagSet`, not cobra.** Cost, stated: we write the
   usage text and the `flag.ErrHelp` branch ourselves. Bought: no new `go.mod` line and
   no third-party control of exit codes, usage output or error copy — R14 (one line,
   codes 0/1/2) and R14a (one refusal template) are the whole point of this scenario and
   cobra fights both.

4. **Three artifacts, two files.** Per *Default profile*, the progress list is "a
   checklist in the specification under one stable heading" — it is `cfg.ProgressHeading`
   inside `specification.md`, not a third file. `specification.md` = `# <name>` + the
   progress heading, nothing under it. The state file = the four `cfg.StateHeadings`
   separated by blank lines, **no title line**, because S05 replaces the whole state body
   and a title would be silently dropped.

5. **"No prose" is asserted positively.** Both files are asserted by **exact equality**
   against a skeleton built from a **non-default** `Config` in the test's Given, plus an
   `ElementsMatch` over the directory entries proving only those two files exist. An
   empty scaffold fails the equality assertions; any added prose fails them too.

6. **Plain writes, no atomicity.** Both files are new files in a newly created directory:
   there is no prior content to corrupt, so R12's temp-file-plus-rename is not needed
   here. Recorded as a debt below — a mid-write I/O failure leaves a half-scaffolded
   directory, and nothing cleans it up.

7. **`init` does not exist**, so `new feature` creates the feature root itself
   (`os.MkdirAll` on `<root>/<cfg.FeatureDirectory>`) before creating the leaf.

## User-visible contract

| Invocation | stdout | stderr | exit |
| --- | --- | --- | --- |
| `brief new feature payments` | `docs/specifications/payments` (path relative to the working directory), one line | empty | 0 |
| `brief --help` / `brief -h` / `brief help` | usage | empty | 0 |
| `brief new feature --help` | usage | empty | 0 |
| `brief` (no args) | empty | `brief: no command given; expected one of: new` | 2 |
| `brief bogus` | empty | `brief: unknown command "bogus"; expected one of: new` | 2 |
| `brief new` (no type) | empty | `brief new: no type given; expected one of: feature` | 2 |
| `brief new step x` | empty | `brief new: unknown type "step"; expected one of: feature` | 2 |
| `brief new feature` (no name) | empty | `brief new feature: no name given; run 'brief new feature <name>'` | 2 |
| `brief new feature a b` | empty | `brief new feature: too many arguments; run 'brief new feature <name>'` | 2 |
| `brief new feature -x p` | empty | `brief new feature: flag provided but not defined: -x; run 'brief new feature <name>'` | 2 |
| malformed `.brief.yaml` | empty | `brief new feature: <cfg path>: <cause, flattened to one line>; fix it or remove it to fall back to the shipped defaults (no files changed)` | 1 |
| feature directory already exists | empty | `brief new feature: <cause, one line>` | 1 |

`cli.Run` **renders** all user-facing copy to the injected stderr and **returns** the
error only so `cli.ExitCode` can classify it. `main` prints nothing.

## Implementation Plan

- [x] Step 1: `internal/platform/config/config_test.go` `Test_the_default_profile_names_the_specification_file` / `Test_the_default_profile_names_the_state_file` — assert `Default().SpecificationFile == "specification.md"` and `Default().StateFile == "STATE.md"`; red is a compile failure until Step 2 (red)
- [x] Step 2: `config.go` `Config` — add `SpecificationFile` (`yaml:"specification-file"`) and `StateFile` (`yaml:"state-file"`) with those shipped defaults; extend the `Config` doc comment (green). R2 makes filenames configuration, and *Bootstrap hazards* makes a post-crossover schema change expensive — this is the cheap window
- [x] Step 3: `resolve_test.go` `Test_a_config_file_overrides_the_state_file_name` — a `.brief.yaml` carrying `state-file: NOTES.md` resolves to `NOTES.md` while `SpecificationFile` keeps its shipped value. Expect **green on arrival** (the overlay decoder is generic); say so rather than manufacturing a red (new)
- [x] Step 4: `resolve_test.go` `Test_names_the_offending_file_when_the_config_is_invalid` — `errors.As(err, &target)` yields a `*config.InvalidConfigError` whose `Path` is the `.brief.yaml` that failed, **and** `errors.Is(err, config.ErrInvalidConfig)` still holds (red)
- [x] Step 5: `internal/platform/config/errors.go` `InvalidConfigError` — `Path`, `Err`, `Error()` and `Unwrap() []error{ErrInvalidConfig, Err}`, with `Error()` as `fmt.Sprintf("%s: %v: %v", e.Path, ErrInvalidConfig, e.Err)`; `resolve.go` returns `fmt.Errorf("resolve config: %w", &InvalidConfigError{…})` at both classification sites (the `startDir` stat and the decode failure), which reproduces today's `resolve config: <path>: invalid brief config: <cause>` exactly (green). Grep `resolve_test.go` for error-*text* assertions before starting: this plan asserts the text is unchanged but did not read that file, so verify rather than assume. This is why a typed error and not a third return value: `fmt.Errorf("%w: %w")` yields `Unwrap() []error`, so `errors.Unwrap` returns nil and `cli` could not reach the cause it must render into R14a
- [x] Step 6: `internal/scaffold/scaffold_test.go` (`package scaffold_test`) — `Test_creates_the_feature_directory_under_the_configured_feature_directory`, `Test_returns_the_path_of_the_created_feature_directory`, `Test_writes_the_specification_skeleton_with_the_configured_progress_heading_and_nothing_under_it` (exact equality on the file bytes), `Test_writes_the_state_file_with_the_four_configured_headings_and_nothing_under_them` (exact equality), `Test_creates_only_the_specification_and_the_state_file` (`assert.ElementsMatch` over `os.ReadDir` names). Every fixture value differs from `Default()` — `FeatureDirectory: "specs"`, `ProgressHeading: "## Progress"`, `Traps: "## Gotchas"`, `SpecificationFile: "SPEC.md"`, `StateFile: "NOTES.md"` — so a hardcoded `config.Default()` cannot pass (red)
- [x] Step 7: `internal/scaffold/doc.go`, `scaffold.go` `Server` / `NewServer(cfg config.Config, root string) *Server` / `(*Server).NewFeature(ctx, name) (string, error)`, `render.go` `specificationSkeleton` + `stateSkeleton` — minimal green: create the feature root, the leaf directory and the two files (green). No `Option` type yet: there is no optional dependency, and an exported `Option` with no `WithX` is dead code
- [x] Step 8: `scaffold_test.go` `Test_does_not_overwrite_an_existing_specification_when_the_feature_directory_exists` — seed the directory with a specification whose content is a **sentinel** (`# handwritten by a person`), never what the scaffold would write, then assert `NewFeature` errors and the file is byte-identical; plus `Test_returns_an_error_when_the_feature_name_escapes_the_feature_root` (name `../escaped`, asserting no directory appears beside the feature root — Step 6 is its control arm, proving the same probe sees directories that *are* created) (red — the overwrite case was incidentally green already via `os.Mkdir` refusing an existing leaf directory; the escape case reddened as the hazard predicted)
- [x] Step 9: `scaffold.go` — `os.MkdirAll` the feature root, then `os.OpenRoot` it and do every write through the `*os.Root`: `Root.Mkdir(name, 0o755)` for the leaf, `Root.OpenFile(…, O_WRONLY|O_CREATE|O_EXCL, 0o600)` for each file (green). Perm `0o600` rather than `0o644` because gosec G302/G306 fires on the looser mode and git stores only the exec bit, so nothing downstream cares — do not reach for a `nolint`. `os.Root` refuses any name escaping the root, and `Mkdir` + `O_EXCL` make overwriting an existing feature impossible. **This guard is not optional**: `brief new feature brief` in this very repo would otherwise truncate the 46 KB APPROVED `specification.md`
- [x] Step 10: `internal/cli/run_test.go` (`package cli_test`) — `Test_creates_the_feature_and_prints_its_path`, `Test_returns_a_usage_error_when_no_name_is_given`, `Test_returns_a_usage_error_when_no_command_is_given`, `Test_returns_a_usage_error_when_the_command_is_unknown`, `Test_returns_a_usage_error_when_no_type_is_given`, `Test_returns_a_usage_error_when_the_type_is_unknown`, `Test_returns_a_usage_error_when_a_flag_is_not_defined`, `Test_returns_a_usage_error_when_there_are_too_many_arguments`, `Test_prints_usage_to_stdout_when_help_is_requested_for_the_binary` (`brief --help`, which never reaches a flag set), `Test_prints_usage_to_stdout_when_help_is_requested_for_the_subcommand` (`brief new feature --help`, which is `flag.ErrHelp`). Each calls `cli.Run(t.Context(), wd, args, &stdout, &stderr)`; each failure test asserts `require.ErrorIs(…, cli.ErrUsage)`, empty stdout, and that stderr is **exactly one non-empty line** — that last assertion is the only thing that catches a path returning an error without rendering it (red)
- [x] Step 11: `internal/cli/doc.go`, `cli.go` `Run(ctx context.Context, wd string, args []string, stdout, stderr io.Writer) error` + `ErrUsage` + the usage text, `new.go` `runNewFeature` — dispatch on `args[0]`/`args[1]`, handling `-h`/`--help`/`help` at the top level (usage to stdout, nil error) before the unknown-command branch, distinguishing *missing* from *unknown* at both levels, `flag.NewFlagSet(…, flag.ContinueOnError)` with `SetOutput(io.Discard)`, resolve config from `wd`, construct the `scaffold.Server` with the resolved `Config` and the project root, print the created path relative to `wd` (green)
- [x] Step 12: `run_test.go` `Test_creates_the_feature_where_an_ancestor_config_directs` — `.brief.yaml` with `feature-directory: specs` and `state-file: NOTES.md` at a temp root, `wd` two levels below it, asserting on the **filesystem paths** `<root>/specs/<name>/NOTES.md` (not on the stdout string, whose `..` segments make the expectation brittle). This is the config-threading guard: it reddens if anyone reaches for `config.Default()` (green on arrival — the implementation already threads `cfg`/`root` from `config.Resolve`; mutation-verified at Step 18)
- [x] Step 13: `run_test.go` `Test_refuses_on_one_line_when_the_config_file_is_invalid` — the fixture is an **unknown-key** `.brief.yaml` (yaml.v3 emits a genuinely multiline `yaml: unmarshal errors:\n  line N: …`; a syntax error yields a single-line cause and would pass with no flattening code at all). Asserts `errors.Is(err, config.ErrInvalidConfig)`, empty stdout, `strings.Count(stderr, "\n") == 1`, the line carrying the config path and ending `(no files changed)`, and that no feature directory was created (red — confirmed: 2 newlines, missing the `(no files changed)` suffix)
- [x] Step 14: `internal/cli/refusal.go` — the R14a renderer (`brief <command>: <path>: <problem>; <next action> (no files changed)`) and the newline-flattening helper it uses; `new.go` branches on `errors.As` for `*config.InvalidConfigError` and renders every other error as one flattened line (green)
- [x] Step 15: `internal/cli/exit_test.go` `Test_reports_success_when_there_is_no_error`, `Test_reports_a_usage_failure_for_a_usage_error`, `Test_reports_a_validation_failure_for_any_other_error` — R14's 0/1/2 (red), then `cli.go` `ExitCode(err error) int` (green)
- [x] Step 16: `cmd/brief/main.go` — `os.Getwd()`, `cli.Run`, `os.Exit(cli.ExitCode(err))`, printing nothing else (new). No test file under `cmd/`: there is no logic there, which is also what keeps every test black-box. Proved with `go build ./...` plus a smoke run of the built binary in a scratch directory (`go run` needs a module-rooted cwd, so built `-o "$TMPDIR/smoke/brief" ./cmd/brief` and ran it from there): `brief new feature demo` printed `docs/specifications/demo`, exit 0, both files scaffolded with empty prose; `--help` printed usage exit 0; no-args and `bogus` printed the usage refusals exit 2; a second `new feature demo` refused one line exit 1
- [x] Step 17: `go doc ./internal/scaffold` and `go doc ./internal/cli` — read what a caller gets; fix any doc comment that does not state the contract (refactor). Reviewed both packages plus `go doc ./internal/scaffold Server.NewFeature`/`./internal/cli.Run`/`./internal/cli.ExitCode`; every doc comment already states its contract — no changes needed
- [x] Step 18: mutation verification, one at a time, each backed up to `$TMPDIR` and restored byte-identical (diff-confirmed) after each check: (a) `cli/new.go`, replaced the threaded `cfg`/`root` with `config.Default()`/`wd` → reddened `Test_creates_the_feature_where_an_ancestor_config_directs` (Step 12) exactly as predicted; (b) `render.go`, injected a prose line into `stateSkeleton` → reddened `Test_writes_the_state_file_with_the_four_configured_headings_and_nothing_under_them` (Step 6) exactly as predicted; (d) `refusal.go`, made `flattenOneLine` a no-op → reddened `Test_refuses_on_one_line_when_the_config_file_is_invalid` (Step 13) exactly as predicted.
  **(c) did not match the plan's prediction and is reported rather than silently marked "verified":** dropping `O_EXCL` alone from `writeExclusive`'s `OpenFile` call left `Test_does_not_overwrite_an_existing_specification_when_the_feature_directory_exists` green, because `root.Mkdir(name, …)` — called before either file is opened — already refuses with `EEXIST` on the pre-existing leaf directory, so the file-open step this test targets is never reached in that scenario. Followed the standing brief's "verify guards individually" instruction: with **both** `Mkdir`'s exclusivity *and* `O_EXCL` disabled, the test reddened (truncation observed); with `Mkdir` alone tolerant of `EEXIST` and `O_EXCL` restored, the test passed — proving `O_EXCL` is not dead code, just unreachable from any input `NewFeature` can currently construct, since `Mkdir` guarantees the leaf directory is brand-new by the time either file is opened. Restored the original code (both guards intact); no production change made. This is a genuine gap between the plan's per-guard framing and this call graph's actual coverage — flagging it for the reviewer rather than resolving it unilaterally, since removing `O_EXCL` would be a deliberate policy call (defense-in-depth against a future refactor that relaxes `Mkdir`) and adding a test that reaches it needs the same relaxation, which is out of this scenario's scope
- [x] Step 19: `go build ./...`, `go test ./...` (unpiped, from the repo root), `go test -race ./internal/scaffold/... ./internal/cli/... ./internal/platform/config/...`, `golangci-lint run ./...`. All green, 0 issues. 38 PASS / 0 FAIL / 0 SKIP, delta **+26** from the baseline of 12: config gains 4 (Steps 1, 4 — Step 3 reused an existing assertion shape and added one test), scaffold gains 7 (new package), cli gains 15 (new package). `golangci-lint` caught real issues after Step 18 (err113 dynamic errors in a test, a gci import-order nit, an unused `newUsage` constant, and 5 `wrapcheck` unwrapped-external-error sites) — all fixed; see Handoff for the `scaffold: %w` wrap convention adopted to fix wrapcheck without reintroducing the `new feature: new feature:` double-prefix. Also fixed via `errors.AsType` (a `modernize` finding, Go's newer generic replacement for `errors.As`+`var target` in `refusal.go`). Re-ran the full smoke sequence after these fixes: create (exit 0, stdout path), re-create existing feature (`brief new feature: scaffold: mkdirat demo2: file exists`, exit 1, no double prefix), invalid config (one line ending `(no files changed)`, exit 1) — all match the contract table. Post-advisor cleanup: dropped the redundant outer `scaffold:` wrap from `NewFeature`'s two `writeExclusive` call sites (the inner `write %s: %w` wrap already satisfies `wrapcheck` there — reverified lint stayed at 0 issues after the change, not assumed)
- [ ] Step 20: mark SCENARIO-02 done in `specification.md`'s `## BDD Acceptance Progress`, and rewrite `docs/specifications/brief/STATE.md` folding in the Handoff below

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `internal/scaffold` owns both `new feature` and `new step` — S03's `NewStep` goes in this package; both need the same feature-directory layout and the progress list, and a feature package may not import another feature package.
- **No `Store` port anywhere in `scaffold`** — an in-memory adapter cannot model mtime identity (S06), temp-file absence (S05) or byte-identity after refusal (S08), so every test that matters runs against the real filesystem regardless. Tests use `t.TempDir()`. The port that *is* warranted is S05's atomic-write platform primitive, not a `Store`.
- `NewServer(cfg config.Config, root string) *Server` — both values required positionally, so there is no defaulting code where `config.Default()` could creep back in. S05 adds `opts ...Option` when it has a real optional dependency.
- `cli.Run(ctx, wd string, args []string, stdout, stderr io.Writer) error` — the working directory is a **parameter**; config resolves from `wd`, and the project root is the directory holding the resolved `.brief.yaml`, falling back to `wd` when the shipped profile is in effect.
- `cli.Run` renders all user-facing copy to its stderr and returns the error only for classification; `cli.ExitCode(err) int` maps nil→0, `cli.ErrUsage`→2, everything else→1. `main` prints nothing and holds the only `os.Exit`.
- Hand-rolled two-level dispatch over `flag.FlagSet`; `go.mod` gains no dependency. Help is stdout + exit 0 at both levels (`brief --help` is handled before the unknown-command branch; `brief new feature --help` is `flag.ErrHelp`), and *missing* argument copy (`no command given` / `no type given`) is distinct from *unknown* argument copy. S03 adds `step` to the `new` type switch and to both usage strings.
- `config.Config` gained `SpecificationFile` (`specification.md`) and `StateFile` (`STATE.md`); `config.InvalidConfigError{Path, Err}` carries the offending path and its `Unwrap() []error` keeps `errors.Is(err, ErrInvalidConfig)` true. S04/S05/S13/S22 locate files through those two keys, never through a literal.
- Scaffold shape: `specification.md` = `# <name>` + `cfg.ProgressHeading`; state file = the four `cfg.StateHeadings.Ordered()` headings, blank-line separated, **no title** — S05 replaces the whole state body and a title would be dropped. No `## Intent` / `## Rules` headings are scaffolded: their text is not in config, and hardcoding heading text violates R2.
- Success prints the created directory path relative to `wd` on **stdout**, one line, stderr empty, exit 0. S03 matches this shape for the created step file.

**Left unbuilt** — named so nobody assumes it exists:
- `brief new step` — S03. Today it is `brief new: unknown type "step"; expected one of: feature`, exit 2.
- `scaffold.ErrFeatureExists` and the R14a refusal copy for an existing feature — S08. Today `os.Root.Mkdir` returns EEXIST, rendered as one plain line, exit 1; S08 adds the template and the every-file-byte-identical sweep.
- Feature-name validation — S07. It must cover **path separators and `..`, not only whitespace**: `os.Root` turns `brief new feature ../x` into an error rather than an escape, but the message is not a usage refusal and the exit code is 1, not 2.
- Atomic writes — S05's platform primitive. Nothing here writes through a temp file.
- Progress-list *entry* writing, and markdown or frontmatter *parsing* of any kind — S03 onward.

**Traps** — things that look right and are not:
- **Adding `fmt.Fprintln(os.Stderr, err)` to `main` double-prints every refusal** — `cli.Run` already rendered it. `main` prints nothing but an `os.Getwd()` failure.
- `flag.ExitOnError` calls `os.Exit` below `main`, and `flag.ContinueOnError` still writes multi-line usage unless the flag set's output is `io.Discard` — either breaks R14's one-line rule.
- **Do not add rollback-on-failure cleanup (`os.RemoveAll` of the feature directory) before S08's existence check lands** — a failed run against a pre-existing feature would delete a real one.
- Do not call `os.Getwd()` inside `internal/cli`, and do not `t.Chdir` in a test — the working directory is a parameter precisely so neither is needed (and per S01, macOS `t.TempDir()` symlinks make `os.Getwd` comparisons lie).
- A config fixture value equal to a shipped default makes a threading test vacuous — every fixture value in Steps 6 and 12 must differ from `Default()`.
- A *syntax-error* `.brief.yaml` yields a single-line cause and cannot prove the newline flattening; that fixture must be an unknown key.

**Open debts:**
- A mid-write I/O failure leaves a half-scaffolded directory and nothing removes it; once S08 lands the retry is refused and the user must delete it by hand. Unowned — dies unless re-opened.
- `feature-directory: ""` in a config file puts feature directories at the project root. Folds into STATE's existing unowned heading/cap-value validation debt; not built.
