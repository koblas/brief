package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oneLine asserts buf holds exactly one non-empty line: no trailing blank
// line, no second line of stack-trace-shaped noise.
func oneLine(t *testing.T, buf *bytes.Buffer) string {
	t.Helper()

	s := buf.String()
	require.NotEmpty(t, s)
	assert.Equal(t, 1, strings.Count(s, "\n"))

	return strings.TrimSuffix(s, "\n")
}

func Test_creates_the_feature_and_prints_its_path(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "feature", "payments"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, "docs/specifications/payments\n", stdout.String())
	assert.DirExists(t, filepath.Join(wd, "docs", "specifications"))
	assert.DirExists(t, filepath.Join(wd, "docs", "specifications", "payments"))
}

func Test_refuses_on_one_line_when_the_feature_already_exists(t *testing.T) {
	wd := t.TempDir()

	require.NoError(t, cli.Run(t.Context(), wd, []string{"new", "feature", "payments"}, nil, &bytes.Buffer{}, &bytes.Buffer{}))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "feature", "payments"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, scaffold.ErrFeatureExists)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	line := oneLine(t, &stderr)
	assert.Equal(t,
		"brief new feature: "+filepath.Join(wd, "docs", "specifications", "payments")+
			": feature already exists; run 'brief new step payments' to add a step to it, or choose a different name (no files changed)",
		line)
}

func Test_returns_a_usage_error_when_the_feature_name_contains_whitespace(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "feature", "pay ments"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief new feature: name "pay ments" contains whitespace; run 'brief new feature <name>' with a name containing no whitespace`, oneLine(t, &stderr))
	assert.NoDirExists(t, filepath.Join(wd, "docs", "specifications", "pay ments"))
	assert.NoDirExists(t, filepath.Join(wd, "docs", "specifications"))
}

func Test_returns_a_usage_error_when_the_feature_name_is_empty(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "feature", ""}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief new feature: name is empty; run 'brief new feature <name>' with a non-empty name", oneLine(t, &stderr))
	assert.NoDirExists(t, filepath.Join(wd, "docs", "specifications"))
}

func Test_keeps_the_refusal_on_one_line_when_the_feature_name_contains_a_newline(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "feature", "pay\nments"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	line := oneLine(t, &stderr)
	assert.Contains(t, line, `"pay\nments"`)
}

func Test_returns_a_usage_error_when_no_name_is_given(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "feature"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief new feature: no name given; run 'brief new feature <name>'", oneLine(t, &stderr))
}

func Test_returns_a_usage_error_when_no_command_is_given(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief: no command given; expected one of: new, start, finish, status, check", oneLine(t, &stderr))
}

// Test_returns_a_usage_error_when_the_command_is_unknown pins R7's "no Did
// you mean" clause: a one-edit near-miss of a real command name gets the
// same one-line error as an unrelated typo, never a cobra suggestion —
// DisableSuggestions already makes this so, so that row is green on
// arrival.
func Test_returns_a_usage_error_when_the_command_is_unknown(t *testing.T) {
	tests := []struct {
		name    string
		command string
		stderr  string
	}{
		{
			name:    "unrelated typo",
			command: "bogus",
			stderr:  `brief: unknown command "bogus"; expected one of: new, start, finish, status, check`,
		},
		{
			name:    "near-miss of a real command",
			command: "startt",
			stderr:  `brief: unknown command "startt"; expected one of: new, start, finish, status, check`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, []string{tc.command}, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Empty(t, stdout.String())
			assert.Equal(t, tc.stderr, oneLine(t, &stderr))
		})
	}
}

func Test_returns_a_usage_error_when_no_type_is_given(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief new: no type given; expected one of: feature, step", oneLine(t, &stderr))
}

func Test_returns_a_usage_error_when_the_type_is_unknown(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "widget", "x"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief new: unknown type "widget"; expected one of: feature, step`, oneLine(t, &stderr))
}

func Test_returns_a_usage_error_when_a_flag_is_not_defined(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "feature", "-x", "p"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief new feature: unknown shorthand flag: 'x' in -x; run 'brief new feature <name>'", oneLine(t, &stderr))
}

func Test_returns_a_usage_error_when_there_are_too_many_arguments(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "feature", "a", "b"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief new feature: too many arguments; run 'brief new feature <name>'", oneLine(t, &stderr))
}

func Test_creates_the_feature_where_an_ancestor_config_directs(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".brief.yaml"),
		[]byte("feature-directory: specs\nstate-file: NOTES.md\n"), 0o600))
	wd := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(wd, 0o755))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "feature", "payments"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.FileExists(t, filepath.Join(root, "specs", "payments", "NOTES.md"))
}

