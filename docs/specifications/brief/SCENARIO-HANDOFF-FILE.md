# SCENARIO-HANDOFF-FILE: the handoff moves out of the step file into its own file

Not a new scenario. This plan amends three shipped ones — SCENARIO-03, SCENARIO-05 and
SCENARIO-06 — to `## Decisions taken` item 0 and **R21**. The decision and its history are
settled; nothing here re-litigates either.

## Scenario

The three amended acceptance lines, quoted from `specification.md`:

```gherkin
  Scenario: SCENARIO-03 New step scaffolds the next step file and its progress entry
    And no handoff file exists yet, because only finish writes one

  Scenario: SCENARIO-05 Finishing writes the handoff and replaces the state, then marks done
    Then the step's handoff file holds exactly what I supplied
    And the step file itself is byte-identical apart from its status field

  Scenario: SCENARIO-06 Finishing a finished step with the same inputs changes nothing
    And all three files are byte-identical, mtime included, because the write was skipped
```

## Decisions

Six were asked for. Each is stated with the constraint that forced it.

### 1. `handoff-file-suffix`, derived from the step filename — not a second `fmt` pattern

`config.HandoffHeading` is replaced by `HandoffFileSuffix` (`yaml:"handoff-file-suffix"`),
default **`-HANDOFF.md`**. The handoff filename is `stepPattern.ID(n) + suffix`, so the
default profile names `SCENARIO-01.md`'s handoff `SCENARIO-01-HANDOFF.md`.

An independent `handoff-file-pattern` is rejected because it admits
`handoff-file-pattern == step-file-pattern`, in which case `finish` writes the handoff body
over the step file. That is data loss from one config typo, in the exact class R21 exists to
prevent. Derivation makes it unrepresentable: the two names cannot coincide unless the suffix
is exactly the step file's extension, which is one comparison to refuse. It also cannot drift
in number formatting (`SCENARIO-01.md` beside `HANDOFF-1.md`).

Derivation inherits `stepfile.Compile`'s round-trip discipline through a new
`stepfile.CompileHandoff(step Pattern, suffix string) (HandoffPattern, error)`, refusing with
`stepfile.ErrInvalidHandoffSuffix` a suffix that is empty, contains a path separator, contains
`%`, **contains a digit**, or equals the step filename's extension. The digit rule is
load-bearing and not decorative: suffix `1.md` against `STEP-%d.md` renders `STEP-1` + `1.md`
= `STEP-11.md`, which `Pattern.Number` recognizes as step 11 — a handoff file that a
directory scan would read as a step. With no digit in the suffix and the extension case
refused, the rendered handoff name can never round-trip through `Pattern.Number`.

### 2. Naming in `internal/platform/stepfile`; the read stays in `scaffold`

`stepfile` already owns per-step file naming both directions and owns `Pattern.ID`, which the
derivation needs. A new platform package would import `stepfile` and re-export half of it.
Its package doc gains one sentence saying it owns the handoff file's name too, or the doc
becomes false.

The *read* stays where the caller is. Today there is exactly one production reader — the
identity comparison inside `Finish` — and its read is `*os.Root`-scoped to the feature
directory, which `brief handoff`, `check` and the R6 synthesis will not be. A `ReadHandoff`
helper with one caller is speculative. What matters is that no package derives the filename
itself, and after this plan none does.

### 3. `finish`'s write order: handoff → state → step status → progress

Re-derived from scratch for four writes. The invariant is that frontmatter `status:` is the
sole doneness authority, so **every prefix of the sequence that leaves `status: open` is a
state a retry converges from**: the full path runs again and rewrites each earlier write with
a byte-identical body.

- Crash after **handoff**: an orphan handoff file beside an open step. `start` still inherits
  the old state, which is correct for a step that is not done. Converges.
- Crash after **state**: state replaced, step still open. `start` on that same step now
  inherits the decisions of the step about to be worked — misleading, which is why state goes
  second rather than first. Converges.
- Crash after **status**: the step reads done and every record it certifies is present.
  Converges via the spec conjunct of identity (below).
- Crash after **progress**: complete.

The decisive ordering constraint is that **`status: done` must not land before the handoff
file exists.** A reader that sees done with no handoff has lost the record; worse, SCENARIO-16
(unbuilt) turns a divergence between the supplied and the recorded handoff into a refusal, and
an absent recorded handoff read as divergence would refuse the very retry that repairs the
crash. Ordering so that no prefix reports done without its record removes that hazard instead
of relying on SCENARIO-16 special-casing absence.

