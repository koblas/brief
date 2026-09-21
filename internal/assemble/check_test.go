package assemble_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

// ruleCase is one row of Test_check_assigns_each_producer_its_stable_rule_id's
// table: setup builds the smallest fixture (or Server override) that trips
// exactly one of Check's producers and returns the Server plus the feature
// argument to check; wantRule is that producer's own stable Rule.
type ruleCase struct {
	name     string
	setup    func(t *testing.T) (*assemble.Server, string)
	wantRule assemble.Rule
}

// ruleCaseSpecMissing builds a feature with no specification file.
func ruleCaseSpecMissing(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(checkConformingState(cfg)), 0o600))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseSpecUnreadable builds a feature whose specification path is a
// directory, so reading it as a file fails without depending on OS
// permission bits.
func ruleCaseSpecUnreadable(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(featureDir, cfg.SpecificationFile), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(checkConformingState(cfg)), 0o600))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseSpecFence builds a feature whose specification opens a fence it
// never closes.
func ruleCaseSpecFence(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	checkWriteFeature(t, cfg, featureDir, "# demo\n\n```\nunterminated\n", checkConformingState(cfg))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseSpecHeading builds a feature whose specification carries no
// progress heading.
func ruleCaseSpecHeading(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	checkWriteFeature(t, cfg, featureDir, "# demo\n\nno progress list here\n", checkConformingState(cfg))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseStateMissing builds a feature with no state file.
func ruleCaseStateMissing(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(checkConformingSpec(cfg)), 0o600))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseStateUnreadable builds a feature whose state path is a
// directory.
func ruleCaseStateUnreadable(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(checkConformingSpec(cfg)), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(featureDir, cfg.StateFile), 0o755))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseStateCap builds a feature whose state body is over the
// configured cap.
func ruleCaseStateCap(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	cfg.StateCapLines = 20
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkStateOfLines(cfg, 21))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseStateFence builds a feature whose state body opens a fence it
// never closes.
func ruleCaseStateFence(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkStateUnterminatedFence(cfg))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseStateHeading builds a feature whose state body is missing one of
// its configured headings.
func ruleCaseStateHeading(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkStateMissingHeading(cfg, cfg.StateHeadings.Traps))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseStepsUnlistable builds a Server whose readDir seam is overridden
// to fail listing a feature's step files.
func ruleCaseStepsUnlistable(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))

	srv := assemble.NewServer(cfg, root)
	assemble.SetReadDirForTest(srv, func(*os.Root) ([]os.DirEntry, error) {
		return nil, &fs.PathError{Op: "readdirent", Path: ".", Err: syscall.EACCES}
	})

	return srv, "demo"
}

// ruleCaseStepUnreadable builds a feature whose step file is a symlink
// escaping the feature root, so reading it fails.
func ruleCaseStepUnreadable(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	require.NoError(t, os.Symlink(filepath.Join(featureDir, "nonexistent-target.md"), filepath.Join(featureDir, "STEP-01.md")))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseFrontmatter builds a feature whose step file carries no
// frontmatter at all.
func ruleCaseFrontmatter(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", "not frontmatter at all\n")

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseChecklist builds a feature whose done step carries an unticked
// checklist item.
func ruleCaseChecklist(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil,
		[]string{"- [x] first thing", "- [ ] second thing"}))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseDependsOnSelf builds a feature whose step depends on itself.
func ruleCaseDependsOnSelf(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "open", []string{"STEP-01"}, []string{"- [ ] a task"}))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseDependsOnDangling builds a feature whose step depends on an id
// naming no step file.
func ruleCaseDependsOnDangling(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "open", []string{"STEP-99"}, []string{"- [ ] a task"}))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseHandoffCap builds a feature whose done step's handoff file is
// over the configured cap.
func ruleCaseHandoffCap(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	cfg.HandoffCapLines = 10
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))
	checkWriteHandoff(t, cfg, featureDir, "STEP-01", string(checkBodyOfLines(11)))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseFeatureSymlink builds a symlink where a feature directory is
// expected.
func ruleCaseFeatureSymlink(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	realDir := filepath.Join(root, "real-demo")
	checkWriteFeature(t, cfg, realDir, checkConformingSpec(cfg), checkConformingState(cfg))
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	require.NoError(t, os.Symlink(realDir, filepath.Join(root, cfg.FeatureDirectory, "demo")))

	return assemble.NewServer(cfg, root), "demo"
}

