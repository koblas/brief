// check --hook's own stdout-write failure stays on real disk for the same
// reason check_hook_disk_test.go does: runCheckHook's opt-in gate and
// edited-path resolution have no seam. Every other command's own
// stdout-write failure is covered on rwfs.Mem in
// stdout_failure_internal_test.go.

package cli_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingStdout refuses every Write, the way a full disk or a closed
// descriptor does.
type failingStdout struct{}

func (failingStdout) Write([]byte) (int, error) {
	return 0, &fs.PathError{Op: "write", Path: "/dev/stdout", Err: syscall.ENOSPC}
}

// Test_check_hook_reports_a_stdout_write_failure_on_stderr uses the same
// single-ERROR tree as
// Test_check_hook_reports_only_the_feature_containing_the_edited_path_as_additional_context:
// a feature with no step files and no STATE.md, so the hook has a finding
// to write.
func Test_check_hook_reports_a_stdout_write_failure_on_stderr(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(""), 0o600))

	alphaDir := filepath.Join(wd, "docs", "specifications", "alpha")
	require.NoError(t, os.MkdirAll(alphaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(alphaDir, "specification.md"), []byte(conformingSpec), 0o600))

	var stderr strings.Builder
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code"},
		strings.NewReader(hookPayload(filepath.Join(alphaDir, "specification.md"))), failingStdout{}, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Equal(t, "brief check: writing to stdout failed: write /dev/stdout: no space left on device"+
		"; fix the output destination, then run 'brief check [feature]' again\n", stderr.String())
}
