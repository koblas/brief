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

// fsAbs joins segments under "/", the absolute-path form RootFS's tests key their fstest.MapFS fixtures against.
func fsAbs(elem ...string) string {
	return filepath.FromSlash("/" + filepath.ToSlash(filepath.Join(elem...)))
}

func Test_RootFS_finds_a_git_directory_walking_up(t *testing.T) {
	fsys := fstest.MapFS{
		"repo/.git/HEAD":  &fstest.MapFile{Data: []byte("ref: refs/heads/main\n")},
		"repo/a/b/marker": &fstest.MapFile{Data: []byte("x")},
	}

	got, ok := repo.RootFS(fsys, fsAbs("repo", "a", "b"))

	require.True(t, ok)
	assert.Equal(t, fsAbs("repo"), got)
}

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

// A ".git" file (linked worktree) counts the same as a ".git" directory.
func Test_RootFS_accepts_a_git_file(t *testing.T) {
	fsys := fstest.MapFS{
		"repo/.git": &fstest.MapFile{Data: []byte("gitdir: ../.git/worktrees/x\n")},
	}

	got, ok := repo.RootFS(fsys, fsAbs("repo"))

	require.True(t, ok)
	assert.Equal(t, fsAbs("repo"), got)
}

func Test_RootFS_reports_false_when_no_git_directory_exists_anywhere_above(t *testing.T) {
	fsys := fstest.MapFS{
		"start/marker": &fstest.MapFile{Data: []byte("x")},
	}

	_, ok := repo.RootFS(fsys, fsAbs("start"))

	assert.False(t, ok)
}

func Test_Root_wires_the_root_fs_over_real_files(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	start := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(start, 0o755))

	got, ok := repo.Root(start)

	require.True(t, ok)
	assert.Equal(t, root, got)
}
