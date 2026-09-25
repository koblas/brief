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
// Finish open every rwfs.FS through: fsys is a "/"-rooted namespace, so one
// rwfs.Mem fixture can back a command-level test. Production leaves this
// unset (nil): every open reads and writes real disk, confined by a real
// os.Root.
func WithFS(fsys rwfs.FS) Option {
	return func(s *Server) { s.rootFS = fsys }
}

// fsName maps abs, an absolute OS path, onto the name a WithFS fixture
// expects: the leading path separator stripped, forward-slash separated,
// "." for the root itself.
func fsName(abs string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), string(filepath.Separator))
	if trimmed == "" {
		return "."
	}

	return trimmed
}

// mkdirAll creates dir and every missing parent directory: s.rootFS's own
// MkdirAll, mapped through fsName, when WithFS is set; plain os.MkdirAll
// otherwise. It and openDir below return their underlying call's own error
// unwrapped, deliberately: every call site already wraps its own
// "scaffold: ..." context around it, so wrapping here would stack a second,
// redundant prefix.
func (s *Server) mkdirAll(dir string, perm fs.FileMode) error {
	if s.rootFS != nil {
		return s.rootFS.MkdirAll(fsName(dir), perm) //nolint:wrapcheck // see the block comment above
	}

	return os.MkdirAll(dir, perm) //nolint:wrapcheck // see the block comment above
}

// openDir opens dir — already known to exist — as an rwfs.FS rooted there:
// s.rootFS's own OpenRoot, mapped through fsName, when WithFS is set;
// rwfs.OpenOS otherwise.
func (s *Server) openDir(dir string) (rwfs.FS, error) {
	if s.rootFS != nil {
		return s.rootFS.OpenRoot(fsName(dir)) //nolint:wrapcheck // see the block comment above
	}

	return rwfs.OpenOS(dir) //nolint:wrapcheck // see the block comment above
}
