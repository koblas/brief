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
	cfg.AcceptanceHeading = "## Fixture Scenario"
	cfg.HandoffFileSuffix = ".fixture-handoff.md"
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
// (fenced gherkin carrying a "#" comment line and acceptanceMarker) and a
// checklist section listing checklistItems. It carries no handoff section:
// the handoff moved to its own file, written separately by newFixture.
func fixtureStep(cfg config.Config, id, status, title, acceptanceMarker string, checklistItems []string) string {
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

	return sb.String()
}

// handoffFilePath returns the path of feature's step id's handoff file
// under featureDir, using cfg's configured handoff-file suffix, deriving
// the name by literal string concatenation rather than through
// stepfile.CompileHandoff, so a production naming bug cannot hide behind
// the fixture's own derivation.
func handoffFilePath(cfg config.Config, featureDir, id string) string {
	return filepath.Join(featureDir, id+cfg.HandoffFileSuffix)
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
			[]string{"did the first thing"})), 0o600))
	require.NoError(t, os.WriteFile(handoffFilePath(cfg, featureDir, "STEP-01"), []byte("HANDOFF-ONLY-01\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-02.md"),
		[]byte(fixtureStep(cfg, "STEP-02", "done", "STEP-02", "ACCEPTANCE-02",
			[]string{"did the second thing"})), 0o600))
	require.NoError(t, os.WriteFile(handoffFilePath(cfg, featureDir, "STEP-02"), []byte("HANDOFF-ONLY-02\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-03.md"),
		[]byte(fixtureStep(cfg, "STEP-03", "open", "STEP-03 Assemble the brief", "ACCEPTANCE-03",
			[]string{"CHECKLIST-03-A", "CHECKLIST-03-B"})), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-04.md"),
		[]byte(fixtureStep(cfg, "STEP-04", "open", "STEP-04", "ACCEPTANCE-04",
			[]string{"pending"})), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-05.md"),
		[]byte(fixtureStep(cfg, "STEP-05", "open", "STEP-05", "ACCEPTANCE-05",
			[]string{"pending"})), 0o600))

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
	assert.True(t, brief.Step.Acceptance.Found)
	assert.Equal(t, cfg.ChecklistHeading, brief.Step.Checklist.Heading)
	assert.Contains(t, brief.Step.Checklist.Body, "CHECKLIST-03-A")
	assert.Contains(t, brief.Step.Checklist.Body, "CHECKLIST-03-B")
	assert.True(t, brief.Step.Checklist.Found)
}

// Test_a_section_distinguishes_present_but_empty_from_not_found_at_all pins
// that stepFromEntry and stateSections must not discard markdown.Section's
// ok: a state file missing a configured heading entirely must not render
// the same empty Body as a heading present with nothing under it.
// Section.Found carries that distinction, which the CRLF fix depends on
// being observable rather than silently collapsed.
func Test_a_section_distinguishes_present_but_empty_from_not_found_at_all(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"),
		[]byte(fixtureStep(cfg, "STEP-01", "open", "STEP-01", "ACCEPTANCE-01", nil)), 0o600))

	notes := cfg.StateHeadings.BindingDecisions + "\n\n" +
		cfg.StateHeadings.LeftUnbuilt + "\n\n" +
		cfg.StateHeadings.Traps + "\n\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(notes), 0o600))

	spec := "# demo\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.NoError(t, err)
	require.Len(t, brief.Inherited, 4)

	// BindingDecisions' heading is present with nothing under it.
	assert.True(t, brief.Inherited[0].Found)
	assert.Empty(t, brief.Inherited[0].Body)

	// OpenDebts' heading is absent from notes entirely.
	assert.False(t, brief.Inherited[3].Found)
	assert.Empty(t, brief.Inherited[3].Body)
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

// Test_reports_two_done_and_three_open also proves the naming discipline
// from the read side: newFixture writes a handoff file for both done
// steps, so a count of 2 done and 3 open already rules out a handoff file
// being folded in as a sixth step file.
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

