package scaffold

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"unicode"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/stepfile"
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

// Result is what NewFeature and NewStep return: Feature is the feature
// name given to the call, Step is the created step's id (empty for
// NewFeature — no call creates a feature and a step in one invocation),
// Path is the single path the caller's own text-mode contract prints (the
// feature directory for NewFeature, the step file for NewStep), and
// Created lists every path this call brought into existence, in the order
// it wrote them. Created never includes a file the call only modified — a
// specification NewStep appends a progress entry to is not "created" by
// that call — and is never nil, so a caller can range over it without a
// nil check.
type Result struct {
	Feature string
	Step    string
	Path    string
	Created []string
}

// NewFeature creates the feature directory for name under the configured
// feature directory, writing an empty specification skeleton and an empty
// state file into it, and returns a Result naming the created directory
// and the two files written into it, in write order.
//
// name is validated before anything touches disk: an empty name, or one
// containing whitespace, is refused as ErrInvalidFeatureName and no
// directory — not even the configured feature directory itself — is
// created.
//
// Every write goes through an *os.Root rooted at the feature directory.
// Root.Mkdir refuses a name that escapes the root (a "../x" name cannot
// traverse out); when it fails because the feature directory already
// exists, that failure is reported as a *RefusalError wrapping
// ErrFeatureExists naming the existing directory — the guard that stops an
// existing specification from being truncated: brief new feature brief run
// inside this repository would otherwise overwrite this project's own
// approved specification.md. Every other Mkdir failure, including the
// traversal case above, keeps its plain wrapped-error shape. Each file is
// additionally opened O_CREATE|O_EXCL, a second guard behind Mkdir's that
// cannot fire while Mkdir guarantees a brand-new leaf, and which holds the
// line if that guarantee is ever relaxed.
func (s *Server) NewFeature(_ context.Context, name string) (Result, error) {
	if err := validateFeatureName(name); err != nil {
		return Result{}, err
	}

	featureRoot := filepath.Join(s.root, s.cfg.FeatureDirectory)
	if err := os.MkdirAll(featureRoot, 0o755); err != nil {
		return Result{}, fmt.Errorf("scaffold: %w", err)
	}

	root, err := os.OpenRoot(featureRoot)
	if err != nil {
		return Result{}, fmt.Errorf("scaffold: %w", err)
	}
	defer func() { _ = root.Close() }()

	if err := root.Mkdir(name, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			featurePath := filepath.Join(featureRoot, name)

			return Result{}, &RefusalError{
				Path:    featurePath,
				Problem: "feature already exists",
				Fix:     fmt.Sprintf("run 'brief new step %s' to add a step to it, or choose a different name", name),
				Err:     ErrFeatureExists,
			}
		}

		return Result{}, fmt.Errorf("scaffold: %w", err)
	}

	featurePath := filepath.Join(featureRoot, name)
	specPath := filepath.Join(featurePath, s.cfg.SpecificationFile)
	statePath := filepath.Join(featurePath, s.cfg.StateFile)

	if err := writeExclusive(root, filepath.Join(name, s.cfg.SpecificationFile), specificationSkeleton(s.cfg, name)); err != nil {
		return Result{}, err
	}

	if err := writeExclusive(root, filepath.Join(name, s.cfg.StateFile), stateSkeleton(s.cfg)); err != nil {
		return Result{}, err
	}

	return Result{
		Feature: name,
		Path:    featurePath,
		Created: []string{specPath, statePath},
	}, nil
}

