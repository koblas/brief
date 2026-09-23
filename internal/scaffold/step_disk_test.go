package scaffold_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_reports_a_specification_write_that_cannot_be_committed_on_new_step
// covers NewStep's own specification write, which shares replaceString
// with Finish: a directory planted at the specification's own temp
// sibling blocks atomicfile's rename outright, the same real-filesystem
// mechanism Finish's own three blocked writes exercise in
// finish_disk_test.go — rwfs.Mem's WriteFile is a single in-memory
// assignment with no temp file or rename to block, so this case has no
// Mem equivalent. The step file is written before the specification
// (scaffold.go's NewStep doc), so the failure this induces is expected to
// leave an orphan step file behind — visible and repairable — rather than
// losing it.
func Test_reports_a_specification_write_that_cannot_be_committed_on_new_step(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)
	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	blocked := filepath.Join(featureDir, "."+cfg.SpecificationFile+".brief-tmp")
	require.NoError(t, os.Mkdir(blocked, 0o755))

	_, err = srv.NewStep(context.Background(), "widgets")

	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrPartialWrite,
		"the step file ahead of the blocked specification write already landed")
	assert.FileExists(t, filepath.Join(featureDir, "STEP-01.md"),
		"the step file lands before the specification write, so it must survive the specification write's failure")
}

// Test_refuses_an_unknown_feature pins openFeatureDir's own
// ErrNoSuchFeature for NewStep, the same way
// Test_refuses_an_unknown_feature_on_finish does for Finish: "widgets" has
// no directory at all, decided entirely by the entry point's open chain
// before NewStepFS is ever reached.
func Test_refuses_an_unknown_feature(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewStep(context.Background(), "widgets")

	require.ErrorIs(t, err, scaffold.ErrNoSuchFeature)
	assert.NoDirExists(t, filepath.Join(root, "specs", "widgets"))
}

func Test_refuses_a_feature_name_that_escapes_the_feature_root(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewStep(context.Background(), "../escaped")

	require.Error(t, err)
	assert.NoFileExists(t, filepath.Join(root, "escaped"))
}

// Test_refuses_an_empty_feature_argument pins that an empty feature
// argument is refused the same way a traversal attempt is —
// validFeatureArgument rejects it before openFeatureDir's first open.
func Test_refuses_an_empty_feature_argument(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewStep(context.Background(), "")

	require.ErrorIs(t, err, scaffold.ErrNoSuchFeature)
}

// Test_refuses_an_invalid_step_file_pattern_and_writes_nothing stays on
// disk: stepfile.Compile runs inside the NewStep entry point itself,
// before openFeatureDir or any filesystem call, the same way
// validateFeatureName does for NewFeature — there is no FS-taking core to
// redirect this onto rwfs.Mem for, only the real entry point, and its own
// "writes nothing" claim is a claim about the real feature directory.
func Test_refuses_an_invalid_step_file_pattern_and_writes_nothing(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%s.md"
	srv := scaffold.NewServer(cfg, root)
	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	_, err = srv.NewStep(context.Background(), "widgets")

	require.ErrorIs(t, err, stepfile.ErrInvalidPattern)

	entries, readErr := os.ReadDir(filepath.Join(root, "specs", "widgets"))
	require.NoError(t, readErr)
	assert.ElementsMatch(t, []string{cfg.SpecificationFile, cfg.StateFile}, namesOf(entries))
}