// Test_each_finished_step_s_handoff_file_carries_its_marker is the control
// arm for the absence claim above: it reads each handoff file directly
// from disk, so the previous test's absence cannot pass merely because
// the fixture never wrote the marker anywhere at all. With the handoff
// moved out of the step file, Start no longer reads handoff files by any
// path, so this control no longer proves a filter inside Start discards
// them — only that the fixture is not vacuous. See STATE.md.
func Test_each_finished_step_s_handoff_file_carries_its_marker(t *testing.T) {
	root, cfg := newFixture(t)
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")

	for id, marker := range map[string]string{
		"STEP-01": "HANDOFF-ONLY-01",
		"STEP-02": "HANDOFF-ONLY-02",
	} {
		got, err := os.ReadFile(handoffFilePath(cfg, featureDir, id))
		require.NoError(t, err, "%s: handoff file not found", id)
		assert.Contains(t, string(got), marker, "%s: handoff file did not carry its own marker", id)
	}
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
// is the control arm for the acceptance absence claim above: it proves the
// same probe Start uses — ParseFrontmatter to strip the YAML, then
// markdown.Section on what is left — would in fact see every other step's
// acceptance marker if nothing filtered them out, so its absence from the
// previous test's brief is a real filter and not a fixture that never had
// the marker to begin with. The probe parses frontmatter first because
// stepFromEntry (assemble.go) runs markdown.Section on e.rest, never on a
// step file's raw bytes; running it on the raw bytes here would let a "#"
// inside the YAML front matter — or the front matter's own line shape —
// desync this control arm from what production actually reads.
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

		_, rest, err := stepfile.ParseFrontmatter(body)
		require.NoError(t, err)

		got, ok := markdown.Section(string(rest), cfg.AcceptanceHeading)
		require.True(t, ok, "%s: acceptance heading not found by the probe", n)
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
		[]byte(fixtureStep(cfg, "STEP-2", "done", "STEP-2", "ACCEPTANCE-2", nil)), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-9.md"),
		[]byte(fixtureStep(cfg, "STEP-9", "open", "STEP-9", "ACCEPTANCE-9", nil)), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-10.md"),
		[]byte(fixtureStep(cfg, "STEP-10", "open", "STEP-10", "ACCEPTANCE-10", nil)), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))
	spec := "# demo\n\n" + cfg.ProgressHeading + "\n\n- [x] STEP-2\n- [ ] STEP-9\n- [ ] STEP-10\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

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
		[]byte(fixtureStep(cfg, "STEP-01", "done", "STEP-01", "ACCEPTANCE-01", nil)), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-02.md"),
		[]byte(fixtureStep(cfg, "STEP-02", "done", "STEP-02", "ACCEPTANCE-02", nil)), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))
	spec := "# demo\n\n" + cfg.ProgressHeading + "\n\n- [x] STEP-01\n- [x] STEP-02\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

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

// Test_refuses_a_specification_that_does_not_exist is SCENARIO-13's headline
// claim: before this scenario, an absent specification.md made Start
// assemble and return a full brief at exit 0, silently omitting the
// progress context every other check in this file already proves is read.
// The fixture is otherwise newFixture's conforming shape minus the
// specification file, so this is a single-variable change from
// Test_returns_the_lowest_numbered_open_step_s_id_and_title's control arm.
func Test_refuses_a_specification_that_does_not_exist(t *testing.T) {
	root, cfg := newFixture(t)
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.Remove(filepath.Join(featureDir, cfg.SpecificationFile)))

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.ErrorIs(t, err, assemble.ErrMalformedFeature)

	var refusal *assemble.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, filepath.Join(featureDir, cfg.SpecificationFile), refusal.Path)
	assert.Zero(t, refusal.Line)
	assert.NotContains(t, refusal.Fix, "brief new feature")
	assert.Nil(t, brief.Step)
	assert.Zero(t, brief.Done)
	assert.Empty(t, brief.Inherited)
}

// Test_refuses_a_specification_with_no_progress_heading is the second
// structural specification check: present, readable and fence-closed, but
// missing the configured progress heading entirely — the shape a hand-edited
// specification.md could produce. It differs from newFixture's control arm
// in exactly one variable, the specification body.
func Test_refuses_a_specification_with_no_progress_heading(t *testing.T) {
	root, cfg := newFixture(t)
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	spec := "# demo\n\nno progress list here\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.ErrorIs(t, err, assemble.ErrMalformedFeature)

	var refusal *assemble.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, filepath.Join(featureDir, cfg.SpecificationFile), refusal.Path)
	assert.Zero(t, refusal.Line)
	assert.Contains(t, refusal.Detail, cfg.ProgressHeading)
	assert.Contains(t, refusal.Fix, cfg.ProgressHeading)
	assert.Nil(t, brief.Step)
	assert.Zero(t, brief.Done)
	assert.Empty(t, brief.Inherited)
}

