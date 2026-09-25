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
	assert.Equal(t,
		"brief new feature: created payments ("+
			filepath.Join("docs", "specifications", "payments", "specification.md")+", "+
			filepath.Join("docs", "specifications", "payments", "STATE.md")+
			"); add a step with 'brief new step payments'\n",
		stderr.String())
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
		"brief new feature: "+filepath.Join("docs", "specifications", "payments")+
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
	assert.Equal(t, "brief: no command given; expected one of: new, start, finish, status, check, init, doctor, uninstall", oneLine(t, &stderr))
}

func Test_returns_a_usage_error_when_the_command_is_unknown(t *testing.T) {
	tests := []struct {
		name    string
		command string
		stderr  string
	}{
		{
			name:    "unrelated typo",
			command: "bogus",
			stderr:  `brief: unknown command "bogus"; expected one of: new, start, finish, status, check, init, doctor, uninstall`,
		},
		{
			name:    "near-miss of a real command",
			command: "startt",
			stderr:  `brief: unknown command "startt"; expected one of: new, start, finish, status, check, init, doctor, uninstall`,
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
	assert.Equal(t, filepath.Join("..", "..", "specs", "payments")+"\n", stdout.String())
	assert.Equal(t,
		"brief new feature: created payments ("+
			filepath.Join("..", "..", "specs", "payments", "specification.md")+", "+
			filepath.Join("..", "..", "specs", "payments", "NOTES.md")+
			"); add a step with 'brief new step payments'\n",
		stderr.String())
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
	assert.Contains(t, line, ".brief.yaml")
	assert.NotContains(t, line, configPath)
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

// A nil args slice must not fall through to cobra's own os.Args[1:]
// fallback, which would parse the test binary's flags instead of brief's.
func Test_returns_a_usage_error_when_args_are_nil(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, nil, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief: no command given; expected one of: new, start, finish, status, check, init, doctor, uninstall", oneLine(t, &stderr))
}

func Test_prints_root_usage_and_a_nil_error_for_brief_help(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.NotEmpty(t, stdout.String())
}

// The "--help --version" row pins that the first argument alone decides
// which flag's error fires: "--version" trailing after "--help" never
// classifies as the version flag's own error.
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

// Control for the table above: "--help"/"-h" alone, with no trailing
// argument, still routes to root help.
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

func Test_returns_a_usage_error_for_an_unknown_double_dash_flag_at_the_root(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"--bogus"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief: unknown flag: --bogus; run 'brief <command> --help'", oneLine(t, &stderr))
}

func Test_returns_a_usage_error_for_an_unknown_double_dash_flag_under_new(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "--bogus"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief new: unknown flag: --bogus; run 'brief new <type> --help'", oneLine(t, &stderr))
}

// A go test binary's debug.ReadBuildInfo reports "(devel)", the same
// bytes as the missing-build-info fallback, so this cannot tell them
// apart; both rules are pinned against fake readers in version_internal_test.go.
func Test_version_flag_through_Run_prints_one_brief_line_to_stdout(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"--version"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, "brief (devel)\n", stdout.String())
}

// "--version --json" is not a member of this family: scanJSONFlag strips
// "--json" ahead of dispatch, relaxing the sole-argument rule instead of
// tripping it (see Test_version_with_json_relaxes_the_sole_argument_rule).
func Test_reports_a_version_flag_with_trailing_arguments_as_taking_no_arguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "--version extra", args: []string{"--version", "extra"}},
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

// The value check fires before any trailing argument is looked at:
// "--version=x extra" and "--version= --version" both report the value
// error, never the trailing-argument error the table above pins.
func Test_reports_a_version_flag_with_a_value_as_taking_no_value(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "--version=x", args: []string{"--version=x"}},
		{name: "--version=", args: []string{"--version="}},
		{name: "--version=x extra", args: []string{"--version=x", "extra"}},
		{name: "--version= --version", args: []string{"--version=", "--version"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, tt.args, nil, &stdout, &stderr)

			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Empty(t, stdout.String())
			assert.Equal(t, `brief: '--version' takes no value; run 'brief --version'`, oneLine(t, &stderr))
		})
	}
}

// A standalone "-" is pflag's own convention for stdin, never a flag, so
// it keeps the ordinary unknown-command wording rather than the
// flag-shaped wording the table above pins.
func Test_treats_a_bare_dash_as_a_plain_unknown_command(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "root",
			args:    []string{"-"},
			wantErr: `brief: unknown command "-"; expected one of: new, start, finish, status, check, init, doctor, uninstall`,
		},
		{
			name:    "new",
			args:    []string{"new", "-"},
			wantErr: `brief new: unknown type "-"; expected one of: feature, step`,
		},
		{
			name:    "help",
			args:    []string{"help", "-"},
			wantErr: `brief help: unknown command "-"; expected one of: new, start, finish, status, check, init, doctor, uninstall`,
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
