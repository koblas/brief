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

// newTwoFeatureFixture writes two conforming, empty feature directories,
// "alpha" and "beta" (fs.ReadDir order), under the default profile's
// layout, and returns the working directory Run should be called with.
// Neither feature has any step files: no row here needs one, since every
// case in this file fails before a feature's own contents are ever read.
func newTwoFeatureFixture(t *testing.T) string {
	t.Helper()

	wd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications", "alpha"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications", "beta"), 0o755))

	return wd
}

// writeUnknownFeatureInput writes a throwaway handoff and state body under a
// fresh temp directory, independent of wd: finish reads both before it ever
// opens the feature directory, so a not-found row still needs bodies that
// read without error even though their content is never inspected.
func writeUnknownFeatureInput(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()
	handoffPath := filepath.Join(dir, "handoff.md")
	statePath := filepath.Join(dir, "state.md")
	require.NoError(t, os.WriteFile(handoffPath, []byte("x\n"), 0o600))
	require.NoError(t, os.WriteFile(statePath, []byte("x\n"), 0o600))

	return handoffPath, statePath
}

// unknownFeatureRow is one row shared by every unknown-feature test in this
// file: newArgs builds the failing command's own argv (a func, so finish's
// row can allocate its own handoff/state fixture per test run), command is
// the "brief <command>" prefix the stderr line names, wantFeature is the
// feature argument the line quotes, and wantTail is the "(no files
// changed)" suffix on a write command, "" on a read command.
type unknownFeatureRow struct {
	name        string
	newArgs     func(t *testing.T) []string
	command     string
	wantFeature string
	wantTail    string
}

// unknownFeatureRows is the one row list shared by every "known:" variant
// below: start, check, new step and finish, one representative not-found
// argv apiece.
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

// runUnknownFeatureRow runs row against wd and asserts R14a's not-found
// line: empty stdout, exit 1, and "brief <command>: no feature <feature>"
// in <feature dir>; <fix><tail>" on stderr, byte for byte.
func runUnknownFeatureRow(t *testing.T, wd string, row unknownFeatureRow, featureDir, fix string) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, row.newArgs(t), nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	want := "brief " + row.command + `: no feature "` + row.wantFeature + `" in ` + featureDir + "; " + fix + row.wantTail
	assert.Equal(t, want, oneLine(t, &stderr))
}

// Test_an_unknown_feature_names_the_known_ones pins R14a's not-found copy
// for every command taking a feature argument, against a fixture with two
// known features: the exact stderr line, empty stdout, exit 1, and the "(no
// files changed)" tail only on the write commands (new step, finish).
func Test_an_unknown_feature_names_the_known_ones(t *testing.T) {
	for _, row := range unknownFeatureRows() {
		t.Run(row.name, func(t *testing.T) {
			wd := newTwoFeatureFixture(t)

			runUnknownFeatureRow(t, wd, row, filepath.Join("docs", "specifications"), "known: alpha, beta")
		})
	}
}

// Test_a_feature_argument_with_a_path_separator_gets_the_same_not_found_copy
// pins that "check a/b" — validFeatureArgument's own rejection of a feature
// argument carrying a path separator — renders through the identical
// not-found copy, not a separate message.
func Test_a_feature_argument_with_a_path_separator_gets_the_same_not_found_copy(t *testing.T) {
	wd := newTwoFeatureFixture(t)
	row := unknownFeatureRow{
		command:     "check",
		wantFeature: "a/b",
		newArgs:     func(*testing.T) []string { return []string{"check", "a/b"} },
	}

	runUnknownFeatureRow(t, wd, row, filepath.Join("docs", "specifications"), "known: alpha, beta")
}

// Test_an_unknown_feature_with_no_feature_directory_suggests_creating_one
// pins the empty-list branch of the not-found copy when the configured
// feature directory does not exist at all.
func Test_an_unknown_feature_with_no_feature_directory_suggests_creating_one(t *testing.T) {
	for _, row := range unknownFeatureRows() {
		t.Run(row.name, func(t *testing.T) {
			wd := t.TempDir()

			runUnknownFeatureRow(t, wd, row, filepath.Join("docs", "specifications"), "known: none; run 'brief new feature ghost' to create it")
		})
	}
}

// Test_an_unknown_feature_with_an_empty_feature_directory_suggests_creating_one
// is the companion case: the configured feature directory exists but holds
// no feature of its own — the same empty-list copy as a directory that
// does not exist at all.
func Test_an_unknown_feature_with_an_empty_feature_directory_suggests_creating_one(t *testing.T) {
	for _, row := range unknownFeatureRows() {
		t.Run(row.name, func(t *testing.T) {
			wd := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications"), 0o755))

			runUnknownFeatureRow(t, wd, row, filepath.Join("docs", "specifications"), "known: none; run 'brief new feature ghost' to create it")
		})
	}
}

// Test_an_unknown_feature_line_is_relative_to_the_working_directory pins
// R6: the printed feature directory follows a custom feature-directory
// config value and a working directory below the config root, deriving the
// expected directory from the fixture's own root rather than a literal.
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

// Test_an_unlistable_feature_directory_is_a_failure_not_a_not_found is the
// control arm for the "known:" list itself: when the configured feature
// directory cannot even be listed (here, because it is a regular file
// rather than a directory), the refusal must surface that failure rather
// than claim "known: none" — which the same argv against a real empty
// directory does print, proving the difference is the listing failure and
// nothing else about the argv.
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
