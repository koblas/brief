package agentfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests run Find against real directories: they pin the OS adapter
// itself (absolute paths, symlinks), not FindIn's matching rules.

// writeAgentFile writes body at path, creating every parent directory it
// needs.
func writeAgentFile(t *testing.T, path, body string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

func Test_find_reports_the_real_absolute_path_of_a_nested_agent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".claude", "agents", "developer", "Agent.md")
	writeAgentFile(t, path, developerAgent)

	defs := agentfile.Find(root, t.TempDir(), "developer")

	require.Len(t, defs, 1)
	assert.Equal(t, path, defs[0].Path)

	_, err := os.Lstat(defs[0].Path)
	assert.NoError(t, err, "Path must name a real file a caller can Lstat")
}

func Test_find_resolves_a_relative_root_to_an_absolute_path(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	writeAgentFile(t, filepath.Join(cwd, "repo", ".claude", "agents", "developer.md"), developerAgent)

	defs := agentfile.Find("repo", "", "developer")

	require.Len(t, defs, 1)
	assert.True(t, filepath.IsAbs(defs[0].Path))
	assert.Equal(t, filepath.Join(cwd, "repo", ".claude", "agents", "developer.md"), defs[0].Path)
}

// Seeds the working directory with a match: an unguarded empty home would
// resolve against it, so an empty result proves the empty-home rule.
func Test_find_with_an_empty_home_searches_no_user_scope(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	writeAgentFile(t, filepath.Join(cwd, ".claude", "agents", "developer.md"), developerAgent)

	defs := agentfile.Find(t.TempDir(), "", "developer")

	assert.Empty(t, defs)
}

func Test_find_symlink_handling(t *testing.T) {
	t.Run("a symlinked leaf is read through its link", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "shared", "dev.md")
		writeAgentFile(t, target, developerAgent)
		link := filepath.Join(root, ".claude", "agents", "developer.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
		require.NoError(t, os.Symlink(target, link))

		defs := agentfile.Find(root, "", "developer")

		require.Len(t, defs, 1)
		assert.Equal(t, link, defs[0].Path, "Path is the link as found, not its target")
	})

	t.Run("a symlinked directory is not descended", func(t *testing.T) {
		root := t.TempDir()
		writeAgentFile(t, filepath.Join(root, "shared", "developer.md"), developerAgent)
		agents := filepath.Join(root, ".claude", "agents")
		require.NoError(t, os.MkdirAll(agents, 0o755))
		require.NoError(t, os.Symlink(filepath.Join(root, "shared"), filepath.Join(agents, "team")))

		defs := agentfile.Find(root, "", "developer")

		assert.Empty(t, defs)
	})

	t.Run("a dangling symlinked leaf is skipped", func(t *testing.T) {
		root := t.TempDir()
		link := filepath.Join(root, ".claude", "agents", "developer.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
		require.NoError(t, os.Symlink(filepath.Join(root, "gone.md"), link))

		defs := agentfile.Find(root, "", "developer")

		assert.Empty(t, defs)
	})
}
