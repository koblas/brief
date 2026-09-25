package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wantStderr is written out literally per row, not built from a production
// invocation constant, so the assertion pins the literal rather than the code.
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
		{
			name:       "completion",
			args:       []string{"completion", "--bogus"},
			wantStderr: "brief completion: unknown flag: --bogus; run 'brief completion <bash|zsh|fish|powershell>'",
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

func Test_new_reports_the_version_flag_as_unknown(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "--version"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief new: unknown flag: --version; run 'brief new <type> --help'", oneLine(t, &stderr))
}

// "new --version" stays in its own standalone test above; "new"/"help"
// "--version=x"/"--version=" stay in the cross-site table below.
func Test_reports_the_version_flag_as_unknown_outside_the_root(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "help --version",
			args:       []string{"help", "--version"},
			wantStderr: "brief help: unknown flag: --version; run 'brief help <command>'",
		},
		{
			name:       "start --version",
			args:       []string{"start", "--version", "demo"},
			wantStderr: "brief start: unknown flag: --version; run 'brief start <feature>'",
		},
		{
			name:       "finish --version",
			args:       []string{"finish", "demo", "SCENARIO-01", "--version"},
			wantStderr: "brief finish: unknown flag: --version; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
		},
		{
			name:       "status --version",
			args:       []string{"status", "--version"},
			wantStderr: "brief status: unknown flag: --version; run 'brief status'",
		},
		{
			name:       "check --version",
			args:       []string{"check", "--version"},
			wantStderr: "brief check: unknown flag: --version; run 'brief check [feature]'",
		},
		{
			name:       "new feature --version",
			args:       []string{"new", "feature", "--version", "payments"},
			wantStderr: "brief new feature: unknown flag: --version; run 'brief new feature <name>'",
		},
		{
			name:       "new step --version",
			args:       []string{"new", "step", "--version", "demo"},
			wantStderr: "brief new step: unknown flag: --version; run 'brief new step <feature>'",
		},
		{
			name:       "completion --version",
			args:       []string{"completion", "--version"},
			wantStderr: "brief completion: unknown flag: --version; run 'brief completion <bash|zsh|fish|powershell>'",
		},
		{
			name:       "start --version=x, pflag strips the value",
			args:       []string{"start", "--version=x", "demo"},
			wantStderr: "brief start: unknown flag: --version; run 'brief start <feature>'",
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

// pflag quotes the whole shorthand cluster when every letter is undefined
// ("-xy" -> "in -xy") but only the residual once a leading defined shorthand
// ("-h") is consumed ("-hx" -> "in -x").
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

// A single-dash spelling of a long flag parses as a pflag shorthand cluster,
// not the long flag. "-help"/"-handoff" start with cobra's auto "h" help
// shorthand, so pflag consumes it and reports only the residual cluster.
func Test_rejects_a_single_dash_long_flag_as_one_usage_line_naming_the_command_invocation(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "start -json",
			args:       []string{"start", "-json", "demo"},
			wantStderr: "brief start: unknown shorthand flag: 'j' in -json; run 'brief start <feature>'",
		},
		{
			name:       "finish -handoff",
			args:       []string{"finish", "demo", "SCENARIO-01", "-handoff", "h", "--state", "s"},
			wantStderr: "brief finish: unknown shorthand flag: 'a' in -andoff; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
		},
		{
			name:       "finish -state",
			args:       []string{"finish", "demo", "SCENARIO-01", "--handoff", "h", "-state", "s"},
			wantStderr: "brief finish: unknown shorthand flag: 's' in -state; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
		},
		{
			name:       "start -help",
			args:       []string{"start", "-help", "demo"},
			wantStderr: "brief start: unknown shorthand flag: 'e' in -elp; run 'brief start <feature>'",
		},
		{
			name:       "finish -help",
			args:       []string{"finish", "demo", "SCENARIO-01", "-help"},
			wantStderr: "brief finish: unknown shorthand flag: 'e' in -elp; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
		},
		{
			name:       "status -help",
			args:       []string{"status", "-help"},
			wantStderr: "brief status: unknown shorthand flag: 'e' in -elp; run 'brief status'",
		},
		{
			name:       "check -help",
			args:       []string{"check", "-help"},
			wantStderr: "brief check: unknown shorthand flag: 'e' in -elp; run 'brief check [feature]'",
		},
		{
			name:       "new feature -help",
			args:       []string{"new", "feature", "-help", "payments"},
			wantStderr: "brief new feature: unknown shorthand flag: 'e' in -elp; run 'brief new feature <name>'",
		},
		{
			name:       "new step -help",
			args:       []string{"new", "step", "-help", "demo"},
			wantStderr: "brief new step: unknown shorthand flag: 'e' in -elp; run 'brief new step <feature>'",
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

func Test_flattens_a_flag_error_that_embeds_a_newline_to_one_stderr_line(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "start, undefined long flag with an embedded newline",
			args:       []string{"start", "--fo\no", "x"},
			wantStderr: "brief start: unknown flag: --fo o; run 'brief start <feature>'",
		},
		{
			name:       "status, undefined shorthand flag with an embedded newline",
			args:       []string{"status", "-z\nq"},
			wantStderr: "brief status: unknown shorthand flag: 'z' in -z q; run 'brief status'",
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

// "finish" is the only leaf with a value-taking flag; flag parsing runs
// before its argument-count check, so a missing value is reported even with
// no positional given at all.
func Test_reports_a_flag_missing_its_value_as_one_usage_line_naming_the_command_invocation(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "--handoff at the end",
			args:       []string{"finish", "demo", "SCENARIO-01", "--handoff"},
			wantStderr: "brief finish: flag needs an argument: --handoff; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
		},
		{
			name:       "--state at the end, --handoff already given",
			args:       []string{"finish", "demo", "SCENARIO-01", "--handoff", "h.md", "--state"},
			wantStderr: "brief finish: flag needs an argument: --state; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
		},
		{
			name:       "--handoff at the end, --state already given",
			args:       []string{"finish", "demo", "SCENARIO-01", "--state", "s.md", "--handoff"},
			wantStderr: "brief finish: flag needs an argument: --handoff; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
		},
		{
			name:       "no positionals at all",
			args:       []string{"finish", "--handoff"},
			wantStderr: "brief finish: flag needs an argument: --handoff; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
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

// "--handoff=" / "--state=" are not flag-parse errors: pflag accepts the
// explicit empty value, so parsing succeeds and the required-flag check
// reports it instead.
func Test_reports_an_empty_flag_value_as_that_flag_being_required(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "--handoff= with --state given",
			args:       []string{"finish", "demo", "SCENARIO-01", "--handoff=", "--state", "s.md"},
			wantStderr: "brief finish: --handoff is required; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
		},
		{
			name:       "--state= with --handoff given",
			args:       []string{"finish", "demo", "SCENARIO-01", "--handoff", "h.md", "--state="},
			wantStderr: "brief finish: --state is required; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
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

// pflag takes the next token as a flag's value even when that token itself
// starts with "--".
func Test_takes_the_next_flag_as_the_value_of_a_flag_missing_its_value(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "--handoff swallows --state, s.md becomes a third positional",
			args:       []string{"finish", "demo", "SCENARIO-01", "--handoff", "--state", "s.md"},
			wantStderr: "brief finish: too many arguments; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
		},
		{
			name:       "--handoff swallows --state, leaving --state itself unset",
			args:       []string{"finish", "demo", "SCENARIO-01", "--handoff", "--state"},
			wantStderr: "brief finish: --state is required; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
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

// pflag's ParseFlags stops at the first bad token, so an undefined flag is a
// usage error regardless of where --help/-h falls relative to it.
func Test_reports_an_undefined_flag_as_a_usage_error_whichever_side_of_help_it_is_on(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "start --help --bogus",
			args:       []string{"start", "--help", "--bogus"},
			wantStderr: "brief start: unknown flag: --bogus; run 'brief start <feature>'",
		},
		{
			name:       "start --bogus --help",
			args:       []string{"start", "--bogus", "--help"},
			wantStderr: "brief start: unknown flag: --bogus; run 'brief start <feature>'",
		},
		{
			name:       "finish --help --bogus",
			args:       []string{"finish", "demo", "SCENARIO-01", "--help", "--bogus"},
			wantStderr: "brief finish: unknown flag: --bogus; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
		},
		{
			name:       "finish --bogus --help",
			args:       []string{"finish", "demo", "SCENARIO-01", "--bogus", "--help"},
			wantStderr: "brief finish: unknown flag: --bogus; run 'brief finish <feature> <step> --handoff <path> --state <path>'",
		},
		{
			name:       "status --help --bogus",
			args:       []string{"status", "--help", "--bogus"},
			wantStderr: "brief status: unknown flag: --bogus; run 'brief status'",
		},
		{
			name:       "status --bogus --help",
			args:       []string{"status", "--bogus", "--help"},
			wantStderr: "brief status: unknown flag: --bogus; run 'brief status'",
		},
		{
			name:       "check --help --bogus",
			args:       []string{"check", "--help", "--bogus"},
			wantStderr: "brief check: unknown flag: --bogus; run 'brief check [feature]'",
		},
		{
			name:       "check --bogus --help",
			args:       []string{"check", "--bogus", "--help"},
			wantStderr: "brief check: unknown flag: --bogus; run 'brief check [feature]'",
		},
		{
			name:       "new feature --help --bogus",
			args:       []string{"new", "feature", "--help", "--bogus", "payments"},
			wantStderr: "brief new feature: unknown flag: --bogus; run 'brief new feature <name>'",
		},
		{
			name:       "new feature --bogus --help",
			args:       []string{"new", "feature", "--bogus", "--help", "payments"},
			wantStderr: "brief new feature: unknown flag: --bogus; run 'brief new feature <name>'",
		},
		{
			name:       "new step --help --bogus",
			args:       []string{"new", "step", "--help", "--bogus", "demo"},
			wantStderr: "brief new step: unknown flag: --bogus; run 'brief new step <feature>'",
		},
		{
			name:       "new step --bogus --help",
			args:       []string{"new", "step", "--bogus", "--help", "demo"},
			wantStderr: "brief new step: unknown flag: --bogus; run 'brief new step <feature>'",
		},
		{
			name:       "start -h --bogus",
			args:       []string{"start", "-h", "--bogus"},
			wantStderr: "brief start: unknown flag: --bogus; run 'brief start <feature>'",
		},
		{
			name:       "start --bogus -h",
			args:       []string{"start", "--bogus", "-h"},
			wantStderr: "brief start: unknown flag: --bogus; run 'brief start <feature>'",
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

// Control arm for the -h rows above: without it, "start --bogus -h" would
// pass even if -h were itself undefined, since pflag errors on --bogus
// before -h is ever parsed.
func Test_prints_help_for_the_h_shorthand_alone(t *testing.T) {
	wd := t.TempDir()
	var helpStdout, helpStderr bytes.Buffer

	helpErr := cli.Run(t.Context(), wd, []string{"start", "--help"}, nil, &helpStdout, &helpStderr)

	require.NoError(t, helpErr)

	var shortStdout, shortStderr bytes.Buffer

	shortErr := cli.Run(t.Context(), wd, []string{"start", "-h"}, nil, &shortStdout, &shortStderr)

	require.NoError(t, shortErr)
	assert.Equal(t, 0, cli.ExitCode(shortErr))
	assert.Empty(t, shortStderr.String())
	assert.Contains(t, shortStdout.String(), "brief start reads; it never writes.")
	assert.Equal(t, helpStdout.String(), shortStdout.String())
}

// "--json=<v>", any value including an explicit empty one, is caught before
// ExecuteContext ever runs, so it always reports "takes no value".
func Test_json_flag_with_a_value_never_reaches_the_bool_flag_rewrite(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "--json=maybe",
			args:       []string{"start", "--json=maybe", "demo"},
			wantStderr: "brief start: '--json' takes no value; run 'brief start <feature> --json'",
		},
		{
			name:       "--json= with an explicit empty value",
			args:       []string{"start", "--json=", "demo"},
			wantStderr: "brief start: '--json' takes no value; run 'brief start <feature> --json'",
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

// Control arm: differs from a rejected row above only in dash count, and
// succeeds — proving the rejection is specific to the single-dash spelling.
func Test_accepts_the_double_dash_json_flag(t *testing.T) {
	wd := newStartFixture(t, "open")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--json", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Empty(t, stderr.String())
}

func Test_accepts_the_double_dash_handoff_and_state_flags(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
}

// A shorthand cluster made entirely of "h" characters parses like a single
// "-h" for pflag's own shorthand-cluster parser.
func Test_prints_help_for_the_hh_cluster_alone(t *testing.T) {
	tests := []struct {
		name       string
		helpArgs   []string
		clusterArg []string
	}{
		{name: "root", helpArgs: []string{"--help"}, clusterArg: []string{"-hh"}},
		{name: "new", helpArgs: []string{"new", "--help"}, clusterArg: []string{"new", "-hh"}},
		{name: "help", helpArgs: []string{"help", "--help"}, clusterArg: []string{"help", "-hh"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var helpStdout, helpStderr bytes.Buffer

			helpErr := cli.Run(t.Context(), wd, tt.helpArgs, nil, &helpStdout, &helpStderr)

			require.NoError(t, helpErr)
			assert.Contains(t, helpStdout.String(), "Usage:")

			var clusterStdout, clusterStderr bytes.Buffer

			clusterErr := cli.Run(t.Context(), wd, tt.clusterArg, nil, &clusterStdout, &clusterStderr)

			require.NoError(t, clusterErr)
			assert.Equal(t, 0, cli.ExitCode(clusterErr))
			assert.Empty(t, clusterStderr.String())
			assert.NotEmpty(t, clusterStdout.String())
			assert.Equal(t, helpStdout.String(), clusterStdout.String())
		})
	}
}

// Crosses root, "new" and the help stub against one shared list of
// dash-prefixed tokens: each token's kind must be identical at every site,
// differing only in the invocation named in the "run '...'" tail.
func Test_classifies_dash_prefixed_tokens_consistently_across_disabled_parsing_sites(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		// "--"
		{name: "root --", args: []string{"--"}, wantStderr: `brief: unknown command "--"; expected one of: new, start, finish, status, check, init, doctor, uninstall`},
		{name: "new --", args: []string{"new", "--"}, wantStderr: `brief new: unknown type "--"; expected one of: feature, step`},
		{name: "help --", args: []string{"help", "--"}, wantStderr: `brief help: unknown command "--"; expected one of: new, start, finish, status, check, init, doctor, uninstall`},

		// "--help=true"
		{name: "root --help=true", args: []string{"--help=true"}, wantStderr: "brief: '--help' takes no value; run 'brief --help'"},
		{name: "new --help=true", args: []string{"new", "--help=true"}, wantStderr: "brief new: '--help' takes no value; run 'brief new --help'"},
		{name: "help --help=true", args: []string{"help", "--help=true"}, wantStderr: "brief help: '--help' takes no value; run 'brief help <command>'"},

		// "-h=x"
		{name: "root -h=x", args: []string{"-h=x"}, wantStderr: "brief: '-h' takes no value; run 'brief --help'"},
		{name: "new -h=x", args: []string{"new", "-h=x"}, wantStderr: "brief new: '-h' takes no value; run 'brief new --help'"},
		{name: "help -h=x", args: []string{"help", "-h=x"}, wantStderr: "brief help: '-h' takes no value; run 'brief help <command>'"},

		// "-hh=x"
		{name: "root -hh=x", args: []string{"-hh=x"}, wantStderr: "brief: '-hh' takes no value; run 'brief --help'"},
		{name: "new -hh=x", args: []string{"new", "-hh=x"}, wantStderr: "brief new: '-hh' takes no value; run 'brief new --help'"},
		{name: "help -hh=x", args: []string{"help", "-hh=x"}, wantStderr: "brief help: '-hh' takes no value; run 'brief help <command>'"},

		// "-hh=" (explicit empty value)
		{name: "root -hh=", args: []string{"-hh="}, wantStderr: "brief: '-hh' takes no value; run 'brief --help'"},
		{name: "new -hh=", args: []string{"new", "-hh="}, wantStderr: "brief new: '-hh' takes no value; run 'brief new --help'"},
		{name: "help -hh=", args: []string{"help", "-hh="}, wantStderr: "brief help: '-hh' takes no value; run 'brief help <command>'"},

		// "-hx"
		{name: "root -hx", args: []string{"-hx"}, wantStderr: "brief: unknown shorthand flag: 'x' in -x; run 'brief <command> --help'"},
		{name: "new -hx", args: []string{"new", "-hx"}, wantStderr: "brief new: unknown shorthand flag: 'x' in -x; run 'brief new <type> --help'"},
		{name: "help -hx", args: []string{"help", "-hx"}, wantStderr: "brief help: unknown shorthand flag: 'x' in -x; run 'brief help <command>'"},

		// "-hhx"
		{name: "root -hhx", args: []string{"-hhx"}, wantStderr: "brief: unknown shorthand flag: 'x' in -x; run 'brief <command> --help'"},
		{name: "new -hhx", args: []string{"new", "-hhx"}, wantStderr: "brief new: unknown shorthand flag: 'x' in -x; run 'brief new <type> --help'"},
		{name: "help -hhx", args: []string{"help", "-hhx"}, wantStderr: "brief help: unknown shorthand flag: 'x' in -x; run 'brief help <command>'"},

		// "--=x"
		{name: "root --=x", args: []string{"--=x"}, wantStderr: "brief: bad flag syntax: --=x; run 'brief <command> --help'"},
		{name: "new --=x", args: []string{"new", "--=x"}, wantStderr: "brief new: bad flag syntax: --=x; run 'brief new <type> --help'"},
		{name: "help --=x", args: []string{"help", "--=x"}, wantStderr: "brief help: bad flag syntax: --=x; run 'brief help <command>'"},

		// "---x"
		{name: "root ---x", args: []string{"---x"}, wantStderr: "brief: bad flag syntax: ---x; run 'brief <command> --help'"},
		{name: "new ---x", args: []string{"new", "---x"}, wantStderr: "brief new: bad flag syntax: ---x; run 'brief new <type> --help'"},
		{name: "help ---x", args: []string{"help", "---x"}, wantStderr: "brief help: bad flag syntax: ---x; run 'brief help <command>'"},

		// "--version=x"
		{name: "root --version=x", args: []string{"--version=x"}, wantStderr: "brief: '--version' takes no value; run 'brief --version'"},
		{name: "new --version=x", args: []string{"new", "--version=x"}, wantStderr: "brief new: unknown flag: --version; run 'brief new <type> --help'"},
		{name: "help --version=x", args: []string{"help", "--version=x"}, wantStderr: "brief help: unknown flag: --version; run 'brief help <command>'"},

		// "--version=" (explicit empty value)
		{name: "root --version=", args: []string{"--version="}, wantStderr: "brief: '--version' takes no value; run 'brief --version'"},
		{name: "new --version=", args: []string{"new", "--version="}, wantStderr: "brief new: unknown flag: --version; run 'brief new <type> --help'"},
		{name: "help --version=", args: []string{"help", "--version="}, wantStderr: "brief help: unknown flag: --version; run 'brief help <command>'"},

		// "--bogus"
		{name: "root --bogus", args: []string{"--bogus"}, wantStderr: "brief: unknown flag: --bogus; run 'brief <command> --help'"},
		{name: "new --bogus", args: []string{"new", "--bogus"}, wantStderr: "brief new: unknown flag: --bogus; run 'brief new <type> --help'"},
		{name: "help --bogus", args: []string{"help", "--bogus"}, wantStderr: "brief help: unknown flag: --bogus; run 'brief help <command>'"},

		// "-x"
		{name: "root -x", args: []string{"-x"}, wantStderr: "brief: unknown shorthand flag: 'x' in -x; run 'brief <command> --help'"},
		{name: "new -x", args: []string{"new", "-x"}, wantStderr: "brief new: unknown shorthand flag: 'x' in -x; run 'brief new <type> --help'"},
		{name: "help -x", args: []string{"help", "-x"}, wantStderr: "brief help: unknown shorthand flag: 'x' in -x; run 'brief help <command>'"},

		// -v is reserved for a future --verbose, never a --version alias.
		{name: "root -v", args: []string{"-v"}, wantStderr: "brief: unknown shorthand flag: 'v' in -v; run 'brief <command> --help'"},
		{name: "new -v", args: []string{"new", "-v"}, wantStderr: "brief new: unknown shorthand flag: 'v' in -v; run 'brief new <type> --help'"},
		{name: "help -v", args: []string{"help", "-v"}, wantStderr: "brief help: unknown shorthand flag: 'v' in -v; run 'brief help <command>'"},

		// "-v=x"
		{name: "root -v=x", args: []string{"-v=x"}, wantStderr: "brief: unknown shorthand flag: 'v' in -v=x; run 'brief <command> --help'"},
		{name: "new -v=x", args: []string{"new", "-v=x"}, wantStderr: "brief new: unknown shorthand flag: 'v' in -v=x; run 'brief new <type> --help'"},
		{name: "help -v=x", args: []string{"help", "-v=x"}, wantStderr: "brief help: unknown shorthand flag: 'v' in -v=x; run 'brief help <command>'"},

		// "-vh"
		{name: "root -vh", args: []string{"-vh"}, wantStderr: "brief: unknown shorthand flag: 'v' in -vh; run 'brief <command> --help'"},
		{name: "new -vh", args: []string{"new", "-vh"}, wantStderr: "brief new: unknown shorthand flag: 'v' in -vh; run 'brief new <type> --help'"},
		{name: "help -vh", args: []string{"help", "-vh"}, wantStderr: "brief help: unknown shorthand flag: 'v' in -vh; run 'brief help <command>'"},

		// "-hv" (leading defined -h is consumed first, same skip rule as "-hx")
		{name: "root -hv", args: []string{"-hv"}, wantStderr: "brief: unknown shorthand flag: 'v' in -v; run 'brief <command> --help'"},
		{name: "new -hv", args: []string{"new", "-hv"}, wantStderr: "brief new: unknown shorthand flag: 'v' in -v; run 'brief new <type> --help'"},
		{name: "help -hv", args: []string{"help", "-hv"}, wantStderr: "brief help: unknown shorthand flag: 'v' in -v; run 'brief help <command>'"},
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

// A non-ASCII residual byte is quoted using pflag's own quirk of taking the
// residual's first byte, not its real UTF-8 rune, so the quoted character is
// not the one actually typed — pinned deliberately, not "fixed".
func Test_quotes_a_multibyte_unknown_shorthand_flag_byte_identically_across_leaf_root_and_new(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		wantLeaf string
		wantRoot string
		wantNew  string
	}{
		{
			name:     "an accented character in the Latin-1 Supplement block",
			arg:      "-é",
			wantLeaf: "brief start: unknown shorthand flag: 'Ã' in -é; run 'brief start <feature>'",
			wantRoot: "brief: unknown shorthand flag: 'Ã' in -é; run 'brief <command> --help'",
			wantNew:  "brief new: unknown shorthand flag: 'Ã' in -é; run 'brief new <type> --help'",
		},
		{
			name:     "a character outside the Latin-1 Supplement block",
			arg:      "-Ж",
			wantLeaf: "brief start: unknown shorthand flag: 'Ã' in -Ж; run 'brief start <feature>'",
			wantRoot: "brief: unknown shorthand flag: 'Ã' in -Ж; run 'brief <command> --help'",
			wantNew:  "brief new: unknown shorthand flag: 'Ã' in -Ж; run 'brief new <type> --help'",
		},
		{
			name:     "the accented character again, after a leading defined -h",
			arg:      "-hé",
			wantLeaf: "brief start: unknown shorthand flag: 'Ã' in -é; run 'brief start <feature>'",
			wantRoot: "brief: unknown shorthand flag: 'Ã' in -é; run 'brief <command> --help'",
			wantNew:  "brief new: unknown shorthand flag: 'Ã' in -é; run 'brief new <type> --help'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()

			var leafStdout, leafStderr bytes.Buffer
			leafErr := cli.Run(t.Context(), wd, []string{"start", tt.arg, "demo"}, nil, &leafStdout, &leafStderr)
			require.ErrorIs(t, leafErr, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(leafErr))
			assert.Empty(t, leafStdout.String())
			leafLine := oneLine(t, &leafStderr)
			assert.Equal(t, tt.wantLeaf, leafLine)

			var rootStdout, rootStderr bytes.Buffer
			rootErr := cli.Run(t.Context(), wd, []string{tt.arg}, nil, &rootStdout, &rootStderr)
			require.ErrorIs(t, rootErr, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(rootErr))
			assert.Empty(t, rootStdout.String())
			rootLine := oneLine(t, &rootStderr)
			assert.Equal(t, tt.wantRoot, rootLine)

			var newStdout, newStderr bytes.Buffer
			newErr := cli.Run(t.Context(), wd, []string{"new", tt.arg}, nil, &newStdout, &newStderr)
			require.ErrorIs(t, newErr, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(newErr))
			assert.Empty(t, newStdout.String())
			newLine := oneLine(t, &newStderr)
			assert.Equal(t, tt.wantNew, newLine)

			leafMsg := flagMessagePart(t, leafLine, "brief start: ")
			rootMsg := flagMessagePart(t, rootLine, "brief: ")
			newMsg := flagMessagePart(t, newLine, "brief new: ")

			assert.Equal(t, leafMsg, rootMsg)
			assert.Equal(t, leafMsg, newMsg)
		})
	}
}

// flagMessagePart extracts the message between "brief <path>: " and the
// trailing "; run '<invocation>'".
func flagMessagePart(t *testing.T, line, prefix string) string {
	t.Helper()

	rest, ok := strings.CutPrefix(line, prefix)
	require.True(t, ok, "expected %q to start with %q", line, prefix)

	msg, _, ok := strings.Cut(rest, "; run '")
	require.True(t, ok, "expected %q to contain \"; run '\"", rest)

	return msg
}

func Test_accepts_the_double_dash_help_flag_on_every_leaf(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "start --help", args: []string{"start", "--help"}},
		{name: "finish --help", args: []string{"finish", "--help"}},
		{name: "status --help", args: []string{"status", "--help"}},
		{name: "check --help", args: []string{"check", "--help"}},
		{name: "new feature --help", args: []string{"new", "feature", "--help"}},
		{name: "new step --help", args: []string{"new", "step", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tt.args, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Equal(t, 0, cli.ExitCode(err))
			assert.Empty(t, stderr.String())
		})
	}
}
