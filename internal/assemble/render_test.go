package assemble_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Round-trips a fully populated Brief through RenderJSON and back,
// proving every exported field survives the wire format.
func Test_RenderJSON_writes_every_field_of_a_brief_with_an_open_step(t *testing.T) {
	b := assemble.Brief{
		Done: 2,
		Open: 3,
		Step: &assemble.Step{
			ID:         "STEP-03",
			Title:      "STEP-03 Assemble the brief",
			Acceptance: assemble.Section{Heading: "## Fixture Scenario", Body: "ACCEPTANCE-03", Found: true},
			Checklist:  assemble.Section{Heading: "## Fixture Checklist", Body: "- [ ] CHECKLIST-03-A", Found: true},
		},
		Inherited: []assemble.Section{
			{Heading: "## Decisions Fixture", Body: "STATE-DECISION-A", Found: true},
			{Heading: "## Debts Fixture", Body: "STATE-DEBT-A", Found: true},
		},
	}

	var out bytes.Buffer
	err := assemble.RenderJSON(&out, b)
	require.NoError(t, err)

	var got assemble.Brief
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))

	assert.Equal(t, b, got)
}

// Asserts the raw bytes, not unmarshalled output: unmarshalling would
// collapse null and absent into the same Go zero value.
func Test_RenderJSON_marshals_a_nil_step_as_null(t *testing.T) {
	nilStep := assemble.Brief{Done: 5, Open: 0}
	var out bytes.Buffer
	require.NoError(t, assemble.RenderJSON(&out, nilStep))

	assert.Contains(t, out.String(), `"step":null`)

	openStep := assemble.Brief{Step: &assemble.Step{ID: "STEP-01", Title: "STEP-01 Title"}}
	var controlOut bytes.Buffer
	require.NoError(t, assemble.RenderJSON(&controlOut, openStep))

	assert.Contains(t, controlOut.String(), `"step":{`)
}

// An empty feature and a completed one both share "step":null; only the
// counts distinguish them.
func Test_RenderJSON_keeps_the_zero_counts_a_caller_needs(t *testing.T) {
	b := assemble.Brief{Done: 0, Open: 0}

	var out bytes.Buffer
	require.NoError(t, assemble.RenderJSON(&out, b))

	assert.Contains(t, out.String(), `"done":0`)
	assert.Contains(t, out.String(), `"open":0`)
}

// Same null-vs-absent claim as the step field, pinned on bytes.
func Test_RenderJSON_marshals_absent_shortfalls_as_null(t *testing.T) {
	none := assemble.Brief{}
	var out bytes.Buffer
	require.NoError(t, assemble.RenderJSON(&out, none))

	assert.Contains(t, out.String(), `"shortfalls":null`)

	some := assemble.Brief{Shortfalls: []assemble.Shortfall{{Path: "/f", Detail: "d", Fix: "f"}}}
	var someOut bytes.Buffer
	require.NoError(t, assemble.RenderJSON(&someOut, some))

	assert.Contains(t, someOut.String(), `"shortfalls":[{`)
}

// RenderJSON carries an absent-heading section RenderText leaves out
// entirely.
func Test_RenderJSON_keeps_a_section_RenderText_omits(t *testing.T) {
	b := assemble.Brief{
		Step: &assemble.Step{ID: "STEP-01", Title: "STEP-01 Title"},
		Inherited: []assemble.Section{
			{Heading: "## Absent Fixture", Body: "", Found: false},
		},
	}

	var jsonOut bytes.Buffer
	require.NoError(t, assemble.RenderJSON(&jsonOut, b))
	assert.Contains(t, jsonOut.String(), `"found":false`)

	var textOut bytes.Buffer
	require.NoError(t, assemble.RenderText(&textOut, b))
	assert.NotContains(t, textOut.String(), "## Absent Fixture")
}

