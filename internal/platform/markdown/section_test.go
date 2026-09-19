package markdown_test

import (
	"strings"
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

func Test_SectionRange_returns_the_offsets_of_an_empty_section(t *testing.T) {
	body := "## Scenario\n\n## Implementation Plan\n"

	start, end, ok := markdown.SectionRange(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "\n", body[start:end])
}

func Test_SectionRange_returns_the_offsets_of_a_section_followed_by_a_same_level_heading(t *testing.T) {
	body := "## Scenario\n\nfirst\n\n## Implementation Plan\n\nsecond\n"

	start, end, ok := markdown.SectionRange(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "\nfirst\n\n", body[start:end])
	assert.True(t, strings.HasPrefix(body[end:], "## Implementation Plan"))
}

func Test_SectionRange_returns_the_offsets_of_the_last_section_in_a_file(t *testing.T) {
	body := "## Handoff\n\nsome content\n"

	start, end, ok := markdown.SectionRange(body, "## Handoff")

	assert.True(t, ok)
	assert.Equal(t, len(body), end)
	assert.Equal(t, "\nsome content\n", body[start:end])
}

func Test_SectionRange_does_not_treat_a_fenced_heading_line_as_the_anchor(t *testing.T) {
	body := "```\n## Scenario\n```\n\n## Scenario\n\nreal body\n"

	start, end, ok := markdown.SectionRange(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "\nreal body\n", body[start:end])
}

// Test_SectionRange_a_backtick_fence_does_not_close_an_open_tilde_fence
// reproduces the reviewer's finding directly: a boolean inFence toggle
// closes a ~~~ block on any ``` run, so the "## Not a heading" line inside
// it is read as the real terminating heading and the section is truncated
// before "## Next".
func Test_SectionRange_a_backtick_fence_does_not_close_an_open_tilde_fence(t *testing.T) {
	body := "## Handoff\n\nTraps:\n\n~~~\n```\n## Not a heading\n~~~\n\n## Next\n\nafter\n"

	start, end, ok := markdown.SectionRange(body, "## Handoff")

	assert.True(t, ok)
	assert.Equal(t, "\nTraps:\n\n~~~\n```\n## Not a heading\n~~~\n\n", body[start:end])
	assert.True(t, strings.HasPrefix(body[end:], "## Next"))
}

// Test_SectionRange_a_shorter_fence_run_does_not_close_a_longer_opening_fence
// mirrors SCENARIO-06's spliceHandoff decoy: a four-backtick fence
// containing a three-backtick line must stay open across the shorter run,
// per CommonMark's rule that only a run at least as long as the opener,
// of the same character, closes a fence.
func Test_SectionRange_a_shorter_fence_run_does_not_close_a_longer_opening_fence(t *testing.T) {
	body := "## Handoff\n\n````\n```\nstill fenced\n````\n\n## Next\n\nafter\n"

	start, end, ok := markdown.SectionRange(body, "## Handoff")

	assert.True(t, ok)
	assert.Equal(t, "\n````\n```\nstill fenced\n````\n\n", body[start:end])
	assert.True(t, strings.HasPrefix(body[end:], "## Next"))
}

// Test_SectionRange_a_closing_candidate_with_an_info_string_does_not_close_the_fence
// pins CommonMark §4.5: only an opening fence may carry an info string, so
// a line that looks like a closer but carries one — "```python" inside a
// block opened with "```" — is not a valid close and stays inside the
// fenced block as ordinary content.
func Test_SectionRange_a_closing_candidate_with_an_info_string_does_not_close_the_fence(t *testing.T) {
	body := "## Handoff\n\n```\n## Not a heading\n```python\nstill fenced\n```\n\n## Next\n\nafter\n"

	start, end, ok := markdown.SectionRange(body, "## Handoff")

	assert.True(t, ok)
	assert.Equal(t, "\n```\n## Not a heading\n```python\nstill fenced\n```\n\n", body[start:end])
	assert.True(t, strings.HasPrefix(body[end:], "## Next"))
}

// Test_SectionRange_recognizes_a_fence_delimiter_indented_up_to_three_spaces
// reproduces the reviewer's second defect: fenceRe anchored at column 0
// treats an indented fence as ordinary content, so the "## not a heading"
// line inside it — which headingLevelOf already tolerates up to three
// leading spaces on — is read as the terminating heading.
func Test_SectionRange_recognizes_a_fence_delimiter_indented_up_to_three_spaces(t *testing.T) {
	body := "## Scenario\n\n  ```\n  ## not a heading\n  ```\n\n## Implementation Plan\n\nafter\n"

	start, end, ok := markdown.SectionRange(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "\n  ```\n  ## not a heading\n  ```\n\n", body[start:end])
	assert.True(t, strings.HasPrefix(body[end:], "## Implementation Plan"))
}

// Test_SectionRange_finds_a_heading_line_that_ends_in_a_carriage_return
// reproduces the CRLF finding: strings.Split(body, "\n") on a CRLF file
// leaves a trailing "\r" on every line, and findHeading's equality check
// right-trimmed only " \t" so it never matched the configured heading.
func Test_SectionRange_finds_a_heading_line_that_ends_in_a_carriage_return(t *testing.T) {
	body := "## Scenario\r\n\r\nfirst\r\n\r\n## Implementation Plan\r\n"

	start, end, ok := markdown.SectionRange(body, "## Scenario")

	assert.True(t, ok)
	assert.Contains(t, body[start:end], "first")
}

func Test_SectionRange_returns_false_for_a_heading_not_present(t *testing.T) {
	body := "## Scenario\n\nbody\n"

	_, _, ok := markdown.SectionRange(body, "## Missing")

	assert.False(t, ok)
}

func Test_SectionRange_ends_at_the_start_of_an_indented_terminating_heading_line(t *testing.T) {
	body := "## Scenario\n\nfirst\n\n  ## Implementation Plan\n\nsecond\n"

	start, end, ok := markdown.SectionRange(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "\nfirst\n\n", body[start:end])
	assert.True(t, strings.HasPrefix(body[end:], "  ## Implementation Plan"),
		"end must point at the start of the terminating line, including its leading whitespace")
}

func Test_SectionRange_treats_an_anchor_with_no_trailing_newline_as_ending_at_the_body_length(t *testing.T) {
	body := "## Handoff"

	start, end, ok := markdown.SectionRange(body, "## Handoff")

	assert.True(t, ok)
	assert.Equal(t, len(body), start)
	assert.Equal(t, len(body), end)
}

// Test_SectionRange_ends_at_a_terminating_heading_indented_exactly_three_spaces
// pins headingLevelOf's exact boundary: CommonMark tolerates up to three
// leading spaces before an ATX heading, so a comparison of "> 3" — never
// ">= 3" — is what keeps this indentation a heading.
func Test_SectionRange_ends_at_a_terminating_heading_indented_exactly_three_spaces(t *testing.T) {
	body := "## Scenario\n\nfirst\n\n   ## Implementation Plan\n\nsecond\n"

	start, end, ok := markdown.SectionRange(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "\nfirst\n\n", body[start:end])
	assert.True(t, strings.HasPrefix(body[end:], "   ## Implementation Plan"))
}

// Test_SectionRange_does_not_end_at_a_terminating_heading_indented_four_spaces
// is the previous test's negative half: one more leading space makes the
// line an indented code block per CommonMark, not a heading, so it never
// terminates the section.
func Test_SectionRange_does_not_end_at_a_terminating_heading_indented_four_spaces(t *testing.T) {
	body := "## Scenario\n\nfirst\n\n    ## Implementation Plan\n\nsecond\n"

	start, end, ok := markdown.SectionRange(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, len(body), end)
	assert.Contains(t, body[start:end], "## Implementation Plan")
}

// Test_SectionRange_recognizes_a_fence_delimiter_indented_exactly_three_spaces
// pins fenceDelim's own "> 3" boundary: a fence delimiter opened at three
// leading spaces still opens the fence, so the zero-indent "## not a
// heading" line inside it is never read as the terminator. The decoy
// heading is deliberately at zero indent, not three, so this test can only
// pass by the fence itself opening — a heading-boundary mutation on the
// same line could otherwise mask a fence-boundary regression here.
func Test_SectionRange_recognizes_a_fence_delimiter_indented_exactly_three_spaces(t *testing.T) {
	body := "## Scenario\n\n   ```\n## not a heading\n   ```\n\n## Implementation Plan\n\nafter\n"

	start, end, ok := markdown.SectionRange(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "\n   ```\n## not a heading\n   ```\n\n", body[start:end])
	assert.True(t, strings.HasPrefix(body[end:], "## Implementation Plan"))
}

// Test_SectionRange_does_not_treat_a_fence_delimiter_indented_four_spaces_as_a_fence
// is the previous test's negative half: fenceDelim rejects a delimiter at
// four leading spaces, so it never opens a fence, leaving the real,
// zero-indent heading after it to terminate the section as usual.
func Test_SectionRange_does_not_treat_a_fence_delimiter_indented_four_spaces_as_a_fence(t *testing.T) {
	body := "## Scenario\n\nfirst\n\n    ```\n\n## Implementation Plan\n\nafter\n"

	start, end, ok := markdown.SectionRange(body, "## Scenario")

	assert.True(t, ok)
	assert.Equal(t, "\nfirst\n\n    ```\n\n", body[start:end])
	assert.True(t, strings.HasPrefix(body[end:], "## Implementation Plan"))
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

func Test_HeadingStart_returns_the_offset_just_past_the_heading_line(t *testing.T) {
	body := "## Handoff\n\nbody\n"

	start, ok := markdown.HeadingStart(body, "## Handoff")

	assert.True(t, ok)
	assert.Equal(t, "\nbody\n", body[start:])
}

func Test_HeadingStart_returns_false_when_the_heading_is_absent(t *testing.T) {
	body := "## Scenario\n\nbody\n"

	_, ok := markdown.HeadingStart(body, "## Handoff")

	assert.False(t, ok)
}

// Test_HeadingStart_never_anchors_on_a_fenced_decoy pins the same
// fence-aware anchor search SectionRange uses: a heading-shaped line
// inside a fenced code block is never the anchor.
func Test_HeadingStart_never_anchors_on_a_fenced_decoy(t *testing.T) {
	body := "```\n## Handoff\n```\n\n## Handoff\n\nreal body\n"

	start, ok := markdown.HeadingStart(body, "## Handoff")

	assert.True(t, ok)
	assert.Equal(t, "\nreal body\n", body[start:])
}

func Test_TrailingHeading_returns_false_when_the_heading_is_last(t *testing.T) {
	body := "## Scenario\n\nfirst\n\n## Handoff\n\nlast section, nothing follows\n"

	_, _, found := markdown.TrailingHeading(body, "## Handoff")

	assert.False(t, found)
}

func Test_TrailingHeading_returns_the_first_heading_that_follows(t *testing.T) {
	body := "## Handoff\n\nbody\n\n## Notes\n\nkeep this\n"

	line, lineNo, found := markdown.TrailingHeading(body, "## Handoff")

	assert.True(t, found)
	assert.Equal(t, "## Notes", line)
	assert.Equal(t, 5, lineNo)
}

// Test_TrailingHeading_reports_a_trailing_heading_of_a_different_level
// pins the "any level" contract: content under a "### " subheading after
// the anchor is just as much at risk as content under a "## " one, so a
// level-restricted check would miss it.
func Test_TrailingHeading_reports_a_trailing_heading_of_a_different_level(t *testing.T) {
	body := "## Handoff\n\nbody\n\n### Deeper\n\nstill at risk\n"

	line, _, found := markdown.TrailingHeading(body, "## Handoff")

	assert.True(t, found)
	assert.Equal(t, "### Deeper", line)
}

// Test_TrailingHeading_never_reports_a_fenced_decoy_as_the_successor pins
// the same fence-aware scan SectionRange's terminator search uses: a
// heading-shaped line inside a fenced code block after the anchor is not a
// trailing heading.
func Test_TrailingHeading_never_reports_a_fenced_decoy_as_the_successor(t *testing.T) {
	body := "## Handoff\n\n```\n## Not a heading\n```\n"

	_, _, found := markdown.TrailingHeading(body, "## Handoff")

	assert.False(t, found)
}

func Test_TrailingHeading_returns_false_when_the_heading_itself_is_absent(t *testing.T) {
	body := "## Scenario\n\nbody\n"

	_, _, found := markdown.TrailingHeading(body, "## Handoff")

	assert.False(t, found)
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
