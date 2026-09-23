package assemble_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/stepfile"
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

// onlyFinding is a small assertion helper: it requires findings to have
// exactly one entry and returns it, so call sites read as one line rather
// than three.
func onlyFinding(t *testing.T, findings []assemble.Finding) assemble.Finding {
	t.Helper()

	require.Len(t, findings, 1)

	return findings[0]
}

// checkFSRunner returns a thunk that runs CheckFS against fsys, compiling
// cfg's own step-file and handoff-file patterns the same way
// (*assemble.Server).Check does once per call.
func checkFSRunner(t *testing.T, cfg config.Config, fsys assemble.FeatureFS) func() ([]assemble.Finding, error) {
	t.Helper()

	pattern, err := stepfile.Compile(cfg.StepFilePattern)
	require.NoError(t, err)
	handoffPattern, err := stepfile.CompileHandoff(pattern, cfg.HandoffFileSuffix, cfg.StateFile, cfg.SpecificationFile)
	require.NoError(t, err)

	srv := assemble.NewServer(cfg, "")

	return func() ([]assemble.Finding, error) {
		return srv.CheckFS(fsys, pattern, handoffPattern), nil
	}
}

// ruleCase is one row of
// Test_check_assigns_each_producer_its_stable_rule_id's table: setup
// builds the smallest MapFS fixture that trips exactly one of CheckFS's
// content producers; wantRule is that producer's own stable Rule. The
// three producers whose own subject is the OS adapter one level up — an
// escaping symlink, a symlinked feature directory, an unreadable feature
// directory — are check_disk_test.go's own table instead.
type ruleCase struct {
	name     string
	setup    func(t *testing.T) func() ([]assemble.Finding, error)
	wantRule assemble.Rule
}

// ruleCaseSpecMissing builds a feature with no specification file.
func ruleCaseSpecMissing(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	fsys := featureFS(map[string]string{cfg.StateFile: checkConformingState(cfg)})

	return checkFSRunner(t, cfg, fsys)
}

// ruleCaseSpecUnreadable builds a feature whose specification read fails
// for a reason other than not existing — injected through failFS, since
// specFault's own contract is "classify this read error", not "reproduce a
// real EISDIR": the classification does not care what OS fact produced the
// error, only that it is not fs.ErrNotExist.
func ruleCaseSpecUnreadable(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	fsys := featureFS(map[string]string{cfg.StateFile: checkConformingState(cfg)})
	fsys.FS = failFS{FS: fsys.FS, failReadFile: cfg.SpecificationFile, err: syscall.EACCES}

	return checkFSRunner(t, cfg, fsys)
}

// ruleCaseSpecFence builds a feature whose specification opens a fence it
// never closes.
func ruleCaseSpecFence(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	fsys := featureFS(map[string]string{
		cfg.SpecificationFile: "# demo\n\n```\nunterminated\n",
		cfg.StateFile:         checkConformingState(cfg),
	})

	return checkFSRunner(t, cfg, fsys)
}

// ruleCaseSpecHeading builds a feature whose specification carries no
// progress heading.
func ruleCaseSpecHeading(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	fsys := featureFS(map[string]string{
		cfg.SpecificationFile: "# demo\n\nno progress list here\n",
		cfg.StateFile:         checkConformingState(cfg),
	})

	return checkFSRunner(t, cfg, fsys)
}

// ruleCaseStateMissing builds a feature with no state file.
func ruleCaseStateMissing(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	fsys := featureFS(map[string]string{cfg.SpecificationFile: checkConformingSpec(cfg)})

	return checkFSRunner(t, cfg, fsys)
}

// ruleCaseStateUnreadable builds a feature whose state read fails for a
// reason other than not existing, the state-side twin of
// ruleCaseSpecUnreadable.
func ruleCaseStateUnreadable(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	fsys := featureFS(map[string]string{cfg.SpecificationFile: checkConformingSpec(cfg)})
	fsys.FS = failFS{FS: fsys.FS, failReadFile: cfg.StateFile, err: syscall.EACCES}

	return checkFSRunner(t, cfg, fsys)
}

// ruleCaseStateCap builds a feature whose state body is over the
// configured cap.
func ruleCaseStateCap(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	cfg.StateCapLines = 20
	fsys := featureFS(map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		cfg.StateFile:         checkStateOfLines(cfg, 21),
	})

	return checkFSRunner(t, cfg, fsys)
}

