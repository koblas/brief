package scaffold_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_re_finishing_a_done_step_with_a_different_handoff_is_refused inverts
// the arm that used to prove the handoff conjunct silently overwrote the
// recorded file: a done step re-finished with a handoff that differs from
// the one on disk must now be refused, naming the handoff file specifically,
// rather than discarding the caller's new bytes while claiming success.
func Test_re_finishing_a_done_step_with_a_different_handoff_is_refused(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	differentHandoff := []byte("DIFFERENT-HANDOFF-02\n")

	_, err := fx.finish(t, "STEP-02", differentHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrAlreadyFinished)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, fx.handoffPath(), refusal.Path)
	assert.Contains(t, refusal.Problem, "already done")
	assert.Contains(t, refusal.Problem, "handoff differs")
	assert.Contains(t, refusal.Fix, "diff the handoff you passed")
}

// Test_re_finishing_a_done_step_whose_handoff_file_is_missing_rewrites_it
// is the single-variable proof that a missing handoff file exempts a done
// step from the divergence refusal entirely, rather than merely relaxing a
// byte comparison: removing it before an otherwise-identical re-finish must
// still write, covering the crash-after-state retry and a migrated step
// alike.
func Test_re_finishing_a_done_step_whose_handoff_file_is_missing_rewrites_it(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	require.NoError(t, fx.mem.Remove(fx.handoffName()))

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile(fx.handoffName())
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newHandoff), string(got))
}

// Test_re_finishing_a_done_step_whose_handoff_file_is_missing_accepts_a_different_state
// is Step 4's control arm: identical to
// Test_a_refused_re_finish_leaves_every_file_byte_identical but for the
// handoff file's presence — one variable, opposite outcome, same probe. A
// missing handoff file exempts the step from the divergence refusal, so a
// differing state now writes rather than being refused.
func Test_re_finishing_a_done_step_whose_handoff_file_is_missing_accepts_a_different_state(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	require.NoError(t, fx.mem.Remove(fx.handoffName()))
	differentState := differentStateBody(fx.cfg)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, differentState)
	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile(fx.cfg.StateFile)
	require.NoError(t, readErr)
	assert.Equal(t, string(differentState), string(got))
}

// Test_re_finishing_a_done_step_with_a_different_state_body_is_refused
// inverts the arm that used to prove the state conjunct silently
// overwrote the recorded file: a done step re-finished with a state body
// that differs from the one on disk must now be refused, naming
// cfg.StateFile specifically.
func Test_re_finishing_a_done_step_with_a_different_state_body_is_refused(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	differentState := differentStateBody(fx.cfg)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, differentState)

	require.ErrorIs(t, err, scaffold.ErrAlreadyFinished)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, filepath.Join(testFeaturePath, fx.cfg.StateFile), refusal.Path)
	assert.Contains(t, refusal.Problem, "already done")
	assert.Contains(t, refusal.Problem, "state differs")
	assert.Contains(t, refusal.Fix, "diff the state you passed")
}

// Test_re_finishing_a_done_step_with_both_inputs_differing_names_the_handoff_first
// pins R14a's first-thing-wrong order for measured case (d): when both the
// handoff and the state differ from what is recorded, the refusal names the
// handoff file, matching the write order (handoff lands before state).
func Test_re_finishing_a_done_step_with_both_inputs_differing_names_the_handoff_first(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	differentHandoff := []byte("DIFFERENT-HANDOFF-02\n")
	differentState := differentStateBody(fx.cfg)

	_, err := fx.finish(t, "STEP-02", differentHandoff, differentState)

	require.ErrorIs(t, err, scaffold.ErrAlreadyFinished)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, fx.handoffPath(), refusal.Path, "both inputs differing must still name the handoff first")
}

