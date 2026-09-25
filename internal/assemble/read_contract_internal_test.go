package assemble

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/platform/rwfs/rwfstest"
	"github.com/stretchr/testify/require"
)

// dirOpenerAdapter adapts dirFS (fs.go) to rwfstest.DirOpener: Go does not
// consider two distinct named interface types interchangeable in a
// method's return position.
type dirOpenerAdapter struct{ d dirFS }

func (a dirOpenerAdapter) FS() fs.FS { return a.d.FS() }

func (a dirOpenerAdapter) Lstat(name string) (fs.FileInfo, error) { return a.d.Lstat(name) }

func (a dirOpenerAdapter) OpenRoot(name string) (rwfstest.DirOpener, error) {
	sub, err := a.d.OpenRoot(name)
	if err != nil {
		return nil, err
	}

	return dirOpenerAdapter{d: sub}, nil
}

// seedFixtureTree lists the entries every fixture in this file seeds,
// relative to the directory openFeatureDir opens.
var seedFixtureTree = map[string]string{
	"a.txt":         "hello",
	"list/b.txt":    "x",
	"list/a.txt":    "x",
	"sub/a.txt":     "nested",
	"sub/a.txt.bak": "unused", // ensures "sub" is not pruned to a single-entry edge case
}

// newOSRootFixture builds osRoot through (*Server).openFeatureDir with no
// WithFS option, over a real temp directory tree.
func newOSRootFixture(t *testing.T) rwfstest.DirOpener {
	t.Helper()

	dir := t.TempDir()
	for name, body := range seedFixtureTree {
		full := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(body), 0o600))
	}

	s := NewServer(config.Default(), dir)
	d, err := s.openFeatureDir(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	return dirOpenerAdapter{d: d}
}

// memFixtureDir is the multi-segment name openFeatureDir's own fsName
// mapping produces from an absolute OS directory.
const memFixtureDir = "repo/docs/specifications"

// newMemDirFixture builds memDirFS through (*Server).openFeatureDir over
// an rwfs.Mem, opened at memFixtureDir.
func newMemDirFixture(t *testing.T) rwfstest.DirOpener {
	t.Helper()

	seed := fstest.MapFS{}
	for name, body := range seedFixtureTree {
		seed[memFixtureDir+"/"+name] = &fstest.MapFile{Data: []byte(body), Mode: 0o600}
	}
	mem := rwfs.NewMem(seed)

	s := NewServer(config.Default(), "/"+memFixtureDir, WithFS(mem))
	d, err := s.openFeatureDir("/" + memFixtureDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	return dirOpenerAdapter{d: d}
}

// Both fixtures are built through the real openFeatureDir entry point
// Start, Check and Status all use.
func Test_osRoot_and_memDirFS_share_the_read_contract(t *testing.T) {
	t.Run("osRoot", func(t *testing.T) {
		rwfstest.ReadContract(t, func(t *testing.T) fs.FS {
			t.Helper()

			return newOSRootFixture(t).FS()
		})
		rwfstest.ReadOpenRootContract(t, newOSRootFixture,
			rwfstest.OpenRootNotDirUnclassified(
				"osRoot.OpenRoot (fs.go) calls (*os.Root).OpenRoot directly, unlike rwfs.OS's own OpenRoot, which "+
					"runs the raw failure through classifyOpenRootErr first; confirmed on go1.27.1/darwin that the "+
					"raw call reports a bare, unwrapped \"not a directory\" string rather than syscall.ENOTDIR when "+
					"name exists as a file. Reachable in production through Start (topRoot.OpenRoot(feature) with "+
					"no prior Lstat, assemble.go); Check and Status both Lstat and check IsDir() first, so neither "+
					"ever reaches this branch (check.go, status.go). Classifying it would change Start's own "+
					"wrapped error text, so this stays a documented exception rather than a fix",
			),
		)
	})

	t.Run("memDirFS", func(t *testing.T) {
		rwfstest.ReadContract(t, func(t *testing.T) fs.FS {
			t.Helper()

			return newMemDirFixture(t).FS()
		})
		rwfstest.ReadOpenRootContract(t, newMemDirFixture)
	})
}