func Test_refuses_on_one_line_when_the_config_file_is_invalid(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("not-a-real-key: true\n"), 0o600))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), root, []string{"new", "feature", "payments"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, config.ErrInvalidConfig)
	assert.Empty(t, stdout.String())
	assert.Equal(t, 1, strings.Count(stderr.String(), "\n"))

	line := strings.TrimSuffix(stderr.String(), "\n")
	assert.Contains(t, line, configPath)
	assert.True(t, strings.HasSuffix(line, "(no files changed)"), "line %q must end with (no files changed)", line)

	assert.NoDirExists(t, filepath.Join(root, "docs", "specifications", "payments"))
}

func Test_prints_usage_to_stdout_when_help_is_requested_for_the_binary(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.NotEmpty(t, stdout.String())
}

func Test_prints_usage_to_stdout_when_help_is_requested_for_the_subcommand(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "feature", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.NotEmpty(t, stdout.String())
}

// Test_returns_a_usage_error_when_args_are_nil guards the cobra port's
// os.Args fallback: cobra's *Command reads os.Args[1:] itself when
// SetArgs receives a nil slice, which would parse the test binary's own
// flags instead of brief's. Run must always hand cobra a non-nil copy, so
// a nil args slice here still reaches the ordinary no-command usage error.
func Test_returns_a_usage_error_when_args_are_nil(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, nil, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief: no command given; expected one of: new, start, finish, status, check", oneLine(t, &stderr))
}

// Test_prints_root_usage_and_a_nil_error_for_brief_help pins today's
// dispatch: "brief help" with no topic prints the root usage text, exit 0.
func Test_prints_root_usage_and_a_nil_error_for_brief_help(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.NotEmpty(t, stdout.String())
}

// Test_reports_a_help_flag_with_trailing_arguments_as_taking_no_arguments
// pins that root's "-h"/"--help" routing to cmd.Help() applies only when
// it is the sole argument, matching runNew's own sole-argument rule
// (new.go's runNew) and the help stub's own -h/--help rule: any trailing
// argument alongside "-h"/"--help" is reported as that flag taking no
// arguments, naming it exactly as typed and pointing at "brief help
// <command>" — never as an unknown command naming "-h"/"--help" itself. The
// "--help --version" row pins that the first argument alone decides which
// flag's error fires (R8): "--version" trailing after "--help" never
// classifies as the version flag's own trailing-argument error.
func Test_reports_a_help_flag_with_trailing_arguments_as_taking_no_arguments(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "--help bogus",
			args:    []string{"--help", "bogus"},
			wantErr: `brief: '--help' takes no arguments; run 'brief help <command>'`,
		},
		{
			name:    "-h bogus",
			args:    []string{"-h", "bogus"},
			wantErr: `brief: '-h' takes no arguments; run 'brief help <command>'`,
		},
		{
			name:    "--help start",
			args:    []string{"--help", "start"},
			wantErr: `brief: '--help' takes no arguments; run 'brief help <command>'`,
		},
		{
			name:    "--help --version",
			args:    []string{"--help", "--version"},
			wantErr: `brief: '--help' takes no arguments; run 'brief help <command>'`,
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
			assert.Equal(t, tt.wantErr, oneLine(t, &stderr))
		})
	}
}

// Test_still_prints_root_help_for_a_bare_help_flag is the control arm for
// the table above: "--help"/"-h" alone, with no trailing argument, still
// routes to root help — nil error, exit 0, non-empty stdout — proving the
// rejection above is about trailing arguments, not about the flag itself.
func Test_still_prints_root_help_for_a_bare_help_flag(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "--help", args: []string{"--help"}},
		{name: "-h", args: []string{"-h"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tt.args, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Equal(t, 0, cli.ExitCode(err))
			assert.Empty(t, stderr.String())
			assert.NotEmpty(t, stdout.String())
		})
	}
}

// Test_returns_a_usage_error_when_the_root_command_is_a_single_dash_flag
// pins that "-x" at the root is reported the way pflag itself would report
// an undefined shorthand flag on a leaf — root disables cobra's flag
// parsing and so never reaches that frame itself — pointing at "brief
// <command> --help" rather than naming a bogus command.
func Test_returns_a_usage_error_when_the_root_command_is_a_single_dash_flag(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"-x"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief: unknown shorthand flag: 'x' in -x; run 'brief <command> --help'", oneLine(t, &stderr))
}

