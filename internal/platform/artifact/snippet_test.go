package artifact_test

import (
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_snippet_block_names_the_feature_directory(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")

	s := string(block)
	assert.True(t, strings.HasPrefix(s, artifact.SnippetBegin+"\n"), "must open with the begin marker")
	assert.True(t, strings.HasSuffix(s, "\n"+artifact.SnippetEnd), "must close with the end marker")
	assert.False(t, strings.HasSuffix(s, "\n"), "must carry no trailing newline")
	assert.Contains(t, s, "`docs/specifications/`")
}

func Test_snippet_block_trims_a_trailing_slash_to_one(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications/")

	assert.Contains(t, string(block), "`docs/specifications/`")
	assert.NotContains(t, string(block), "//")
}

func Test_recognize_snippet_reports_current_for_its_own_dir(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")

	match := artifact.RecognizeSnippet(block)

	assert.Equal(t, artifact.OriginCurrent, match.Origin)
	assert.Equal(t, "docs/specifications", match.Dir)
}

func Test_recognize_snippet_reports_current_for_a_different_dir(t *testing.T) {
	block := artifact.SnippetBlock("elsewhere")

	match := artifact.RecognizeSnippet(block)

	assert.Equal(t, artifact.OriginCurrent, match.Origin)
	assert.Equal(t, "elsewhere", match.Dir)
}

func Test_recognize_snippet_reports_edited_for_unrecognized_bytes(t *testing.T) {
	match := artifact.RecognizeSnippet([]byte("not a brief block at all"))

	assert.Equal(t, artifact.OriginEdited, match.Origin)
	assert.Empty(t, match.Dir)
}

func Test_recognize_snippet_does_not_recognize_a_crlf_normalized_block(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")
	crlf := strings.ReplaceAll(string(block), "\n", "\r\n")

	match := artifact.RecognizeSnippet([]byte(crlf))

	assert.Equal(t, artifact.OriginEdited, match.Origin)
}

func Test_snippet_kind_is_off_render_and_recognize(t *testing.T) {
	require.Nil(t, artifact.Render(artifact.KindSnippet))
	assert.Equal(t, artifact.OriginEdited, artifact.Recognize(artifact.KindSnippet, artifact.SnippetBlock("docs/specifications")))
}