Secondary reasons the handoff goes first: it is the only **create** among four writes (the
other three replace or edit), and the least destructive write belongs at the front; and the
progress checkbox goes last because it is a pure projection of `status:`, making a stale
checkbox the mildest of the four inconsistencies.

The order is documented on `Finish` and is **not** test-verified: there is no seam that makes
a write fail after validation without the earlier validation refusing first (`atomicfile`
renames over a read-only file successfully, and a non-regular state file is refused up
front). The one observable consequence — that a crash prefix leaves the step open and a retry
finishes it — is verified by
`Test_a_step_whose_frontmatter_is_still_open_is_marked_done_even_when_every_input_matches_what_is_on_disk`.
STATE.md's existing open debt ("a mid-write I/O failure leaves a half-applied result; `check`
is the detector") already owns the rest.

### 4. Identity: four conjuncts, one of which may not exist

```
fm.Done()                                   (the step-file conjunct)
&& handoff file exists and its bytes == handoff
&& on-disk state bytes == state
&& tickProgressEntry(spec) == spec
```

An absent handoff file is **not** identical, so the write proceeds. That single rule serves
three cases: the first finish, the crash-after-state retry, and a step hand-marked done
before this change (the migration).

The step-file conjunct is `fm.Done()` and **deliberately not** a byte comparison of
`SetStatus(body,"done")` against `body`. With the splice gone the step-file write body is a
pure function of the on-disk body, so a byte comparison would hold in almost exactly the cases
`fm.Done()` holds — and where the two differ (a `status:done` line SetStatus would respace)
`fm.Done()` is the *correct* predicate, because the doneness authority is the parsed value,
not the byte shape, and R11 requires mtime preserved. Keeping the byte comparison out is also
what preserves
`Test_a_step_whose_frontmatter_is_still_open_is_marked_done_even_when_every_input_matches_what_is_on_disk`
as the single-variable proof of the gate: with it in, the reverted status line alone forces the
write and the test stops discriminating.

A genuine no-op writes nothing at all, so all four files keep their mtime.

### 5. Migration of `brief`'s own tree: move the content, delete the section, `check` reports

`docs/specifications/brief/SCENARIO-01.md` … `-06.md` each carry a `## Handoff` section.
Each moves to `SCENARIO-0N-HANDOFF.md` **body only, no title line** — `stateSkeleton`'s
precedent, for its reason: `finish` replaces the whole body, so a title would be dropped by
the first re-finish. The section is then deleted from the step file, because leaving it
creates two records of the same thing that can drift, and the whole decision is that the step
file no longer holds the handoff. Frontmatter is *not* added — the crossover is a separate
activity and is out of scope here.

**The migration is itself the vindication, and two of the six files prove it.** A "from the
`## Handoff` heading to EOF" extraction — the contract the deleted `spliceHandoff` shipped —
would corrupt two files:

- `SCENARIO-03.md` has a `## Handoff` line at **line 184 inside a fenced example block** and
  the real section at 359.
- `SCENARIO-06.md`'s handoff section is at 99 and is **followed by `## Crossover note` at
  159**, which is not part of the handoff and must stay in the step file.

A leftover `## Handoff` section in a step file is **ignored** by `start` and `finish` — never
refused. Refusing would lock `brief` out of its own tree mid-migration (the bootstrap hazard
this spec names), and R7 puts judgment about prose content outside the tool. It is
**reported** by `check` (SCENARIO-22, unbuilt) as a finding; the exact text is carried in the
Handoff below so a successor greps for it.

### 6. R21's second clause, verified rather than asserted

The two remaining in-file writes are `stepfile.SetStatus` (the frontmatter `status:` line) and
`tickProgressEntry` (one progress checkbox line). Both already edit by line index within a
`strings.Split`/`Join` of the whole file, so a mistaken index mutates one line rather than
deleting a range — R21's "duplicates or misplaces rather than deletes".

Verification is **exact byte equality of the whole file against the original with one
substring replaced**, which is strictly stronger than a line-count invariant and needs no
index arithmetic in the test. Both fixtures are strengthened so the assertion has something to
lose: the step fixture keeps its fenced `## Fixture Handoff` decoy, keeps a bare handoff
heading, and gains a trailing `## Notes` section; the specification fixture gains a section
after the progress list. Each assertion is then proved load-bearing by an individually applied
mutation (Steps 30–32).

## Where the amended spec is silent, and what this plan chose

