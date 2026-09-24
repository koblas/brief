package repo_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fsAbs joins slash-separated segments under "/", the way every RootFS
// test names an absolute path its fstest.MapFS fixture is keyed against
// (RootFS's own fsName strips the leading "/" to get the fs.FS-relative
// name back).
func fsAbs(elem ...string) string {
	return filepath.FromSlash("/" + filepath.ToSlash(filepath.Join(elem...)))
}

// Test_RootFS_finds_a_git_directory_walking_up pins RootFS's own walk: a
// ".git" directory several levels below itself is found from a deeper
// start directory, reporting the directory that holds it.
func Test_RootFS_finds_a_git_directory_walking_up(t *testing.T) {
	fsys := fstest.MapFS{
		"repo/.git/HEAD":  &fstest.MapFile{Data: []byte("ref: refs/heads/main\n")},
		"repo/a/b/marker": &fstest.MapFile{Data: []byte("x")},
	}

	got, ok := repo.RootFS(fsys, fsAbs("repo", "a", "b"))

	require.True(t, ok)
	assert.Equal(t, fsAbs("repo"), got)
}

// Test_RootFS_returns_the_nearest_git_directory_not_a_farther_one pins the
// nearest-wins guard: a nested repository's own ".git" is found before the
// walk ever reaches the outer repository that contains it.
func Test_RootFS_returns_the_nearest_git_directory_not_a_farther_one(t *testing.T) {
	fsys := fstest.MapFS{
		"repo/.git/HEAD":           &fstest.MapFile{Data: []byte("x")},
		"repo/near/.git/HEAD":      &fstest.MapFile{Data: []byte("x")},
		"repo/near/sub/marker.txt": &fstest.MapFile{Data: []byte("x")},
	}

	got, ok := repo.RootFS(fsys, fsAbs("repo", "near", "sub"))

	require.True(t, ok)
	assert.Equal(t, fsAbs("repo", "near"), got)
}

// Test_RootFS_accepts_a_git_file pins the linked-worktree case: a ".git"
// entry that is a regular file (naming its real gitdir elsewhere) still
// counts, mirroring doctor's own env-git check.
func Test_RootFS_accepts_a_git_file(t *testing.T) {
	fsys := fstest.MapFS{
		"repo/.git": &fstest.MapFile{Data: []byte("gitdir: ../.git/worktrees/x\n")},
	}

	got, ok := repo.RootFS(fsys, fsAbs("repo"))

	require.True(t, ok)
	assert.Equal(t, fsAbs("repo"), got)
}

// Test_RootFS_reports_false_when_no_git_directory_exists_anywhere_above
// covers the "no enclosing repository" arm every caller falls back to its
// own unbounded behavior for.
func Test_RootFS_reports_false_when_no_git_directory_exists_anywhere_above(t *testing.T) {
	fsys := fstest.MapFS{
		"start/marker": &fstest.MapFile{Data: []byte("x")},
	}

	_, ok := repo.RootFS(fsys, fsAbs("start"))

	assert.False(t, ok)
}

// Test_Root_wires_the_root_fs_over_real_files is Root's own OS-adapter
// smoke test: production rootFS() (os.DirFS("/")) resolves a real ".git"
// directory the same way the disk-backed walk always did. RootFS's own
// walk logic — a ".git" directory or file, and the "not found" arm — is
// pinned exhaustively above.
func Test_Root_wires_the_root_fs_over_real_files(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	start := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(start, 0o755))

	got, ok := repo.Root(start)

	require.True(t, ok)
	assert.Equal(t, root, got)
}