// ruleCaseFeatureUnreadable builds a Server whose openRoot seam is
// overridden to fail opening a feature directory.
func ruleCaseFeatureUnreadable(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	srv := assemble.NewServer(cfg, root)
	assemble.SetOpenRootForTest(srv, func(parent *os.Root, name string) (*os.Root, error) {
		if name == "demo" {
			return nil, &fs.PathError{Op: "openat", Path: name, Err: syscall.EACCES}
		}

		return parent.OpenRoot(name)
	})

	return srv, "demo"
}

// Test_check_assigns_each_producer_its_stable_rule_id pins R8: every one of
// Check's producers stamps its Finding with a stable, never-renamed Rule id
// a script can branch on instead of parsing English.
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
		{name: "step unreadable", setup: ruleCaseStepUnreadable, wantRule: assemble.RuleStepUnreadable},
		{name: "frontmatter", setup: ruleCaseFrontmatter, wantRule: assemble.RuleFrontmatter},
		{name: "checklist", setup: ruleCaseChecklist, wantRule: assemble.RuleChecklist},
		{name: "depends-on self", setup: ruleCaseDependsOnSelf, wantRule: assemble.RuleDependsOn},
		{name: "depends-on dangling", setup: ruleCaseDependsOnDangling, wantRule: assemble.RuleDependsOn},
		{name: "handoff cap", setup: ruleCaseHandoffCap, wantRule: assemble.RuleHandoffCap},
		{name: "feature symlink", setup: ruleCaseFeatureSymlink, wantRule: assemble.RuleFeatureSymlink},
		{name: "feature unreadable", setup: ruleCaseFeatureUnreadable, wantRule: assemble.RuleFeatureUnreadable},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, feature := c.setup(t)

			findings, err := srv.Check(t.Context(), feature)
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
	assert.Equal(t, "state is 21 lines, over the cap of 20", f.Detail)
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
	assert.Equal(t, "state has an unclosed ``` fence", f.Detail)
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
	assert.Equal(t, `state is missing the "## Gotchas" section`, f.Detail)
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
	assert.Contains(t, f.Detail, "no such file")
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
	assert.Contains(t, f.Detail, cfg.ProgressHeading)
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
	assert.Contains(t, findings[0].Detail, "frontmatter does not parse")
	assert.Equal(t, filepath.Join(featureDir, "STEP-01"+cfg.HandoffFileSuffix), findings[1].Path)
	assert.Contains(t, findings[1].Detail, "over the cap")

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
	assert.Equal(t, `checklist item "second thing" is not ticked`, f.Detail)
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
	assert.Equal(t, `step "STEP-01" depends on "STEP-99", which names no step file`, f.Detail)
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
	assert.Equal(t, `step "STEP-01" depends on "STEP-01", which is not finished`, f.Detail)
}

