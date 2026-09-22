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

// overCapStateLines is one line past config.Default's state-cap-lines (80),
// the smallest state body overCapState needs to trip the state-cap rule.
const overCapStateLines = 81

// overCapState returns a state body of exactly n lines, carrying the
// default profile's four required headings, padded with filler past
// config.Default's state-cap-lines so conform.OverCap's rule fires —
// mirrors internal/assemble's own checkStateOfLines test helper.
func overCapState(n int) string {
	headings := []string{"## Binding decisions", "## Left unbuilt", "## Traps", "## Open debts"}

	var lines []string
	for _, h := range headings {
		lines = append(lines, h, "", "content")
	}

	for i := 0; len(lines) < n; i++ {
		lines = append(lines, fmt.Sprintf("filler line %d", i))
	}

	return strings.Join(lines[:n], "\n") + "\n"
}

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
	assert.Equal(t, ""+
		"demo  (in flight)\n"+
		"  ERROR  "+filepath.Join("docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md")+":61  handoff is 61 lines, over the cap of 60\n",
		stdout.String())
	assert.Equal(t, "brief check: 1 ERROR, 0 WARN in 1 feature (1 handoff-cap); ERRORs block finish on in-flight features\n", stderr.String())
}

// Test_check_groups_two_features_with_a_blank_line_and_counts_rules_by_count_then_id
// pins three things at once: two feature groups separated by exactly one
// blank line, an unscoped run with more than one feature carrying the
// narrowing clause, and the rule tally's own order — "state-cap" (count 2)
// ahead of "checklist" and "handoff-cap" (count 1 apiece) despite sorting
// after both alphabetically, and the tied pair itself broken by rule id
// ascending ("checklist" before "handoff-cap").
func Test_check_groups_two_features_with_a_blank_line_and_counts_rules_by_count_then_id(t *testing.T) {
	wd := t.TempDir()

	alphaDir := filepath.Join(wd, "docs", "specifications", "alpha")
	require.NoError(t, os.MkdirAll(alphaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(alphaDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(alphaDir, "STATE.md"), []byte(overCapState(overCapStateLines)), 0o600))
	writeCheckStep(t, alphaDir, "SCENARIO-01", "done", []string{"- [x] first thing", "- [ ] second thing"})

	betaDir := filepath.Join(wd, "docs", "specifications", "beta")
	require.NoError(t, os.MkdirAll(betaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(betaDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(betaDir, "STATE.md"), []byte(overCapState(overCapStateLines)), 0o600))
	writeCheckStep(t, betaDir, "SCENARIO-01", "done", []string{"- [x] first thing"})
	require.NoError(t, os.WriteFile(filepath.Join(betaDir, "SCENARIO-01-HANDOFF.md"), []byte(checkBodyOfLines(61)), 0o600))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Equal(t, ""+
		"alpha  (complete)\n"+
		"  WARN  "+filepath.Join("docs", "specifications", "alpha", "STATE.md")+":81  state is 81 lines, over the cap of 80\n"+
		"  WARN  "+filepath.Join("docs", "specifications", "alpha", "SCENARIO-01.md")+":16  checklist item \"second thing\" is not ticked\n"+
		"\n"+
		"beta  (complete)\n"+
		"  WARN  "+filepath.Join("docs", "specifications", "beta", "STATE.md")+":81  state is 81 lines, over the cap of 80\n"+
		"  WARN  "+filepath.Join("docs", "specifications", "beta", "SCENARIO-01-HANDOFF.md")+":61  handoff is 61 lines, over the cap of 60\n",
		stdout.String())
	assert.Equal(t,
		"brief check: 0 ERROR, 4 WARN in 2 features (2 state-cap, 1 checklist, 1 handoff-cap); 'brief check <feature>' narrows to one\n",
		stderr.String())
}

// Test_check_omits_the_line_suffix_for_a_whole_file_finding pins the
// whole-file shape: a finding with Line 0 (here, a missing state file)
// prints the bare path, never a trailing ":0". The fixture has no step
// files at all, so it is in flight (Total == 0, the same rule
// assemble.Status's Complete() applies) and the finding is ERROR, exit 1
// — not the WARN/exit-0 a naive "no step is un-done" reading would give a
// zero-step feature.
func Test_check_omits_the_line_suffix_for_a_whole_file_finding(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check"}, nil, &stdout, &stderr)

	require.Error(t, err)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Contains(t, stdout.String(), "\n  ERROR  "+filepath.Join("docs", "specifications", "demo", "STATE.md")+"  ")
	assert.NotContains(t, stdout.String(), "STATE.md:0")
}

// Test_check_summary_drops_the_ERRORs_clause_when_every_finding_is_WARN
// pins R18's exit rule directly: a fully-done feature carrying an over-cap
// handoff prints its WARN finding on stdout but still exits 0 — only ERROR
// fails the run — and its stderr summary carries neither tail clause: no
// ERROR to block finish on, and only one feature so no narrowing to offer.
func Test_check_summary_drops_the_ERRORs_clause_when_every_finding_is_WARN(t *testing.T) {
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
	assert.Equal(t, ""+
		"demo  (complete)\n"+
		"  WARN  "+filepath.Join("docs", "specifications", "demo", "SCENARIO-01-HANDOFF.md")+":61  handoff is 61 lines, over the cap of 60\n",
		stdout.String())
	assert.Equal(t, "brief check: 0 ERROR, 1 WARN in 1 feature (1 handoff-cap)\n", stderr.String())
}

// Test_check_summary_drops_the_narrowing_clause_for_a_named_feature pins
// the other independent condition: naming a feature drops the narrowing
// clause even when the run carries an ERROR finding that would otherwise
// license the ERRORs-block-finish clause alone.
func Test_check_summary_drops_the_narrowing_clause_for_a_named_feature(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "alpha")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(conformingState), 0o600))
	writeCheckStep(t, featureDir, "SCENARIO-01", "open", []string{"- [ ] do the thing"})
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md"), []byte(checkBodyOfLines(61)), 0o600))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check", "alpha"}, nil, &stdout, &stderr)

	require.Error(t, err)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Equal(t, "brief check: 1 ERROR, 0 WARN in 1 feature (1 handoff-cap); ERRORs block finish on in-flight features\n", stderr.String())
}

