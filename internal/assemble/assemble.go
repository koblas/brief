package assemble

import (
	"context"
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
// ErrNoSuchFeature when feature has no directory, ErrMalformedFeature when
// the feature has no state file, and a wrapped error when a step file's
// frontmatter does not parse. Start reads only; it writes nothing to
// disk.
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

	stateBytes, err := root.ReadFile(s.cfg.StateFile)
	if err != nil {
		return Brief{}, ErrMalformedFeature
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
		return Brief{}, err
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

		brief.Step = stepFromEntry(e, s.cfg)

		break
	}

	return brief, nil
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

	title, _ := markdown.Title(body)
	acceptance, _ := markdown.Section(body, cfg.AcceptanceHeading)
	checklist, _ := markdown.Section(body, cfg.ChecklistHeading)

	return &Step{
		ID:         e.fm.ID,
		Title:      title,
		Acceptance: Section{Heading: cfg.AcceptanceHeading, Body: acceptance},
		Checklist:  Section{Heading: cfg.ChecklistHeading, Body: checklist},
	}
}

// stateSections returns one Section per cfg.StateHeadings entry, in
// configured order, with each body extracted from body — empty when the
// heading is not present in it.
func stateSections(body string, cfg config.Config) []Section {
	headings := cfg.StateHeadings.Ordered()
	sections := make([]Section, 0, len(headings))

	for _, heading := range headings {
		text, _ := markdown.Section(body, heading)
		sections = append(sections, Section{Heading: heading, Body: text})
	}

	return sections
}