// ruleCaseStateFence builds a feature whose state body opens a fence it
// never closes.
func ruleCaseStateFence(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	fsys := featureFS(map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		cfg.StateFile:         checkStateUnterminatedFence(cfg),
	})

	return checkFSRunner(t, cfg, fsys)
}

// ruleCaseStateHeading builds a feature whose state body is missing one of
// its configured headings.
func ruleCaseStateHeading(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	fsys := featureFS(map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		cfg.StateFile:         checkStateMissingHeading(cfg, cfg.StateHeadings.Traps),
	})

	return checkFSRunner(t, cfg, fsys)
}

// ruleCaseStepsUnlistable builds a feature whose step-file listing fails,
// injected through failFS the same way ruleCaseSpecUnreadable's read
// failure is.
func ruleCaseStepsUnlistable(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	fsys := featureFS(conformingFeatureFiles(cfg))
	fsys.FS = failFS{FS: fsys.FS, failReadDir: ".", err: syscall.EACCES}

	return checkFSRunner(t, cfg, fsys)
}

// ruleCaseFrontmatter builds a feature whose step file carries no
// frontmatter at all.
func ruleCaseFrontmatter(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	fsys := featureFS(map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		cfg.StateFile:         checkConformingState(cfg),
		"STEP-01.md":          "not frontmatter at all\n",
	})

	return checkFSRunner(t, cfg, fsys)
}

// ruleCaseChecklist builds a feature whose done step carries an unticked
// checklist item.
func ruleCaseChecklist(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	fsys := featureFS(map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		cfg.StateFile:         checkConformingState(cfg),
		"STEP-01.md": checkStepBody(cfg, "STEP-01", "done", nil,
			[]string{"- [x] first thing", "- [ ] second thing"}),
	})

	return checkFSRunner(t, cfg, fsys)
}

// ruleCaseDependsOnSelf builds a feature whose step depends on itself.
func ruleCaseDependsOnSelf(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	fsys := featureFS(map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		cfg.StateFile:         checkConformingState(cfg),
		"STEP-01.md":          checkStepBody(cfg, "STEP-01", "open", []string{"STEP-01"}, []string{"- [ ] a task"}),
	})

	return checkFSRunner(t, cfg, fsys)
}

// ruleCaseDependsOnDangling builds a feature whose step depends on an id
// naming no step file.
func ruleCaseDependsOnDangling(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	fsys := featureFS(map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		cfg.StateFile:         checkConformingState(cfg),
		"STEP-01.md":          checkStepBody(cfg, "STEP-01", "open", []string{"STEP-99"}, []string{"- [ ] a task"}),
	})

	return checkFSRunner(t, cfg, fsys)
}

// ruleCaseHandoffCap builds a feature whose done step's handoff file is
// over the configured cap.
func ruleCaseHandoffCap(t *testing.T) func() ([]assemble.Finding, error) {
	t.Helper()

	cfg := fixtureConfig()
	cfg.HandoffCapLines = 10
	fsys := featureFS(map[string]string{
		cfg.SpecificationFile:             checkConformingSpec(cfg),
		cfg.StateFile:                     checkConformingState(cfg),
		"STEP-01.md":                      checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}),
		"STEP-01" + cfg.HandoffFileSuffix: string(checkBodyOfLines(11)),
	})

	return checkFSRunner(t, cfg, fsys)
}

