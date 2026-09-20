package assemble

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/koblas/brief/internal/platform/conform"
	"github.com/koblas/brief/internal/platform/stepfile"
)

// Severity is a Finding's urgency.
type Severity string

const (
	// SeverityError marks a Finding on a feature still in flight — any step
	// not done, or a step file that could not be read or parsed (R18).
	SeverityError Severity = "ERROR"
	// SeverityWarn marks the same fault on a feature whose every step reads
	// as done.
	SeverityWarn Severity = "WARN"
)

// Finding is one fault Check found in a feature's on-disk layout that
// scaffold.Finish would now refuse to write over. Path is absolute; Line is
// the 1-based line within Path the fault concerns, 0 when it names the
// whole file. Detail is the same string the write-path refusal would
// print — Finding carries no Fix: the refusal copy's "... and retry" is
// write-path language with no meaning in a report of a tree Finish was
// never asked to write.
type Finding struct {
	Severity Severity
	Path     string
	Line     int
	Detail   string
}

// Check reports every fault in feature's on-disk layout that
// scaffold.Finish would now refuse to write over — the backstop role R18
// assigns it: Finish makes each fault unwritable going forward, Check
// reports one that predates the tool. feature == "" checks every feature
// directory under the configured feature directory, in fs.ReadDir's
// documented byte order — the same order Status uses; a non-directory,
// non-symlink entry is skipped, matching Status's own stance on a regular
// file. feature naming one directory checks only that feature, returning
// ErrNoSuchFeature when it has none, and refusing the same way when feature
// is "." or ".." or contains a path separator — none of those name a
// feature this configuration knows about. A missing feature-directory root
// is zero features, not an error, when feature == "": Check returns
// (nil, nil), matching Status. The same missing root refuses with
// ErrNoSuchFeature when feature names one, since the feature obviously has
// no directory when the root holding it does not exist either.
//
// A feature directory (or the single named feature) that exists but cannot
// be opened or listed — a permission failure, most often — contributes a
// Finding naming it rather than silently dropping it: an empty result must
// never be confused with "conforming", the exact failure this backstop
// exists to prevent. A symlink where a feature directory is expected is
// marked the same way, never followed, matching Status's stance: brief
// does not read through a symbolic link in the feature directory.
//
// Findings are ordered: features in ReadDir order (or the single named
// feature); within a feature, the specification (C1), then the state file
// — C2 (missing or unreadable), C3 (over cfg.StateCapLines), C4 (an
// unterminated fence), C5 (missing one of cfg.StateHeadings) — then step
// files in ascending step-number order; within a step, C6 (frontmatter
// absent or unparseable), C7 (a done step's unticked checklist item), C8
// (a depends-on id naming no step file), C9 (a self-dependency), C10 (the
// step's handoff file over cfg.HandoffCapLines).
//
// Check narrows the population Finish's band applies to, never the
// predicate: an open step's unticked checklist item, and an open step's
// known-but-unmet dependency — already counted by Status's Blocked — are
// ordinary in-progress work, not findings. C8 and C9 carry no such
// narrowing: a self-dependency or a dangling depends-on id is a fault on a
// done step exactly as much as an open one. Every other rule applies
// without narrowing too, including C10: a done or open step's over-cap
// handoff is a finding either way, and a missing handoff file is never
// one, since scaffold.Finish's own re-finish exemption (R16) treats it as
// nothing to diverge from.
//
// Check walks each feature's step files itself rather than through
// readSteps, which returns on the first unreadable or unparseable step
// file: reusing it here would collapse a whole feature's C7-C10 findings
// behind its first bad step file. A step whose frontmatter cannot be read
// or does not parse is itself a finding (C6), and Check still evaluates
// C10 for it, since the handoff cap does not depend on frontmatter.
//
// Severity is decided once per feature, after every step file is walked:
// SeverityError when any step is not done or could not be read or parsed;
// SeverityWarn when every step reads as done. A feature with no step files
// is vacuously "every step done" and takes SeverityWarn. A feature-level
// Finding — an unreadable or symlinked feature directory, or a feature
// whose step files could not be listed at all — always takes
// SeverityError: its doneness cannot be measured, and treating the
// unmeasurable case as anything less would understate it.
func (s *Server) Check(_ context.Context, feature string) ([]Finding, error) {
	pattern, err := stepfile.Compile(s.cfg.StepFilePattern)
	if err != nil {
		return nil, fmt.Errorf("assemble: %w", err)
	}

	handoffPattern, err := stepfile.CompileHandoff(pattern, s.cfg.HandoffFileSuffix, s.cfg.StateFile, s.cfg.SpecificationFile)
	if err != nil {
		return nil, fmt.Errorf("assemble: %w", err)
	}

	if feature != "" && !validFeatureArgument(feature) {
		return nil, ErrNoSuchFeature
	}

	topRoot, err := os.OpenRoot(filepath.Join(s.root, s.cfg.FeatureDirectory))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("assemble: %w", err)
		}

		if feature == "" {
			return nil, nil
		}

		return nil, ErrNoSuchFeature
	}
	defer func() { _ = topRoot.Close() }()

	if feature != "" {
		return s.checkNamedFeature(topRoot, feature, pattern, handoffPattern)
	}

	entries, err := fs.ReadDir(topRoot.FS(), ".")
	if err != nil {
		return nil, fmt.Errorf("assemble: %w", err)
	}

	var all []Finding

	for _, e := range entries {
		featurePath := filepath.Join(s.root, s.cfg.FeatureDirectory, e.Name())

		switch {
		case e.Type()&fs.ModeSymlink != 0:
			all = append(all, symlinkFeatureFinding(featurePath))
		case e.IsDir():
			root, err := s.openRoot(topRoot, e.Name())
			if err != nil {
				all = append(all, unreadableFeatureFinding(featurePath, err))
				continue
			}

			all = append(all, s.checkFeatureDir(root, pattern, handoffPattern, featurePath)...)

			_ = root.Close()
		}
	}

	return all, nil
}

