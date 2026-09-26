package markdown_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/stretchr/testify/assert"
)

func Test_FirstUnchecked_reports_nothing(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "every item is ticked",
			body: "## Implementation Plan\n\n- [x] one\n- [x] two\n",
		},
		{
			name: "an empty bracket pair is prose, not an item",
			body: "## Implementation Plan\n\n- [] no space in the marker\n",
		},
		{
			name: "the heading is absent",
			body: "## Some Other Heading\n\n- [ ] item\n",
		},
		{
			name: "the only unchecked item is inside a fenced block",
			body: "## Implementation Plan\n\n```\n- [ ] fenced\n```\n",
		},
		{
			name: "the unchecked item belongs to the next section",
			body: "## Implementation Plan\n\n- [x] one\n\n## Notes\n\n- [ ] not this section's\n",
		},
		{
			name: "a star bullet is prose, not an item",
			body: "## Implementation Plan\n\n* [ ] not a hyphen bullet\n",
		},
		{
			name: "an uppercase X is ticked",
			body: "## Implementation Plan\n\n- [X] done\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, found := markdown.FirstUnchecked(c.body, "## Implementation Plan")

			assert.False(t, found)
		})
	}
}

func Test_finds_the_first_unchecked_item_with_its_line_number_in_the_whole_body(t *testing.T) {
	body := "---\nid: STEP-01\nstatus: open\ndepends-on: []\n---\n\n# Title\n\n## Implementation Plan\n\n- [x] one\n- [ ] two\n"

	line, text, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.True(t, found)
	assert.Equal(t, 12, line)
	assert.Equal(t, "two", text)
}

func Test_counts_an_indented_item(t *testing.T) {
	body := "## Implementation Plan\n\n    - [ ] indented\n"

	line, text, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.True(t, found)
	assert.Equal(t, 3, line)
	assert.Equal(t, "indented", text)
}

func Test_returns_empty_text_for_a_bare_unchecked_item(t *testing.T) {
	body := "## Implementation Plan\n\n- [ ]\n"

	line, text, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.True(t, found)
	assert.Equal(t, 3, line)
	assert.Empty(t, text)
}

func Test_trims_the_carriage_return_from_an_item_in_a_CRLF_body(t *testing.T) {
	body := "## Implementation Plan\r\n\r\n- [ ] two\r\n"

	_, text, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.True(t, found)
	assert.Equal(t, "two", text)
}

func Test_trims_tabs_around_an_items_text(t *testing.T) {
	body := "## Implementation Plan\n\n- [ ]\ttabbed\t\n"

	_, text, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.True(t, found)
	assert.Equal(t, "tabbed", text)
}

func Test_trims_trailing_whitespace_before_a_carriage_return(t *testing.T) {
	body := "## Implementation Plan\r\n\r\n- [ ] two \r\n"

	_, text, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.True(t, found)
	assert.Equal(t, "two", text)
}

func Test_CountChecklistItems_reports_the_item_count_and_whether_the_heading_was_found(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		wantN  int
		wantOK bool
	}{
		{
			name:   "the heading is absent",
			body:   "## Some Other Heading\n\n- [ ] item\n",
			wantN:  0,
			wantOK: false,
		},
		{
			name:   "the heading is present with no items",
			body:   "## Implementation Plan\n\nnothing here yet\n",
			wantN:  0,
			wantOK: true,
		},
		{
			name:   "ticked and unticked items are both counted",
			body:   "## Implementation Plan\n\n- [x] one\n- [ ] two\n",
			wantN:  2,
			wantOK: true,
		},
		{
			name:   "an indented item is counted",
			body:   "## Implementation Plan\n\n    - [ ] indented\n",
			wantN:  1,
			wantOK: true,
		},
		{
			name:   "an item inside a fence is not counted",
			body:   "## Implementation Plan\n\n```\n- [ ] fenced\n```\n\n- [ ] real\n",
			wantN:  1,
			wantOK: true,
		},
		{
			name:   "an item under a subheading of the section is counted",
			body:   "## Implementation Plan\n\n### Red\n\n- [ ] one\n",
			wantN:  1,
			wantOK: true,
		},
		{
			name:   "an item in the next same-level section is not counted",
			body:   "## Implementation Plan\n\n- [x] one\n\n## Notes\n\n- [ ] not this section's\n",
			wantN:  1,
			wantOK: true,
		},
		{
			name:   "CRLF item lines are counted",
			body:   "## Implementation Plan\r\n\r\n- [ ] one\r\n- [x] two\r\n",
			wantN:  2,
			wantOK: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, ok := markdown.CountChecklistItems(c.body, "## Implementation Plan")

			assert.Equal(t, c.wantOK, ok)
			assert.Equal(t, c.wantN, n)
		})
	}
}
