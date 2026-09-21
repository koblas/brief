package assemble

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/koblas/brief/internal/platform/stepfile"
)

// NextStep names the lowest-numbered not-done step a FeatureStatus row
// reports: ID is pattern.ID(n) — the same token scaffold.findStepFile
// resolves for "brief finish" — Title is markdown.Title of the step body
// after frontmatter (empty when the step file has no "# " heading), and
// Path is the step file's own absolute path.
type NextStep struct {
	ID    string
	Title string
	Path  string
}

// FeatureStatus is one feature's status line: its directory name, its own
// absolute directory path, how many of its step files are done, how many
// it has in total, the lowest-numbered not-done step (nil when there is
// none), how many not-done steps are blocked on an unfinished dependency,
// and Problem, non-nil when the feature could not be read at all. When
// Problem is set, Done, Total, Next and Blocked stay at their zero values —
// a partial count would look measured and was not; Path is still set, so a
// caller can still name the feature's own directory.
type FeatureStatus struct {
	Name    string
	Path    string
	Done    int
	Total   int
	Next    *NextStep
	Blocked int
	Problem *Problem
}

// Complete reports whether row's feature is done: every step file read
// without error (Problem == nil), at least one step file exists (Total >
// 0), and every one of them is done (Done == Total). A feature with no
// step files at all is not complete — it has nothing to be complete about.
func (row FeatureStatus) Complete() bool {
	return row.Problem == nil && row.Total > 0 && row.Done == row.Total
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
// regular file entry is skipped with no row at all — the feature directory
// legitimately holds a README.md or a .DS_Store beside real features.
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
			rows = append(rows, featureStatus(topRoot, s.openRoot, s.readDir, pattern, e.Name(), entryPath))
		case e.Type()&fs.ModeSymlink != 0:
			// A symlink is marked without being resolved or opened: brief
			// does not follow symbolic links in the feature directory, so
			// its target is irrelevant to what the row reports.
			rows = append(rows, FeatureStatus{
				Name: e.Name(),
				Path: entryPath,
				Problem: &Problem{
					Path:   entryPath,
					Detail: "is a symbolic link, not read as a feature directory",
					Fix:    "replace it with a real directory",
				},
			})
		default:
			// A regular file (or other non-directory, non-symlink entry)
			// is not a feature and produces no row: the feature directory
			// legitimately holds a README.md or a .DS_Store beside real
			// feature directories.
		}
	}

	return rows, nil
}

// featureStatus reads one feature directory, name, under topRoot and
// summarizes it as a FeatureStatus. A failure opening or listing the
// directory, or reading or parsing one of its step files, is degraded into
// the returned row's Problem rather than propagated — the first such
// failure wins, and the row's counts stay at their zero values.
// displayPath is name's absolute path, used to build Problem.Path. openRoot
// opens name under topRoot — (*os.Root).OpenRoot in production, a fake in a
// test that injects a permission failure independent of effective uid.
// readDir lists the opened root's own entries — s.readDir in production, a
// fake in a test that injects a listing failure the same way, independent
// of openRoot's own failure.
func featureStatus(
	topRoot *os.Root,
	openRoot func(*os.Root, string) (*os.Root, error),
	readDir func(*os.Root) ([]os.DirEntry, error),
	pattern stepfile.Pattern,
	name, displayPath string,
) FeatureStatus {
	root, err := openRoot(topRoot, name)
	if err != nil {
		return FeatureStatus{Name: name, Path: displayPath, Problem: newProblem(displayPath, err, false)}
	}
	defer func() { _ = root.Close() }()

	dirEntries, err := readDir(root)
	if err != nil {
		return FeatureStatus{Name: name, Path: displayPath, Problem: newProblem(displayPath, err, false)}
	}

	steps, err := readSteps(root, pattern, dirEntries)
	if err != nil {
		return FeatureStatus{Name: name, Path: displayPath, Problem: newProblem(displayPath, err, true)}
	}

	row := FeatureStatus{Name: name, Path: displayPath, Total: len(steps)}

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

		if row.Next == nil {
			title, _ := markdown.Title(string(e.rest))
			row.Next = &NextStep{
				ID:    pattern.ID(e.number),
				Title: title,
				Path:  filepath.Join(displayPath, pattern.Name(e.number)),
			}
		}

		if _, unmet := idx.FirstUnmet(e.fm); unmet {
			row.Blocked++
		}
	}

	return row
}
