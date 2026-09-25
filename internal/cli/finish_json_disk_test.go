// These two stay on disk: each plants a real directory at atomicfile's
// temp-sibling name to block one of Finish's writes mid-sequence.

package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// files_changed must report what actually happened on disk: a directory
// blocks the second of Finish's writes, after the first already landed.
func Test_finish_json_files_changed_is_true_when_a_write_lands_before_the_failure(t *testing.T) {
	wd := newFinishCLIFixture(t)
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	blocked := filepath.Join(featureDir, ".STATE.md.brief-tmp")
	require.NoError(t, os.Mkdir(blocked, 0o755))

	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"}, nil, &stdout, &stderr)

	require.Error(t, err)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	got := decodeErrorDocument(t, stdout.Bytes(), "finish")
	assert.Equal(t, "failure", got.Kind)
	require.NotNil(t, got.FilesChanged)
	assert.True(t, *got.FilesChanged)

	gotHandoff, readErr := os.ReadFile(filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md"))
	require.NoError(t, readErr)
	assert.Equal(t, "NEW-HANDOFF\n", string(gotHandoff), "the handoff write ahead of the blocked one must actually have landed")
}

// Control for the mid-sequence case above: a directory blocks Finish's
// very first write, so nothing has landed when it fails.
func Test_finish_json_files_changed_is_false_when_the_first_write_fails(t *testing.T) {
	wd := newFinishCLIFixture(t)
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	originalState, readErr := os.ReadFile(filepath.Join(featureDir, "STATE.md"))
	require.NoError(t, readErr)

	blocked := filepath.Join(featureDir, ".SCENARIO-01-HANDOFF.md.brief-tmp")
	require.NoError(t, os.Mkdir(blocked, 0o755))

	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"}, nil, &stdout, &stderr)

	require.Error(t, err)
	assert.Empty(t, stderr.String())

	got := decodeErrorDocument(t, stdout.Bytes(), "finish")
	assert.Equal(t, "failure", got.Kind)
	require.NotNil(t, got.FilesChanged)
	assert.False(t, *got.FilesChanged)

	gotState, readErr := os.ReadFile(filepath.Join(featureDir, "STATE.md"))
	require.NoError(t, readErr)
	assert.Equal(t, string(originalState), string(gotState), "the state write follows the blocked handoff write, so it must not have run")
}
