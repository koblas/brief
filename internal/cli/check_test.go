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

// checkBodyOfLines returns a body of exactly n distinct lines, with a
// trailing newline.
func checkBodyOfLines(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}

	return strings.Join(lines, "\n") + "\n"
}

// conformingState is a state body carrying the default profile's four
// required headings, each with content.
const conformingState = "## Binding decisions\n\nsome decision\n\n" +
	"## Left unbuilt\n\nsomething left\n\n" +
	"## Traps\n\na trap\n\n" +
	"## Open debts\n\na debt\n"

// conformingSpec is a specification body carrying the default profile's
// progress heading.
const conformingSpec = "# demo\n\n## BDD Acceptance Progress\n\n- [x] SCENARIO-01\n"

// writeCheckStep writes one step file under featureDir, using the default
// step-file-pattern's naming.
func writeCheckStep(t *testing.T, featureDir, id, status string, checklistItems []string) {
	t.Helper()

	var checklist strings.Builder
	for _, item := range checklistItems {
		checklist.WriteString(item + "\n")
	}

	step := "---\n" +
		"id: " + id + "\n" +
		"status: " + status + "\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# " + id + "\n\n" +
		"## Scenario\n\nthe acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		checklist.String()
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, id+".md"), []byte(step), 0o600))
}

func Test_check_prints_findings_on_stdout_and_exits_1_for_an_error_finding(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(conformingState), 0o600))
	writeCheckStep(t, featureDir, "SCENARIO-01", "open", []string{"- [ ] do the thing"})

	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md"), []byte(checkBodyOfLines(61)), 0o600))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check"}, nil, &stdout, &stderr)

	require.Error(t, err)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Equal(t,
		"[ERROR] "+filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md")+":61 — handoff is 61 lines, over the cap of 60\n",
		stdout.String())
	assert.NotEmpty(t, stderr.String())
}

// Test_check_reports_no_findings_and_exits_0_for_a_conforming_repository is
// the R14/SCENARIO-10 shape: empty stdout, one stderr line, exit 0.
func Test_check_reports_no_findings_and_exits_0_for_a_conforming_repository(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(conformingState), 0o600))
	writeCheckStep(t, featureDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, 1, strings.Count(stderr.String(), "\n"))
}

// Test_check_says_no_findings_and_exits_0_for_a_repository_with_no_features
// pins the other half of the same shape: no docs/specifications at all.
func Test_check_says_no_findings_and_exits_0_for_a_repository_with_no_features(t *testing.T) {
	wd := t.TempDir()

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, 1, strings.Count(stderr.String(), "\n"))
}

// Test_check_exits_0_when_every_finding_is_WARN_severity pins R18's exit
// rule directly: a fully-done feature carrying an over-cap handoff prints
// its WARN finding on stdout but still exits 0 — only ERROR fails the run.
func Test_check_exits_0_when_every_finding_is_WARN_severity(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(conformingState), 0o600))
	writeCheckStep(t, featureDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md"), []byte(checkBodyOfLines(61)), 0o600))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Contains(t, stdout.String(), "[WARN]")
}

// Test_check_refuses_an_unknown_feature_with_no_files_changed_tail pins the
// read-refusal shape: one stderr line, no "(no files changed)" tail — Check
// never writes, so there is nothing that promise could be about.
func Test_check_refuses_an_unknown_feature_with_no_files_changed_tail(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications"), 0o755))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check", "ghost"}, nil, &stdout, &stderr)

	require.Error(t, err)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, 1, strings.Count(stderr.String(), "\n"))
	assert.NotContains(t, stderr.String(), "(no files changed)")
}

// Test_check_scopes_to_the_named_feature_only asserts a malformed sibling
// feature contributes no findings when one feature is named.
func Test_check_scopes_to_the_named_feature_only(t *testing.T) {
	wd := t.TempDir()

	goodDir := filepath.Join(wd, "docs", "specifications", "alpha")
	require.NoError(t, os.MkdirAll(goodDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(goodDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(goodDir, "STATE.md"), []byte(conformingState), 0o600))
	writeCheckStep(t, goodDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})

	badDir := filepath.Join(wd, "docs", "specifications", "beta")
	require.NoError(t, os.MkdirAll(badDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(badDir, "specification.md"), []byte("# beta\n\nno progress list\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(badDir, "STATE.md"), []byte(conformingState), 0o600))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check", "alpha"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
}

func Test_returns_a_usage_error_when_check_is_given_two_arguments(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check", "alpha", "beta"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, 1, strings.Count(stderr.String(), "\n"))
}

func Test_returns_a_usage_error_when_check_is_given_an_undefined_flag(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check", "--bogus"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
}

func Test_prints_usage_to_stdout_when_help_is_requested_for_check(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Contains(t, stdout.String(), "brief check")
}
