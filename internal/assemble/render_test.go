package assemble_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_RenderJSON_writes_every_field_of_a_brief_with_an_open_step round-trips
// a fully populated Brief through RenderJSON and back into a fresh
// assemble.Brief, and asserts the decoded value equals the original —
// proving every exported field survives the wire format under its
// documented tag.
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

// Test_RenderJSON_marshals_a_nil_step_as_null asserts the raw bytes carry
// "step":null for a Brief with no open step — the completion discriminator
// a structured caller keys off. Unmarshalling would collapse null and
// absent into the same Go zero value, so the claim is made on the bytes
// themselves. The control arm proves the same assertion would fail for a
// Brief that does carry an open step, where the same byte range reads
// "step":{ instead.
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

// Test_RenderJSON_keeps_the_zero_counts_a_caller_needs asserts the raw
// bytes carry "done":0 and "open":0 for a Brief with no steps at all, so a
// caller can tell an empty feature apart from a completed one — both share
// "step":null, and only the counts distinguish them.
func Test_RenderJSON_keeps_the_zero_counts_a_caller_needs(t *testing.T) {
	b := assemble.Brief{Done: 0, Open: 0}

	var out bytes.Buffer
	require.NoError(t, assemble.RenderJSON(&out, b))

	assert.Contains(t, out.String(), `"done":0`)
	assert.Contains(t, out.String(), `"open":0`)
}

// Test_RenderJSON_marshals_absent_shortfalls_as_null asserts the raw bytes
// carry "shortfalls":null when Brief.Shortfalls is nil, and an array when
// it is populated — the same null-vs-absent claim as the step field,
// pinned on bytes for the same reason.
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

// Test_RenderJSON_keeps_a_section_RenderText_omits asserts RenderJSON
// carries an absent-heading section RenderText leaves out entirely: the
// raw bytes hold "found":false for it, a claim unmarshalling can't make
// since a decoded, omitted "found" key reads as false either way. Through
// RenderText the same Brief omits the section's heading altogether.
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

// Test_RenderJSON_leaves_angle_brackets_and_ampersands_unescaped asserts a
// section body's angle brackets and ampersand reach the wire unrewritten —
// a plain json.Marshal would rewrite them to their \uNNNN escapes because
// Go's encoder defaults to HTML-safe escaping.
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

// Test_RenderFindings_writes_the_profile_s_finding_shape pins the exact
// byte shape SCENARIO-22 ships: "[SEVERITY] <path>:<line> — <problem>", one
// line per Finding, no Fix — the write path's refusal carries one, a
// report of a tree Finish was never asked to write does not.
func Test_RenderFindings_writes_the_profile_s_finding_shape(t *testing.T) {
	findings := []assemble.Finding{
		{Severity: assemble.SeverityError, Path: "/repo/docs/specifications/demo/STEP-01.md", Line: 12, Detail: `checklist item "x" is not ticked`},
		{Severity: assemble.SeverityWarn, Path: "/repo/docs/specifications/demo/NOTES.md", Line: 0, Detail: "state is 90 lines, over the cap of 80"},
	}

	var out bytes.Buffer
	require.NoError(t, assemble.RenderFindings(&out, findings))

	assert.Equal(t, ""+
		"[ERROR] /repo/docs/specifications/demo/STEP-01.md:12 — checklist item \"x\" is not ticked\n"+
		"[WARN] /repo/docs/specifications/demo/NOTES.md:0 — state is 90 lines, over the cap of 80\n",
		out.String())
}

// Test_RenderFindings_writes_nothing_for_an_empty_slice is the SCENARIO-10
// shape's render-side half: a conforming tree's Check result renders as
// zero bytes, never a header or a "no findings" banner.
func Test_RenderFindings_writes_nothing_for_an_empty_slice(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, assemble.RenderFindings(&out, nil))

	assert.Empty(t, out.String())
}
