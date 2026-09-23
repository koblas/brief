package assemble_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/assemble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_features_lists_directory_entries_only pins FeaturesFS's population: a
// regular file and a symlink beside two real feature directories must not
// appear in the result, and the result carries fs.ReadDir's own order.
// fstest.MapFS represents a symlink entry a DirEntry's Type() can report,
// the same fs.ModeSymlink bit os.Root.FS() (Features' own backing FS)
// reports for one, so this reaches the same branch either way.
func Test_features_lists_directory_entries_only(t *testing.T) {
	fsys := fstest.MapFS{
		"alpha/STEP-01.md": &fstest.MapFile{Data: []byte("x")},
		"beta/STEP-01.md":  &fstest.MapFile{Data: []byte("x")},
		"README.md":        &fstest.MapFile{Data: []byte("not a feature\n")},
		"linked":           &fstest.MapFile{Mode: fs.ModeSymlink, Data: []byte("outside")},
	}

	names, err := assemble.FeaturesFS(fsys)

	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta"}, names)
}

// Test_features_returns_nil_when_the_feature_directory_is_missing pins the
// same "nothing to return is not an error" contract Status already keeps
// for a repository that has never run brief: real disk, since the subject
// is Features' own os.Root.OpenRoot adapter, not FeaturesFS.
func Test_features_returns_nil_when_the_feature_directory_is_missing(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	srv := assemble.NewServer(cfg, root)

	names, err := srv.Features(t.Context())

	require.NoError(t, err)
	assert.Nil(t, names)
}

// Test_features_returns_an_error_when_the_feature_directory_is_a_regular_file
// is the control for the test above: a configured feature-directory path
// that exists but is not a directory is a misconfiguration, not an empty
// repository, and must not be swallowed by the same ErrNotExist guard —
// real disk, the same adapter-level reason as the test above.
func Test_features_returns_an_error_when_the_feature_directory_is_a_regular_file(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, cfg.FeatureDirectory), []byte("not a directory"), 0o600))

	srv := assemble.NewServer(cfg, root)

	names, err := srv.Features(t.Context())

	require.Error(t, err)
	assert.Nil(t, names)
}
