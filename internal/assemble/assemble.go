package assemble

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/conform"
	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/platform/stepfile"
)

// Server reads feature directories under a project root, using cfg to
// name the feature directory, the step-file pattern, the state file and
// every heading Start extracts. rootFS backs every dirFS this Server opens
// (WithFS); nil, the default, means every open reads real disk through a
// real, nested os.Root, exactly as before that option existed.
type Server struct {
	cfg    config.Config
	root   string
	rootFS rwfs.FS

	// openRoot opens name as a subdirectory of parent, on the production
	// (os.Root-backed) path only. Defaults to (*os.Root).OpenRoot; a test
	// overrides it (export_test.go) to inject an open failure that does not
	// depend on OS permission bits or effective uid.
	openRoot func(parent *os.Root, name string) (*os.Root, error)
}

// NewServer returns a Server rooted at root, using cfg for every path and
// heading it reads. cfg and root are required positionally: there is no
// optional dependency there for a functional option to default. opts
// applies over that production default; today WithFS is the only Option.
func NewServer(cfg config.Config, root string, opts ...Option) *Server {
	s := &Server{
		cfg:      cfg,
		root:     root,
		openRoot: (*os.Root).OpenRoot,
	}

	for _, o := range opts {
		o(s)
	}

	return s
}

// FeatureFS pairs one feature's own filesystem, rooted so its top-level
// entries are the feature's own files, with the absolute OS directory it
// is rooted at. Path is the prefix every RefusalError, Finding, Problem
// and Shortfall path is joined onto.
type FeatureFS struct {
	FS   fs.FS
	Path string
}

// stepEntry is one step file found while enumerating a feature directory:
// its step number, and its frontmatter and the body after it.
type stepEntry struct {
	number int
	fm     stepfile.Frontmatter
	rest   []byte
}

// Start reads feature's directory and returns everything an implementer
// needs to begin its next open step: the next open step's id, title,
// acceptance criteria and checklist, the done/open counts across every
// step file, and every section of the feature's state file. Start returns
// ErrNoSuchFeature when feature has no directory, and a *RefusalError
// wrapping ErrMalformedFeature when the feature's structure is malformed
// (see the package doc). An absent or empty acceptance section in the
// briefed step, a checklist with no items, or an absent state-file
// heading, does not refuse: each is appended to Brief.Shortfalls instead.
// Start reads only; it writes nothing to disk.
func (s *Server) Start(_ context.Context, feature string) (Brief, error) {
	// feature is validated before either directory opens, so a traversal
	// attempt ("../x"), a path-separator name, or an empty string refuses
	// as ErrNoSuchFeature without depending on OpenRoot's own error shape.
	if !validFeatureArgument(feature) {
		return Brief{}, ErrNoSuchFeature
	}

	featureDirPath := filepath.Join(s.root, s.cfg.FeatureDirectory)
	featurePath := filepath.Join(featureDirPath, feature)

	topRoot, err := s.openFeatureDir(featureDirPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Brief{}, ErrNoSuchFeature
		}

		return Brief{}, fmt.Errorf("assemble: open feature %s: %w", feature, err)
	}
	defer func() { _ = topRoot.Close() }()

	root, err := topRoot.OpenRoot(feature)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Brief{}, ErrNoSuchFeature
		}

		return Brief{}, fmt.Errorf("assemble: open feature %s: %w", feature, err)
	}
	defer func() { _ = root.Close() }()

	return s.StartFS(FeatureFS{FS: root.FS(), Path: featurePath})
}

