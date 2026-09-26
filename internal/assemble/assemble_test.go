package assemble_test

import (
	"errors"
	"io/fs"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureConfig returns a Config whose every field this package reads
// differs from config.Default().
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

// fixtureStep renders a step file's body with an acceptance section
// (fenced gherkin carrying a "#" comment line and acceptanceMarker) and a
// checklist section listing checklistItems. The handoff is written
// separately by newFixtureFiles.
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

// newFixtureFiles returns fixtureConfig()'s own five step files (STEP-01,
// STEP-02 done; STEP-03, STEP-04, STEP-05 open), a NOTES.md and a SPEC.md,
// keyed by name relative to the feature directory so a test can delete or
// overwrite one entry before wrapping the result with featureFS.
func newFixtureFiles(cfg config.Config) map[string]string {
	files := map[string]string{
		"STEP-01.md": fixtureStep(cfg, "STEP-01", "done", "STEP-01", "ACCEPTANCE-01",
			[]string{"did the first thing"}),
		"STEP-01" + cfg.HandoffFileSuffix: "HANDOFF-ONLY-01\n",
		"STEP-02.md": fixtureStep(cfg, "STEP-02", "done", "STEP-02", "ACCEPTANCE-02",
			[]string{"did the second thing"}),
		"STEP-02" + cfg.HandoffFileSuffix: "HANDOFF-ONLY-02\n",
		"STEP-03.md": fixtureStep(cfg, "STEP-03", "open", "STEP-03 Assemble the brief", "ACCEPTANCE-03",
			[]string{"CHECKLIST-03-A", "CHECKLIST-03-B"}),
		"STEP-04.md": fixtureStep(cfg, "STEP-04", "open", "STEP-04", "ACCEPTANCE-04",
			[]string{"pending"}),
		"STEP-05.md": fixtureStep(cfg, "STEP-05", "open", "STEP-05", "ACCEPTANCE-05",
			[]string{"pending"}),
	}

	files[cfg.StateFile] = cfg.StateHeadings.BindingDecisions + "\n\n" +
		"STATE-DECISION-A (STEP-01)\n" +
		"STATE-DECISION-B (STEP-02)\n\n" +
		cfg.StateHeadings.LeftUnbuilt + "\n\n" +
		"STATE-UNBUILT-A\n\n" +
		cfg.StateHeadings.Traps + "\n\n" +
		"STATE-TRAP-A\n\n" +
		cfg.StateHeadings.OpenDebts + "\n\n" +
		"STATE-DEBT-A\n"

	files[cfg.SpecificationFile] = "# demo\n\n" + cfg.ProgressHeading + "\n\n" +
		"- [x] STEP-01\n- [x] STEP-02\n- [ ] STEP-03\n- [ ] STEP-04\n- [ ] STEP-05\n"

	return files
}

// newFixtureFS wraps newFixtureFiles as a FeatureFS.
func newFixtureFS(t *testing.T) (assemble.FeatureFS, config.Config) {
	t.Helper()

	cfg := fixtureConfig()

	return featureFS(newFixtureFiles(cfg)), cfg
}

// allSectionText concatenates every section body a rendered brief would
// expose, for a presence or absence claim against the whole brief.
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
	fsys, cfg := newFixtureFS(t)
	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(fsys)

	require.NoError(t, err)
	require.NotNil(t, brief.Step)
	require.Equal(t, "STEP-03", brief.Step.ID)
	require.Equal(t, "STEP-03 Assemble the brief", brief.Step.Title)
}

