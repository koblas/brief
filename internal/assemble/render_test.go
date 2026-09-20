package assemble_test

import (
	"bytes"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_RenderText_renders_the_id_done_open_counts_and_title(t *testing.T) {
	b := assemble.Brief{
		Done: 2,
		Open: 3,
		Step: &assemble.Step{
			ID:    "STEP-03",
			Title: "STEP-03 Assemble the brief",
		},
	}

	var out bytes.Buffer
	err := assemble.RenderText(&out, b)

	require.NoError(t, err)
	assert.Equal(t, "STEP-03 — 2 done, 3 open\n\n# STEP-03 Assemble the brief\n", out.String())
}

func Test_RenderText_renders_the_step_s_sections_and_the_state_sections_in_order(t *testing.T) {
	b := assemble.Brief{
		Done: 2,
		Open: 3,
		Step: &assemble.Step{
			ID:         "STEP-03",
			Title:      "STEP-03 Assemble the brief",
			Acceptance: assemble.Section{Heading: "## Fixture Scenario", Body: "ACCEPTANCE-03"},
			Checklist:  assemble.Section{Heading: "## Fixture Checklist", Body: "- [ ] CHECKLIST-03-A"},
		},
		Inherited: []assemble.Section{
			{Heading: "## Decisions Fixture", Body: "STATE-DECISION-A"},
			{Heading: "## Debts Fixture", Body: "STATE-DEBT-A"},
		},
	}

	var out bytes.Buffer
	err := assemble.RenderText(&out, b)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"STEP-03 — 2 done, 3 open\n\n"+
		"# STEP-03 Assemble the brief\n\n"+
		"## Fixture Scenario\n\n"+
		"ACCEPTANCE-03\n\n"+
		"## Fixture Checklist\n\n"+
		"- [ ] CHECKLIST-03-A\n\n"+
		"## Decisions Fixture\n\n"+
		"STATE-DECISION-A\n\n"+
		"## Debts Fixture\n\n"+
		"STATE-DEBT-A\n",
		out.String())
}

func Test_RenderText_skips_a_state_section_with_an_empty_body(t *testing.T) {
	b := assemble.Brief{
		Step: &assemble.Step{ID: "STEP-03", Title: "STEP-03 Assemble the brief"},
		Inherited: []assemble.Section{
			{Heading: "## Decisions Fixture", Body: "STATE-DECISION-A"},
			{Heading: "## Left Fixture", Body: ""},
			{Heading: "## Debts Fixture", Body: "STATE-DEBT-A"},
		},
	}

	var out bytes.Buffer
	err := assemble.RenderText(&out, b)

	require.NoError(t, err)
	assert.NotContains(t, out.String(), "## Left Fixture")
	assert.Contains(t, out.String(), "## Decisions Fixture")
	assert.Contains(t, out.String(), "## Debts Fixture")
}

func Test_RenderText_writes_nothing_when_there_is_no_next_step(t *testing.T) {
	b := assemble.Brief{Done: 5, Open: 0}

	var out bytes.Buffer
	err := assemble.RenderText(&out, b)

	require.NoError(t, err)
	assert.Empty(t, out.String())
}

// Test_render_status_writes_four_space_separated_fields_per_feature pins
// the exact bytes of the status table: single-space separated, no padding,
// "-" substituted only at render time for a row whose Next is empty.
func Test_render_status_writes_four_space_separated_fields_per_feature(t *testing.T) {
	rows := []assemble.FeatureStatus{
		{Name: "alpha", Done: 1, Total: 3, Next: "SCENARIO-02", Blocked: 0},
		{Name: "beta", Done: 3, Total: 3, Next: "", Blocked: 0},
		{Name: "gamma", Done: 1, Total: 4, Next: "SCENARIO-02", Blocked: 2},
	}

	var out bytes.Buffer
	err := assemble.RenderStatusText(&out, rows)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"alpha 1/3 SCENARIO-02 0\n"+
		"beta 3/3 - 0\n"+
		"gamma 1/4 SCENARIO-02 2\n",
		out.String())
}

// Test_render_status_text_prints_the_marker_for_a_malformed_feature pins
// the exact bytes of a malformed row: "!" in all three computed fields,
// never a single-field marker — "-" already means "no next step" (09) and
// "0/0" already means an empty feature directory, so either would fabricate
// a value that was never measured.
func Test_render_status_text_prints_the_marker_for_a_malformed_feature(t *testing.T) {
	rows := []assemble.FeatureStatus{
		{Name: "delta", Problem: &assemble.Problem{Path: "/repo/docs/specifications/delta", Detail: "no frontmatter found", Fix: "fix it"}},
	}

	var out bytes.Buffer
	err := assemble.RenderStatusText(&out, rows)

	require.NoError(t, err)
	assert.Equal(t, "delta ! ! !\n", out.String())
}
