package scaffold

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/koblas/brief/internal/platform/rwfs"
)

// Option configures a Server built by NewServer.
type Option func(*Server)

// WithFS overrides the production filesystem NewFeature, NewStep and
// Finish open every rwfs.FS through: fsys is a "/"-rooted namespace, the
// same shape internal/setup's WithFSRoot and internal/doctor's WithRootFS
// already take, so one rwfs.Mem fixture can back every seamed package a
// command-level test constructs. Production leaves this unset (nil, the
// zero value): NewFeature keeps calling os.MkdirAll then rwfs.OpenOS
// directly, and NewStep's and Finish's own openFeatureDir keeps calling
// rwfs.OpenOS alone — both confined by a real os.Root, exactly as before
// this option existed.
func WithFS(fsys rwfs.FS) Option {
	return func(s *Server) { s.rootFS = fsys }
}

// fsName maps abs, an absolute OS path, onto the name a WithFS fixture
// expects: the leading path separator stripped, forward-slash separated,
// "." for the root itself. Duplicated from internal/setup's, internal/
// platform/config's and internal/doctor's own identical helper, following
// this codebase's own precedent of copying an eight-line mapping rather
// than sharing it across packages with no other reason to depend on one
// another.
func fsName(abs string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), string(filepath.Separator))
	if trimmed == "" {
		return "."
	}

	return trimmed
}

// mkdirAll creates dir and every missing parent directory: s.rootFS's own
// MkdirAll, mapped through fsName, when WithFS is set; plain os.MkdirAll
// otherwise — NewFeature's own production call, unseamed and unchanged.
// Both methods below return their underlying call's own error unwrapped,
// deliberately: every call site (NewFeature, openFeatureDir) already
// reconstructs its own "scaffold: ..." context around the exact same error
// os.MkdirAll, rwfs.OpenOS or rwfs.FS has always returned — wrapping it
// again here would stack a second, redundant prefix onto a message a
// caller already names.

func (s *Server) mkdirAll(dir string, perm fs.FileMode) error {
	if s.rootFS != nil {
		return s.rootFS.MkdirAll(fsName(dir), perm) //nolint:wrapcheck // see the block comment above
	}

	return os.MkdirAll(dir, perm) //nolint:wrapcheck // see the block comment above
}

// openDir opens dir — already known to exist — as an rwfs.FS rooted there:
// s.rootFS's own OpenRoot, mapped through fsName, when WithFS is set;
// rwfs.OpenOS otherwise, the production call every call site made before
// this option existed.
func (s *Server) openDir(dir string) (rwfs.FS, error) {
	if s.rootFS != nil {
		return s.rootFS.OpenRoot(fsName(dir)) //nolint:wrapcheck // see the block comment above
	}

	return rwfs.OpenOS(dir) //nolint:wrapcheck // see the block comment above
}
