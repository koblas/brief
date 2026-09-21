package scaffold_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_finish_reports_the_absolute_handoff_and_state_paths_changed_true_and_next
// pins FinishResult's shape on a first, writing finish against
// newFinishFixture: STEP-03 is the lowest-numbered step still open once
// STEP-02 is counted done, so it is Next — depends-on is never consulted.
func Test_finish_reports_the_absolute_handoff_and_state_paths_changed_true_and_next(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	res, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)

	require.NoError(t, err)
	assert.Equal(t, "widgets", res.Feature)
	assert.Equal(t, "STEP-02", res.Step)
	assert.True(t, res.Changed)
	assert.Equal(t, fx.handoffPath(), res.HandoffPath)
	assert.Equal(t, filepath.Join(fx.featureDir(), fx.cfg.StateFile), res.StatePath)
	assert.Equal(t, "STEP-03", res.Next)
}

// Test_finish_no_op_reports_changed_false_and_still_names_next proves the
// R11 no-op returns the populated result rather than a zero FinishResult:
// Changed flips to false, but Next is still computed.
func Test_finish_no_op_reports_changed_false_and_still_names_next(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	first, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)
	require.True(t, first.Changed)

	second, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)

	require.NoError(t, err)
	assert.False(t, second.Changed)
	assert.Equal(t, "STEP-03", second.Next)
}

// Test_finish_next_is_empty_when_every_other_step_is_done builds a
// two-step feature — STEP-01 already done, STEP-02 the step under test —
// so finishing STEP-02 leaves nothing open.
func Test_finish_next_is_empty_when_every_other_step_is_done(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step01 := "---\nid: STEP-01\nstatus: done\ndepends-on: []\n---\n\n" +
		"# STEP-01\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"), []byte(step01), 0o600))

	step02 := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-02.md"), []byte(step02), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [x] STEP-01\n- [ ] STEP-02\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	state := oldStateBody(cfg)
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))

	srv := scaffold.NewServer(cfg, root)

	res, err := srv.Finish(context.Background(), "widgets", "STEP-02", []byte("h"), newStateBody(cfg))

	require.NoError(t, err)
	assert.Empty(t, res.Next)
}

// Test_finish_names_a_sibling_with_unparseable_frontmatter_as_next builds a
// two-step feature whose second step's frontmatter does not parse — the
// same defect siblingFrontmatter tolerates for checkStepDependencies — and
// finishes the first: the unparseable sibling counts as not done, so it is
// named Next.
func Test_finish_names_a_sibling_with_unparseable_frontmatter_as_next(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step01 := "---\nid: STEP-01\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-01\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"), []byte(step01), 0o600))

	step02 := "---\nid: [unterminated\n---\n\nbroken frontmatter\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-02.md"), []byte(step02), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n- [ ] STEP-02\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	state := oldStateBody(cfg)
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))

	srv := scaffold.NewServer(cfg, root)

	res, err := srv.Finish(context.Background(), "widgets", "STEP-01", []byte("h"), newStateBody(cfg))

	require.NoError(t, err)
	assert.Equal(t, "STEP-02", res.Next)
}
