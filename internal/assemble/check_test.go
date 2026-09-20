package assemble_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checkBodyOfLines returns a body of exactly n distinct lines, with a
// trailing newline, mirroring scaffold_test's own helper of the same
// shape.
func checkBodyOfLines(n int) []byte {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}

	return []byte(strings.Join(lines, "\n") + "\n")
}

// checkConformingSpec returns a specification body carrying cfg's
// configured progress heading and nothing else Check's C1 rule requires.
func checkConformingSpec(cfg config.Config) string {
	return "# demo\n\n" + cfg.ProgressHeading + "\n\ncontent\n"
}

// checkConformingState returns a state body carrying cfg's four configured
// headings, each with non-empty content, comfortably under any cap this
// file uses.
func checkConformingState(cfg config.Config) string {
	var b strings.Builder
	for _, h := range cfg.StateHeadings.Ordered() {
		b.WriteString(h + "\n\ncontent\n\n")
	}

	return b.String()
}

// checkStateOfLines returns a state body of exactly n lines carrying cfg's
// four configured headings, padded with filler so a truncation bug cannot
// hide behind a repeated line — mirrors scaffold_test's stateBodyOfLines.
func checkStateOfLines(cfg config.Config, n int) string {
	var lines []string

	for i, h := range cfg.StateHeadings.Ordered() {
		lines = append(lines, h, "", fmt.Sprintf("content %d", i))
	}

	for i := 0; len(lines) < n; i++ {
		lines = append(lines, fmt.Sprintf("filler line %d", i))
	}

	return strings.Join(lines[:n], "\n") + "\n"
}

// checkStateMissingHeading returns a state body carrying every one of
// cfg's four configured headings except missing, with the heading line and
// its section both absent.
func checkStateMissingHeading(cfg config.Config, missing string) string {
	var b strings.Builder
	for _, h := range cfg.StateHeadings.Ordered() {
		if h == missing {
			continue
		}

		b.WriteString(h + "\n\ncontent\n\n")
	}

	return b.String()
}

// checkStateUnterminatedFence returns a state body carrying every one of
// cfg's four configured headings, with an opened fence never closed.
func checkStateUnterminatedFence(cfg config.Config) string {
	var b strings.Builder
	for _, h := range cfg.StateHeadings.Ordered() {
		b.WriteString(h + "\n\ncontent\n\n")
	}

	b.WriteString("```\nunterminated\n")

	return b.String()
}

// checkStepBody renders one step file's body: frontmatter (id, status,
// depends-on) plus a checklist section holding checklistItems verbatim, so
// a test can pin exact line numbers and exact ticked/unticked shapes.
func checkStepBody(cfg config.Config, id, status string, dependsOn, checklistItems []string) string {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString("id: " + id + "\n")
	b.WriteString("status: " + status + "\n")

	if len(dependsOn) == 0 {
		b.WriteString("depends-on: []\n")
	} else {
		b.WriteString("depends-on:\n")
		for _, d := range dependsOn {
			b.WriteString("  - " + d + "\n")
		}
	}

	b.WriteString("---\n\n")
	b.WriteString("# " + id + "\n\n")
	b.WriteString(cfg.ChecklistHeading + "\n\n")

	for _, item := range checklistItems {
		b.WriteString(item + "\n")
	}

	return b.String()
}

// checkFeatureDir returns, and creates, the path of feature under root's
// configured feature directory.
func checkFeatureDir(cfg config.Config, root, feature string) string {
	return filepath.Join(root, cfg.FeatureDirectory, feature)
}

// checkWriteFeature writes spec and state at their configured names under
// featureDir.
func checkWriteFeature(t *testing.T, cfg config.Config, featureDir, spec, state string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))
}

// checkWriteStep writes one step file under featureDir.
func checkWriteStep(t *testing.T, featureDir, id, body string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(featureDir, id+".md"), []byte(body), 0o600))
}

