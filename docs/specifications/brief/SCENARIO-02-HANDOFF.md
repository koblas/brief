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
