package scaffold_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func Test_re_finishing_a_done_step_whose_handoff_file_is_missing_rewrites_it(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	require.NoError(t, fx.mem.Remove(fx.handoffName()))

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile(fx.handoffName())
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newHandoff), string(got))
}

// Control arm for Test_a_refused_re_finish_leaves_every_file_byte_identical:
// identical fixture but for the handoff file's presence.
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

// rwfs.Mem.Snapshot's ModTime advances on every write, so equality here
// proves the refusal happens before the first write. Control arm below:
// Test_the_snapshot_probe_sees_a_write_on_a_legitimate_finish.
func Test_a_refused_re_finish_leaves_every_file_byte_identical(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	before := fx.mem.Snapshot()
	differentState := differentStateBody(fx.cfg)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, differentState)

	require.ErrorIs(t, err, scaffold.ErrAlreadyFinished)
	assert.Equal(t, before, fx.mem.Snapshot())
}

func Test_the_snapshot_probe_sees_a_write_on_a_legitimate_finish(t *testing.T) {
	fx := newFinishFixtureFS(t)
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	assert.NotEqual(t, before, fx.mem.Snapshot())
}

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

// Diverges on the specification's checkbox, not on a caller input. Every
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

// Proven by snapshot equality, which a byte comparison alone cannot:
// rwfs.Mem's ModTime advances even on a write that reproduces the same
// bytes.
func Test_re_finishing_a_done_step_with_the_same_inputs_preserves_every_snapshot_entry(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	assert.Equal(t, before, fx.mem.Snapshot())
}

// Control arm for Test_re_finishing_a_done_step_with_the_same_inputs_preserves_every_snapshot_entry:
// changes only the specification's own progress tick, not a caller input.
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

// An already-ticked entry whose own title text happens to contain a
// literal "[ ]" (not the checklist marker) must not be rewritten by
// tickProgressEntry, or every re-finish would silently edit the
// specification and break the no-op verdict.
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

func Test_re_finishing_a_done_step_with_no_checklist_items_and_the_same_inputs_is_a_noop(t *testing.T) {
	cases := []struct {
		name string
		body func(config.Config) string
	}{
		{name: "no checklist heading", body: doneNoHeadingStep02Body},
		{name: "zero checklist items", body: doneZeroItemStep02Body},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newFinishedFixtureFS(t)
			putStepFS(t, fx, "STEP-02.md", c.body(fx.cfg))
			before := fx.mem.Snapshot()

			_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

			require.NoError(t, err)
			assert.Equal(t, before, fx.mem.Snapshot())
		})
	}
}

func Test_re_finishing_a_done_step_with_no_checklist_items_and_a_different_handoff_is_refused_as_already_finished(t *testing.T) {
	cases := []struct {
		name string
		body func(config.Config) string
	}{
		{name: "no checklist heading", body: doneNoHeadingStep02Body},
		{name: "zero checklist items", body: doneZeroItemStep02Body},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newFinishedFixtureFS(t)
			putStepFS(t, fx, "STEP-02.md", c.body(fx.cfg))
			differentHandoff := []byte("DIFFERENT-HANDOFF-02\n")

			_, err := fx.finish(t, "STEP-02", differentHandoff, fx.newState)

			require.ErrorIs(t, err, scaffold.ErrAlreadyFinished)
			assert.NotErrorIs(t, err, scaffold.ErrUnplannedStep)
		})
	}
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