func Test_returns_the_next_step_s_acceptance_criteria_and_checklist(t *testing.T) {
	fsys, cfg := newFixtureFS(t)
	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(fsys)

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

// A heading missing entirely must not render the same empty Body as one
// present with nothing under it; Section.Found carries the distinction.
func Test_a_section_distinguishes_present_but_empty_from_not_found_at_all(t *testing.T) {
	cfg := fixtureConfig()

	notes := cfg.StateHeadings.BindingDecisions + "\n\n" +
		cfg.StateHeadings.LeftUnbuilt + "\n\n" +
		cfg.StateHeadings.Traps + "\n\n"
	spec := "# demo\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n"

	fsys := featureFS(map[string]string{
		"STEP-01.md":          fixtureStep(cfg, "STEP-01", "open", "STEP-01", "ACCEPTANCE-01", nil),
		cfg.StateFile:         notes,
		cfg.SpecificationFile: spec,
	})

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(fsys)

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
	fsys, cfg := newFixtureFS(t)
	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(fsys)

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

// A handoff file exists for both done steps, so this count also rules out
// one being folded in as a sixth step file.
func Test_reports_two_done_and_three_open(t *testing.T) {
	fsys, cfg := newFixtureFS(t)
	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(fsys)

	require.NoError(t, err)
	assert.Equal(t, 2, brief.Done)
	assert.Equal(t, 3, brief.Open)
}

func Test_reads_inherited_context_from_the_state_file_not_from_the_step_handoffs(t *testing.T) {
	fsys, cfg := newFixtureFS(t)
	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(fsys)

	require.NoError(t, err)

	whole := allSectionText(brief)

	assert.NotContains(t, whole, "HANDOFF-ONLY-01")
	assert.NotContains(t, whole, "HANDOFF-ONLY-02")
}

// Control arm for the absence claim above: proves the fixture is not
// vacuous by reading each handoff file's marker directly.
func Test_each_finished_step_s_handoff_file_carries_its_marker(t *testing.T) {
	fsys, cfg := newFixtureFS(t)

	for id, marker := range map[string]string{
		"STEP-01": "HANDOFF-ONLY-01",
		"STEP-02": "HANDOFF-ONLY-02",
	} {
		got, err := fs.ReadFile(fsys.FS, id+cfg.HandoffFileSuffix)
		require.NoError(t, err, "%s: handoff file not found", id)
		assert.Contains(t, string(got), marker, "%s: handoff file did not carry its own marker", id)
	}
}

func Test_carries_no_other_step_s_acceptance_criteria(t *testing.T) {
	fsys, cfg := newFixtureFS(t)
	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(fsys)

	require.NoError(t, err)
	require.NotNil(t, brief.Step)

	whole := allSectionText(brief)

	assert.Contains(t, whole, "ACCEPTANCE-03")
	assert.NotContains(t, whole, "ACCEPTANCE-01")
	assert.NotContains(t, whole, "ACCEPTANCE-02")
	assert.NotContains(t, whole, "ACCEPTANCE-04")
	assert.NotContains(t, whole, "ACCEPTANCE-05")
}

// Control arm for the acceptance absence claim above: proves the same
// probe Start uses would see every other step's marker unfiltered.
func Test_every_step_file_in_the_fixture_carries_acceptance_criteria_the_same_probe_reads(t *testing.T) {
	fsys, cfg := newFixtureFS(t)

	for n, marker := range map[string]string{
		"STEP-01.md": "ACCEPTANCE-01",
		"STEP-02.md": "ACCEPTANCE-02",
		"STEP-03.md": "ACCEPTANCE-03",
		"STEP-04.md": "ACCEPTANCE-04",
		"STEP-05.md": "ACCEPTANCE-05",
	} {
		body, err := fs.ReadFile(fsys.FS, n)
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

	fsys := featureFS(map[string]string{
		"STEP-2.md":           fixtureStep(cfg, "STEP-2", "done", "STEP-2", "ACCEPTANCE-2", nil),
		"STEP-9.md":           fixtureStep(cfg, "STEP-9", "open", "STEP-9", "ACCEPTANCE-9", nil),
		"STEP-10.md":          fixtureStep(cfg, "STEP-10", "open", "STEP-10", "ACCEPTANCE-10", nil),
		cfg.StateFile:         "",
		cfg.SpecificationFile: "# demo\n\n" + cfg.ProgressHeading + "\n\n- [x] STEP-2\n- [ ] STEP-9\n- [ ] STEP-10\n",
	})

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(fsys)

	require.NoError(t, err)
	require.NotNil(t, brief.Step)
	assert.Equal(t, "STEP-9", brief.Step.ID)
}

func Test_returns_no_next_step_when_every_step_is_done(t *testing.T) {
	cfg := fixtureConfig()

	fsys := featureFS(map[string]string{
		"STEP-01.md":          fixtureStep(cfg, "STEP-01", "done", "STEP-01", "ACCEPTANCE-01", nil),
		"STEP-02.md":          fixtureStep(cfg, "STEP-02", "done", "STEP-02", "ACCEPTANCE-02", nil),
		cfg.StateFile:         "",
		cfg.SpecificationFile: "# demo\n\n" + cfg.ProgressHeading + "\n\n- [x] STEP-01\n- [x] STEP-02\n",
	})

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(fsys)

	require.NoError(t, err)
	assert.Nil(t, brief.Step)
	assert.Equal(t, 2, brief.Done)
	assert.Equal(t, 0, brief.Open)
}

// Single-variable change from newFixtureFiles's conforming shape: same
// fixture, minus the specification file.
func Test_refuses_a_specification_that_does_not_exist(t *testing.T) {
	cfg := fixtureConfig()
	files := newFixtureFiles(cfg)
	delete(files, cfg.SpecificationFile)

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(featureFS(files))

	require.ErrorIs(t, err, assemble.ErrMalformedFeature)

	var refusal *assemble.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.SpecificationFile), refusal.Path)
	assert.Zero(t, refusal.Line)
	assert.NotContains(t, refusal.Fix, "brief new feature")
	assert.Nil(t, brief.Step)
	assert.Zero(t, brief.Done)
	assert.Empty(t, brief.Inherited)
}

// Present, readable and fence-closed, but missing the configured progress
// heading entirely — a single-variable change from newFixtureFiles.
func Test_refuses_a_specification_with_no_progress_heading(t *testing.T) {
	cfg := fixtureConfig()
	files := newFixtureFiles(cfg)
	files[cfg.SpecificationFile] = "# demo\n\nno progress list here\n"

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(featureFS(files))

	require.ErrorIs(t, err, assemble.ErrMalformedFeature)

	var refusal *assemble.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.SpecificationFile), refusal.Path)
	assert.Zero(t, refusal.Line)
	assert.Contains(t, refusal.Detail, cfg.ProgressHeading)
	assert.Contains(t, refusal.Fix, cfg.ProgressHeading)
	assert.Nil(t, brief.Step)
	assert.Zero(t, brief.Done)
	assert.Empty(t, brief.Inherited)
}

