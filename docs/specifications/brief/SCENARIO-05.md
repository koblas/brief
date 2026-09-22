---
id: SCENARIO-05
status: done
depends-on: []
---

# SCENARIO-05: Finishing writes the handoff and replaces the state, then marks done

## Scenario

```gherkin
Scenario: SCENARIO-05 Finishing writes the handoff and replaces the state, then marks done
  Given an open step whose checklist is complete
  And an existing state file carrying entries from earlier steps
  When I finish it with a valid handoff block and a valid replacement state body
  Then the handoff block of that step file holds what I supplied
  And the state file holds exactly the replacement body, not the old body plus the handoff
  And the step is marked done in the progress list
  And no temp file remains in the feature directory
```

## Decisions

### 1. `finish` lives in `internal/scaffold`, as `(*Server).Finish`

`scaffold` already owns everything `finish` needs and nothing it does not: `RefusalError`,
`ErrNoSuchFeature`/`ErrMalformedFeature`/`ErrNoProgressHeading`, the progress-section scanner
(`insertProgressEntry`), the `atomicfile` write discipline, and the `os.Root`-scoped traversal
guard. A separate `internal/finish` package would have to import `internal/scaffold` — which
the dependency rule forbids — or duplicate all four. The split this codebase actually runs on
is **read side (`assemble`) vs write side (`scaffold`)**, which STATE.md already fixed for
`assemble` ("`status`/`next`/`show`/`handoff`/`state get` join it as further methods, never new
packages"); `finish`, `state set` and every later write command join `scaffold` by the same
rule. `finish` needs nothing from `assemble` — it reads only the three files it is about to
write.

Consequence: the package name understates the package. `scaffold`'s `doc.go` must be rewritten
in this scenario to say it owns the **write path** for a feature directory — creating one
(`new feature`, `new step`) and closing a step in one (`finish`) — while `assemble` owns the
read path and the two never import each other. A rename of the package is a mechanical debt,
recorded, unowned.

### 2. Three files, one atomic outcome — honestly, there isn't one

`finish` writes three files: the **state file** (whole body replaced), the **step file**
(handoff spliced + `status: done`), the **specification** (progress checkbox ticked). R12 gives
atomicity per file. R20 forbids a journal, a lock and a CAS. So a three-file atomic outcome is
**not available** and this plan does not pretend otherwise.

What it does instead:

- **Validate everything before writing anything** (order pinned in §3). After validation the
  only remaining failure is the write itself, which shrinks the window to two gaps.
- **Order the writes so every gap self-heals on a re-run**: `state` → `step file` →
  `specification`.
  - Crash after `state`: the step is still **open**, so a re-run takes the full path and
    converges. Damage: the state file has advanced ahead of an open step; `start` returns the
    same step with the new inherited context.
  - Crash after `step file`: frontmatter says done, the checkbox lags. `next`/`status` read
    frontmatter (the authority, per STATE.md), so the tool behaves correctly; only the human
    projection is stale, and a re-run fixes it.
  - The reverse order is strictly worse, and the reason is SCENARIO-16, not aesthetics: writing
    the step file first can leave `status: done` beside the **old** state body, and a retry then
    hits SCENARIO-16's differing-inputs path (recorded state ≠ supplied state) and is **refused**
    — the feature becomes unrepairable through the tool. State-first leaves the step open, so
    no identity check fires and the retry converges. This binds how 06 and 16 implement identity
    detection: identity is only consulted for a step whose frontmatter already says done.
- **A mid-sequence failure never reports success.** `Finish` returns the write error, and that
  error is **not** a `RefusalError`: the `(no files changed)` tail would be a lie. It renders
  through `renderRefusal`'s plain flatten branch (no tail), exit 1, and its message names the
  file that failed and says to re-run the same command.

Remaining exposure — recorded as an open debt, `check` (SCENARIO-22) is the natural detector: a
half-applied `finish` is possible, and only the **first** write's failure is testable (there is
no seam to inject a failure between writes without a filesystem port, which `scaffold`'s
`doc.go` deliberately rejected). Step 21 tests the first-write failure by making the feature
directory unwritable.

### 3. Validation order — pinned, because five later scenarios extend it

R14a says a refusal names the **first** thing wrong, so the order is a contract, not an
implementation detail. For this scenario:

