package assemble

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/koblas/brief/internal/platform/stepfile"
)

// Server reads feature directories under a project root, using cfg to
// name the feature directory, the step-file pattern, the state file and
// every heading Start extracts.
type Server struct {
	cfg  config.Config
	root string
}

// NewServer returns a Server rooted at root, using cfg for every path and
// heading it reads. Both arguments are required positionally: there is no
// optional dependency here for a functional option to default.
func NewServer(cfg config.Config, root string) *Server {
	return &Server{cfg: cfg, root: root}
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
// step file, and every section of the feature's state file. "Next" is the
// lowest-numbered step file whose frontmatter status is not "done";
// depends-on is parsed but ignored for ordering. Start returns
// ErrNoSuchFeature when feature has no directory. It returns a
// *RefusalError, wrapping ErrMalformedFeature, when the feature's
// specification is missing, unreadable, carries an unclosed fenced code
// block, or has no configured progress heading; when its state file is
// missing, unreadable or carries an unclosed fence; or when the briefed
// step's frontmatter carries no "id:" or its checklist heading is absent.
// A step file whose frontmatter is absent or does not parse also returns a
// *RefusalError, wrapping whatever sentinel readSteps produced. Checks run
// in that order — specification, then state file, then step files, then
// the briefed step — and the first failure wins, so Start never returns a
// Brief that silently omits inherited context or a malformed next step.
// Start reads only; it writes nothing to disk.
func (s *Server) Start(_ context.Context, feature string) (Brief, error) {
	featureDirPath := filepath.Join(s.root, s.cfg.FeatureDirectory)

	topRoot, err := os.OpenRoot(featureDirPath)
	if err != nil {
		return Brief{}, ErrNoSuchFeature
	}
	defer func() { _ = topRoot.Close() }()

	root, err := topRoot.OpenRoot(feature)
	if err != nil {
		return Brief{}, ErrNoSuchFeature
	}
	defer func() { _ = root.Close() }()

	featurePath := filepath.Join(featureDirPath, feature)

	if err := s.checkSpecification(root, featurePath); err != nil {
		return Brief{}, err
	}

	stateBytes, err := s.readStateFile(root, featurePath)
	if err != nil {
		return Brief{}, err
	}

	pattern, err := stepfile.Compile(s.cfg.StepFilePattern)
	if err != nil {
		return Brief{}, fmt.Errorf("assemble: %w", err)
	}

	dirEntries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return Brief{}, fmt.Errorf("assemble: %w", err)
	}

	steps, err := readSteps(root, pattern, dirEntries)
	if err != nil {
		return Brief{}, &RefusalError{Problem: *newProblem(featurePath, err, true), Err: err}
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
		stepPath := filepath.Join(featurePath, pattern.Name(e.number))

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

		break
	}

	return brief, nil
}

// checkSpecification reads feature's specification through root and
// refuses with a *RefusalError, wrapping ErrMalformedFeature, when it is
// absent, unreadable, carries an unclosed fenced code block, or has no
// line matching cfg.ProgressHeading — the model's rule (see doc.go) that a
// feature's progress list lives in its specification, and that omission is
// undetectable rather than nameable once a fence swallows it. Absent gets
// its own imperative rather than reusing the unreadable case's "make it
// readable": a file that does not exist cannot be made readable.
func (s *Server) checkSpecification(root *os.Root, featurePath string) error {
	specPath := filepath.Join(featurePath, s.cfg.SpecificationFile)

	specBytes, err := root.ReadFile(s.cfg.SpecificationFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &RefusalError{
				Path:   specPath,
				Detail: s.cfg.SpecificationFile + " not found",
				Fix:    fmt.Sprintf("write a %s with a %q heading and re-run", s.cfg.SpecificationFile, s.cfg.ProgressHeading),
				Err:    ErrMalformedFeature,
			}
		}

		return &RefusalError{Problem: *newProblem(specPath, err, false), Err: ErrMalformedFeature}
	}

	if line, delim, unterminated := markdown.UnterminatedFence(string(specBytes)); unterminated {
		return &RefusalError{
			Path:   specPath,
			Detail: fmt.Sprintf("specification has an unclosed %s fence opened at line %d", delim, line),
			Fix:    "close the fence and re-run",
			Line:   line,
			Err:    ErrMalformedFeature,
		}
	}

	if _, found := markdown.Section(string(specBytes), s.cfg.ProgressHeading); !found {
		return &RefusalError{
			Path:   specPath,
			Detail: fmt.Sprintf("no %q heading found", s.cfg.ProgressHeading),
			Fix:    fmt.Sprintf("add a %q heading to the specification", s.cfg.ProgressHeading),
			Err:    ErrMalformedFeature,
		}
	}

	return nil
}

// readStateFile reads feature's state file through root and refuses with a
// *RefusalError, wrapping ErrMalformedFeature, when it is absent,
// unreadable, or carries an unclosed fenced code block. stateSections
// below finds each configured heading's section by scanning forward for a
// terminator, the same way Section always has; an open fence makes that
// scan run to end of file, so every heading after the fence opens sits
// inside it and reads as absent — R10's "the worst this tool could
// produce": a brief that looks complete while silently omitting every
// inherited section. Refusing here means Start never returns that shape.
func (s *Server) readStateFile(root *os.Root, featurePath string) ([]byte, error) {
	statePath := filepath.Join(featurePath, s.cfg.StateFile)

	stateBytes, err := root.ReadFile(s.cfg.StateFile)
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

// readSteps reads and parses the frontmatter of every entry dirEntries
// recognizes as a step file by pattern, returning them sorted by step
// number — numeric order, never directory or lexicographic order.
func readSteps(root *os.Root, pattern stepfile.Pattern, dirEntries []os.DirEntry) ([]stepEntry, error) {
	var steps []stepEntry

	for _, e := range dirEntries {
		if e.IsDir() {
			continue
		}

		n, ok := pattern.Number(e.Name())
		if !ok {
			continue
		}

		body, err := root.ReadFile(e.Name())
		if err != nil {
			return nil, fmt.Errorf("assemble: %w", err)
		}

		fm, rest, err := stepfile.ParseFrontmatter(body)
		if err != nil {
			return nil, fmt.Errorf("assemble: %w", err)
		}

		steps = append(steps, stepEntry{number: n, fm: fm, rest: rest})
	}

	sort.Slice(steps, func(i, j int) bool { return steps[i].number < steps[j].number })

	return steps, nil
}

// stepFromEntry extracts a Step's title, acceptance criteria and
// checklist from e's parsed body — never the whole file, so a "#"
// character inside a YAML frontmatter value is never read as a heading —
// using cfg's configured headings.
func stepFromEntry(e stepEntry, cfg config.Config) *Step {
	body := string(e.rest)

	// ok is discarded deliberately: unlike Acceptance and Checklist, Step
	// carries no Found flag for Title, so a step file with no "# " line
	// renders as an empty title rather than a distinguishable "missing"
	// state — every step file this package reads is expected to open with
	// one, since stepSkeleton always writes it.
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
// configured order, with each body extracted from body — empty when the
// heading is not present in it.
func stateSections(body string, cfg config.Config) []Section {
	headings := cfg.StateHeadings.Ordered()
	sections := make([]Section, 0, len(headings))

	for _, heading := range headings {
		text, found := markdown.Section(body, heading)
		sections = append(sections, Section{Heading: heading, Body: text, Found: found})
	}

	return sections
}