// An open fence before the progress heading makes it read as absent, so
// Start refuses rather than silently reporting that.
func Test_refuses_a_specification_whose_fence_is_unterminated(t *testing.T) {
	cfg := fixtureConfig()
	files := newFixtureFiles(cfg)
	files[cfg.SpecificationFile] = "```\nunterminated\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n"

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(featureFS(files))

	require.ErrorIs(t, err, assemble.ErrMalformedFeature)

	var refusal *assemble.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.SpecificationFile), refusal.Path)
	assert.Equal(t, 1, refusal.Line)
	assert.Contains(t, refusal.Detail, "unclosed")
	assert.Nil(t, brief.Step)
	assert.Zero(t, brief.Done)
	assert.Empty(t, brief.Inherited)
}

func Test_returns_an_error_when_the_state_file_is_missing(t *testing.T) {
	cfg := fixtureConfig()

	fsys := featureFS(map[string]string{
		"STEP-01.md":          fixtureStep(cfg, "STEP-01", "open", "STEP-01", "ACCEPTANCE-01", nil),
		cfg.SpecificationFile: "# demo\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n",
	})

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(fsys)

	require.ErrorIs(t, err, assemble.ErrMalformedFeature)
	assert.Nil(t, brief.Step)
	assert.Zero(t, brief.Done)
	assert.Empty(t, brief.Inherited)
}

