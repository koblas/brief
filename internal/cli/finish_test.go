package cli_test

import (
	"bytes"
	"fmt"
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
		"- [x] do the thing\n"
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

func Test_finishes_the_step_when_the_flags_precede_the_feature_and_step(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "--handoff", handoffPath, "--state", statePath, "demo", "SCENARIO-01"}, nil, &stdout, &stderr)

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

// Test_finish_leaves_a_legacy_handoff_section_in_the_step_file_untouched
// is the migration-tolerance contract for brief's own tree and for any
// adopter's: a step file that still carries a "## Handoff" section with
// prose under it — left behind by a tree not yet migrated — finishes
// successfully and that section survives byte for byte. Finish never
// reads or refuses it; check (SCENARIO-22, unbuilt) is what will report it
// as a finding.
func Test_finish_leaves_a_legacy_handoff_section_in_the_step_file_untouched(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Implementation Plan\n\n" +
		"- [x] do the thing\n\n" +
		"## Handoff\n\n" +
		"LEGACY HANDOFF PROSE, PLEASE KEEP\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte(step), 0o600))

	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(state), 0o600))

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))

	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", state)
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr)
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(featureDir, "SCENARIO-01.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(got), "## Handoff\n\nLEGACY HANDOFF PROSE, PLEASE KEEP\n")

	handoffGot, readErr := os.ReadFile(filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md"))
	require.NoError(t, readErr)
	assert.Equal(t, "NEW-HANDOFF\n", string(handoffGot))
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
	got, readErr := os.ReadFile(filepath.Join(wd, "docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(got), "HANDOFF-FROM-STDIN")
}

// Test_writes_the_handoff_file_beside_the_step_file pins the default
// profile's handoff-file-suffix end to end through the CLI, which no
// scaffold test pins: "SCENARIO-01-HANDOFF.md" holds the supplied body
// verbatim.
func Test_writes_the_handoff_file_beside_the_step_file(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr)
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(wd, "docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md"))
	require.NoError(t, readErr)
	assert.Equal(t, "NEW-HANDOFF\n", string(got))
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

// Test_refuses_a_re_finish_whose_handoff_differs_from_the_recorded_one is
// the CLI slice for SCENARIO-16: re-finishing a done step with a handoff
// that differs from the one recorded on disk is refused rather than
// silently discarding the new handoff or overwriting the record. The
// whole stderr string is asserted, not merely Contains, because today's
// code also prints "brief finish: SCENARIO-01 is done" on exactly these
// inputs — a Contains assertion here would still pass with the refusal
// deleted.
func Test_refuses_a_re_finish_whose_handoff_differs_from_the_recorded_one(t *testing.T) {
	wd := newFinishCLIFixture(t)
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n")
	argv := []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}
	var firstStdout, firstStderr bytes.Buffer

	firstErr := cli.Run(t.Context(), wd, argv, nil, &firstStdout, &firstStderr)
	require.NoError(t, firstErr)

	names := []string{"SCENARIO-01.md", "STATE.md", "specification.md", "SCENARIO-01-HANDOFF.md"}
	before := make(map[string][]byte, len(names))
	for _, name := range names {
		data, readErr := os.ReadFile(filepath.Join(featureDir, name))
		require.NoError(t, readErr)
		before[name] = data
	}

	differentHandoffPath := writeInput(t, "different-handoff.md", "DIFFERENT-HANDOFF\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", differentHandoffPath, "--state", statePath}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	handoffFile := filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md")
	want := fmt.Sprintf(
		"brief finish: %s: step \"SCENARIO-01\" is already done and the given handoff differs "+
			"from the one recorded here; diff the handoff you passed against it, then edit this "+
			"file directly if the new handoff is correct (no files changed)\n",
		handoffFile)
	assert.Equal(t, want, stderr.String())

	for _, name := range names {
		data, readErr := os.ReadFile(filepath.Join(featureDir, name))
		require.NoError(t, readErr)
		assert.Equal(t, before[name], data, "%s must be byte-identical after a refused re-finish", name)
	}
}

// overCapBody returns a handoff/state body of exactly n lines, each
// distinct so a truncation bug cannot hide behind a repeated line, with a
// trailing newline.
func overCapBody(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "line"
	}

	return strings.Join(lines, "\n") + "\n"
}