// checkWriteHandoff writes id's handoff file under featureDir, using cfg's
// configured suffix.
func checkWriteHandoff(t *testing.T, cfg config.Config, featureDir, id, body string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(featureDir, id+cfg.HandoffFileSuffix), []byte(body), 0o600))
}

// findingsByPath is a small assertion helper: it requires findings to have
// exactly one entry and returns it, so call sites read as one line rather
// than three.
func onlyFinding(t *testing.T, findings []assemble.Finding) assemble.Finding {
	t.Helper()

	require.Len(t, findings, 1)

	return findings[0]
}

// Test_check_reports_an_over_cap_handoff_file pins C10's own data — the
// finding names the right path, line, measured value and cap (the
// Gherkin's own assertion) — by checking each fact rather than the whole
// sentence verbatim: whether that sentence is textually identical to
// scaffold.Finish's own refusal is check_drift_test.go's claim, not this
// one, and pinning the same exact string here would make this test go red
// on the identical mutation that test exists to catch, hiding which of the
// two claims actually broke.
func Test_check_reports_an_over_cap_handoff_file(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	cfg.HandoffCapLines = 10
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))
	checkWriteHandoff(t, cfg, featureDir, "STEP-01", string(checkBodyOfLines(11)))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(featureDir, "STEP-01"+cfg.HandoffFileSuffix), f.Path)
	assert.Equal(t, 11, f.Line, "a cap finding's line is cap+1, the first line over it")
	assert.Contains(t, f.Problem, "11")
	assert.Contains(t, f.Problem, "10")
	assert.Contains(t, f.Problem, "over the cap")
	assert.Equal(t, assemble.SeverityWarn, f.Severity)
}

// Test_check_reports_nothing_for_a_missing_handoff_file is the control arm
// for C10: a done step with no handoff file at all is not a finding — a
// missing handoff is exactly "this step has never been finished under a
// caps-aware finish", which scaffold.Finish's own re-finish exemption
// (R16) already treats as nothing to diverge from.
func Test_check_reports_nothing_for_a_missing_handoff_file(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func Test_check_reports_a_state_body_over_the_configured_cap(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	cfg.StateCapLines = 20
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkStateOfLines(cfg, 21))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(featureDir, cfg.StateFile), f.Path)
	assert.Equal(t, 21, f.Line, "a cap finding's line is cap+1, the first line over it")
	assert.Equal(t, "state is 21 lines, over the cap of 20", f.Problem)
}

func Test_check_reports_a_state_body_with_an_unterminated_fence(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkStateUnterminatedFence(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(featureDir, cfg.StateFile), f.Path)
	assert.Equal(t, "state has an unclosed ``` fence", f.Problem)
}

func Test_check_reports_a_state_body_missing_a_required_heading(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkStateMissingHeading(cfg, cfg.StateHeadings.Traps))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(featureDir, cfg.StateFile), f.Path)
	assert.Equal(t, `state is missing the "## Gotchas" section`, f.Problem)
}

func Test_check_reports_a_missing_state_file(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(checkConformingSpec(cfg)), 0o600))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(featureDir, cfg.StateFile), f.Path)
	assert.Contains(t, f.Problem, "no such file")
}

func Test_check_reports_a_specification_with_no_progress_heading(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, "# demo\n\nno progress list here\n", checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(featureDir, cfg.SpecificationFile), f.Path)
	assert.Contains(t, f.Problem, cfg.ProgressHeading)
}

