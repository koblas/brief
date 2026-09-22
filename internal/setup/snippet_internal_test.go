package setup

// White-box package: scanSnippetMarkers, mergeSnippet and removeSnippet are
// unexported byte-level decision logic (the R5 span rules pinned in
// SCENARIO-07's plan) whose case count — separator encoding, marker defect
// classification — is impractical to drive economically through the public
// Init/Uninstall surface for every combination; snippet_test.go covers the
// public surface, this file covers the extracted logic directly.

import (
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_mergeSnippet_encodes_the_separator_so_remove_can_undo_it pins the
// three append/remove round trips the byte rules require: no trailing
// newline, one trailing newline, and a trailing blank line all merge then
// remove back to byte-identical bytes — the discriminator that proves the
// separator is actually recoverable, not merely "looks right" for one case.
func Test_mergeSnippet_encodes_the_separator_so_remove_can_undo_it(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")

	cases := []struct {
		name     string
		existing string
	}{
		{name: "no trailing newline", existing: "foo"},
		{name: "one trailing newline", existing: "foo\n"},
		{name: "trailing blank line", existing: "foo\n\n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			merged := mergeSnippet([]byte(c.existing), nil, block)

			span, prob := scanSnippetMarkers(merged)
			require.Nil(t, prob)
			require.NotNil(t, span)

			restored := removeSnippet(merged, *span)

			assert.Equal(t, c.existing, string(restored))
		})
	}
}

// Test_mergeSnippet_appending_to_O_ending_in_newline pins the exact
// separator the byte rules specify for that one case: one "\n" before the
// block, one "\n" after it.
func Test_mergeSnippet_appending_to_O_ending_in_newline(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")

	merged := mergeSnippet([]byte("foo\n"), nil, block)

	assert.Equal(t, "foo\n\n"+string(block)+"\n", string(merged))
}

// Test_mergeSnippet_appending_to_O_without_a_trailing_newline pins the
// other case's own separator: a blank line before the block, nothing after
// it.
func Test_mergeSnippet_appending_to_O_without_a_trailing_newline(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")

	merged := mergeSnippet([]byte("foo"), nil, block)

	assert.Equal(t, "foo\n\n"+string(block), string(merged))
}

// Test_mergeSnippet_on_an_empty_or_missing_file_matches_create pins the
// "existing empty O" rule: nil (file did not exist) and an empty slice
// (file existed, zero bytes) both produce exactly block + "\n" — the same
// bytes Create produces.
func Test_mergeSnippet_on_an_empty_or_missing_file_matches_create(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")
	want := string(block) + "\n"

	cases := []struct {
		name     string
		existing []byte
	}{
		{name: "nil (file did not exist)", existing: nil},
		{name: "empty (file existed, zero bytes)", existing: []byte{}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, want, string(mergeSnippet(c.existing, nil, block)))
		})
	}
}

// Test_mergeSnippet_replaces_the_span_in_place pins the "replace" rule: a
// non-nil span is substituted with the new block, surrounding bytes
// untouched.
func Test_mergeSnippet_replaces_the_span_in_place(t *testing.T) {
	oldBlock := artifact.SnippetBlock("elsewhere")
	existing := "before\n\n" + string(oldBlock) + "\nafter"
	span, prob := scanSnippetMarkers([]byte(existing))
	require.Nil(t, prob)
	require.NotNil(t, span)

	newBlock := artifact.SnippetBlock("docs/specifications")
	merged := mergeSnippet([]byte(existing), span, newBlock)

	assert.Equal(t, "before\n\n"+string(newBlock)+"\nafter", string(merged))
}

// Test_removeSnippet_at_offset_zero_drops_the_span_and_its_own_newline pins
// the "span at offset 0" rule: nothing precedes the span, so there is no
// preceding "\n\n" to fold in — only the span and its own trailing newline
// (if any) are dropped.
func Test_removeSnippet_at_offset_zero_drops_the_span_and_its_own_newline(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")
	existing := string(block) + "\nafter"
	span, prob := scanSnippetMarkers([]byte(existing))
	require.Nil(t, prob)
	require.NotNil(t, span)

	restored := removeSnippet([]byte(existing), *span)

	assert.Equal(t, "after", string(restored))
}

// Test_removeSnippet_on_a_block_the_user_moved_drops_only_the_span pins the
// "any other position" rule: a span not preceded by a blank line (the user
// relocated it next to their own prose) drops the span and its own trailing
// newline only — the single preceding newline, and everything else, stays.
func Test_removeSnippet_on_a_block_the_user_moved_drops_only_the_span(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")
	existing := "before\n" + string(block) + "\nafter"
	span, prob := scanSnippetMarkers([]byte(existing))
	require.Nil(t, prob)
	require.NotNil(t, span)

	restored := removeSnippet([]byte(existing), *span)

	assert.Equal(t, "before\nafter", string(restored))
}

// Test_scanSnippetMarkers_finds_no_span_in_plain_text pins the Zero case:
// text with no marker line at all reports a nil span and no problem.
func Test_scanSnippetMarkers_finds_no_span_in_plain_text(t *testing.T) {
	span, prob := scanSnippetMarkers([]byte("just some prose\nnothing more\n"))

	assert.Nil(t, span)
	assert.Nil(t, prob)
}

// Test_scanSnippetMarkers_reports_a_marker_defect_with_its_line pins every
// named refusal shape (SCENARIO-07's plan Step 6) and the 1-based line
// number each reports.
func Test_scanSnippetMarkers_reports_a_marker_defect_with_its_line(t *testing.T) {
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
			span, prob := scanSnippetMarkers([]byte(c.body))

			assert.Nil(t, span)
			require.NotNil(t, prob)
			assert.Equal(t, c.wantLine, prob.line)
		})
	}
}

// Test_scanSnippetMarkers_finds_the_span_of_one_valid_block pins the
// happy-path span: begin marker's line through the end marker's own text,
// excluding its line terminator.
func Test_scanSnippetMarkers_finds_the_span_of_one_valid_block(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")
	body := "before\n\n" + string(block) + "\nafter"

	span, prob := scanSnippetMarkers([]byte(body))

	require.Nil(t, prob)
	require.NotNil(t, span)
	assert.Equal(t, string(block), string([]byte(body)[span.start:span.end]))
	assert.Equal(t, 3, span.beginLine)
}
