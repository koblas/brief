// Test_new_feature_json_is_one_exact_document and
// Test_new_step_json_names_the_step_and_its_file moved onto rwfs.Mem in
// new_internal_test.go. This file keeps only the ENAMETOOLONG case: a
// real filesystem name-length limit rwfs.Mem does not model.

package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_new_feature_json_files_changed_is_true_when_the_state_write_fails
// exercises "new feature"'s own write-site gap at the CLI boundary: a
// .brief.yaml naming a state-file over 255 bytes long passes R1's own
// validation (it is a plain file name with no path separator) but the
// filesystem itself refuses it with ENAMETOOLONG, so the specification's
// write lands first and the state write fails after it. files_changed
// must report true, not false, since the specification did land. The
// control arm reads that file back: it still carries the specification
// skeleton, proving files_changed's "true" is not vacuous.
func Test_new_feature_json_files_changed_is_true_when_the_state_write_fails(t *testing.T) {
	wd := t.TempDir()
	longStateFile := strings.Repeat("A", 256) + ".md"
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"),
		[]byte("state-file: \""+longStateFile+"\"\n"), 0o600))

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"new", "feature", "payments", "--json"}, nil, &stdout, &stderr)

	require.Error(t, err)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	got := decodeErrorDocument(t, stdout.Bytes(), "new feature")
	assert.Equal(t, "failure", got.Kind)
	require.NotNil(t, got.FilesChanged)
	assert.True(t, *got.FilesChanged)

	featureDir := filepath.Join(wd, "docs", "specifications", "payments")
	gotSpec, readErr := os.ReadFile(filepath.Join(featureDir, "specification.md"))
	require.NoError(t, readErr)
	assert.Equal(t, "# payments\n\n## BDD Acceptance Progress\n", string(gotSpec),
		"the specification write ahead of the blocked state write must actually have landed")
}