// Test_refuses_a_handoff_over_the_cap_and_names_the_handoff_path is the CLI
// slice for SCENARIO-17: config.Default's handoff-cap-lines is 60, so a
// 61-line handoff file is refused, naming the --handoff path rather than
// the scaffold.HandoffSource placeholder. Equal, not Contains, on the whole
// stderr line, since a passing run also succeeds silently on unrelated
// inputs — the byte-identity and mtime proof for "nothing lands" lives at
// the scaffold level (finish_cap_test.go); this test does not restate it
// with a weaker probe.
func Test_refuses_a_handoff_over_the_cap_and_names_the_handoff_path(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", overCapBody(61))
	statePath := writeInput(t, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	want := fmt.Sprintf(
		"brief finish: %s: handoff is 61 lines, over the cap of 60; cut the handoff to 60 lines or "+
			"fewer, or raise handoff-cap-lines in .brief.yaml, and retry (no files changed)\n",
		handoffPath)
	assert.Equal(t, want, stderr.String())
}

// Test_names_stdin_when_the_piped_handoff_is_over_the_cap is the stdin half
// of the handoff locator upgrade: --handoff - has no path at all to fall
// back to, so the refusal must name "<stdin>" rather than the raw
// scaffold.HandoffSource placeholder or an empty string.
func Test_names_stdin_when_the_piped_handoff_is_over_the_cap(t *testing.T) {
	wd := newFinishCLIFixture(t)
	statePath := writeInput(t, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n")
	stdin := strings.NewReader(overCapBody(61))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", "-", "--state", statePath}, stdin, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	want := "brief finish: <stdin>: handoff is 61 lines, over the cap of 60; cut the handoff to 60 lines or " +
		"fewer, or raise handoff-cap-lines in .brief.yaml, and retry (no files changed)\n"
	assert.Equal(t, want, stderr.String())
}

// Test_refuses_a_state_body_over_the_cap_and_names_the_state_path is the
// CLI slice for SCENARIO-18: config.Default's state-cap-lines is 80, so an
// 81-line state file is refused, naming the --state path rather than the
// scaffold.StateSource placeholder.
func Test_refuses_a_state_body_over_the_cap_and_names_the_state_path(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", overCapBody(81))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	want := fmt.Sprintf(
		"brief finish: %s: state is 81 lines, over the cap of 80; cut the state to 80 lines or "+
			"fewer, or raise state-cap-lines in .brief.yaml, and retry (no files changed)\n",
		statePath)
	assert.Equal(t, want, stderr.String())
}

// Test_refuses_a_state_body_missing_a_heading_and_names_the_state_path is
// the CLI slice for SCENARIO-19: a state body missing one of
// config.Default's four required headings is refused, naming the --state
// path rather than the scaffold.StateSource placeholder, with the "(no
// files changed)" tail present.
func Test_refuses_a_state_body_missing_a_heading_and_names_the_state_path(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", "## Binding decisions\n\n## Left unbuilt\n\n## Open debts\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	want := fmt.Sprintf(
		`brief finish: %s: state is missing the "## Traps" section; add a "## Traps" heading to the state body — an empty section is valid — and retry (no files changed)`+"\n",
		statePath)
	assert.Equal(t, want, stderr.String())
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

// Test_names_stdin_when_the_piped_state_s_fence_is_unterminated is the
// stdin half of the state locator upgrade: --state - has no path at all
// to fall back to, so the refusal must name "<stdin>" rather than an
// empty string. The handoff argument carries no such check — it is never
// fence-checked, since nothing reads it structurally — so sourceLocator's
// <stdin> branch is exercised here rather than through --handoff.
func Test_names_stdin_when_the_piped_state_s_fence_is_unterminated(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	stdin := strings.NewReader("Repro:\n\n```bash\ngo test ./...\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", "-"}, stdin, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	line := oneLine(t, &stderr)
	assert.Contains(t, line, "<stdin>:3")
}

// Test_names_the_state_path_when_its_fence_is_unterminated is the state
// half of the same locator upgrade.
func Test_names_the_state_path_when_its_fence_is_unterminated(t *testing.T) {
	wd := newFinishCLIFixture(t)
	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", "## Binding decisions\n\n```\nunterminated\n")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	line := oneLine(t, &stderr)
	assert.Contains(t, line, statePath+":3")
}

func Test_prints_the_finish_usage_for_help(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Contains(t, stdout.String(), "Closes step in feature: writes the body at --handoff to the step's own")
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

// Test_preserves_a_CRLF_step_body_when_marking_it_done repoints the
// reviewer's original CRLF fixture at the one remaining in-file step-file
// write: SetStatus's frontmatter edit must not disturb a CRLF body. The
// fixture's original discriminator — the CR-tolerant handoff-anchor scan —
// no longer exists, since the handoff moved out of the step file; this is
// the CRLF claim worth keeping from that finding.
func Test_preserves_a_CRLF_step_body_when_marking_it_done(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\r\n\r\n" +
		"## Implementation Plan\r\n\r\n" +
		"- [x] do the thing\r\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte(step), 0o600))

	state := "## Binding decisions\n\nsome decision\n\n## Left unbuilt\n\nsomething left\n\n## Traps\n\na trap\n\n## Open debts\n\na debt\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(state), 0o600))

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))

	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", state)
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief finish: SCENARIO-01 is done\n", stderr.String())

	got, readErr := os.ReadFile(filepath.Join(featureDir, "SCENARIO-01.md"))
	require.NoError(t, readErr)
	want := strings.Replace(step, "status: open\n", "status: done\n", 1)
	assert.Equal(t, want, string(got))
}

