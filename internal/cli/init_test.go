// OS-subject: cases here chmod a real directory unwritable or shape a real
// ENOTDIR from an ancestor segment that is a regular file, neither
// reproducible against rwfs.Mem. Every other init scenario moved to
// init_internal_test.go or init_bound_agent_internal_test.go.

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

// The portable fixture uses an explicit --host claude-code: a bare "init"
// would also trip host detection on the same ".claude/skills" path.
func Test_init_refuses_an_unwritable_target_and_prints_the_manual_output(t *testing.T) {
	t.Run("a regular file blocks a plugin directory ancestor", func(t *testing.T) {
		control := t.TempDir()
		var printStdout, printStderr bytes.Buffer
		require.NoError(t, cli.Run(t.Context(), control, []string{"init", "--host", "claude-code", "--print"}, nil, &printStdout, &printStderr))

		wd := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(wd, ".claude"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude", "skills"), []byte("not a directory"), 0o600))
		entriesBefore, readErr := os.ReadDir(wd)
		require.NoError(t, readErr)
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)

		assert.Equal(t, 1, cli.ExitCode(err))
		assert.Equal(t, "brief init: .claude/skills: not a directory; apply the output below by hand (no files changed)\n", stderr.String())
		assert.Equal(t, printStdout.String(), stdout.String())

		entriesAfter, readErr := os.ReadDir(wd)
		require.NoError(t, readErr)
		assert.Equal(t, entriesBefore, entriesAfter)
	})

	t.Run("an unwritable directory, --json", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root ignores directory write permission")
		}

		wd := t.TempDir()
		blocker := filepath.Join(wd, ".claude")
		require.NoError(t, os.Mkdir(blocker, 0o500))
		t.Cleanup(func() { _ = os.Chmod(blocker, 0o755) })
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--json"}, nil, &stdout, &stderr)

		assert.Equal(t, 1, cli.ExitCode(err))
		assert.Empty(t, stderr.String())

		decoded := decodeErrorDocument(t, stdout.Bytes(), "init")
		assert.Equal(t, "refusal", decoded.Kind)
		require.NotNil(t, decoded.FilesChanged)
		assert.False(t, *decoded.FilesChanged)
		assert.Equal(t, "run 'brief init --print --json' and apply the artifacts by hand", decoded.Fix)

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &raw))
		assert.NotContains(t, raw, "artifacts")
	})
}

// apply's write order lands the feature root before the config write fails,
// so text mode prints the feature root's "created" row; the config row,
// which never landed, is never printed.
func Test_init_partial_write_prints_the_rows_that_landed(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(wd, ".brief.yaml"), 0o755))

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--force"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Equal(t, "created docs/specifications/\n", stdout.String())
	assert.Contains(t, stderr.String(), "brief init: ")
	assert.NotContains(t, stdout.String(), ".brief.yaml")

	info, statErr := os.Stat(filepath.Join(wd, "docs", "specifications"))
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

// --json sibling of the test above: files_changed true, since the feature
// root did land, and no "artifacts" field.
func Test_init_partial_write_with_json(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(wd, ".brief.yaml"), 0o755))

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--force", "--json"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	decoded := decodeErrorDocument(t, stdout.Bytes(), "init")
	assert.Equal(t, "failure", decoded.Kind)
	require.NotNil(t, decoded.FilesChanged)
	assert.True(t, *decoded.FilesChanged)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &raw))
	assert.NotContains(t, raw, "artifacts")

	info, statErr := os.Stat(filepath.Join(wd, "docs", "specifications"))
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}
