// This file keeps only the ENAMETOOLONG case, a real filesystem
// name-length limit rwfs.Mem does not model.

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

// The state-file name is valid but over the filesystem's ENAMETOOLONG
// limit, so the specification write lands and the state write fails after it.
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
