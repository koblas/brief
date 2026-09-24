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

// fsAbs joins slash-separated segments under "/", the way every
// WithFSRoot-backed test names an absolute path its fstest.MapFS fixture
// is keyed against — setup's own fsName (fs.go) strips the leading "/"
// internally to get the fs.FS-relative name back. Identical to config's,
// repo's and doctor's own fsAbs test helper, duplicated for the same
// reason those packages duplicate fsName from each other.
func fsAbs(elem ...string) string {
	return filepath.FromSlash("/" + filepath.ToSlash(filepath.Join(elem...)))
}

// identityResolveRoot is a setup.Option's own resolveRoot stand-in for
// filepath.EvalSymlinks: mem (below) names no real directory on disk, so
// resolving root's own symlinks for real would always fail. It returns
// root unchanged, matching a root with no symlinks in it at all — every
// Mem-backed fixture in this package builds root as a plain string, never
// a symlink.
func identityResolveRoot(root string) (string, error) { return root, nil }

// memKey turns abs — an already-absolute path, real or fsAbs-fabricated —
// into the name a Mem's own fstest.MapFS is keyed against: fsName's own
// inverse (fs.go, package-private, so this test package cannot call it
// directly), duplicated for the same reason fsAbs above is.
func memKey(abs string) string {
	return strings.TrimPrefix(filepath.ToSlash(abs), "/")
}

// newRealRootMem returns a real, empty, writable t.TempDir() as wd
// alongside a Mem already seeded with that same directory as an explicit
// entry — the one seam this package deliberately leaves unconverted
// (checkWritable, R10) still walks real disk from wd upward regardless of
// fsRoot, so any test exercising a real (non-DryRun, non-Print) apply needs
// wd to genuinely exist and be writable; every read and write Init or
// Uninstall itself performs still goes through mem, never touching a file
// under wd. A DryRun- or Print-only test skips checkWritable entirely and
// can use a fully virtual fsAbs(...) wd instead, with no real directory at
// all.
func newRealRootMem(t *testing.T) (string, *rwfs.Mem) {
	t.Helper()

	wd := t.TempDir()
	mem := rwfs.NewMem(fstest.MapFS{memKey(wd): &fstest.MapFile{Mode: fs.ModeDir | 0o755}})

	return wd, mem
}

// newMemServer builds a Server whose every read and write under a
// repository root, and whose boundAgentTargets/agentsMissingSkill own
// root-resolution, run against mem rather than real disk (WithFSRoot,
// WithResolveRoot) — opts are appended after both, so a test can still
// layer WithHomeDir or another option without repeating either seam.
func newMemServer(mem *rwfs.Mem, opts ...setup.Option) *setup.Server {
	all := make([]setup.Option, 0, len(opts)+2)
	all = append(all, setup.WithFSRoot(mem), setup.WithResolveRoot(identityResolveRoot))
	all = append(all, opts...)

	return setup.NewServer(all...)
}
