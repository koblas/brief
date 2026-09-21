package markdown_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/stretchr/testify/assert"
)

// Test_FirstUnchecked_reports_nothing collects every shape that must NOT
// yield an open item. They are gathered because they share one assertion —
// found is false — and differ only in the reason, so a table names the
// seven reasons in one place instead of repeating the same three lines
// seven times.
//
// Each case must fail for its own reason, not by accident of another: every
// body here carries the configured heading (except the absent-heading case,
// whose whole point is that it does not), so a bug that stopped finding the
// section would redden the positive tests below rather than silently making
// this whole table pass.
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
			// Replaces an earlier "the section holds no items" case whose
			// body carried no checklist syntax at all, so no mutation could
			// flip it — decorative by the rule this table is written under.
			// An empty bracket pair is the grammar point the package doc
			// claims is prose and nothing else covered.
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

// Test_trims_tabs_around_an_items_text pins what the item regexp's own
// horizontal-whitespace class has to do and nothing else did: the text is
// returned without the tabs that separate it from the marker or trail it.
// Before the marker and the text came from one match, a hand-rolled split
// trimmed " \t" by hand, and nothing covered the tab half of it.
func Test_trims_tabs_around_an_items_text(t *testing.T) {
	body := "## Implementation Plan\n\n- [ ]\ttabbed\t\n"

	_, text, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.True(t, found)
	assert.Equal(t, "tabbed", text)
}

// Test_trims_trailing_whitespace_before_a_carriage_return is the case the
// tab and CRLF tests each half-cover and neither pins: horizontal
// whitespace AND a carriage return, together, at the end of one line.
//
// It is what fixes the order of the item regexp's two tail elements.
// Trimming the whitespace before consuming the "\r" yields "two"; consuming
// the "\r" first leaves the trim class nothing to eat at the end of the
// text, and the lazy group swallows both, yielding "two \r". Every other
// test here is blind to that swap, because each supplies only one of the
// two characters.
func Test_trims_trailing_whitespace_before_a_carriage_return(t *testing.T) {
	body := "## Implementation Plan\r\n\r\n- [ ] two \r\n"

	_, text, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.True(t, found)
	assert.Equal(t, "two", text)
}