// StartFS is Start's core: fsys is one feature's own filesystem, already
// opened and confined, so a step file symlinked outside it is never
// reachable. Unlike Start, it assumes fsys already names a real, contained
// feature directory and never returns ErrNoSuchFeature.
func (s *Server) StartFS(fsys FeatureFS) (Brief, error) {
	if err := s.checkSpecification(fsys); err != nil {
		return Brief{}, err
	}

	stateBytes, err := s.readStateFile(fsys)
	if err != nil {
		return Brief{}, err
	}

	pattern, err := stepfile.Compile(s.cfg.StepFilePattern)
	if err != nil {
		return Brief{}, fmt.Errorf("assemble: %w", err)
	}

	dirEntries, err := fs.ReadDir(fsys.FS, ".")
	if err != nil {
		return Brief{}, fmt.Errorf("assemble: %w", err)
	}

	steps, err := readSteps(fsys.FS, pattern, dirEntries)
	if err != nil {
		return Brief{}, &RefusalError{Problem: *newProblem(fsys.Path, err, true), Err: err}
	}

	brief := Brief{Inherited: stateSections(string(stateBytes), s.cfg)}

	for _, e := range steps {
		if e.fm.Done() {
			brief.Done++
		} else {
			brief.Open++
		}
	}

	for _, e := range steps {
		if e.fm.Done() {
			continue
		}

		step := stepFromEntry(e, s.cfg)
		stepPath := filepath.Join(fsys.Path, pattern.Name(e.number))

		if step.ID == "" {
			return Brief{}, &RefusalError{
				Path:   stepPath,
				Detail: `no "id:" found in frontmatter`,
				Fix:    `add an "id:" field to the step file's frontmatter`,
				Err:    ErrMalformedFeature,
			}
		}

		if !step.Checklist.Found {
			return Brief{}, &RefusalError{
				Path:   stepPath,
				Detail: fmt.Sprintf("no %q heading found", s.cfg.ChecklistHeading),
				Fix:    fmt.Sprintf("add a %q heading to the step file", s.cfg.ChecklistHeading),
				Err:    ErrMalformedFeature,
			}
		}

		brief.Step = step

		if !step.Acceptance.Found {
			brief.Shortfalls = append(brief.Shortfalls, Shortfall{
				Path:   stepPath,
				Detail: fmt.Sprintf("no %q heading found", s.cfg.AcceptanceHeading),
				Fix:    fmt.Sprintf("add a %q heading to the step file", s.cfg.AcceptanceHeading),
			})
		} else if strings.TrimSpace(step.Acceptance.Body) == "" {
			brief.Shortfalls = append(brief.Shortfalls, Shortfall{
				Path:   stepPath,
				Detail: fmt.Sprintf("%q is empty", s.cfg.AcceptanceHeading),
				Fix:    "write the step's acceptance criteria under it",
			})
		}

		if n, found := conform.ChecklistItemCount(e.rest, s.cfg.ChecklistHeading); found && n == 0 {
			brief.Shortfalls = append(brief.Shortfalls, Shortfall{
				Path:   stepPath,
				Detail: fmt.Sprintf("%q has no checklist items", s.cfg.ChecklistHeading),
				Fix: `add them as "- [ ]" lines before implementing, ` +
					"since brief finish refuses a step with none",
			})
		}

		break
	}

	statePath := filepath.Join(fsys.Path, s.cfg.StateFile)

	for _, section := range brief.Inherited {
		if section.Found {
			continue
		}

		brief.Shortfalls = append(brief.Shortfalls, Shortfall{
			Path:   statePath,
			Detail: fmt.Sprintf("no %q heading found", section.Heading),
			Fix:    fmt.Sprintf("add a %q heading to the state file", section.Heading),
		})
	}

	return brief, nil
}

// checkSpecification refuses with a *RefusalError wrapping
// ErrMalformedFeature when feature's specification is absent, unreadable,
// carries an unclosed fenced code block, or has no cfg.ProgressHeading section.
func (s *Server) checkSpecification(fsys FeatureFS) error {
	_, refusal := s.specFault(fsys)
	if refusal != nil {
		return refusal
	}

	return nil
}

