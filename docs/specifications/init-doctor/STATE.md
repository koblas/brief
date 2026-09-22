# init-doctor — current state

Scenarios complete: SCENARIO-01, SCENARIO-02, SCENARIO-03. Last updated by SCENARIO-03.

## Binding decisions
- `config.Locate` is the one walk-up; `config.Inspect` is the one all-violations decoder;
  `Resolve` = `Locate` + first element of `Inspect`, reporting only `violations(cfg)[0]` in
  `Config` field order (R14a). No second rule list or walk-up anywhere; `doctor`'s
  config-values (every element) and `init`'s own refusal (first element) both build on this
  (SCENARIO-01, SCENARIO-02, SCENARIO-03)
- A value error is `*config.InvalidConfigError{Path, Err: *config.ValueError{Key, Value,
  Reason, Err}}`, recovered via `errors.AsType[*config.ValueError]` (SCENARIO-01, SCENARIO-03)
- `internal/doctor` and `internal/setup` each import only `internal/platform/*` + stdlib —
  never each other, `scaffold`, or one another as a feature package. Neither has a `Store`
  port: both read/write the real filesystem directly. Both call `config.Locate` directly,
  never `resolveRoot`/`Resolve`: `doctor` turns an invalid config into rows, `init` classifies
  it itself via `Inspect` so `--force` can still rewrite it (SCENARIO-02, SCENARIO-03)
- Rendered artifacts and their digest registry live in `internal/platform/artifact`
  (`ConfigFile()`, `Recognize(Kind, body) Origin`), not in `internal/setup` — S04 and S10 both
  consume `Recognize`. Every render is deterministic (no version/date/map order); a changed
  render appends a new digest, old ones stay (S10's "older release" origin) (SCENARIO-03)
- `.brief.yaml` line shapes: `# ` (hash space) = doc prose, kept forever; `#<yaml>` (hash, no
  space) = a commented value or structural line — stripping one leading `#` from every line
  not beginning `# ` reproduces `ConfigFile()`'s own marshalled bytes exactly, decoding to
  `config.Default()`. No live line at all (no `---`) (SCENARIO-03)
- `init` plan-then-apply: every refusal (unknown host, invalid/unparseable config, feature
  root not a directory) is decided before the first write. Apply order is feature root then
  `.brief.yaml` (the opt-in marker lands last); a failure after the first write wraps
  `setup.ErrPartialWrite`, which `filesChangedFor` checks alongside `scaffold.ErrPartialWrite`.
  `DryRun` computes the identical plan and skips only the apply step (SCENARIO-03)
- `--force` only ever rewrites `.brief.yaml` from `config.Default()` (`created`, detail
  "rewritten from defaults") and never reads old config content to decide refusal — a read
  failure (a directory at that path) is treated as "not current" and surfaces at the write
  attempt instead. JSON `created` = written fresh; `modified` is reserved for S07's CLAUDE.md
  merge. Existing valid config not byte-equal to the render → `kept`, detail "edited locally",
  and its own `feature-directory` governs the feature root (SCENARIO-03)
- Omitted `--host` means `setup.HostNone` ("none") in S03; accepted hosts come from
  `setup.Hosts()`. S06 appends `claude-code`; S09 replaces the default with R8 detection
  (SCENARIO-03)
- `classifyRefusal` checks `*setup.RefusalError` **before** `*config.InvalidConfigError`:
  `init`'s own refusal wraps an `*InvalidConfigError` as its `Err`, and the reverse order
  would silently reclassify it, losing the `brief init --force` fix (SCENARIO-03)
- Command order: `new, start, finish, status, check, init, doctor` (S04 appends `uninstall`)
- No config anywhere is WARN (fix `brief init`), not ERROR — un-inited repos exit 0.
  Environment seams enter via `doctor.With*` options and `run`'s trailing `...doctor.Option`
  (SCENARIO-02)

## Left unbuilt
- Line number for a value error (no `yaml.Node` decode) — unowned (SCENARIO-01)
- Validation of `roles.*` and `optional-conventions` — S08 (SCENARIO-01)
- Heading-shape rules, absolute-path/`..` check on `feature-directory` — unowned (SCENARIO-01)
- `host-plugin`, `host-hook`, `host-snippet`, `host-agents`, `roles` doctor rows, env-path's
  ERROR arm, and `artifact.Recognize`'s `older` origin — S10 (SCENARIO-02, SCENARIO-03)
- `uninstall` in the command list (inserted after `doctor`) and `setup.(*Server).Uninstall`,
  `removed` action, JSON `removed` — S04
- Host `claude-code`, kinds `plugin`/`hook`/`agent`, `--no-hook`, its own stderr copy — S06;
  `snippet`, `merged` — S07; `--with-agents`, `roles` lines in the config — S08
- `--print`, host detection, `--dry-run`/`--print` exclusivity, R10 writability pre-check — S09

## Traps
- Any CLI test that writes a `.brief.yaml` to reach a scaffold/assemble seam is refused at
  load if invalid — build config directly instead of routing setup through `cli.Run`
  (SCENARIO-01)
- `ValueError.Err` must wrap stepfile errors with `%w`; `%q`-formatting a `Value` containing a
  backslash doubles it in `Error()` text (SCENARIO-01)
- A `t.TempDir` has no `.git` above it — a "healthy" doctor fixture must create one or env-git
  flips to WARN; `chmod 0o000`/`0o555` root-dir tests pass vacuously as root — skip under
  `os.Geteuid() == 0`; `filepath.EvalSymlinks` on the found PATH binary must never leak into
  `Check.Path` (SCENARIO-02)
- Root-help rows (`doctor`'s, `init`'s) are exempt from the 80-column test (fixed-column
  `cmdList`) — `initLong`/`doctorLong` prose is not (SCENARIO-02, SCENARIO-03)
- `doctor.Report.Counts()`'s switch has no `default` arm — S10 adding a fifth `Severity`
  without a matching `case` makes `counts.Error+Warn+OK+Skip != len(Checks)` uncaught
  (SCENARIO-02)
- `--force` over a config with a non-default `feature-directory` leaves the old feature root
  in place and creates the default one; S04 must still never remove either (SCENARIO-03)
- An all-comment `.brief.yaml` resolves via `Inspect`'s `io.EOF`-as-empty branch — any future
  `artifact` render that emits one live line breaks the `Default()` round trip (SCENARIO-03)
- `filesChangedFor` only returns non-nil for commands carrying `writesFilesAnnotation` —
  forgetting it on a new write command makes `files_changed` null instead of `false`/`true`
  (SCENARIO-03)

## Open debts
- `roles.*` / `optional-conventions` validation — S08 must close it, or the spec must say
  explicitly these stay unvalidated
