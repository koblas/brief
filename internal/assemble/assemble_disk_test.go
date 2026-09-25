// This file keeps the scenarios whose subject is the OS adapter Start
// builds around one feature's own name, and the write-sweep proving Start
// touches no file on disk. StartFS's own content rules are pinned against
// fstest.MapFS in assemble_test.go.
package assemble_test

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFixture writes feature "demo" under a fresh temp root, using
// newFixtureFiles (assemble_test.go), and returns the root and its config.
func newFixture(t *testing.T) (string, config.Config) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	for name, body := range newFixtureFiles(cfg) {
		require.NoError(t, os.WriteFile(filepath.Join(featureDir, name), []byte(body), 0o600))
	}

	return root, cfg
}

func Test_returns_an_error_when_the_feature_does_not_exist(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "demo")

	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
}

func Test_returns_an_error_when_the_feature_name_escapes_the_feature_root(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "../escaped")

	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
}

// An empty feature argument is rejected before either OpenRoot call, not
// via its own opaque "empty path" failure.
func Test_returns_an_error_when_the_feature_name_is_empty(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "")

	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
}

// Covers Start's first OpenRoot call (the configured feature directory
// itself); the sibling test below covers the second.
func Test_start_reports_a_generic_failure_when_the_feature_root_itself_is_not_a_directory(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDirPath := filepath.Join(root, cfg.FeatureDirectory)
	require.NoError(t, os.WriteFile(featureDirPath, []byte("not a directory"), 0o600))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "demo")

	require.Error(t, err)
	require.NotErrorIs(t, err, assemble.ErrNoSuchFeature)
	assert.Contains(t, err.Error(), featureDirPath)
}

// Only a genuinely absent directory is ErrNoSuchFeature: a regular file
// standing where "demo"'s directory belongs must not read as "no such
// feature demo".
func Test_start_reports_a_generic_failure_for_an_unreadable_feature_entry(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, cfg.FeatureDirectory, "demo"), []byte("not a directory"), 0o600))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "demo")

	require.Error(t, err)
	assert.NotErrorIs(t, err, assemble.ErrNoSuchFeature)
}

// fileSnapshot is one file's identity for a disk-unchanged sweep. mtime is
// included because a write that reproduces identical bytes still touches it.
type fileSnapshot struct {
	sha256 [32]byte
	mode   os.FileMode
	mtime  time.Time
}

// snapshotTree walks every regular file under root and returns its
// fileSnapshot, keyed by path. Reads go through an *os.Root scoped to
// root, so a symlink swapped in mid-walk cannot redirect a read outside it.
func snapshotTree(t *testing.T, root string) map[string]fileSnapshot {
	t.Helper()

	r, err := os.OpenRoot(root)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	snap := map[string]fileSnapshot{}

	walkErr := fs.WalkDir(r.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err)

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		require.NoError(t, err)

		data, err := r.ReadFile(path)
		require.NoError(t, err)

		snap[path] = fileSnapshot{sha256: sha256.Sum256(data), mode: info.Mode(), mtime: info.ModTime()}

		return nil
	})
	require.NoError(t, walkErr)

	return snap
}

func Test_writes_nothing_to_disk(t *testing.T) {
	root, cfg := newFixture(t)
	srv := assemble.NewServer(cfg, root)

	before := snapshotTree(t, root)

	_, err := srv.Start(t.Context(), "demo")

	require.NoError(t, err)
	assert.Equal(t, before, snapshotTree(t, root))
}

// Control arm for the test above: proves snapshotTree detects a change.
func Test_the_disk_sweep_sees_a_write(t *testing.T) {
	root, _ := newFixture(t)

	before := snapshotTree(t, root)

	require.NoError(t, os.WriteFile(filepath.Join(root, "intruder.txt"), []byte("uninvited"), 0o600))

	assert.NotEqual(t, before, snapshotTree(t, root))
}
