// OS-subject: both cases below chmod a real directory unwritable (0o555)
// to force setup's own R10 pre-write check to refuse partway through
// Uninstall's removal order — an rwfs.Mem target has no permission model
// to fail against (uninstall_internal_test.go's own newMemSetupSeam always
// passes a no-op WithWritableCheck), so this stays the one place that
// exercises a real partial write. Every other uninstall scenario in this
// package moved to uninstall_internal_test.go (rwfs.Mem) or
// uninstall_bound_agent_internal_test.go (a real bound agent file,
// OS-subject for a different reason).

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

// Test_uninstall_partial_write_prints_the_rows_that_landed pins the
// partial-write case (setup.ErrPartialWrite): the CLAUDE.md block is
// removed before the agents directory — made unwritable — blocks the next
// removal, so text mode prints the CLAUDE.md "removed" row that actually
// landed on stdout before the refusal line on stderr; no agent row, which
// never landed, is ever printed. Skipped under root, which ignores
// directory write permission.
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

// Test_uninstall_partial_write_with_json is
// Test_uninstall_partial_write_prints_the_rows_that_landed's own --json
// sibling: the same partial write reports the standard error document —
// files_changed true, since the CLAUDE.md block did land, and no
// "artifacts" field, the same contract a partial write's text mode observes
// by printing landed rows instead. Skipped under root, which ignores
// directory write permission.
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
