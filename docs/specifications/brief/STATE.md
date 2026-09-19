# brief — current state

Scenarios complete: SCENARIO-01..06, amended by SCENARIO-HANDOFF-FILE (touches 03/05/06): the
handoff moved out of the step file into its own file (R21). Every splice-era entry is gone,
replaced by the whole-file contract below. The crossover reads this file next.

## Binding decisions

- Config: `.brief.yaml`, upward walk, nearest wins, missing anywhere = `config.Default()`.
  `Resolve(startDir) (Config, string, error)`, `errors.Is`-comparable `ErrInvalidConfig`. (01-04)
- `stepfile.Frontmatter.Done()` is the sole doneness authority. `SetStatus` edits `status:`
  **textually**, preserving CRLF, and is now the only in-file step-file write — R21's second
  clause (preserve everything outside the edited span) covers it and `tickProgressEntry` alone.
  No `status:` key → `ErrNoStatusField`. (03-05, HANDOFF-FILE)
- **The handoff lives in its own file, never spliced into the step file (R21).** Named
  `stepPattern.ID(n) + cfg.HandoffFileSuffix`
  (`stepfile.CompileHandoff(step, suffix, stateFile, specificationFile)`), default suffix
  `-HANDOFF.md`. Refuses a digit in the suffix; refuses a step Pattern whose `ID` does not vary
  with the step number (`.%d` — `filepath.Ext` eats the whole rendered name at every step,
  aliasing every step's handoff onto one file); and refuses **case-folded rendered-name**
  collisions, at several representative step numbers, against the step's own name,
  `cfg.StateFile`, or `cfg.SpecificationFile` — comparing rendered names rather than suffix
  strings is load-bearing because a multi-dot step pattern (`SCENARIO-%02d.step.md`) makes
  `Pattern.ID`'s stripped extension narrower than the pattern's whole literal suffix; case
  folding is load-bearing because this repo's dev platform (macOS/APFS) is case-insensitive.
  (HANDOFF-FILE, fix)
- `markdown.Section` is the one exported section reader, fence-aware both ends via a **state
  machine** (CommonMark §4.5: same character, ≥ opener length, no info string, ≤3 leading
  spaces; `findHeading` is first-occurrence-wins). `UnterminatedFence` shares that machine and
  is the sole fence-open detector, validating the state argument and refusing an on-disk state
  file a terminator scan could not read past. No exported way remains to recover a section's
  byte offsets — offsets served splicing only, and every write here is whole-file. (04-06,
  HANDOFF-FILE)
- `finish`'s check order: step-file pattern → handoff-file-suffix compiles → feature dir → step
  found → frontmatter parses → state's fence → spec readable → progress entry → state file
  present → identity → write. Its four **writes** land handoff file → state file → step file →
  specification: every prefix short of the last write leaves `status:` "open", so a crash-retry
  takes the full path again and converges. `status: done` must never land before the handoff
  file exists, or SCENARIO-16's (unbuilt) divergence refusal would refuse the retry that repairs
  the crash. **Test-verified at all four positions**: a directory planted at each write's own
  temp sibling (or, for the handoff, at its final name) forces that write to fail without
  disturbing any other file, and each test also clears the obstruction and retries, proving
  actual convergence rather than just the open-status precondition
  (`Test_reports_a_handoff_write_that_cannot_be_committed`,
  `_a_state_write_…`, `_a_step_file_write_…`, `_a_specification_write_…`). (HANDOFF-FILE, fix)
- **Identity/no-op (R11):** re-finishing a done step with identical inputs writes nothing, mtime
  preserved on all four files. Gate: `fm.Done() && handoff file exists and matches && state
  matches && ticked spec matches`, taken **before `SetStatus`**. Absent/unreadable handoff is
  not identical — converges a crash-after-state retry and a migrated (hand-marked-done) step
  alike. **No step-body byte comparison** — one would make the frontmatter-gate test stop
  discriminating `fm.Done()`. SCENARIO-16 inverts this into a refusal over the same conjuncts.
  (06, HANDOFF-FILE)
- `RefusalError` carries `Line int` (0 = whole-path refusal); `cli/refusal.go` renders
  `<path>:<line>`. `scaffold.StateSource` is the one remaining placeholder `Path`, for a refusal
  about the state argument's own bytes — `cli/finish.go` upgrades it to the real `--state`
  source (or `"<stdin>"`). The handoff argument is never fence-checked; nothing reads it
  structurally. (fix ×3, HANDOFF-FILE)
- A `## Handoff` section left in a step file by an unmigrated tree is **ignored** by `start` and
  `finish`, never refused — R7 keeps prose judgment out of the tool; refusing would lock `brief`
  out of its own tree mid-migration. (HANDOFF-FILE)
- `cli.Run` takes `stdin` before `stdout`. `finish` prints nothing to stdout, one stderr line
  `brief finish: <step> is done`. (05-06)

## Left unbuilt

- Differing-inputs refusal (16, today overwrites and reports success; git + branch mitigates),
  caps (17/18), state-body required-heading check (19, deferred), open-checklist refusal (20),
  unfinished-`depends-on` refusal (21).
- `assemble.Server.Status`/`.Next`/`.Show`/`.Handoff`/`.StateGet`, `--json` (15), complete-feature
  stderr+exit-0 (12), unknown-feature enumeration (09), `markdown.Headings` + checklist parser
  (20/22), R13 output truncation, R9's diff/finding output, `FinishResult`.
- `scaffold.HandoffSource` and `cli/finish.go`'s `--handoff` source upgrade — deleted with the
  handoff fence refusal; SCENARIO-17's over-cap handoff refusal re-adds both.
- `HandoffPattern.Number` (recognizing a handoff filename by pattern) — not built; `check` (22)
  and the R6 synthesis are the first callers needing to enumerate handoff files.

## Traps

- `scaffold` is named for `new feature`/`new step` while also owning `finish` — unowned rename
  debt; `RefusalError`'s eventual platform move (SCENARIO-13) touches every site here.
- `tickProgressEntry` mutates a whole line on `[ ]`→`[x]`: an already-ticked title containing
  `[ ]` gets mutated too. Pre-existing, unowned.
- `root = wd` unless `.brief.yaml` found — avoid `t.Chdir`/`os.Getwd` in tests; gosec's `G703`
  flags a straight-line read→transform→write of one fixed path as traversal — route through a
  helper function-call boundary.
- The `-HANDOFF.md` suffix sorts *before* its step file (`-` < `.`). Cosmetic; do not change the
  default without migrating the tree again.
- `atomicfile.Create`'s temp-sibling mode carries an owner-write exception: when name exists and
  its own mode has no owner-write bit, an abandoned write's sibling inherits that mode too, and
  every later attempt for the same name fails `permission denied` until the sibling is removed by
  hand. Pre-existing, unowned; do not add an open-EACCES-remove-retry without its own scenario.
- `atomicfile.Create`'s `Mode().Perm()` masks setuid/setgid/sticky, so a replace silently drops
  them. `Lstat`→`Chmod` is an inherent read-then-write window. `PendingFile` is not documented or
  guarded as safe for concurrent `Close`. All three pre-existing, no constructible failure —
  `brief` has one writer — unowned.

## Open debts

- Heading/cap value validation — unowned until SCENARIO-19.
- A mid-write I/O failure leaves a half-applied result on disk that a same-argument retry
  repairs (test-verified at all four `finish` write positions, see Binding decisions); nothing
  yet detects a half-applied result proactively. Unowned; `check` (22) is the detector.
- Invalid-`step-file-pattern` refusal names the feature dir, not the config — unowned.
- `insertProgressEntry`/`tickProgressEntry` insert an LF line into a CRLF file (mixed endings);
  `ParseFrontmatter`'s `rest` keeps a leading `"\r"` after a CRLF close when a blank line follows;
  `brief start` on a CRLF feature emits mixed line endings. All three unowned MINOR.
- `SetStatus` returns `ErrNoStatusField` for a missing frontmatter *delimiter* too, not only a
  missing key. Unowned MINOR.
- `assemble`'s sentinels duplicate `scaffold`'s (e.g. `ErrNoSuchFeature`, never a
  `*RefusalError`) — SCENARIO-13 closes this.
- `check` (22) must report a `## Handoff` section surviving in a step file as a finding; until
  then the leftover is silently ignored. Unowned — dies unless re-opened.
- **Data loss:** a symlinked specification or step file is silently replaced by a regular file.
  `finish` reads through the symlink (`root.ReadFile` follows in-root symlinks), computes the
  tick against the link's target content, then `replaceBytes`'s rename destroys the link and the
  real target file never receives the tick. The state file already refuses a non-regular target
  (`finish.go`'s `IsRegular()` check before its read); the specification and step-file reads carry
  no such refusal. Pre-existing, real, unowned — needs its own scenario (a refusal, not a silent
  fix) — dies unless re-opened.

## Crossover note

An identical re-finish being a true no-op means the crossover's "mark SCENARIO-01 through 06
done" step is safely re-runnable. `finish` reads the state file on every call now; once the
crossover lands, `STATE.md` (this file) is that state file. `SCENARIO-01.md`…`-06.md` and their
new `SCENARIO-0N-HANDOFF.md` files still carry no frontmatter — the crossover owns adding it.
