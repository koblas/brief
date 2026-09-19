# SCENARIO-03: New step scaffolds the next step file and its progress entry

## Scenario

```gherkin
Scenario: SCENARIO-03 New step scaffolds the next step file and its progress entry  [orig: 03b]
  Given a conforming feature
  When I create a new step
  Then the step file has an id, frontmatter, an empty checklist and a handoff anchor
  And its frontmatter carries an empty depends-on key
  And a matching entry appears in the progress list
```

---

## Decisions this scenario takes

Five deferred decisions come due here. Each is stated with the constraint that forced it.
Items marked **[spec silent — chosen]** are places the specification does not decide and this
plan does; a later scenario may reverse one only by saying so.

### 1. `step-file-pattern` is a single-integer `fmt` pattern, and the reader is a strict round-trip

`config.Default().StepFilePattern` is `"SCENARIO-%02d.md"` and SCENARIO-01 shipped it. Keeping
`fmt` semantics costs nothing and changing the shipped default would be a migration. What was
missing is the **read** side: a `fmt.Sprintf` pattern is write-only until the match is defined.

A new platform package `internal/platform/stepfile` owns both directions, because `scaffold`
writes step files and `start` / `status` / `next` / `check` / `finish` will all read them, and a
feature package may not import another feature package.

**Validation (`Compile`), refusing with `ErrInvalidPattern`:** **[spec silent — chosen]**

- The pattern must contain **exactly one** integer verb, matching `%[0-9-]*d` (`%d`, `%02d`,
  `%3d`, `%-4d`). Zero verbs → refused (every step would get the same filename). Two or more →
  refused (`Sprintf` would need two arguments).
- **No other `%` may appear anywhere**, so `%s`, `%v` and the escape `%%` are all refused. This
  is what makes the literal prefix and suffix unambiguous without unescaping.
- Empty pattern → refused. A pattern containing `/` or `\`, or equal to `.` or `..` → refused;
  a step "file" pattern that names a subdirectory breaks the flat feature-directory model.

**Matching (`Number`) is strict, by round trip:** a directory entry is a step file for `n` iff
`Sprintf(pattern, n) == filename` exactly. `SCENARIO-7.md` under `%02d` is therefore **not** a
step file. Leniency was considered and rejected: with a lenient parse `SCENARIO-7.md` and
`SCENARIO-07.md` both yield 7, producing a file that is discoverable by the scanner but not
addressable by id or by `depends-on` — an unaddressable-step class baked into the scanner every
later read command inherits. Strict matching costs nothing on width overflow (`%02d` with 100
renders and round-trips `SCENARIO-100.md`), and a near-miss filename is exactly the kind of
thing `check` (SCENARIO-22) can later report as a finding.

**Step id** is the rendered filename with its extension stripped — `SCENARIO-03` for the default
pattern, `step-3` for `step-%d.txt`. **[spec silent — chosen]**

### 2. The next N is the highest existing step number plus one

Derived by scanning the feature directory with `Number`, taking the maximum, adding one; no
matching files → 1. Rejected: *count of files + 1* and *progress-list length + 1*, both of which
collide with an existing file the moment a number is skipped or a step file is deleted, and the
second of which infers a machine field from prose, against R3. Max+1 also means a number is
never reused, so an id is stable forever — which SCENARIO-05's progress marking and
SCENARIO-21's `depends-on` references both depend on. The gap case is tested.

### 3. `status` is a frontmatter field; doneness has exactly one authority

R3 names status as a YAML field ("Step id, dependencies and status are parsed from YAML, never
inferred from English"). The model line "a step is done when its handoff is recorded" cannot be
the parsing rule, because this scenario scaffolds an **empty but present** handoff anchor, so
anchor presence is true from birth; the alternative — "anchor has non-blank content" — is prose
inference for the graph, which is precisely what R3 forbids.

So: **`status:` in step frontmatter is the sole authoritative doneness field.** The progress-list
checkbox is a projection `finish` keeps in sync; the handoff body is the audit trail. The model
line describes *when* `finish` flips `status`, not how a reader computes it. **[spec silent —
chosen: three candidate doneness signals existed and the spec does not rank them.]**

### 4. The handoff anchor is the configured heading line, alone

`cfg.HandoffHeading` (default `## Handoff`), written last in the file with nothing under it. The
Default profile's "three labelled groups" are the caller's obligation under R7, not the
scaffold's — and `finish` (SCENARIO-05) replaces the handoff body wholesale, so pre-written
labels would be overwritten and would spend lines of the ~60-line cap. The trichotomy
SCENARIO-13 needs is then decidable by a line scan: **absent** = no line equal to
`cfg.HandoffHeading`; **present-and-empty** = that line exists with no non-blank line between it
and EOF-or-the-next-heading; **present-and-filled** = otherwise.

