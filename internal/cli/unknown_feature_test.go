package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noFilesChangedSuffix is the tail a write command's not-found refusal
// carries; a read command's carries none.
const noFilesChangedSuffix = " (no files changed)"

// newTwoFeatureFixture writes two empty feature directories, "alpha" and
// "beta", and returns the working directory Run should be called with.
func newTwoFeatureFixture(t *testing.T) string {
	t.Helper()

	wd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications", "alpha"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications", "beta"), 0o755))

	return wd
}

// writeUnknownFeatureInput writes a throwaway handoff and state body:
// finish reads both before it opens the feature directory.
func writeUnknownFeatureInput(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()
	handoffPath := filepath.Join(dir, "handoff.md")
	statePath := filepath.Join(dir, "state.md")
	require.NoError(t, os.WriteFile(handoffPath, []byte("x\n"), 0o600))
	require.NoError(t, os.WriteFile(statePath, []byte("x\n"), 0o600))

	return handoffPath, statePath
}

// unknownFeatureRow is one row shared by every unknown-feature test:
// newArgs builds the failing argv, command names the stderr prefix,
// wantFeature is the quoted argument, wantTail the write-command suffix.
type unknownFeatureRow struct {
	name        string
	newArgs     func(t *testing.T) []string
	command     string
	wantFeature string
	wantTail    string
}

// unknownFeatureRows lists one representative not-found argv per command
// that takes a feature argument.
func unknownFeatureRows() []unknownFeatureRow {
	return []unknownFeatureRow{
		{
			name:        "start",
			command:     "start",
			wantFeature: "ghost",
			newArgs:     func(*testing.T) []string { return []string{"start", "ghost"} },
		},
		{
			name:        "check",
			command:     "check",
			wantFeature: "ghost",
			newArgs:     func(*testing.T) []string { return []string{"check", "ghost"} },
		},
		{
			name:        "new step",
			command:     "new step",
			wantFeature: "ghost",
			wantTail:    noFilesChangedSuffix,
			newArgs:     func(*testing.T) []string { return []string{"new", "step", "ghost"} },
		},
		{
			name:        "finish",
			command:     "finish",
			wantFeature: "ghost",
			wantTail:    noFilesChangedSuffix,
			newArgs: func(t *testing.T) []string {
				t.Helper()

				handoffPath, statePath := writeUnknownFeatureInput(t)

				return []string{"finish", "ghost", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}
			},
		},
	}
}

// runUnknownFeatureRow runs row against wd and asserts the not-found line:
// empty stdout, exit 1, and the exact stderr line.
func runUnknownFeatureRow(t *testing.T, wd string, row unknownFeatureRow, featureDir, fix string) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, row.newArgs(t), nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	want := "brief " + row.command + `: no feature "` + row.wantFeature + `" in ` + featureDir + "; " + fix + row.wantTail
	assert.Equal(t, want, oneLine(t, &stderr))
}

func Test_an_unknown_feature_names_the_known_ones(t *testing.T) {
	for _, row := range unknownFeatureRows() {
		t.Run(row.name, func(t *testing.T) {
			wd := newTwoFeatureFixture(t)

			runUnknownFeatureRow(t, wd, row, filepath.Join("docs", "specifications"), "known: alpha, beta")
		})
	}
}

func Test_a_feature_argument_with_a_path_separator_gets_the_same_not_found_copy(t *testing.T) {
	wd := newTwoFeatureFixture(t)
	row := unknownFeatureRow{
		command:     "check",
		wantFeature: "a/b",
		newArgs:     func(*testing.T) []string { return []string{"check", "a/b"} },
	}

	runUnknownFeatureRow(t, wd, row, filepath.Join("docs", "specifications"), "known: alpha, beta")
}

// emptyFeatureRows covers start, new step and finish with an empty
// feature argument. check is excluded: an empty feature there means
// "check every feature", not a feature literally named "".
func emptyFeatureRows() []unknownFeatureRow {
	return []unknownFeatureRow{
		{
			name:        "start",
			command:     "start",
			wantFeature: "",
			newArgs:     func(*testing.T) []string { return []string{"start", ""} },
		},
		{
			name:        "new step",
			command:     "new step",
			wantFeature: "",
			wantTail:    noFilesChangedSuffix,
			newArgs:     func(*testing.T) []string { return []string{"new", "step", ""} },
		},
		{
			name:        "finish",
			command:     "finish",
			wantFeature: "",
			wantTail:    noFilesChangedSuffix,
			newArgs: func(t *testing.T) []string {
				t.Helper()

				handoffPath, statePath := writeUnknownFeatureInput(t)

				return []string{"finish", "", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}
			},
		},
	}
}

func Test_an_empty_feature_argument_gets_the_same_not_found_copy(t *testing.T) {
	for _, row := range emptyFeatureRows() {
		t.Run(row.name, func(t *testing.T) {
			wd := newTwoFeatureFixture(t)

			runUnknownFeatureRow(t, wd, row, filepath.Join("docs", "specifications"), "known: alpha, beta")
		})
	}
}

func Test_an_unknown_feature_with_no_feature_directory_suggests_creating_one(t *testing.T) {
	for _, row := range unknownFeatureRows() {
		t.Run(row.name, func(t *testing.T) {
			wd := t.TempDir()

			runUnknownFeatureRow(t, wd, row, filepath.Join("docs", "specifications"), "known: none; run 'brief new feature ghost' to create it")
		})
	}
}

// Companion case: the feature directory exists but holds no feature.
func Test_an_unknown_feature_with_an_empty_feature_directory_suggests_creating_one(t *testing.T) {
	for _, row := range unknownFeatureRows() {
		t.Run(row.name, func(t *testing.T) {
			wd := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications"), 0o755))

			runUnknownFeatureRow(t, wd, row, filepath.Join("docs", "specifications"), "known: none; run 'brief new feature ghost' to create it")
		})
	}
}

func Test_an_unknown_feature_line_is_relative_to_the_working_directory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".brief.yaml"), []byte("feature-directory: specs\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "specs", "alpha"), 0o755))
	wd := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(wd, 0o755))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "ghost"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	wantDir, relErr := filepath.Rel(wd, filepath.Join(root, "specs"))
	require.NoError(t, relErr)
	assert.Equal(t, `brief start: no feature "ghost" in `+wantDir+`; known: alpha`, oneLine(t, &stderr))
}

// Control arm for the "known:" list: when the feature directory cannot be
// listed at all, the refusal must surface that failure, not "known: none".
func Test_an_unlistable_feature_directory_is_a_failure_not_a_not_found(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wd, "docs", "specifications"), []byte("not a directory"), 0o600))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "ghost"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.NotContains(t, stderr.String(), "known:")
}
