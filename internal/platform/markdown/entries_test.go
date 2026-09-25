package markdown_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/stretchr/testify/assert"
)

// Test_Entries_recognizes_every_column_zero_marker pins D1's three
// recognized bullet grammars — "- ", "* " and "N. " — each anchored at
// column 0 directly under the heading, one blank line below it so the
// case never depends on continuation-line folding (built in a later
// scenario).
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
// two shapes D1 says are never an entry — a paragraph line and an indented
// line — each placed first in the section (never directly under a list
// item), so the case is not accidentally exercising the continuation-line
// rule a later scenario builds.
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

// Test_Entries_returns_nothing_when_the_heading_is_absent matches
// Section's own ("", false) contract for a heading that never appears.
func Test_Entries_returns_nothing_when_the_heading_is_absent(t *testing.T) {
	got := markdown.Entries("no heading here\n", "## Heading")

	assert.Empty(t, got)
}