1. step-file pattern compiles (`stepfile.ErrInvalidPattern`)
2. feature directory opens (`ErrNoSuchFeature`; also the traversal guard, as in `NewStep`)
3. a step file exists whose `pattern.ID(n)` equals the `<step>` argument (`ErrNoSuchStep`, new)
4. its frontmatter parses (`stepfile.ErrNoFrontmatter` / wrapped YAML error)
5. its handoff anchor is present (`ErrMalformedFeature`, naming `cfg.HandoffHeading`)
6. the specification is readable (`ErrMalformedFeature`, same copy `NewStep` already uses)
7. the specification carries the progress heading (`ErrNoProgressHeading`)
8. the progress list carries an entry for this step id (`ErrNoProgressEntry`, new)
9. the state file exists and is a regular file (`ErrMalformedFeature`)

Where the later scenarios slot in, so they do not re-litigate this:

- **17/18/19** (over-cap handoff, over-cap state, missing required state heading) are pure
  input-shape checks needing no disk read — they go **before 1**.
- **20/21** (open checklist item, unfinished dependency) need the frontmatter and the checklist
  section, both already in hand — they go **after 4**.
- **06/16** (identical re-finish is a no-op, differing re-finish is refused) need the *recorded*
  handoff, so they go **after 5**, and only when `Frontmatter.Done()` is already true.

Step 8 is deliberately a refusal rather than a shrug: writing `status: done` and only then
discovering there is no entry to tick is exactly the half-applied outcome §2 exists to avoid.

### 4. The state body is **not** validated here — neither half

The prompt for this scenario presumed something must decide the replacement body is
well-formed before it lands. The specification already weighed that and decided otherwise, in
`## Phasing`:

> The five `finish` refusals (17–21) and `check` (22) land after the crossover, so for that
> window `brief` manages its own specification with caps unenforced at the write path and no
> backstop. […] The mitigation is git and a branch, per *Bootstrap hazards* below — not care.
> This was weighed against pulling 17 and 18 forward and the ordering was chosen deliberately.

So SCENARIO-05 writes the state body **verbatim**, whatever headings it does or does not carry,
and SCENARIO-19 owns **both** halves — the detection and the copy — arriving genuinely red.
Same for the caps (17/18). What this scenario does provide is the **insertion point**: a single
validation phase, ordered as in §3, where each later check is one guard and one refusal.

### 5. The handoff splice

The anchor is the bare `cfg.HandoffHeading` line with nothing under it — what
`scaffold.stepSkeleton` writes. Triage's binding constraint forbids parse-and-re-render: every
byte of the step file outside the spliced region must come out identical.

- **One scanner, in `internal/platform/markdown`.** Add
  `SectionRange(body, heading string) (start, end int, ok bool)` returning byte offsets of the
  section **body** — `start` is just past the anchor line's newline, `end` is the first byte of
  the terminating heading line, or `len(body)`. Reimplement the existing `Section` on top of it
  (`Section` = the trimmed `body[start:end]`), so the read side and the write side can never
  drift apart about where a section ends. `Section`'s existing tests must stay green unchanged.
- **Fence-awareness applies to the anchor search, not just the end scan.** This document itself
  is the proof: a plan file routinely contains a fenced ```` ```markdown ```` block with a bare
  `## Handoff` line inside it, and post-crossover `brief finish brief SCENARIO-05` on a naive
  first-match would splice into the example. Building on `markdown`'s scanner gives this for
  free — the developer must **not** hand-roll a `strings.Index` for the anchor.
- **Splice on the frontmatter-stripped body**, per SCENARIO-04's rule that a `#` in a YAML value
  is never a heading: `ParseFrontmatter` first, compute `frontLen = len(body) - len(rest)`,
  run `SectionRange` on `rest`, splice within `rest`, then concatenate `body[:frontLen]` back on.
- **Replacement text, exactly:** `"\n" + strings.Trim(handoff, "\n") + "\n"`, plus one further
  `"\n"` when `end < len(rest)` so a following heading keeps its blank-line separator. This is a
  **fixed point**: splicing a body that is already there reproduces the file byte-for-byte —
  which is what SCENARIO-06 and SCENARIO-16 will compare against, and Step 12 pins it. It holds
  for a handoff body whose lines are not unfenced `##`-or-higher headings; the profile's three
  labelled groups are `**bold**` labels, not headings, so the shape that ships is inside the
  guarantee. A body carrying an unfenced `##` heading is not — see **Traps**.
- **The caller supplies the body only, never the heading.** The heading text is configuration
  (R2); a caller pasting `## Handoff` under a repo that renamed the heading would get two
  headings. A handoff file that begins with the configured heading is written verbatim, produces
  a duplicated heading, and is **not** refused here — a trap, and an open debt owned by
  SCENARIO-19's family.

