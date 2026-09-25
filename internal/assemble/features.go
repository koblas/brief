package assemble

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Features returns the names of every real directory under the configured
// feature directory, in fs.ReadDir's byte order. A regular file or a
// symlink is excluded. A missing feature directory is zero features, not
// an error: Features returns (nil, nil). An unreadable or non-directory
// root still propagates an error.
func (s *Server) Features(_ context.Context) ([]string, error) {
	topRoot, err := s.openFeatureDir(filepath.Join(s.root, s.cfg.FeatureDirectory))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("assemble: %w", err)
	}
	defer func() { _ = topRoot.Close() }()

	return FeaturesFS(topRoot.FS())
}

// FeaturesFS is Features' core: it lists fsys's own top-level entries
// directly, with no nested open, so any fs.FS — real or in-memory —
// suffices. Same contract as Features.
func FeaturesFS(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("assemble: %w", err)
	}

	var names []string

	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}

	return names, nil
}

// FeatureContaining reports the name of the feature directory that
// contains path, and whether one does: path's first component under the
// configured feature directory, when that component is a real directory
// (never a symlink or a regular file) and path names something beneath it.
// It reports false for a path outside the feature directory, the feature
// directory itself, a file directly inside it, and a relative path.
func (s *Server) FeatureContaining(path string) (string, bool) {
	featureRoot := filepath.Join(s.root, s.cfg.FeatureDirectory)

	if name, ok := featureContainingLexical(featureRoot, path); ok {
		return name, true
	}

	// Retry through filepath.EvalSymlinks only after a lexical miss, and
	// only here: a feature directory that is itself a symlink must be
	// rejected as one, which resolving symlinks first would launder. This
	// rescues a path reached through a symlinked ancestor of the project
	// root (e.g. macOS's "/var" symlinked to "/private/var").
	resolvedRoot, err := filepath.EvalSymlinks(featureRoot)
	if err != nil {
		return "", false
	}

	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", false
	}

	return featureContainingLexical(resolvedRoot, resolvedPath)
}

// featureContainingLexical reports path's first component under
// featureRoot, and whether it names a real directory with at least one
// more component after it. Neither argument is resolved for symlinks.
func featureContainingLexical(featureRoot, path string) (string, bool) {
	rel, err := filepath.Rel(featureRoot, path)
	if err != nil {
		return "", false
	}

	// Without this guard a "../" escape would split into two parts below,
	// with ".." itself passing as a plausible feature directory.
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}

	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	if len(parts) < 2 {
		return "", false
	}

	first := parts[0]

	info, err := os.Lstat(filepath.Join(featureRoot, first))
	if err != nil || info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return "", false
	}

	return first, true
}
