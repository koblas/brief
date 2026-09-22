---
id: SCENARIO-09
status: done
---

# SCENARIO-09: Status reports one four-field line per feature

## Scenario

```gherkin
Scenario: SCENARIO-09 Status reports one four-field line per feature  [orig: new]
  Given three features in different states
  When I ask for status
  Then I get one line per feature, sorted by name
  And each line carries name, steps done over total, next step, and blocked count
  And a feature with no next step shows "-" in that field
  And there is no header line and no legend
```

## The contract this scenario pins

**Invocation:** `brief status` — no arguments, no flags other than `-h`/`--help`.

**stdout:** one line per feature, `\n`-terminated (including the last), in `fs.ReadDir`
byte order of the feature directory names:

```
<name> <done>/<total> <next> <blocked>
```

- **separator** — a single `0x20` space, no padding, no alignment, no trailing space.
  Padding would still be whitespace-separated, but it makes one feature's line depend on
  the longest *other* feature's name: adding a feature elsewhere rewrites every line's
  bytes, which is noise in a `diff` of two runs and in every golden test. Alignment is
  `column -t`'s job, and *Decisions taken* 3 calls this table "the machine format".
- **`<name>`** — the feature's directory name. SCENARIO-07 guarantees it carries no
  whitespace, which is what makes the four fields recoverable by `strings.Fields` / `cut`.
- **`<done>/<total>`** — e.g. `1/3`. No spaces around the slash. `total` is the number of
  step files in the feature directory; `done` is how many of those have
  `stepfile.Frontmatter.Done()` true (STATE.md: `Done()` is the sole doneness authority —
  the specification's progress list is **not** read).
- **`<next>`** — `pattern.ID(n)` of the lowest-numbered step file that is not done, e.g.
  `SCENARIO-02`; `-` when there is none.
- **`<blocked>`** — a count, `0` or more.

**stderr:** nothing on success. **Exit codes:** 0 ok; 2 for a usage error (extra argument,
undefined flag); 1 for anything else (an invalid `.brief.yaml`, an unreadable tree).
`brief status --help` prints the subcommand usage to stdout and exits 0.

## Decisions this plan takes

**Placement — `(*assemble.Server).Status`, not a new `internal/status`.** Three things
point at `assemble`: STATE.md's *Left unbuilt* names `assemble.Server.Status` literally;
`internal/assemble/doc.go` says "assemble owns reading a feature, scaffold owns writing
one"; and `internal/platform/stepfile`'s package doc says "start, **status**, next, check
and finish all read them back through it". A new `internal/status` would need `assemble`'s
step enumeration (`readSteps`), and the dependency rule forbids a feature package importing
another feature package — so it would force moving step enumeration down to
`internal/platform` with no scenario demanding it. `assemble` is the read side of this
binary; `status` is a read.

`assemble/doc.go` currently says "Start is the only entry point" — that sentence is now
wrong and this scenario updates it.

**Next step ignores dependencies.** Same rule `Start` already uses: the lowest-numbered
step file whose frontmatter status is not `done`; `depends-on` is parsed but never orders.
R4's transitive closure is explicitly a later phase (*Phasing*, and the SCENARIO-21
comment). Two different definitions of "next" in one binary would be a defect.

**Next step is `pattern.ID(n)`, not the frontmatter `id:`.** `scaffold.findStepFile`
resolves `brief finish <feature> <step>` by comparing `pattern.ID(n) == step`, so the
printed token is exactly the one `finish` accepts, and it cannot be absent or drift from a
hand-edited frontmatter. (`assemble.RenderText` prints `fm.ID` instead — see *Traps*.)

**Blocked is computed here, not deferred.** No later scenario owns it (10 is the empty
repo, 11 is the malformed feature, 12 is the completed feature), so a hardcoded `0` would
be a permanent lie in the contract. It is the R4 dependency sense, at its cheapest:

- a step counts as blocked when it is **not done** and declares at least one `depends-on`
  id that is not the id of a done step in the same feature;
- ids are matched as `pattern.ID(n)`, consistent with `findStepFile` and with the `<next>`
  field;