// Test_finish_refuses_a_step_with_an_open_checklist_item is the CLI slice
// for SCENARIO-20: a step whose "## Implementation Plan" checklist still
// carries an unticked item is refused, naming the step file and the
// item's line, with the "(no files changed)" tail present. No cli code
// change backs this: scaffold.RefusalError.Path is already the real step
// file path, never a placeholder cli must swap in.
func Test_finish_refuses_a_step_with_an_open_checklist_item(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	stepPath := filepath.Join(featureDir, "SCENARIO-01.md")
	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Scenario\n\n" +
		"the acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		"- [ ] do the thing\n"
	require.NoError(t, os.WriteFile(stepPath, []byte(step), 0o600))

	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(state), 0o600))

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))

	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", state)
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	want := fmt.Sprintf(
		`brief finish: %s:15: checklist item "do the thing" is not ticked; tick it with [x] once it is done, or remove it, and retry (no files changed)`+"\n",
		stepPath)
	assert.Equal(t, want, stderr.String())
}

// Test_finish_refuses_a_step_whose_dependency_is_unfinished is the CLI
// slice for SCENARIO-21: SCENARIO-02 declares depends-on: [SCENARIO-01],
// and SCENARIO-01 is open, so the refusal names SCENARIO-02's own step
// file, with the "(no files changed)" tail, exit 1, empty stdout, and the
// tree left byte-identical. No cli code change backs this:
// scaffold.RefusalError.Path is already the real step file path, never a
// placeholder cli must swap in.
func Test_finish_refuses_a_step_whose_dependency_is_unfinished(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step01Path := filepath.Join(featureDir, "SCENARIO-01.md")
	step01 := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Scenario\n\n" +
		"the acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		"- [ ] do the thing\n"
	require.NoError(t, os.WriteFile(step01Path, []byte(step01), 0o600))

	step02Path := filepath.Join(featureDir, "SCENARIO-02.md")
	step02 := "---\n" +
		"id: SCENARIO-02\n" +
		"status: open\n" +
		"depends-on: [SCENARIO-01]\n" +
		"---\n\n" +
		"# SCENARIO-02 Demo step\n\n" +
		"## Scenario\n\n" +
		"the acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		"- [x] do the thing\n"
	require.NoError(t, os.WriteFile(step02Path, []byte(step02), 0o600))

	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(state), 0o600))

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n- [ ] SCENARIO-02\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))

	names := []string{"SCENARIO-01.md", "SCENARIO-02.md", "STATE.md", "specification.md"}
	before := make(map[string][]byte, len(names))
	for _, name := range names {
		data, readErr := os.ReadFile(filepath.Join(featureDir, name))
		require.NoError(t, readErr)
		before[name] = data
	}

	handoffPath := writeInput(t, "handoff.md", "NEW-HANDOFF\n")
	statePath := writeInput(t, "state.md", state)
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"finish", "demo", "SCENARIO-02", "--handoff", handoffPath, "--state", statePath}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	want := fmt.Sprintf(
		`brief finish: %s: step "SCENARIO-02" depends on "SCENARIO-01", which is not finished; finish SCENARIO-01 first, or remove it from this step's depends-on, and retry (no files changed)`+"\n",
		step02Path)
	assert.Equal(t, want, stderr.String())

	for _, name := range names {
		data, readErr := os.ReadFile(filepath.Join(featureDir, name))
		require.NoError(t, readErr)
		assert.Equal(t, before[name], data, "%s must be byte-identical after a refused finish", name)
	}
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
