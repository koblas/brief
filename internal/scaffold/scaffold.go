package scaffold

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"unicode"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/platform/stepfile"
)

// Server creates feature scaffolds under a project root, using cfg to name
// the feature directory, the specification and state files, and the
// headings written into each. rootFS backs every rwfs.FS this Server opens
// (WithFS); nil, the default, means every open reads and writes real disk,
// exactly as before that option existed.
type Server struct {
	cfg    config.Config
	root   string
	rootFS rwfs.FS
}

// NewServer returns a Server rooted at root, using cfg for every path and
// heading it writes. cfg and root are required positionally: there is no
// optional dependency there for a functional option to default. opts
// applies over that production default; today WithFS is the only Option.
func NewServer(cfg config.Config, root string, opts ...Option) *Server {
	s := &Server{cfg: cfg, root: root}

	for _, o := range opts {
		o(s)
	}

	return s
}

// Result is what NewFeature and NewStep return: Feature is the feature
// name given to the call, Step is the created step's id (empty for
// NewFeature — no call creates a feature and a step in one invocation),
// Path is the single path the caller's own text-mode contract prints (the
// feature directory for NewFeature, the step file for NewStep), Created
// lists every path this call brought into existence, in the order it wrote
// them, and Modified lists every path it rewrote in place instead —
// NewStep's own specification, whose progress list it appends an entry to,
// never NewFeature's, which creates both its files fresh. Neither slice
// ever includes a path the other already names, and neither is ever nil,
// so a caller can range over either without a nil check.
type Result struct {
	Feature  string
	Step     string
	Path     string
	Created  []string
	Modified []string
}

// NewFeature creates the feature directory for name under the configured
// feature directory, writing an empty specification skeleton and an empty
// state file into it, and returns a Result naming the created directory
// and the two files written into it. Result.Created is always exactly
// [specification, state], in that order — a caller may index it directly
// rather than ranging over it.
//
// name is validated before anything touches disk: an empty name, or one
// containing whitespace, is refused as ErrInvalidFeatureName and no
// directory — not even the configured feature directory itself — is
// created.
//
// NewFeature builds the one rwfs.FS this call uses through s.mkdirAll and
// s.openDir — an OS adapter rooted at the configured feature directory, via
// rwfs.OpenOS, after os.MkdirAll has ensured that directory exists, in
// production; a WithFS-backed test substitutes an rwfs.Mem for both, so no
// real disk is touched. MkdirAll itself stays a plain os call rather than
// going through rwfs in production, so a symlinked feature directory is
// created through exactly as many hops as it always was, never more.
// NewFeatureFS carries every check and write that follows.
func (s *Server) NewFeature(_ context.Context, name string) (Result, error) {
	if err := validateFeatureName(name); err != nil {
		return Result{}, err
	}

	featureRoot := filepath.Join(s.root, s.cfg.FeatureDirectory)
	if err := s.mkdirAll(featureRoot, 0o755); err != nil {
		return Result{}, fmt.Errorf("scaffold: %w", err)
	}

	fsys, err := s.openDir(featureRoot)
	if err != nil {
		return Result{}, fmt.Errorf("scaffold: %w", peelReadErr(err))
	}
	defer func() { _ = fsys.Close() }()

	return s.NewFeatureFS(fsys, featureRoot, name)
}

