// FeatureContaining's every test here runs against real directories: its
// whole contract is os.Lstat and filepath.EvalSymlinks behavior — a
// symlinked ancestor of the project root, a "." or ".." escape, a regular
// file standing where a feature directory belongs — which an in-memory
// fs.FS cannot reproduce, since FeatureContaining never takes an fs.FS in
// the first place (it reads the OS path space directly, not through
// Server's FeatureFS seam).
package assemble_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newFeatureContainingServer returns a Server rooted at root, using
// config.Default() — FeatureContaining never reads any other configured
// value.
func newFeatureContainingServer(root string) *assemble.Server {
	return assemble.NewServer(config.Default(), root)
}

func Test_FeatureContaining_ReturnsTheFeatureNameForAPathInsideIt(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "docs", "specifications", "auth")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	specPath := filepath.Join(featureDir, "specification.md")
	require.NoError(t, os.WriteFile(specPath, []byte("x"), 0o600))

	srv := newFeatureContainingServer(root)

	name, ok := srv.FeatureContaining(specPath)

	require.True(t, ok)
	assert.Equal(t, "auth", name)
}

func Test_FeatureContaining_ReturnsFalseForARelativePath(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "docs", "specifications", "auth")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte("x"), 0o600))

	srv := newFeatureContainingServer(root)

	_, ok := srv.FeatureContaining("docs/specifications/auth/specification.md")

	assert.False(t, ok)
}

func Test_FeatureContaining_ReturnsFalseForAPathOutsideTheRoot(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "specifications", "auth"), 0o755))

	other := t.TempDir()
	outsidePath := filepath.Join(other, "notes.md")
	require.NoError(t, os.WriteFile(outsidePath, []byte("x"), 0o600))

	srv := newFeatureContainingServer(root)

	_, ok := srv.FeatureContaining(outsidePath)

	assert.False(t, ok)
}

func Test_FeatureContaining_ReturnsFalseForADotDotEscape(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "docs", "specifications", "auth")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "outside.md"), []byte("x"), 0o600))

	srv := newFeatureContainingServer(root)

	escaped := featureDir + "/../../../outside.md"

	_, ok := srv.FeatureContaining(escaped)

	assert.False(t, ok)
}

func Test_FeatureContaining_ReturnsFalseForTheFeatureRootItself(t *testing.T) {
	root := t.TempDir()
	featureRoot := filepath.Join(root, "docs", "specifications")
	require.NoError(t, os.MkdirAll(filepath.Join(featureRoot, "auth"), 0o755))

	srv := newFeatureContainingServer(root)

	_, ok := srv.FeatureContaining(featureRoot)

	assert.False(t, ok)
}

func Test_FeatureContaining_ReturnsFalseForAFileDirectlyInTheFeatureRoot(t *testing.T) {
	root := t.TempDir()
	featureRoot := filepath.Join(root, "docs", "specifications")
	require.NoError(t, os.MkdirAll(featureRoot, 0o755))

	filePath := filepath.Join(featureRoot, "notes.md")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o600))

	srv := newFeatureContainingServer(root)

	_, ok := srv.FeatureContaining(filePath)

	assert.False(t, ok)
}

func Test_FeatureContaining_ReturnsFalseWhenTheFirstPathElementIsARegularFile(t *testing.T) {
	root := t.TempDir()
	featureRoot := filepath.Join(root, "docs", "specifications")
	require.NoError(t, os.MkdirAll(featureRoot, 0o755))

	// "auth" is a regular file, not a feature directory, so a path
	// underneath it (as a string) still must not resolve to a feature.
	require.NoError(t, os.WriteFile(filepath.Join(featureRoot, "auth"), []byte("x"), 0o600))

	srv := newFeatureContainingServer(root)

	_, ok := srv.FeatureContaining(filepath.Join(featureRoot, "auth", "specification.md"))

	assert.False(t, ok)
}

// Test_FeatureContaining_ReturnsTheFeatureNameThroughASymlinkedAncestor
// pins the macOS trap: t.TempDir() returns a path under "/var/folders/…",
// itself a symlink to "/private/var/…". A hook payload path arriving
// already resolved through the symlink (as filepath.EvalSymlinks would
// leave it) must still match a Server rooted at the unresolved form.
func Test_FeatureContaining_ReturnsTheFeatureNameThroughASymlinkedAncestor(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "docs", "specifications", "auth")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	specPath := filepath.Join(featureDir, "specification.md")
	require.NoError(t, os.WriteFile(specPath, []byte("x"), 0o600))

	resolvedSpecPath, err := filepath.EvalSymlinks(specPath)
	require.NoError(t, err)

	srv := newFeatureContainingServer(root)

	name, ok := srv.FeatureContaining(resolvedSpecPath)

	require.True(t, ok)
	assert.Equal(t, "auth", name)
}