// Test_refuses_a_specification_whose_fence_is_unterminated mirrors
// Test_refuses_a_state_file_whose_fence_is_unterminated: an open fence
// before the progress heading makes that heading unreadable, indistinguishable
// from it never having been written at all, so Start refuses rather than
// silently reporting the heading absent.
func Test_refuses_a_specification_whose_fence_is_unterminated(t *testing.T) {
	root, cfg := newFixture(t)
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	spec := "```\nunterminated\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.ErrorIs(t, err, assemble.ErrMalformedFeature)

	var refusal *assemble.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, filepath.Join(featureDir, cfg.SpecificationFile), refusal.Path)
	assert.Equal(t, 1, refusal.Line)
	assert.Contains(t, refusal.Detail, "unclosed")
	assert.Nil(t, brief.Step)
	assert.Zero(t, brief.Done)
	assert.Empty(t, brief.Inherited)
}

func Test_returns_an_error_when_the_state_file_is_missing(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"),
		[]byte(fixtureStep(cfg, "STEP-01", "open", "STEP-01", "ACCEPTANCE-01", nil)), 0o600))
	spec := "# demo\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.ErrorIs(t, err, assemble.ErrMalformedFeature)
	assert.Nil(t, brief.Step)
	assert.Zero(t, brief.Done)
	assert.Empty(t, brief.Inherited)
}