### 6. Marking done — frontmatter is written textually, never re-encoded

`status:` in the frontmatter is the sole doneness authority (STATE.md); the progress checkbox is
a projection. `finish` writes both, step file before specification (§2).

- **`stepfile.SetStatus(body []byte, status string) ([]byte, error)`** (new, plus
  `ErrNoStatusField`): finds the first `status:` line **inside** the frontmatter delimiters and
  replaces that line, preserving every other byte. It must **not** decode-and-re-marshal:
  `Frontmatter` has no `KnownFields`, so a round trip silently drops any key it does not know,
  and reorders the ones it does. A step file with no `status:` key is refused rather than having
  one inserted — where to insert a key is a guess, and naming the problem is not.
- **`tickProgressEntry(body, heading, id string) (string, error)`** (new, in
  `scaffold/render.go` beside `insertProgressEntry`, plus `ErrNoProgressEntry`): within the
  progress section it flips the first matching item's `[ ]` to `[x]`, changing nothing else on
  the line. An item already `[x]` is left byte-identical. **Match rule:** the item text after
  `- [ ] `/`- [x] ` begins with `id`, followed by end-of-line or a rune that is not
  `[A-Za-z0-9_-]` — so `- [ ] STEP-10` is not matched when finishing `STEP-1`, while
  `- [ ] STEP-02: Assemble the thing` is (STATE.md: a reader matching by id tolerates a
  human-added `: <title>`).
- It uses `insertProgressEntry`'s section scan (heading equality after right-trim; section ends
  at the next line starting with `#`; **not** fence-aware) **deliberately**: `new step` and
  `finish` must agree about where the progress section ends in the same file. The handoff splice
  uses the fence-aware scanner for the opposite reason — its content is caller-supplied prose
  that routinely contains fenced `#` lines. Two rules, each with a reason; STATE.md's standing
  trap against unifying them still holds.

### 7. The command surface

`brief finish <feature> <step> --handoff <path> --state <path>`, both flags required, `-`
meaning stdin and permitted on **at most one** — fixed by `## Product Verdict` 1 and the command
table, not re-opened here.

- `cli.Run` gains a `stdin io.Reader` parameter (`Run(ctx, wd, args, stdin, stdout, stderr)`).
  `cmd/brief/main.go` passes `os.Stdin`; tests pass a `bytes.Buffer`. Every existing call site is
  updated mechanically in Step 1, before any behavior change.
- `internal/cli` reads the two inputs into `[]byte` and passes them down. The feature package
  never opens a caller-supplied path: `Finish` takes bytes.
- **`--help` copy is load-bearing.** `## Product Verdict` records that the word **COMPLETE** in
  the `--state` help is the one place R8's semantics reach the caller at the moment they matter.
  Pinned: `--state <path>` reads *"the COMPLETE replacement body for the state file; it replaces
  the file, it is never appended to"*. Step 17 asserts `COMPLETE` appears in the help output.
- **stdout stays empty on success**, reserved for R9's findings so that a script can treat any
  stdout line from `finish` as a finding (R14a: findings never appear on a failed run). The
  confirmation is one line on stderr, exit 0 — R14's established "empty stdout, one line on
  stderr" shape. The copy describes **state, not action** (`brief finish: STEP-02 is done`), so
  SCENARIO-06's no-op path can print the same line without lying.

### 8. Deliberately not built

R9's dropped-entry accounting; the caps (17/18); the state-body heading check (19); the open
checklist refusal (20); the unfinished dependency refusal (21); identical re-finish as a no-op
(06); differing re-finish refused (16); `--json`; `status`; `check`; the R6 synthesis. What
`finish` does **today** when 17–21's preconditions are violated: it proceeds and writes. That is
this scenario's honest starting point, and it is what makes those five arrive red.

## User-visible contract

```
brief finish <feature> <step> --handoff <path> --state <path>
```

