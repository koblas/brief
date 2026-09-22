package assemble_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_features_lists_directory_entries_only pins Features' population: a
// regular file and a symlink beside two real feature directories must not
// appear in the result, and the result carries fs.ReadDir's own order.
func Test_features_lists_directory_entries_only(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureRoot := filepath.Join(root, cfg.FeatureDirectory)

	require.NoError(t, os.MkdirAll(filepath.Join(featureRoot, "alpha"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(featureRoot, "beta"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureRoot, "README.md"), []byte("not a feature\n"), 0o600))

	outside := filepath.Join(root, "outside")
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(featureRoot, "linked")))

	srv := assemble.NewServer(cfg, root)

	names, err := srv.Features(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta"}, names)
}

// Test_features_returns_nil_when_the_feature_directory_is_missing pins the
// same "nothing to return is not an error" contract Status already keeps
// for a repository that has never run brief.
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
// repository, and must not be swallowed by the same ErrNotExist guard.
func Test_features_returns_an_error_when_the_feature_directory_is_a_regular_file(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, cfg.FeatureDirectory), []byte("not a directory"), 0o600))

	srv := assemble.NewServer(cfg, root)

	names, err := srv.Features(t.Context())

	require.Error(t, err)
	assert.Nil(t, names)
}
