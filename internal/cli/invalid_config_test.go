package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// invalidConfigRefusalLine is the one stderr line every command in
// Test_every_command_refuses_an_invalid_config_value must render,
// verbatim, for %s the command's own path (R1, R14a): the repository's
// ".brief.yaml" sets handoff-cap-lines to 0, which fails validate's cap
// rule before any command-specific logic runs.
const invalidConfigRefusalLine = "brief %s: .brief.yaml: handoff-cap-lines is 0, must be at least 1; " +
	"fix it or remove it to fall back to the shipped defaults (no files changed)\n"

// finishInvalidConfigArgs is Test_every_command_refuses_an_invalid_config_value's
// "finish" row: --handoff and --state name real, readable files so
// readSource never intervenes ahead of resolveRoot's own refusal.
func finishInvalidConfigArgs(t *testing.T) []string {
	t.Helper()

	handoffPath := writeInput(t, "handoff.md", "h\n")
	statePath := writeInput(t, "state.md", "s\n")

	return []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}
}

// Test_every_command_refuses_an_invalid_config_value is R1's own
// cross-command contract: status, start, check, finish, new feature and
// new step all resolve configuration the same way (resolveRoot), so an
// invalid value refuses identically regardless of which command reaches
// it first. finish's own args point at real, readable --handoff/--state
// files so readSource never intervenes ahead of resolveRoot's own
// refusal.
func Test_every_command_refuses_an_invalid_config_value(t *testing.T) {
	cases := []struct {
		name    string
		command string
		args    func(t *testing.T) []string
	}{
		{
			name:    "status",
			command: "status",
			args:    func(*testing.T) []string { return []string{"status"} },
		},
		{
			name:    "start",
			command: "start",
			args:    func(*testing.T) []string { return []string{"start", "payments"} },
		},
		{
			name:    "check",
			command: "check",
			args:    func(*testing.T) []string { return []string{"check"} },
		},
		{
			name:    "finish",
			command: "finish",
			args:    finishInvalidConfigArgs,
		},
		{
			name:    "new feature",
			command: "new feature",
			args:    func(*testing.T) []string { return []string{"new", "feature", "payments"} },
		},
		{
			name:    "new step",
			command: "new step",
			args:    func(*testing.T) []string { return []string{"new", "step", "payments"} },
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("handoff-cap-lines: 0\n"), 0o600))

			var stdout, stderr bytes.Buffer
			err := cli.Run(t.Context(), wd, c.args(t), nil, &stdout, &stderr)

			require.Error(t, err)
			assert.Equal(t, 1, cli.ExitCode(err))
			assert.Empty(t, stdout.String())
			assert.Equal(t, fmt.Sprintf(invalidConfigRefusalLine, c.command), stderr.String())
			assert.NoDirExists(t, filepath.Join(wd, "docs", "specifications"))
		})
	}
}