// Test_refuses_a_state_file_whose_fence_is_unterminated pins the rule that
// makes an unclosed fence a refusal rather than a silent omission:
// stateSections finds each configured heading by scanning forward for a
// terminator, the same as every other
// caller of markdown.Section, so a fence opened before the first heading
// and never closed puts every one of them inside it — the control arm
// (Test_carries_every_state_file_section_as_inherited_context) already
// proves the same four headings are readable from a balanced state file,
// so this is a single-variable change from that fixture, not a
// freestanding claim. Without this check Start would return a Brief with
// every Inherited section empty at exit 0 — R10's "the worst this tool
// could produce" — rather than refuse.
func Test_refuses_a_state_file_whose_fence_is_unterminated(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"),
		[]byte(fixtureStep(cfg, "STEP-01", "open", "STEP-01", "ACCEPTANCE-01", nil)), 0o600))
	spec := "# demo\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	state := "```\nunterminated\n" + cfg.StateHeadings.BindingDecisions + "\n\nsome decision\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.Error(t, err)
	require.ErrorIs(t, err, assemble.ErrMalformedFeature)
	assert.Contains(t, err.Error(), "unclosed")
	assert.Nil(t, brief.Step)
	assert.Zero(t, brief.Done)
	assert.Empty(t, brief.Inherited)
}

func Test_returns_an_error_when_a_step_file_has_no_frontmatter(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	spec := "# demo\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"), []byte("# STEP-01\n\nno frontmatter here\n"), 0o600))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "demo")

	require.ErrorIs(t, err, stepfile.ErrNoFrontmatter)
}

// Test_start_names_the_step_file_whose_frontmatter_cannot_be_read is
// SCENARIO-04's core claim: every shape of a step-file frontmatter parse
// failure names that step file, absolute, and points at 'brief check' —
// never the feature directory, and never scaffolding a step that already
// exists. The "second step bad" row proves the name is not hard-coded to
// the first file readSteps visits: STEP-01 is well-formed there and
// STEP-02 is the one named.
func Test_start_names_the_step_file_whose_frontmatter_cannot_be_read(t *testing.T) {
	cfg := fixtureConfig()

	cases := []struct {
		name       string
		files      map[string]string
		wantStep   string
		wantDetail string
		noFm       bool
	}{
		{
			name:       "no frontmatter",
			files:      map[string]string{"STEP-01.md": "no frontmatter here\n"},
			wantStep:   "STEP-01.md",
			wantDetail: "no frontmatter found",
			noFm:       true,
		},
		{
			name:       "unclosed delimiter",
			files:      map[string]string{"STEP-01.md": "---\nid: STEP-01\n"},
			wantStep:   "STEP-01.md",
			wantDetail: "no frontmatter found: no closing frontmatter delimiter",
			noFm:       true,
		},
		{
			name:       "bad YAML",
			files:      map[string]string{"STEP-01.md": "---\nid: [open\n---\n\n# STEP-01\n"},
			wantStep:   "STEP-01.md",
			wantDetail: "parse frontmatter",
		},
		{
			name: "second step file is the bad one",
			files: map[string]string{
				"STEP-01.md": fixtureStepWithDeps(cfg, "STEP-01", "open", "STEP-01", nil),
				"STEP-02.md": "no frontmatter here either\n",
			},
			wantStep:   "STEP-02.md",
			wantDetail: "no frontmatter found",
			noFm:       true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
			require.NoError(t, os.MkdirAll(featureDir, 0o755))
			spec := "# demo\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n"
			require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))

			for name, body := range tc.files {
				require.NoError(t, os.WriteFile(filepath.Join(featureDir, name), []byte(body), 0o600))
			}

			srv := assemble.NewServer(cfg, root)

			_, err := srv.Start(t.Context(), "demo")

			require.Error(t, err)

			if tc.noFm {
				require.ErrorIs(t, err, stepfile.ErrNoFrontmatter)
			} else {
				require.NotErrorIs(t, err, stepfile.ErrNoFrontmatter)
			}

			var refusal *assemble.RefusalError
			require.ErrorAs(t, err, &refusal)
			assert.Equal(t, filepath.Join(featureDir, tc.wantStep), refusal.Path)
			assert.Contains(t, refusal.Detail, tc.wantDetail)
			assert.Equal(t, "run 'brief check demo' to list every fault", refusal.Fix)
		})
	}
}

// Test_start_still_refuses_a_step_file_whose_frontmatter_does_not_parse is
// SCENARIO-11's tripwire against readSteps becoming tolerant, sharpened by
// SCENARIO-13: this step file's frontmatter delimiters are present and
// well-formed — unlike the no-frontmatter fixture above — but the YAML they
// enclose is not, so this is a single-variable change from that fixture
// rather than a byte-for-byte duplicate exercising the same absent-delimiter
// branch under a different name. Status's per-feature tolerance lives in
// assemble.featureStatus, not in readSteps itself, because readSteps is
// shared with Start (assemble.go and status.go are its only two callers).
// Moving the tolerance down would silently make Start tolerant too.
func Test_start_still_refuses_a_step_file_whose_frontmatter_does_not_parse(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	spec := "# demo\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(""), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-01.md"),
		[]byte("---\nid: STEP-01\nstatus: [open\n---\n\n# STEP-01\n"), 0o600))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "demo")

	require.Error(t, err)
	require.NotErrorIs(t, err, stepfile.ErrNoFrontmatter)

	var refusal *assemble.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, filepath.Join(featureDir, "STEP-01.md"), refusal.Path)
	assert.Contains(t, refusal.Detail, "yaml")
}

// Test_refuses_the_briefed_step_when_its_frontmatter_carries_no_id is check
// 7: presence only, and only on the step Start would brief. newFixture's
// STEP-03 is the step Start would pick (the lowest-numbered open one), so
// blanking only its id is a single-variable change from
// Test_returns_the_lowest_numbered_open_step_s_id_and_title's control arm —
// STEP-01, STEP-02, STEP-04 and STEP-05 all keep a valid id.
func Test_refuses_the_briefed_step_when_its_frontmatter_carries_no_id(t *testing.T) {
	root, cfg := newFixture(t)
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	step := "---\n" +
		"id:\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# STEP-03 Assemble the brief\n\n" +
		cfg.AcceptanceHeading + "\n\naccept\n\n" +
		cfg.ChecklistHeading + "\n\n- [ ] task\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-03.md"), []byte(step), 0o600))

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.ErrorIs(t, err, assemble.ErrMalformedFeature)

	var refusal *assemble.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, filepath.Join(featureDir, "STEP-03.md"), refusal.Path)
	assert.Zero(t, refusal.Line)
	assert.Nil(t, brief.Step)
}

// Test_refuses_the_briefed_step_when_its_checklist_heading_is_absent is
// check 8: the briefed step carries an acceptance section but no line
// matching cfg.ChecklistHeading. It is the checklist-side twin of the id
// test above, changing the same single step in newFixture's fixture.
func Test_refuses_the_briefed_step_when_its_checklist_heading_is_absent(t *testing.T) {
	root, cfg := newFixture(t)
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	step := "---\n" +
		"id: STEP-03\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# STEP-03 Assemble the brief\n\n" +
		cfg.AcceptanceHeading + "\n\naccept\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-03.md"), []byte(step), 0o600))

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.ErrorIs(t, err, assemble.ErrMalformedFeature)

	var refusal *assemble.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, filepath.Join(featureDir, "STEP-03.md"), refusal.Path)
	assert.Contains(t, refusal.Detail, cfg.ChecklistHeading)
	assert.Nil(t, brief.Step)
}

// Test_a_step_with_an_empty_but_present_checklist_stays_conforming is the
// control arm for the check above: SCENARIO-13's Handoff rules a
// present-but-empty checklist conforming, because "new step" writes an
// empty checklist heading and start must still work against it. It reuses
// newFixture's STEP-04 (open, not the briefed step) with STEP-03 marked
// done so STEP-04 becomes the one Start briefs.
func Test_a_step_with_an_empty_but_present_checklist_stays_conforming(t *testing.T) {
	root, cfg := newFixture(t)
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-03.md"),
		[]byte(fixtureStep(cfg, "STEP-03", "done", "STEP-03 Assemble the brief", "ACCEPTANCE-03", nil)), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-04.md"),
		[]byte(fixtureStep(cfg, "STEP-04", "open", "STEP-04", "ACCEPTANCE-04", nil)), 0o600))

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.NoError(t, err)
	require.NotNil(t, brief.Step)
	assert.Equal(t, "STEP-04", brief.Step.ID)
	assert.True(t, brief.Step.Checklist.Found)
	assert.Empty(t, brief.Step.Checklist.Body)
}

// Test_start_reports_an_absent_acceptance_heading_as_a_shortfall is
// SCENARIO-14's degrade case: STEP-03's acceptance heading is dropped,
// single-variable from Test_refuses_the_briefed_step_when_its_frontmatter_carries_no_id's
// fixture, but Start still returns a Brief rather than a *RefusalError.
func Test_start_reports_an_absent_acceptance_heading_as_a_shortfall(t *testing.T) {
	root, cfg := newFixture(t)
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "demo")
	step := "---\n" +
		"id: STEP-03\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# STEP-03 Assemble the brief\n\n" +
		cfg.ChecklistHeading + "\n\n- [ ] task\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-03.md"), []byte(step), 0o600))

	srv := assemble.NewServer(cfg, root)

	brief, err := srv.Start(t.Context(), "demo")

	require.NoError(t, err)
	require.NotNil(t, brief.Step)
	assert.False(t, brief.Step.Acceptance.Found)
	require.Len(t, brief.Shortfalls, 1)
	assert.Equal(t, filepath.Join(featureDir, "STEP-03.md"), brief.Shortfalls[0].Path)
	assert.Contains(t, brief.Shortfalls[0].Detail, cfg.AcceptanceHeading)
}

func Test_returns_an_error_when_the_feature_name_escapes_the_feature_root(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "../escaped")

	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
}

// Test_returns_an_error_when_the_feature_name_is_empty pins that an empty
// feature argument is refused the same way a traversal attempt is —
// validFeatureArgument rejects it before either os.Root.OpenRoot call —
// rather than reaching topRoot.OpenRoot("") and surfacing its own opaque
// "empty path" failure, which names no feature and suggests no fix.
func Test_returns_an_error_when_the_feature_name_is_empty(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "")

	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
}

// Test_start_reports_a_generic_failure_when_the_feature_root_itself_is_not_a_directory
// covers Start's first os.Root.OpenRoot call — the configured feature
// directory itself, not feature's own subdirectory — the same way
// Test_start_reports_a_generic_failure_for_an_unreadable_feature_entry
// already covers the second: a regular file standing where the configured
// feature directory belongs must fail generically, carrying that path in
// its message, never as ErrNoSuchFeature.
func Test_start_reports_a_generic_failure_when_the_feature_root_itself_is_not_a_directory(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDirPath := filepath.Join(root, cfg.FeatureDirectory)
	require.NoError(t, os.WriteFile(featureDirPath, []byte("not a directory"), 0o600))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "demo")

	require.Error(t, err)
	require.NotErrorIs(t, err, assemble.ErrNoSuchFeature)
	assert.Contains(t, err.Error(), featureDirPath)
}

// Test_start_reports_a_generic_failure_for_an_unreadable_feature_entry pins
// that only a genuinely absent directory is ErrNoSuchFeature: a feature
// entry that exists but cannot be opened as a
// directory — here, a regular file standing where "demo"'s directory
// belongs — must not read as "no such feature demo", since demo plainly
// does exist. The portable, privilege-independent substitute for a
// permission failure is the same technique
// Test_status_propagates_a_feature_root_that_is_not_a_directory already
// uses: a regular file makes os.Root.OpenRoot fail with "not a directory",
// never fs.ErrNotExist.
func Test_start_reports_a_generic_failure_for_an_unreadable_feature_entry(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, cfg.FeatureDirectory, "demo"), []byte("not a directory"), 0o600))

	srv := assemble.NewServer(cfg, root)

	_, err := srv.Start(t.Context(), "demo")

	require.Error(t, err)
	assert.NotErrorIs(t, err, assemble.ErrNoSuchFeature)
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
