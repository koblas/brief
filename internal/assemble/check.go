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
	// not done, or a step file that could not be read or parsed.
	SeverityError Severity = "ERROR"
	// SeverityWarn marks the same fault on a feature whose every step reads
	// as done.
	SeverityWarn Severity = "WARN"
)

// Rule is the stable id a Finding's fault carries, so a script driving
// Check branches on this rather than parsing the English in Detail.
type Rule string

const (
	// RuleFeatureSymlink marks a symlink where a feature directory is expected.
	RuleFeatureSymlink Rule = "feature-symlink"
	// RuleFeatureUnreadable marks a feature directory that exists but could not be opened.
	RuleFeatureUnreadable Rule = "feature-unreadable"
	// RuleSpecMissing marks an absent specification file.
	RuleSpecMissing Rule = "spec-missing"
	// RuleSpecUnreadable marks a specification file that exists but could not be read.
	RuleSpecUnreadable Rule = "spec-unreadable"
	// RuleStateMissing marks an absent state file.
	RuleStateMissing Rule = "state-missing"
	// RuleStateUnreadable marks a state file that exists but could not be read.
	RuleStateUnreadable Rule = "state-unreadable"
	// RuleStepsUnlistable marks a feature directory whose step files could not be listed at all.
	RuleStepsUnlistable Rule = "steps-unlistable"
	// RuleStepUnreadable marks a step file that exists but could not be read.
	RuleStepUnreadable Rule = "step-unreadable"
	// RuleFrontmatter marks a step file whose frontmatter does not parse.
	RuleFrontmatter Rule = "frontmatter"
	// RuleFence marks a specification or state body with an unterminated fenced code block.
	RuleFence Rule = "fence"
	// RuleHeading marks a specification or state body missing one of its configured headings.
	RuleHeading Rule = "heading"
	// RuleStateCap marks a state body over cfg.StateCapLines.
	RuleStateCap Rule = "state-cap"
	// RuleHandoffCap marks a step's handoff file over cfg.HandoffCapLines.
	RuleHandoffCap Rule = "handoff-cap"
	// RuleChecklist marks a done step's unticked checklist item.
	RuleChecklist Rule = "checklist"
	// RuleDependsOn marks a step's self-dependency or a depends-on id naming no step file.
	RuleDependsOn Rule = "depends-on"
)

// Finding is one fault Check found in a feature's on-disk layout that
// scaffold.Finish would now refuse to write over. Path is absolute; Line
// is the 1-based line within Path the fault concerns, 0 for the whole
// file. Finding carries no Fix, unlike Problem. Feature is the owning
// feature's directory name, FeaturePath its absolute directory, and
// InFlight is the group-header discriminator GroupByFeature carries
// forward.
type Finding struct {
	Rule        Rule
	Severity    Severity
	Path        string
	Line        int
	Detail      string
	Feature     string
	FeaturePath string
	InFlight    bool
}

// Check reports every fault in feature's on-disk layout that
// scaffold.Finish would now refuse to write over, so a tree that predates
// a rule is still surfaced. feature == "" checks every feature directory,
// in fs.ReadDir's byte order; feature naming one directory checks only
// that feature, returning ErrNoSuchFeature when it has none. A feature
// directory that cannot be opened or listed, or that is a symlink,
// contributes a Finding naming it rather than being silently dropped: an
// empty result must never be confused with "conforming".
//
// Findings are ordered: features in ReadDir order; within a feature, the
// specification, then the state file, then step files in ascending
// step-number order and their own rules within each. Severity is decided
// once per feature: SeverityError when any step is not done, could not be
// read or parsed, or there are no step files at all; SeverityWarn when
// every step reads as done. A feature-level fault always takes
// SeverityError, since its doneness cannot be measured.
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

	topRoot, err := s.openFeatureDir(filepath.Join(s.root, s.cfg.FeatureDirectory))
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
			root, err := topRoot.OpenRoot(e.Name())
			if err != nil {
				all = append(all, unreadableFeatureFinding(featurePath, err))
				continue
			}

			all = append(all, s.CheckFS(FeatureFS{FS: root.FS(), Path: featurePath}, pattern, handoffPattern)...)

			_ = root.Close()
		}
	}

	return all, nil
}