// Test_check_assigns_each_producer_its_stable_rule_id pins R8: every one of
// CheckFS's content producers stamps its Finding with a stable,
// never-renamed Rule id a script can branch on instead of parsing English.
func Test_check_assigns_each_producer_its_stable_rule_id(t *testing.T) {
	cases := []ruleCase{
		{name: "spec missing", setup: ruleCaseSpecMissing, wantRule: assemble.RuleSpecMissing},
		{name: "spec unreadable", setup: ruleCaseSpecUnreadable, wantRule: assemble.RuleSpecUnreadable},
		{name: "spec fence", setup: ruleCaseSpecFence, wantRule: assemble.RuleFence},
		{name: "spec heading", setup: ruleCaseSpecHeading, wantRule: assemble.RuleHeading},
		{name: "state missing", setup: ruleCaseStateMissing, wantRule: assemble.RuleStateMissing},
		{name: "state unreadable", setup: ruleCaseStateUnreadable, wantRule: assemble.RuleStateUnreadable},
		{name: "state cap", setup: ruleCaseStateCap, wantRule: assemble.RuleStateCap},
		{name: "state fence", setup: ruleCaseStateFence, wantRule: assemble.RuleFence},
		{name: "state heading", setup: ruleCaseStateHeading, wantRule: assemble.RuleHeading},
		{name: "steps unlistable", setup: ruleCaseStepsUnlistable, wantRule: assemble.RuleStepsUnlistable},
		{name: "frontmatter", setup: ruleCaseFrontmatter, wantRule: assemble.RuleFrontmatter},
		{name: "checklist", setup: ruleCaseChecklist, wantRule: assemble.RuleChecklist},
		{name: "depends-on self", setup: ruleCaseDependsOnSelf, wantRule: assemble.RuleDependsOn},
		{name: "depends-on dangling", setup: ruleCaseDependsOnDangling, wantRule: assemble.RuleDependsOn},
		{name: "handoff cap", setup: ruleCaseHandoffCap, wantRule: assemble.RuleHandoffCap},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			check := c.setup(t)

			findings, err := check()
			require.NoError(t, err)

			f := onlyFinding(t, findings)
			assert.Equal(t, c.wantRule, f.Rule)
		})
	}
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
	cfg.HandoffCapLines = 10
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"})
	files["STEP-01"+cfg.HandoffFileSuffix] = string(checkBodyOfLines(11))

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-01"+cfg.HandoffFileSuffix), f.Path)
	assert.Equal(t, 11, f.Line, "a cap finding's line is cap+1, the first line over it")
	assert.Contains(t, f.Detail, "11")
	assert.Contains(t, f.Detail, "10")
	assert.Contains(t, f.Detail, "over the cap")
	assert.Equal(t, assemble.SeverityWarn, f.Severity)
}

// Test_check_reports_nothing_for_a_missing_handoff_file is the control arm
// for C10: a done step with no handoff file at all is not a finding — a
// missing handoff is exactly "this step has never been finished under a
// caps-aware finish", which scaffold.Finish's own re-finish exemption
// (R16) already treats as nothing to diverge from.
func Test_check_reports_nothing_for_a_missing_handoff_file(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"})

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func Test_check_reports_a_state_body_over_the_configured_cap(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StateCapLines = 20
	files := map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		cfg.StateFile:         checkStateOfLines(cfg, 21),
		"STEP-01.md":          checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}),
	}

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.StateFile), f.Path)
	assert.Equal(t, 21, f.Line, "a cap finding's line is cap+1, the first line over it")
	assert.Equal(t, "state is 21 lines, over the cap of 20", f.Detail)
}

func Test_check_reports_a_state_body_with_an_unterminated_fence(t *testing.T) {
	cfg := fixtureConfig()
	files := map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		cfg.StateFile:         checkStateUnterminatedFence(cfg),
		"STEP-01.md":          checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}),
	}

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.StateFile), f.Path)
	assert.Equal(t, "state has an unclosed ``` fence", f.Detail)
}

func Test_check_reports_a_state_body_missing_a_required_heading(t *testing.T) {
	cfg := fixtureConfig()
	files := map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		cfg.StateFile:         checkStateMissingHeading(cfg, cfg.StateHeadings.Traps),
		"STEP-01.md":          checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}),
	}

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.StateFile), f.Path)
	assert.Equal(t, `state is missing the "## Gotchas" section`, f.Detail)
}

func Test_check_reports_a_missing_state_file(t *testing.T) {
	cfg := fixtureConfig()
	files := map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		"STEP-01.md":          checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}),
	}

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.StateFile), f.Path)
	assert.Contains(t, f.Detail, "file does not exist")
}

// Test_check_marks_a_zero_step_feature_in_flight_not_complete is the cheap
// optional closing STATE.md's own trap: a feature with no step files at
// all is vacuously "every step done" by a naive empty loop, but
// assemble.Status's own Complete() requires Total > 0 and so calls the
// same feature "in progress". A zero-step feature's own finding (here, a
// missing state heading — C5, which fires regardless of step count) must
// take SeverityError and InFlight true, agreeing with Status rather than
// contradicting it with a WARN/"(complete)" reading.
func Test_check_marks_a_zero_step_feature_in_flight_not_complete(t *testing.T) {
	cfg := fixtureConfig()
	files := map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		cfg.StateFile:         checkStateMissingHeading(cfg, cfg.StateHeadings.Traps),
	}

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, assemble.SeverityError, f.Severity)
	assert.True(t, f.InFlight)

	groups := assemble.GroupByFeature(findings)
	require.Len(t, groups, 1)
	assert.True(t, groups[0].InFlight)
}

