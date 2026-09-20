package assemble

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/stepfile"
)

// FeatureStatus is one feature's status line: its directory name, how many
// of its step files are done, how many it has in total, the pattern.ID of
// the lowest-numbered not-done step (empty when there is none — the
// caller's renderer, not FeatureStatus, is where that becomes "-"), and how
// many not-done steps are blocked on an unfinished dependency.
type FeatureStatus struct {
	Name    string
	Done    int
	Total   int
	Next    string
	Blocked int
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
// direct dependencies only, and an id naming no step file blocks. Status
// propagates an error from a missing or unreadable feature root, or from a
// step file whose frontmatter does not parse, rather than degrading into a
// partial result.
func (s *Server) Status(_ context.Context) ([]FeatureStatus, error) {
	topRoot, err := os.OpenRoot(filepath.Join(s.root, s.cfg.FeatureDirectory))
	if err != nil {
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
		if !e.IsDir() {
			continue
		}

		row, err := featureStatus(topRoot, pattern, e.Name())
		if err != nil {
			return nil, err
		}

		rows = append(rows, row)
	}

	return rows, nil
}

// featureStatus reads one feature directory, name, under topRoot and
// summarizes it as a FeatureStatus.
func featureStatus(topRoot *os.Root, pattern stepfile.Pattern, name string) (FeatureStatus, error) {
	root, err := topRoot.OpenRoot(name)
	if err != nil {
		return FeatureStatus{}, fmt.Errorf("assemble: %w", err)
	}
	defer func() { _ = root.Close() }()

	dirEntries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return FeatureStatus{}, fmt.Errorf("assemble: %w", err)
	}

	steps, err := readSteps(root, pattern, dirEntries)
	if err != nil {
		return FeatureStatus{}, err
	}

	row := FeatureStatus{Name: name, Total: len(steps)}

	done := make(map[string]bool, len(steps))

	for _, e := range steps {
		if e.fm.Done() {
			row.Done++
			done[pattern.ID(e.number)] = true
		}
	}

	for _, e := range steps {
		if e.fm.Done() {
			continue
		}

		if row.Next == "" {
			row.Next = pattern.ID(e.number)
		}

		for _, dep := range e.fm.DependsOn {
			if !done[dep] {
				row.Blocked++
				break
			}
		}
	}

	return row, nil
}