// Test_returns_a_usage_error_when_the_new_type_is_a_single_dash_flag pins
// the same wording for "-x" under "new", pointing at "brief new <type>
// --help".
func Test_returns_a_usage_error_when_the_new_type_is_a_single_dash_flag(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "-x"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief new: unknown shorthand flag: 'x' in -x; run 'brief new <type> --help'", oneLine(t, &stderr))
}

// Test_returns_a_usage_error_for_an_unknown_double_dash_flag_at_the_root
// pins the double-dash shape of the same rule, using a plausible-looking
// flag ("--bogus") that brief does not define, to prove the wording is
// generic rather than specific to any one bogus name. "--version" is not
// used here since it is now its own argKind (see version_internal_test.go).
func Test_returns_a_usage_error_for_an_unknown_double_dash_flag_at_the_root(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"--bogus"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief: unknown flag: --bogus; run 'brief <command> --help'", oneLine(t, &stderr))
}

// Test_returns_a_usage_error_for_an_unknown_double_dash_flag_under_new is
// the "new" shape of the same rule.
func Test_returns_a_usage_error_for_an_unknown_double_dash_flag_under_new(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "--bogus"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief new: unknown flag: --bogus; run 'brief new <type> --help'", oneLine(t, &stderr))
}

// Test_version_flag_through_Run_prints_one_brief_line_to_stdout is the only
// test proving cli.Run wires debug.ReadBuildInfo into "--version" — see
// version_internal_test.go for the fallback and pass-through tables against
// fake readers. A go test binary's own debug.ReadBuildInfo reports
// Main.Version as "(devel)", so the exact stdout line is assertable here
// too, by way of R2's fallback rather than R1's pass-through.
func Test_version_flag_through_Run_prints_one_brief_line_to_stdout(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"--version"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, "brief (devel)\n", stdout.String())
}

// Test_reports_a_version_flag_with_trailing_arguments_as_taking_no_arguments
// pins that root's "--version" sole-argument handling (see
// version_internal_test.go) applies only when it is the sole argument: any
// trailing argument alongside "--version" is reported as that flag taking
// no arguments, pointing at "brief --version" — never as the unknown-flag
// wording argVersionFlag's msg would otherwise carry (R4).
//
// The four rows are one behavior — any trailing argument, whatever its
// shape — not four independent rules: "extra" and "--json" are the
// specification's own examples, and "--help"/"--version" pin that args[1]
// is never classified at all (R8), a shape no mutation in this arm can
// discriminate. No mutation reddens one row without reddening all four.
//
// Mutation-verified, restored byte-identical after each: widening the
// argVersionFlag arm's guard from "len(args) == 1" to "len(args) >= 1"
// reddens every row here (nil error, version printed instead of the usage
// error); changing that arm's run hint from "brief --version" to "brief
// help <command>" reddens every row here on the hint text while the sibling
// table's "--help --version" control row (R8) stays green, proving the two
// arms report independently.
func Test_reports_a_version_flag_with_trailing_arguments_as_taking_no_arguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "--version extra", args: []string{"--version", "extra"}},
		{name: "--version --json", args: []string{"--version", "--json"}},
		{name: "--version --help", args: []string{"--version", "--help"}},
		{name: "--version --version", args: []string{"--version", "--version"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tt.args, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Empty(t, stdout.String())
			assert.Equal(t, `brief: '--version' takes no arguments; run 'brief --version'`, oneLine(t, &stderr))
		})
	}
}

// Test_treats_a_bare_dash_as_a_plain_unknown_command is the control arm
// for classifyDashArg's argNotFlag case: a standalone "-" is pflag's own
// convention for stdin, never a flag (parseArgs treats len(s) == 1 the
// same as no "-" prefix at all), so it keeps the ordinary
// unknown-command/unknown-type wording rather than the flag-shaped wording
// above.
//
// Mutation-verified: making classifyDashArg return argUnknownFlag for "-"
// reds all three rows, the help row included.
func Test_treats_a_bare_dash_as_a_plain_unknown_command(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "root",
			args:    []string{"-"},
			wantErr: `brief: unknown command "-"; expected one of: new, start, finish, status, check`,
		},
		{
			name:    "new",
			args:    []string{"new", "-"},
			wantErr: `brief new: unknown type "-"; expected one of: feature, step`,
		},
		{
			name:    "help",
			args:    []string{"help", "-"},
			wantErr: `brief help: unknown command "-"; expected one of: new, start, finish, status, check`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tt.args, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Empty(t, stdout.String())
			assert.Equal(t, tt.wantErr, oneLine(t, &stderr))
		})
	}
}