func Test_check_reports_a_specification_with_no_progress_heading(t *testing.T) {
	cfg := fixtureConfig()
	files := map[string]string{
		cfg.SpecificationFile: "# demo\n\nno progress list here\n",
		cfg.StateFile:         checkConformingState(cfg),
		"STEP-01.md":          checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}),
	}

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.SpecificationFile), f.Path)
	assert.Contains(t, f.Detail, cfg.ProgressHeading)
}

// Test_check_reports_a_step_whose_frontmatter_does_not_parse_and_still_reports_its_handoff_cap
// is C6 paired with C10's independence from it: STEP-01's frontmatter is
// garbage, and its handoff file is over cap — both must be reported, proof
// that Check never reuses readSteps's all-or-nothing stance.
func Test_check_reports_a_step_whose_frontmatter_does_not_parse_and_still_reports_its_handoff_cap(t *testing.T) {
	cfg := fixtureConfig()
	cfg.HandoffCapLines = 10
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = "not frontmatter at all\n"
	files["STEP-01"+cfg.HandoffFileSuffix] = string(checkBodyOfLines(11))

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)
	require.Len(t, findings, 2)

	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-01.md"), findings[0].Path)
	assert.Contains(t, findings[0].Detail, "frontmatter does not parse")
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-01"+cfg.HandoffFileSuffix), findings[1].Path)
	assert.Contains(t, findings[1].Detail, "over the cap")

	for _, f := range findings {
		assert.Equal(t, assemble.SeverityError, f.Severity, "an unreadable step's own feature is always in flight")
	}
}

func Test_check_reports_an_unticked_checklist_item_on_a_done_step(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = checkStepBody(cfg, "STEP-01", "done", nil,
		[]string{"- [x] first thing", "- [ ] second thing"})

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-01.md"), f.Path)
	assert.Equal(t, 12, f.Line)
	assert.Equal(t, `checklist item "second thing" is not ticked`, f.Detail)
}

