package repo

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// rootFS returns the production root FS, rooted at "/". Assumes a
// single-rooted, forward-slash namespace (darwin/linux only).
func rootFS() fs.FS {
	return os.DirFS("/")
}

// fsName maps abs, an absolute OS path, onto the fs.FS-relative name
// rootFS expects: the leading separator stripped, forward-slash
// separated, "." for the root itself.
func fsName(abs string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), "/")
	if trimmed == "" {
		return "."
	}

	return trimmed
}

// Root walks upward from startDir for a ".git" entry — a directory, or a
// file naming a linked worktree's real gitdir — and returns the directory
// that holds it. ok is false when none exists above startDir, or startDir
// cannot be made absolute.
func Root(startDir string) (string, bool) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}

	return RootFS(rootFS(), abs)
}

// RootFS is Root's filesystem-agnostic core: fsys is the namespace to
// walk (production passes rootFS(), a test a fstest.MapFS), and startAbs
// is already an absolute, slash-separated path.
func RootFS(fsys fs.FS, startAbs string) (string, bool) {
	for dir := startAbs; ; {
		candidate := filepath.Join(dir, ".git")
		if _, statErr := fs.Stat(fsys, fsName(candidate)); statErr == nil {
			return dir, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}

		dir = parent
	}
}
