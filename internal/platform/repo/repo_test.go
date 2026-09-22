package repo_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_Root_finds_a_git_directory_walking_up pins Root's own walk: a
// ".git" directory several levels below itself is found from a deeper
// start directory, reporting the directory that holds it.
func Test_Root_finds_a_git_directory_walking_up(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	start := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(start, 0o755))

	got, ok := repo.Root(start)

	require.True(t, ok)
	assert.Equal(t, root, got)
}

// Test_Root_accepts_a_git_file pins the linked-worktree case: a ".git"
// entry that is a regular file (naming its real gitdir elsewhere) still
// counts, mirroring doctor's own env-git check.
func Test_Root_accepts_a_git_file(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: ../.git/worktrees/x\n"), 0o600))

	got, ok := repo.Root(root)

	require.True(t, ok)
	assert.Equal(t, root, got)
}

// Test_Root_reports_false_when_no_git_directory_exists_anywhere_above
// covers the "no enclosing repository" arm every caller falls back to its
// own unbounded behavior for.
func Test_Root_reports_false_when_no_git_directory_exists_anywhere_above(t *testing.T) {
	start := t.TempDir()

	_, ok := repo.Root(start)

	assert.False(t, ok)
}