// Test_check_reports_nothing_for_an_unticked_checklist_item_on_an_open_step
// is C7's control arm: the same unticked item on an open step is ordinary
// in-progress work, not a finding.
func Test_check_reports_nothing_for_an_unticked_checklist_item_on_an_open_step(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = checkStepBody(cfg, "STEP-01", "open", nil,
		[]string{"- [x] first thing", "- [ ] second thing"})

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func Test_check_reports_a_dependency_id_that_names_no_step_file(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = checkStepBody(cfg, "STEP-01", "open", []string{"STEP-99"}, []string{"- [ ] a task"})

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-01.md"), f.Path)
	assert.Equal(t, `step "STEP-01" depends on "STEP-99", which names no step file`, f.Detail)
}

// Test_check_reports_nothing_for_an_ordinary_unmet_dependency is C8's
// control arm: a depends-on id that names a real, known, not-yet-done step
// is ordinary in-progress work, already counted by Status's Blocked.
func Test_check_reports_nothing_for_an_ordinary_unmet_dependency(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = checkStepBody(cfg, "STEP-01", "open", nil, []string{"- [ ] a task"})
	files["STEP-02.md"] = checkStepBody(cfg, "STEP-02", "open", []string{"STEP-01"}, []string{"- [ ] a task"})

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func Test_check_reports_a_step_that_depends_on_itself(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = checkStepBody(cfg, "STEP-01", "open", []string{"STEP-01"}, []string{"- [ ] a task"})

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-01.md"), f.Path)
	assert.Equal(t, `step "STEP-01" depends on "STEP-01", which is not finished`, f.Detail)
}

// Test_check_reports_a_dangling_dependency_that_is_not_first_in_the_list is
// C8's masking regression: idx.FirstUnmet stops at the first unmet id, so a
// dangling id (STEP-99) listed after a merely-open-and-known one (STEP-01)
// used to be silently dropped. checkStepDependencyFindings must walk every
// declared id rather than reuse that refusal predicate.
func Test_check_reports_a_dangling_dependency_that_is_not_first_in_the_list(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = checkStepBody(cfg, "STEP-01", "open", nil, []string{"- [ ] a task"})
	files["STEP-02.md"] = checkStepBody(cfg, "STEP-02", "open", []string{"STEP-01", "STEP-99"}, []string{"- [ ] a task"})

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-02.md"), f.Path)
	assert.Equal(t, `step "STEP-02" depends on "STEP-99", which names no step file`, f.Detail)
}

// Test_check_reports_a_self_dependency_masked_behind_an_earlier_unmet_dependency
// is C9's counterpart masking regression: a self-dependency listed after an
// ordinary open-and-known one used to be hidden the same way — FirstUnmet
// returned the earlier, unmet-but-known id and never reached the self id at
// all.
func Test_check_reports_a_self_dependency_masked_behind_an_earlier_unmet_dependency(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = checkStepBody(cfg, "STEP-01", "open", nil, []string{"- [ ] a task"})
	files["STEP-02.md"] = checkStepBody(cfg, "STEP-02", "open", []string{"STEP-01", "STEP-02"}, []string{"- [ ] a task"})

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-02.md"), f.Path)
	assert.Equal(t, `step "STEP-02" depends on "STEP-02", which is not finished`, f.Detail)
}

// Test_check_reports_a_self_dependency_on_a_done_step is C9's done-step
// regression: idx.FirstUnmet short-circuits on fm.Done(), so a done step's
// self-dependency used to report nothing at all — a permanently
// unfinishable step (per stepfile's own doc) that Check is the only reader
// positioned to flag, per R18's backstop role. SCENARIO-22 narrowed only
// C7 (the checklist rule) to done steps; C8 and C9 carry no such narrowing.
func Test_check_reports_a_self_dependency_on_a_done_step(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = checkStepBody(cfg, "STEP-01", "done", []string{"STEP-01"}, []string{"- [x] a task"})

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-01.md"), f.Path)
	assert.Equal(t, `step "STEP-01" depends on "STEP-01", which is not finished`, f.Detail)
	assert.Equal(t, assemble.SeverityWarn, f.Severity)
}

// Test_check_reports_a_dangling_dependency_on_a_done_step is C8's
// done-step regression, the same short-circuit as the self-dependency case
// above but for an id that names no step file at all.
func Test_check_reports_a_dangling_dependency_on_a_done_step(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = checkStepBody(cfg, "STEP-01", "done", []string{"STEP-99"}, []string{"- [x] a task"})

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-01.md"), f.Path)
	assert.Equal(t, `step "STEP-01" depends on "STEP-99", which names no step file`, f.Detail)
	assert.Equal(t, assemble.SeverityWarn, f.Severity)
}

// Test_check_assigns_ERROR_when_a_feature_is_still_in_flight_and_WARN_when_every_step_is_done
// pins severity's one variable: the same defect (an over-cap handoff) on a
// feature with an open sibling step is ERROR; on a feature whose every
// step is done, it is WARN.
func Test_check_assigns_ERROR_when_a_feature_is_still_in_flight_and_WARN_when_every_step_is_done(t *testing.T) {
	cfg := fixtureConfig()
	cfg.HandoffCapLines = 10

	inFlightFiles := conformingFeatureFiles(cfg)
	inFlightFiles["STEP-01.md"] = checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"})
	inFlightFiles["STEP-01"+cfg.HandoffFileSuffix] = string(checkBodyOfLines(11))
	inFlightFiles["STEP-02.md"] = checkStepBody(cfg, "STEP-02", "open", nil, []string{"- [ ] a task"})

	findingsInFlight, err := checkFSRunner(t, cfg, featureFS(inFlightFiles))()
	require.NoError(t, err)
	f := onlyFinding(t, findingsInFlight)
	assert.Equal(t, assemble.SeverityError, f.Severity)

	doneFiles := conformingFeatureFiles(cfg)
	doneFiles["STEP-01.md"] = checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"})
	doneFiles["STEP-01"+cfg.HandoffFileSuffix] = string(checkBodyOfLines(11))

	findingsDone, err := checkFSRunner(t, cfg, featureFS(doneFiles))()
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
	cfg.HandoffCapLines = 10
	cfg.StateCapLines = 20

	files := map[string]string{
		cfg.SpecificationFile:             "# demo\n\nno progress list here\n",
		cfg.StateFile:                     checkStateOfLines(cfg, 21) + "```\nunterminated\n",
		"STEP-01.md":                      checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}),
		"STEP-01" + cfg.HandoffFileSuffix: string(checkBodyOfLines(11)),
		"STEP-02.md": checkStepBody(cfg, "STEP-02", "done", nil,
			[]string{"- [x] first thing", "- [ ] second thing"}),
		"STEP-03.md": checkStepBody(cfg, "STEP-03", "open", []string{"STEP-98"}, []string{"- [ ] a task"}),
		"STEP-04.md": checkStepBody(cfg, "STEP-04", "open", []string{"STEP-04"}, []string{"- [ ] a task"}),
		"STEP-05.md": "not frontmatter at all\n",
	}

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)

	problems := make([]string, 0, len(findings))
	rules := make([]assemble.Rule, 0, len(findings))

	for _, f := range findings {
		problems = append(problems, f.Detail)
		rules = append(rules, f.Rule)
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

	assert.Equal(t, []assemble.Rule{
		assemble.RuleHeading,
		assemble.RuleStateCap,
		assemble.RuleFence,
		assemble.RuleHandoffCap,
		assemble.RuleChecklist,
		assemble.RuleDependsOn,
		assemble.RuleDependsOn,
		assemble.RuleFrontmatter,
	}, rules)

	for _, f := range findings {
		assert.Equal(t, assemble.SeverityError, f.Severity)
	}
}

