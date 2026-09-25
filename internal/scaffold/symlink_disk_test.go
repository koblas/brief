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

// This file proves the one guarantee that is OS-only and unreproducible on
// rwfs.Mem: os.Root refuses a name, or a symbolic link, that would resolve
// outside its own root. Mem's own OpenRoot has no notion of "outside" and
// would follow such a symlink wherever it points.

func Test_new_feature_refuses_when_the_feature_entry_is_a_symlink(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	outsideDir := filepath.Join(root, "outside-widgets")
	require.NoError(t, os.MkdirAll(outsideDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	require.NoError(t, os.Symlink(filepath.Join("..", "outside-widgets"), filepath.Join(root, cfg.FeatureDirectory, "widgets")))
	before := snapshotTree(t, outsideDir)

	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewFeature(context.Background(), "widgets")

	require.Error(t, err)
	assert.Equal(t, before, snapshotTree(t, outsideDir), "nothing must be written through the symlink's target")
}

// The outside directory is itself a conforming feature — a real NewStep
// against it directly would succeed — so a refusal here can only be
// explained by the symlink being refused, not the target being unusable.
func Test_new_step_refuses_a_feature_entry_that_symlinks_outside_the_feature_directory(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	outsideDir := filepath.Join(root, "outside-widgets")
	require.NoError(t, os.MkdirAll(outsideDir, 0o755))
	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, cfg.SpecificationFile), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, cfg.StateFile), []byte(""), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	require.NoError(t, os.Symlink(filepath.Join("..", "outside-widgets"), filepath.Join(root, cfg.FeatureDirectory, "widgets")))
	before := snapshotTree(t, outsideDir)

	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewStep(context.Background(), "widgets")

	require.Error(t, err)
	assert.Equal(t, before, snapshotTree(t, outsideDir), "nothing must be written through the symlink's target")
}

func Test_finish_refuses_a_feature_entry_that_symlinks_outside_the_feature_directory(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	outsideDir := filepath.Join(root, "outside-widgets")
	require.NoError(t, os.MkdirAll(outsideDir, 0o755))
	step := "---\nid: STEP-01\nstatus: open\ndepends-on: []\n---\n\n# STEP-01\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n"
	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n"
	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "STEP-01.md"), []byte(step), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, cfg.SpecificationFile), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, cfg.StateFile), []byte(state), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	require.NoError(t, os.Symlink(filepath.Join("..", "outside-widgets"), filepath.Join(root, cfg.FeatureDirectory, "widgets")))
	before := snapshotTree(t, outsideDir)

	srv := scaffold.NewServer(cfg, root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-01", []byte("h"), []byte(state))

	require.Error(t, err)
	assert.Equal(t, before, snapshotTree(t, outsideDir), "nothing must be written through the symlink's target")
}

// Control arm for both symlink refusals above: the same fixture shape,
// with the symlink's target moved inside the configured feature directory,
// must now be followed — os.Root refuses only a resolution that would
// leave its own root, never a symlink as such.
func Test_new_step_follows_a_feature_entry_that_symlinks_inside_the_feature_directory(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	realDir := filepath.Join(root, cfg.FeatureDirectory, "real-widgets")
	require.NoError(t, os.MkdirAll(realDir, 0o755))
	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(realDir, cfg.SpecificationFile), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(realDir, cfg.StateFile), []byte(""), 0o600))
	// Relative target with no "..", so it resolves inside the feature directory.
	require.NoError(t, os.Symlink("real-widgets", filepath.Join(root, cfg.FeatureDirectory, "widgets")))

	srv := scaffold.NewServer(cfg, root)

	res, err := srv.NewStep(context.Background(), "widgets")

	require.NoError(t, err)
	got, readErr := os.ReadFile(filepath.Join(realDir, "STEP-01.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(got), "id: STEP-01", "the step file must land through the symlink, inside the feature directory")
	assert.Equal(t, filepath.Join(root, cfg.FeatureDirectory, "widgets", "STEP-01.md"), res.Path)
}
