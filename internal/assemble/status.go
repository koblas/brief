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
			rows = append(rows, s.statusRow(topRoot, pattern, e.Name(), entryPath))
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

// statusRow opens one feature directory, name, under topRoot as its own
// os.Root — a permission failure here is degraded into the row's Problem
// rather than propagated, the adapter-level counterpart to StatusFS's own
// degrade-not-propagate stance on every fault reachable once the directory
// is open — and delegates to StatusFS.
func (s *Server) statusRow(topRoot *os.Root, pattern stepfile.Pattern, name, displayPath string) FeatureStatus {
	root, err := s.openRoot(topRoot, name)
	if err != nil {
		return FeatureStatus{Name: name, Path: displayPath, Problem: newProblem(displayPath, err, false)}
	}
	defer func() { _ = root.Close() }()

	return s.StatusFS(FeatureFS{FS: root.FS(), Path: displayPath}, pattern)
}

// StatusFS is Status's core for one feature: fsys is that feature's own
// filesystem, already opened and confined the same way StartFS's fsys is,
// and pattern is the step-file pattern Status compiles once for every
// feature it walks. A failure listing fsys, reading or parsing one of its
// step files, or either of the two faults assemble.Start itself refuses a
// feature over — an unreadable or heading-less specification (specFault),
// or a missing or unreadable state file (readStateFile) — is degraded into
// the returned row's Problem rather than propagated: the first such failure
// wins, checked in that order (listing, then specification, then state,
// then step files — the same spec-then-state order Check applies), and the
// row's counts stay at their zero values. A row's Problem is therefore a
// subset of what would make Start refuse, not the whole set: Start also
// refuses on the briefed step's own missing "id:" or absent checklist
// heading, which StatusFS never reads far enough to see. Name is
// filepath.Base(fsys.Path).
func (s *Server) StatusFS(fsys FeatureFS, pattern stepfile.Pattern) FeatureStatus {
	name := filepath.Base(fsys.Path)

	dirEntries, err := fs.ReadDir(fsys.FS, ".")
	if err != nil {
		return FeatureStatus{Name: name, Path: fsys.Path, Problem: newProblem(fsys.Path, err, false)}
	}

	if _, refusal := s.specFault(fsys); refusal != nil {
		problem := refusal.Problem

		return FeatureStatus{Name: name, Path: fsys.Path, Problem: &problem}
	}

	if _, err := s.readStateFile(fsys); err != nil {
		problem := stateFaultProblem(fsys.Path, err)

		return FeatureStatus{Name: name, Path: fsys.Path, Problem: &problem}
	}

	steps, err := readSteps(fsys.FS, pattern, dirEntries)
	if err != nil {
		return FeatureStatus{Name: name, Path: fsys.Path, Problem: newProblem(fsys.Path, err, true)}
	}

	row := FeatureStatus{Name: name, Path: fsys.Path, Total: len(steps)}

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
				Path:  filepath.Join(fsys.Path, pattern.Name(e.number)),
			}
		}

		if _, unmet := idx.FirstUnmet(e.fm); unmet {
			row.Blocked++
		}
	}

	return row
}

// stateFaultProblem renders err — readStateFile's own error, always a
// *RefusalError by that function's contract — as the Problem StatusFS
// reports for a missing, unreadable or fence-broken state file. The type
// assertion falls back to newProblem for any other error shape rather than
// panicking, so a future readStateFile change that stops honoring its own
// contract degrades into an ordinary Problem instead of crashing Status.
func stateFaultProblem(displayPath string, err error) Problem {
	if refusal, ok := errors.AsType[*RefusalError](err); ok {
		return refusal.Problem
	}

	return *newProblem(displayPath, err, false)
}