### 5. The atomic-write primitive is pulled forward from SCENARIO-05

STATE.md assigns it to SCENARIO-05; this plan **moves it here and says so**. `## Phasing` is the
authority: "Atomicity is pre-crossover whatever else moves, because from the crossover onward a
bad write damages this tool's own specification." SCENARIO-03 is the first command that modifies
an existing file, and the file it modifies in this repository is a 47 KB approved Source of
Truth. A truncate-then-write would put a crash window over it.

Scope is deliberately minimal — `internal/platform/atomicfile.WriteFile(root *os.Root, name
string, data []byte, perm fs.FileMode) error`:

- temp sibling with the **deterministic** name `.<name>.brief-tmp`, opened
  `O_CREATE|O_TRUNC|O_WRONLY` (**not** `O_EXCL`: a leftover temp from a crash must be overwritten,
  not wedge every future write; R20's single-writer guarantee makes collision a non-concern);
- when the target already exists **and `root.Lstat` reports it as a regular file**, the temp takes
  the target's mode, so an atomic write does not silently demote a `0644` specification to
  `0600`; `perm` applies otherwise. The `IsRegular` guard is load-bearing, not defensive: without
  it, a target that is a *directory* hands `ModeDir` to `OpenFile` and the write fails at temp
  creation instead of at `Rename` — which is exactly the injection Step 6 uses, and it would make
  Step 6 and Step 8 prove something other than what they name. An explicit `Chmod` on the temp
  defeats umask and a leftover temp's stale mode;
- write, close, `root.Rename(tmp, name)`; on any failure after the temp opens, best-effort
  `root.Remove(tmp)`, then return the wrapped error;
- **no `fsync`.** R12 says atomic, not durable. Durability across power loss is not claimed.

SCENARIO-05 then *uses* this primitive rather than inventing it, and its "no temp file remains"
assertion exercises it through `finish`.

### 6. Other choices this plan makes

- **`ChecklistHeading` joins `config.Config`**, default `"## Implementation Plan"`, because R2
  makes heading text configuration and "an empty checklist" is unobservable without a heading to
  write it under. Precedent: SCENARIO-02 added `SpecificationFile` and `StateFile` the same way.
  Adding a config key pre-crossover is free; post-crossover it is a migration (*Bootstrap
  hazards*). **[spec silent — chosen]**
- **The step file writes a `# <id>` title heading** as well as the frontmatter `id:` key — the
  gherkin lists "an id" separately from "frontmatter". It is an id, not prose, so *Out of scope:
  Authoring* is respected. SCENARIO-04 owns what "title" means once a planner extends that line.
- **Frontmatter is hand-rendered, not `yaml.Marshal`ed**, so key order and the exact
  `depends-on: []` spelling are pinned bytes. Readers still parse it with yaml.v3.
- **Write order: step file first, then the specification.** If the specification write fails
  after the step file lands, the result is an orphan step file with no progress entry — visible,
  repairable, and the specification is untouched. The reverse order leaves a progress entry
  pointing at nothing, and a re-run (N recomputed from files, unchanged) would append a *second*
  entry for the same N. No rollback is planned: the failure cannot be injected without a
  contrived fixture, and an untested branch is worse than the window. Recorded as a debt.
- **Validation order — a refusal names the first thing wrong (R14a):** config resolves →
  `step-file-pattern` compiles → feature directory opens → specification reads → progress heading
  found → *then* N is computed and anything is written. Nothing is created before all five pass.

---

## User-visible contract

```
brief new step <feature>
```

One positional argument. No flags beyond `--help`.

| Condition | stdout | stderr | exit |
| --- | --- | --- | --- |
| success | created step file's path, relative to `wd`, one line | empty | 0 |
| `brief new step --help` | the subcommand usage block | empty | 0 |
| no feature given | empty | `brief new step: no feature given; run 'brief new step <feature>'` | 2 |
| too many arguments | empty | `brief new step: too many arguments; run 'brief new step <feature>'` | 2 |
| undefined flag | empty | `brief new step: flag provided but not defined: -x; run 'brief new step <feature>'` | 2 |
| invalid `.brief.yaml` | empty | existing `*config.InvalidConfigError` template (SCENARIO-01/02) | 1 |
| unknown feature | empty | R14a line, contains the feature directory path, ends `(no files changed)` | 1 |
| specification file missing | empty | R14a line, contains the specification path, ends `(no files changed)` | 1 |
| no progress heading | empty | R14a line, contains the specification path **and the configured heading text**, ends `(no files changed)` | 1 |
| invalid `step-file-pattern` | empty | R14a line, names `step-file-pattern` and the offending value, ends `(no files changed)` | 1 |
| filesystem failure | empty | one flattened line (existing `renderRefusal` fallback) | 1 |

Refusal lines carry **absolute** paths (consistent with SCENARIO-01's config refusal); the
success line carries a path **relative to `wd`** (consistent with `new feature`).

R14's "an unknown feature lists the known ones" is **not** built here — it needs a feature
enumerator, which `status` (SCENARIO-09) builds. This scenario's unknown-feature copy names the
path it looked for and the command that creates it.

---

## File shapes

Step file, rendered with the shipped defaults for id `SCENARIO-03`:

```
---
id: SCENARIO-03
status: open
depends-on: []
---

# SCENARIO-03

## Implementation Plan

## Handoff
```

(frontmatter block, blank line, `# <id>`, blank line, `cfg.ChecklistHeading`, blank line,
`cfg.HandoffHeading`, single trailing newline — every heading from config, none hardcoded.)

Progress entry, appended as one line: `- [ ] SCENARIO-03` — id only, no title, because the tool
writes no prose. SCENARIO-05 must match it by id at the start of the item text, tolerating a
trailing `: <title>` a human added.

Splice rule for `insertProgressEntry(body, heading, entry)` — pure, string in / string out:

1. Remember whether `body` ends with `\n`; strip it, split on `\n`, re-append at the end.
2. Heading match: first line whose right-trimmed text equals `heading`. None → `ErrNoProgressHeading`.
   A second occurrence is ignored (first wins).
3. Section end: the next line starting with `#`, or end of file.
4. Insert **after the last checklist item** (`- [ ]` / `- [x]`, leading whitespace allowed) inside
   that section. No item in the section → insert a blank line then the entry, immediately after
   the heading — this is the freshly-scaffolded feature's shape and therefore the commonest path.
5. Every other byte of the file is preserved.

---

## Implementation Plan

Commands below run from the worktree root. `go test ./...` unpiped is the evidence; see
`.claude/rules/agent-briefs.md` for the standing verification and mutation rules.

**A. `internal/platform/stepfile` — the pattern's two directions**

- [x] Step 1: `internal/platform/stepfile/pattern_test.go` (`package stepfile_test`) — the
      `Compile` refusal matrix: `""`, `"SCENARIO.md"` (no verb), `"%d-%d.md"` (two verbs),
      `"SCENARIO-%s.md"` (wrong verb), `"100%%-%d.md"` (escape present), `"steps/%d.md"` (path
      separator); each asserts `require.ErrorIs(t, err, stepfile.ErrInvalidPattern)` (red —
      `go test ./internal/platform/stepfile/...` fails to build, package absent)
- [x] Step 2: same file — `Name`/`ID`/`Number` matrix on `"SCENARIO-%02d.md"`: `Name(3)` is
      `"SCENARIO-03.md"`, `ID(3)` is `"SCENARIO-03"`, `Number("SCENARIO-03.md")` is `(3, true)`,
      `Number("SCENARIO-100.md")` is `(100, true)` (width overflow round-trips),
      `Number("SCENARIO-7.md")` is `(0, false)` (strict — unpadded is not a step file),
      `Number("SCENARIO-XX.md")`, `Number("SCENARIO-03.markdown")` and `Number("STATE.md")` are
      `(0, false)`; plus one non-default pattern `"step-%d.txt"` asserting `Name(3)` is
      `"step-3.txt"` and `ID(3)` is `"step-3"` (red)
- [x] Step 3: `internal/platform/stepfile/{doc.go,pattern.go}` — `ErrInvalidPattern`, `Pattern`
      with unexported prefix/verb/suffix, `Compile`, `Name`, `ID`, `Number` (round-trip match)
      (green — `go test ./internal/platform/stepfile/...`)

**B. `internal/platform/atomicfile` — the R12 primitive, pulled forward**

- [x] Step 4: `internal/platform/atomicfile/write_test.go` (`package atomicfile_test`) — writes a
      new file's content; replaces an existing file's content; after a successful write the
      directory holds **exactly** the target (`assert.ElementsMatch` on `os.ReadDir`, so any temp
      name is caught) (red)
- [x] Step 5: same file — `Test_keeps_the_existing_files_permissions_when_it_replaces_it`: target
      pre-created `0o644`, write, `os.Stat().Mode().Perm()` is still `0o644` (red)
- [x] Step 6: same file — `Test_leaves_the_target_unchanged_and_no_temp_file_when_the_rename_cannot_land`:
      the target name is an existing **non-empty directory**, so the temp opens normally (per the
      `IsRegular` guard) and `Rename` is what fails; assert `require.Error`, the directory still
      holds its sentinel file, and the parent holds no `.brief-tmp` entry (red)
- [x] Step 7: `internal/platform/atomicfile/{doc.go,write.go}` — `WriteFile` per *Decision 5*,
      including the `root.Lstat` + `Mode().IsRegular()` guard on the mode lookup; doc comment
      states plainly that no `fsync` is performed and durability is not claimed (green —
      `go test ./internal/platform/atomicfile/...`)
- [x] Step 8: mutation-verify Step 6 — delete the `root.Remove(tmp)` cleanup, observe only that
      test redden, restore, `diff` byte-identical. **Before trusting it, confirm the temp file
      actually existed**: with the cleanup removed the `.brief-tmp` entry must be present on disk.
      If it is not, the write failed before temp creation and the mutation proved nothing. Also
      note for the record (do **not** keep as a test): `chmod 0o400` on the target and the write
      still succeeds, because rename replaces a directory entry regardless of the target's mode —
      that is what distinguishes this implementation from `root.WriteFile`, and it is a trap, not
      a promise

**C. `internal/platform/config` — the checklist heading**

- [x] Step 9: `internal/platform/config/config_test.go` — `Default().ChecklistHeading` is
      `"## Implementation Plan"`; `resolve_test.go` — a `.brief.yaml` setting
      `checklist-heading:` overrides it and leaves the other headings at their defaults (red)
- [x] Step 10: `internal/platform/config/config.go` — add `ChecklistHeading string
      \`yaml:"checklist-heading"\`` and its default; extend the `Config` doc comment (green)

**D. `internal/scaffold` — `NewStep`**

New tests go in `internal/scaffold/step_test.go` (`package scaffold_test`), keeping
`scaffold_test.go` under the size guidance. Extend the existing `fixtureConfig()` helper with
`ChecklistHeading`, `HandoffHeading` and `StepFilePattern` values that all differ from
`config.Default()`, so a hardcoded default cannot pass any test here. **Fix the pattern at
`StepFilePattern = "STEP-%02d.md"`** — every seeded filename below is written in terms of *that*
pattern, not the shipped one. Seeding `SCENARIO-*.md` files under a `STEP-*` fixture pattern
makes them non-matching noise, the scan finds nothing, and Steps 18 and 19 pass under both the
strict and the lenient `Number` — proving nothing.

- [x] Step 11: `step_test.go` `Test_writes_the_step_file_with_frontmatter_a_title_an_empty_checklist_and_a_handoff_anchor`
      — scaffold a feature, call `NewStep`, assert **exact byte equality** of the file against the
      *File shapes* skeleton rendered with the fixture's headings and pattern (red)
- [x] Step 12: `step_test.go` `Test_appends_the_progress_entry_under_the_progress_heading_when_the_list_is_empty`
      — against the spec file `new feature` just wrote; assert the whole file equals
      `"# widgets\n\n## Progress\n\n- [ ] <id>\n"` (red — the commonest real invocation, and a
      different branch from Step 13)
- [x] Step 13: `step_test.go` `Test_appends_the_progress_entry_after_the_last_existing_checklist_item`
      — seed a specification with two existing entries **and a `## Notes` section after the
      progress list**; assert the whole file equals the seed with exactly one line inserted after
      the last item, proving the trailing section is byte-preserved (red)
- [x] Step 14: `internal/scaffold/render.go` — `stepSkeleton(cfg, id)`, `progressEntry(id)` and
      `insertProgressEntry(body, heading, entry)`, all unexported and exercised only through
      `NewStep` (never called directly by a test) (new)
- [x] Step 15: `internal/scaffold/errors.go` — `ErrNoSuchFeature`, `ErrMalformedFeature`,
      `ErrNoProgressHeading`, and `RefusalError{Path, Problem, Fix string; Err error}` whose
      `Error()` is `"<path>: <problem>; <fix>"` and whose `Unwrap()` returns the sentinel (new)
- [x] Step 16: `internal/scaffold/scaffold.go` `(*Server).NewStep(ctx, feature) (string, error)`
      — validation order from *Decision 6*, `featureRoot.OpenRoot(feature)` as both the traversal
      guard and the existence check, step file via `writeExclusive`, specification via
      `atomicfile.WriteFile` (green — Steps 11–13 pass)
- [x] Step 17: `step_test.go` `Test_returns_the_path_of_the_created_step_file` and
      `Test_leaves_no_temp_file_in_the_feature_directory` (`ElementsMatch` on `os.ReadDir` of the
      feature directory) (red → green)
- [x] Step 18: `step_test.go` `Test_numbers_the_next_step_from_the_highest_existing_step_file`
      — seed `STEP-01.md` and `STEP-03.md` (both matching the fixture pattern) **and two progress
      entries**, assert `STEP-04.md` is created; the seed is asymmetric on purpose so *count+1*
      and *list-length+1* both yield 03 and only max+1 yields 04 (red → green)
- [x] Step 19: `step_test.go` `Test_ignores_files_that_do_not_match_the_step_file_pattern_when_numbering`
      — seed `STEP-01.md` (matching) plus `SPEC.md`, `NOTES.md` and **`STEP-7.md`** — a near-miss
      of the *fixture* pattern, unpadded; assert `STEP-02.md` is created. This is the control arm
      for the strict round trip: a lenient `Number` reads `STEP-7.md` as 7 and yields `STEP-08.md`
      (red → green)
- [x] Step 20a: `step_test.go` — three separate refusal tests, each asserting its sentinel **and**
      that no step file exists under the feature directory: unknown feature (`ErrNoSuchFeature`);
      specification missing (`ErrMalformedFeature`); progress heading missing
      (`ErrNoProgressHeading`), with the specification additionally asserted **byte-identical** to
      its seeded, visibly-different content (red → green)
- [x] Step 20b: `step_test.go` — two more separate refusal tests: `cfg.StepFilePattern` set to
      `"STEP-%s.md"` refuses with `stepfile.ErrInvalidPattern` and writes nothing; feature name
      `"../escaped"` returns an error and `assert.NoFileExists` beside the feature root (red → green)
- [x] Step 21: mutation-verify, one at a time, stashing each per the standing brief. (1) max+1 →
      count+1 must redden Step 18 only. (2) strict `Number` → lenient (drop the round-trip
      re-render, keep the digit parse) must redden Step 19 only. (3) The progress-heading guard
      lives *inside* `insertProgressEntry`, so it cannot be disabled independently of the splice —
      mutate it surgically to **append at EOF when the heading is absent** instead of returning
      `ErrNoProgressHeading`, and confirm that reddens Step 20a's byte-identity arm. Report which
      mutation reddened which test

**E. `internal/cli` — the command surface**

- [x] Step 22: `internal/cli/run_test.go` — repoint `Test_returns_a_usage_error_when_the_type_is_unknown`
      off `"step"` (it becomes a real type) onto `"widget"`, and update its expected line to
      `expected one of: feature, step` (update — this test currently asserts `unknown type "step"`
      and will otherwise fail silently-late)
- [x] Step 23: `internal/cli/run_test.go` — update `Test_returns_a_usage_error_when_no_type_is_given`
      to `expected one of: feature, step` (update — a separate assertion from Step 22)
- [x] Step 24: `internal/cli/new_step_test.go` (`package cli_test`) — the usage matrix through
      `cli.Run`: happy path (relative path on stdout, empty stderr, nil error); no feature given;
      too many arguments; undefined flag `-x`; `--help` to stdout with a nil error. Each usage
      case asserts `require.ErrorIs(t, err, cli.ErrUsage)`, empty stdout, and the exact line from
      the *User-visible contract* table via the existing `oneLine` helper (red)
- [x] Step 25: `internal/cli/new_step_test.go` — the refusal matrix: unknown feature, missing
      progress heading, invalid `step-file-pattern` from an ancestor `.brief.yaml`. Each asserts
      `require.ErrorIs` on the scaffold sentinel, `cli.ExitCode(err)` is 1, exactly one line on
      stderr, that line `Contains` the absolute path (and for the heading case, the configured
      heading text) and `HasSuffix` `"(no files changed)"` (red)
- [x] Step 26: `internal/cli/refusal.go` — add an `errors.AsType[*scaffold.RefusalError]` branch
      rendering `brief <cmd>: <path>: <problem>; <fix> (no files changed)`, above the flatten
      fallback (green)
- [x] Step 27: `internal/cli/new.go` — `newStepUsage` const, `runNewStep`, and the `"step"` case
      in `runNew`'s switch with both `expected one of: feature, step` messages updated (green)
- [x] Step 28: `internal/cli/cli.go` — add the `brief new step <feature>` line to the top-level
      `usage` const (update). **`cmd/brief/main.go` needs no change** — no new dependency crosses
      the wiring boundary and the exit-code mapping already covers both classes
- [x] Step 29: `internal/scaffold/doc.go` and `internal/cli/doc.go` — extend the package docs to
      cover `new step`; `go doc ./internal/scaffold` and `go doc ./internal/platform/stepfile`
      read as contracts (update)
- [ ] Step 30: `go build ./...`, `go test ./...` unpiped with the exact test count and its delta,
      `go test -race ./internal/scaffold/... ./internal/cli/... ./internal/platform/...`,
      `golangci-lint run ./...` all green → tick SCENARIO-03 in `specification.md`'s
      `## BDD Acceptance Progress`, and rewrite `STATE.md` from the Handoff below

---
