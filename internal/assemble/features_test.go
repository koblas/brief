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

// Real disk: the subject is Features' own os.Root.OpenRoot adapter, not FeaturesFS.
func Test_features_returns_nil_when_the_feature_directory_is_missing(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	srv := assemble.NewServer(cfg, root)

	names, err := srv.Features(t.Context())

	require.NoError(t, err)
	assert.Nil(t, names)
}

// Control for the test above: a regular file must not be swallowed by the
// same ErrNotExist guard. Real disk, same adapter-level reason.
func Test_features_returns_an_error_when_the_feature_directory_is_a_regular_file(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, cfg.FeatureDirectory), []byte("not a directory"), 0o600))

	srv := assemble.NewServer(cfg, root)

	names, err := srv.Features(t.Context())

	require.Error(t, err)
	assert.Nil(t, names)
}
