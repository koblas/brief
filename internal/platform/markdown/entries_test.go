package markdown_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/stretchr/testify/assert"
)

// Test_Entries_recognizes_every_column_zero_marker pins D1's three
// recognized bullet grammars — "- ", "* " and "N. " — each anchored at
// column 0 directly under the heading, one blank line below it so the
// case never depends on continuation-line folding.
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

// Test_Entries_excludes_lines_that_are_not_column_zero_items collects the
// two shapes D1 says are never an entry of their own — a paragraph line
// and an indented line — each placed first in the section, directly under
// the heading rather than under a preceding item, so there is no entry in
// progress for either to fold into as continuation text.
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

// Test_Entries_excludes_a_column_zero_thematic_break pins D1's thematic-
// break exclusion: a line made only of one marker character repeated
// three or more times, space-separated, starts with what looks like a
// valid bullet marker ("* " or "- ") but is a horizontal rule (CommonMark
// §4.1), never an entry. Test_Entries_recognizes_every_column_zero_marker
// is this test's own control arm, proving the same marker characters
// still open a genuine entry when they are not a thematic break.
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

// Test_Entries_returns_nothing_when_the_heading_is_absent matches
// Section's own ("", false) contract for a heading that never appears.
func Test_Entries_returns_nothing_when_the_heading_is_absent(t *testing.T) {
	got := markdown.Entries("no heading here\n", "## Heading")

	assert.Empty(t, got)
}

// Test_Entries_folds_a_wrapped_continuation_line_into_the_entry_text pins
// D1's continuation-line folding: an unindented wrapped second physical
// line joins the item regardless of its own indentation ("lazy
// continuation"), and a following indented sub-item folds in too, its own
// list marker kept rather than stripped — only the parent item's own
// leading marker is stripped, once, at Text's start. Line stays the item's
// own first line even though Text now spans two later lines (D6).
func Test_Entries_folds_a_wrapped_continuation_line_into_the_entry_text(t *testing.T) {
	body := "## Heading\n\n- item text\nwrapped continuation\n  - sub item\n"

	got := markdown.Entries(body, "## Heading")

	assert.Equal(t, []markdown.Entry{{Line: 3, Text: "item text wrapped continuation - sub item"}}, got)
}

// Test_Entries_continuation_stops_at_blank_line_next_item_heading_and_fence
// pins every boundary that ends an in-progress entry's continuation, one
// continuation line between the item and the boundary in every case, and
// asserts the exact resulting Entries rather than only an absence — an
// implementation that folds nothing would otherwise pass this table
// vacuously. The underscore, compact (no interior spaces) and up-to-three-
// leading-space cases pin thematicBreakRe's other two marker characters and
// CommonMark's own indentation tolerance, none of which
// Test_Entries_excludes_a_column_zero_thematic_break exercises on its own.
// The four-or-more-space case is the negative control on that same
// boundary: past three leading spaces thematicBreakRe no longer matches,
// so the line is ordinary continuation text and folds in rather than
// ending the entry.
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
