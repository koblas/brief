package scaffold_test

import (
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_finish_reports_the_absolute_handoff_and_state_paths_changed_true_and_next(t *testing.T) {
	fx := newFinishFixtureFS(t)

	res, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.NoError(t, err)
	assert.Equal(t, "widgets", res.Feature)
	assert.Equal(t, "STEP-02", res.Step)
	assert.True(t, res.Changed)
	assert.Equal(t, fx.handoffPath(), res.HandoffPath)
	assert.Equal(t, filepath.Join(testFeaturePath, fx.cfg.StateFile), res.StatePath)
	assert.Equal(t, scaffold.FinishNext{ID: "STEP-03", Title: "STEP-03", Path: filepath.Join(testFeaturePath, "STEP-03.md")}, res.Next)
	assert.NotNil(t, res.Dropped, "Dropped must be a non-nil empty slice, never nil, when nothing was dropped")
	assert.Empty(t, res.Dropped)
}

func Test_finish_no_op_reports_changed_false_and_still_names_next(t *testing.T) {
	fx := newFinishFixtureFS(t)

	first, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)
	require.True(t, first.Changed)

	second, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.NoError(t, err)
	assert.False(t, second.Changed)
	assert.Equal(t, scaffold.FinishNext{ID: "STEP-03", Title: "STEP-03", Path: filepath.Join(testFeaturePath, "STEP-03.md")}, second.Next)
	assert.NotNil(t, second.Dropped, "re-finishing with identical inputs must still report a non-nil empty Dropped")
	assert.Empty(t, second.Dropped)
}

func Test_finish_next_is_empty_when_every_other_step_is_done(t *testing.T) {
	cfg := fixtureConfig()
	pattern, patternErr := stepfilePattern(cfg)
	require.NoError(t, patternErr)
	handoffPattern, handoffPatternErr := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, handoffPatternErr)

	step01 := "---\nid: STEP-01\nstatus: done\ndepends-on: []\n---\n\n" +
		"# STEP-01\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n"
	step02 := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n"
	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [x] STEP-01\n- [ ] STEP-02\n"

	mem := rwfs.NewMem(fstest.MapFS{
		"STEP-01.md":          &fstest.MapFile{Data: []byte(step01), Mode: 0o600},
		"STEP-02.md":          &fstest.MapFile{Data: []byte(step02), Mode: 0o600},
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte(spec), Mode: 0o600},
		cfg.StateFile:         &fstest.MapFile{Data: []byte(oldStateBody(cfg)), Mode: 0o600},
	})

	srv := scaffold.NewServer(cfg, "")

	res, err := srv.FinishFS(mem, testFeaturePath, "widgets", "STEP-02", []byte("h"), newStateBody(cfg), pattern, handoffPattern)

	require.NoError(t, err)
	assert.Equal(t, scaffold.FinishNext{}, res.Next)
}

func Test_finish_names_a_sibling_with_unparseable_frontmatter_as_next(t *testing.T) {
	cfg := fixtureConfig()
	pattern, patternErr := stepfilePattern(cfg)
	require.NoError(t, patternErr)
	handoffPattern, handoffPatternErr := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, handoffPatternErr)

	step01 := "---\nid: STEP-01\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-01\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n"
	step02 := "---\nid: [unterminated\n---\n\nbroken frontmatter\n"
	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n- [ ] STEP-02\n"

	mem := rwfs.NewMem(fstest.MapFS{
		"STEP-01.md":          &fstest.MapFile{Data: []byte(step01), Mode: 0o600},
		"STEP-02.md":          &fstest.MapFile{Data: []byte(step02), Mode: 0o600},
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte(spec), Mode: 0o600},
		cfg.StateFile:         &fstest.MapFile{Data: []byte(oldStateBody(cfg)), Mode: 0o600},
	})

	srv := scaffold.NewServer(cfg, "")

	res, err := srv.FinishFS(mem, testFeaturePath, "widgets", "STEP-01", []byte("h"), newStateBody(cfg), pattern, handoffPattern)

	require.NoError(t, err)
	assert.Equal(t, scaffold.FinishNext{ID: "STEP-02", Title: "", Path: filepath.Join(testFeaturePath, "STEP-02.md")}, res.Next)
}