// Test_a_refused_re_finish_leaves_every_file_byte_identical pins the
// state-divergence arm's "nothing lands" claim: rwfs.Mem.Snapshot's
// ModTime advances on every write, including one that rewrites identical
// bytes, so snapshot equality alone proves the refusal happens before the
// first write — not merely that any write that did happen reproduced the
// same bytes. Test_the_snapshot_probe_sees_a_write_on_a_legitimate_finish,
// below, is the control arm proving the probe can see a write at all.
func Test_a_refused_re_finish_leaves_every_file_byte_identical(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	before := fx.mem.Snapshot()
	differentState := differentStateBody(fx.cfg)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, differentState)

	require.ErrorIs(t, err, scaffold.ErrAlreadyFinished)
	assert.Equal(t, before, fx.mem.Snapshot())
}

// Test_the_snapshot_probe_sees_a_write_on_a_legitimate_finish is the
// probe-sanity control for Test_a_refused_re_finish_leaves_every_file_byte_identical:
// the same Snapshot probe, over a first, successful Finish call on an open
// step, must show a difference — proving the probe is capable of detecting
// a write at all, so its silence on the refused arm is evidence of
// "nothing landed" rather than of a probe that never fires.
func Test_the_snapshot_probe_sees_a_write_on_a_legitimate_finish(t *testing.T) {
	fx := newFinishFixtureFS(t)
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	assert.NotEqual(t, before, fx.mem.Snapshot())
}

// Test_re_finishing_a_done_step_with_an_un_ticked_entry_and_a_different_handoff_is_refused
// pins that an un-ticked progress entry is not an escape hatch from the
// divergence arms: specTicked participates only in the noop-vs-write split
// (SCENARIO-06/R11), never as a signal that inputs are free to diverge.
func Test_re_finishing_a_done_step_with_an_un_ticked_entry_and_a_different_handoff_is_refused(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	spec := readStepFS(t, fx, fx.cfg.SpecificationFile)
	unticked := strings.Replace(spec, "- [x] STEP-02: Assemble the thing", "- [ ] STEP-02: Assemble the thing", 1)
	putStepFS(t, fx, fx.cfg.SpecificationFile, unticked)
	differentHandoff := []byte("DIFFERENT-HANDOFF-02\n")

	_, err := fx.finish(t, "STEP-02", differentHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrAlreadyFinished)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, fx.handoffPath(), refusal.Path)
}

func Test_re_finishing_a_done_step_whose_progress_entry_was_un_ticked_re_ticks_it(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	spec := readStepFS(t, fx, fx.cfg.SpecificationFile)
	unticked := strings.Replace(spec, "- [x] STEP-02: Assemble the thing", "- [ ] STEP-02: Assemble the thing", 1)
	putStepFS(t, fx, fx.cfg.SpecificationFile, unticked)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile(fx.cfg.SpecificationFile)
	require.NoError(t, readErr)
	assert.Contains(t, string(got), "- [x] STEP-02: Assemble the thing")
}

// Test_a_write_on_the_progress_entry_divergence_arm_changes_the_snapshot
// is SCENARIO-16's control arm for the Mem snapshot probe: it diverges on
// the specification's checkbox, not on a caller input, so it stays valid
// once SCENARIO-16 turns differing-input divergence into a refusal. Every
// one of the four files Finish writes changes, proven as one snapshot
// comparison rather than four separate mtime probes.
func Test_a_write_on_the_progress_entry_divergence_arm_changes_the_snapshot(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	spec := readStepFS(t, fx, fx.cfg.SpecificationFile)
	unticked := strings.Replace(spec, "- [x] STEP-02: Assemble the thing", "- [ ] STEP-02: Assemble the thing", 1)
	putStepFS(t, fx, fx.cfg.SpecificationFile, unticked)
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	after := fx.mem.Snapshot()
	assert.NotEqual(t, before["STEP-02.md"], after["STEP-02.md"], "step file kept its old snapshot entry")
	assert.NotEqual(t, before[fx.cfg.StateFile], after[fx.cfg.StateFile], "state file kept its old snapshot entry")
	assert.NotEqual(t, before[fx.cfg.SpecificationFile], after[fx.cfg.SpecificationFile], "specification kept its old snapshot entry")
	assert.NotEqual(t, before[fx.handoffName()], after[fx.handoffName()], "handoff file kept its old snapshot entry")
}

