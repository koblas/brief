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

// newFinishCLIFixture writes one open step, "SCENARIO-01", for feature
// "demo" under the default profile's layout, with a fully ticked checklist
// and a bare handoff anchor, and returns the working directory Run should
// be called with.
func newFinishCLIFixture(t *testing.T) string {
	t.Helper()

	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Scenario\n\n" +
		"the acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		"- [x] do the thing\n\n" +
		"## Handoff\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte(step), 0o600))

	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(state), 0o600))

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))

	return wd
}

// writeInput writes contents to a fresh file under t.TempDir() and returns
// its path.
func writeInput(t *testing.T, name, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	return path
}

func Test_finishes_the_step_and_prints_nothing_to_stdout(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief finish: SCENARIO-01 is done\n", stderr.String())
}

// Test_finishing_an_already_finished_step_a_second_time_prints_the_same_line_and_succeeds
// pins the user-visible contract of a no-op re-finish: exit 0, nothing on
// stdout (reserved for R9 findings), the same state-describing stderr
// line printed by a writing run. That indistinguishability is deliberate —
// before the scaffold package's identity check existed, an overwriting
// second finish also returned nil and printed this exact line, so this
// test cannot redden under any of the identity check's conjuncts; it is
// not the test that proves the no-op, only the one that proves the
// no-op is invisible at the command surface.
func Test_finishing_an_already_finished_step_a_second_time_prints_the_same_line_and_succeeds(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")
	argv := []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}
	var firstStdout, firstStderr bytes.Buffer

	firstErr := cli.Run(t.Context(), wd, argv, nil, &firstStdout, &firstStderr)
	require.NoError(t, firstErr)

	var secondStdout, secondStderr bytes.Buffer
	secondErr := cli.Run(t.Context(), wd, argv, nil, &secondStdout, &secondStderr)

	require.NoError(t, secondErr)
	assert.Empty(t, secondStdout.String())
	assert.Equal(t, "brief finish: SCENARIO-01 is done\n", secondStderr.String())
}

func Test_replaces_the_state_file_on_disk_when_finishing(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	newState := "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n"
	statePath := writeInput(t, "state.md", newState)
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr)
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(wd, "docs", "specifications", "demo", "STATE.md"))
	require.NoError(t, readErr)
	assert.Equal(t, newState, string(got))
}

func Test_reads_the_handoff_body_from_stdin_when_the_path_is_a_dash(t *testing.T) {
	wd := newFinishCLIFixture(t)
	statePath := writeInput(t, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")
	stdin := strings.NewReader("HANDOFF-FROM-STDIN\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", "-", "--state", statePath}, stdin, &stdout, &stderr)

	require.NoError(t, err)
	got, readErr := os.ReadFile(filepath.Join(wd, "docs", "specifications", "demo", "SCENARIO-01.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(got), "HANDOFF-FROM-STDIN")
}

func Test_reads_the_state_body_from_stdin_when_the_path_is_a_dash(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	newState := "## Binding decisions\n\nSTATE-FROM-STDIN\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n"
	stdin := strings.NewReader(newState)
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", "-"}, stdin, &stdout, &stderr)

	require.NoError(t, err)
	got, readErr := os.ReadFile(filepath.Join(wd, "docs", "specifications", "demo", "STATE.md"))
	require.NoError(t, readErr)
	assert.Equal(t, newState, string(got))
}

func Test_returns_a_usage_error_when_no_feature_is_given_to_finish(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief finish: no feature given; run 'brief finish <feature> <step> --handoff <path> --state <path>'", oneLine(t, &stderr))
}

func Test_returns_a_usage_error_when_no_step_is_given_to_finish(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief finish: no step given; run 'brief finish <feature> <step> --handoff <path> --state <path>'", oneLine(t, &stderr))
}

func Test_returns_a_usage_error_when_finish_is_given_too_many_arguments(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "extra"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief finish: too many arguments; run 'brief finish <feature> <step> --handoff <path> --state <path>'", oneLine(t, &stderr))
}

func Test_returns_a_usage_error_when_handoff_is_omitted(t *testing.T) {
	wd := t.TempDir()
	statePath := writeInput(t, "state.md", "")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--state", statePath}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief finish: --handoff is required; run 'brief finish <feature> <step> --handoff <path> --state <path>'", oneLine(t, &stderr))
}

func Test_returns_a_usage_error_when_state_is_omitted(t *testing.T) {
	wd := t.TempDir()
	handoffPath := writeInput(t, "handoff.md", "")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief finish: --state is required; run 'brief finish <feature> <step> --handoff <path> --state <path>'", oneLine(t, &stderr))
}

func Test_returns_a_usage_error_when_dash_is_given_for_both_handoff_and_state(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", "-", "--state", "-"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief finish: - may be given for at most one of --handoff and --state", oneLine(t, &stderr))
}

func Test_returns_a_usage_error_for_an_unknown_finish_flag(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--bogus"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief finish: flag provided but not defined: -bogus; run 'brief finish <feature> <step> --handoff <path> --state <path>'", oneLine(t, &stderr))
}

func Test_prints_the_finish_usage_for_help(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Contains(t, stdout.String(), "COMPLETE")
}

func Test_returns_an_error_when_the_handoff_path_is_unreadable(t *testing.T) {
	wd := newFinishCLIFixture(t)
	statePath := writeInput(t, "state.md", "")
	missing := filepath.Join(t.TempDir(), "does-not-exist.md")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", missing, "--state", statePath}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	line := oneLine(t, &stderr)
	assert.Contains(t, line, missing)
	assert.False(t, strings.HasSuffix(line, "(no files changed)"), "line %q must not carry the write-refusal tail", line)
}

func Test_returns_an_error_for_an_unknown_feature_on_finish(t *testing.T) {
	wd := t.TempDir()
	handoffPath := writeInput(t, "handoff.md", "h")
	statePath := writeInput(t, "state.md", "s")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "ghost", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	line := oneLine(t, &stderr)
	assert.True(t, strings.HasSuffix(line, "(no files changed)"), "line %q must end with (no files changed)", line)
}