// validFeatureArgument reports whether feature is a well-formed single
// path component: not empty, not "." or "..", and free of any
// os.IsPathSeparator character, so a caller cannot walk it into the
// feature-directory root itself or a directory outside any feature.
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

// checkNamedFeature runs Check's rules against the single feature named by
// feature, under topRoot. It Lstats feature before opening it, so a
// symlink is marked rather than followed, and a missing entry refuses with
// ErrNoSuchFeature rather than being folded into an "unreadable" Finding.
func (s *Server) checkNamedFeature(topRoot dirFS, feature string, pattern stepfile.Pattern, handoffPattern stepfile.HandoffPattern) ([]Finding, error) {
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

	root, err := topRoot.OpenRoot(feature)
	if err != nil {
		return []Finding{unreadableFeatureFinding(featurePath, err)}, nil
	}
	defer func() { _ = root.Close() }()

	return s.CheckFS(FeatureFS{FS: root.FS(), Path: featurePath}, pattern, handoffPattern), nil
}

// symlinkFeatureFinding is the Finding Check reports for a symlink where a
// feature directory is expected: SeverityError, since nothing about what
// it points at can be measured. It stamps its own
// Feature/FeaturePath/InFlight, since it never passes through CheckFS's
// severity loop.
func symlinkFeatureFinding(featurePath string) Finding {
	return Finding{
		Rule:        RuleFeatureSymlink,
		Severity:    SeverityError,
		Path:        featurePath,
		Detail:      "is a symbolic link, not read as a feature directory",
		Feature:     filepath.Base(featurePath),
		FeaturePath: featurePath,
		InFlight:    true,
	}
}

// unreadableFeatureFinding is the Finding Check reports for a feature
// directory that exists but could not be opened as its own root.
// SeverityError, since an unreadable feature's doneness cannot be
// measured.
func unreadableFeatureFinding(featurePath string, err error) Finding {
	problem := newProblem(featurePath, err, false)

	return Finding{
		Rule:        RuleFeatureUnreadable,
		Severity:    SeverityError,
		Path:        problem.Path,
		Detail:      problem.Detail,
		Feature:     filepath.Base(featurePath),
		FeaturePath: featurePath,
		InFlight:    true,
	}
}

// CheckFS is Check's core for one feature: fsys is that feature's own
// filesystem, already opened and confined. It runs every rule Check owns
// against fsys and assigns the one severity every finding in it shares.
// Unlike Check, it never produces the top-level symlink or
// unreadable-directory findings, since those concern the entry that would
// have named fsys, not fsys itself.
func (s *Server) CheckFS(fsys FeatureFS, pattern stepfile.Pattern, handoffPattern stepfile.HandoffPattern) []Finding {
	// Capacity 4 is a typical-case guess, not a bound; append grows past it.
	findings := make([]Finding, 0, 4)

	findings = append(findings, s.checkSpecFindings(fsys)...)
	findings = append(findings, s.checkStateFindings(fsys)...)

	stepFindings, inFlight := s.checkStepFindings(fsys, pattern, handoffPattern)
	findings = append(findings, stepFindings...)

	sev := SeverityWarn
	if inFlight {
		sev = SeverityError
	}

	name := filepath.Base(fsys.Path)

	for i := range findings {
		findings[i].Severity = sev
		findings[i].Feature = name
		findings[i].FeaturePath = fsys.Path
		findings[i].InFlight = inFlight
	}

	return findings
}

// checkSpecFindings reuses specFault, the same classifier checkSpecification
// refuses a specification against, and renders its *RefusalError as at
// most one Finding.
func (s *Server) checkSpecFindings(fsys FeatureFS) []Finding {
	rule, refusal := s.specFault(fsys)
	if refusal == nil {
		return nil
	}

	return []Finding{{Rule: rule, Path: refusal.Path, Line: refusal.Line, Detail: refusal.Detail}}
}