// Single-variable change from the control arm proving the same four
// headings readable from a balanced state file.
func Test_refuses_a_state_file_whose_fence_is_unterminated(t *testing.T) {
	cfg := fixtureConfig()

	fsys := featureFS(map[string]string{
		"STEP-01.md":          fixtureStep(cfg, "STEP-01", "open", "STEP-01", "ACCEPTANCE-01", nil),
		cfg.SpecificationFile: "# demo\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n",
		cfg.StateFile:         "```\nunterminated\n" + cfg.StateHeadings.BindingDecisions + "\n\nsome decision\n",
	})

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(fsys)

	require.Error(t, err)
	require.ErrorIs(t, err, assemble.ErrMalformedFeature)
	assert.Contains(t, err.Error(), "unclosed")
	assert.Nil(t, brief.Step)
	assert.Zero(t, brief.Done)
	assert.Empty(t, brief.Inherited)
}

func Test_returns_an_error_when_a_step_file_has_no_frontmatter(t *testing.T) {
	cfg := fixtureConfig()

	fsys := featureFS(map[string]string{
		cfg.SpecificationFile: "# demo\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n",
		cfg.StateFile:         "",
		"STEP-01.md":          "# STEP-01\n\nno frontmatter here\n",
	})

	srv := assemble.NewServer(cfg, "")

	_, err := srv.StartFS(fsys)

	require.ErrorIs(t, err, stepfile.ErrNoFrontmatter)
}

// The "second step bad" row proves the named step is not hard-coded to
// the first file readSteps visits.
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
			files := map[string]string{
				cfg.SpecificationFile: "# demo\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n",
				cfg.StateFile:         "",
			}
			maps.Copy(files, tc.files)

			srv := assemble.NewServer(cfg, "")

			_, err := srv.StartFS(featureFS(files))

			require.Error(t, err)
			assert.Equal(t, tc.noFm, errors.Is(err, stepfile.ErrNoFrontmatter))

			var refusal *assemble.RefusalError
			require.ErrorAs(t, err, &refusal)
			assert.Equal(t, filepath.Join(testFeaturePath, tc.wantStep), refusal.Path)
			assert.Contains(t, refusal.Detail, tc.wantDetail)
			assert.Equal(t, "run 'brief check demo' to list every fault", refusal.Fix)
		})
	}
}

// The frontmatter delimiters are present and well-formed, unlike the
// no-frontmatter fixture above, but the YAML they enclose is not.
func Test_start_still_refuses_a_step_file_whose_frontmatter_does_not_parse(t *testing.T) {
	cfg := fixtureConfig()

	fsys := featureFS(map[string]string{
		cfg.SpecificationFile: "# demo\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-01\n",
		cfg.StateFile:         "",
		"STEP-01.md":          "---\nid: STEP-01\nstatus: [open\n---\n\n# STEP-01\n",
	})

	srv := assemble.NewServer(cfg, "")

	_, err := srv.StartFS(fsys)

	require.Error(t, err)
	require.NotErrorIs(t, err, stepfile.ErrNoFrontmatter)

	var refusal *assemble.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-01.md"), refusal.Path)
	assert.Contains(t, refusal.Detail, "yaml")
}

// Presence only, and only on the step Start would brief: blanking STEP-03's
// id is a single-variable change from the control arm above.
func Test_refuses_the_briefed_step_when_its_frontmatter_carries_no_id(t *testing.T) {
	cfg := fixtureConfig()
	files := newFixtureFiles(cfg)
	files["STEP-03.md"] = "---\n" +
		"id:\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# STEP-03 Assemble the brief\n\n" +
		cfg.AcceptanceHeading + "\n\naccept\n\n" +
		cfg.ChecklistHeading + "\n\n- [ ] task\n"

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(featureFS(files))

	require.ErrorIs(t, err, assemble.ErrMalformedFeature)

	var refusal *assemble.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-03.md"), refusal.Path)
	assert.Zero(t, refusal.Line)
	assert.Nil(t, brief.Step)
}