- **direct dependencies only** — no transitive closure;
- an id naming no step file in the feature counts as **not done**, so it blocks. `brief`
  does not judge whether the id is a typo (R7); SCENARIO-22's `check` is the finding path;
- a **done** step is never counted, whatever its dependencies say;
- the blocked step may also be `<next>` — the two fields are computed independently.

**`-` means "no next step", for both reasons.** Every step done → `-`. Zero step files →
`0/0` and `-`. A freshly `brief new feature`'d directory has a specification and a state
file and no steps; that is conforming, not malformed, so it must render `0/0 - 0` and must
not be SCENARIO-11's `!`. The sentinel is applied in the renderer only: `FeatureStatus.Next`
stays `""`, so a later `--json` caller gets an empty/absent field rather than a `"-"` string.

**Sort order is `fs.ReadDir`'s documented byte order, deliberately not re-sorted.**
Verified, not assumed: `io/fs.ReadDir` is documented "returns a list of directory entries
sorted by filename", it delegates to `fsys.ReadDir` when the FS implements `fs.ReadDirFS`,
`fs.ReadDirFS.ReadDir`'s own interface contract repeats "sorted by filename", and
`os.Root.FS()` is documented to implement `fs.ReadDirFS`. So `fs.ReadDir(root.FS(), ".")`
— the call `readSteps` and `findStepFile` already use — is contractually sorted, and for a
feature directory the filename *is* the feature name: same key, same comparison. Adding a
`sort.Slice` over it would be code no mutation can redden (deleting it changes nothing),
i.e. dead code that looks load-bearing. `readSteps`'s explicit sort does not transfer: it sorts by the *step number*
parsed out of the filename, which is genuinely not byte order (`SCENARIO-10.md` sorts
before `SCENARIO-9.md`). Byte order, not case-insensitive collation, is the contract —
pinned by a test with case-mixed names so it cannot pass on this repo's case-insensitive
APFS dev platform and fail on a Linux CI. State the reliance in `Status`'s doc comment.

**What reads the tree.** Per feature, only the step files: enumerate the feature directory
with `fs.ReadDir`, recognize step files with `stepfile.Pattern.Number`, read each and call
`stepfile.ParseFrontmatter` for `Status`/`DependsOn`. `markdown.Section` /
`markdown.Title` are not needed here and must not be called — no heading is read, and
nothing in this scenario is prose. Reuse `stepfile` and `config`; reimplement neither.
`os.OpenRoot`, as `Start` and `Finish` already do, so a symlink cannot escape the root.

## Deliberately NOT built — leave 10, 11 and 12 a real red

- **Feature root missing / unreadable.** `os.OpenRoot(<root>/<feature-directory>)` fails and
  that error propagates: exit 1, nothing on stdout. Do not add an empty-list path, do not
  write a stderr line, do not return exit 0. **SCENARIO-10 owns** turning this into empty
  stdout + one stderr line + exit 0. Do not write a test pinning today's behaviour — 10
  would have to delete it.
- **Feature root present but holding no feature directories.** This already falls out as
  empty stdout + exit 0, so SCENARIO-10's only remaining red is the stderr line. Do not add
  that line here and do not write a test for this shape — both belong to 10.
- **A step file whose frontmatter does not parse.** The error propagates and the whole
  command fails; no line is printed for any feature. Do not add per-feature error
  tolerance, no `!` field, no partial output. **SCENARIO-11 owns** that.
- **Completed-feature messaging on `start`.** SCENARIO-12. `status` already renders a
  completed feature as `-`; nothing about `start` changes here.
- **`--json`.** *Decisions taken* 3: `--json` is `start`-only, and `status`'s table already
  is the machine format. No flag, no struct tags.
- **R13 output truncation** and the output budget — unowned, not this scenario.
- No feature-name re-validation on the read path (STATE.md: `NewFeature` is the sole choke
  point, deliberately).

## Implementation Plan

Each step names the test first, then the production change it forces. Verification commands
are in `.claude/rules/agent-briefs.md`; run them from the worktree root.

