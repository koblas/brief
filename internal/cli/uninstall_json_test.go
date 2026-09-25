// OS-subject: chmods a real wd unwritable to force a real os.Remove
// failure, which rwfs.Mem has no permission model to reproduce.

package cli_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_uninstall_json_failure_reports_files_changed_false(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	require.NoError(t, os.Chmod(wd, 0o555))
	t.Cleanup(func() { _ = os.Chmod(wd, 0o755) })

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none", "--json"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	decoded := decodeErrorDocument(t, stdout.Bytes(), "uninstall")

	assert.Equal(t, "failure", decoded.Kind)
	require.NotNil(t, decoded.FilesChanged)
	assert.False(t, *decoded.FilesChanged)
}