// Checklist-side twin of the id test above: an acceptance section but no
// line matching cfg.ChecklistHeading.
func Test_refuses_the_briefed_step_when_its_checklist_heading_is_absent(t *testing.T) {
	cfg := fixtureConfig()
	files := newFixtureFiles(cfg)
	files["STEP-03.md"] = "---\n" +
		"id: STEP-03\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# STEP-03 Assemble the brief\n\n" +
		cfg.AcceptanceHeading + "\n\naccept\n"

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(featureFS(files))

	require.ErrorIs(t, err, assemble.ErrMalformedFeature)

	var refusal *assemble.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-03.md"), refusal.Path)
	assert.Contains(t, refusal.Detail, cfg.ChecklistHeading)
	assert.Nil(t, brief.Step)
}

// Control arm for the check above: a present-but-empty checklist stays
// conforming, since "new step" writes an empty checklist heading.
func Test_a_step_with_an_empty_but_present_checklist_stays_conforming(t *testing.T) {
	cfg := fixtureConfig()
	files := newFixtureFiles(cfg)
	files["STEP-03.md"] = fixtureStep(cfg, "STEP-03", "done", "STEP-03 Assemble the brief", "ACCEPTANCE-03", nil)
	files["STEP-04.md"] = fixtureStep(cfg, "STEP-04", "open", "STEP-04", "ACCEPTANCE-04", nil)

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(featureFS(files))

	require.NoError(t, err)
	require.NotNil(t, brief.Step)
	assert.Equal(t, "STEP-04", brief.Step.ID)
	assert.True(t, brief.Step.Checklist.Found)
	assert.Empty(t, brief.Step.Checklist.Body)
}

// STEP-03's acceptance heading is dropped, but Start still returns a
// Brief rather than a *RefusalError.
func Test_start_reports_an_absent_acceptance_heading_as_a_shortfall(t *testing.T) {
	cfg := fixtureConfig()
	files := newFixtureFiles(cfg)
	files["STEP-03.md"] = "---\n" +
		"id: STEP-03\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# STEP-03 Assemble the brief\n\n" +
		cfg.ChecklistHeading + "\n\n- [ ] task\n"

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(featureFS(files))

	require.NoError(t, err)
	require.NotNil(t, brief.Step)
	assert.False(t, brief.Step.Acceptance.Found)
	require.Len(t, brief.Shortfalls, 1)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-03.md"), brief.Shortfalls[0].Path)
	assert.Contains(t, brief.Shortfalls[0].Detail, cfg.AcceptanceHeading)
}

func Test_start_reports_a_whitespace_only_acceptance_section_as_a_shortfall(t *testing.T) {
	cfg := fixtureConfig()
	files := newFixtureFiles(cfg)
	files["STEP-03.md"] = "---\n" +
		"id: STEP-03\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# STEP-03 Assemble the brief\n\n" +
		cfg.AcceptanceHeading + "\n\n  \n\t\n\n" +
		cfg.ChecklistHeading + "\n\n- [ ] task\n"

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(featureFS(files))

	require.NoError(t, err)
	require.NotNil(t, brief.Step)
	assert.True(t, brief.Step.Acceptance.Found)
	require.Len(t, brief.Shortfalls, 1)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-03.md"), brief.Shortfalls[0].Path)
	assert.Equal(t, `"`+cfg.AcceptanceHeading+`" is empty`, brief.Shortfalls[0].Detail)
	assert.Equal(t, "write the step's acceptance criteria under it", brief.Shortfalls[0].Fix)
}

