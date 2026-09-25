package assemble

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/koblas/brief/internal/platform/stepfile"
)

// NextStep names the lowest-numbered not-done step a FeatureStatus row
// reports: ID is pattern.ID(n), Title is the step body's heading after
// frontmatter (empty when there is none), and Path is the step file's
// absolute path.
type NextStep struct {
	ID    string
	Title string
	Path  string
}

// FeatureStatus is one feature's status line: its name, its directory
// path, its done/total step counts, its lowest-numbered not-done step
// (nil when there is none), how many not-done steps are blocked on an
// unfinished dependency, and Problem, non-nil when the feature could not
// be read at all — in which case the counts stay zero rather than partial.
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
// configured feature directory, in fs.ReadDir's byte order. Next is the
// lowest-numbered not-done step; Blocked counts a not-done step whose
// depends-on names no done step, direct dependencies only. A missing
// feature root is zero features, not an error. A feature directory or
// step file that cannot be read or parsed is degraded into that row's
// Problem instead, so one malformed feature never blinds Status to the
// rest.
func (s *Server) Status(_ context.Context) ([]FeatureStatus, error) {
	topRoot, err := s.openFeatureDir(filepath.Join(s.root, s.cfg.FeatureDirectory))
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

// statusRow opens feature directory name under topRoot and delegates to
// StatusFS; an open failure here is degraded into the row's Problem too.
func (s *Server) statusRow(topRoot dirFS, pattern stepfile.Pattern, name, displayPath string) FeatureStatus {
	root, err := topRoot.OpenRoot(name)
	if err != nil {
		return FeatureStatus{Name: name, Path: displayPath, Problem: newProblem(displayPath, err, false)}
	}
	defer func() { _ = root.Close() }()

	return s.StatusFS(FeatureFS{FS: root.FS(), Path: displayPath}, pattern)
}

// StatusFS is Status's core for one feature. A failure listing fsys,
// reading its specification or state file, or reading/parsing a step file
// is degraded into the returned row's Problem rather than propagated —
// checked in that order, first failure wins — leaving the row's counts at
// zero. This is a subset of what makes Start refuse: Start also refuses on
// the briefed step's own missing "id:" or absent checklist heading, which
// StatusFS never reads far enough to see.
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

// stateFaultProblem renders err, readStateFile's own error, as the Problem
// StatusFS reports for a missing, unreadable or fence-broken state file.
func stateFaultProblem(displayPath string, err error) Problem {
	if refusal, ok := errors.AsType[*RefusalError](err); ok {
		return refusal.Problem
	}

	// Falls back to newProblem for any other error shape rather than
	// panicking, so an errant caller degrades into a Problem instead of
	// crashing Status.
	return *newProblem(displayPath, err, false)
}
