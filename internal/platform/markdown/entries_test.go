package markdown_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/stretchr/testify/assert"
)

func Test_Entries_recognizes_every_column_zero_marker(t *testing.T) {
	cases := []struct {
		name   string
		marker string
	}{
		{name: "hyphen bullet", marker: "-"},
		{name: "asterisk bullet", marker: "*"},
		{name: "numbered item", marker: "1."},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := "## Heading\n\n" + c.marker + " item text\n"

			got := markdown.Entries(body, "## Heading")

			assert.Equal(t, []markdown.Entry{{Line: 3, Text: "item text"}}, got)
		})
	}
}

func Test_Entries_excludes_lines_that_are_not_column_zero_items(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "a paragraph line", body: "## Heading\n\nparagraph text\n"},
		{name: "an indented list item", body: "## Heading\n\n  - indented item\n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := markdown.Entries(c.body, "## Heading")

			assert.Empty(t, got)
		})
	}
}

func Test_Entries_excludes_a_column_zero_thematic_break(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{name: "asterisks separated by spaces", line: "* * *"},
		{name: "hyphens separated by spaces", line: "- - -"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := "## Heading\n\n" + c.line + "\n"

			got := markdown.Entries(body, "## Heading")

			assert.Empty(t, got)
		})
	}
}

func Test_Entries_returns_nothing_when_the_heading_is_absent(t *testing.T) {
	got := markdown.Entries("no heading here\n", "## Heading")

	assert.Empty(t, got)
}

func Test_Entries_folds_a_wrapped_continuation_line_into_the_entry_text(t *testing.T) {
	body := "## Heading\n\n- item text\nwrapped continuation\n  - sub item\n"

	got := markdown.Entries(body, "## Heading")

	assert.Equal(t, []markdown.Entry{{Line: 3, Text: "item text wrapped continuation - sub item"}}, got)
}

func Test_Entries_continuation_stops_at_blank_line_next_item_heading_and_fence(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []markdown.Entry
	}{
		{
			name: "a blank line then a second item stops the continuation",
			body: "## Heading\n\n- item one\ncontinuation\n\n- item two\n",
			want: []markdown.Entry{{Line: 3, Text: "item one continuation"}, {Line: 6, Text: "item two"}},
		},
		{
			name: "a second item directly after the continuation line stops it immediately",
			body: "## Heading\n\n- item one\ncontinuation\n- item two\n",
			want: []markdown.Entry{{Line: 3, Text: "item one continuation"}, {Line: 5, Text: "item two"}},
		},
		{
			name: "a heading deeper than the section's own heading ends the continuation but not the section",
			body: "## Heading\n\n- item one\ncontinuation\n### Sub heading\n- item two\n",
			want: []markdown.Entry{{Line: 3, Text: "item one continuation"}, {Line: 6, Text: "item two"}},
		},
		{
			name: "an opening fence ends the continuation and neither its contents nor the line after the closing fence fold in",
			body: "## Heading\n\n- item one\ncontinuation\n```\nfence content\n```\ntrailing text\n",
			want: []markdown.Entry{{Line: 3, Text: "item one continuation"}},
		},
		{
			name: "a thematic break directly after the continuation stops it immediately and is not itself an entry",
			body: "## Heading\n\n- item one\ncontinuation\n* * *\n",
			want: []markdown.Entry{{Line: 3, Text: "item one continuation"}},
		},
		{
			name: "an underscore thematic break directly after the continuation stops it immediately",
			body: "## Heading\n\n- item one\ncontinuation\n___\n",
			want: []markdown.Entry{{Line: 3, Text: "item one continuation"}},
		},
		{
			name: "a compact thematic break with no spaces between markers directly after the continuation stops it immediately",
			body: "## Heading\n\n- item one\ncontinuation\n---\n",
			want: []markdown.Entry{{Line: 3, Text: "item one continuation"}},
		},
		{
			name: "a thematic break indented up to three spaces directly after the continuation still stops it immediately",
			body: "## Heading\n\n- item one\ncontinuation\n   ***\n",
			want: []markdown.Entry{{Line: 3, Text: "item one continuation"}},
		},
		{
			name: "a thematic break indented four spaces is outside thematicBreakRe's bound and folds in as continuation text",
			body: "## Heading\n\n- item one\n    ***\n",
			want: []markdown.Entry{{Line: 3, Text: "item one ***"}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := markdown.Entries(c.body, "## Heading")

			assert.Equal(t, c.want, got)
		})
	}
}