func Test_start_reports_a_checklist_with_no_items_as_a_shortfall(t *testing.T) {
	cfg := fixtureConfig()
	files := newFixtureFiles(cfg)
	files["STEP-03.md"] = "---\n" +
		"id: STEP-03\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# STEP-03 Assemble the brief\n\n" +
		cfg.AcceptanceHeading + "\n\naccept\n\n" +
		cfg.ChecklistHeading + "\n\n```\n- [ ] fenced, not a real item\n```\n"

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(featureFS(files))

	require.NoError(t, err)
	require.NotNil(t, brief.Step)
	assert.True(t, brief.Step.Checklist.Found)
	require.Len(t, brief.Shortfalls, 1)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-03.md"), brief.Shortfalls[0].Path)
	assert.Equal(t, `"`+cfg.ChecklistHeading+`" has no checklist items`, brief.Shortfalls[0].Detail)
	assert.Equal(t,
		`add them as "- [ ]" lines before implementing, since brief finish refuses a step with none`,
		brief.Shortfalls[0].Fix)
}

// Control arm for the whitespace-only-acceptance shortfall above.
func Test_start_names_nothing_for_an_acceptance_section_holding_only_a_comment(t *testing.T) {
	cfg := fixtureConfig()
	files := newFixtureFiles(cfg)
	files["STEP-03.md"] = "---\n" +
		"id: STEP-03\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# STEP-03 Assemble the brief\n\n" +
		cfg.AcceptanceHeading + "\n\n<!-- filled in later -->\n\n" +
		cfg.ChecklistHeading + "\n\n- [ ] task\n"

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(featureFS(files))

	require.NoError(t, err)
	assert.Empty(t, brief.Shortfalls)
}

// Control arm for the checklist-with-no-items shortfall above.
func Test_start_names_nothing_for_a_checklist_with_at_least_one_item(t *testing.T) {
	cfg := fixtureConfig()

	cases := []struct {
		name      string
		checklist string
	}{
		{name: "one unticked item", checklist: "- [ ] task"},
		{name: "one ticked item", checklist: "- [x] task"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := newFixtureFiles(cfg)
			files["STEP-03.md"] = "---\n" +
				"id: STEP-03\n" +
				"status: open\n" +
				"depends-on: []\n" +
				"---\n\n" +
				"# STEP-03 Assemble the brief\n\n" +
				cfg.AcceptanceHeading + "\n\naccept\n\n" +
				cfg.ChecklistHeading + "\n\n" + c.checklist + "\n"

			srv := assemble.NewServer(cfg, "")

			brief, err := srv.StartFS(featureFS(files))

			require.NoError(t, err)
			assert.Empty(t, brief.Shortfalls)
		})
	}
}

func Test_start_orders_acceptance_then_checklist_then_state_shortfalls(t *testing.T) {
	cfg := fixtureConfig()
	files := newFixtureFiles(cfg)
	files["STEP-03.md"] = "---\n" +
		"id: STEP-03\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# STEP-03 Assemble the brief\n\n" +
		cfg.AcceptanceHeading + "\n\n\n" +
		cfg.ChecklistHeading + "\n\nno items here\n"
	files[cfg.StateFile] = cfg.StateHeadings.LeftUnbuilt + "\n\nSTATE-UNBUILT-A\n\n" +
		cfg.StateHeadings.Traps + "\n\nSTATE-TRAP-A\n\n" +
		cfg.StateHeadings.OpenDebts + "\n\nSTATE-DEBT-A\n"

	srv := assemble.NewServer(cfg, "")

	brief, err := srv.StartFS(featureFS(files))

	require.NoError(t, err)
	require.Len(t, brief.Shortfalls, 3)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-03.md"), brief.Shortfalls[0].Path)
	assert.Contains(t, brief.Shortfalls[0].Detail, cfg.AcceptanceHeading)
	assert.Equal(t, filepath.Join(testFeaturePath, "STEP-03.md"), brief.Shortfalls[1].Path)
	assert.Contains(t, brief.Shortfalls[1].Detail, cfg.ChecklistHeading)
	assert.Equal(t, filepath.Join(testFeaturePath, cfg.StateFile), brief.Shortfalls[2].Path)
	assert.Contains(t, brief.Shortfalls[2].Detail, cfg.StateHeadings.BindingDecisions)
}