- [x] Step 1: `internal/assemble/status_test.go` `Test_status_counts_done_over_total_and_names_the_next_step` — one hand-built feature (3 step files, step 1 `status: done`) through `(*Server).Status`; expects one `FeatureStatus{Name, Done: 1, Total: 3, Next: "SCENARIO-02", Blocked: 0}`. Build fixtures by writing step files directly rather than through `scaffold` — `scaffold` cannot set `status:` or `depends-on` (red: `Status`/`FeatureStatus` undefined)
- [x] Step 2: `internal/assemble/status.go` — `FeatureStatus` (`Name`, `Done`, `Total`, `Next string`, `Blocked int`) and `(*Server).Status(ctx) ([]FeatureStatus, error)`: enumerate feature directories, reuse `readSteps` for each, count and pick next (green)
- [x] Step 3: `internal/assemble/status_test.go` `Test_status_counts_a_step_whose_dependency_is_unfinished_as_blocked` — an open step declaring `depends-on: [SCENARIO-01]` where SCENARIO-01 is open → `Blocked: 1`; a second open step depending on a **done** step → still not counted; and assert the blocked step is nonetheless the one reported in `Next` (red)
- [x] Step 4: `internal/assemble/status.go` — blocked computation over the done-set keyed by `pattern.ID(n)`, direct dependencies only (green)
- [x] Step 5: `internal/assemble/status_test.go` `Test_status_reports_an_unknown_dependency_id_as_blocking` — `depends-on: [SCENARIO-99]` with no such step file → `Blocked: 1` (red or green on arrival; if green, say so and say why rather than manufacturing a red)
- [x] Step 6: `internal/assemble/status_test.go` `Test_status_reports_no_next_step_for_a_completed_feature` and `Test_status_reports_no_next_step_for_a_feature_with_no_step_files` — `Next: ""`, `Done == Total`, and `0/0` respectively (red/green as they fall)
- [x] Step 7: `internal/assemble/status_test.go` `Test_status_orders_features_in_byte_order_not_case_insensitive_order` — feature directories `Zeta`, `alpha`, `Beta` (no two differing only by case, so APFS cannot collide them) → exactly `Beta`, `Zeta`, `alpha` (green on arrival is expected — it pins byte order against a future case-insensitive collation; say so in the report)
- [x] Step 8: `internal/assemble/status.go` — `Status` doc comment states that feature order is `fs.ReadDir`'s byte order and is relied on rather than re-sorted; update `internal/assemble/doc.go`'s "Start is the only entry point" sentence (update)
- [x] Step 9: `internal/assemble/render_test.go` `Test_render_status_writes_four_space_separated_fields_per_feature` — exact-bytes equality on a three-row slice built in the test, one row with `Next: ""` expecting `-`, one with `Blocked: 2` (red: `RenderStatusText` undefined)
- [x] Step 10: `internal/assemble/render.go` `RenderStatusText(w io.Writer, rows []FeatureStatus) error` — `"%s %d/%d %s %d\n"` with `-` substituted for an empty `Next`; wrap a write error once, as `RenderText` does (green)
- [x] Step 11: `internal/cli/status_test.go` `Test_status_prints_one_line_per_feature_and_nothing_else` — three features in different states through `cli.Run`, asserting **exact** `stdout` bytes and `assert.Empty(stderr)`; `want` is written out literally from the fixture's own inputs (`"alpha 1/3 SCENARIO-02 0\nbeta 3/3 - 0\ngamma 1/4 SCENARIO-02 1\n"` shape), never copied from a run (red: unknown command `status`)
- [x] Step 12: `internal/cli/status_test.go` `Test_status_prints_one_line_for_a_single_feature` — the **control arm** for the no-header/no-legend absence claim: the same exact-bytes probe against a one-feature fixture. A header or a legend is a constant line; an exact match at both one row and three rows means no constant line can hide in either, so line count varies with feature count and with nothing else (red)
- [x] Step 13: `internal/cli/status.go` — `runStatus`: `flag.FlagSet` named `status` with `SetOutput(io.Discard)`, `--help` → `statusUsage` to stdout + nil, undefined flag and any positional argument → `usageError`; then `config.Resolve(wd)`, root = `filepath.Dir(source)` when a config was found, `assemble.NewServer(cfg, root)`, `renderRefusal(stderr, "status", err)` on error, `assemble.RenderStatusText(stdout, rows)` on success — mirroring `internal/cli/start.go` exactly (new)
- [x] Step 14: `internal/cli/cli.go` — dispatch `case "status": return runStatus(ctx, wd, args[1:], stdout, stderr)`; add a `brief status` line to the `usage` const and to its "Run 'brief … --help'" sentence; extend both dispatcher messages to `expected one of: new, start, finish, status` (update — this reddens the two existing tests at `internal/cli/run_test.go:111` and `:122`, which pin those strings; update their expectations in the same step)
- [x] Step 15: `internal/cli/status_test.go` `Test_returns_a_usage_error_when_status_is_given_an_argument` and `Test_returns_a_usage_error_when_status_is_given_an_undefined_flag` — `cli.ExitCode(err) == 2`, `stdout` empty, one stderr line naming `brief status`; plus `Test_prints_usage_to_stdout_when_help_is_requested_for_status` — exit 0, stdout non-empty, stderr empty (red, then green if Step 13 already covers them — report which)
- [x] Step 16: `cmd/brief/main.go` — **no change**. `cli.Run` is already wired, `status` introduces no new dependency and no new exit-code class. Confirm by reading, change nothing (verify)
- [x] Step 17: mutation verification, one at a time, each stashed per the standing brief: (a) prepend a header line inside `RenderStatusText` → Step 11 **and** Step 12 go red; (b) append a legend line after the loop → Step 11 and Step 12 go red; (c) change `-` to `""` for an empty `Next` → Step 9 goes red; (d) count a done step's unfinished dependency as blocked → Step 3 goes red; (e) drop the `!done` condition from the blocked count → Step 3 goes red. Report which mutation reddened which named test
- [x] Step 18: `go build ./...`, `go test ./...` unpiped from the worktree root, `go test -race ./internal/assemble/... ./internal/cli/...`, `golangci-lint run ./...`; report the exact test count and the delta
- [x] Step 19: `go doc ./internal/assemble` — confirm `Status`, `FeatureStatus` and `RenderStatusText` read as a contract and that the package doc no longer claims `Start` is the only entry point (verify)
- [x] Step 20: all tests green → tick SCENARIO-09 in `docs/specifications/brief/specification.md`, write `SCENARIO-09-HANDOFF.md`, and rewrite `docs/specifications/brief/STATE.md` — **by hand, not through `brief finish`**: no `SCENARIO-NN.md` in this tree carries frontmatter (checked 01, 07, 08), so `brief finish brief SCENARIO-09 …` cannot resolve the step. Verified by running the binary: `go run ./cmd/brief start brief` → `brief start: assemble: no frontmatter found`, exit 1. SCENARIO-07 and 08 were closed the same way

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- `status` lives on `(*assemble.Server).Status`, rendered by `assemble.RenderStatusText` —
  `assemble` owns reading a feature, `scaffold` owns writing one, and a separate
  `internal/status` could not import `assemble.readSteps` under the dependency rule.
  `next`, `show`, `handoff` and `state get` belong here too.
