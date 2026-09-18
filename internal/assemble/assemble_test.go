package assemble_test

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureConfig returns a Config whose every field this package reads
// differs from config.Default(), mirroring scaffold_test.go's fixture, plus
// the acceptance heading this scenario adds.
func fixtureConfig() config.Config {
	cfg := config.Default()
	cfg.FeatureDirectory = "specs"
	cfg.SpecificationFile = "SPEC.md"
	cfg.StateFile = "NOTES.md"
	cfg.StepFilePattern = "STEP-%02d.md"
	cfg.ProgressHeading = "## Progress"
	cfg.ChecklistHeading = "## Fixture Checklist"
	cfg.HandoffHeading = "## Fixture Handoff"
	cfg.AcceptanceHeading = "## Fixture Scenario"
	cfg.StateHeadings = config.StateHeadings{
		BindingDecisions: "## Decisions Fixture",
		LeftUnbuilt:      "## Left Fixture",
		Traps:            "## Gotchas",
		OpenDebts:        "## Debts Fixture",
	}

	return cfg
}

// fixtureStep renders a step file's body byte-for-byte as
// scaffold.stepSkeleton emits its frontmatter, with an acceptance section
// (fenced gherkin carrying a "#" comment line and acceptanceMarker), a
// checklist section listing checklistItems, and a handoff section holding
// handoffBody.
func fixtureStep(cfg config.Config, id, status, title, acceptanceMarker string, checklistItems []string, handoffBody string) string {
	var sb strings.Builder

	sb.WriteString("---\n")
	sb.WriteString("id: " + id + "\n")
	sb.WriteString("status: " + status + "\n")
	sb.WriteString("depends-on: []\n")
	sb.WriteString("---\n\n")
	sb.WriteString("# " + title + "\n\n")
	sb.WriteString(cfg.AcceptanceHeading + "\n\n")
	sb.WriteString("```gherkin\n")
	sb.WriteString("Scenario: demo\n")
	sb.WriteString("  # " + acceptanceMarker + " comment\n")
	sb.WriteString("  Given a thing\n")
	sb.WriteString("```\n\n")
	sb.WriteString(cfg.ChecklistHeading + "\n\n")

	for _, item := range checklistItems {
		sb.WriteString("- [ ] " + item + "\n")
	}

	sb.WriteString("\n" + cfg.HandoffHeading + "\n\n")

	if handoffBody != "" {
		sb.WriteString(handoffBody + "\n")
	}

	return sb.String()
}

// newFixture writes feature "demo" under a fresh temp root, using
// fixtureConfig(): five step files (STEP-01, STEP-02 done; STEP-03,
// STEP-04, STEP-05 open), a NOTES.md carrying the four fixture state
// headings each with a marked body, and a SPEC.md whose progress list
// carries all five entries. It returns the root and the config the
// fixture was written with.
func newFixture(t *testing.T) (string, config.Config) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"),
		[]byte(fixtureStep(cfg, "STEP-01", "done", "STEP-01", "ACCEPTANCE-01",
			[]string{"did the first thing"}, "HANDOFF-ONLY-01")), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-02.md"),
		[]byte(fixtureStep(cfg, "STEP-02", "done", "STEP-02", "ACCEPTANCE-02",
			[]string{"did the second thing"}, "HANDOFF-ONLY-02")), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-03.md"),
		[]byte(fixtureStep(cfg, "STEP-03", "open", "STEP-03 Assemble the brief", "ACCEPTANCE-03",
			[]string{"CHECKLIST-03-A", "CHECKLIST-03-B"}, "")), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-04.md"),
		[]byte(fixtureStep(cfg, "STEP-04", "open", "STEP-04", "ACCEPTANCE-04",
			[]string{"pending"}, "")), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-05.md"),
		[]byte(fixtureStep(cfg, "STEP-05", "open", "STEP-05", "ACCEPTANCE-05",
			[]string{"pending"}, "")), 0o600))

	notes := cfg.StateHeadings.BindingDecisions + "\n\n" +
		"STATE-DECISION-A (STEP-01)\n" +
		"STATE-DECISION-B (STEP-02)\n\n" +
		cfg.StateHeadings.LeftUnbuilt + "\n\n" +
		"STATE-UNBUILT-A\n\n" +
		cfg.StateHeadings.Traps + "\n\n" +
		"STATE-TRAP-A\n\n" +
		cfg.StateHeadings.OpenDebts + "\n\n" +
		"STATE-DEBT-A\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(notes), 0o600))

	spec := "# demo\n\n" + cfg.ProgressHeading + "\n\n" +
		"- [x] STEP-01\n- [x] STEP-02\n- [ ] STEP-03\n- [ ] STEP-04\n- [ ] STEP-05\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	return root, cfg
}

