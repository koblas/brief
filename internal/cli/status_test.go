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

// Test_status_shows_a_dash_for_a_completed_feature pins R14's "-" in the
// next-step field for a feature whose steps are all done, asserted
// end-to-end through the CLI rather than only at the FeatureStatus level.
func Test_status_shows_a_dash_for_a_completed_feature(t *testing.T) {
	wd := t.TempDir()
	writeStatusStep(t, wd, "alpha", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	writeStatusStep(t, wd, "alpha", "SCENARIO-02.md", "SCENARIO-02", "done", nil)

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, "alpha 2/2 - 0\n", stdout.String())
}

// Test_status_says_no_features_were_found_when_the_feature_root_is_absent
// is the R14 "nothing to return is not an error" case for a repository
// that has never run brief: no docs/specifications directory at all.
func Test_status_says_no_features_were_found_when_the_feature_root_is_absent(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t,
		"brief status: no features found in docs/specifications; run 'brief new feature <name>' to create one\n",
		stderr.String())
}

// Test_status_says_no_features_were_found_when_the_feature_root_is_empty
// covers the same notice when docs/specifications exists but holds no
// feature directories — stdout and the exit code are already empty/0 on
// arrival; this test is red only on the stderr line.
func Test_status_says_no_features_were_found_when_the_feature_root_is_empty(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications"), 0o755))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t,
		"brief status: no features found in docs/specifications; run 'brief new feature <name>' to create one\n",
		stderr.String())
}

// writeMalformedStatusFeature writes one step file with no frontmatter at
// all under feature's default layout, so featureDirFor(wd, feature) reads
// as a malformed feature rather than a conforming one.
func writeMalformedStatusFeature(t *testing.T, wd, feature string) {
	t.Helper()

	featureDir := filepath.Join(wd, "docs", "specifications", feature)
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte("no frontmatter here\n"), 0o600))
}

// Test_status_reports_the_other_features_unchanged_when_one_is_malformed
// is the control arm for SCENARIO-11's core claim: two runs differing in
// exactly one variable — the malformed feature present or absent — both
// asserted against literal expected stdout strings. Never derive one
// expectation from the other run's output: that would only prove the
// renderer is consistent with itself, not that the malformed feature left
// the rest alone.
func Test_status_reports_the_other_features_unchanged_when_one_is_malformed(t *testing.T) {
	wdWithout := newStatusFixture(t)
	var stdoutWithout, stderrWithout bytes.Buffer

	errWithout := cli.Run(t.Context(), wdWithout, []string{"status"}, nil, &stdoutWithout, &stderrWithout)

	require.NoError(t, errWithout)
	assert.Equal(t, ""+
		"alpha 1/3 SCENARIO-02 0\n"+
		"beta 3/3 - 0\n"+
		"gamma 1/4 SCENARIO-02 1\n",
		stdoutWithout.String())

	wdWith := newStatusFixture(t)
	writeMalformedStatusFeature(t, wdWith, "delta")
	var stdoutWith, stderrWith bytes.Buffer

	errWith := cli.Run(t.Context(), wdWith, []string{"status"}, nil, &stdoutWith, &stderrWith)

	require.NoError(t, errWith)
	assert.Equal(t, ""+
		"alpha 1/3 SCENARIO-02 0\n"+
		"beta 3/3 - 0\n"+
		"delta ! ! !\n"+
		"gamma 1/4 SCENARIO-02 1\n",
		stdoutWith.String())
}

// Test_status_names_the_reason_for_a_malformed_feature_on_stderr pins the
// exact stderr copy: an absolute path, no "(no files changed)" tail (that
// promise belongs to a refusal that changed nothing; status never wrote
// anything to begin with), and exit 0.
func Test_status_names_the_reason_for_a_malformed_feature_on_stderr(t *testing.T) {
	wd := t.TempDir()
	writeMalformedStatusFeature(t, wd, "delta")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Equal(t,
		"brief status: "+filepath.Join(wd, "docs", "specifications", "delta")+
			": no frontmatter found; fix its frontmatter, or run 'brief new step delta' to scaffold a conforming step file\n",
		stderr.String())
	assert.NotContains(t, stderr.String(), "(no files changed)")
}

// Test_status_on_a_repository_whose_only_feature_is_malformed_prints_a_row_not_the_no_features_notice
// is the 10↔11 interaction most likely to rot: SCENARIO-10's notice keys
// on len(rows) == 0, and a malformed feature always yields a row, so the
// two must stay mutually exclusive.
func Test_status_on_a_repository_whose_only_feature_is_malformed_prints_a_row_not_the_no_features_notice(t *testing.T) {
	wd := t.TempDir()
	writeMalformedStatusFeature(t, wd, "delta")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Equal(t, "delta ! ! !\n", stdout.String())
	assert.NotContains(t, stderr.String(), "no features found")
	assert.Equal(t, 1, bytes.Count(stderr.Bytes(), []byte("\n")))
}

// Test_status_on_a_repository_whose_only_entry_is_a_symlink_prints_a_row_not_the_no_features_notice
// is the second half of that seam: the symlink case must not be swallowed
// by the notice either.
func Test_status_on_a_repository_whose_only_entry_is_a_symlink_prints_a_row_not_the_no_features_notice(t *testing.T) {
	wd := t.TempDir()
	realDir := filepath.Join(wd, "real-delta")
	require.NoError(t, os.MkdirAll(realDir, 0o755))

	specs := filepath.Join(wd, "docs", "specifications")
	require.NoError(t, os.MkdirAll(specs, 0o755))
	require.NoError(t, os.Symlink(realDir, filepath.Join(specs, "delta")))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Equal(t, "delta ! ! !\n", stdout.String())
	assert.NotContains(t, stderr.String(), "no features found")
}

// Test_status_on_a_repository_whose_only_entry_is_a_regular_file_still_says_no_features_found
// is the deliberate opposite, pinning SCENARIO-10's decision: a regular
// file produces no row, so the repository still reads as having zero
// features. Together with the two tests above, this fixes the boundary
// the entry-type switch must draw.
func Test_status_on_a_repository_whose_only_entry_is_a_regular_file_still_says_no_features_found(t *testing.T) {
	wd := t.TempDir()
	specs := filepath.Join(wd, "docs", "specifications")
	require.NoError(t, os.MkdirAll(specs, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(specs, "README.md"), []byte("not a feature\n"), 0o600))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t,
		"brief status: no features found in docs/specifications; run 'brief new feature <name>' to create one\n",
		stderr.String())
}

// Test_status_writes_one_stderr_line_per_malformed_feature pins the 1:1
// row-to-line mapping: two malformed features out of four produce exactly
// two stderr lines, in row order.
func Test_status_writes_one_stderr_line_per_malformed_feature(t *testing.T) {
	wd := newStatusFixture(t)
	writeMalformedStatusFeature(t, wd, "delta")
	writeMalformedStatusFeature(t, wd, "epsilon")

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 2, bytes.Count(stderr.Bytes(), []byte("\n")))

	lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], filepath.Join(wd, "docs", "specifications", "delta"))
	assert.Contains(t, lines[1], filepath.Join(wd, "docs", "specifications", "epsilon"))
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
	assert.Contains(t, stdout.String(), "!")
	assert.Contains(t, stdout.String(), "exits 0")
}
