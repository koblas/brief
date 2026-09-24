// Both cases below stay black-box: they decode through
// decodeErrorDocument/decodeUsageErrorDocument (json_refusal_test.go,
// json_usage_test.go), shared package cli_test helpers init_json_internal_test.go's
// own white-box package cannot import without duplicating their full
// key-set assertions. Every other init --json scenario in this package
// moved there (rwfs.Mem).

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

// Test_init_json_refusal_reports_files_changed_false pins R3's refusal
// shape for init: kind "refusal", files_changed false (the config never
// parsed, so nothing was ever written), and a message naming the fix.
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

// Test_init_json_unknown_host_fix_agrees_with_the_message pins the MINOR
// fix: R8's own unknown-host line trails prose ("to wire it by hand")
// after its "; run '...'" clause, the one shape usageFix's own generic
// extraction cannot recover (it requires the message to end at the closing
// quote — see Test_json_mode_usage_error_fix_stops_at_the_quote_when_the_line_has_trailing_prose
// in json_usage_test.go). error.fix must still name the same --print hint
// the message itself does, not init's own --host invocation.
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
