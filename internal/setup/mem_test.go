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

// findArtifact returns res's own first Artifact of kind, failing the test
// if there is none — the by-kind lookup a Mem-backed test uses in place of
// an index into Result.Artifacts, so it never depends on that list's own
// row order except in the one test that deliberately pins it.
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

// emptyHomeDir is a setup.Option pinning WithHomeDir to a function that
// always reports "", nil — newMemServer's own default, mirroring doctor's
// own emptyHomeDir: a Mem-backed fixture holds no real "~/.claude/agents"
// for agentfile.ResolveBinding to search, so without this every
// HostClaudeCode Mem test would silently depend on whatever role agents
// happen to exist under the developer's own real home directory.
func emptyHomeDir() setup.Option {
	return setup.WithHomeDir(func() (string, error) { return "", nil })
}

// newMemServer builds a Server whose every read and write under a
// repository root, and whose boundAgentTargets/agentsMissingSkill own
// root-resolution, run against mem rather than real disk (WithFSRoot,
// WithResolveRoot), homeDir defaulted to emptyHomeDir — opts are appended
// last, so a test can still override any of the three without repeating
// the others. No Mem fixture in this package may bind a bare-name planner
// or implementer role: agentfile.ResolveBinding and planBoundAgent always
// search and read real disk regardless of fsRoot, so such a binding would
// silently resolve against nothing rather than the fixture's own content —
// a test that needs one stays on disk (bound_agent_test.go,
// bound_agent_internal_test.go, uninstall_bound_agent_test.go,
// missing_skill_test.go, missing_skill_internal_test.go).
func newMemServer(mem *rwfs.Mem, opts ...setup.Option) *setup.Server {
	all := make([]setup.Option, 0, len(opts)+3)
	all = append(all, setup.WithFSRoot(mem), setup.WithResolveRoot(identityResolveRoot), emptyHomeDir())
	all = append(all, opts...)

	return setup.NewServer(all...)
}
