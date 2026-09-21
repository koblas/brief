package cli_test

import (
	"bytes"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_reports_an_undefined_long_flag_as_one_usage_line_naming_the_command_invocation
// is SCENARIO-02's table: every leaf reports an undefined long flag through
// the same root SetFlagErrorFunc frame — "brief <path>: <pflag error>; run
// '<invocation>'" — with empty stdout and exit 2. Expected stderr is written
// out literally per row, never built from a production invocation constant
// (e.g. finishInvocation): building it from the constant would pin nothing.
func Test_reports_an_undefined_long_flag_as_one_usage_line_naming_the_command_invocation(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "start",
			args:       []string{"start", "--bogus", "demo"},
			wantStderr: "brief start: unknown flag: --bogus; run 'brief start <feature>'",
		},
		{
			name:       "finish",
			args:       []string{"finish", "demo", "SCENARIO-01", "--bogus"},
			wantStderr: "brief finish: unknown flag: --bogus; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
		},
		{
			name:       "status",
			args:       []string{"status", "--bogus"},
			wantStderr: "brief status: unknown flag: --bogus; run 'brief status'",
		},
		{
			name:       "check",
			args:       []string{"check", "--bogus"},
			wantStderr: "brief check: unknown flag: --bogus; run 'brief check [feature]'",
		},
		{
			name:       "new feature",
			args:       []string{"new", "feature", "--bogus", "payments"},
			wantStderr: "brief new feature: unknown flag: --bogus; run 'brief new feature <name>'",
		},
		{
			name:       "new step",
			args:       []string{"new", "step", "--bogus", "demo"},
			wantStderr: "brief new step: unknown flag: --bogus; run 'brief new step <feature>'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tt.args, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Empty(t, stdout.String())
			assert.Equal(t, tt.wantStderr, oneLine(t, &stderr))
		})
	}
}

// Test_reports_an_undefined_short_flag_as_one_usage_line_naming_the_command_invocation
// is SCENARIO-03's table: every leaf reports an undefined shorthand flag
// through the same root SetFlagErrorFunc frame as SCENARIO-02's long-flag
// table, plus the grouped-shorthand shapes pflag produces for a cluster of
// short flags. pflag quotes the whole cluster when every letter in it is
// undefined ("-xy" -> "in -xy") but quotes only the residual cluster once a
// leading defined shorthand ("-h") has been consumed ("-hx" -> "in -x").
func Test_reports_an_undefined_short_flag_as_one_usage_line_naming_the_command_invocation(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "new feature",
			args:       []string{"new", "feature", "-x", "payments"},
			wantStderr: "brief new feature: unknown shorthand flag: 'x' in -x; run 'brief new feature <name>'",
		},
		{
			name:       "start",
			args:       []string{"start", "-x", "demo"},
			wantStderr: "brief start: unknown shorthand flag: 'x' in -x; run 'brief start <feature>'",
		},
		{
			name:       "finish",
			args:       []string{"finish", "demo", "SCENARIO-01", "-x"},
			wantStderr: "brief finish: unknown shorthand flag: 'x' in -x; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
		},
		{
			name:       "status",
			args:       []string{"status", "-x"},
			wantStderr: "brief status: unknown shorthand flag: 'x' in -x; run 'brief status'",
		},
		{
			name:       "check",
			args:       []string{"check", "-x"},
			wantStderr: "brief check: unknown shorthand flag: 'x' in -x; run 'brief check [feature]'",
		},
		{
			name:       "new step",
			args:       []string{"new", "step", "-x", "demo"},
			wantStderr: "brief new step: unknown shorthand flag: 'x' in -x; run 'brief new step <feature>'",
		},
		{
			name:       "new feature, all-undefined cluster",
			args:       []string{"new", "feature", "-xy", "payments"},
			wantStderr: "brief new feature: unknown shorthand flag: 'x' in -xy; run 'brief new feature <name>'",
		},
		{
			name:       "new feature, residual cluster after a defined -h",
			args:       []string{"new", "feature", "-hx", "payments"},
			wantStderr: "brief new feature: unknown shorthand flag: 'x' in -x; run 'brief new feature <name>'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tt.args, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Empty(t, stdout.String())
			assert.Equal(t, tt.wantStderr, oneLine(t, &stderr))
		})
	}
}
