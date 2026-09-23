package config_test

import (
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fsAbs joins slash-separated segments under "/", the way every
// LocateWithinFS test names an absolute path its fstest.MapFS fixture is
// keyed against (fsName strips the leading "/" to get the fs.FS-relative
// name back).
func fsAbs(elem ...string) string {
	return filepath.FromSlash("/" + filepath.ToSlash(filepath.Join(elem...)))
}

// Test_locate_within_fs_nearest_and_shadowed pins LocateWithinFS's own
// nearest-wins and shadowed-ancestor guards: the nearest ".brief.yaml"
// wins, every farther ancestor config is reported as shadowed
// (nearest-first), and a repository with none reports an empty nearest and
// no shadowed ancestors.
func Test_locate_within_fs_nearest_and_shadowed(t *testing.T) {
	t.Run("the nearest of two configs wins, the farther one is shadowed", func(t *testing.T) {
		fsys := fstest.MapFS{
			"repo/.brief.yaml":      &fstest.MapFile{Data: []byte("progress-heading: \"## Root\"\n")},
			"repo/near/.brief.yaml": &fstest.MapFile{Data: []byte("progress-heading: \"## Near\"\n")},
		}

		nearest, shadowed, err := config.LocateWithinFS(fsys, fsAbs("repo", "near"), "")

		require.NoError(t, err)
		assert.Equal(t, fsAbs("repo", "near", ".brief.yaml"), nearest)
		assert.Equal(t, []string{fsAbs("repo", ".brief.yaml")}, shadowed)
	})

	t.Run("no config anywhere reports an empty nearest and no shadowed ancestors", func(t *testing.T) {
		fsys := fstest.MapFS{
			"repo/a/b/README.md": &fstest.MapFile{Data: []byte("x")},
		}

		nearest, shadowed, err := config.LocateWithinFS(fsys, fsAbs("repo", "a", "b"), "")

		require.NoError(t, err)
		assert.Empty(t, nearest)
		assert.Empty(t, shadowed)
	})
}

// Test_locate_within_fs_missing_start_dir pins the missing-startDir guard:
// a startAbs fs.Stat cannot find refuses as *config.InvalidConfigError,
// naming startAbs itself (not the fs-relative name fs.Stat saw).
func Test_locate_within_fs_missing_start_dir(t *testing.T) {
	fsys := fstest.MapFS{
		"repo/.brief.yaml": &fstest.MapFile{Data: []byte("x")},
	}

	_, _, err := config.LocateWithinFS(fsys, fsAbs("repo", "does-not-exist"), "")

	require.ErrorIs(t, err, config.ErrInvalidConfig)

	var target *config.InvalidConfigError
	require.ErrorAs(t, err, &target)
	assert.Equal(t, fsAbs("repo", "does-not-exist"), target.Path)
}

// Test_locate_within_fs_walk_terminates_at_root pins the walk's own
// termination: a config file sitting exactly at "/" (fsName's own "."
// mapping) is still found from several levels below.
func Test_locate_within_fs_walk_terminates_at_root(t *testing.T) {
	fsys := fstest.MapFS{
		"a/b/marker.txt": &fstest.MapFile{Data: []byte("x")},
		".brief.yaml":    &fstest.MapFile{Data: []byte("progress-heading: \"## Root\"\n")},
	}

	nearest, shadowed, err := config.LocateWithinFS(fsys, fsAbs("a", "b"), "")

	require.NoError(t, err)
	assert.Equal(t, fsAbs(".brief.yaml"), nearest)
	assert.Empty(t, shadowed)
}

// Test_locate_within_fs_stops_at_boundary pins the boundary guard: a
// boundary directory that is walked but never exceeded — a config above it
// is never found, one at or below it still is, and an empty boundary is
// unbounded.
func Test_locate_within_fs_stops_at_boundary(t *testing.T) {
	t.Run("a config above the boundary is not found", func(t *testing.T) {
		fsys := fstest.MapFS{
			".brief.yaml":         &fstest.MapFile{Data: []byte("x")},
			"repo/sub/marker.txt": &fstest.MapFile{Data: []byte("x")},
		}

		nearest, shadowed, err := config.LocateWithinFS(fsys, fsAbs("repo", "sub"), fsAbs("repo"))

		require.NoError(t, err)
		assert.Empty(t, nearest)
		assert.Empty(t, shadowed)
	})

	t.Run("a config at the boundary is found", func(t *testing.T) {
		fsys := fstest.MapFS{
			"boundary/.brief.yaml":    &fstest.MapFile{Data: []byte("x")},
			"boundary/sub/marker.txt": &fstest.MapFile{Data: []byte("x")},
		}

		nearest, shadowed, err := config.LocateWithinFS(fsys, fsAbs("boundary", "sub"), fsAbs("boundary"))

		require.NoError(t, err)
		assert.Equal(t, fsAbs("boundary", ".brief.yaml"), nearest)
		assert.Empty(t, shadowed)
	})

	t.Run("an empty boundary is unbounded, identical to no boundary", func(t *testing.T) {
		fsys := fstest.MapFS{
			".brief.yaml":    &fstest.MapFile{Data: []byte("x")},
			"a/b/marker.txt": &fstest.MapFile{Data: []byte("x")},
		}

		nearest, shadowed, err := config.LocateWithinFS(fsys, fsAbs("a", "b"), "")

		require.NoError(t, err)
		assert.Equal(t, fsAbs(".brief.yaml"), nearest)
		assert.Empty(t, shadowed)
	})
}
