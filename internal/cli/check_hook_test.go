// Both scenarios here return before runCheckHook ever reads wd — the
// opt-in gate (config.LocateInRepo) and FeatureContaining's own
// resolution never run — so neither needs check_hook_disk_test.go's own
// real repository tree, unlike everything else "check --hook" covers.

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

// Test_check_hook_usage_errors covers every text-mode usage-error shape
// "check --hook" reports before it ever reads the repository: exit 2, the
// pinned stderr line, empty stdout. Each case discriminates on its own
// mutation: the host it validates, or the flag combination it rejects. A
// malformed stdin payload is not a case here: it is no longer a usage
// error (exit 2) at all — see check_hook_disk_test.go's own
// Test_check_hook_malformed_payload_in_an_opted_in_repo_exits_1 and
// Test_check_hook_is_silent_for_malformed_stdin_when_no_brief_yaml_is_found
// for its own two shapes. "--hook with --json" is not a case here either:
// R5 routes every usage error to stdout as a JSON document under --json,
// a different assertion shape entirely — covered by
// Test_check_hook_with_json_reports_the_usage_error_as_json below.
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
			// Neither case ever reads wd: both usage errors return before
			// runCheckHook's own opt-in gate (config.LocateInRepo) runs, so
			// a fabricated path proves that as much as a real directory
			// would.
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
	// This usage error returns before runCheckHook ever reads wd (it is
	// checked ahead of the host lookup and the opt-in gate), so a
	// fabricated path proves that as much as a real directory would.
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