- Line shape is `<name> <done>/<total> <next> <blocked>\n`, single-space separated, no
  padding — padding makes one feature's bytes depend on another feature's name length, and
  SCENARIO-11's `!` line must keep the same four-field shape.
- `<next>` is `pattern.ID(n)` of the lowest-numbered not-done step, so it is paste-ready as
  `brief finish <feature> <step>` (`scaffold.findStepFile` compares `pattern.ID(n) == step`).
  `<next>` ignores `depends-on` — same rule as `Start`; R4 ordering is a later phase.
- `-` is the no-next-step sentinel for both "all steps done" and "no step files at all", and
  it is applied **in the renderer only**: `FeatureStatus.Next` stays `""`, so a later
  `--json` caller sees an empty field rather than the string `"-"`.
- Blocked = not-done steps with at least one `depends-on` id that is not a done step's
  `pattern.ID(n)`; direct dependencies only; an id naming no step file blocks; a done step
  is never blocked; a blocked step can still be `<next>`. SCENARIO-21's `finish` refusal
  must use the same done-set-by-`pattern.ID(n)` rule or the two surfaces disagree about the
  same tree — and since `scaffold` cannot import `assemble`, 21 either reimplements it or
  moves it down to `internal/platform/stepfile`.
- `docs/specifications/brief/SCENARIO-09.md` is ticked and STATE.md rewritten **by hand**,
  because the tree is not frontmattered and `brief finish` cannot resolve the step (see
  Traps). Same route SCENARIO-07 and 08 took.
