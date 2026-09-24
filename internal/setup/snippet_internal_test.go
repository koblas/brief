package setup

// White-box package: mergeSnippet and removeSnippet are unexported
// byte-level decision logic (the R5 separator encoding) whose case count is
// impractical to drive economically through the public Init/Uninstall
// surface for every combination; snippet_test.go covers the public surface,
// this file covers the extracted logic directly. Marker scanning itself is
// artifact.ScanSnippetMarkers, tested in internal/platform/artifact.

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/rwfs"
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

			rawSpan, prob := artifact.ScanSnippetMarkers(merged)
			require.Nil(t, prob)
			require.NotNil(t, rawSpan)

			restored := removeSnippet(merged, *toSnippetSpan(rawSpan))

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
	rawSpan, prob := artifact.ScanSnippetMarkers([]byte(existing))
	require.Nil(t, prob)
	require.NotNil(t, rawSpan)

	newBlock := artifact.SnippetBlock("docs/specifications")
	merged := mergeSnippet([]byte(existing), toSnippetSpan(rawSpan), newBlock)

	assert.Equal(t, "before\n\n"+string(newBlock)+"\nafter", string(merged))
}

// Test_removeSnippet_at_offset_zero_drops_the_span_and_its_own_newline pins
// the "span at offset 0" rule: nothing precedes the span, so there is no
// preceding "\n\n" to fold in — only the span and its own trailing newline
// (if any) are dropped.
func Test_removeSnippet_at_offset_zero_drops_the_span_and_its_own_newline(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")
	existing := string(block) + "\nafter"
	rawSpan, prob := artifact.ScanSnippetMarkers([]byte(existing))
	require.Nil(t, prob)
	require.NotNil(t, rawSpan)

	restored := removeSnippet([]byte(existing), *toSnippetSpan(rawSpan))

	assert.Equal(t, "after", string(restored))
}

// Test_removeSnippet_on_a_block_the_user_moved_drops_only_the_span pins the
// "any other position" rule: a span not preceded by a blank line (the user
// relocated it next to their own prose) drops the span and its own trailing
// newline only — the single preceding newline, and everything else, stays.
func Test_removeSnippet_on_a_block_the_user_moved_drops_only_the_span(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")
	existing := "before\n" + string(block) + "\nafter"
	rawSpan, prob := artifact.ScanSnippetMarkers([]byte(existing))
	require.Nil(t, prob)
	require.NotNil(t, rawSpan)

	restored := removeSnippet([]byte(existing), *toSnippetSpan(rawSpan))

	assert.Equal(t, "before\nafter", string(restored))
}

// Test_verifyFileUnchanged pins the read-modify-write guard apply and
// applyUninstall both run immediately before touching CLAUDE.md: nil, the
// happy path, when the file's current bytes still match what planning
// read (or, for a fresh create, the file is still absent); a
// *RefusalError wrapping ErrConcurrentEdit, naming rerunCommand in its own
// Fix, for every other combination planning could not have foreseen — the
// file's bytes changed, it now exists when planning found nothing, or it
// no longer exists at all.
func Test_verifyFileUnchanged(t *testing.T) {
	path := "/repo/CLAUDE.md"

	t.Run("unchanged existing bytes is nil", func(t *testing.T) {
		mem := rwfs.NewMem(fstest.MapFS{"repo/CLAUDE.md": &fstest.MapFile{Data: []byte("stable")}})

		err := verifyFileUnchanged(mem, path, true, []byte("stable"), "brief init")

		assert.NoError(t, err)
	})

	t.Run("still absent when planning found nothing is nil", func(t *testing.T) {
		mem := rwfs.NewMem(fstest.MapFS{})

		err := verifyFileUnchanged(mem, path, false, nil, "brief init")

		assert.NoError(t, err)
	})

	t.Run("bytes changed since planning refuses", func(t *testing.T) {
		mem := rwfs.NewMem(fstest.MapFS{"repo/CLAUDE.md": &fstest.MapFile{Data: []byte("edited by someone else")}})

		err := verifyFileUnchanged(mem, path, true, []byte("stable"), "brief init")

		require.ErrorIs(t, err, ErrConcurrentEdit)

		var refusal *RefusalError
		require.ErrorAs(t, err, &refusal)
		assert.Equal(t, path, refusal.Path)
		assert.Equal(t, "rerun 'brief init'", refusal.Fix)
	})

	t.Run("created out from under planning refuses", func(t *testing.T) {
		mem := rwfs.NewMem(fstest.MapFS{"repo/CLAUDE.md": &fstest.MapFile{Data: []byte("raced into existence")}})

		err := verifyFileUnchanged(mem, path, false, nil, "brief uninstall")

		require.ErrorIs(t, err, ErrConcurrentEdit)
	})

	t.Run("removed out from under planning refuses", func(t *testing.T) {
		mem := rwfs.NewMem(fstest.MapFS{})

		err := verifyFileUnchanged(mem, path, true, []byte("stable"), "brief uninstall")

		require.ErrorIs(t, err, ErrConcurrentEdit)
	})

	t.Run("an unrelated read failure is returned unwrapped", func(t *testing.T) {
		mem := rwfs.NewMem(fstest.MapFS{"repo/CLAUDE.md": &fstest.MapFile{Mode: fs.ModeDir | 0o755}})

		err := verifyFileUnchanged(mem, path, true, []byte("stable"), "brief init")

		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrConcurrentEdit)
	})
}