// Test_check_reports_a_dangling_dependency_that_is_not_first_in_the_list is
// C8's masking regression: idx.FirstUnmet stops at the first unmet id, so a
// dangling id (STEP-99) listed after a merely-open-and-known one (STEP-01)
// used to be silently dropped. checkStepDependencyFindings must walk every
// declared id rather than reuse that refusal predicate.
func Test_check_reports_a_dangling_dependency_that_is_not_first_in_the_list(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "open", nil, []string{"- [ ] a task"}))
	checkWriteStep(t, featureDir, "STEP-02", checkStepBody(cfg, "STEP-02", "open", []string{"STEP-01", "STEP-99"}, []string{"- [ ] a task"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(featureDir, "STEP-02.md"), f.Path)
	assert.Equal(t, `step "STEP-02" depends on "STEP-99", which names no step file`, f.Detail)
}

// Test_check_reports_a_self_dependency_masked_behind_an_earlier_unmet_dependency
// is C9's counterpart masking regression: a self-dependency listed after an
// ordinary open-and-known one used to be hidden the same way — FirstUnmet
// returned the earlier, unmet-but-known id and never reached the self id at
// all.
func Test_check_reports_a_self_dependency_masked_behind_an_earlier_unmet_dependency(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "open", nil, []string{"- [ ] a task"}))
	checkWriteStep(t, featureDir, "STEP-02", checkStepBody(cfg, "STEP-02", "open", []string{"STEP-01", "STEP-02"}, []string{"- [ ] a task"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(featureDir, "STEP-02.md"), f.Path)
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
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", []string{"STEP-01"}, []string{"- [x] a task"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(featureDir, "STEP-01.md"), f.Path)
	assert.Equal(t, `step "STEP-01" depends on "STEP-01", which is not finished`, f.Detail)
	assert.Equal(t, assemble.SeverityWarn, f.Severity)
}

// Test_check_reports_a_dangling_dependency_on_a_done_step is C8's
// done-step regression, the same short-circuit as the self-dependency case
// above but for an id that names no step file at all.
func Test_check_reports_a_dangling_dependency_on_a_done_step(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", []string{"STEP-99"}, []string{"- [x] a task"}))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, filepath.Join(featureDir, "STEP-01.md"), f.Path)
	assert.Equal(t, `step "STEP-01" depends on "STEP-99", which names no step file`, f.Detail)
	assert.Equal(t, assemble.SeverityWarn, f.Severity)
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

// Test_check_stamps_an_in_flight_ordinary_feature_finding_with_its_name_path_and_in_flight
// pins Finding.Feature/FeaturePath/InFlight for an ordinary (non
// feature-level) finding on a feature still in flight, through both the
// all-features scan and the named-feature path.
func Test_check_stamps_an_in_flight_ordinary_feature_finding_with_its_name_path_and_in_flight(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "open", []string{"STEP-99"}, []string{"- [ ] a task"}))

	srv := assemble.NewServer(cfg, root)

	allFindings, err := srv.Check(t.Context(), "")
	require.NoError(t, err)
	allFinding := onlyFinding(t, allFindings)
	assert.Equal(t, "demo", allFinding.Feature)
	assert.Equal(t, featureDir, allFinding.FeaturePath)
	assert.True(t, allFinding.InFlight)

	namedFindings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)
	namedFinding := onlyFinding(t, namedFindings)
	assert.Equal(t, "demo", namedFinding.Feature)
	assert.Equal(t, featureDir, namedFinding.FeaturePath)
	assert.True(t, namedFinding.InFlight)
}

// Test_check_stamps_a_complete_ordinary_feature_finding_with_its_name_path_and_in_flight
// is the complete-feature counterpart: InFlight is false when every step
// reads as done, through both entry paths.
func Test_check_stamps_a_complete_ordinary_feature_finding_with_its_name_path_and_in_flight(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	cfg.HandoffCapLines = 10
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))
	checkWriteHandoff(t, cfg, featureDir, "STEP-01", string(checkBodyOfLines(11)))

	srv := assemble.NewServer(cfg, root)

	allFindings, err := srv.Check(t.Context(), "")
	require.NoError(t, err)
	allFinding := onlyFinding(t, allFindings)
	assert.Equal(t, "demo", allFinding.Feature)
	assert.Equal(t, featureDir, allFinding.FeaturePath)
	assert.False(t, allFinding.InFlight)

	namedFindings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)
	namedFinding := onlyFinding(t, namedFindings)
	assert.Equal(t, "demo", namedFinding.Feature)
	assert.Equal(t, featureDir, namedFinding.FeaturePath)
	assert.False(t, namedFinding.InFlight)
}

// Test_check_stamps_a_symlinked_feature_finding_with_its_own_name_path_and_in_flight_true
// pins the same three fields for symlinkFeatureFinding, which never passes
// through checkFeatureDir's own stamping loop — the trap a new
// feature-level producer must not repeat.
func Test_check_stamps_a_symlinked_feature_finding_with_its_own_name_path_and_in_flight_true(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()

	realDir := filepath.Join(root, "real-gamma")
	checkWriteFeature(t, cfg, realDir, checkConformingSpec(cfg), checkConformingState(cfg))

	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	linkPath := filepath.Join(root, cfg.FeatureDirectory, "gamma")
	require.NoError(t, os.Symlink(realDir, linkPath))

	srv := assemble.NewServer(cfg, root)

	allFindings, err := srv.Check(t.Context(), "")
	require.NoError(t, err)
	allFinding := onlyFinding(t, allFindings)
	assert.Equal(t, "gamma", allFinding.Feature)
	assert.Equal(t, linkPath, allFinding.FeaturePath)
	assert.True(t, allFinding.InFlight)

	namedFindings, err := srv.Check(t.Context(), "gamma")
	require.NoError(t, err)
	namedFinding := onlyFinding(t, namedFindings)
	assert.Equal(t, "gamma", namedFinding.Feature)
	assert.Equal(t, linkPath, namedFinding.FeaturePath)
	assert.True(t, namedFinding.InFlight)
}