1. **Whether the handoff body is normalized on write.** Chosen: **verbatim**, byte for byte,
   exactly as the state body already is — SCENARIO-05's "holds exactly what I supplied". The
   state path has produced zero defects across five rounds and trimming is what made the old
   identity comparison need two different normalizations.
2. **The handoff argument's unterminated-fence refusal.** Chosen: **deleted** with the splice.
   Its only stated reason was that an open fence breaks the next scan, and there is no next
   scan; nothing reads the handoff body structurally. A refusal with no constructible failure
   is a MINOR smell whose test can only assert "it refuses", never "it prevents something".
   The state argument's check **stays**, for the reason `assemble.Start` already refuses an
   unterminated fence on read (R10 case): the state file's four headings are found by a
   terminator scan.
3. **Which command validates the suffix.** Chosen: the `finish` path only. `new step` writes
   no handoff file and so needs no compiled suffix. A broken `handoff-file-suffix` therefore
   surfaces at `finish`, not at `new step`; `check` is the eventual backstop.
4. **An unreadable (not merely absent) handoff file during identity.** Chosen: treated as not
   identical, no refusal — the handoff file is this command's output, not an input the caller
   must repair, so absence and unreadability collapse to the same decision and any real
   failure surfaces from the write.
5. **SCENARIO-06 still says "all three files are byte-identical" and there are now four.**
   Read as "every file in the feature directory"; `snapshotTree` already asserts exactly that,
   so no test change is needed and none is invented.
6. **Sort order of the default suffix.** `SCENARIO-01-HANDOFF.md` sorts *before*
   `SCENARIO-01.md` (`-` < `.`). Accepted; `.handoff.md` was the alternative and reads as a
   build artifact rather than the audit-trail record the spec calls it.

## Implementation Plan

Additive first, delete last: `config.HandoffHeading` survives until nothing reads it, so the
suite compiles between steps.

### The naming primitive