| Case | stdout | stderr | exit |
| --- | --- | --- | --- |
| happy path | empty | `brief finish: <step> is done` | 0 |
| `--handoff -` (body on stdin) | empty | same | 0 |
| no feature / no step given | empty | `brief finish: no feature given; run 'brief finish <feature> <step> --handoff <path> --state <path>'` | 2 |
| too many positional arguments | empty | `brief finish: too many arguments; run '…'` | 2 |
| `--handoff` omitted | empty | `brief finish: --handoff is required; run '…'` | 2 |
| `--state` omitted | empty | `brief finish: --state is required; run '…'` | 2 |
| `-` given for both flags | empty | `brief finish: - may be given for at most one of --handoff and --state` | 2 |
| unknown flag | empty | `brief finish: flag provided but not defined: -bogus; run '…'` | 2 |
| `--help` | the usage text, containing `COMPLETE` | empty | 0 |
| `--handoff` path unreadable | empty | one flattened line naming the path, no `(no files changed)` tail | 1 |
| unknown feature / unknown step / missing anchor / no progress entry | empty | R14a refusal, `… (no files changed)` | 1 |
| a write fails | empty | one flattened line naming the file, **no** `(no files changed)` tail | 1 |

An unreadable input path is exit **1**, not 2: the invocation was well-formed, the file was not
there — the same classification `start` gives an unknown feature. Flagged as a choice; the
specification is silent.

## Fixture

`newFinishFixture(t)` under `t.TempDir()`, reusing `scaffold_test.go`'s existing
`fixtureConfig()` (every read field already differs from `config.Default()`). Feature `widgets`:

- **`STEP-01.md`** — `status: done`; handoff body carrying `OLD-HANDOFF-01`. Exists so the
  target's `depends-on` is satisfied.
- **`STEP-02.md` — the target.** Frontmatter written byte-for-byte in `stepSkeleton`'s shape
  plus **one key `Frontmatter` does not know** (`owner: planner`) and `depends-on: [STEP-01]`.
  Body: `# STEP-02 Assemble the thing`; `## Fixture Checklist` with two `- [x]` items **and a
  fenced block containing a decoy `## Fixture Handoff` line**; then the real `## Fixture
  Handoff`, bare.
  *Both properties are preconditions, not decoration:* the checklist must be fully ticked and
  the dependency must be done, or SCENARIO-20 and SCENARIO-21 will retroactively redden this
  scenario's happy path.
- **`STEP-03.md`** — `status: open`, one `- [ ]` checklist item. Must be untouched.
- **`NOTES.md`** — the four fixture state headings, each body carrying `OLD-STATE-ENTRY`.
- **`SPEC.md`** — title, `## Progress`, three entries in this order:
  `- [x] STEP-01: already done`, `- [ ] STEP-02: Assemble the thing`, `- [ ] STEP-03`.

Inputs: `newHandoff` carries `NEW-HANDOFF-02` and a fenced block containing a `#` line;
`newState` carries the four headings with `NEW-STATE-ENTRY`. Old and new markers are distinct,
so no "unchanged" assertion can pass vacuously.

Second, smaller fixture for the tick boundary: `StepFilePattern "STEP-%d.md"`, steps `STEP-1`
(target) and `STEP-10`, progress entries in the order `- [ ] STEP-10` then `- [ ] STEP-1`, so a
plain prefix match ticks the wrong line.

## Implementation Plan