// A plain json.Marshal would rewrite these to \uNNNN escapes, since Go's
// encoder defaults to HTML-safe escaping.
func Test_RenderJSON_leaves_angle_brackets_and_ampersands_unescaped(t *testing.T) {
	b := assemble.Brief{
		Step: &assemble.Step{
			ID:         "STEP-01",
			Title:      "STEP-01 Title",
			Acceptance: assemble.Section{Heading: "## Fixture", Body: "a <b> & c", Found: true},
		},
	}

	var out bytes.Buffer
	require.NoError(t, assemble.RenderJSON(&out, b))

	escapedLT := "\\u003c"
	escapedAmp := "\\u0026"

	assert.Contains(t, out.String(), "a <b> & c")
	assert.NotContains(t, out.String(), escapedLT)
	assert.NotContains(t, out.String(), escapedAmp)
}

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

// One golden across every row shape, the longest feature name placed
// last, proving column width is computed from every row.
func Test_RenderStatusText_prints_a_table_with_header_and_aligned_columns(t *testing.T) {
	rows := []assemble.FeatureStatus{
		{Name: "alpha", Done: 1, Total: 3, Blocked: 0, Next: &assemble.NextStep{ID: "SCENARIO-02", Title: "Open the door"}},
		{Name: "beta", Done: 3, Total: 3, Blocked: 0},
		{Name: "gamma", Done: 0, Total: 0, Blocked: 0},
		{Name: "epsilon", Done: 1, Total: 2, Blocked: 1, Next: &assemble.NextStep{ID: "SCENARIO-02"}},
		{Name: "zzz-longest-feature-name", Problem: &assemble.Problem{Path: "/repo/docs/specifications/zzz-longest-feature-name", Detail: "no frontmatter found", Fix: "fix it"}},
	}

	var out bytes.Buffer
	err := assemble.RenderStatusText(&out, rows)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"FEATURE                   DONE  BLOCKED  NEXT\n"+
		"alpha                     1/3   0        SCENARIO-02  Open the door\n"+
		"beta                      3/3   0        (complete)\n"+
		"gamma                     0/0   0        -\n"+
		"epsilon                   1/2   1        SCENARIO-02\n"+
		"zzz-longest-feature-name  -     -        (malformed, see below)\n",
		out.String())

	for line := range strings.SplitSeq(strings.TrimSuffix(out.String(), "\n"), "\n") {
		assert.False(t, strings.HasSuffix(line, " "), "line %q must not end in a space", line)
	}
}

// A freshly scaffolded step file opens with "# <id>" as its only heading,
// so NEXT must not double the id.
func Test_RenderStatusText_shows_the_id_alone_when_the_title_equals_it(t *testing.T) {
	rows := []assemble.FeatureStatus{
		{Name: "alpha", Done: 0, Total: 1, Blocked: 0, Next: &assemble.NextStep{ID: "SCENARIO-01", Title: "SCENARIO-01"}},
	}

	var out bytes.Buffer
	err := assemble.RenderStatusText(&out, rows)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"alpha    0/1   0        SCENARIO-01\n",
		out.String())
}

// Zero features means zero bytes, not a bare header.
func Test_RenderStatusText_writes_nothing_for_an_empty_slice(t *testing.T) {
	var out bytes.Buffer
	err := assemble.RenderStatusText(&out, nil)

	require.NoError(t, err)
	assert.Empty(t, out.String())
}

// Both the name and the title carry a tab or newline, so a fix that
// flattens only one of the two fields still reddens this test.
func Test_RenderStatusText_flattens_a_tab_or_newline_in_the_feature_name_or_title(t *testing.T) {
	rows := []assemble.FeatureStatus{
		{Name: "a\tb", Done: 0, Total: 1, Blocked: 0, Next: &assemble.NextStep{ID: "SCENARIO-01", Title: "Open\nthe door"}},
	}

	var out bytes.Buffer
	err := assemble.RenderStatusText(&out, rows)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"FEATURE  DONE  BLOCKED  NEXT\n"+
		"a b      0/1   0        SCENARIO-01  Open the door\n",
		out.String())
}