// NewStep creates the next step file for feature and appends its progress
// entry, returning a Result naming the created step's id and file. Created
// holds only the step file: the specification's progress entry is a
// modification of an existing file, not a creation, and is never listed.
//
// Validation runs in the order a refusal must name the first thing wrong
// (R14a): the configured step-file-pattern compiles, the feature directory
// opens (also the traversal guard: a feature name that escapes the
// feature directory is reported as ErrNoSuchFeature rather than as a
// traversal error, since Root.OpenRoot cannot distinguish the two),
// the specification is read, and the specification carries the configured
// progress heading. Nothing is created until all four pass; only then is
// the next step number computed and written.
//
// The step file is written before the specification: if the specification
// write then fails, the result is an orphan step file with no progress
// entry — visible and repairable — rather than a progress entry pointing
// at a step file that was never created.
func (s *Server) NewStep(_ context.Context, feature string) (Result, error) {
	featureDirPath := filepath.Join(s.root, s.cfg.FeatureDirectory)
	featurePath := filepath.Join(featureDirPath, feature)

	pattern, err := stepfile.Compile(s.cfg.StepFilePattern)
	if err != nil {
		return Result{}, &RefusalError{
			Path:    featurePath,
			Problem: fmt.Sprintf("step-file-pattern %q is invalid: %v", s.cfg.StepFilePattern, err),
			Fix:     "fix step-file-pattern in .brief.yaml",
			Err:     stepfile.ErrInvalidPattern,
		}
	}

	topRoot, err := os.OpenRoot(featureDirPath)
	if err != nil {
		return Result{}, noSuchFeatureRefusal(featurePath, feature)
	}
	defer func() { _ = topRoot.Close() }()

	root, err := topRoot.OpenRoot(feature)
	if err != nil {
		return Result{}, noSuchFeatureRefusal(featurePath, feature)
	}
	defer func() { _ = root.Close() }()

	specPath := filepath.Join(featurePath, s.cfg.SpecificationFile)

	specBytes, err := root.ReadFile(s.cfg.SpecificationFile)
	if err != nil {
		return Result{}, &RefusalError{
			Path:    specPath,
			Problem: "specification file is missing",
			Fix:     "scaffold the feature again to restore it",
			Err:     ErrMalformedFeature,
		}
	}

	spec := string(specBytes)

	if _, err := insertProgressEntry(spec, s.cfg.ProgressHeading, ""); err != nil {
		return Result{}, &RefusalError{
			Path:    specPath,
			Problem: fmt.Sprintf("no %q heading found", s.cfg.ProgressHeading),
			Fix:     fmt.Sprintf("add a %q heading to the specification", s.cfg.ProgressHeading),
			Err:     ErrNoProgressHeading,
		}
	}

	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return Result{}, fmt.Errorf("scaffold: %w", err)
	}

	next := 1

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		if n, ok := pattern.Number(e.Name()); ok && n >= next {
			next = n + 1
		}
	}

	id := pattern.ID(next)
	stepName := pattern.Name(next)

	if err := writeExclusive(root, stepName, stepSkeleton(s.cfg, id)); err != nil {
		return Result{}, err
	}

	newSpec, err := insertProgressEntry(spec, s.cfg.ProgressHeading, progressEntry(id))
	if err != nil {
		return Result{}, fmt.Errorf("scaffold: %w", err)
	}

	if err := replaceString(root, s.cfg.SpecificationFile, newSpec); err != nil {
		return Result{}, fmt.Errorf("scaffold: %w", err)
	}

	stepPath := filepath.Join(featurePath, stepName)

	return Result{
		Feature: feature,
		Step:    id,
		Path:    stepPath,
		Created: []string{stepPath},
	}, nil
}

// noSuchFeatureRefusal reports that feature has no directory at path,
// naming the command that creates one.
func noSuchFeatureRefusal(path, feature string) error {
	return &RefusalError{
		Path:    path,
		Problem: "no such feature",
		Fix:     fmt.Sprintf("run 'brief new feature %s' to create it", feature),
		Err:     ErrNoSuchFeature,
	}
}

// validateFeatureName refuses an empty name, or one carrying a rune
// unicode.IsSpace reports true for, wrapping ErrInvalidFeatureName with
// the offending name so the refusal names what was wrong. status is a
// whitespace-separated four-field contract (strings.Fields splits on the
// same predicate); a name breaking it here is what makes that contract
// hold everywhere it is read.
func validateFeatureName(name string) error {
	if name == "" {
		return fmt.Errorf("name is empty: %w", ErrInvalidFeatureName)
	}

	for _, r := range name {
		if unicode.IsSpace(r) {
			return fmt.Errorf("name %q contains whitespace: %w", name, ErrInvalidFeatureName)
		}
	}

	return nil
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
