// This file keeps the assemble_test.go scenarios whose subject is the OS
// adapter Start builds around one feature's own name — the os.Root
// containment chain (an escaping or empty feature argument, a regular file
// standing where a directory belongs) and the write-sweep control that
// proves Start touches no file on disk, a claim an in-memory fs.FS cannot
// make since it has no write method to omit calling. StartFS's own content
// rules are pinned against fstest.MapFS in assemble_test.go.
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
// fixtureConfig(): the same five step files, NOTES.md and SPEC.md
// newFixtureFiles (assemble_test.go) builds in memory. It returns the root
// and the config the fixture was written with.
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

// Test_returns_an_error_when_the_feature_name_is_empty pins that an empty
// feature argument is refused the same way a traversal attempt is —
// validFeatureArgument rejects it before either os.Root.OpenRoot call —
// rather than reaching topRoot.OpenRoot("") and surfacing its own opaque
// "empty path" failure, which names no feature and suggests no fix.
func Test_returns_an_error_when_the_feature_name_is_empty(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "")

	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
}

// Test_start_reports_a_generic_failure_when_the_feature_root_itself_is_not_a_directory
// covers Start's first os.Root.OpenRoot call — the configured feature
// directory itself, not feature's own subdirectory — the same way
// Test_start_reports_a_generic_failure_for_an_unreadable_feature_entry
// already covers the second: a regular file standing where the configured
// feature directory belongs must fail generically, carrying that path in
// its message, never as ErrNoSuchFeature.
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

// Test_start_reports_a_generic_failure_for_an_unreadable_feature_entry pins
// that only a genuinely absent directory is ErrNoSuchFeature: a feature
// entry that exists but cannot be opened as a
// directory — here, a regular file standing where "demo"'s directory
// belongs — must not read as "no such feature demo", since demo plainly
// does exist. The portable, privilege-independent substitute for a
// permission failure is the same technique
// Test_status_propagates_a_feature_root_that_is_not_a_directory already
// uses: a regular file makes os.Root.OpenRoot fail with "not a directory",
// never fs.ErrNotExist.
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

// fileSnapshot is one file's identity for a disk-unchanged sweep: its
// content hash, its permission bits and its modification time. mtime is
// included because a write that reproduces identical bytes still touches
// mtime, and a sweep that only hashed content would miss that.
type fileSnapshot struct {
	sha256 [32]byte
	mode   os.FileMode
	mtime  time.Time
}

// snapshotTree walks every regular file under root and returns its
// fileSnapshot, keyed by its path relative to root. Reads go through an
// *os.Root scoped to root rather than an absolute path built from the
// walk callback, so a symlink swapped in mid-walk cannot redirect a read
// outside root.
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

// Test_the_disk_sweep_sees_a_write is the control arm for the test above:
// it proves snapshotTree actually detects a change, so the previous
// test's equal snapshots are evidence Start wrote nothing rather than
// evidence the sweep cannot see a write at all.
func Test_the_disk_sweep_sees_a_write(t *testing.T) {
	root, _ := newFixture(t)

	before := snapshotTree(t, root)

	require.NoError(t, os.WriteFile(filepath.Join(root, "intruder.txt"), []byte("uninvited"), 0o600))

	assert.NotEqual(t, before, snapshotTree(t, root))
}