func Test_RenderFindings_writes_a_group_header_and_its_indented_findings(t *testing.T) {
	groups := []assemble.FeatureFindings{
		{
			Name: "demo", Path: "/repo/docs/specifications/demo", InFlight: true,
			Findings: []assemble.Finding{
				{Severity: assemble.SeverityError, Path: "/repo/docs/specifications/demo/STEP-01.md", Line: 12, Detail: `checklist item "x" is not ticked`},
				{Severity: assemble.SeverityError, Path: "/repo/docs/specifications/demo/NOTES.md", Line: 0, Detail: "state is 90 lines, over the cap of 80"},
			},
		},
	}

	var out bytes.Buffer
	require.NoError(t, assemble.RenderFindings(&out, groups))

	assert.Equal(t, ""+
		"demo  (in flight)\n"+
		"  ERROR  /repo/docs/specifications/demo/STEP-01.md:12  checklist item \"x\" is not ticked\n"+
		"  ERROR  /repo/docs/specifications/demo/NOTES.md  state is 90 lines, over the cap of 80\n",
		out.String())
}

// "(complete)" is read from InFlight, never recomputed from severity.
func Test_RenderFindings_labels_a_complete_feature_s_header_from_InFlight(t *testing.T) {
	groups := []assemble.FeatureFindings{
		{
			Name: "demo", Path: "/repo/docs/specifications/demo", InFlight: false,
			Findings: []assemble.Finding{
				{Severity: assemble.SeverityWarn, Path: "/repo/docs/specifications/demo/NOTES.md", Detail: "state is 90 lines, over the cap of 80"},
			},
		},
	}

	var out bytes.Buffer
	require.NoError(t, assemble.RenderFindings(&out, groups))

	assert.Contains(t, out.String(), "demo  (complete)\n")
}

func Test_RenderFindings_separates_groups_with_exactly_one_blank_line(t *testing.T) {
	groups := []assemble.FeatureFindings{
		{
			Name: "alpha", Path: "/repo/docs/specifications/alpha", InFlight: true,
			Findings: []assemble.Finding{{Severity: assemble.SeverityError, Path: "/repo/docs/specifications/alpha/A.md", Detail: "alpha problem"}},
		},
		{
			Name: "beta", Path: "/repo/docs/specifications/beta", InFlight: false,
			Findings: []assemble.Finding{{Severity: assemble.SeverityWarn, Path: "/repo/docs/specifications/beta/B.md", Detail: "beta problem"}},
		},
	}

	var out bytes.Buffer
	require.NoError(t, assemble.RenderFindings(&out, groups))

	assert.Equal(t, ""+
		"alpha  (in flight)\n"+
		"  ERROR  /repo/docs/specifications/alpha/A.md  alpha problem\n"+
		"\n"+
		"beta  (complete)\n"+
		"  WARN  /repo/docs/specifications/beta/B.md  beta problem\n",
		out.String())
}

func Test_RenderFindings_flattens_a_tab_or_newline_in_the_detail(t *testing.T) {
	groups := []assemble.FeatureFindings{
		{
			Name: "demo", Path: "/repo/docs/specifications/demo", InFlight: true,
			Findings: []assemble.Finding{
				{Severity: assemble.SeverityError, Path: "/repo/docs/specifications/demo/A.md", Detail: "a\tb\nc"},
			},
		},
	}

	var out bytes.Buffer
	require.NoError(t, assemble.RenderFindings(&out, groups))

	assert.Contains(t, out.String(), "  a b c\n")
}

// Symmetric with Detail above, for the group header's own Name.
func Test_RenderFindings_flattens_a_tab_or_newline_in_the_group_name(t *testing.T) {
	groups := []assemble.FeatureFindings{
		{
			Name: "de\tmo\nx", Path: "/repo/docs/specifications/demo", InFlight: true,
			Findings: []assemble.Finding{
				{Severity: assemble.SeverityError, Path: "/repo/docs/specifications/demo/A.md", Detail: "a problem"},
			},
		},
	}

	var out bytes.Buffer
	require.NoError(t, assemble.RenderFindings(&out, groups))

	assert.Contains(t, out.String(), "de mo x  (in flight)\n")
}

// A conforming tree's Check result renders as zero bytes, never a header
// or a "no findings" banner.
func Test_RenderFindings_writes_nothing_for_an_empty_slice(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, assemble.RenderFindings(&out, nil))

	assert.Empty(t, out.String())
}