// Test_check_reports_nothing_for_a_conforming_feature is the population
// control every rule test above implicitly relies on: a feature that trips
// none of C1-C10 reports no findings at all.
func Test_check_reports_nothing_for_a_conforming_feature(t *testing.T) {
	cfg := fixtureConfig()
	files := conformingFeatureFiles(cfg)
	files["STEP-01.md"] = checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"})
	files["STEP-01"+cfg.HandoffFileSuffix] = "a fine handoff\n"
	files["STEP-02.md"] = checkStepBody(cfg, "STEP-02", "open", []string{"STEP-01"}, []string{"- [ ] a task"})

	findings, err := checkFSRunner(t, cfg, featureFS(files))()
	require.NoError(t, err)
	assert.Empty(t, findings)
}

// Test_check_reports_a_feature_whose_step_files_cannot_be_listed is the
// same C3 defect one layer deeper than an unreadable feature directory: the
// feature directory opens fine but its step files cannot be listed (the
// branch a stricter-than-darwin permission model, such as Linux's, takes
// at a directory readable to enter but not to list). checkStepFindings
// used to swallow this failure silently (return nil, true) rather than
// report it. It carries fuller assertions than the "steps unlistable" row
// in the table above, which only pins the Rule.
func Test_check_reports_a_feature_whose_step_files_cannot_be_listed(t *testing.T) {
	cfg := fixtureConfig()
	fsys := featureFS(conformingFeatureFiles(cfg))
	fsys.FS = failFS{FS: fsys.FS, failReadDir: ".", err: syscall.EACCES}

	findings, err := checkFSRunner(t, cfg, fsys)()
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, assemble.RuleStepsUnlistable, f.Rule)
	assert.Equal(t, testFeaturePath, f.Path)
	assert.Contains(t, f.Detail, "permission denied")
	assert.Equal(t, assemble.SeverityError, f.Severity)
}

// Test_GroupByFeature_folds_findings_into_one_group_per_feature_in_order
// pins the fold: two features' findings collapse into one FeatureFindings
// apiece, in first-appearance order, each carrying its Name/Path/InFlight
// from the findings themselves and every one of its own findings verbatim.
func Test_GroupByFeature_folds_findings_into_one_group_per_feature_in_order(t *testing.T) {
	findings := []assemble.Finding{
		{Rule: assemble.RuleChecklist, Feature: "alpha", FeaturePath: "/repo/docs/specifications/alpha", InFlight: true, Detail: "alpha finding 1"},
		{Rule: assemble.RuleHandoffCap, Feature: "alpha", FeaturePath: "/repo/docs/specifications/alpha", InFlight: true, Detail: "alpha finding 2"},
		{Rule: assemble.RuleStateCap, Feature: "beta", FeaturePath: "/repo/docs/specifications/beta", InFlight: false, Detail: "beta finding"},
	}

	groups := assemble.GroupByFeature(findings)

	require.Len(t, groups, 2)

	assert.Equal(t, "alpha", groups[0].Name)
	assert.Equal(t, "/repo/docs/specifications/alpha", groups[0].Path)
	assert.True(t, groups[0].InFlight)
	assert.Equal(t, findings[0:2], groups[0].Findings)

	assert.Equal(t, "beta", groups[1].Name)
	assert.Equal(t, "/repo/docs/specifications/beta", groups[1].Path)
	assert.False(t, groups[1].InFlight)
	assert.Equal(t, findings[2:3], groups[1].Findings)
}

// Test_GroupByFeature_returns_an_empty_result_for_no_findings is the
// zero-input arm: no findings folds into no groups.
func Test_GroupByFeature_returns_an_empty_result_for_no_findings(t *testing.T) {
	groups := assemble.GroupByFeature(nil)

	assert.Empty(t, groups)
}
