package setup_test

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// treeEntry is one snapshotTree entry: isDir alone for a directory, body
// for a regular file's exact bytes.
type treeEntry struct {
	isDir bool
	body  []byte
}

// snapshotTree walks every path under root (root itself excluded), keyed
// by its path relative to root, recording whether it is a directory or a
// regular file's own bytes. It walks through an os.Root scoped to root
// rather than raw path-joined os.ReadFile calls, so every read stays
// confined to that directory tree.
func snapshotTree(t *testing.T, root string) map[string]treeEntry {
	t.Helper()

	r, err := os.OpenRoot(root)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	fsys := r.FS()
	out := map[string]treeEntry{}

	walkErr := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, entryErr error) error {
		require.NoError(t, entryErr)

		if p == "." {
			return nil
		}

		if d.IsDir() {
			out[p] = treeEntry{isDir: true}

			return nil
		}

		body, readErr := fs.ReadFile(fsys, p)
		require.NoError(t, readErr)

		out[p] = treeEntry{body: body}

		return nil
	})
	require.NoError(t, walkErr)

	return out
}

// Test_init_then_uninstall_leaves_the_tree_as_before_except_the_feature_root
// pins R6's own asymmetry end to end: against a repository already
// carrying unrelated files (a root README, a nested source file, a
// ".claude/" file), Init then Uninstall (default, unedited config)
// reproduces the exact pre-existing tree, plus exactly the empty feature
// root directory Init created and Uninstall deliberately never removes —
// asserted present, not merely tolerated if it happened to still be there.
func Test_init_then_uninstall_leaves_the_tree_as_before_except_the_feature_root(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, "README.md"), []byte("hello\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "src", "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wd, "src", "pkg", "main.go"), []byte("package pkg\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, ".claude"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude", "notes.md"), []byte("notes\n"), 0o600))

	before := snapshotTree(t, wd)

	srv := setup.NewServer()
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	_, err = srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})
	require.NoError(t, err)

	after := snapshotTree(t, wd)

	featureRootRel := filepath.Join("docs", "specifications")

	expected := map[string]treeEntry{}
	maps.Copy(expected, before)
	expected["docs"] = treeEntry{isDir: true}
	expected[featureRootRel] = treeEntry{isDir: true}

	assert.Equal(t, expected, after)

	entry, ok := after[featureRootRel]
	require.True(t, ok, "feature root must survive uninstall")
	assert.True(t, entry.isDir)
}
