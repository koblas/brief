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

// stepTitle is the heading writeStatusStep writes for id: deliberately
// distinct from id itself, unlike an earlier fixture that wrote "# <id>" —
// a NEXT-column assertion against that fixture proved only that the id was
// echoed twice, never that the table's title column carries the step's own
// markdown.Title.
func stepTitle(id string) string {
	return "Implement " + id
}

// writeStatusStep writes one step file for feature under wd's default
// feature-directory layout, with a heading distinct from its id (stepTitle).
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
		"# " + stepTitle(id) + "\n\n" +
		"## Scenario\n\nsome acceptance text\n\n" +
		"## Implementation Plan\n\n- [ ] a task\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, name), []byte(step), 0o600))
}

// newStatusFixture writes three features under wd's default layout:
// "alpha" (1/3 done, next SCENARIO-02, nothing blocked), "beta" (3/3 done,
// complete) and "gamma" (1/4 done, next SCENARIO-02, one step blocked on
// gamma's own unfinished SCENARIO-02).
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

func Test_status_prints_the_table_and_nothing_else(t *testing.T) {
	wd := newStatusFixture(t)
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"alpha    1/3   0        SCENARIO-02  "+stepTitle("SCENARIO-02")+"\n"+
		"beta     3/3   0        (complete)\n"+
		"gamma    1/4   1        SCENARIO-02  "+stepTitle("SCENARIO-02")+"\n",
		stdout.String())
	assert.Equal(t, "brief status: 3 features: 2 in progress, 1 complete, 0 malformed\n", stderr.String())
}

// Test_status_prints_a_table_for_a_single_feature pins that the header
// prints even for one row — R9 draws the "no header" line at zero rows, not
// at one.
func Test_status_prints_a_table_for_a_single_feature(t *testing.T) {
	wd := t.TempDir()
	writeStatusStep(t, wd, "alpha", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	writeStatusStep(t, wd, "alpha", "SCENARIO-02.md", "SCENARIO-02", "open", nil)

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"alpha    1/2   0        SCENARIO-02  "+stepTitle("SCENARIO-02")+"\n",
		stdout.String())
	assert.Equal(t, "brief status: 1 feature: 1 in progress, 0 complete, 0 malformed\n", stderr.String())
}

// Test_status_shows_the_complete_marker_for_a_completed_feature pins
// "(complete)" in the NEXT column for a feature whose steps are all done.
func Test_status_shows_the_complete_marker_for_a_completed_feature(t *testing.T) {
	wd := t.TempDir()
	writeStatusStep(t, wd, "alpha", "SCENARIO-01.md", "SCENARIO-01", "done", nil)
	writeStatusStep(t, wd, "alpha", "SCENARIO-02.md", "SCENARIO-02", "done", nil)

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"alpha    2/2   0        (complete)\n",
		stdout.String())
	assert.Equal(t, "brief status: 1 feature: 0 in progress, 1 complete, 0 malformed\n", stderr.String())
}

// Test_status_says_no_features_were_found_when_the_feature_root_is_absent
// is the R14 "nothing to return is not an error" case for a repository
// that has never run brief: no docs/specifications directory at all. The
// exact-match assertion on stderr is itself the "no summary line" proof: a
// summary line appended would fail this equality.
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
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"alpha    1/3   0        SCENARIO-02  "+stepTitle("SCENARIO-02")+"\n"+
		"beta     3/3   0        (complete)\n"+
		"gamma    1/4   1        SCENARIO-02  "+stepTitle("SCENARIO-02")+"\n",
		stdoutWithout.String())

	wdWith := newStatusFixture(t)
	writeMalformedStatusFeature(t, wdWith, "delta")
	var stdoutWith, stderrWith bytes.Buffer

	errWith := cli.Run(t.Context(), wdWith, []string{"status"}, nil, &stdoutWith, &stderrWith)

	require.NoError(t, errWith)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"alpha    1/3   0        SCENARIO-02  "+stepTitle("SCENARIO-02")+"\n"+
		"beta     3/3   0        (complete)\n"+
		"delta    -     -        (malformed, see below)\n"+
		"gamma    1/4   1        SCENARIO-02  "+stepTitle("SCENARIO-02")+"\n",
		stdoutWith.String())
}

// Test_status_names_the_reason_for_a_malformed_feature_on_stderr pins the
// exact stderr frame: "brief status: <feature>: <rel path>: <detail>;
// <fix>", no "(no files changed)" tail (that promise belongs to a refusal
// that changed nothing; status never wrote anything to begin with), and
// exit 0.
func Test_status_names_the_reason_for_a_malformed_feature_on_stderr(t *testing.T) {
	wd := t.TempDir()
	writeMalformedStatusFeature(t, wd, "delta")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Equal(t, ""+
		"brief status: delta: "+filepath.Join("docs", "specifications", "delta", "SCENARIO-01.md")+
		": no frontmatter found; run 'brief check delta' to list every fault\n"+
		"brief status: 1 feature: 0 in progress, 0 complete, 1 malformed\n",
		stderr.String())
	assert.NotContains(t, stderr.String(), "(no files changed)")
}

