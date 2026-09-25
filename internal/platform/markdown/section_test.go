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

func Test_HeadingLine_returns_the_1_based_line_of_the_matching_line(t *testing.T) {
	body := "para\n\n## Heading\n\nbody\n"

	got, ok := markdown.HeadingLine(body, "## Heading")

	assert.True(t, ok)
	assert.Equal(t, 3, got)
}

func Test_HeadingLine_returns_false_when_the_heading_is_absent(t *testing.T) {
	_, ok := markdown.HeadingLine("no heading here\n", "## Missing")

	assert.False(t, ok)
}

func Test_HeadingLine_skips_a_matching_line_inside_a_fenced_code_block(t *testing.T) {
	body := "```\n## Heading\n```\n\n## Heading\n"

	got, ok := markdown.HeadingLine(body, "## Heading")

	assert.True(t, ok)
	assert.Equal(t, 5, got)
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

func Test_Section_returns_empty_string_for_an_empty_section(t *testing.T) {
	body := "## Scenario\n\n## Implementation Plan\n"

	got, ok := markdown.Section(body, "## Scenario")

	assert.True(t, ok)
	assert.Empty(t, got)
}

// Test_Section_returns_its_body_when_it_is_the_last_section_in_a_file
// pins Section against a body with no terminating heading at all — the
// section runs to end of file.
func Test_Section_returns_its_body_when_it_is_the_last_section_in_a_file(t *testing.T) {
	body := "## Handoff\n\nsome content\n"

	got, ok := markdown.Section(body, "## Handoff")

	assert.True(t, ok)
	assert.Equal(t, "some content", got)
}

// Test_Section_does_not_treat_a_fenced_heading_line_as_the_anchor pins the
// fence-aware anchor search: a heading-shaped line inside a fenced code
// block, before the real heading, is never mistaken for the anchor.
func Test_Section_does_not_treat_a_fenced_heading_line_as_the_anchor(t *testing.T) {
	body := "```\n## Scenario\n```\n\n## Scenario\n\nreal body\n"

	got, ok := markdown.Section(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "real body", got)
}

// Test_Section_a_backtick_fence_does_not_close_an_open_tilde_fence
// reproduces the reviewer's finding directly: a boolean inFence toggle
// closes a ~~~ block on any ``` run, so the "## Not a heading" line inside
// it is read as the real terminating heading and the section is truncated
// before "## Next".
func Test_Section_a_backtick_fence_does_not_close_an_open_tilde_fence(t *testing.T) {
	body := "## Handoff\n\nTraps:\n\n~~~\n```\n## Not a heading\n~~~\n\n## Next\n\nafter\n"

	got, ok := markdown.Section(body, "## Handoff")

	assert.True(t, ok)
	assert.Equal(t, "Traps:\n\n~~~\n```\n## Not a heading\n~~~", got)
}

// Test_Section_a_shorter_fence_run_does_not_close_a_longer_opening_fence
// mirrors SCENARIO-06's original spliceHandoff decoy: a four-backtick
// fence containing a three-backtick line must stay open across the
// shorter run, per CommonMark's rule that only a run at least as long as
// the opener, of the same character, closes a fence.
func Test_Section_a_shorter_fence_run_does_not_close_a_longer_opening_fence(t *testing.T) {
	body := "## Handoff\n\n````\n```\nstill fenced\n````\n\n## Next\n\nafter\n"

	got, ok := markdown.Section(body, "## Handoff")

	assert.True(t, ok)
	assert.Equal(t, "````\n```\nstill fenced\n````", got)
}

// Test_Section_a_closing_candidate_with_an_info_string_does_not_close_the_fence
// pins CommonMark §4.5: only an opening fence may carry an info string, so
// a line that looks like a closer but carries one — "```python" inside a
// block opened with "```" — is not a valid close and stays inside the
// fenced block as ordinary content.
func Test_Section_a_closing_candidate_with_an_info_string_does_not_close_the_fence(t *testing.T) {
	body := "## Handoff\n\n```\n## Not a heading\n```python\nstill fenced\n```\n\n## Next\n\nafter\n"

	got, ok := markdown.Section(body, "## Handoff")

	assert.True(t, ok)
	assert.Equal(t, "```\n## Not a heading\n```python\nstill fenced\n```", got)
}

// Test_Section_recognizes_a_fence_delimiter_indented_up_to_three_spaces
// reproduces the reviewer's second defect: fenceRe anchored at column 0
// treats an indented fence as ordinary content, so the "## not a heading"
// line inside it — which headingLevelOf already tolerates up to three
// leading spaces on — is read as the terminating heading.
func Test_Section_recognizes_a_fence_delimiter_indented_up_to_three_spaces(t *testing.T) {
	body := "## Scenario\n\n  ```\n  ## not a heading\n  ```\n\n## Implementation Plan\n\nafter\n"

	got, ok := markdown.Section(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "  ```\n  ## not a heading\n  ```", got)
}

// Test_Section_finds_a_heading_line_that_ends_in_a_carriage_return
// reproduces the CRLF finding: strings.Split(body, "\n") on a CRLF file
// leaves a trailing "\r" on every line, and findHeading's equality check
// right-trimmed only " \t" so it never matched the configured heading.
func Test_Section_finds_a_heading_line_that_ends_in_a_carriage_return(t *testing.T) {
	body := "## Scenario\r\n\r\nfirst\r\n\r\n## Implementation Plan\r\n"

	got, ok := markdown.Section(body, "## Scenario")

	assert.True(t, ok)
	assert.Contains(t, got, "first")
}

// Test_Section_returns_empty_string_when_the_anchor_has_no_trailing_newline
// pins the last-line-with-no-newline edge case: the anchor is the last
// line in body and carries no trailing newline at all.
func Test_Section_returns_empty_string_when_the_anchor_has_no_trailing_newline(t *testing.T) {
	body := "## Handoff"

	got, ok := markdown.Section(body, "## Handoff")

	assert.True(t, ok)
	assert.Empty(t, got)
}

// Test_Section_ends_at_a_terminating_heading_indented_exactly_three_spaces
// pins headingLevelOf's exact boundary: CommonMark tolerates up to three
// leading spaces before an ATX heading, so a comparison of "> 3" — never
// ">= 3" — is what keeps this indentation a heading.
func Test_Section_ends_at_a_terminating_heading_indented_exactly_three_spaces(t *testing.T) {
	body := "## Scenario\n\nfirst\n\n   ## Implementation Plan\n\nsecond\n"

	got, ok := markdown.Section(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "first", got)
}

// Test_Section_recognizes_a_fence_delimiter_indented_exactly_three_spaces
// pins fenceDelim's own "> 3" boundary: a fence delimiter opened at three
// leading spaces still opens the fence, so the zero-indent "## not a
// heading" line inside it is never read as the terminator. The decoy
// heading is deliberately at zero indent, not three, so this test can only
// pass by the fence itself opening — a heading-boundary mutation on the
// same line could otherwise mask a fence-boundary regression here.
func Test_Section_recognizes_a_fence_delimiter_indented_exactly_three_spaces(t *testing.T) {
	body := "## Scenario\n\n   ```\n## not a heading\n   ```\n\n## Implementation Plan\n\nafter\n"

	got, ok := markdown.Section(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "   ```\n## not a heading\n   ```", got)
}

// Test_Section_does_not_treat_a_fence_delimiter_indented_four_spaces_as_a_fence
// is the previous test's negative half: fenceDelim rejects a delimiter at
// four leading spaces, so it never opens a fence, leaving the real,
// zero-indent heading after it to terminate the section as usual.
func Test_Section_does_not_treat_a_fence_delimiter_indented_four_spaces_as_a_fence(t *testing.T) {
	body := "## Scenario\n\nfirst\n\n    ```\n\n## Implementation Plan\n\nafter\n"

	got, ok := markdown.Section(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "first\n\n    ```", got)
}

// Test_Section_uses_the_first_occurrence_when_a_heading_appears_twice pins
// findHeading's documented contract on a body with two identical headings:
// the first occurrence is the anchor and the second is read as its
// terminator, with no duplicate-heading error — a hand-edited STATE.md
// with two "## Traps" sections silently loses the second under exactly
// this rule, so a future change to that policy must turn this test red
// rather than pass unnoticed.
func Test_Section_uses_the_first_occurrence_when_a_heading_appears_twice(t *testing.T) {
	body := "## Traps\n\nfirst copy\n\n## Traps\n\nsecond copy\n"

	got, ok := markdown.Section(body, "## Traps")

	assert.True(t, ok)
	assert.Equal(t, "first copy", got)
}

func Test_UnterminatedFence_returns_false_when_every_fence_closes(t *testing.T) {
	body := "Repro:\n\n```bash\ngo test ./...\n```\n"

	_, _, ok := markdown.UnterminatedFence(body)

	assert.False(t, ok)
}

func Test_UnterminatedFence_returns_false_when_body_has_no_fence_at_all(t *testing.T) {
	body := "just some plain text\n"

	_, _, ok := markdown.UnterminatedFence(body)

	assert.False(t, ok)
}

// Test_UnterminatedFence_reports_the_line_and_delimiter_of_an_open_fence
// reproduces the reviewer's odd-fence-count repro directly: a single
// opening ``` with no matching close.
func Test_UnterminatedFence_reports_the_line_and_delimiter_of_an_open_fence(t *testing.T) {
	body := "Repro:\n\n```bash\ngo test ./...\n"

	line, delim, ok := markdown.UnterminatedFence(body)

	assert.True(t, ok)
	assert.Equal(t, 3, line)
	assert.Equal(t, "```", delim)
}

// Test_UnterminatedFence_treats_a_closing_delimiter_indented_four_spaces_as_unterminated
// reproduces the reviewer's second repro shape: a closing fence indented
// four spaces is not a fence delimiter at all per CommonMark's three-space
// tolerance, so the opening fence is left open to end of file.
func Test_UnterminatedFence_treats_a_closing_delimiter_indented_four_spaces_as_unterminated(t *testing.T) {
	body := "```\nbody\n    ```\n"

	line, delim, ok := markdown.UnterminatedFence(body)

	assert.True(t, ok)
	assert.Equal(t, 1, line)
	assert.Equal(t, "```", delim)
}

func Test_UnterminatedFence_returns_false_when_a_shorter_run_reopens_inside_a_longer_fence(t *testing.T) {
	body := "````\n```\nstill fenced\n````\n"

	_, _, ok := markdown.UnterminatedFence(body)

	assert.False(t, ok)
}
