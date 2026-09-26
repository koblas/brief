package scaffold_test

import (
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readStepFS reads name from fx's own rwfs.Mem as a string — the Mem
// counterpart of readFileString(t, fx.stepPath(name)).
func readStepFS(t *testing.T, fx finishFixtureFS, name string) string {
	t.Helper()

	data, err := fx.mem.ReadFile(name)
	require.NoError(t, err)

	return string(data)
}

// reopenStep01FS rewrites newFinishFixtureFS's STEP-01 from "status: done"
// to "status: open", so STEP-02's depends-on: [STEP-01] goes unmet — the
// fixture's happy path ships STEP-01 done, satisfied by default.
func reopenStep01FS(t *testing.T, fx finishFixtureFS) {
	t.Helper()

	reopened := strings.Replace(readStepFS(t, fx, "STEP-01.md"), "status: done\n", "status: open\n", 1)
	putStepFS(t, fx, "STEP-01.md", reopened)
}

func Test_finish_refuses_a_step_whose_dependency_is_not_finished(t *testing.T) {
	fx := newFinishFixtureFS(t)
	reopenStep01FS(t, fx)
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrUnmetDependency)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, fx.stepPath("STEP-02.md"), refusal.Path)
	assert.Equal(t, 0, refusal.Line)
	assert.Equal(t,
		fx.stepPath("STEP-02.md")+
			`: step "STEP-02" depends on "STEP-01", which is not finished; finish STEP-01 first, or remove it from this step's depends-on, and retry`,
		err.Error())

	assert.Equal(t, before, fx.mem.Snapshot())
}

func Test_finish_refuses_a_dependency_id_that_names_no_step_file(t *testing.T) {
	fx := newFinishFixtureFS(t)
	unknownDep := strings.Replace(step02Body(fx.cfg), "depends-on: [STEP-01]\n", "depends-on: [STEP-99]\n", 1)
	putStepFS(t, fx, "STEP-02.md", unknownDep)
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrUnmetDependency)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, fx.stepPath("STEP-02.md"), refusal.Path)
	assert.Equal(t, 0, refusal.Line)
	assert.Equal(t,
		fx.stepPath("STEP-02.md")+
			`: step "STEP-02" depends on "STEP-99", which names no step file; correct the id in this step's depends-on, or remove it, and retry`,
		err.Error())

	assert.Equal(t, before, fx.mem.Snapshot())
}

// STEP-01 exists but its frontmatter is garbage, so it is recorded as a
// known, not-done step rather than skipped.
func Test_finish_refuses_a_dependency_whose_step_file_does_not_parse(t *testing.T) {
	fx := newFinishFixtureFS(t)
	putStepFS(t, fx, "STEP-01.md", "not frontmatter at all\n")

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrUnmetDependency)
	require.NotErrorIs(t, err, scaffold.ErrMalformedFeature)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Contains(t, refusal.Problem, "is not finished")
	assert.NotContains(t, refusal.Problem, "names no step file")
}

// A self-dependency takes the "is not finished" branch, naming the step's
// own id on both sides of the line.
func Test_finish_refuses_a_step_that_depends_on_itself(t *testing.T) {
	fx := newFinishFixtureFS(t)
	selfDep := strings.Replace(step02Body(fx.cfg), "depends-on: [STEP-01]\n", "depends-on: [STEP-02]\n", 1)
	putStepFS(t, fx, "STEP-02.md", selfDep)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrUnmetDependency)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t,
		fx.stepPath("STEP-02.md")+
			`: step "STEP-02" depends on "STEP-02", which is not finished; finish STEP-02 first, or remove it from this step's depends-on, and retry`,
		err.Error())
}

// STEP-03 declares depends-on: [] in a feature that also holds STEP-02, an
// unrelated not-done sibling; the four writes are asserted so this cannot
// pass on a silent no-op.
func Test_finish_accepts_a_step_with_no_declared_dependencies(t *testing.T) {
	fx := newFinishFixtureFS(t)
	ticked := strings.Replace(readStepFS(t, fx, "STEP-03.md"), "- [ ] not done yet\n", "- [x] not done yet\n", 1)
	putStepFS(t, fx, "STEP-03.md", ticked)
	handoff := []byte("STEP-03 handoff\n")

	_, err := fx.finish(t, "STEP-03", handoff, fx.newState)
	require.NoError(t, err)

	stepGot, readErr := fx.mem.ReadFile("STEP-03.md")
	require.NoError(t, readErr)
	assert.Contains(t, string(stepGot), "status: done")

	specGot, readErr := fx.mem.ReadFile(fx.cfg.SpecificationFile)
	require.NoError(t, readErr)
	assert.Contains(t, string(specGot), "- [x] STEP-03")

	stateGot, readErr := fx.mem.ReadFile(fx.cfg.StateFile)
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newState), string(stateGot))

	handoffGot, readErr := fx.mem.ReadFile("STEP-03" + fx.cfg.HandoffFileSuffix)
	require.NoError(t, readErr)
	assert.Equal(t, string(handoff), string(handoffGot))
}

func Test_finish_accepts_a_step_whose_dependency_is_done(t *testing.T) {
	fx := newFinishFixtureFS(t)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile("STEP-02.md")
	require.NoError(t, readErr)

	fm, _, parseErr := stepfile.ParseFrontmatter(got)
	require.NoError(t, parseErr)
	assert.True(t, fm.Done())
}

func Test_finish_reports_an_open_checklist_item_before_an_unfinished_dependency(t *testing.T) {
	fx := newFinishFixtureFS(t)
	reopenStep01FS(t, fx)
	putStepFS(t, fx, "STEP-02.md", step02BodyWithOpenItem(fx.cfg))

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOpenChecklistItem)
	assert.NotErrorIs(t, err, scaffold.ErrUnmetDependency)
}

func Test_finish_reports_an_unplanned_checklist_before_an_unfinished_dependency(t *testing.T) {
	fx := newFinishFixtureFS(t)
	reopenStep01FS(t, fx)
	putStepFS(t, fx, "STEP-02.md", zeroItemStep02Body(fx.cfg))

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrUnplannedStep)
	assert.NotErrorIs(t, err, scaffold.ErrUnmetDependency)
}

func Test_finish_reports_an_unfinished_dependency_before_the_specification_read(t *testing.T) {
	fx := newFinishFixtureFS(t)
	reopenStep01FS(t, fx)
	spec := "# widgets\n\n" + fx.cfg.ProgressHeading + "\n\n" +
		"- [ ] STEP-01: already done\n" +
		"- [ ] STEP-03\n"
	require.NoError(t, fx.mem.WriteFile(fx.cfg.SpecificationFile, []byte(spec), 0o600))

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrUnmetDependency)
	assert.NotErrorIs(t, err, scaffold.ErrNoProgressEntry)
}

func Test_re_finishing_a_done_step_whose_dependency_is_open_with_recorded_inputs_stays_a_noop(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	reopenStep01FS(t, fx)
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	assert.Equal(t, before, fx.mem.Snapshot())
}

func Test_re_finishing_a_done_step_whose_dependency_is_open_with_divergent_inputs_is_still_ErrAlreadyFinished(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	reopenStep01FS(t, fx)
	divergentHandoff := []byte("DIFFERENT-HANDOFF-02\n")

	_, err := fx.finish(t, "STEP-02", divergentHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrAlreadyFinished)
	assert.NotErrorIs(t, err, scaffold.ErrUnmetDependency)
}
