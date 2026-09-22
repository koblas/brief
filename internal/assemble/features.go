package assemble

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

	entries, err := fs.ReadDir(topRoot.FS(), ".")
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
