package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_creates_the_step_and_prints_its_path(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	require.NoError(t, cli.Run(t.Context(), wd, []string{"new", "feature", "payments"}, &bytes.Buffer{}, &bytes.Buffer{}))

	err := cli.Run(t.Context(), wd, []string{"new", "step", "payments"}, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, "docs/specifications/payments/SCENARIO-01.md\n", stdout.String())
}

func Test_returns_a_usage_error_when_no_feature_is_given_for_step(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "step"}, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief new step: no feature given; run 'brief new step <feature>'", oneLine(t, &stderr))
}

func Test_returns_a_usage_error_when_there_are_too_many_arguments_for_step(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "step", "a", "b"}, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief new step: too many arguments; run 'brief new step <feature>'", oneLine(t, &stderr))
}

func Test_returns_a_usage_error_when_a_flag_is_not_defined_for_step(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "step", "-x", "p"}, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief new step: flag provided but not defined: -x; run 'brief new step <feature>'", oneLine(t, &stderr))
}

func Test_prints_usage_to_stdout_when_help_is_requested_for_new_step(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "step", "--help"}, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.NotEmpty(t, stdout.String())
}

func Test_refuses_on_one_line_for_an_unknown_feature_for_step(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "step", "payments"}, &stdout, &stderr)

	require.ErrorIs(t, err, scaffold.ErrNoSuchFeature)
	assert.Empty(t, stdout.String())
	assert.Equal(t, 1, cli.ExitCode(err))

	line := oneLine(t, &stderr)
	assert.Contains(t, line, filepath.Join(wd, "docs", "specifications", "payments"))
	assert.True(t, strings.HasSuffix(line, "(no files changed)"), "line %q must end with (no files changed)", line)
}

func Test_refuses_on_one_line_when_the_specification_has_no_progress_heading(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "payments")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte("# payments\n\nno progress list here.\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(""), 0o600))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "step", "payments"}, &stdout, &stderr)

	require.ErrorIs(t, err, scaffold.ErrNoProgressHeading)
	assert.Empty(t, stdout.String())
	assert.Equal(t, 1, cli.ExitCode(err))

	line := oneLine(t, &stderr)
	assert.Contains(t, line, filepath.Join(featureDir, "specification.md"))
	assert.Contains(t, line, "## BDD Acceptance Progress")
	assert.True(t, strings.HasSuffix(line, "(no files changed)"), "line %q must end with (no files changed)", line)
}

func Test_refuses_on_one_line_for_an_invalid_step_file_pattern_from_an_ancestor_config(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".brief.yaml"), []byte("step-file-pattern: \"SCENARIO-%s.md\"\n"), 0o600))

	var setupStdout, setupStderr bytes.Buffer
	require.NoError(t, cli.Run(t.Context(), root, []string{"new", "feature", "payments"}, &setupStdout, &setupStderr))

	wd := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(wd, 0o755))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"new", "step", "payments"}, &stdout, &stderr)

	require.ErrorIs(t, err, stepfile.ErrInvalidPattern)
	assert.Empty(t, stdout.String())
	assert.Equal(t, 1, cli.ExitCode(err))

	line := oneLine(t, &stderr)
	assert.Contains(t, line, "step-file-pattern")
	assert.Contains(t, line, "SCENARIO-%s.md")
	assert.True(t, strings.HasSuffix(line, "(no files changed)"), "line %q must end with (no files changed)", line)
}
