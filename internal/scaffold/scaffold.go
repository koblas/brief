package scaffold

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/config"
)

// Server creates feature scaffolds under a project root, using cfg to name
// the feature directory, the specification and state files, and the
// headings written into each.
type Server struct {
	cfg  config.Config
	root string
}

// NewServer returns a Server rooted at root, using cfg for every path and
// heading it writes. Both arguments are required positionally: there is no
// optional dependency here for a functional option to default.
func NewServer(cfg config.Config, root string) *Server {
	return &Server{cfg: cfg, root: root}
}

// NewFeature creates the feature directory for name under the configured
// feature directory, writing an empty specification skeleton and an empty
// state file into it, and returns the created directory's path.
//
// Every write goes through an *os.Root rooted at the feature directory.
// Root.Mkdir refuses a name that escapes the root (a "../x" name cannot
// traverse out) and refuses an existing feature outright — that refusal is
// what stops an existing specification from being truncated: brief new
// feature brief run inside this repository would otherwise overwrite this
// project's own approved specification.md. Each file is additionally opened
// O_CREATE|O_EXCL, which cannot fire while Mkdir guarantees a brand-new
// leaf, and which holds the line if that guarantee is ever relaxed.
func (s *Server) NewFeature(_ context.Context, name string) (string, error) {
	featureRoot := filepath.Join(s.root, s.cfg.FeatureDirectory)
	if err := os.MkdirAll(featureRoot, 0o755); err != nil {
		return "", fmt.Errorf("scaffold: %w", err)
	}

	root, err := os.OpenRoot(featureRoot)
	if err != nil {
		return "", fmt.Errorf("scaffold: %w", err)
	}
	defer func() { _ = root.Close() }()

	if err := root.Mkdir(name, 0o755); err != nil {
		return "", fmt.Errorf("scaffold: %w", err)
	}

	if err := writeExclusive(root, filepath.Join(name, s.cfg.SpecificationFile), specificationSkeleton(s.cfg, name)); err != nil {
		return "", err
	}

	if err := writeExclusive(root, filepath.Join(name, s.cfg.StateFile), stateSkeleton(s.cfg)); err != nil {
		return "", err
	}

	return filepath.Join(featureRoot, name), nil
}

// writeExclusive creates name under root and writes contents to it,
// refusing rather than truncating if the file already exists.
func writeExclusive(root *os.Root, name, contents string) error {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}

	if _, err := f.WriteString(contents); err != nil {
		_ = f.Close()

		return fmt.Errorf("write %s: %w", name, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}

	return nil
}