// validFeatureArgument reports whether feature is a well-formed single
// path component: not empty, not "." or "..", and free of any
// os.IsPathSeparator character — "/" everywhere, "/" and "\" on Windows,
// never "\" alone on POSIX, where it is an ordinary filename character.
// Check rejects everything else before it ever reaches OpenRoot, so a
// caller cannot walk it into the feature-directory root itself or a
// directory outside any feature.
func validFeatureArgument(feature string) bool {
	if feature == "." || feature == ".." {
		return false
	}

	for i := range len(feature) {
		if os.IsPathSeparator(feature[i]) {
			return false
		}
	}

	return true
}

// checkNamedFeature runs Check's rules against the single feature named by
// feature, under topRoot. It Lstats feature before opening it, so a
// symlink is marked rather than followed — the same stance the
// all-features loop in Check takes — and a missing entry refuses with
// ErrNoSuchFeature rather than being folded into an "unreadable" Finding.
func (s *Server) checkNamedFeature(topRoot *os.Root, feature string, pattern stepfile.Pattern, handoffPattern stepfile.HandoffPattern) ([]Finding, error) {
	featurePath := filepath.Join(s.root, s.cfg.FeatureDirectory, feature)

	info, err := topRoot.Lstat(feature)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNoSuchFeature
		}

		return nil, fmt.Errorf("assemble: %w", err)
	}

	if info.Mode()&fs.ModeSymlink != 0 {
		return []Finding{symlinkFeatureFinding(featurePath)}, nil
	}

	if !info.IsDir() {
		return nil, ErrNoSuchFeature
	}

	root, err := s.openRoot(topRoot, feature)
	if err != nil {
		return []Finding{unreadableFeatureFinding(featurePath, err)}, nil
	}
	defer func() { _ = root.Close() }()

	return s.checkFeatureDir(root, pattern, handoffPattern, featurePath), nil
}

// symlinkFeatureFinding is the Finding Check reports for a symlink where a
// feature directory is expected: SeverityError, since a symlink is never
// read through, so nothing about what it points at can be measured.
func symlinkFeatureFinding(featurePath string) Finding {
	return Finding{Severity: SeverityError, Path: featurePath, Detail: "is a symbolic link, not read as a feature directory"}
}

// unreadableFeatureFinding is the Finding Check reports for a feature
// directory that exists but could not be opened as its own root — most
// often a permission failure. newProblem's Detail (never its Fix, which
// Finding does not carry) becomes the Finding's own Detail; SeverityError,
// since an unreadable feature's doneness cannot be measured.
func unreadableFeatureFinding(featurePath string, err error) Finding {
	problem := newProblem(featurePath, err, false)

	return Finding{Severity: SeverityError, Path: problem.Path, Detail: problem.Detail}
}

// checkFeatureDir runs every rule Check owns against one feature directory
// (root, rooted at featurePath) and assigns the one severity every finding
// in it shares.
func (s *Server) checkFeatureDir(root *os.Root, pattern stepfile.Pattern, handoffPattern stepfile.HandoffPattern, featurePath string) []Finding {
	// Capacity 4 is a rough guess (C1 contributes at most one, C2-C5 at
	// most one apiece but C2 excludes the rest, so at most three from the
	// state block), not a hard bound — append still grows it past that for
	// a feature whose step files contribute more.
	findings := make([]Finding, 0, 4)

	findings = append(findings, s.checkSpecFindings(root, featurePath)...)
	findings = append(findings, s.checkStateFindings(root, featurePath)...)

	stepFindings, inFlight := s.checkStepFindings(root, pattern, handoffPattern, featurePath)
	findings = append(findings, stepFindings...)

	sev := SeverityWarn
	if inFlight {
		sev = SeverityError
	}

	for i := range findings {
		findings[i].Severity = sev
	}

	return findings
}

