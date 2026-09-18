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
