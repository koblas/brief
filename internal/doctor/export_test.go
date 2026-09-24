package doctor

import (
	"io/fs"

	"github.com/koblas/brief/internal/platform/agentfile"
)

// WithRootFS overrides the production root FS (os.DirFS("/")) that
// (*Server).locateInRepo, (*Server).inspect and checkEnvGit read through —
// exported only so this package's own external tests (doctor_test,
// host_test) can substitute a fstest.MapFS. export_test.go is excluded
// from production builds, so this is never part of the public Option
// surface WithHomeDir and its siblings are.
func WithRootFS(fsys fs.FS) Option {
	return func(s *Server) { s.rootFS = func() fs.FS { return fsys } }
}

// WithHomeTree overrides the agentfile.Tree a bare-name role binding's own
// Rule 5 search runs against for the user scope, bypassing WithHomeDir and
// agentfile.DirTree entirely — exported only so this package's own
// external tests can inject an in-memory Tree (fstest.MapFS-backed)
// instead of a real "~/.claude/agents" directory. Same test-only scope as
// WithRootFS.
func WithHomeTree(fn func() agentfile.Tree) Option {
	return func(s *Server) { s.homeTree = fn }
}
