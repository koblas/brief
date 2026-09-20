package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeStatusStep writes one step file for feature under wd's default
// feature-directory layout.
func writeStatusStep(t *testing.T, wd, feature, name, id, status string, dependsOn []string) {
	t.Helper()

	featureDir := filepath.Join(wd, "docs", "specifications", feature)
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	deps := "depends-on: []\n"
	if len(dependsOn) > 0 {
		var sb strings.Builder

		sb.WriteString("depends-on:\n")
		for _, dep := range dependsOn {
			sb.WriteString("  - " + dep + "\n")
		}

		deps = sb.String()
	}

	step := "---\n" +
		"id: " + id + "\n" +
		"status: " + status + "\n" +
		deps +
		"---\n\n" +
		"# " + id + "\n\n" +
		"## Scenario\n\nsome acceptance text\n\n" +
		"## Implementation Plan\n\n- [ ] a task\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, name), []byte(step), 0o600))
}

// newStatusFixture writes three features under wd's default layout:
// "alpha" (1/3 done, next SCENARIO-02, nothing blocked), "beta" (3/3 done,
// no next step) and "gamma" (1/4 done, next SCENARIO-02, one step blocked
// on gamma's own unfinished SCENARIO-02).
func newStatusFixture(t *testing.T) string {
	t.Helper()

	wd := t.TempDir()

	writeStatusStep(t, wd, "alpha", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	writeStatusStep(t, wd, "alpha", "SCENARIO-02.md", "SCENARIO-02", "open", nil)
	writeStatusStep(t, wd, "alpha", "SCENARIO-03.md", "SCENARIO-03", "open", nil)

	writeStatusStep(t, wd, "beta", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	writeStatusStep(t, wd, "beta", "SCENARIO-02.md", "SCENARIO-02", "done", nil)
	writeStatusStep(t, wd, "beta", "SCENARIO-03.md", "SCENARIO-03", "done", nil)

	writeStatusStep(t, wd, "gamma", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	writeStatusStep(t, wd, "gamma", "SCENARIO-02.md", "SCENARIO-02", "open", nil)
	writeStatusStep(t, wd, "gamma", "SCENARIO-03.md", "SCENARIO-03", "open", []string{"SCENARIO-02"})
	writeStatusStep(t, wd, "gamma", "SCENARIO-04.md", "SCENARIO-04", "open", nil)

	return wd
}

func Test_status_prints_one_line_per_feature_and_nothing_else(t *testing.T) {
	wd := newStatusFixture(t)
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, ""+
		"alpha 1/3 SCENARIO-02 0\n"+
		"beta 3/3 - 0\n"+
		"gamma 1/4 SCENARIO-02 1\n",
		stdout.String())
}

// Test_status_prints_one_line_for_a_single_feature is the control arm for
// the no-header/no-legend absence claim above: the same exact-bytes probe
// against a one-feature fixture. A header or a legend would be a constant
// line present at both feature counts; matching exactly at one row and at
// three rows means line count varies only with feature count.
func Test_status_prints_one_line_for_a_single_feature(t *testing.T) {
	wd := t.TempDir()
	writeStatusStep(t, wd, "alpha", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	writeStatusStep(t, wd, "alpha", "SCENARIO-02.md", "SCENARIO-02", "open", nil)

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, "alpha 1/2 SCENARIO-02 0\n", stdout.String())
}

func Test_returns_a_usage_error_when_status_is_given_an_argument(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status", "alpha"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "brief status")
	assert.Equal(t, 1, bytes.Count(stderr.Bytes(), []byte("\n")))
}

func Test_returns_a_usage_error_when_status_is_given_an_undefined_flag(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status", "--bogus"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "brief status")
	assert.Equal(t, 1, bytes.Count(stderr.Bytes(), []byte("\n")))
}

func Test_prints_usage_to_stdout_when_help_is_requested_for_status(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.NotEmpty(t, stdout.String())
}