- [x] Step 1: `internal/cli/cli.go`, `cmd/brief/main.go`, all four `internal/cli/*_test.go` — add `stdin io.Reader` to `cli.Run` and update every call site; no behavior change, suite stays green (update)
- [x] Step 2: `internal/platform/markdown/section_test.go` `Test_SectionRange_…` — offsets of an empty section, of a section followed by a same-level heading, of the last section in a file; a fenced `## X` is not the anchor; unknown heading → `ok == false` (red)
- [x] Step 3: `internal/platform/markdown/section.go` `SectionRange`; reimplement `Section` on top of it; extend `doc.go` (green — existing `Section` tests must stay green untouched)
- [x] Step 4: `internal/platform/stepfile/frontmatter_test.go` `Test_SetStatus_…` — replaces the `status:` line, leaves every other byte including an unknown key identical; a `status:` line after the closing `---` is not touched; no `status:` key → `ErrNoStatusField` (red)
- [x] Step 5: `internal/platform/stepfile/frontmatter.go` `SetStatus`, `ErrNoStatusField`; extend `doc.go` (green)
- [x] Step 6: `internal/scaffold/finish_test.go` — `newFinishFixture(t)` per **## Fixture**, plus `Test_writes_the_supplied_handoff_under_the_step_s_handoff_anchor` — **section-scoped, not `Contains`**: `markdown.Section(rest, cfg.HandoffHeading)` equals the supplied body, and the checklist section holding the fenced decoy is unchanged. A bare `Contains(NEW-HANDOFF-02)` survives mutation 23(a) and proves nothing (red)
- [x] Step 7: `internal/scaffold/errors.go` — `ErrNoSuchStep`, `ErrNoProgressEntry`; `internal/scaffold/render.go` — `spliceHandoff`; `internal/scaffold/finish.go` — `(*Server).Finish(ctx, feature, step string, handoff, state []byte) error` with §3's validation order and §2's write order (green)
- [x] Step 8: `finish_test.go` `Test_leaves_the_rest_of_the_step_file_byte_identical` — the file equals the original with exactly the handoff region replaced, computed by the test from the original bytes (green on arrival)
- [x] Step 9: `finish_test.go` `Test_replaces_the_state_file_with_exactly_the_supplied_body` — equals `newState`; `OLD-STATE-ENTRY` absent (proves no concatenation) (green on arrival)
- [x] Step 10: `finish_test.go` `Test_marks_the_step_done_in_its_frontmatter` — `ParseFrontmatter(...).Done()` is true (green on arrival)
- [x] Step 11: `finish_test.go` `Test_leaves_the_frontmatter_byte_identical_apart_from_the_status_line` — `owner: planner` and `depends-on: [STEP-01]` survive in place; mutation target for Step 22(b) (green on arrival)
- [x] Step 12: `finish_test.go` `Test_finishing_with_a_handoff_already_in_place_reproduces_the_file_byte_for_byte` — the splice is a fixed point, which SCENARIO-06/16 compare against. Not SCENARIO-06: nothing here asserts the write was skipped or mtime preserved (new)
- [x] Step 13: `finish_test.go` `Test_ticks_the_progress_entry_for_the_finished_step` and `Test_leaves_every_other_progress_entry_unchanged` — `STEP-02`'s entry becomes `- [x]` with its `: Assemble the thing` suffix intact; `STEP-01` stays `[x]`, `STEP-03` stays `[ ]` (green on arrival)
- [x] Step 14: `finish_test.go` `Test_does_not_tick_an_entry_whose_id_merely_starts_with_the_finished_id` — the `STEP-%d.md` fixture: finishing `STEP-1` leaves `- [ ] STEP-10` open (green on arrival — the boundary rule was built into `tickProgressEntry` from Step 7 rather than added after a naive prefix match; Step 23(c) mutation-verifies it is load-bearing)
- [x] Step 15: `finish_test.go` `Test_finish_leaves_no_temp_file_in_the_feature_directory` (renamed from the plan's name — `step_test.go` already declares `Test_leaves_no_temp_file_in_the_feature_directory` for `NewStep`) — directory listing is exactly the five fixture files; and `Test_the_temp_file_sweep_sees_a_temp_file` — **control arm**: create `.NOTES.md.brief-tmp` in the same directory and assert the same sweep fails it (new)
- [x] Step 16: `finish_test.go` — refusal matrix, each asserting the sentinel via `require.ErrorIs` **and** a full-tree byte-identity sweep: unknown feature (`ErrNoSuchFeature`), unknown step (`ErrNoSuchStep`), step file with no handoff anchor (`ErrMalformedFeature`), specification with no progress heading (`ErrNoProgressHeading`), progress list with no entry for the id (`ErrNoProgressEntry`), missing state file (`ErrMalformedFeature`), feature name escaping the feature root (`ErrNoSuchFeature`) — all green on arrival: Step 7 built the full validation pipeline in one pass rather than guard-by-guard, so every case here exercises code that already existed
- [x] Step 17: `internal/cli/finish_test.go` — command slice through `cli.Run`: happy path (exit 0, empty stdout, one stderr line, files changed on disk); `--handoff -` reads stdin; usage matrix from **## User-visible contract** (no feature, no step, too many args, each flag omitted, `-` twice, unknown flag), each exit 2 with empty stdout; `--help` prints usage containing `COMPLETE`; unreadable `--handoff` path → exit 1; unknown feature → exit 1 with the `(no files changed)` tail (red)
- [x] Step 18: `internal/cli/finish.go` — `finishUsage`, `runFinish`: flag parsing, the at-most-one-`-` check, `readSource(path, stdin)`, `config.Resolve(wd)`, `scaffold.NewServer(cfg, root).Finish`, errors through `renderRefusal(stderr, "finish", err)`, the stderr confirmation line. Also `splitLeadingPositionals`, not in the plan: `flag.FlagSet.Parse` stops at the first non-flag token, and this command's contract puts `<feature> <step>` before every flag — undiscovered until Step 17's happy-path test read "too many arguments" instead of running (green, plus one unplanned fix)
- [x] Step 19: `internal/cli/cli.go` — dispatch `case "finish"`; add the `brief finish` line to the `usage` const; update **both** `expected one of: new, start` strings to `new, start, finish` (update)
- [x] Step 20: `internal/cli/run_test.go`, `internal/cli/exit_test.go` — update the three pinned copies of `expected one of: new, start` (update)
- [x] Step 21: `finish_test.go` `Test_returns_an_error_and_changes_nothing_when_the_feature_directory_is_not_writable` — `os.Chmod(featureDir, 0o555)` with a `t.Cleanup` restoring `0o755` registered after the directory exists; assert the error is returned, is **not** a `*RefusalError`, and the full-tree sweep is byte-identical. Not running as root, so no skip needed; the test genuinely exercises the failure (new)
- [x] Step 22: `internal/scaffold/doc.go` — rewrite the package doc per §1 (write path: create a feature, create a step, close a step; `assemble` owns the read path); document `Finish`'s validation order and write order as contracts on the method. Also stripped `SCENARIO-XX` narrative citations from `Finish`'s doc comment and from a pre-existing one in `render.go`'s `stateSkeleton` — the clean-architecture skill names `SCENARIO-04`-style ids as exactly the forbidden narrative-citation shape (update, plus one unplanned cleanup)
- [x] Step 23: **mutation verification**, one at a time, restored from a `$TMPDIR` copy rather than `git stash` (the working tree already carries this whole scenario's uncommitted diff, so `git stash push -- <file>` on it stashes the legitimate implementation along with the mutation — hit this on mutation (a), recovered via `git stash pop` after `git checkout --`, switched to `cp`-backup for the remaining four), each diffed byte-identical after restore:
  - (a) delete fence tracking from `SectionRange`'s **anchor** scan → `Test_writes_the_supplied_handoff_under_the_step_s_handoff_anchor` reddens: the checklist section's fenced decoy `## Fixture Handoff` becomes the anchor and the real splice lands there instead
  - (b) make `SetStatus` decode-and-re-marshal `Frontmatter` → `Test_leaves_the_frontmatter_byte_identical_apart_from_the_status_line` reddens: `owner: planner` and `depends-on: [STEP-01]`'s literal form both vanish
  - (c) replace `tickProgressEntry`'s boundary rule (`progressItemNames`) with a plain prefix match → `Test_does_not_tick_an_entry_whose_id_merely_starts_with_the_finished_id` reddens: finishing `STEP-1` ticks `STEP-10` instead
  - (d) make `Finish` append the state body instead of replacing it → `Test_replaces_the_state_file_with_exactly_the_supplied_body` reddens on `OLD-STATE-ENTRY` reappearing
  - (e) drop the progress-entry guard in `tickProgressEntry` → `Test_refuses_a_progress_list_with_no_entry_for_the_step` reddens (falls through to the state-file-missing refusal instead, since that fixture also omits `NOTES.md`)
- [x] Step 24: `go build ./...` clean; `go test -count=1 ./...` green; `go test -race` clean on all four touched packages; `golangci-lint run ./...` — 0 issues after fixing 9 findings this step surfaced (dupword from the new usage line abutting "…next step's brief" with "brief finish…"; nonamedreturns on `SectionRange` and `splitLeadingPositionals`; a `strings.Cut` simplification; two test-file string-concat-in-loop; `assert.NotErrorAs`; two unwrapped `wrapcheck` errors in `readSource`); `go doc ./internal/scaffold` and `go doc ./internal/scaffold Server.Finish` read as contracts, no `SCENARIO-XX` narrative left. A post-green `advisor()` review then found the fixed-point splice was only proven for "no heading follows the anchor" — every fixture put the handoff last, so `spliceHandoff`'s `end < len(body)` branch (the extra blank-line separator before a following section) had no test; added
  `Test_finishing_with_a_trailing_section_after_the_handoff_reproduces_the_file_byte_for_byte`,
  confirmed via `go test -coverprofile` that the branch was the gap and is now hit. Same review
  also found §2's plan prose ("names the file that failed and says to re-run the same command")
  did not match the write-failure message (file named, no re-run instruction); added
  `writeFailure` in `finish.go` to say so, and an assertion on it in Step 21's test. Final count:
  **170 tests, delta +44 from the verified 126 baseline**, 0 skips
- [x] Step 25: mark SCENARIO-05 done in `specification.md`'s `## BDD Acceptance Progress`, and rewrite `docs/specifications/brief/STATE.md` folding in the Handoff below