// checkStateFindings reports every state-file fault: missing or unreadable
// gates the rest, since there is nothing to measure without a readable
// body; over cfg.StateCapLines, an unterminated fence and a missing
// configured heading each run independently — unlike a refusal, Check
// stops at none of them.
func (s *Server) checkStateFindings(fsys FeatureFS) []Finding {
	statePath := filepath.Join(fsys.Path, s.cfg.StateFile)

	stateBytes, err := fs.ReadFile(fsys.FS, s.cfg.StateFile)
	if err != nil {
		problem := newProblem(statePath, err, false)

		rule := RuleStateUnreadable
		if errors.Is(err, fs.ErrNotExist) {
			rule = RuleStateMissing
		}

		return []Finding{{Rule: rule, Path: problem.Path, Detail: problem.Detail}}
	}

	var findings []Finding

	if v := conform.OverCap(stateBytes, "state", s.cfg.StateCapLines); v != nil {
		// conform.OverCap stays line-less, so the write path's own pinned
		// refusal bytes never move; a cap finding's line is cap+1, the
		// first line over it, set here at the call site instead.
		findings = append(findings, Finding{Rule: RuleStateCap, Path: statePath, Line: s.cfg.StateCapLines + 1, Detail: v.Problem})
	}

	if v := conform.UnterminatedFence(stateBytes, "state"); v != nil {
		findings = append(findings, Finding{Rule: RuleFence, Path: statePath, Line: v.Line, Detail: v.Problem})
	}

	if v := conform.MissingHeading(stateBytes, "state", s.cfg.StateHeadings); v != nil {
		findings = append(findings, Finding{Rule: RuleHeading, Path: statePath, Line: v.Line, Detail: v.Problem})
	}

	return findings
}

// parsedStep is one step file entry found while walking a feature
// directory for checkStepFindings, plus whichever of readErr/parseErr
// stopped Check from reaching its frontmatter.
type parsedStep struct {
	name     string
	number   int
	body     []byte
	fm       stepfile.Frontmatter
	readErr  error
	parseErr error
}

// checkStepFindings walks every step file pattern recognizes, in ascending
// step-number order, and reports each one's faults. It returns the
// findings and whether the feature reads as still in flight: true when any
// step is not done, could not be read or parsed, the step files could not
// be listed at all, or there are no step files at all. A listing failure
// becomes its own Finding naming fsys.Path rather than being silently
// folded away, since zero findings would otherwise read as "conforming".
func (s *Server) checkStepFindings(fsys FeatureFS, pattern stepfile.Pattern, handoffPattern stepfile.HandoffPattern) ([]Finding, bool) {
	dirEntries, err := fs.ReadDir(fsys.FS, ".")
	if err != nil {
		problem := newProblem(fsys.Path, err, false)

		return []Finding{{Rule: RuleStepsUnlistable, Path: problem.Path, Detail: problem.Detail}}, true
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

		body, err := fs.ReadFile(fsys.FS, e.Name())
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
		// ps.fm, even a zero Frontmatter for an unreadable/unparseable step:
		// Recording it as a known, not-done step blocks a dependant with the
		// "is not finished" detail rather than the wrong "names no step file".
		idx.Record(pattern.ID(ps.number), ps.fm)
	}

	var findings []Finding

	// A feature with no step files at all is not vacuously "every step
	// done": it is in flight, matching (FeatureStatus).Complete's own
	// Total > 0 requirement (Test_check_marks_a_zero_step_feature_in_flight_not_complete).
	inFlight := len(parsed) == 0

	for _, ps := range parsed {
		stepPath := filepath.Join(fsys.Path, ps.name)
		stepID := pattern.ID(ps.number)

		switch {
		case ps.readErr != nil:
			findings = append(findings, Finding{Rule: RuleStepUnreadable, Path: stepPath, Detail: fmt.Sprintf("step file cannot be read: %v", ps.readErr)})
			inFlight = true
		case ps.parseErr != nil:
			findings = append(findings, Finding{Rule: RuleFrontmatter, Path: stepPath, Detail: fmt.Sprintf("frontmatter does not parse: %v", ps.parseErr)})
			inFlight = true
		default:
			if !ps.fm.Done() {
				inFlight = true
			}

			findings = append(findings, checkStepChecklistFinding(s.cfg.ChecklistHeading, ps, stepPath)...)
			findings = append(findings, checkStepDependencyFindings(idx, ps.fm, stepID, stepPath)...)
		}

		if v := checkHandoffCapFinding(fsys, handoffPattern, ps.number, s.cfg.HandoffCapLines); v != nil {
			findings = append(findings, *v)
		}
	}

	return findings, inFlight
}