- [ ] Step 1: `internal/platform/config/config_test.go` `Test_the_default_profile_names_the_handoff_file_suffix` — asserts `config.Default().HandoffFileSuffix == "-HANDOFF.md"`; red is the compile error naming the missing field (red)
- [ ] Step 2: `internal/platform/config/config.go` — add `HandoffFileSuffix` + `yaml:"handoff-file-suffix"` + its default; extend the `Config` doc comment (green)
- [ ] Step 3: `internal/platform/config/resolve_test.go` `Test_Resolve_reads_the_handoff_file_suffix_from_the_config_file` — a `.brief.yaml` setting only `handoff-file-suffix` overrides it and leaves `StepFilePattern` at its shipped value (green on arrival — say so; `decodeConfig` already overlays onto `Default()`)
- [ ] Step 4: `internal/platform/stepfile/handoff_test.go` `Test_CompileHandoff_names_the_handoff_file_from_the_step_id_and_the_suffix` — `SCENARIO-%02d.md` + `-HANDOFF.md` → `SCENARIO-01-HANDOFF.md` for n=1 (red)
- [ ] Step 5: `internal/platform/stepfile/handoff.go` `HandoffPattern`, `CompileHandoff`, `ErrInvalidHandoffSuffix`; extend `doc.go` to say this package owns the handoff file's name (green)
- [ ] Step 6: `handoff_test.go` `Test_CompileHandoff_refuses_a_suffix_containing_a_digit` — suffix `1.md` against `STEP-%d.md`, `require.ErrorIs` `ErrInvalidHandoffSuffix`; the doc comment names the `STEP-11.md` collision it prevents (red)
- [ ] Step 7: `handoff_test.go` — one test each for the remaining refusals: empty suffix, `/` and `\`, `%`, and a suffix equal to the step filename's extension (`.md` against `SCENARIO-%02d.md`, which would name the step file itself) (red → green in Step 8)
- [ ] Step 8: `internal/platform/stepfile/handoff.go` — the four remaining refusal rules (green)
- [ ] Step 9: `handoff_test.go` `Test_a_handoff_file_name_is_never_recognized_as_a_step_file` — for `SCENARIO-%02d.md` and `STEP-%d.md`, asserts `Pattern.Number(HandoffPattern.Name(n))` is false at n = 1, 9, 10, 99, 100, no loop, one assertion per value (green on arrival — it pins the property Steps 6–8 protect)

### `finish` writes the handoff to its own file

- [ ] Step 10: `internal/scaffold/finish_test.go` — `fixtureConfig` gains a `HandoffFileSuffix` differing from the default (e.g. `.fixture-handoff.md`); `finishFixture` gains `handoffPath()` deriving `STEP-02.fixture-handoff.md` literally, not through `stepfile`, so a production naming bug cannot hide behind the test's own derivation (new)
- [ ] Step 11: `finish_test.go` — `step02Body` gains a trailing `## Notes\n\nPLEASE KEEP THIS\n` after its bare handoff heading, and `newFinishFixture`'s specification gains a `## Notes` section after the progress list, so every "preserved outside the span" assertion has content to lose. **Expect this to redden roughly fifteen tests across `finish_test.go` and `finish_idempotent_test.go` at once, all with `ErrHandoffNotLast`, and to stay red until Step 16** — `step02Body` is `status: open` with a heading after the anchor, exactly what the old `TrailingHeading` gate refuses. That refusal, not an assertion mismatch, is the expected failure for Steps 12–14; do not "fix" it by dropping `## Notes`, which deletes the point of this step. The discriminating red for Step 13 arrives at Step 32, since the splice's destruction of `## Notes` is unobservable while the guard refuses first (new)
- [ ] Step 12: `finish_test.go` `Test_writes_the_supplied_handoff_to_its_own_file` — the file at `handoffPath()` exists and its bytes equal `fx.newHandoff` **verbatim**, including its trailing newline and its nested fence (red: no such file)
- [ ] Step 13: `finish_test.go` `Test_leaves_the_step_file_byte_identical_apart_from_the_status_line` — replaces `Test_leaves_the_rest_of_the_step_file_byte_identical`; `assert.Equal` against `step02Body` with `status: open` → `status: done`, nothing appended. This single assertion is R21's second clause at the Server level, the legacy-section preservation proof, and the assertion the splice cannot pass (red)
- [ ] Step 14: `finish_test.go` `Test_leaves_the_specification_byte_identical_apart_from_the_finished_step_s_progress_line` — `assert.Equal` against the fixture spec with `- [ ] STEP-02: …` → `- [x] STEP-02: …`, proving the trailing `## Notes` section survives (red only in that the fixture is new; report honestly if green on arrival)
- [ ] Step 15: delete the tests for refusals that cease to exist, each named with its reason, before the production change so the package still compiles: `Test_finish_refuses_a_step_file_whose_handoff_anchor_is_not_the_last_heading` and `Test_finish_refuses_the_structural_defect_over_the_handoff_fence_when_both_are_present` (`ErrHandoffNotLast` is deleted); `Test_finish_refuses_a_step_file_whose_on_disk_handoff_section_has_an_unterminated_fence` and `Test_refuses_a_step_file_whose_handoff_heading_is_swallowed_by_an_earlier_open_fence` and `Test_refuses_a_step_file_with_no_handoff_anchor` (there is no anchor to validate); `Test_finish_refuses_a_handoff_with_an_unterminated_fence` and `Test_finish_refuses_a_handoff_whose_closing_fence_is_indented_four_spaces` (silence 2 — the handoff argument is no longer fence-checked); `Test_finish_does_not_refuse_a_re_finish_whose_own_prior_handoff_contains_a_heading` and `Test_finishing_twice_with_a_nested_fenced_handoff_is_a_fixed_point` and `Test_finishing_with_a_handoff_already_in_place_reproduces_the_file_byte_for_byte` (the fixed point they pin is the splice's; a whole-file write of the same bytes is a fixed point by construction, and a prior reviewer already found the nested-fence one had lost its discriminating power). Their surviving claim — that adversarial handoff content round-trips — is Step 12's verbatim assertion (update)
- [ ] Step 16: `internal/scaffold/finish.go` — `Finish` rewritten: compile the handoff suffix after the step-file pattern and refuse `ErrInvalidHandoffSuffix` naming the feature directory and `handoff-file-suffix`; `findStepFile` also returns the step number; delete the `validateHandoffAnchor` call and the function; delete the `checkArgumentFence` call for the handoff argument (the state one stays); read the existing handoff file for identity; the four writes in the Decision-3 order; rewrite the doc comment to the new check order, write order and identity (green)
- [ ] Step 17: `internal/scaffold/render.go` — delete `spliceHandoff`; `internal/scaffold/errors.go` — delete `ErrHandoffNotLast` and the `HandoffSource` constant; `internal/scaffold/doc.go` — drop both from the sentinel list and drop the splice narrative (update)
- [ ] Step 18: `internal/cli/finish.go` — the `refusal.Path` switch collapses to the `StateSource` case alone; update `finishUsage` so it says the handoff is written to its own file beside the step file rather than "replaces its handoff block" (update)
- [ ] Step 19: `internal/scaffold/finish_test.go` `Test_finish_leaves_no_temp_file_in_the_feature_directory` — the `want` list gains the handoff file name; its control arm `Test_the_temp_file_sweep_sees_a_temp_file` is unchanged and keeps the probe honest (update)
- [ ] Step 20: `internal/scaffold/finish_test.go` `Test_returns_an_error_and_changes_nothing_when_the_feature_directory_is_not_writable` — add that no handoff file was created, so the first of the four writes failing leaves nothing partial (update)

### Identity and the no-op

- [ ] Step 21: `finish_idempotent_test.go` `Test_re_finishing_a_done_step_with_a_different_handoff_replaces_it` — repointed at the handoff **file**, asserting its bytes equal `differentHandoff` verbatim (no `strings.Trim`, no `markdown.Section`). This is the arm that reddens if the handoff conjunct is dropped from identity; SCENARIO-16 later inverts it into a refusal over the same conjunct (update)
- [ ] Step 22: `finish_idempotent_test.go` `Test_re_finishing_a_done_step_whose_handoff_file_is_missing_rewrites_it` — delete the handoff file from `newFinishedFixture`, re-finish with identical inputs, assert the file is back with the same bytes. Single-variable proof that "exists" is part of the conjunct, and the crash-recovery and migration path in one (red)
- [ ] Step 23: `finish_idempotent_test.go` — the `names` list in `Test_re_finishing_a_done_step_with_the_same_inputs_preserves_every_modification_time` and in its control arm `Test_the_modification_time_probe_sees_a_write_when_the_progress_entry_diverges` gains the handoff file; the control arm asserts the handoff file's mtime also moved, since identity is all-or-nothing across the four writes (update)
- [ ] Step 24: `finish_idempotent_test.go` `Test_a_step_whose_frontmatter_is_still_open_is_marked_done_even_when_every_input_matches_what_is_on_disk` — body unchanged; its comment states that its discriminating power depends on the step-body byte comparison staying **out** of identity, so a later "helpful" re-addition cannot silently kill it (update)
- [ ] Step 25: `finish_idempotent_test.go` `Test_re_finishing_a_done_step_with_the_same_inputs_leaves_every_file_byte_identical` — unchanged; `snapshotTree` already covers every file in the directory, which is how SCENARIO-06's "all three files" reads now that there are four (green on arrival)

### `new step` writes no handoff file

- [ ] Step 26: `internal/scaffold/step_test.go` `Test_writes_no_handoff_file` — after `NewStep`, `os.Stat` of the derived handoff path returns `fs.ErrNotExist`; the path comes from the same test-local helper the control arm uses. Expected **green on arrival** — `NewStep` never wrote a handoff file — so say so rather than manufacturing a red; its non-vacuity is Step 28's job, not a fabricated failure (new)
- [ ] Step 27: `internal/scaffold/render.go` `stepSkeleton` — drop `cfg.HandoffHeading` and its blank line; `internal/scaffold/step_test.go` `Test_writes_the_step_file_with_frontmatter_a_title_and_an_empty_checklist` — renamed from `…_and_a_handoff_anchor`, asserting the exact skeleton with no handoff heading (red → green together)
- [ ] Step 28: `internal/scaffold/step_test.go` `Test_the_handoff_probe_sees_a_handoff_file_after_a_finish` — the control arm for Step 26: same helper, same feature, after a `Finish`, `os.Stat` succeeds. Without it, Step 26 passes vacuously by probing the wrong path (new)
- [ ] Step 29: `internal/scaffold/step_test.go` `Test_a_handoff_file_does_not_advance_the_next_step_number` — a feature holding `STEP-01.md` and `STEP-01<suffix>` scaffolds `STEP-02.md`, not `STEP-03.md` (green on arrival; the discriminating mutation is `Pattern.Number`'s digits-only scan, not the round-trip check) (new)

### R21's second clause, proved by mutation

- [ ] Step 30: mutate `stepfile.SetStatus` to return `s[:openLen] + join(lines,"\n")` without the closing delimiter and body — Step 13 must redden, alone. Stash the mutation per `.claude/rules/agent-briefs.md`, restore, `diff` byte-identical (verify)
- [ ] Step 31: mutate `tickProgressEntry` to `strings.Join(lines[:matchIdx+1], "\n")` — Step 14 must redden, and `Test_leaves_every_other_progress_entry_unchanged` with it. Individually, not with Step 30 (verify)
- [ ] Step 32: reinstate a splice by appending the handoff to the step body in `Finish` — Step 13 must redden. This is the mutation proving the old write path is genuinely gone and not merely unused (verify)

### The read side

- [ ] Step 33: `internal/assemble/assemble_test.go` — `fixtureConfig` gains a fixture `HandoffFileSuffix`; `fixtureStep` drops its handoff section; `newFixture` writes `STEP-01<suffix>` and `STEP-02<suffix>` carrying `HANDOFF-ONLY-01`/`-02` (update)
- [ ] Step 34: `internal/assemble/assemble_test.go` — the existing absence claim (`NotContains HANDOFF-ONLY-01`/`-02` over `allSectionText`) is unchanged and now means `Start` does not read handoff files at all; its control arm splits out as `Test_each_finished_step_s_handoff_file_carries_its_marker`, reading each handoff file from disk so the absence cannot pass on an empty fixture (update)
- [ ] Step 35: `internal/assemble/assemble_test.go` `Test_a_handoff_file_is_not_counted_as_a_step` — with two handoff files in the feature directory, the brief still reports 2 done and 3 open. Proves the naming discipline from the read side (new)
- [ ] Step 36: `internal/cli/finish_test.go` — `newFinishCLIFixture`'s step body drops `## Handoff`; `Test_finishes_the_step_and_prints_nothing_to_stdout` gains nothing, but a new `Test_writes_the_handoff_file_beside_the_step_file` asserts `SCENARIO-01-HANDOFF.md` — the **default** suffix, which no `scaffold` test pins — holds the supplied body (new)
- [ ] Step 37: `internal/cli/finish_test.go` `Test_finish_leaves_a_legacy_handoff_section_in_the_step_file_untouched` — a fixture step file that still carries `## Handoff` with prose under it finishes successfully and that section survives byte for byte. This is the migration-tolerance contract for `brief`'s own tree and for any adopter's (new)
- [ ] Step 38: `internal/cli/finish_test.go` `Test_names_stdin_when_the_piped_handoff_s_fence_is_unterminated` → repointed to the **state** argument as `Test_names_stdin_when_the_piped_state_s_fence_is_unterminated`, keeping `sourceLocator`'s `<stdin>` branch tested now that no handoff refusal reaches it; delete `Test_names_the_handoff_path_when_its_fence_is_unterminated` (update)
- [ ] Step 39: `internal/cli/finish_test.go` `Test_finishes_a_step_whose_handoff_heading_ends_in_a_carriage_return` → repointed as `Test_preserves_a_CRLF_step_body_when_marking_it_done`: same CRLF fixture, asserting the step file equals the original with only `status:` changed. Its original discriminator (the CR-tolerant anchor scan) is gone; CRLF preservation through the one remaining in-file step-file write is the claim worth keeping (update)

### Deleting the scanners

- [ ] Step 40: `internal/platform/markdown/section.go` — `SectionRange` becomes unexported `sectionRange` (implementation unchanged, `Section` still built on it); delete `HeadingStart` and `TrailingHeading`; `doc.go` drops the read-side/write-side paragraph and the `SectionRange` naming (update)
- [ ] Step 41: `internal/platform/markdown/section_test.go` — delete the three `Test_HeadingStart_*` and five `Test_TrailingHeading_*` tests; the functions they test cease to exist, which is the whole justification and is stated once. Before deleting `Test_TrailingHeading_never_reports_a_fenced_decoy_as_the_successor`, confirm `Test_Section_does_not_treat_a_hash_line_inside_a_fenced_block_as_a_heading` still covers the terminator scan's fence-awareness (update)
- [ ] Step 42: `internal/platform/markdown/section_test.go` — repoint every `Test_SectionRange_*` case to `Section`, renamed `Test_Section_*`, asserting the trimmed body instead of offsets: the empty section returns `""`, the section followed by a same-level heading returns its own body, the last section in a file returns its body, the anchor with no trailing newline returns `""`, and every fence-machine case (tilde vs backtick, shorter run, info string, three- vs four-space indentation, CR-terminated heading, heading not present, first-occurrence-wins) carries over unchanged in meaning. Report the count before and after and name every case dropped as already covered by an existing `Test_Section_*`. One sub-claim is deliberately lost: "`end` points at the terminating line including its leading whitespace" is unrepresentable through `Section` and existed only for splicing (update)
- [ ] Step 43: mutate `fenceState` so a closing run of any length closes the fence — the repointed shorter-run test must redden; separately mutate it to ignore the info string — that test must redden. Individually, proving the repointed tests kept their discriminating power (verify)

### Removing the old key

- [ ] Step 44: `internal/platform/config/config.go` — delete `HandoffHeading` and its yaml tag; update the `Config` doc comment; `internal/platform/config/resolve_test.go:115` — repoint the default assertion to `HandoffFileSuffix`. `Test_Resolve_refuses_a_config_carrying_an_unknown_key` already covers the consequence that a `.brief.yaml` naming `handoff-heading` is now refused; confirm it and add nothing (update)
- [ ] Step 45: `internal/scaffold/scaffold_test.go`, `internal/scaffold/step_test.go`, `internal/scaffold/finish_test.go`, `internal/assemble/assemble_test.go` — remove every remaining `cfg.HandoffHeading` reference; the fixture bodies that still carry a literal `## Handoff` line keep it as a literal, since it is now ordinary prose the tool must ignore (update)

### Migrating `brief`'s own tree

- [ ] Step 46: copy all six of `docs/specifications/brief/SCENARIO-01.md` … `-06.md` to `$TMPDIR` **before touching anything**; then for each, extract the handoff section's body **verbatim** into `SCENARIO-0N-HANDOFF.md` (no title line) and delete heading plus body from the step file. The boundary is read by eye per file, never "to EOF": `SCENARIO-03.md`'s first `## Handoff` at line 184 is inside a fenced example and its real section is at 359; `SCENARIO-06.md`'s section ends before `## Crossover note` at line 159, which stays in the step file (update)
- [ ] Step 47: prove the migration lossless, per file, by `cmp` against the `$TMPDIR` copy from Step 46 — **not against `HEAD`**, which moves the moment anything is committed and would then compare a migrated file with itself and pass vacuously. The copy must equal the byte concatenation of the truncated step file's prefix, the `## Handoff` heading line, the new handoff file, and — for `SCENARIO-06.md` — the retained suffix from `## Crossover note` on. Where a pinned reference is wanted instead, it is `f225479:docs/specifications/brief/SCENARIO-0N.md`. A migration that silently drops a line is the exact failure this decision exists to prevent, so this check is the deliverable, not a read-through (verify)
- [ ] Step 48: tidy each new handoff file's leading and trailing blank lines to start at the first non-blank line and end with exactly one newline, then prove nothing but blank lines changed: `diff` of the two files with blank lines removed must be empty (update)
- [ ] Step 49: `docs/specifications/brief/STATE.md` — rewritten by the developer from this plan's Handoff, replacing every splice-era entry rather than appending to them (update)

### Verification

- [ ] Step 50: `go build ./...`; `go test -count=1 ./...` unpiped, reporting the exact test count against the arithmetic below; `go test -race -count=1` on `./internal/scaffold/...`, `./internal/assemble/...`, `./internal/cli/...`, `./internal/platform/...`; `golangci-lint run ./...`; `go doc ./internal/scaffold`, `go doc ./internal/scaffold Server.Finish`, `go doc ./internal/platform/stepfile` and `go doc ./internal/platform/markdown` read as contracts with no history and no `SCENARIO-XX` narrative (verify)
- [ ] Step 51: confirm the skip count did not move: `go test -v ./... 2>&1 | grep -c -- "--- SKIP"` (verify)

**Expected test count: 223, from 226.** Additions, 19: Steps 1, 3, 4, 6 (one each), Step 7
(four refusals), Steps 9, 12, 14, 22, 26, 28, 29, 34, 35, 36, 37 (one each). Deletions, 22:
Step 15 (ten in `scaffold`), Step 38 (one in `cli`), Step 41 (eight in `markdown`), Step 42
(three `SectionRange` cases folded into an existing `Test_Section_*` asserting the identical
claim — heading-not-present, indented terminating heading, four-space-indented hash line).
Renames are count-neutral: Steps 13, 27, 38's stdin repoint, 39, and the sixteen `Test_Section_*`
repoints. The only legitimate variance is a further Step 42 fold; each one decrements the total
by one and must be named. Any other number is a finding, not a rounding error.

Commands for the red/green loop, run from the repo root:

```bash
go test -count=1 -run '^Test_writes_the_supplied_handoff_to_its_own_file$' ./internal/scaffold/
go test -count=1 ./internal/scaffold/ ./internal/platform/stepfile/
go test -count=1 ./...
```

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:

- The handoff filename is `stepPattern.ID(n) + cfg.HandoffFileSuffix`, compiled by
  `stepfile.CompileHandoff` — an independent `handoff-file-pattern` is rejected because it
  admits a pattern identical to `step-file-pattern`, in which case `finish` writes the handoff
  body over the step file.
- `HandoffFileSuffix` must contain no digit — suffix `1.md` against `STEP-%d.md` renders
  `STEP-11.md`, which `Pattern.Number` accepts as step 11. SCENARIO-22's `check` and R6's
  synthesis depend on a handoff file never round-tripping as a step file.
- `config.HandoffHeading` no longer exists; `handoff-heading` in a `.brief.yaml` is refused as
  an unknown key, and a later `init` template must write `handoff-file-suffix`.
- The handoff body is written **verbatim**, like the state body. SCENARIO-16's divergence
  comparison and SCENARIO-17's cap must both measure the supplied bytes, not a normalized form.
- `finish`'s write order is handoff → state → step `status:` → progress checkbox. `status: done`
  must never land before the handoff file exists: SCENARIO-16's divergence refusal would
  otherwise refuse the retry that repairs a crash. Every earlier prefix leaves `status: open`,
  the sole doneness authority, so a retry converges.
- Identity is `fm.Done() && handoff file exists and matches && state matches && ticked spec
  matches`. An absent handoff file is not identical. **Do not add a step-body byte comparison**
  — it makes `Test_a_step_whose_frontmatter_is_still_open_is_marked_done_…` stop discriminating
  the `fm.Done()` gate.
- A `## Handoff` section left in a step file is ignored by `start` and `finish`, never refused.
  R7 keeps prose judgment out of the tool, and refusing would lock `brief` out of its own tree
  mid-migration.
- `markdown.Section` and `markdown.UnterminatedFence` are the only exported readers left.
  `Section` is built on unexported `sectionRange`; no caller outside the package gets offsets,
  because offsets exist only to splice.

**Left unbuilt** — named so nobody assumes it exists:

- `scaffold.HandoffSource` and `cli/finish.go`'s `--handoff` source upgrade — deleted with the
  handoff fence refusal; **SCENARIO-17**'s over-cap handoff refusal re-adds both, and its test
  is what proves the `<stdin>` locator for `--handoff`.
- The handoff argument's unterminated-fence refusal — deleted; re-add only if R6's synthesis
  concatenates handoff bodies into one stream, where an open fence in one would swallow the next.
- `HandoffPattern.Number` (recognizing a handoff filename) — not built; `check` (22) and the R6
  synthesis are the first callers needing to enumerate handoff files. Validation of
  `handoff-file-suffix` likewise runs only on the `finish` path, never on `new step`.
- `spliceHandoff`, `markdown.SectionRange`, `markdown.HeadingStart`, `markdown.TrailingHeading`,
  `scaffold.ErrHandoffNotLast` — deleted. Do not reintroduce a scanner to serve a write.
- Frontmatter on `docs/specifications/brief/SCENARIO-01.md` … `-06.md` — still absent; the
  crossover owns it, not this migration.

**Traps** — things that look right and are not:

- **`## Handoff` to EOF is wrong on `brief`'s own files.** `SCENARIO-03.md` has a `## Handoff`
  line at 184 **inside a fenced block** (real section at 359); `SCENARIO-06.md`'s section is
  followed by `## Crossover note` at 159, which is not handoff content.
- The `-HANDOFF.md` suffix sorts *before* its step file (`-` < `.`), so `ls` shows the handoff
  first. Cosmetic; do not "fix" it by changing the default without migrating the tree.
- Carried forward, unowned and unfixed here: `tickProgressEntry` mutates a whole line, so an
  already-ticked title containing `[ ]` is mutated too; `SetStatus`'s mixed-line-ending branch
  is untested and it returns `ErrNoStatusField` for a missing frontmatter *delimiter* as well as
  a missing key; refusal *ordering* in `Finish` is pinned by no test (the new
  `ErrInvalidHandoffSuffix` slots in second and names the feature directory, not the config
  file); `scaffold` still owns `finish` despite its name.

**Open debts** this plan creates:

- `check` (SCENARIO-22) must report a `## Handoff` section surviving in a step file:
  `[MINOR] <step file>:<line> — handoff section in a step file is no longer read; move it to
  <handoff file>`. Until then the leftover is silently ignored.
- `finish`'s four-write order has no test seam and is documented only. A mid-write I/O failure
  still leaves a half-applied result; `check` is the eventual detector.
