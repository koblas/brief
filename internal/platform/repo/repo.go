package repo

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// rootFS returns the production root FS: the whole namespace RootFS walks,
// rooted at "/". This assumes a single-rooted, forward-slash namespace —
// true for brief's darwin/linux target (no Windows evidence anywhere in
// the tree: no CI workflow, devenv.nix names only a linux Buildkite
// agent) — and is not evaluated on a Windows volume path ("C:\..."), which
// fsName below cannot represent.
func rootFS() fs.FS {
	return os.DirFS("/")
}

// fsName maps abs, an absolute OS path, onto the name rootFS (or a test's
// own fstest.MapFS standing in for it) expects: the leading path separator
// stripped, forward-slash separated, "." for the root itself. Duplicated
// from an identical helper in internal/platform/config rather than shared,
// since repo is what config imports for its own boundary and a shared
// package would invert that dependency for an eight-line mapping.
func fsName(abs string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), "/")
	if trimmed == "" {
		return "."
	}

	return trimmed
}

// Root walks upward from startDir for a ".git" entry — a directory, or a
// file (a linked worktree's ".git" is a file naming its real gitdir
// elsewhere) — and returns the directory that holds it. ok is false when
// none exists anywhere above startDir, or startDir cannot be made absolute.
// It is RootFS(rootFS(), abs) — the OS adapter, owning filepath.Abs's own
// cwd-dependent resolution.
func Root(startDir string) (string, bool) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}

	return RootFS(rootFS(), abs)
}

// RootFS is Root's own core: fsys is the whole filesystem namespace the
// walk runs against — production passes rootFS(), a test a fstest.MapFS
// holding just the ancestors in play — and startAbs is already an
// absolute, slash-separated OS path; filepath.Abs's own cwd-dependent
// resolution stays in Root, never here. The walk runs entirely on
// startAbs's own string form (filepath.Join / filepath.Dir), identical
// regardless of fsys; only the existence check goes through it, by way of
// fsName.
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
