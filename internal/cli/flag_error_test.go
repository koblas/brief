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

// Test_rejects_a_single_dash_long_flag_as_one_usage_line_naming_the_command_invocation
// is SCENARIO-04's table: a single-dash spelling of a long flag ("-json",
// "-handoff", "-state", "-help") parses as a pflag shorthand cluster, not
// as the long flag, and is rejected through the same root SetFlagErrorFunc
// frame as SCENARIO-02/03. "-help" and "-handoff" both start with "h",
// cobra's auto help shorthand: pflag consumes it first and reports only
// the residual cluster ("-elp", "-andoff"), the same residual rule
// SCENARIO-03 pinned for "-hx". "-json" and "-state" have no defined first
// letter, so the whole word is quoted.
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

// Test_flattens_a_flag_error_that_embeds_a_newline_to_one_stderr_line pins
// that the root FlagErrorFunc frame runs pflag's own error text through
// flattenOneLine before embedding it: a flag name carrying a literal
// newline (shell-quoted, e.g. $'--fo\no') would otherwise make pflag's
// error itself span two lines, breaking R14's one-line contract.
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

// Test_reports_a_flag_missing_its_value_as_one_usage_line_naming_the_command_invocation
// is SCENARIO-05's table: "finish" is the only leaf with a value-taking flag
// (--handoff, --state, both String); a bare --json can never be "missing its
// value" because pflag gives every Bool an implicit NoOptDefVal. Flag
// parsing runs before runFinish's own argument-count check, so a missing
// value is reported even when no positional was given at all. Expected
// stderr is written out literally per row, per the file's existing rule.
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

// Test_reports_an_empty_flag_value_as_that_flag_being_required pins that
// "--handoff=" / "--state=" are not flag-parse errors: pflag accepts the
// explicit empty value, so parsing succeeds and runFinish's own
// handoffPath == "" / statePath == "" guard reports it as the flag being
// required, in brief's own required-flag wording rather than pflag's
// "needs an argument" wording.
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

// Test_takes_the_next_flag_as_the_value_of_a_flag_missing_its_value pins
// that pflag takes the next token as a flag's value even when that token
// itself starts with "--": "--handoff --state s.md" reads "--state" as
// --handoff's value and "s.md" as a third positional, so the result is
// "too many arguments", never a "needs an argument" error; "--handoff
// --state" with nothing after it reads "--state" as --handoff's value and
// leaves --state itself unset, so the result is "--state is required".
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

// Test_reports_an_undefined_flag_as_a_usage_error_whichever_side_of_help_it_is_on
// is SCENARIO-06's table: pflag's ParseFlags stops at the first bad token,
// so an undefined flag is a usage error through the same root
// SetFlagErrorFunc frame as SCENARIO-02 regardless of where --help/-h falls
// relative to it. Every leaf gets both orders of --help; -h is scoped to
// start only, since cobra's InitDefaultHelpFlag registers -h identically on
// every leaf and a per-leaf -h row would prove nothing more about the
// frame. Expected stderr reuses SCENARIO-02's literals verbatim: --help/-h
// in the args changes nothing about the line pflag reports.
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

// Test_prints_help_for_the_h_shorthand_alone is the control arm for the -h
// rows above: it proves -h alone still prints help (nil error, exit 0,
// empty stderr, the same start prose Test_prints_the_start_usage_for_help
// pins for --help), and that -h's stdout is byte-identical to --help's.
// Without this, "start --bogus -h" would pass even if -h were itself
// undefined, since pflag errors on --bogus before -h is ever parsed.
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

// Test_reports_an_invalid_bool_flag_value_without_leaking_strconv_wording
// pins that a bool flag given a value strconv.ParseBool rejects is
// reported in brief's own wording, naming the value exactly as given —
// including "" for "--json=" — rather than pflag's raw "invalid argument
// %q for %q flag: strconv.ParseBool: parsing %q: invalid syntax". The rule
// lives in the root FlagErrorFunc frame and so applies to any bool flag,
// not one hard-coded name; start's --json is the only bool flag brief
// defines today.
func Test_reports_an_invalid_bool_flag_value_without_leaking_strconv_wording(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "--json=maybe",
			args:       []string{"start", "--json=maybe", "demo"},
			wantStderr: `brief start: invalid value "maybe" for --json (want true or false, or no value); run 'brief start <feature>'`,
		},
		{
			name:       "--json= with an explicit empty value",
			args:       []string{"start", "--json=", "demo"},
			wantStderr: `brief start: invalid value "" for --json (want true or false, or no value); run 'brief start <feature>'`,
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

// Test_accepts_the_double_dash_json_flag is one shape of the control arm
// for the tables above: "--json" differs from a rejected row only in the
// flag's dash count ("-json" -> "--json") and succeeds, proving the
// rejection above is specific to the single-dash spelling rather than to
// the flag or command itself. JSON body is already pinned in
// start_test.go.
func Test_accepts_the_double_dash_json_flag(t *testing.T) {
	wd := newStartFixture(t, "open")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--json", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Empty(t, stderr.String())
}

// Test_accepts_the_double_dash_handoff_and_state_flags is the
// --handoff/--state shape of the control arm: finish's disk effects are
// already pinned in finish_test.go, so this asserts only the observable
// this file's rejected rows share, nil error and exit 0.
func Test_accepts_the_double_dash_handoff_and_state_flags(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
}

// Test_accepts_the_double_dash_help_flag_on_every_leaf is the --help shape
// of the control arm, one row per leaf: help text is already pinned in
// help_test.go's goldens, so each row asserts only nil error, exit 0, and
// empty stderr.
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