// checkSpecFindings is C1: it reuses checkSpecification, the exact rule
// Start refuses a feature's specification against, and renders its
// *RefusalError as at most one Finding.
func (s *Server) checkSpecFindings(root *os.Root, featurePath string) []Finding {
	err := s.checkSpecification(root, featurePath)
	if err == nil {
		return nil
	}

	var refusal *RefusalError
	if !errors.As(err, &refusal) {
		return nil
	}

	return []Finding{{Path: refusal.Path, Line: refusal.Line, Detail: refusal.Detail}}
}

// checkStateFindings is C2-C5. C2 (missing or unreadable) gates the rest:
// without a readable body there is nothing for conform's predicates to
// measure. C3, C4 and C5 each run independently of one another — unlike
// scaffold.Finish's refuse-at-the-first-fault band, Check is a report and
// stops at none of them. The state body is read directly through root
// rather than through readStateFile, whose own unterminated-fence refusal
// copy would be a second definition of the fault conform.UnterminatedFence
// already owns.
func (s *Server) checkStateFindings(root *os.Root, featurePath string) []Finding {
	statePath := filepath.Join(featurePath, s.cfg.StateFile)

	stateBytes, err := root.ReadFile(s.cfg.StateFile)
	if err != nil {
		problem := newProblem(statePath, err, false)

		return []Finding{{Path: problem.Path, Detail: problem.Detail}}
	}

	var findings []Finding

	if v := conform.OverCap(stateBytes, "state", s.cfg.StateCapLines); v != nil {
		// conform.OverCap stays line-less, so the write path's own pinned
		// refusal bytes never move; a cap finding's line is cap+1, the
		// first line over it, set here at the call site instead.
		findings = append(findings, Finding{Path: statePath, Line: s.cfg.StateCapLines + 1, Detail: v.Problem})
	}

	if v := conform.UnterminatedFence(stateBytes, "state"); v != nil {
		findings = append(findings, Finding{Path: statePath, Line: v.Line, Detail: v.Problem})
	}

	if v := conform.MissingHeading(stateBytes, "state", s.cfg.StateHeadings); v != nil {
		findings = append(findings, Finding{Path: statePath, Line: v.Line, Detail: v.Problem})
	}

	return findings
}

// parsedStep is one step file entry discovered while walking a feature
// directory for checkStepFindings: its step number and name, its body when
// readable, and whichever of readErr/parseErr stopped Check from reaching
// its frontmatter.
type parsedStep struct {
	name     string
	number   int
	body     []byte
	fm       stepfile.Frontmatter
	readErr  error
	parseErr error
}