// Test_check_stamps_an_unreadable_feature_finding_with_its_own_name_path_and_in_flight_true
// is unreadableFeatureFinding's counterpart to the symlink test above — the
// same trap, the other feature-level producer.
func Test_check_stamps_an_unreadable_feature_finding_with_its_own_name_path_and_in_flight_true(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	injectUnreadable := func(parent *os.Root, name string) (*os.Root, error) {
		if name == "demo" {
			return nil, &fs.PathError{Op: "openat", Path: name, Err: syscall.EACCES}
		}

		return parent.OpenRoot(name)
	}

	allSrv := assemble.NewServer(cfg, root)
	assemble.SetOpenRootForTest(allSrv, injectUnreadable)

	allFindings, err := allSrv.Check(t.Context(), "")
	require.NoError(t, err)
	allFinding := onlyFinding(t, allFindings)
	assert.Equal(t, "demo", allFinding.Feature)
	assert.Equal(t, featureDir, allFinding.FeaturePath)
	assert.True(t, allFinding.InFlight)

	namedSrv := assemble.NewServer(cfg, root)
	assemble.SetOpenRootForTest(namedSrv, injectUnreadable)

	namedFindings, err := namedSrv.Check(t.Context(), "demo")
	require.NoError(t, err)
	namedFinding := onlyFinding(t, namedFindings)
	assert.Equal(t, "demo", namedFinding.Feature)
	assert.Equal(t, featureDir, namedFinding.FeaturePath)
	assert.True(t, namedFinding.InFlight)
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

// Test_check_refuses_a_named_feature_when_the_feature_root_does_not_exist
// is the named-feature half of the root-missing case: Check("") degrades a
// missing feature-directory root to zero features (nil, nil), but a named
// feature obviously has no directory when the root holding it does not
// exist either, so it must refuse with ErrNoSuchFeature rather than share
// the empty-repository degrade.
func Test_check_refuses_a_named_feature_when_the_feature_root_does_not_exist(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "ghost")
	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
	assert.Nil(t, findings)
}

// Test_check_refuses_a_named_feature_that_is_a_regular_file pins the third
// leg of the same precedence chain: a name that exists under the feature
// directory but is a regular file, not a directory, is not a feature
// either — ErrNoSuchFeature, the same as a name with no entry at all,
// never an "unreadable" Finding.
func Test_check_refuses_a_named_feature_that_is_a_regular_file(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, cfg.FeatureDirectory, "README.md"), []byte("not a feature\n"), 0o600))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "README.md")
	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
	assert.Nil(t, findings)
}

// Test_check_reports_an_unreadable_feature_directory_in_the_all_features_scan
// is C3's companion: a feature directory that exists but cannot be opened
// as its own root must contribute a Finding naming it, not silently zero
// findings — the exact backstop failure R18 gives Check to prevent. The
// injected failure is a fake open, not chmod: root bypasses permission
// checks, so this must reproduce identically under any CI identity.
func Test_check_reports_an_unreadable_feature_directory_in_the_all_features_scan(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	srv := assemble.NewServer(cfg, root)
	assemble.SetOpenRootForTest(srv, func(parent *os.Root, name string) (*os.Root, error) {
		if name == "demo" {
			return nil, &fs.PathError{Op: "openat", Path: name, Err: syscall.EACCES}
		}

		return parent.OpenRoot(name)
	})

	findings, err := srv.Check(t.Context(), "")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, featureDir, f.Path)
	assert.Contains(t, f.Detail, "permission denied")
	assert.Equal(t, assemble.SeverityError, f.Severity)
}

// Test_check_reports_an_unreadable_named_feature_directory is the
// named-feature counterpart: naming the same unreadable feature directly
// must report the same Finding, not collapse it into ErrNoSuchFeature —
// the directory plainly exists, it just could not be opened.
func Test_check_reports_an_unreadable_named_feature_directory(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	srv := assemble.NewServer(cfg, root)
	assemble.SetOpenRootForTest(srv, func(parent *os.Root, name string) (*os.Root, error) {
		if name == "demo" {
			return nil, &fs.PathError{Op: "openat", Path: name, Err: syscall.EACCES}
		}

		return parent.OpenRoot(name)
	})

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, featureDir, f.Path)
	assert.Contains(t, f.Detail, "permission denied")
	assert.Equal(t, assemble.SeverityError, f.Severity)
}