- Feature order is `fs.ReadDir`'s byte order of the directory names, relied on deliberately
  and **not** re-sorted — a `sort.Slice` over the same key is unfalsifiable dead code. The
  reliance is on documented behaviour, not observed behaviour: `io/fs.ReadDir` and the
  `fs.ReadDirFS.ReadDir` interface contract both say "sorted by filename", and `os.Root.FS()`
  is documented to implement `fs.ReadDirFS`. A future change of enumeration source must
  re-establish byte order itself.
- Doneness comes from `stepfile.Frontmatter.Done()`; `status` never reads the
  specification's progress list, and never reads any markdown heading.
- Dispatcher wording is now `expected one of: new, start, finish, status`, in both the
  no-command and unknown-command messages.

**Left unbuilt** — named so nobody assumes it exists:

- Missing/unreadable feature root → the `os.OpenRoot` error propagates, exit 1, empty
  stdout. **SCENARIO-10** owns empty stdout + one stderr line + exit 0.
- An unparseable step frontmatter fails the whole command; no feature gets a line.
  **SCENARIO-11** owns the `!` field, the stderr reason and exit 0.
- `assemble.Server.Next`/`.Show`/`.Handoff`/`.StateGet`, R13 truncation, the output budget
  — none built. No `status --json` flag and no `FeatureStatus` JSON tags
  (*Decisions taken* 3).

**Traps** — things that look right and are not:

- `assemble.RenderText` prints `b.Step.ID` (the **frontmatter** `id:`) while `status` prints
  `pattern.ID(n)`. They are equal in any scaffolded tree — `scaffold.go` sets
  `id := pattern.ID(next)` — and diverge only after a hand edit. Do not "unify" them by
  switching `status` to `fm.ID`: only `pattern.ID(n)` is what `finish` resolves.
- `fs.ReadDir` already returns entries sorted by name, so a "sorted by name" assertion
  passes with no sort in the code at all. The case-mix fixture (`Zeta`/`alpha`/`Beta`) is
  the discriminating probe — it fails against a case-insensitive collation, which is the
  only change that can actually break the contract. Do not claim the ordering is
  mutation-verified by deleting a sort.
- `assert.NoDirExists` / "no output" style assertions on a fresh `t.TempDir()` are vacuous
  here for the same reason they were in S07. The header/legend absence claim is only
  falsifiable as exact-bytes equality at **two different feature counts** (Steps 11 and 12).
- A feature directory with zero step files is conforming, not malformed — `0/0 - 0`, never
  `!`. SCENARIO-11's fixture must make its feature malformed some other way.
- `status` takes no feature argument, so `flag.FlagSet.Parse` needs no
  `splitLeadingPositionals` dance; any leftover positional is a usage error.
- **`brief status` cannot read `brief`'s own tree today.** No `SCENARIO-NN.md` here carries
  YAML frontmatter (the crossover never added it), so `readSteps` returns
  `ErrNoFrontmatter`: `go run ./cmd/brief start brief` → `brief start: assemble: no
  frontmatter found`, exit 1. `status` will do the same until SCENARIO-11 softens it to `!`
  + stderr + exit 0. Do not add frontmatter to one plan file — the other eight still fail.
- `renderRefusal` appends `(no files changed)` to a `*config.InvalidConfigError`, which
  `status` reaches through a bad `.brief.yaml`; R14a says a **read** refusal drops that tail.
  Pre-existing — `start` does the same. Do not fix it here, do not file it against 09.
