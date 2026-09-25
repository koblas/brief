package artifact_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ScanSnippetMarkers_finds_no_span_in_plain_text(t *testing.T) {
	span, prob := artifact.ScanSnippetMarkers([]byte("just some prose\nnothing more\n"))

	assert.Nil(t, span)
	assert.Nil(t, prob)
}

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

func Test_ScanSnippetMarkers_finds_the_span_of_one_valid_block(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")
	body := "before\n\n" + string(block) + "\nafter"

	span, prob := artifact.ScanSnippetMarkers([]byte(body))

	require.Nil(t, prob)
	require.NotNil(t, span)
	assert.Equal(t, string(block), string([]byte(body)[span.Start:span.End]))
	assert.Equal(t, 3, span.BeginLine)
}