// Test_check_reports_a_feature_whose_step_files_cannot_be_listed is the
// same C3 defect one layer deeper: the feature directory opens fine but
// its step files cannot be listed (the branch a stricter-than-darwin
// permission model, such as Linux's, takes at a directory readable to
// enter but not to list). checkStepFindings used to swallow this failure
// silently (return nil, true) rather than report it.
func Test_check_reports_a_feature_whose_step_files_cannot_be_listed(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))

	srv := assemble.NewServer(cfg, root)
	assemble.SetReadDirForTest(srv, func(*os.Root) ([]os.DirEntry, error) {
		return nil, &fs.PathError{Op: "readdirent", Path: ".", Err: syscall.EACCES}
	})

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, featureDir, f.Path)
	assert.Contains(t, f.Detail, "permission denied")
	assert.Equal(t, assemble.SeverityError, f.Severity)
}

// Test_check_refuses_a_feature_argument_containing_a_path_separator pins
// the MINOR fix alongside MAJOR 2/3: "." and ".." would otherwise reopen
// the feature-directory root itself (or its parent) as if it were a
// feature, and a multi-component argument would reach a nested directory
// no "brief new" or "brief finish" call ever named — none of those are a
// feature this configuration knows about.
func Test_check_refuses_a_feature_argument_containing_a_path_separator(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	// "a/b" is deliberately absent: os.Root.Lstat rejects it as ErrNotExist
	// against this empty fixture whether or not the separator guard ran, so
	// it would not discriminate the guard from its absence. Each entry below
	// does: "." reopens the feature root itself and would succeed without the
	// guard, and the two escaping forms resolve to a path-escape error
	// distinct from ErrNotExist.
	for _, feature := range []string{".", "..", "demo/../.."} {
		findings, err := srv.Check(t.Context(), feature)
		require.ErrorIsf(t, err, assemble.ErrNoSuchFeature, "feature %q", feature)
		assert.Nilf(t, findings, "feature %q", feature)
	}
}

// Test_check_is_reachable_for_a_feature_name_containing_a_backslash pins
// validFeatureArgument to POSIX path-separator rules: a backslash is a
// legal filename character on POSIX, not a separator, so a feature
// genuinely named with one must remain checkable rather than being
// rejected as though it were a multi-component argument. os.IsPathSeparator
// is what makes this platform-correct rather than hardcoding the POSIX
// answer, so no build guard is needed here.
func Test_check_is_reachable_for_a_feature_name_containing_a_backslash(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, `foo\bar`)
	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), `foo\bar`)

	require.NoError(t, err)
	assert.Empty(t, findings)
}

// Test_check_marks_a_symlinked_feature_directory_rather_than_following_it
// pins Check's symlink stance to Status's: mark, never follow. The target
// is a conforming feature, proving the Finding fires because brief never
// follows the link, not because anything about the target is malformed.
func Test_check_marks_a_symlinked_feature_directory_rather_than_following_it(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()

	realDir := filepath.Join(root, "real-gamma")
	checkWriteFeature(t, cfg, realDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, realDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))

	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	linkPath := filepath.Join(root, cfg.FeatureDirectory, "gamma")
	require.NoError(t, os.Symlink(realDir, linkPath))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, linkPath, f.Path)
	assert.Contains(t, f.Detail, "symbolic link")
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

// Test_check_marks_a_named_symlinked_feature_directory_rather_than_following_it
// is the named-feature half of the same stance: naming the symlink
// directly must not follow it either.
func Test_check_marks_a_named_symlinked_feature_directory_rather_than_following_it(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%02d.md"
	root := t.TempDir()

	realDir := filepath.Join(root, "real-gamma")
	checkWriteFeature(t, cfg, realDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, realDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))

	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	linkPath := filepath.Join(root, cfg.FeatureDirectory, "gamma")
	require.NoError(t, os.Symlink(realDir, linkPath))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "gamma")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, linkPath, f.Path)
	assert.Contains(t, f.Detail, "symbolic link")
	assert.Equal(t, assemble.SeverityError, f.Severity)
}