// Test_check_labels_a_symlinked_feature_directory_under_its_own_name pins
// the feature-level producer's own group: a symlink where a feature
// directory is expected groups under its own link name, always
// "(in flight)" — InFlight is hard-coded true for a feature-level finding
// regardless of what the link's target would have measured.
func Test_check_labels_a_symlinked_feature_directory_under_its_own_name(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications"), 0o755))

	realDir := filepath.Join(wd, "real-gamma")
	require.NoError(t, os.MkdirAll(realDir, 0o755))
	require.NoError(t, os.Symlink(realDir, filepath.Join(wd, "docs", "specifications", "gamma")))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check"}, nil, &stdout, &stderr)

	require.Error(t, err)
	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Equal(t, ""+
		"gamma  (in flight)\n"+
		"  ERROR  "+filepath.Join("docs", "specifications", "gamma")+"  is a symbolic link, not read as a feature directory\n",
		stdout.String())
	assert.Equal(t, "brief check: 1 ERROR, 0 WARN in 1 feature (1 feature-symlink); ERRORs block finish on in-flight features\n", stderr.String())
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

// Test_check_names_the_feature_in_the_no_findings_notice pins the cheap
// optional: a named, conforming feature's "no findings" notice names it
// ("brief check: <feature>: no findings"), unlike the bare "brief check:
// no findings" a run with no feature argument writes
// (Test_check_json_document_golden's control arm, cli/check_json_test.go).
func Test_check_names_the_feature_in_the_no_findings_notice(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "alpha")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(conformingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(conformingState), 0o600))
	writeCheckStep(t, featureDir, "SCENARIO-01", "done", []string{"- [x] do the thing"})

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"check", "alpha"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief check: alpha: no findings\n", stderr.String())
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