// NewFeatureFS is NewFeature's core: fsys is an rwfs.FS already rooted at
// the configured feature directory (featureRoot, its absolute OS path —
// the prefix every path in the returned Result and every *RefusalError
// this call produces is joined onto). name has already passed
// validateFeatureName.
//
// fsys.Mkdir refuses a name that would escape fsys's own root the same way
// os.Root always has — a "../x" name cannot traverse out; on the OS
// adapter this holds via the underlying *os.Root, and fsys.Mkdir's own
// fs.ValidPath check refuses "../x" before that even runs. When Mkdir
// fails because the feature directory already exists, that failure is
// reported as a *RefusalError wrapping ErrFeatureExists naming the
// existing directory — the guard that stops an existing specification
// from being truncated: brief new feature brief run inside this
// repository would otherwise overwrite this project's own approved
// specification.md. Every other Mkdir failure, including the traversal
// case above, keeps its plain wrapped-error shape. Each file is
// additionally written with fsys.CreateExclusive, a second guard behind
// Mkdir's that cannot fire while Mkdir guarantees a brand-new leaf, and
// which holds the line if that guarantee is ever relaxed.
func (s *Server) NewFeatureFS(fsys rwfs.FS, featureRoot, name string) (Result, error) {
	if err := fsys.Mkdir(name, 0o755); err != nil {
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

	// Both failures below are marked ErrPartialWrite: fsys.Mkdir above has
	// already landed the feature directory itself by the time either can
	// fail, so cli's files_changed (R3) must report true, not false.
	if err := writeExclusive(fsys, path.Join(name, s.cfg.SpecificationFile), specificationSkeleton(s.cfg, name)); err != nil {
		return Result{}, markPartial(err)
	}

	if err := writeExclusive(fsys, path.Join(name, s.cfg.StateFile), stateSkeleton(s.cfg)); err != nil {
		return Result{}, markPartial(err)
	}

	return Result{
		Feature:  name,
		Path:     featurePath,
		Created:  []string{specPath, statePath},
		Modified: []string{},
	}, nil
}

// NewStep creates the next step file for feature and appends its progress
// entry, returning a Result naming the created step's id and file. Created
// holds only the step file; Modified holds the specification, which this
// call rewrites in place rather than creates.
//
// Validation runs in the order a refusal must name the first thing wrong
// (R14a): the configured step-file-pattern compiles, feature passes
// validFeatureArgument (the traversal guard: a feature name that escapes
// the feature directory refuses as ErrNoSuchFeature rather than as a
// traversal error), the feature directory opens, the specification is
// read, and the specification carries the configured progress heading.
// Nothing is created until all five pass; only then is the next step
// number computed and written.
//
// NewStep builds the one rwfs.FS this call uses through s.openFeatureDir —
// an OS adapter nested two levels deep, first at the configured feature
// directory, then at feature's own subdirectory, in production; a
// WithFS-backed test substitutes an rwfs.Mem — and delegates every check
// and write past that point to NewStepFS.
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

	top, root, err := s.openFeatureDir(featureDirPath, featurePath, feature)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = top.Close() }()
	defer func() { _ = root.Close() }()

	return s.NewStepFS(root, featurePath, feature, pattern)
}