// Test_check_reports_a_step_whose_frontmatter_does_not_parse_and_still_reports_its_handoff_cap
// is C6 paired with C10's independence from it: STEP-01's frontmatter is
// garbage, and its handoff file is over cap — both must be reported, proof
// that Check never reuses readSteps's all-or-nothing stance.
func Test_check_reports_a_step_whose_frontmatter_does_not_parse_and_still_reports_its_handoff_cap(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	cfg.HandoffCapLines = 10
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", "not frontmatter at all\n")
	checkWriteHandoff(t, cfg, featureDir, "STEP-01", string(checkBodyOfLines(11)))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)
	require.Len(t, findings, 2)

	assert.Equal(t, filepath.Join(featureDir, "STEP-01.md"), findings[0].Path)
	assert.Contains(t, findings[0].Problem, "frontmatter does not parse")
	assert.Equal(t, filepath.Join(featureDir, "STEP-01"+cfg.HandoffFileSuffix), findings[1].Path)
	assert.Contains(t, findings[1].Problem, "over the cap")

	for _, f := range findings {
		assert.Equal(t, assemble.SeverityError, f.Severity, "an unreadable step's own feature is always in flight")
	}
}

func Test_check_reports_an_unticked_checklist_item_on_a_done_step(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil,
		[]string{"- [x] first thing", "- [ ] second thing"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(featureDir, "STEP-01.md"), f.Path)
	assert.Equal(t, 12, f.Line)
	assert.Equal(t, `checklist item "second thing" is not ticked`, f.Problem)
}

