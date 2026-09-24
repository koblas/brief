// Every finish --json scenario except the two below moved onto rwfs.Mem in
// finish_internal_test.go. These two stay on disk: each plants a real
// directory at atomicfile's own temp-sibling name to block one of Finish's
// four writes mid-sequence — real filesystem rename behavior no seam
// replaces.

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

// Test_finish_json_files_changed_is_true_when_a_write_lands_before_the_failure
// is MAJOR 1: files_changed must report what actually happened on disk, not
// "false" for every finish failure regardless of position. A directory
// planted at the state file's own atomicfile temp sibling — the same seam
// internal/scaffold's Test_reports_a_state_write_that_cannot_be_committed
// uses — blocks the second of Finish's four writes, after the handoff file
// (the first) has already landed. The control arm proves the "true" is not
// vacuous: the handoff file this run's own --handoff body predicts is
// actually on disk, not merely reported so.
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

// Test_finish_json_files_changed_is_false_when_the_first_write_fails is the
// control for the mid-sequence case above: a directory planted at the
// handoff file's own temp sibling blocks Finish's very first write, so
// nothing has landed when it fails — files_changed must stay false, the
// same value every other finish refusal in the matrix already carries. The
// control arm proves the "false" is not vacuous: the state file this run
// would have replaced is byte-identical to what newFinishCLIFixture wrote,
// not merely unread.
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
