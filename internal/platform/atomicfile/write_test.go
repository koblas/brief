package atomicfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/atomicfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openRoot returns an *os.Root rooted at a fresh temporary directory.
func openRoot(t *testing.T) (*os.Root, string) {
	t.Helper()

	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })

	return root, dir
}

func Test_writes_a_new_files_content(t *testing.T) {
	root, dir := openRoot(t)

	err := atomicfile.WriteFile(root, "target.txt", []byte("hello"), 0o644)

	require.NoError(t, err)
	got, readErr := os.ReadFile(filepath.Join(dir, "target.txt"))
	require.NoError(t, readErr)
	assert.Equal(t, "hello", string(got))
}

func Test_replaces_an_existing_files_content(t *testing.T) {
	root, dir := openRoot(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "target.txt"), []byte("old"), 0o600))

	err := atomicfile.WriteFile(root, "target.txt", []byte("new"), 0o644)

	require.NoError(t, err)
	got, readErr := os.ReadFile(filepath.Join(dir, "target.txt"))
	require.NoError(t, readErr)
	assert.Equal(t, "new", string(got))
}

func Test_after_a_successful_write_the_directory_holds_exactly_the_target(t *testing.T) {
	root, dir := openRoot(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "target.txt"), []byte("old"), 0o600))

	err := atomicfile.WriteFile(root, "target.txt", []byte("new"), 0o644)
	require.NoError(t, err)

	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}

	assert.ElementsMatch(t, []string{"target.txt"}, names)
}

func Test_keeps_the_existing_files_permissions_when_it_replaces_it(t *testing.T) {
	root, dir := openRoot(t)
	path := filepath.Join(dir, "target.txt")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o400))

	err := atomicfile.WriteFile(root, "target.txt", []byte("new"), 0o600)

	require.NoError(t, err)
	info, statErr := os.Stat(path)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o400), info.Mode().Perm())
}

func Test_leaves_the_target_unchanged_and_no_temp_file_when_the_rename_cannot_land(t *testing.T) {
	root, dir := openRoot(t)
	targetDir := filepath.Join(dir, "target.txt")
	require.NoError(t, os.Mkdir(targetDir, 0o755))
	sentinel := filepath.Join(targetDir, "sentinel.txt")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep me"), 0o600))

	err := atomicfile.WriteFile(root, "target.txt", []byte("new"), 0o644)

	require.Error(t, err)
	assert.FileExists(t, sentinel)

	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}

	assert.ElementsMatch(t, []string{"target.txt"}, names)
}