// checkStepFindings is C6-C10, walked once per feature over every step file
// pattern recognizes, in ascending step-number order. It returns the
// findings and whether the feature reads as still in flight (the severity
// rule Check's own doc comment states): true when any step is not done, or
// could not be read or parsed, or the step files could not be listed at
// all.
//
// A listing failure — the directory opened but could not be read, most
// often a permission failure on the directory itself rather than on
// OpenRoot — becomes its own Finding naming featurePath, the same
// "unreadable, so report it rather than drop it" stance Check takes on the
// feature directory one level up; it is not folded into checkFeatureDir's
// existing findings silently, since 0 findings here would otherwise read
// as "conforming".
//
// The dependency index is built once, over every step file in the feature
// including the one being evaluated, the same way
// scaffold.checkStepDependencies builds it for a single target step: a
// step file that cannot be read or parsed is Recorded as a known, not-done
// step (a zero Frontmatter) rather than skipped, so it blocks a dependant
// with the "is not finished" copy rather than the wrong "names no step
// file" one, and a self-dependency is reachable the same way it is in
// Finish.
func (s *Server) checkStepFindings(root *os.Root, pattern stepfile.Pattern, handoffPattern stepfile.HandoffPattern, featurePath string) ([]Finding, bool) {
	dirEntries, err := s.readDir(root)
	if err != nil {
		problem := newProblem(featurePath, err, false)

		return []Finding{{Path: problem.Path, Detail: problem.Detail}}, true
	}

	var parsed []parsedStep

	for _, e := range dirEntries {
		if e.IsDir() {
			continue
		}

		n, ok := pattern.Number(e.Name())
		if !ok {
			continue
		}

		ps := parsedStep{name: e.Name(), number: n}

		body, err := root.ReadFile(e.Name())
		if err != nil {
			ps.readErr = err
		} else {
			ps.body = body

			fm, _, err := stepfile.ParseFrontmatter(body)
			if err != nil {
				ps.parseErr = err
			} else {
				ps.fm = fm
			}
		}

		parsed = append(parsed, ps)
	}

	sort.Slice(parsed, func(i, j int) bool { return parsed[i].number < parsed[j].number })

	idx := stepfile.NewDependencyIndex()
	for _, ps := range parsed {
		idx.Record(pattern.ID(ps.number), ps.fm)
	}

	var findings []Finding

	inFlight := false

	for _, ps := range parsed {
		stepPath := filepath.Join(featurePath, ps.name)
		stepID := pattern.ID(ps.number)

		switch {
		case ps.readErr != nil:
			findings = append(findings, Finding{Path: stepPath, Detail: fmt.Sprintf("step file cannot be read: %v", ps.readErr)})
			inFlight = true
		case ps.parseErr != nil:
			findings = append(findings, Finding{Path: stepPath, Detail: fmt.Sprintf("frontmatter does not parse: %v", ps.parseErr)})
			inFlight = true
		default:
			if !ps.fm.Done() {
				inFlight = true
			}

			findings = append(findings, checkStepChecklistFinding(s.cfg.ChecklistHeading, ps, stepPath)...)
			findings = append(findings, checkStepDependencyFindings(idx, ps.fm, stepID, stepPath)...)
		}

		if v := checkHandoffCapFinding(root, handoffPattern, ps.number, s.cfg.HandoffCapLines, featurePath); v != nil {
			findings = append(findings, *v)
		}
	}

	return findings, inFlight
}

// checkStepChecklistFinding is C7: an unticked checklist item, but only on
// a done step — an open step's unticked item is ordinary in-progress work,
// exempted by checkStepFindings's own default-branch guard.
func checkStepChecklistFinding(heading string, ps parsedStep, stepPath string) []Finding {
	if !ps.fm.Done() {
		return nil
	}

	v := conform.OpenChecklistItem(ps.body, heading)
	if v == nil {
		return nil
	}

	return []Finding{{Path: stepPath, Line: v.Line, Detail: v.Problem}}
}

// checkStepDependencyFindings is C8 and C9. idx.FirstUnmet is a refusal
// predicate — it stops at the first unmet id and short-circuits on
// fm.Done() — so checkStepDependencyFindings does not call it: a report
// must enumerate every fault, not just the first, and must not exempt a
// done step from either the self-dependency (C9) or the dangling-reference
// (C8) fault. It walks every id in fm.DependsOn directly, in declaration
// order, and emits one Finding per id that is stepID itself (C9) or that
// idx.Known reports nothing recorded under (C8). An id that is Known and
// not stepID is an ordinary known-but-unmet dependency — already counted
// by Status's Blocked for an open step, and never a fault for a done one —
// and is never a finding. Copy matches
// scaffold.checkStepDependencies's own two refusal branches verbatim.
func checkStepDependencyFindings(idx *stepfile.DependencyIndex, fm stepfile.Frontmatter, stepID, stepPath string) []Finding {
	var findings []Finding

	for _, dep := range fm.DependsOn {
		switch {
		case dep == stepID:
			findings = append(findings, Finding{
				Path:   stepPath,
				Detail: fmt.Sprintf("step %q depends on %q, which is not finished", stepID, dep),
			})
		case !idx.Known(dep):
			findings = append(findings, Finding{
				Path:   stepPath,
				Detail: fmt.Sprintf("step %q depends on %q, which names no step file", stepID, dep),
			})
		}
	}

	return findings
}

// checkHandoffCapFinding is C10: number's handoff file, when it exists and
// reads without error, measured against cfg.HandoffCapLines by
// conform.OverCap. A missing or unreadable handoff file is never a
// finding — scaffold.Finish's own re-finish exemption (R16) already treats
// one as nothing to diverge from, and there is no body here to measure.
func checkHandoffCapFinding(root *os.Root, handoffPattern stepfile.HandoffPattern, number, limit int, featurePath string) *Finding {
	name := handoffPattern.Name(number)

	body, err := root.ReadFile(name)
	if err != nil {
		return nil
	}

	v := conform.OverCap(body, "handoff", limit)
	if v == nil {
		return nil
	}

	// conform.OverCap stays line-less; a cap finding's line is limit+1, the
	// first line over it, set here at the call site instead.
	return &Finding{Path: filepath.Join(featurePath, name), Line: limit + 1, Detail: v.Problem}
}