// specFault is checkSpecification's classifier, also reused by Check to
// stamp its own Finding with the same Rule. It returns ("", nil) when the
// specification conforms.
func (s *Server) specFault(fsys FeatureFS) (Rule, *RefusalError) {
	specPath := filepath.Join(fsys.Path, s.cfg.SpecificationFile)

	specBytes, err := fs.ReadFile(fsys.FS, s.cfg.SpecificationFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return RuleSpecMissing, &RefusalError{
				Path:   specPath,
				Detail: s.cfg.SpecificationFile + " not found",
				Fix:    fmt.Sprintf("write a %s with a %q heading and re-run", s.cfg.SpecificationFile, s.cfg.ProgressHeading),
				Err:    ErrMalformedFeature,
			}
		}

		return RuleSpecUnreadable, &RefusalError{Problem: *newProblem(specPath, err, false), Err: ErrMalformedFeature}
	}

	if line, delim, unterminated := markdown.UnterminatedFence(string(specBytes)); unterminated {
		return RuleFence, &RefusalError{
			Path:   specPath,
			Detail: fmt.Sprintf("specification has an unclosed %s fence opened at line %d", delim, line),
			Fix:    "close the fence and re-run",
			Line:   line,
			Err:    ErrMalformedFeature,
		}
	}

	if _, found := markdown.Section(string(specBytes), s.cfg.ProgressHeading); !found {
		return RuleHeading, &RefusalError{
			Path:   specPath,
			Detail: fmt.Sprintf("no %q heading found", s.cfg.ProgressHeading),
			Fix:    fmt.Sprintf("add a %q heading to the specification", s.cfg.ProgressHeading),
			Err:    ErrMalformedFeature,
		}
	}

	return "", nil
}

// readStateFile reads feature's state file and refuses with a
// *RefusalError wrapping ErrMalformedFeature when it is absent, unreadable,
// or carries an unclosed fenced code block — an open fence would otherwise
// make every heading after it read as silently absent.
func (s *Server) readStateFile(fsys FeatureFS) ([]byte, error) {
	statePath := filepath.Join(fsys.Path, s.cfg.StateFile)

	stateBytes, err := fs.ReadFile(fsys.FS, s.cfg.StateFile)
	if err != nil {
		return nil, &RefusalError{Problem: *newProblem(statePath, err, false), Err: ErrMalformedFeature}
	}

	if line, delim, unterminated := markdown.UnterminatedFence(string(stateBytes)); unterminated {
		return nil, &RefusalError{
			Path:   statePath,
			Detail: fmt.Sprintf("state file has an unclosed %s fence opened at line %d", delim, line),
			Fix:    "close the fence and re-run",
			Line:   line,
			Err:    ErrMalformedFeature,
		}
	}

	return stateBytes, nil
}

// readSteps parses the frontmatter of every entry dirEntries recognizes as
// a step file, sorted by step number.
func readSteps(fsys fs.FS, pattern stepfile.Pattern, dirEntries []os.DirEntry) ([]stepEntry, error) {
	var steps []stepEntry

	for _, e := range dirEntries {
		if e.IsDir() {
			continue
		}

		n, ok := pattern.Number(e.Name())
		if !ok {
			continue
		}

		body, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("assemble: %w", err)
		}

		fm, rest, err := stepfile.ParseFrontmatter(body)
		if err != nil {
			return nil, &stepFrontmatterError{name: e.Name(), err: err}
		}

		steps = append(steps, stepEntry{number: n, fm: fm, rest: rest})
	}

	sort.Slice(steps, func(i, j int) bool { return steps[i].number < steps[j].number })

	return steps, nil
}

// stepFromEntry extracts a Step's title, acceptance criteria and checklist
// from e's parsed body, using cfg's configured headings.
func stepFromEntry(e stepEntry, cfg config.Config) *Step {
	// e.rest, never the whole file: a "#" inside a YAML frontmatter value
	// must never be read as a heading.
	body := string(e.rest)

	// ok discarded: Step carries no Found flag for Title, so a step file
	// with no "# " line renders as an empty title, not a distinguishable
	// "missing" state.
	title, _ := markdown.Title(body)
	acceptance, acceptanceFound := markdown.Section(body, cfg.AcceptanceHeading)
	checklist, checklistFound := markdown.Section(body, cfg.ChecklistHeading)

	return &Step{
		ID:         e.fm.ID,
		Title:      title,
		Acceptance: Section{Heading: cfg.AcceptanceHeading, Body: acceptance, Found: acceptanceFound},
		Checklist:  Section{Heading: cfg.ChecklistHeading, Body: checklist, Found: checklistFound},
	}
}

// stateSections returns one Section per cfg.StateHeadings entry, in
// configured order, each body extracted from body.
func stateSections(body string, cfg config.Config) []Section {
	headings := cfg.StateHeadings.Ordered()
	sections := make([]Section, 0, len(headings))

	for _, heading := range headings {
		text, found := markdown.Section(body, heading)
		sections = append(sections, Section{Heading: heading, Body: text, Found: found})
	}

	return sections
}