// NewStepFS is NewStep's core: fsys is an rwfs.FS already opened and
// confined to feature's own directory, and featurePath is that
// directory's absolute OS path — the prefix every path in the returned
// Result and every *RefusalError this call produces is joined onto.
// NewStepFS assumes fsys already names a real, contained feature
// directory; NewStep is NewStepFS preceded by stepfile.Compile and the
// os.Root-equivalent containment chain that turns an absent feature into
// ErrNoSuchFeature.
//
// The step file is written before the specification: if the specification
// write then fails, the result is an orphan step file with no progress
// entry — visible and repairable — rather than a progress entry pointing
// at a step file that was never created.
func (s *Server) NewStepFS(fsys rwfs.FS, featurePath, feature string, pattern stepfile.Pattern) (Result, error) {
	specPath := filepath.Join(featurePath, s.cfg.SpecificationFile)

	specBytes, err := fsys.ReadFile(s.cfg.SpecificationFile)
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

	entries, err := fsys.ReadDir(".")
	if err != nil {
		return Result{}, fmt.Errorf("scaffold: %w", peelReadErr(err))
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

	if err := writeExclusive(fsys, stepName, stepSkeleton(s.cfg, id)); err != nil {
		return Result{}, err
	}

	newSpec, err := insertProgressEntry(spec, s.cfg.ProgressHeading, progressEntry(id))
	if err != nil {
		return Result{}, fmt.Errorf("scaffold: %w", err)
	}

	// Marked ErrPartialWrite: the step file above has already landed by the
	// time this can fail, so cli's files_changed (R3) must report true.
	if err := replaceString(fsys, s.cfg.SpecificationFile, newSpec); err != nil {
		return Result{}, markPartial(fmt.Errorf("scaffold: %w", err))
	}

	stepPath := filepath.Join(featurePath, stepName)

	return Result{
		Feature:  feature,
		Step:     id,
		Path:     stepPath,
		Created:  []string{stepPath},
		Modified: []string{specPath},
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

// openFeatureDir opens feature's own directory under featureDirPath,
// returning both the rwfs.FS the caller must close (top, s.openDir's own
// result, rooted at the configured feature directory — an OS adapter in
// production, an rwfs.Mem view under WithFS — then root, feature's own
// subdirectory, nested inside it via top.OpenRoot). It refuses as
// noSuchFeatureRefusal(featurePath, feature) when feature fails
// validFeatureArgument, checked before either open, so a traversal attempt
// never depends on the open's own error shape, or when either open fails
// with errors.Is(err, fs.ErrNotExist) — a genuinely absent directory. Any
// other open failure — permission denied, or a regular file where a
// directory belongs — is returned wrapped instead, never misreported as
// "no such feature". NewStep and Finish share this rather than duplicating
// the two-level open each carries.
func (s *Server) openFeatureDir(featureDirPath, featurePath, feature string) (rwfs.FS, rwfs.FS, error) {
	if !validFeatureArgument(feature) {
		return nil, nil, noSuchFeatureRefusal(featurePath, feature)
	}

	top, err := s.openDir(featureDirPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, noSuchFeatureRefusal(featurePath, feature)
		}

		return nil, nil, fmt.Errorf("scaffold: open feature %s: %w", feature, peelReadErr(err))
	}

	// top.OpenRoot's own failure keeps rwfs's *fs.PathError wrapping rather
	// than being peeled: unlike rwfs.OpenOS above, the raw cause it wraps
	// for "feature exists but is not a directory" is syscall.ENOTDIR alone,
	// with no path in it — peeling here would discard the one thing this
	// message still needs to name (see peelWriteErr's own doc comment).
	root, err := top.OpenRoot(feature)
	if err != nil {
		_ = top.Close()

		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, noSuchFeatureRefusal(featurePath, feature)
		}

		return nil, nil, fmt.Errorf("scaffold: open feature %s: %w", feature, err)
	}

	return top, root, nil
}

// validFeatureArgument reports whether feature is a well-formed single path
// component: not empty, not "." or "..", and free of any
// os.IsPathSeparator character. NewStep and Finish check it before either
// of their two rwfs.FS.OpenRoot calls, so a traversal attempt ("../x"), a
// path-separator name, or an empty string refuses as noSuchFeatureRefusal
// without depending on OpenRoot's own error shape to distinguish those from
// a genuinely missing directory — an empty string otherwise reaches
// top.OpenRoot("") and surfaces its own opaque failure. Mirrors assemble's
// own validFeatureArgument (internal/assemble/check.go), duplicated rather
// than shared because scaffold and assemble must not import each other.
func validFeatureArgument(feature string) bool {
	if feature == "" || feature == "." || feature == ".." {
		return false
	}

	for i := range len(feature) {
		if os.IsPathSeparator(feature[i]) {
			return false
		}
	}

	return true
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

// writeExclusive creates name under fsys and writes contents to it,
// refusing rather than truncating if the file already exists.
func writeExclusive(fsys rwfs.FS, name, contents string) error {
	if err := fsys.CreateExclusive(name, []byte(contents), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", name, peelWriteErr(err))
	}

	return nil
}
