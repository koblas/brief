package setup

import "github.com/koblas/brief/internal/platform/rwfs"

// WithFSRoot overrides the production fsRoot (diskFS, the real, unconfined
// "/"-rooted filesystem) that every read and write Init and Uninstall
// perform under a repository root, and the config-location walk above it,
// go through — exported only so this package's own external tests
// (setup_test) can substitute an rwfs.Mem. export_test.go is excluded from
// production builds, so this is never part of the public Option surface
// WithHomeDir and its siblings are. It never affects bound_agent.go's own
// confinedAgentFile, which always reads and writes through real disk
// regardless (see fs.go's own diskFS doc comment).
func WithFSRoot(fsys rwfs.FS) Option {
	return func(s *Server) { s.fsRoot = func() rwfs.FS { return fsys } }
}

// WithResolveRoot overrides the production resolveRoot
// (filepath.EvalSymlinks) that boundAgentTargets and agentsMissingSkill
// call on root before either walks agentfile bindings — exported only so
// this package's own external tests can inject an identity function
// against a Server built over an rwfs.Mem, where root names no real
// directory for EvalSymlinks to resolve. Same test-only scope as
// WithFSRoot; never affects bound_agent.go's own confinedAgentFile.
func WithResolveRoot(fn func(string) (string, error)) Option {
	return func(s *Server) { s.resolveRoot = fn }
}

// WithWritableCheck overrides the production writableCheck (checkWritable,
// real os.Lstat plus internal/platform/writable.Probe) that Init runs
// before applying — exported only so this package's own external tests can
// record the targets a real run would have checked, or skip the call
// entirely, without a Mem-backed fixture asserting a refusal (or its
// absence) production could never actually produce against a real,
// unconverted R10. checkWritable itself (writable.go) is never changed by
// this seam; every writable_disk_test.go case still builds its Server with
// plain NewServer(), never this option.
func WithWritableCheck(fn func([]string) error) Option {
	return func(s *Server) { s.writableCheck = fn }
}
