package repo

import (
	"os"
	"path/filepath"
)

// Root walks upward from startDir for a ".git" entry — a directory, or a
// file (a linked worktree's ".git" is a file naming its real gitdir
// elsewhere) — and returns the directory that holds it. ok is false when
// none exists anywhere above startDir, or startDir cannot be made absolute.
func Root(startDir string) (string, bool) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}

	for dir := abs; ; {
		candidate := filepath.Join(dir, ".git")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return dir, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}

		dir = parent
	}
}
