// Both scenarios here return before runCheckHook reads wd, so neither
// needs check_hook_disk_test.go's real repository tree.

package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Covers every text-mode usage-error shape "check --hook" reports before
// reading the repository; a malformed stdin payload is not a usage error.
func Test_check_hook_usage_errors(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		stdin      string
		wantStderr string
	}{
		{
			name:       "unknown host",
			args:       []string{"check", "--hook", "bogus"},
			stdin:      "",
			wantStderr: `brief check: unknown host "bogus"; expected one of: claude-code; run 'brief check --hook claude-code'` + "\n",
		},
		{
			name:       "--hook with a positional feature",
			args:       []string{"check", "auth", "--hook", "claude-code"},
			stdin:      "",
			wantStderr: "brief check: --hook takes no feature argument; run 'brief check --hook claude-code'\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Neither case reads wd, so a fabricated path is enough.
			wd := "/repo"

			var stdout, stderr bytes.Buffer
			err := cli.Run(t.Context(), wd, c.args, strings.NewReader(c.stdin), &stdout, &stderr)

			require.Error(t, err)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Equal(t, c.wantStderr, stderr.String())
			assert.Empty(t, stdout.String())
		})
	}
}

func Test_check_hook_with_json_reports_the_usage_error_as_json(t *testing.T) {
	// This usage error returns before runCheckHook reads wd.
	wd := "/repo"

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"check", "--hook", "claude-code", "--json"}, strings.NewReader(""), &stdout, &stderr)

	require.Error(t, err)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	var doc struct {
		OK    bool `json:"ok"`
		Error struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	assert.False(t, doc.OK)
	assert.Equal(t, "usage", doc.Error.Kind)
	assert.Equal(t, "brief check: --hook and --json cannot be combined; run 'brief check --hook claude-code'", doc.Error.Message)
}
