package setup_test

// This file pins two things a Mem-backed fixture cannot stand in for: Init's
// write path stays unconfined through a real symlink, and a relative wd
// still resolves against the process's own current directory (t.Chdir).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/require"
)

func Test_init_writes_plugin_files_through_a_claude_symlinked_outside_the_repo(t *testing.T) {
	wd := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "skills"), 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(wd, ".claude")))

	srv := setup.NewServer()

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	manifest := filepath.Join(outside, "skills", "brief", ".claude-plugin", "plugin.json")
	_, statErr := os.Stat(manifest)
	require.NoError(t, statErr, "plugin manifest must land under the symlink's own target, not be refused as an escape")
}

func Test_init_resolves_a_relative_wd_against_the_process_cwd(t *testing.T) {
	parent := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(parent, "child"), 0o755))
	t.Chdir(parent)

	srv := setup.NewServer()

	_, err := srv.Init(t.Context(), "child", setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	configPath := filepath.Join(parent, "child", ".brief.yaml")
	_, statErr := os.Stat(configPath)
	require.NoError(t, statErr, "relative wd must resolve against the process cwd, not diskFS's own \"/\" root")
}
