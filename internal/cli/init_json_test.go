// Black-box: decodes through decodeErrorDocument/decodeUsageErrorDocument,
// shared cli_test helpers the white-box init_json_internal_test.go cannot
// import. Every other init --json scenario moved there (rwfs.Mem).

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

func Test_init_json_refusal_reports_files_changed_false(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("handoff-cap-lines: 0\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--json"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	decoded := decodeErrorDocument(t, stdout.Bytes(), "init")

	assert.Equal(t, "refusal", decoded.Kind)
	require.NotNil(t, decoded.FilesChanged)
	assert.False(t, *decoded.FilesChanged)
	assert.Contains(t, decoded.Fix, "brief init --force")
}

// The unknown-host line trails prose after its "; run '...'" clause, a
// shape the generic fix-extraction cannot recover; error.fix must still
// name the same --print hint the message itself does.
func Test_init_json_unknown_host_fix_agrees_with_the_message(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "bogus", "--json"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	message, fix := decodeUsageErrorDocument(t, stdout.Bytes(), "init", new(false))

	assert.Equal(t, `brief init: unknown host "bogus"; expected one of: claude-code, none; run 'brief init --print' to wire it by hand`, message)
	assert.Equal(t, "run 'brief init --print' to wire it by hand", fix)
	assert.Contains(t, message, fix)
}
