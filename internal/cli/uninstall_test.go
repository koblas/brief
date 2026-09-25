// OS-subject: chmods a real directory unwritable to force a partial write
// partway through Uninstall's removal order, which rwfs.Mem has no
// permission model to reproduce.

package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The CLAUDE.md block removes before the agents directory, made
// unwritable, blocks the next removal.
func Test_uninstall_partial_write_prints_the_rows_that_landed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	wd := t.TempDir()
	var initStdout, initStderr bytes.Buffer
	require.NoError(t, cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--with-agents"}, nil, &initStdout, &initStderr))

	agentsDir := filepath.Join(wd, ".claude", "skills", "brief", "agents")
	require.NoError(t, os.Chmod(agentsDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(agentsDir, 0o755) })

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Equal(t, "removed CLAUDE.md\n", stdout.String())
	assert.Contains(t, stderr.String(), "brief uninstall: ")
	assert.NotContains(t, stdout.String(), "agents")

	_, statErr := os.Stat(filepath.Join(wd, "CLAUDE.md"))
	assert.True(t, os.IsNotExist(statErr))
}

// The --json sibling of the test above: the same partial write reports
// files_changed true and no "artifacts" field.
func Test_uninstall_partial_write_with_json(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	wd := t.TempDir()
	var initStdout, initStderr bytes.Buffer
	require.NoError(t, cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--with-agents"}, nil, &initStdout, &initStderr))

	agentsDir := filepath.Join(wd, ".claude", "skills", "brief", "agents")
	require.NoError(t, os.Chmod(agentsDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(agentsDir, 0o755) })

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code", "--json"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	decoded := decodeErrorDocument(t, stdout.Bytes(), "uninstall")
	assert.Equal(t, "failure", decoded.Kind)
	require.NotNil(t, decoded.FilesChanged)
	assert.True(t, *decoded.FilesChanged)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &raw))
	assert.NotContains(t, raw, "artifacts")

	_, statErr := os.Stat(filepath.Join(wd, "CLAUDE.md"))
	assert.True(t, os.IsNotExist(statErr))
}
