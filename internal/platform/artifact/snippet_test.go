package artifact_test

import (
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_snippet_block_names_the_feature_directory pins SnippetBlock's own
// shape (R5): the block opens with the begin marker and closes with the end
// marker on its own line, no trailing newline, and names dir once, with any
// trailing slash trimmed so the rendered text reads the directory with
// exactly one slash.
func Test_snippet_block_names_the_feature_directory(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")

	s := string(block)
	assert.True(t, strings.HasPrefix(s, artifact.SnippetBegin+"\n"), "must open with the begin marker")
	assert.True(t, strings.HasSuffix(s, "\n"+artifact.SnippetEnd), "must close with the end marker")
	assert.False(t, strings.HasSuffix(s, "\n"), "must carry no trailing newline")
	assert.Contains(t, s, "`docs/specifications/`")
}

// Test_snippet_block_trims_a_trailing_slash_to_one pins the trailing-slash
// rule: a dir already ending in "/" still renders exactly one slash, never
// "//".
func Test_snippet_block_trims_a_trailing_slash_to_one(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications/")

	assert.Contains(t, string(block), "`docs/specifications/`")
	assert.NotContains(t, string(block), "//")
}

// Test_recognize_snippet_reports_current_for_its_own_dir pins the happy
// path: a block SnippetBlock rendered for one dir is recognized current for
// that same dir.
func Test_recognize_snippet_reports_current_for_its_own_dir(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")

	match := artifact.RecognizeSnippet(block)

	assert.Equal(t, artifact.OriginCurrent, match.Origin)
	assert.Equal(t, "docs/specifications", match.Dir)
}

// Test_recognize_snippet_reports_current_for_a_different_dir pins the
// binding decision that recognition is config-independent: a block rendered
// for a dir other than the one under test is still OriginCurrent — it is
// brief-written, just for a different feature directory — and Dir reports
// the dir actually embedded in the block, letting the caller decide whether
// that counts as "unchanged" or "needs updating".
func Test_recognize_snippet_reports_current_for_a_different_dir(t *testing.T) {
	block := artifact.SnippetBlock("elsewhere")

	match := artifact.RecognizeSnippet(block)

	assert.Equal(t, artifact.OriginCurrent, match.Origin)
	assert.Equal(t, "elsewhere", match.Dir)
}

// Test_recognize_snippet_reports_edited_for_unrecognized_bytes pins the
// "edited locally" branch: bytes matching no known snippet template report
// OriginEdited and an empty Dir.
func Test_recognize_snippet_reports_edited_for_unrecognized_bytes(t *testing.T) {
	match := artifact.RecognizeSnippet([]byte("not a brief block at all"))

	assert.Equal(t, artifact.OriginEdited, match.Origin)
	assert.Empty(t, match.Dir)
}

// Test_recognize_snippet_does_not_recognize_a_crlf_normalized_block pins
// that written and recognized bytes stay one value: taking a correctly
// rendered block and converting its line endings to CRLF makes it
// unrecognizable — RecognizeSnippet never normalizes line endings on the
// caller's behalf, so a CRLF copy is edited, not current.
func Test_recognize_snippet_does_not_recognize_a_crlf_normalized_block(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")
	crlf := strings.ReplaceAll(string(block), "\n", "\r\n")

	match := artifact.RecognizeSnippet([]byte(crlf))

	assert.Equal(t, artifact.OriginEdited, match.Origin)
}

// Test_snippet_kind_is_off_render_and_recognize documents that KindSnippet
// is deliberately excluded from Render and Recognize: unlike every other
// Kind, a snippet's render takes a parameter (the feature directory), so
// Render(KindSnippet) — which carries no way to pass one — returns nil, and
// Recognize(KindSnippet, ...) — which would need a per-dir digest list — always
// reports OriginEdited. SnippetBlock and RecognizeSnippet are the snippet's
// own API instead.
func Test_snippet_kind_is_off_render_and_recognize(t *testing.T) {
	require.Nil(t, artifact.Render(artifact.KindSnippet))
	assert.Equal(t, artifact.OriginEdited, artifact.Recognize(artifact.KindSnippet, artifact.SnippetBlock("docs/specifications")))
}
