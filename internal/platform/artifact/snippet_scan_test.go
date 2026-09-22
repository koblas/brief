package artifact_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_ScanSnippetMarkers_finds_no_span_in_plain_text pins the Zero case:
// text with no marker line at all reports a nil span and no problem.
func Test_ScanSnippetMarkers_finds_no_span_in_plain_text(t *testing.T) {
	span, prob := artifact.ScanSnippetMarkers([]byte("just some prose\nnothing more\n"))

	assert.Nil(t, span)
	assert.Nil(t, prob)
}

// Test_ScanSnippetMarkers_reports_a_marker_defect_with_its_line pins every
// named refusal shape and the 1-based line number each reports: a lone
// begin marker, a lone end marker, an end before any begin, a second begin
// in one file, and a second begin with no closing end for the first.
func Test_ScanSnippetMarkers_reports_a_marker_defect_with_its_line(t *testing.T) {
	begin := artifact.SnippetBegin
	end := artifact.SnippetEnd

	cases := []struct {
		name     string
		body     string
		wantLine int
	}{
		{name: "lone begin", body: "one\n" + begin + "\nno end after this\n", wantLine: 2},
		{name: "lone end", body: "one\n" + end + "\nno begin before this\n", wantLine: 2},
		{name: "end before begin", body: end + "\ntext\n" + begin + "\n" + end + "\n", wantLine: 1},
		{name: "second begin, one file", body: begin + "\n" + end + "\n" + begin + "\n" + end + "\n", wantLine: 3},
		{name: "second begin, no closing end for the first", body: begin + "\n" + begin + "\n" + end + "\n", wantLine: 2},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			span, prob := artifact.ScanSnippetMarkers([]byte(c.body))

			assert.Nil(t, span)
			require.NotNil(t, prob)
			assert.Equal(t, c.wantLine, prob.Line)
		})
	}
}

// Test_ScanSnippetMarkers_finds_the_span_of_one_valid_block pins the
// happy-path span: begin marker's line through the end marker's own text,
// excluding its line terminator.
func Test_ScanSnippetMarkers_finds_the_span_of_one_valid_block(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")
	body := "before\n\n" + string(block) + "\nafter"

	span, prob := artifact.ScanSnippetMarkers([]byte(body))

	require.Nil(t, prob)
	require.NotNil(t, span)
	assert.Equal(t, string(block), string([]byte(body)[span.Start:span.End]))
	assert.Equal(t, 3, span.BeginLine)
}