// allSectionText concatenates every section body a rendered brief would
// expose: the step's acceptance and checklist sections, and every
// inherited state section. Tests use it to make a presence or absence
// claim against the whole brief rather than one section at a time.
func allSectionText(b assemble.Brief) string {
	var sb strings.Builder

	sb.WriteString(b.Step.Acceptance.Body)
	sb.WriteString(b.Step.Checklist.Body)

	for _, section := range b.Inherited {
		sb.WriteString(section.Body)
	}

	return sb.String()
}

func Test_returns_the_lowest_numbered_open_step_s_id_and_title(t *testing.T) {
	root, cfg := newFixture(t)
	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.NoError(t, err)
	require.NotNil(t, brief.Step)
	require.Equal(t, "STEP-03", brief.Step.ID)
	require.Equal(t, "STEP-03 Assemble the brief", brief.Step.Title)
}

func Test_returns_the_next_step_s_acceptance_criteria_and_checklist(t *testing.T) {
	root, cfg := newFixture(t)
	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.NoError(t, err)
	require.NotNil(t, brief.Step)
	assert.Equal(t, cfg.AcceptanceHeading, brief.Step.Acceptance.Heading)
	assert.Contains(t, brief.Step.Acceptance.Body, "ACCEPTANCE-03")
	assert.Contains(t, brief.Step.Acceptance.Body, "# ACCEPTANCE-03 comment")
	assert.Equal(t, cfg.ChecklistHeading, brief.Step.Checklist.Heading)
	assert.Contains(t, brief.Step.Checklist.Body, "CHECKLIST-03-A")
	assert.Contains(t, brief.Step.Checklist.Body, "CHECKLIST-03-B")
}

func Test_carries_every_state_file_section_as_inherited_context(t *testing.T) {
	root, cfg := newFixture(t)
	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.NoError(t, err)
	require.Len(t, brief.Inherited, 4)

	headings := make([]string, 0, len(brief.Inherited))

	var bodies strings.Builder
	for _, section := range brief.Inherited {
		headings = append(headings, section.Heading)
		bodies.WriteString(section.Body)
		bodies.WriteString("\n")
	}

	assert.Equal(t, []string{
		cfg.StateHeadings.BindingDecisions,
		cfg.StateHeadings.LeftUnbuilt,
		cfg.StateHeadings.Traps,
		cfg.StateHeadings.OpenDebts,
	}, headings)
	assert.Contains(t, bodies.String(), "STATE-DECISION-A (STEP-01)")
	assert.Contains(t, bodies.String(), "STATE-DECISION-B (STEP-02)")
	assert.Contains(t, bodies.String(), "STATE-UNBUILT-A")
	assert.Contains(t, bodies.String(), "STATE-TRAP-A")
	assert.Contains(t, bodies.String(), "STATE-DEBT-A")
}

func Test_reports_two_done_and_three_open(t *testing.T) {
	root, cfg := newFixture(t)
	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.NoError(t, err)
	assert.Equal(t, 2, brief.Done)
	assert.Equal(t, 3, brief.Open)
}

func Test_reads_inherited_context_from_the_state_file_not_from_the_step_handoffs(t *testing.T) {
	root, cfg := newFixture(t)
	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.NoError(t, err)

	whole := allSectionText(brief)

	assert.NotContains(t, whole, "HANDOFF-ONLY-01")
	assert.NotContains(t, whole, "HANDOFF-ONLY-02")
}

func Test_carries_no_other_step_s_acceptance_criteria(t *testing.T) {
	root, cfg := newFixture(t)
	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.NoError(t, err)
	require.NotNil(t, brief.Step)

	whole := allSectionText(brief)

	assert.Contains(t, whole, "ACCEPTANCE-03")
	assert.NotContains(t, whole, "ACCEPTANCE-01")
	assert.NotContains(t, whole, "ACCEPTANCE-02")
	assert.NotContains(t, whole, "ACCEPTANCE-04")
	assert.NotContains(t, whole, "ACCEPTANCE-05")
}

// Test_every_step_file_in_the_fixture_carries_acceptance_criteria_the_same_probe_reads
// is the control arm for the absence claim above: it proves the same
// markdown.Section probe Start uses would in fact see every other step's
// acceptance marker and handoff marker if nothing filtered them out, so
// their absence from the previous test's brief is a real filter and not a
// fixture that never had the marker to begin with.
func Test_every_step_file_in_the_fixture_carries_acceptance_criteria_the_same_probe_reads(t *testing.T) {
	root, cfg := newFixture(t)
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")

	for n, marker := range map[string]string{
		"STEP-01.md": "ACCEPTANCE-01",
		"STEP-02.md": "ACCEPTANCE-02",
		"STEP-03.md": "ACCEPTANCE-03",
		"STEP-04.md": "ACCEPTANCE-04",
		"STEP-05.md": "ACCEPTANCE-05",
	} {
		body, err := os.ReadFile(filepath.Join(featureDir, n))
		require.NoError(t, err)

		got, ok := markdown.Section(string(body), cfg.AcceptanceHeading)
		require.True(t, ok, "%s: acceptance heading not found by the probe", n)
		assert.Contains(t, got, marker, "%s: probe did not read its own marker", n)
	}

	for n, marker := range map[string]string{
		"STEP-01.md": "HANDOFF-ONLY-01",
		"STEP-02.md": "HANDOFF-ONLY-02",
	} {
		body, err := os.ReadFile(filepath.Join(featureDir, n))
		require.NoError(t, err)

		got, ok := markdown.Section(string(body), cfg.HandoffHeading)
		require.True(t, ok, "%s: handoff heading not found by the probe", n)
		assert.Contains(t, got, marker, "%s: probe did not read its own marker", n)
	}
}

