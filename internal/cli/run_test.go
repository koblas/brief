package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/koblas/brief/internal/platform/config"
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
	assert.Equal(t, "brief: no command given; expected one of: new, start, finish", oneLine(t, &stderr))
}

func Test_returns_a_usage_error_when_the_command_is_unknown(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"bogus"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief: unknown command "bogus"; expected one of: new, start, finish`, oneLine(t, &stderr))
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
	assert.Equal(t, "brief new feature: flag provided but not defined: -x; run 'brief new feature <name>'", oneLine(t, &stderr))
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