// Test_status_on_a_repository_whose_only_feature_is_malformed_prints_a_row_not_the_no_features_notice
// is the 10↔11 interaction most likely to rot: SCENARIO-10's notice keys
// on len(rows) == 0, and a malformed feature always yields a row, so the
// two must stay mutually exclusive. The table still prints its header even
// though every row is malformed.
func Test_status_on_a_repository_whose_only_feature_is_malformed_prints_a_row_not_the_no_features_notice(t *testing.T) {
	wd := t.TempDir()
	writeMalformedStatusFeature(t, wd, "delta")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"delta    -     -        (malformed, see below)\n",
		stdout.String())
	assert.NotContains(t, stderr.String(), "no features found")
	assert.Equal(t, 2, bytes.Count(stderr.Bytes(), []byte("\n")))
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
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"delta    -     -        (malformed, see below)\n",
		stdout.String())
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

// Test_status_writes_one_stderr_line_per_malformed_feature_then_the_summary
// pins the 1:1 row-to-line mapping for the malformed lines, plus the
// summary line that always follows them: two malformed features out of
// four produce exactly three stderr lines, in row order, summary last.
func Test_status_writes_one_stderr_line_per_malformed_feature_then_the_summary(t *testing.T) {
	wd := newStatusFixture(t)
	writeMalformedStatusFeature(t, wd, "delta")
	writeMalformedStatusFeature(t, wd, "epsilon")

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 3, bytes.Count(stderr.Bytes(), []byte("\n")))

	lines := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
	require.Len(t, lines, 3)
	assert.Contains(t, lines[0], filepath.Join("docs", "specifications", "delta", "SCENARIO-01.md"))
	assert.Contains(t, lines[1], filepath.Join("docs", "specifications", "epsilon", "SCENARIO-01.md"))
	assert.Equal(t, "brief status: 5 features: 2 in progress, 1 complete, 2 malformed", lines[2])
}

// Test_status_writes_the_table_before_the_malformed_lines_and_the_summary
// passes ONE shared buffer as both stdout and stderr to cli.Run: separate
// buffers cannot observe interleaving at all, since each only ever sees its
// own writes in isolation. Only a shared buffer pins that the table is
// fully written (and flushed) before any stderr byte, per the ordering the
// contract requires.
func Test_status_writes_the_table_before_the_malformed_lines_and_the_summary(t *testing.T) {
	wd := t.TempDir()
	writeStatusStep(t, wd, "alpha", "SCENARIO-01.md", "SCENARIO-01", "open", nil)
	writeMalformedStatusFeature(t, wd, "delta")

	var shared bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"status"}, nil, &shared, &shared)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"alpha    0/1   0        SCENARIO-01  "+stepTitle("SCENARIO-01")+"\n"+
		"delta    -     -        (malformed, see below)\n"+
		"brief status: delta: "+filepath.Join("docs", "specifications", "delta", "SCENARIO-01.md")+
		": no frontmatter found; run 'brief check delta' to list every fault\n"+
		"brief status: 2 features: 1 in progress, 0 complete, 1 malformed\n",
		shared.String())
}

// Test_status_summary pins statusSummary's copy over its discriminating
// shapes: "1 feature" singular versus "N features" plural, every bucket
// printed even when it is zero, and a zero-step feature counted as in
// progress rather than complete or malformed.
func Test_status_summary(t *testing.T) {
	cases := []struct {
		name     string
		features map[string]string // feature name -> one step's status ("" for a zero-step feature)
		want     string
	}{
		{
			name:     "one feature is singular",
			features: map[string]string{"alpha": "open"},
			want:     "brief status: 1 feature: 1 in progress, 0 complete, 0 malformed\n",
		},
		{
			name:     "all features complete leaves in-progress and malformed at zero",
			features: map[string]string{"alpha": "done"},
			want:     "brief status: 1 feature: 0 in progress, 1 complete, 0 malformed\n",
		},
		{
			name:     "a zero-step feature counts as in progress, not complete",
			features: map[string]string{"alpha": ""},
			want:     "brief status: 1 feature: 1 in progress, 0 complete, 0 malformed\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := t.TempDir()

			for feature, status := range c.features {
				featureDir := filepath.Join(wd, "docs", "specifications", feature)
				require.NoError(t, os.MkdirAll(featureDir, 0o755))

				if status != "" {
					writeStatusStep(t, wd, feature, "SCENARIO-01.md", "SCENARIO-01", status, nil)
				}
			}

			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, []string{"status"}, nil, &stdout, &stderr)

			require.NoError(t, err)
			assert.Equal(t, c.want, stderr.String())
		})
	}
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
	assert.Contains(t, stdout.String(), "malformed")
	assert.Contains(t, stdout.String(), "exits 0")
}
