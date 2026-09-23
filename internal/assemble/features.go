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
// feature directory, in fs.ReadDir's documented byte order — the same
// order Status relies on. A regular file or a symlink is excluded: brief
// never treats either as a feature. A missing feature directory is zero
// features, not an error: Features returns (nil, nil), matching Status. An
// unreadable or non-directory feature-directory root still propagates an
// error.
func (s *Server) Features(_ context.Context) ([]string, error) {
	topRoot, err := os.OpenRoot(filepath.Join(s.root, s.cfg.FeatureDirectory))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("assemble: %w", err)
	}
	defer func() { _ = topRoot.Close() }()

	return FeaturesFS(topRoot.FS())
}

// FeaturesFS is Features' core: fsys lists a feature directory's own
// top-level entries directly, with no nested open, so any fs.FS — real or
// in-memory — suffices. It returns the name of every real directory
// entry, in fsys's own ReadDir order; a regular file or a symlink is
// excluded, matching Features' own contract.
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
// contains path, and whether one does: path's first path component under
// the configured feature directory, when that component is a real
// directory (never a symlink or a regular file) and path names something
// beneath it, not the component itself. It reports false for a path
// outside the feature directory (including a ".." escape), the feature
// directory itself, a file directly inside it, and a relative path — every
// caller resolves a relative hook payload path against its own working
// directory before calling this.
//
// Matching runs lexically first, against s.root/FeatureDirectory exactly
// as configured: a feature directory that is itself a symlink must be
// rejected as such, which resolving symlinks first would silently launder.
// Only when the lexical match says path is outside does FeatureContaining
// retry with both sides passed through filepath.EvalSymlinks — the one
// case that rescues a path reached through a symlinked ancestor of the
// project root itself (e.g. macOS's "/var" symlinked to "/private/var")
// from being misreported as outside.
func (s *Server) FeatureContaining(path string) (string, bool) {
	featureRoot := filepath.Join(s.root, s.cfg.FeatureDirectory)

	if name, ok := featureContainingLexical(featureRoot, path); ok {
		return name, true
	}

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

// featureContainingLexical reports path's first path component under
// featureRoot, and whether it names a real directory with at least one
// more component after it — see FeatureContaining for the full contract.
// Neither argument is resolved for symlinks here.
//
// A rel of "." or ".." (path is featureRoot itself, or its direct parent)
// is rejected by the len(parts) < 2 check below without needing its own
// case: filepath.Separator does not appear in either string, so SplitN
// yields one part. Only a deeper escape — rel beginning "../", which does
// split into two — needs the explicit HasPrefix guard: without it, the
// first part would be "..", and an os.Lstat of featureRoot's own parent
// would pass as a plausible "feature directory".
func featureContainingLexical(featureRoot, path string) (string, bool) {
	rel, err := filepath.Rel(featureRoot, path)
	if err != nil {
		return "", false
	}

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