// Test_re_finishing_a_done_step_with_the_same_inputs_preserves_every_snapshot_entry
// is row 6's negative claim (R11): every input already matches what is on
// disk, so a re-finish must write nothing at all — proven by snapshot
// equality, which a byte comparison alone cannot: rwfs.Mem's ModTime
// advances even on a write that reproduces the same bytes, so this would
// catch a regression a byte-only probe would miss.
func Test_re_finishing_a_done_step_with_the_same_inputs_preserves_every_snapshot_entry(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	assert.Equal(t, before, fx.mem.Snapshot())
}

// Test_re_finishing_a_done_step_with_the_same_inputs_and_an_unticked_entry_still_writes
// is row 6's control arm: change exactly the one variable
// Test_re_finishing_a_done_step_with_the_same_inputs_preserves_every_snapshot_entry
// holds fixed — the specification's own progress tick, not a caller
// input — and the no-op flips to a write. Without this control, snapshot
// equality on the row-6 test would be equally consistent with a probe that
// cannot detect a change at all.
func Test_re_finishing_a_done_step_with_the_same_inputs_and_an_unticked_entry_still_writes(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	spec := readStepFS(t, fx, fx.cfg.SpecificationFile)
	unticked := strings.Replace(spec, "- [x] STEP-02: Assemble the thing", "- [ ] STEP-02: Assemble the thing", 1)
	putStepFS(t, fx, fx.cfg.SpecificationFile, unticked)
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	assert.NotEqual(t, before, fx.mem.Snapshot())
}

// Test_a_step_whose_frontmatter_is_still_open_is_marked_done_even_when_every_input_matches_what_is_on_disk
// reverts only the status line, leaving the handoff, state file and ticked
// checkbox untouched, so this is the single-variable proof that the
// fm.Done() gate — not a step-body byte comparison — is what forces the
// write. Identity deliberately carries no step-body comparison: adding one
// back would make this test stop discriminating the gate, since a
// reverted status line alone would then force the write for a reason
// other than fm.Done().
//
// Test_re_finishing_a_done_step_whose_progress_title_contains_an_unticked_marker_stays_a_noop
// is the regression tickProgressEntry's first-occurrence Replace used to
// cause: on an already-ticked entry whose own title text happens to
// contain a literal "[ ]" (not the checklist marker itself), Replace
// rewrote that title-text "[ ]" instead of leaving an already-"[x]" line
// untouched, silently editing the specification on every re-finish and
// breaking the noop verdict (R11).
func Test_re_finishing_a_done_step_whose_progress_title_contains_an_unticked_marker_stays_a_noop(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	spec := readStepFS(t, fx, fx.cfg.SpecificationFile)
	withMarkerInTitle := strings.Replace(spec,
		"- [x] STEP-02: Assemble the thing",
		"- [x] STEP-02: render a [ ] marker",
		1)
	putStepFS(t, fx, fx.cfg.SpecificationFile, withMarkerInTitle)
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	assert.Equal(t, before, fx.mem.Snapshot(), "a re-finish with identical inputs must write nothing")
}

func Test_a_step_whose_frontmatter_is_still_open_is_marked_done_even_when_every_input_matches_what_is_on_disk(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	body := readStepFS(t, fx, "STEP-02.md")
	reverted := strings.Replace(body, "status: done\n", "status: open\n", 1)
	putStepFS(t, fx, "STEP-02.md", reverted)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile("STEP-02.md")
	require.NoError(t, readErr)

	fm, _, parseErr := stepfile.ParseFrontmatter(got)
	require.NoError(t, parseErr)
	assert.True(t, fm.Done())
}
