package assemble

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/stepfile"
)

// FeatureStatus is one feature's status line: its directory name, how many
// of its step files are done, how many it has in total, the pattern.ID of
// the lowest-numbered not-done step (empty when there is none — the
// caller's renderer, not FeatureStatus, is where that becomes "-"), how
// many not-done steps are blocked on an unfinished dependency, and
// Problem, non-nil when the feature could not be read at all. When Problem
// is set, Done, Total, Next and Blocked stay at their zero values — a
// partial count would look measured and was not.
type FeatureStatus struct {
	Name    string
	Done    int
	Total   int
	Next    string
	Blocked int
	Problem *Problem
}

// Status returns one FeatureStatus per feature directory under the
// configured feature directory, in fs.ReadDir's documented byte order of
// the directory names — io/fs.ReadDir and the fs.ReadDirFS interface it
// delegates to both document "sorted by filename", and os.Root.FS is
// documented to implement fs.ReadDirFS, so that order is relied on rather
// than re-established with a sort. Next is the lowest-numbered step file
// whose frontmatter status is not "done", printed as pattern.ID(n) rather
// than the frontmatter's own id — the same token scaffold.findStepFile
// resolves for "brief finish" — and, like Start, ignores depends-on for
// ordering. Blocked counts a not-done step as blocked when it declares at
// least one depends-on id that does not name a done step's pattern.ID(n):
// direct dependencies only, and an id naming no step file blocks. A
// missing feature root is zero features, not an error: Status returns
// (nil, nil). An unreadable top-level feature root, and an invalid
// configured step-file pattern, still propagate an error. A feature
// directory that cannot be opened or listed, or a step file inside it that
// cannot be read or whose frontmatter does not parse, is degraded into
// that row's Problem instead — one malformed feature never blinds Status
// to the rest. A symlink entry in the feature root is marked the same way
// without being followed, because Status never reads through it; a
// regular file entry is skipped with no row at all, matching
// SCENARIO-10's "docs/specifications/ legitimately holds a README.md"
// decision.
func (s *Server) Status(_ context.Context) ([]FeatureStatus, error) {
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

	pattern, err := stepfile.Compile(s.cfg.StepFilePattern)
	if err != nil {
		return nil, fmt.Errorf("assemble: %w", err)
	}

	var rows []FeatureStatus

	for _, e := range entries {
		entryPath := filepath.Join(s.root, s.cfg.FeatureDirectory, e.Name())

		switch {
		case e.IsDir():
			rows = append(rows, featureStatus(topRoot, pattern, e.Name(), entryPath))
		case e.Type()&fs.ModeSymlink != 0:
			// A symlink is marked without being resolved or opened: brief
			// does not follow symbolic links in the feature directory, so
			// its target is irrelevant to what the row reports.
			rows = append(rows, FeatureStatus{
				Name: e.Name(),
				Problem: &Problem{
					Path:   entryPath,
					Detail: "symbolic link is not read as a feature directory",
					Fix:    "replace it with a real directory",
				},
			})
		default:
			// A regular file (or other non-directory, non-symlink entry)
			// is not a feature and produces no row — SCENARIO-10's
			// decision, preserved: docs/specifications/ legitimately holds
			// a README.md or a .DS_Store beside real feature directories.
		}
	}

	return rows, nil
}

// featureStatus reads one feature directory, name, under topRoot and
// summarizes it as a FeatureStatus. A failure opening or listing the
// directory, or reading or parsing one of its step files, is degraded into
// the returned row's Problem rather than propagated — the first such
// failure wins, and the row's counts stay at their zero values.
// displayPath is name's absolute path, used to build Problem.Path.
func featureStatus(topRoot *os.Root, pattern stepfile.Pattern, name, displayPath string) FeatureStatus {
	root, err := topRoot.OpenRoot(name)
	if err != nil {
		return FeatureStatus{Name: name, Problem: newProblem(displayPath, err, false)}
	}
	defer func() { _ = root.Close() }()

	dirEntries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return FeatureStatus{Name: name, Problem: newProblem(displayPath, err, false)}
	}

	steps, err := readSteps(root, pattern, dirEntries)
	if err != nil {
		return FeatureStatus{Name: name, Problem: newProblem(displayPath, err, true)}
	}

	row := FeatureStatus{Name: name, Total: len(steps)}

	idx := stepfile.NewDependencyIndex()

	for _, e := range steps {
		idx.Record(pattern.ID(e.number), e.fm)

		if e.fm.Done() {
			row.Done++
		}
	}

	for _, e := range steps {
		if e.fm.Done() {
			continue
		}

		if row.Next == "" {
			row.Next = pattern.ID(e.number)
		}

		if _, unmet := idx.FirstUnmet(e.fm); unmet {
			row.Blocked++
		}
	}

	return row
}
