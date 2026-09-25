package setup

// mergeSnippet and removeSnippet are unexported byte-level logic whose case
// count is impractical to drive through the public Init/Uninstall surface;
// snippet_test.go covers that surface, this file covers the logic directly.

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func Test_mergeSnippet_appending_to_O_ending_in_newline(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")

	merged := mergeSnippet([]byte("foo\n"), nil, block)

	assert.Equal(t, "foo\n\n"+string(block)+"\n", string(merged))
}

func Test_mergeSnippet_appending_to_O_without_a_trailing_newline(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")

	merged := mergeSnippet([]byte("foo"), nil, block)

	assert.Equal(t, "foo\n\n"+string(block), string(merged))
}

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

func Test_removeSnippet_at_offset_zero_drops_the_span_and_its_own_newline(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")
	existing := string(block) + "\nafter"
	rawSpan, prob := artifact.ScanSnippetMarkers([]byte(existing))
	require.Nil(t, prob)
	require.NotNil(t, rawSpan)

	restored := removeSnippet([]byte(existing), *toSnippetSpan(rawSpan))

	assert.Equal(t, "after", string(restored))
}

func Test_removeSnippet_on_a_block_the_user_moved_drops_only_the_span(t *testing.T) {
	block := artifact.SnippetBlock("docs/specifications")
	existing := "before\n" + string(block) + "\nafter"
	rawSpan, prob := artifact.ScanSnippetMarkers([]byte(existing))
	require.Nil(t, prob)
	require.NotNil(t, rawSpan)

	restored := removeSnippet([]byte(existing), *toSnippetSpan(rawSpan))

	assert.Equal(t, "before\nafter", string(restored))
}

// This is the read-modify-write guard apply and applyUninstall both run
// immediately before touching CLAUDE.md.
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