func Test_takes_the_lowest_numbered_open_step_not_the_first_in_directory_order(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%d.md"
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-2.md"),
		[]byte(fixtureStep(cfg, "STEP-2", "done", "STEP-2", "ACCEPTANCE-2", nil, "")), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-9.md"),
		[]byte(fixtureStep(cfg, "STEP-9", "open", "STEP-9", "ACCEPTANCE-9", nil, "")), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-10.md"),
		[]byte(fixtureStep(cfg, "STEP-10", "open", "STEP-10", "ACCEPTANCE-10", nil, "")), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.NoError(t, err)
	require.NotNil(t, brief.Step)
	assert.Equal(t, "STEP-9", brief.Step.ID)
}

func Test_returns_no_next_step_when_every_step_is_done(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"),
		[]byte(fixtureStep(cfg, "STEP-01", "done", "STEP-01", "ACCEPTANCE-01", nil, "")), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-02.md"),
		[]byte(fixtureStep(cfg, "STEP-02", "done", "STEP-02", "ACCEPTANCE-02", nil, "")), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.NoError(t, err)
	assert.Nil(t, brief.Step)
	assert.Equal(t, 2, brief.Done)
	assert.Equal(t, 0, brief.Open)
}

func Test_returns_an_error_when_the_feature_does_not_exist(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "demo")

	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
}

func Test_returns_an_error_when_the_state_file_is_missing(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"),
		[]byte(fixtureStep(cfg, "STEP-01", "open", "STEP-01", "ACCEPTANCE-01", nil, "")), 0o600))

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.ErrorIs(t, err, assemble.ErrMalformedFeature)
	assert.Nil(t, brief.Step)
	assert.Zero(t, brief.Done)
	assert.Empty(t, brief.Inherited)
}

func Test_returns_an_error_when_a_step_file_has_no_frontmatter(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"), []byte("# STEP-01\n\nno frontmatter here\n"), 0o600))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "demo")

	require.ErrorIs(t, err, stepfile.ErrNoFrontmatter)
}

func Test_returns_an_error_when_the_feature_name_escapes_the_feature_root(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "../escaped")

	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
}

// fileSnapshot is one file's identity for a disk-unchanged sweep: its
// content hash, its permission bits and its modification time. mtime is
// included because a write that reproduces identical bytes still touches
// mtime, and a sweep that only hashed content would miss that.
type fileSnapshot struct {
	sha256 [32]byte
	mode   os.FileMode
	mtime  time.Time
}

// snapshotTree walks every regular file under root and returns its
// fileSnapshot, keyed by its path relative to root. Reads go through an
// *os.Root scoped to root rather than an absolute path built from the
// walk callback, so a symlink swapped in mid-walk cannot redirect a read
// outside root.
func snapshotTree(t *testing.T, root string) map[string]fileSnapshot {
	t.Helper()

	r, err := os.OpenRoot(root)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	snap := map[string]fileSnapshot{}

	walkErr := fs.WalkDir(r.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err)

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		require.NoError(t, err)

		data, err := r.ReadFile(path)
		require.NoError(t, err)

		snap[path] = fileSnapshot{sha256: sha256.Sum256(data), mode: info.Mode(), mtime: info.ModTime()}

		return nil
	})
	require.NoError(t, walkErr)

	return snap
}

func Test_writes_nothing_to_disk(t *testing.T) {
	root, cfg := newFixture(t)
	srv := assemble.NewServer(cfg, root)

	before := snapshotTree(t, root)

	_, err := srv.Start(t.Context(), "demo")

	require.NoError(t, err)
	assert.Equal(t, before, snapshotTree(t, root))
}

// Test_the_disk_sweep_sees_a_write is the control arm for the test above:
// it proves snapshotTree actually detects a change, so the previous
// test's equal snapshots are evidence Start wrote nothing rather than
// evidence the sweep cannot see a write at all.
func Test_the_disk_sweep_sees_a_write(t *testing.T) {
	root, _ := newFixture(t)

	before := snapshotTree(t, root)

	require.NoError(t, os.WriteFile(filepath.Join(root, "intruder.txt"), []byte("uninvited"), 0o600))

	assert.NotEqual(t, before, snapshotTree(t, root))
}
