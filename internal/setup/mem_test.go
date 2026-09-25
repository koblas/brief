package setup_test

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/setup"
)

// memTreeEntry is one memTree entry: isDir alone for a directory, body for
// a file's exact bytes; Mode and ModTime are excluded so comparisons stay
// stable across Mem's advancing fake clock.
type memTreeEntry struct {
	isDir bool
	body  []byte
}

// memTree extracts snap's entries under root (root excluded), keyed by
// each entry's root-relative, slash-separated path.
func memTree(snap fstest.MapFS, root string) map[string]memTreeEntry {
	prefix := memKey(root) + "/"
	out := map[string]memTreeEntry{}

	for name, file := range snap {
		if name == memKey(root) {
			continue
		}

		rel, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}

		if file.Mode.IsDir() {
			out[rel] = memTreeEntry{isDir: true}

			continue
		}

		out[rel] = memTreeEntry{body: file.Data}
	}

	return out
}

// findArtifact returns res's first Artifact of kind, failing the test if
// there is none.
func findArtifact(t *testing.T, res setup.Result, kind setup.Kind) setup.Artifact {
	t.Helper()

	for _, a := range res.Artifacts {
		if a.Kind == kind {
			return a
		}
	}

	t.Fatalf("no %s row in %v", kind, res.Artifacts)

	return setup.Artifact{}
}

// findArtifactByPath returns res's Artifact at path, failing the test if
// there is none.
func findArtifactByPath(t *testing.T, res setup.Result, path string) setup.Artifact {
	t.Helper()

	for _, a := range res.Artifacts {
		if a.Path == path {
			return a
		}
	}

	t.Fatalf("no row for %s in %v", path, res.Artifacts)

	return setup.Artifact{}
}

// fsAbs joins slash-separated segments under "/", the absolute-path form
// setup's fsName expects internally.
func fsAbs(elem ...string) string {
	return filepath.FromSlash("/" + filepath.ToSlash(filepath.Join(elem...)))
}

// identityResolveRoot stands in for filepath.EvalSymlinks: it returns root
// unchanged, since a Mem fixture names no real directory to resolve.
func identityResolveRoot(root string) (string, error) { return root, nil }

// memData returns snap's bytes at key and whether key is present, so a
// table-driven test can compare both branches with the same two assertions.
func memData(snap fstest.MapFS, key string) ([]byte, bool) {
	f, ok := snap[key]
	if !ok {
		return nil, false
	}

	return f.Data, true
}

// memKey turns an absolute path into the name a Mem's fstest.MapFS is
// keyed against.
func memKey(abs string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), "/")
	if trimmed == "" {
		return "."
	}

	return trimmed
}

// newVirtualMem returns a Mem seeded with root as an explicit directory
// entry, since every fsys read confirms wd itself exists first.
func newVirtualMem(root string) *rwfs.Mem {
	return rwfs.NewMem(fstest.MapFS{memKey(root): &fstest.MapFile{Mode: fs.ModeDir | 0o755}})
}

// emptyHomeDir is a setup.Option reporting "" for home, so a Mem-backed
// HostClaudeCode test never depends on the developer's own real home.
func emptyHomeDir() setup.Option {
	return setup.WithHomeDir(func() (string, error) { return "", nil })
}

// writableRecorder stands in for the real-disk writableCheck: every call
// reports nil, and calls records which targets a real run would have
// checked, in order.
type writableRecorder struct {
	calls [][]string
}

func (r *writableRecorder) check(targets []string) error {
	r.calls = append(r.calls, append([]string{}, targets...))

	return nil
}

// newMemServer builds a Server that reads and writes against mem, not real
// disk. A bare-name planner or implementer binding always resolves against
// real disk regardless of fsRoot, so a test that needs one stays on disk.
func newMemServer(mem *rwfs.Mem, opts ...setup.Option) *setup.Server {
	srv, _ := newMemServerRecording(mem, opts...)

	return srv
}

// newMemServerRecording is newMemServer's twin for a test that needs the
// writableRecorder itself.
func newMemServerRecording(mem *rwfs.Mem, opts ...setup.Option) (*setup.Server, *writableRecorder) {
	rec := &writableRecorder{}

	all := make([]setup.Option, 0, len(opts)+4)
	all = append(all,
		setup.WithFSRoot(mem),
		setup.WithResolveRoot(identityResolveRoot),
		emptyHomeDir(),
		setup.WithWritableCheck(rec.check),
	)
	all = append(all, opts...)

	return setup.NewServer(all...), rec
}
