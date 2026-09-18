package markdown_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/stretchr/testify/assert"
)

func Test_Section_returns_the_body_under_the_heading(t *testing.T) {
	body := "# Title\n\n## Scenario\n\nsome body text\n\n## Implementation Plan\n\n- [ ] step\n"

	got, ok := markdown.Section(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "some body text", got)
}

func Test_Section_ends_at_the_next_heading_of_the_same_level(t *testing.T) {
	body := "## Scenario\n\nfirst\n\n## Implementation Plan\n\nsecond\n"

	got, ok := markdown.Section(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "first", got)
}

func Test_Section_ends_at_a_heading_of_higher_level(t *testing.T) {
	body := "## Scenario\n\nfirst\n\n# Next Title\n\nsecond\n"

	got, ok := markdown.Section(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "first", got)
}

func Test_Section_includes_a_deeper_subheading(t *testing.T) {
	body := "## Scenario\n\n### Given\n\nsetup\n\n## Implementation Plan\n\nother\n"

	got, ok := markdown.Section(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "### Given\n\nsetup", got)
}

func Test_Section_does_not_treat_a_hash_line_inside_a_fenced_block_as_a_heading(t *testing.T) {
	body := "## Scenario\n\n```gherkin\nScenario: demo\n  # a comment line\n  Given a thing\n```\n\n## Implementation Plan\n\nother\n"

	got, ok := markdown.Section(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "```gherkin\nScenario: demo\n  # a comment line\n  Given a thing\n```", got)
}

func Test_Section_returns_false_for_a_heading_not_present(t *testing.T) {
	body := "## Scenario\n\nbody\n"

	_, ok := markdown.Section(body, "## Missing")

	assert.False(t, ok)
}

func Test_Section_trims_leading_and_trailing_blank_lines(t *testing.T) {
	body := "## Scenario\n\n\nbody\n\n\n## Implementation Plan\n"

	got, ok := markdown.Section(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "body", got)
}

func Test_Title_returns_the_text_of_the_first_level_one_heading(t *testing.T) {
	body := "# STEP-03 Assemble the brief\n\n## Scenario\n\nbody\n"

	got, ok := markdown.Title(body)

	assert.True(t, ok)
	assert.Equal(t, "STEP-03 Assemble the brief", got)
}

func Test_Title_does_not_treat_a_hash_line_inside_a_fenced_block_as_a_heading(t *testing.T) {
	body := "```\n# not a title\n```\n\n# Real Title\n"

	got, ok := markdown.Title(body)

	assert.True(t, ok)
	assert.Equal(t, "Real Title", got)
}

func Test_Section_ends_at_a_heading_line_with_leading_whitespace(t *testing.T) {
	body := "## Scenario\n\nfirst\n\n  ## Implementation Plan\n\nsecond\n"

	got, ok := markdown.Section(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "first", got)
}

func Test_Section_does_not_treat_a_hash_line_indented_four_or_more_spaces_as_a_heading(t *testing.T) {
	body := "## Implementation Plan\n\n- [ ] Step 1: do a thing\n      # a continuation line\n- [ ] Step 2: do another\n\n## Handoff\n"

	got, ok := markdown.Section(body, "## Implementation Plan")

	assert.True(t, ok)
	assert.Equal(t, "- [ ] Step 1: do a thing\n      # a continuation line\n- [ ] Step 2: do another", got)
}

func Test_Title_returns_false_when_there_is_no_level_one_heading(t *testing.T) {
	body := "## Scenario\n\nbody\n"

	_, ok := markdown.Title(body)

	assert.False(t, ok)
}
