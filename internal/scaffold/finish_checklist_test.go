package scaffold_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// step02BodyWithOpenItem returns newFinishFixture's STEP-02 body with its
// checklist's second item left unticked, landing at line 13.
func step02BodyWithOpenItem(cfg config.Config) string {
	return openItemStep02Body(cfg, "open", "second thing")
}

// doneStep02BodyWithOpenItem is step02BodyWithOpenItem with its frontmatter
// already saying done.
func doneStep02BodyWithOpenItem(cfg config.Config) string {
	return openItemStep02Body(cfg, "done", "second thing")
}

// bareOpenItemStep02Body is step02BodyWithOpenItem with its open item
// carrying no text at all, landing at the same line 13.
func bareOpenItemStep02Body(cfg config.Config) string {
	return openItemStep02Body(cfg, "open", "")
}

func openItemStep02Body(cfg config.Config, status, item string) string {
	itemLine := "- [ ]"
	if item != "" {
		itemLine = "- [ ] " + item
	}

	return "---\n" +
		"id: STEP-02\n" +
		"status: " + status + "\n" +
		"depends-on: [STEP-01]\n" +
		"owner: planner\n" +
		"---\n" +
		"\n" +
		"# STEP-02 Assemble the thing\n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		"- [x] first thing\n" +
		itemLine + "\n" +
		"\n" +
		"## Fixture Handoff" + "\n"
}

func Test_finish_refuses_a_step_with_an_open_checklist_item(t *testing.T) {
	fx := newFinishFixtureFS(t)
	putStepFS(t, fx, "STEP-02.md", step02BodyWithOpenItem(fx.cfg))
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOpenChecklistItem)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, fx.stepPath("STEP-02.md"), refusal.Path)
	assert.Equal(t, 13, refusal.Line)
	assert.Equal(t,
		fx.stepPath("STEP-02.md")+
			`:13: checklist item "second thing" is not ticked; tick it with [x] once it is done, or remove it, and retry`,
		err.Error())

	assert.Equal(t, before, fx.mem.Snapshot())
}

// Stays on disk: exercises NewFeature, NewStep and Finish end to end, and
// asserts the four writes landed, so it cannot pass on a silent no-op.
func Test_finish_accepts_a_step_whose_checklist_is_empty(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	srv := scaffold.NewServer(cfg, root)

	featureRes, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	_, err = srv.NewStep(context.Background(), "widgets")
	require.NoError(t, err)

	pattern, err := stepfile.Compile(cfg.StepFilePattern)
	require.NoError(t, err)

	stateBody, err := os.ReadFile(filepath.Join(featureRes.Path, cfg.StateFile))
	require.NoError(t, err)

	id := pattern.ID(1)
	stepPath := filepath.Join(featureRes.Path, pattern.Name(1))

	_, err = srv.Finish(context.Background(), "widgets", id, []byte("handoff body\n"), stateBody)
	require.NoError(t, err)

	stepGot, readErr := os.ReadFile(stepPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(stepGot), "status: done")

	specGot, readErr := os.ReadFile(filepath.Join(featureRes.Path, cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(specGot), "- [x] "+id)

	stateGot, readErr := os.ReadFile(filepath.Join(featureRes.Path, cfg.StateFile))
	require.NoError(t, readErr)
	assert.Equal(t, string(stateBody), string(stateGot))

	handoffPath := filepath.Join(featureRes.Path, id+cfg.HandoffFileSuffix)
	handoffGot, readErr := os.ReadFile(handoffPath)
	require.NoError(t, readErr)
	assert.Equal(t, "handoff body\n", string(handoffGot))
}

func Test_finish_accepts_a_step_with_no_checklist_heading(t *testing.T) {
	fx := newFinishFixtureFS(t)
	noHeading := strings.Replace(step02BodyWithOpenItem(fx.cfg), fx.cfg.ChecklistHeading, "## Not The Checklist", 1)
	putStepFS(t, fx, "STEP-02.md", noHeading)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.NoError(t, err)
}

func Test_finish_ignores_an_unchecked_item_outside_the_checklist_section(t *testing.T) {
	fx := newFinishFixtureFS(t)
	body := "---\n" +
		"id: STEP-02\n" +
		"status: open\n" +
		"depends-on: [STEP-01]\n" +
		"owner: planner\n" +
		"---\n" +
		"\n" +
		"# STEP-02 Assemble the thing\n" +
		"\n" +
		fx.cfg.ChecklistHeading + "\n" +
		"\n" +
		"- [x] first thing\n" +
		"\n" +
		"## Fixture Handoff" + "\n" +
		"\n" +
		"## Notes\n" +
		"\n" +
		"- [ ] not this section's\n"
	putStepFS(t, fx, "STEP-02.md", body)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.NoError(t, err)
}

func Test_finish_reports_the_open_checklist_item_rather_than_a_missing_specification(t *testing.T) {
	fx := newFinishFixtureFS(t)
	putStepFS(t, fx, "STEP-02.md", step02BodyWithOpenItem(fx.cfg))
	require.NoError(t, fx.mem.Remove(fx.cfg.SpecificationFile))

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOpenChecklistItem)
	assert.NotErrorIs(t, err, scaffold.ErrMalformedFeature)
}

func Test_finish_reports_a_state_body_missing_a_heading_rather_than_the_open_checklist_item(t *testing.T) {
	fx := newFinishFixtureFS(t)
	putStepFS(t, fx, "STEP-02.md", step02BodyWithOpenItem(fx.cfg))
	missing := stateBodyMissingHeadings(fx.cfg, fx.cfg.StateHeadings.Traps)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, missing)

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)
	assert.NotErrorIs(t, err, scaffold.ErrOpenChecklistItem)
}

func Test_finish_reports_the_open_checklist_item_on_a_done_step_with_a_divergent_handoff(t *testing.T) {
	fx := newFinishedFixtureFS(t)
	putStepFS(t, fx, "STEP-02.md", doneStep02BodyWithOpenItem(fx.cfg))
	divergentHandoff := []byte("DIFFERENT-HANDOFF\n")

	_, err := fx.finish(t, "STEP-02", divergentHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOpenChecklistItem)
	assert.NotErrorIs(t, err, scaffold.ErrAlreadyFinished)
}

func Test_finish_refuses_a_bare_open_checklist_item_without_a_quoted_empty_string(t *testing.T) {
	fx := newFinishFixtureFS(t)
	putStepFS(t, fx, "STEP-02.md", bareOpenItemStep02Body(fx.cfg))

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOpenChecklistItem)
	assert.Equal(t,
		fx.stepPath("STEP-02.md")+
			`:13: checklist item is not ticked; tick it with [x] once it is done, or remove it, and retry`,
		err.Error())
}
