package markdown_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/stretchr/testify/assert"
)

func Test_finds_the_first_unchecked_item_with_its_line_number_in_the_whole_body(t *testing.T) {
	body := "---\nid: STEP-01\nstatus: open\ndepends-on: []\n---\n\n# Title\n\n## Implementation Plan\n\n- [x] one\n- [ ] two\n"

	line, text, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.True(t, found)
	assert.Equal(t, 12, line)
	assert.Equal(t, "two", text)
}

func Test_reports_no_unchecked_item_when_every_item_is_ticked(t *testing.T) {
	body := "## Implementation Plan\n\n- [x] one\n- [x] two\n"

	_, _, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.False(t, found)
}

func Test_reports_no_unchecked_item_when_the_section_holds_no_items(t *testing.T) {
	body := "## Implementation Plan\n\n## Next Section\n\nprose\n"

	_, _, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.False(t, found)
}

func Test_reports_no_unchecked_item_when_the_heading_is_absent(t *testing.T) {
	body := "## Some Other Heading\n\n- [ ] item\n"

	_, _, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.False(t, found)
}

func Test_ignores_an_unchecked_item_inside_a_fenced_block(t *testing.T) {
	body := "## Implementation Plan\n\n```\n- [ ] fenced\n```\n"

	_, _, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.False(t, found)
}

func Test_stops_at_the_next_heading_of_the_same_level(t *testing.T) {
	body := "## Implementation Plan\n\n- [x] one\n\n## Notes\n\n- [ ] not this section's\n"

	_, _, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.False(t, found)
}

func Test_counts_an_indented_item(t *testing.T) {
	body := "## Implementation Plan\n\n    - [ ] indented\n"

	line, text, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.True(t, found)
	assert.Equal(t, 3, line)
	assert.Equal(t, "indented", text)
}

func Test_does_not_treat_a_star_bullet_as_an_item(t *testing.T) {
	body := "## Implementation Plan\n\n* [ ] not a hyphen bullet\n"

	_, _, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.False(t, found)
}

func Test_does_not_treat_an_uppercase_X_item_as_unchecked(t *testing.T) {
	body := "## Implementation Plan\n\n- [X] done\n"

	_, _, found := markdown.FirstUnchecked(body, "## Implementation Plan")

	assert.False(t, found)
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