// checkStepChecklistFinding reports an unticked checklist item, but only
// on a done step; an open step's unticked item is ordinary in-progress work.
func checkStepChecklistFinding(heading string, ps parsedStep, stepPath string) []Finding {
	if !ps.fm.Done() {
		return nil
	}

	v := conform.OpenChecklistItem(ps.body, heading)
	if v == nil {
		return nil
	}

	return []Finding{{Rule: RuleChecklist, Path: stepPath, Line: v.Line, Detail: v.Problem}}
}

// checkStepDependencyFindings reports every id in fm.DependsOn that is
// stepID itself (a self-dependency) or that idx.Known reports nothing
// recorded under (a dangling reference), on a done step exactly as much as
// an open one — it does not use idx.FirstUnmet, which stops at the first
// unmet id and would hide the rest. A known, non-self id is an ordinary
// unmet dependency, never a finding.
func checkStepDependencyFindings(idx *stepfile.DependencyIndex, fm stepfile.Frontmatter, stepID, stepPath string) []Finding {
	var findings []Finding

	for _, dep := range fm.DependsOn {
		switch {
		case dep == stepID:
			findings = append(findings, Finding{
				Rule:   RuleDependsOn,
				Path:   stepPath,
				Detail: fmt.Sprintf("step %q depends on %q, which is not finished", stepID, dep),
			})
		case !idx.Known(dep):
			findings = append(findings, Finding{
				Rule:   RuleDependsOn,
				Path:   stepPath,
				Detail: fmt.Sprintf("step %q depends on %q, which names no step file", stepID, dep),
			})
		}
	}

	return findings
}

// FeatureFindings is one feature's findings, folded by GroupByFeature: Name
// and Path are the feature's own name and absolute directory, and InFlight
// is the group-header discriminator ("(in flight)" or "(complete)").
type FeatureFindings struct {
	Name     string
	Path     string
	InFlight bool
	Findings []Finding
}

// GroupByFeature folds findings into one FeatureFindings per distinct
// Finding.Feature, in first-appearance order.
func GroupByFeature(findings []Finding) []FeatureFindings {
	var groups []FeatureFindings

	index := make(map[string]int, len(findings))

	for _, f := range findings {
		i, ok := index[f.Feature]
		if !ok {
			i = len(groups)
			index[f.Feature] = i

			groups = append(groups, FeatureFindings{Name: f.Feature, Path: f.FeaturePath, InFlight: f.InFlight})
		}

		groups[i].Findings = append(groups[i].Findings, f)
	}

	return groups
}

// checkHandoffCapFinding measures number's handoff file against limit when
// it exists and reads without error. A missing or unreadable handoff file
// is never a finding: there is no body here to measure.
func checkHandoffCapFinding(fsys FeatureFS, handoffPattern stepfile.HandoffPattern, number, limit int) *Finding {
	name := handoffPattern.Name(number)

	body, err := fs.ReadFile(fsys.FS, name)
	if err != nil {
		return nil
	}

	v := conform.OverCap(body, "handoff", limit)
	if v == nil {
		return nil
	}

	// conform.OverCap stays line-less; a cap finding's line is limit+1, the
	// first line over it, set here at the call site instead.
	return &Finding{Rule: RuleHandoffCap, Path: filepath.Join(fsys.Path, name), Line: limit + 1, Detail: v.Problem}
}