// Test_check_reports_nothing_for_an_unticked_checklist_item_on_an_open_step
// is C7's control arm: the same unticked item on an open step is ordinary
// in-progress work, not a finding.
func Test_check_reports_nothing_for_an_unticked_checklist_item_on_an_open_step(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "open", nil,
		[]string{"- [x] first thing", "- [ ] second thing"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func Test_check_reports_a_dependency_id_that_names_no_step_file(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "open", []string{"STEP-99"},
		[]string{"- [ ] a task"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(featureDir, "STEP-01.md"), f.Path)
	assert.Equal(t, `step "STEP-01" depends on "STEP-99", which names no step file`, f.Problem)
}

// Test_check_reports_nothing_for_an_ordinary_unmet_dependency is C8's
// control arm: a depends-on id that names a real, known, not-yet-done step
// is ordinary in-progress work, already counted by Status's Blocked.
func Test_check_reports_nothing_for_an_ordinary_unmet_dependency(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "open", nil, []string{"- [ ] a task"}))
	checkWriteStep(t, featureDir, "STEP-02", checkStepBody(cfg, "STEP-02", "open", []string{"STEP-01"}, []string{"- [ ] a task"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func Test_check_reports_a_step_that_depends_on_itself(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "open", []string{"STEP-01"}, []string{"- [ ] a task"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(featureDir, "STEP-01.md"), f.Path)
	assert.Equal(t, `step "STEP-01" depends on "STEP-01", which is not finished`, f.Problem)
}

// Test_check_assigns_ERROR_when_a_feature_is_still_in_flight_and_WARN_when_every_step_is_done
// pins severity's one variable: the same defect (an over-cap handoff) on a
// feature with an open sibling step is ERROR; on a feature whose every
// step is done, it is WARN.
func Test_check_assigns_ERROR_when_a_feature_is_still_in_flight_and_WARN_when_every_step_is_done(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	cfg.HandoffCapLines = 10

	inFlight := t.TempDir()
	inFlightDir := checkFeatureDir(cfg, inFlight, "demo")
	checkWriteFeature(t, cfg, inFlightDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, inFlightDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))
	checkWriteHandoff(t, cfg, inFlightDir, "STEP-01", string(checkBodyOfLines(11)))
	checkWriteStep(t, inFlightDir, "STEP-02", checkStepBody(cfg, "STEP-02", "open", nil, []string{"- [ ] a task"}))

	srvInFlight := assemble.NewServer(cfg, inFlight)
	findingsInFlight, err := srvInFlight.Check(t.Context(), "demo")
	require.NoError(t, err)
	f := onlyFinding(t, findingsInFlight)
	assert.Equal(t, assemble.SeverityError, f.Severity)

	done := t.TempDir()
	doneDir := checkFeatureDir(cfg, done, "demo")
	checkWriteFeature(t, cfg, doneDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, doneDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))
	checkWriteHandoff(t, cfg, doneDir, "STEP-01", string(checkBodyOfLines(11)))

	srvDone := assemble.NewServer(cfg, done)
	findingsDone, err := srvDone.Check(t.Context(), "demo")
	require.NoError(t, err)
	fDone := onlyFinding(t, findingsDone)
	assert.Equal(t, assemble.SeverityWarn, fDone.Severity)
}

// Test_check_orders_findings_specification_then_state_then_steps_ascending
// pins the byte-exact ordering contract across one fixture carrying nine
// of Check's ten rules (every one but C2, which is mutually exclusive with
// C3-C5): the specification (C1), then the state file's C3-C5 in that
// order, then STEP-01 through STEP-05 ascending, each contributing the
// rule its body triggers.
func Test_check_orders_findings_specification_then_state_then_steps_ascending(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	cfg.HandoffCapLines = 10
	cfg.StateCapLines = 20
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	spec := "# demo\n\nno progress list here\n"
	state := checkStateOfLines(cfg, 21) + "```\nunterminated\n"

	checkWriteFeature(t, cfg, featureDir, spec, state)

	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))
	checkWriteHandoff(t, cfg, featureDir, "STEP-01", string(checkBodyOfLines(11)))

	checkWriteStep(t, featureDir, "STEP-02", checkStepBody(cfg, "STEP-02", "done", nil,
		[]string{"- [x] first thing", "- [ ] second thing"}))

	checkWriteStep(t, featureDir, "STEP-03", checkStepBody(cfg, "STEP-03", "open", []string{"STEP-98"}, []string{"- [ ] a task"}))

	checkWriteStep(t, featureDir, "STEP-04", checkStepBody(cfg, "STEP-04", "open", []string{"STEP-04"}, []string{"- [ ] a task"}))

	checkWriteStep(t, featureDir, "STEP-05", "not frontmatter at all\n")

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	problems := make([]string, 0, len(findings))
	for _, f := range findings {
		problems = append(problems, f.Problem)
	}

	assert.Equal(t, []string{
		fmt.Sprintf("no %q heading found", cfg.ProgressHeading),
		"state is 23 lines, over the cap of 20",
		"state has an unclosed ``` fence",
		"handoff is 11 lines, over the cap of 10",
		`checklist item "second thing" is not ticked`,
		`step "STEP-03" depends on "STEP-98", which names no step file`,
		`step "STEP-04" depends on "STEP-04", which is not finished`,
		"frontmatter does not parse: no frontmatter found",
	}, problems)

	for _, f := range findings {
		assert.Equal(t, assemble.SeverityError, f.Severity)
	}
}

func Test_check_reports_nothing_for_a_conforming_feature(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))
	checkWriteHandoff(t, cfg, featureDir, "STEP-01", "a fine handoff\n")
	checkWriteStep(t, featureDir, "STEP-02", checkStepBody(cfg, "STEP-02", "open", []string{"STEP-01"}, []string{"- [ ] a task"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "")
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func Test_check_returns_nil_when_the_feature_directory_root_does_not_exist(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "")
	require.NoError(t, err)
	assert.Nil(t, findings)
}

func Test_check_refuses_an_unknown_named_feature(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "ghost")
	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
	assert.Nil(t, findings)
}

// Test_check_checks_only_the_named_feature is the scoping half of the
// contract: a malformed sibling feature must not contribute findings when
// only one feature is named.
func Test_check_checks_only_the_named_feature(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()

	goodDir := checkFeatureDir(cfg, root, "alpha")
	checkWriteFeature(t, cfg, goodDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, goodDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))

	badDir := checkFeatureDir(cfg, root, "beta")
	checkWriteFeature(t, cfg, badDir, "# beta\n\nno progress list here\n", checkConformingState(cfg))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "alpha")
	require.NoError(t, err)
	assert.Empty(t, findings)
}
