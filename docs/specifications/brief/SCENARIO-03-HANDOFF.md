**Binding decisions** — a later scenario must not contradict these without saying so:

- **`status:` in step frontmatter is the sole authoritative doneness field.** The progress-list
  checkbox is a projection `finish` keeps in sync; the handoff body is the audit trail.
  SCENARIO-21's dependency check reads frontmatter `status` only — never the checkbox, never
  handoff content. Three signals exist and only one is read, or they drift.
- **A step file matches the pattern only by strict round trip** — `Sprintf(pattern, n) ==
  filename`. `SCENARIO-7.md` under `%02d` is not a step file. Loosening this creates step files
  that the scanner sees but no id or `depends-on` can address; report a near-miss as a `check`
  finding (SCENARIO-22) instead.
- **The next step number is `max(existing step numbers) + 1`**, scanned from files, never from
  the progress list. It is what keeps ids stable across a deleted step, which SCENARIO-05's
  progress marking and SCENARIO-21's `depends-on` both rely on.
- **Step id = rendered filename minus its extension**, so the id is derived from
  `step-file-pattern`; changing that key mid-feature orphans every id and `depends-on` edge.
- **The handoff anchor is the bare `cfg.HandoffHeading` line with nothing under it.**
  SCENARIO-13's trichotomy: *absent* = no line equals the heading; *present-and-empty* = the line
  exists with no non-blank line before EOF-or-the-next-heading; *present-and-filled* = otherwise.
- **The atomic-write primitive moved here from SCENARIO-05** (`## Phasing` authorises it).
  `atomicfile.WriteFile(root *os.Root, name, data, perm)` — deterministic `.<name>.brief-tmp`
  sibling, `O_TRUNC` not `O_EXCL`, target's mode preserved on replace, temp removed on failure,
  **no fsync**. SCENARIO-05 uses it; it does not reinvent one.
- **`config.Config` gained `ChecklistHeading`** (default `"## Implementation Plan"`). Every
  heading the scaffold writes comes from config; none is a constant in `scaffold`.
- **Validation order in `NewStep`, because R14a names the first thing wrong:** config → pattern
  compiles → feature directory opens → specification reads → progress heading found → then write.
  **Write order: step file first, then the specification** — reversing it lets a re-run append a
  duplicate progress entry for the same N.
- **`scaffold.RefusalError{Path, Problem, Fix, Err}`** renders `<path>: <problem>; <fix>`;
  `cli/refusal.go` adds the `brief <cmd>: ` prefix and the ` (no files changed)` tail, so the
  R14a template stays in `cli`, in one place.

**Left unbuilt** — named so nobody assumes it exists:

- `RefusalError.Line` — R14a's optional `:<line>`; SCENARIO-20 needs it first.
- Enumerating known features in the unknown-feature refusal (R14) — SCENARIO-09 builds the
  enumerator.
- `scaffold.ErrFeatureExists` and the `new feature` duplicate refusal — still SCENARIO-08.
- Feature-name validation — still SCENARIO-07.
- Reading a step file back: no frontmatter parser, no checklist parser, no handoff-block reader.
  `stepfile` names files; it does not open them. SCENARIO-04 and SCENARIO-05 own those.
- `config` value validation (headings, caps, and now `checklist-heading`) — still unowned.

**Traps** — things that look right and are not:

- **`atomicfile.WriteFile` replaces a `0o400` target successfully**, because rename swaps a
  directory entry and ignores the target's mode. `chmod -w` does not protect a file from `brief`.
  Useful as a one-off check that the implementation really is rename-based; do not pin it as a
  test, nobody chose it as a contract.
- **`writeExclusive`'s `O_EXCL` on the step file is unreachable by construction** — max+1 plus
  the strict round trip guarantee the name is free. Same shape as SCENARIO-02's trap: keep it,
  do not write a test that cannot reach it.
- **`featureRoot.OpenRoot(feature)` reports a traversing name as not-existing**, so
  `brief new step ../escaped` refuses as `ErrNoSuchFeature`, not as a traversal error. Nothing is
  created either way; SCENARIO-07 may reclassify it.
- **`insertProgressEntry` must strip and re-append the trailing newline around the split**, or the
  final `""` element is consumed as a real blank line and the file loses its trailing newline.
- **This repository has no `.brief.yaml`**, so `root = wd`: `brief new step brief` works from the
  repo root and refuses from any subdirectory. Inherited from SCENARIO-02.
- The new `ChecklistHeading`, `HandoffHeading` and `StepFilePattern` fixture values must all
  differ from `config.Default()`, or every threading assertion is vacuous.

**Open debts** (fold into STATE.md):

- A specification-write failure after the step file lands leaves an orphan step file with no
  progress entry. No rollback; `check` (SCENARIO-22) is the natural detector. Unowned.
- The invalid-`step-file-pattern` refusal names the feature directory, not the `.brief.yaml` that
  carries the bad value — `scaffold` is not given the config source path. Unowned.
- `insertProgressEntry` inserts an LF-terminated line into a CRLF file. Mixed line endings.
  Unowned — dies unless re-opened.
