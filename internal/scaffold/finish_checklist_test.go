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
// checklist's second item left unticked ("- [ ] second thing"), landing at
// a known line (13) so a refusal test can pin an exact line number.
func step02BodyWithOpenItem(cfg config.Config) string {
	return openItemStep02Body(cfg, "open", "second thing")
}

// doneStep02BodyWithOpenItem is step02BodyWithOpenItem with its frontmatter
// already saying done, so a test can pin the checklist check ahead of
// (refinish).verdict() on a step that would otherwise take the re-finish
// path.
func doneStep02BodyWithOpenItem(cfg config.Config) string {
	return openItemStep02Body(cfg, "done", "second thing")
}

// bareOpenItemStep02Body is step02BodyWithOpenItem with its open item
// carrying no text at all ("- [ ]" alone), landing at the same line 13, so
// a test can pin the refusal copy's empty-text branch (no "%q").
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

// Test_finish_refuses_a_step_with_an_open_checklist_item pins SCENARIO-20's
// refusal directly against step02BodyWithOpenItem's own line 13, and pairs
// the byte-identity and mtime probes newFinishFixture's other refusal
// tests use, proving nothing landed.
func Test_finish_refuses_a_step_with_an_open_checklist_item(t *testing.T) {
	fx := newFinishFixture(t)
	require.NoError(t, os.WriteFile(fx.stepPath("STEP-02.md"), []byte(step02BodyWithOpenItem(fx.cfg)), 0o600))
	names := []string{"STEP-02.md", fx.cfg.StateFile, fx.cfg.SpecificationFile}
	pinModTimes(t, fx.featureDir(), names, pinnedModTime)
	before := snapshotTree(t, fx.featureDir())
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOpenChecklistItem)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, fx.stepPath("STEP-02.md"), refusal.Path)
	assert.Equal(t, 13, refusal.Line)
	assert.Equal(t,
		fx.stepPath("STEP-02.md")+
			`:13: checklist item "second thing" is not ticked; tick it with [x] once it is done, or remove it, and retry`,
		err.Error())

	assert.Equal(t, before, snapshotTree(t, fx.featureDir()))

	after := modTimes(t, fx.featureDir(), names)
	for _, name := range names {
		assert.True(t, after[name].Equal(pinnedModTime), "%s mtime moved on a refused open-checklist-item finish", name)
	}
}

// Test_finish_accepts_a_step_whose_checklist_is_empty is the `new step` ->
// `finish` path (measured case (a)): NewStep writes a bare checklist
// heading with nothing under it, and that must keep succeeding, not start
// refusing on its very first run. It asserts the four writes actually
// landed, so it cannot pass vacuously on a call that silently no-ops.
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

	err = srv.Finish(context.Background(), "widgets", id, []byte("handoff body\n"), stateBody)
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

// Test_finish_accepts_a_step_with_no_checklist_heading covers 14's degrade
// rule at the write side: a step file with no line matching the
// configured checklist heading at all is accepted, matching
// assemble.Start's read-side Section.Found == false degrade.
func Test_finish_accepts_a_step_with_no_checklist_heading(t *testing.T) {
	fx := newFinishFixture(t)
	noHeading := strings.Replace(step02BodyWithOpenItem(fx.cfg), fx.cfg.ChecklistHeading, "## Not The Checklist", 1)
	require.NoError(t, os.WriteFile(fx.stepPath("STEP-02.md"), []byte(noHeading), 0o600))
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)

	require.NoError(t, err)
}

// Test_finish_ignores_an_unchecked_item_outside_the_checklist_section pins
// the section boundary at the Finish level: an open item under a
// following heading of the same level is not part of the checklist
// section, matching markdown.Section's own same-or-higher-level stop.
func Test_finish_ignores_an_unchecked_item_outside_the_checklist_section(t *testing.T) {
	fx := newFinishFixture(t)
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
	require.NoError(t, os.WriteFile(fx.stepPath("STEP-02.md"), []byte(body), 0o600))
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)

	require.NoError(t, err)
}

// Test_finish_reports_the_open_checklist_item_rather_than_a_missing_specification
// pins the checklist check ahead of the specification read: a step with an
// open item, in a feature whose specification.md has been deleted,
// reports the open item, not the missing specification.
func Test_finish_reports_the_open_checklist_item_rather_than_a_missing_specification(t *testing.T) {
	fx := newFinishFixture(t)
	require.NoError(t, os.WriteFile(fx.stepPath("STEP-02.md"), []byte(step02BodyWithOpenItem(fx.cfg)), 0o600))
	require.NoError(t, os.Remove(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile)))
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOpenChecklistItem)
	assert.NotErrorIs(t, err, scaffold.ErrMalformedFeature)
}

// Test_finish_reports_a_state_body_missing_a_heading_rather_than_the_open_checklist_item
// pins the checklist check after the argument band: a step with an open
// item, given a state body missing a configured heading, reports the
// missing heading, not the open item — checkArgumentHeadings runs first.
func Test_finish_reports_a_state_body_missing_a_heading_rather_than_the_open_checklist_item(t *testing.T) {
	fx := newFinishFixture(t)
	require.NoError(t, os.WriteFile(fx.stepPath("STEP-02.md"), []byte(step02BodyWithOpenItem(fx.cfg)), 0o600))
	missing := stateBodyMissingHeadings(fx.cfg, fx.cfg.StateHeadings.Traps)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, missing)

	require.ErrorIs(t, err, scaffold.ErrMissingStateHeading)
	assert.NotErrorIs(t, err, scaffold.ErrOpenChecklistItem)
}

// Test_finish_reports_the_open_checklist_item_on_a_done_step_with_a_divergent_handoff
// pins the checklist check ahead of (refinish).verdict(): a done step with
// an open item, given a handoff that differs from the recorded one,
// reports the open item, never ErrAlreadyFinished.
func Test_finish_reports_the_open_checklist_item_on_a_done_step_with_a_divergent_handoff(t *testing.T) {
	fx := newFinishedFixture(t)
	require.NoError(t, os.WriteFile(fx.stepPath("STEP-02.md"), []byte(doneStep02BodyWithOpenItem(fx.cfg)), 0o600))
	srv := scaffold.NewServer(fx.cfg, fx.root)
	divergentHandoff := []byte("DIFFERENT-HANDOFF\n")

	err := srv.Finish(context.Background(), "widgets", "STEP-02", divergentHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOpenChecklistItem)
	assert.NotErrorIs(t, err, scaffold.ErrAlreadyFinished)
}

// Test_finish_refuses_a_bare_open_checklist_item_without_a_quoted_empty_string
// pins decision 5's empty-text branch: an item with no text at all ("- [ ]"
// alone) drops the "%q" from the refusal copy rather than naming an empty
// quoted string.
func Test_finish_refuses_a_bare_open_checklist_item_without_a_quoted_empty_string(t *testing.T) {
	fx := newFinishFixture(t)
	require.NoError(t, os.WriteFile(fx.stepPath("STEP-02.md"), []byte(bareOpenItemStep02Body(fx.cfg)), 0o600))
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrOpenChecklistItem)
	assert.Equal(t,
		fx.stepPath("STEP-02.md")+
			`:13: checklist item is not ticked; tick it with [x] once it is done, or remove it, and retry`,
		err.Error())
}
