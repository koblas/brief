package scaffold

import (
	"context"
	"errors"
	"fmt"
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

// FinishNext is FinishResult's own "next open step" shape. Title is
// markdown.Title of the step body after its frontmatter, empty when the
// step file has no "# " heading; Path is the step file's own absolute
// path. The zero value (every field "") means "nothing open".
type FinishNext struct {
	ID, Title, Path string
}

// FinishResult is what Finish returns on success — either after its four
// writes land, or as the no-op case, which still returns the populated
// result rather than a zero value. Feature and Step are the arguments
// Finish was called with. Changed is false only for the no-op: every input
// already matched what was on disk. HandoffPath, StatePath, StepPath and
// SpecPath are absolute — the four files Finish wrote or, on a no-op,
// would have. Modified lists StatePath, StepPath and SpecPath, in that
// write order, when Changed; empty on a no-op. HandoffPath is never in
// Modified: it is this call's own output, named by its own field. Next
// names the lowest-numbered step file whose frontmatter status is not
// "done", counting the just-finished step as done and ignoring
// depends-on, so a blocked step can still be Next; the zero FinishNext
// when every other step is done. Dropped is every state entry the
// replacement body no longer carries, in old-file line order, empty but
// never nil when nothing was dropped; computed only on a genuine write.
type FinishResult struct {
	Feature, Step                    string
	Changed                          bool
	HandoffPath, StatePath, StepPath string
	SpecPath                         string
	Modified                         []string
	Next                             FinishNext
	Dropped                          []DroppedEntry
}

// Finish closes feature's step: it writes handoff to that step's own
// handoff file, replaces the feature's state file with state, then marks
// the step file's frontmatter status "done". handoff is checked against
// cfg.HandoffCapLines; state is checked against cfg.StateCapLines and
// against cfg.StateHeadings, which it must carry all four of (any order,
// an empty section valid). handoff carries no heading check: it is
// written verbatim to its own file and nothing reads it structurally.
//
// Every check runs, and every write body is computed, before the first
// byte reaches disk, in the order FinishFS's call sequence below performs
// them — so a refusal always names the first thing wrong. A re-finish of a
// step whose frontmatter already says done is decided by (refinish).verdict.
func (s *Server) Finish(_ context.Context, feature, step string, handoff, state []byte) (FinishResult, error) {
	featureDirPath := filepath.Join(s.root, s.cfg.FeatureDirectory)
	featurePath := filepath.Join(featureDirPath, feature)

	pattern, err := stepfile.Compile(s.cfg.StepFilePattern)
	if err != nil {
		return FinishResult{}, &RefusalError{
			Path:    featurePath,
			Problem: fmt.Sprintf("step-file-pattern %q is invalid: %v", s.cfg.StepFilePattern, err),
			Fix:     "fix step-file-pattern in .brief.yaml",
			Err:     stepfile.ErrInvalidPattern,
		}
	}

	handoffPattern, err := stepfile.CompileHandoff(pattern, s.cfg.HandoffFileSuffix, s.cfg.StateFile, s.cfg.SpecificationFile)
	if err != nil {
		return FinishResult{}, &RefusalError{
			Path:    featurePath,
			Problem: fmt.Sprintf("handoff-file-suffix %q is invalid: %v", s.cfg.HandoffFileSuffix, err),
			Fix:     "fix handoff-file-suffix in .brief.yaml",
			Err:     stepfile.ErrInvalidHandoffSuffix,
		}
	}

	top, root, err := s.openFeatureDir(featureDirPath, featurePath, feature)
	if err != nil {
		return FinishResult{}, err
	}
	defer func() { _ = top.Close() }()
	defer func() { _ = root.Close() }()

	return s.FinishFS(root, featurePath, feature, step, handoff, state, pattern, handoffPattern)
}

// FinishFS is Finish's core: fsys is an rwfs.FS already opened and
// confined to feature's own directory, featurePath is that directory's
// absolute OS path, and pattern/handoffPattern are the step-file and
// handoff-file patterns Finish compiles once before opening anything.
// FinishFS assumes fsys already names a real, contained feature directory.
func (s *Server) FinishFS(fsys rwfs.FS, featurePath, feature, step string, handoff, state []byte, pattern stepfile.Pattern, handoffPattern stepfile.HandoffPattern) (FinishResult, error) {
	stepFileName, stepNumber, entries, err := findStepFile(fsys, pattern, step)
	if err != nil {
		if !errors.Is(err, ErrNoSuchStep) {
			return FinishResult{}, err
		}

		return FinishResult{}, &RefusalError{
			Path:    featurePath,
			Problem: fmt.Sprintf("no step %q in %s", step, feature),
			Fix:     knownStepsFix(knownStepIDs(pattern, entries), feature),
			Err:     ErrNoSuchStep,
		}
	}

	stepPath := filepath.Join(featurePath, stepFileName)
	handoffName := handoffPattern.Name(stepNumber)

	stepBody, err := fsys.ReadFile(stepFileName)
	if err != nil {
		return FinishResult{}, fmt.Errorf("scaffold: %w", peelReadErr(err))
	}

	fm, _, err := stepfile.ParseFrontmatter(stepBody)
	if err != nil {
		return FinishResult{}, &RefusalError{
			Path:    stepPath,
			Problem: fmt.Sprintf("frontmatter does not parse: %v", err),
			Fix:     "fix the step file's YAML frontmatter",
			Err:     ErrMalformedFeature,
		}
	}

	if refusal := checkArgumentCap(handoff, HandoffSource, "handoff", s.cfg.HandoffCapLines); refusal != nil {
		return FinishResult{}, refusal
	}

	if refusal := checkArgumentCap(state, StateSource, "state", s.cfg.StateCapLines); refusal != nil {
		return FinishResult{}, refusal
	}

	if refusal := checkArgumentFence(state, StateSource, "state"); refusal != nil {
		return FinishResult{}, refusal
	}

	if refusal := checkArgumentHeadings(state, StateSource, "state", s.cfg.StateHeadings); refusal != nil {
		return FinishResult{}, refusal
	}

	if refusal := checkStepChecklist(stepBody, stepPath, s.cfg.ChecklistHeading); refusal != nil {
		return FinishResult{}, refusal
	}

	if err := checkStepDependencies(fsys, pattern, fm, step, stepPath); err != nil {
		return FinishResult{}, err
	}

	specPath := filepath.Join(featurePath, s.cfg.SpecificationFile)

	specBytes, err := fsys.ReadFile(s.cfg.SpecificationFile)
	if err != nil {
		return FinishResult{}, &RefusalError{
			Path:    specPath,
			Problem: "specification file is missing",
			Fix:     "scaffold the feature again to restore it",
			Err:     ErrMalformedFeature,
		}
	}

	newSpec, err := tickProgressEntry(string(specBytes), s.cfg.ProgressHeading, step)
	if err != nil {
		return FinishResult{}, progressRefusal(specPath, s.cfg, feature, step, err)
	}

	statePath := filepath.Join(featurePath, s.cfg.StateFile)

	stateInfo, err := fsys.Lstat(s.cfg.StateFile)
	if err != nil || !stateInfo.Mode().IsRegular() {
		return FinishResult{}, &RefusalError{
			Path:    statePath,
			Problem: "state file is missing",
			Fix:     "scaffold the feature again to restore it",
			Err:     ErrMalformedFeature,
		}
	}

	stateBytes, err := fsys.ReadFile(s.cfg.StateFile)
	if err != nil {
		return FinishResult{}, fmt.Errorf("scaffold: %w", peelReadErr(err))
	}

	handoffPath := filepath.Join(featurePath, handoffName)
	existingHandoff, handoffReadErr := fsys.ReadFile(handoffName)
	handoffMatches := handoffReadErr == nil && string(existingHandoff) == string(handoff)

	r := refinish{
		done:            fm.Done(),
		handoffRecorded: handoffReadErr == nil,
		handoffMatches:  handoffMatches,
		stateMatches:    string(state) == string(stateBytes),
		specTicked:      newSpec == string(specBytes),
	}

	newStepBody, err := stepfile.SetStatus(stepBody, "done")
	if err != nil {
		return FinishResult{}, &RefusalError{
			Path:    stepPath,
			Problem: `frontmatter has no "status:" field`,
			Fix:     `add a "status:" key to the step file's frontmatter`,
			Err:     err,
		}
	}

	// Computed here, before any write, so it is present even on the no-op.
	next := nextOpenStep(fsys, pattern, entries, stepNumber, featurePath)

	switch r.verdict() {
	case refinishNoop:
		return FinishResult{
			Feature: feature, Step: step, Changed: false,
			HandoffPath: handoffPath, StatePath: statePath, StepPath: stepPath, SpecPath: specPath,
			Modified: []string{}, Next: next, Dropped: []DroppedEntry{},
		}, nil
	case refinishHandoffDiverged:
		return FinishResult{}, alreadyFinishedRefusal(handoffPath, step, "handoff")
	case refinishStateDiverged:
		return FinishResult{}, alreadyFinishedRefusal(statePath, step, "state")
	case refinishWrite:
		// Falls through to the four writes below.
	}

	// Computed after the verdict, so it never changes which fault a
	// refusal names first.
	dropped := droppedEntries(stateBytes, state, s.cfg.StateHeadings)

	if err := applyFinishWrites(fsys, s.cfg, feature, step, handoffName, handoff, stepFileName, newStepBody, newSpec, state); err != nil {
		return FinishResult{}, err
	}

	return FinishResult{
		Feature: feature, Step: step, Changed: true,
		HandoffPath: handoffPath, StatePath: statePath, StepPath: stepPath, SpecPath: specPath,
		Modified: []string{statePath, stepPath, specPath}, Next: next, Dropped: dropped,
	}, nil
}

// applyFinishWrites lands Finish's four writes — handoff file, state file,
// step file, specification — in that fixed order, chosen so a crash
// between them always converges on a retry: "status: done" must not land
// before the handoff file exists, or the divergence refusal would refuse
// the very retry that repairs a crash. Every write here is idempotent, so
// a retry from any point re-runs harmlessly, or hits Finish's no-op gate
// once everything already matches. It returns writeFailure for whichever
// write fails first, never a *RefusalError, since an earlier write may
// already have landed; partial is true once the handoff write (the first
// of the four) has landed.
func applyFinishWrites(fsys rwfs.FS, cfg config.Config, feature, step, handoffName string, handoff []byte, stepFileName string, newStepBody []byte, newSpec string, state []byte) error {
	landed := false

	if err := replaceBytes(fsys, handoffName, handoff); err != nil {
		return writeFailure(err, feature, step, landed)
	}

	landed = true

	if err := replaceBytes(fsys, cfg.StateFile, state); err != nil {
		return writeFailure(err, feature, step, landed)
	}

	if err := replaceBytes(fsys, stepFileName, newStepBody); err != nil {
		return writeFailure(err, feature, step, landed)
	}

	if err := replaceString(fsys, cfg.SpecificationFile, newSpec); err != nil {
		return writeFailure(err, feature, step, landed)
	}

	return nil
}

// checkArgumentFence refuses when body opens a fenced code block it never
// closes. It names source (StateSource) rather than a file, since Finish
// never learns where body's bytes came from.
func checkArgumentFence(body []byte, source, label string) *RefusalError {
	return refusalFromViolation(source, conform.UnterminatedFence(body, label))
}

// checkArgumentCap refuses when body measures more lines than limit,
// naming source (HandoffSource or StateSource). A body of exactly limit
// lines is accepted.
func checkArgumentCap(body []byte, source, label string, limit int) *RefusalError {
	return refusalFromViolation(source, conform.OverCap(body, label, limit))
}

// checkArgumentHeadings refuses when body carries no section for one of
// headings' four entries, naming source (StateSource) and only the first
// missing heading.
func checkArgumentHeadings(body []byte, source, label string, headings config.StateHeadings) *RefusalError {
	return refusalFromViolation(source, conform.MissingHeading(body, label, headings))
}

// checkStepChecklist refuses when stepBody's checklist section (under
// heading) holds an item not ticked with "[x]"/"[X]", naming stepPath and
// the item's 1-based line number. stepBody must be the whole step file as
// read from disk, since the line number is counted from its top. A
// checklist with no items, or an absent heading, is never refused.
func checkStepChecklist(stepBody []byte, stepPath, heading string) *RefusalError {
	return refusalFromViolation(stepPath, conform.OpenChecklistItem(stepBody, heading))
}

// refusalFromViolation renders v into a *RefusalError naming path. It
// returns nil when v is nil.
func refusalFromViolation(path string, v *conform.Violation) *RefusalError {
	if v == nil {
		return nil
	}

	return &RefusalError{
		Path:    path,
		Line:    v.Line,
		Problem: v.Problem,
		Fix:     v.Fix,
		Err:     v.Err,
	}
}

// checkStepDependencies refuses when fm — the frontmatter of the step
// being finished, named stepID and living at stepPath — declares a
// depends-on id that is not a done step. It returns nil immediately when
// fm.DependsOn is empty, without scanning root, so an unrelated broken
// sibling can never affect an ordinary finish. Otherwise it scans root and
// records every step file into a stepfile.DependencyIndex — a sibling that
// cannot be read or parsed is still recorded, from a zero Frontmatter, so
// it blocks rather than being silently skipped — and renders the result as
// one of two refusal copies: "is not finished" for a dependency that
// exists but is not done (a self-dependency takes this branch too, and is
// then permanently unfinishable until edited by hand), or "names no step
// file" for one that names nothing at all. A done fm is never refused
// here. It returns error, not *RefusalError, since root's directory
// failing to list at all is a distinct, non-refusal fault; callers recover
// the refusal with errors.As.
func checkStepDependencies(fsys rwfs.FS, pattern stepfile.Pattern, fm stepfile.Frontmatter, stepID, stepPath string) error {
	if len(fm.DependsOn) == 0 {
		return nil
	}

	entries, err := fsys.ReadDir(".")
	if err != nil {
		return fmt.Errorf("scaffold: %w", peelReadErr(err))
	}

	idx := stepfile.NewDependencyIndex()

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		n, ok := pattern.Number(e.Name())
		if !ok {
			continue
		}

		idx.Record(pattern.ID(n), siblingFrontmatter(fsys, e.Name()))
	}

	dep, unmet := idx.FirstUnmet(fm)
	if !unmet {
		return nil
	}

	if idx.Known(dep) {
		return &RefusalError{
			Path:    stepPath,
			Problem: fmt.Sprintf("step %q depends on %q, which is not finished", stepID, dep),
			Fix:     fmt.Sprintf("finish %s first, or remove it from this step's depends-on, and retry", dep),
			Err:     ErrUnmetDependency,
		}
	}

	return &RefusalError{
		Path:    stepPath,
		Problem: fmt.Sprintf("step %q depends on %q, which names no step file", stepID, dep),
		Fix:     "correct the id in this step's depends-on, or remove it, and retry",
		Err:     ErrUnmetDependency,
	}
}

// siblingFrontmatter reads and parses name, one of root's own step files.
// A file that cannot be read or whose frontmatter does not parse returns a
// zero Frontmatter (never done) rather than propagating the error, so that
// sibling counts as not done rather than being silently skipped.
func siblingFrontmatter(fsys rwfs.FS, name string) stepfile.Frontmatter {
	body, err := fsys.ReadFile(name)
	if err != nil {
		return stepfile.Frontmatter{}
	}

	fm, _, err := stepfile.ParseFrontmatter(body)
	if err != nil {
		return stepfile.Frontmatter{}
	}

	return fm
}

// writeFailure wraps a write-path error with the invocation that would
// retry it. It is never a *RefusalError, since the write it reports on
// already changed a file. partial marks it with ErrPartialWrite when an
// earlier write in this same call has already landed.
func writeFailure(err error, feature, step string, partial bool) error {
	wrapped := fmt.Errorf("scaffold: %w; run 'brief finish %s %s --handoff <path> --state <path>' to retry", err, feature, step)
	if !partial {
		return wrapped
	}

	return markPartial(wrapped)
}

// findStepFile scans root for the step file whose pattern.ID equals step,
// deriving the id from each candidate's step number rather than
// reverse-parsing it out of the filename. It returns the matching filename
// and step number, plus the directory listing it read (reused by
// nextOpenStep). It returns ErrNoSuchStep, not wrapped, when no entry
// matches; a directory-listing failure returns that error wrapped instead.
func findStepFile(fsys rwfs.FS, pattern stepfile.Pattern, step string) (string, int, []os.DirEntry, error) {
	entries, err := fsys.ReadDir(".")
	if err != nil {
		return "", 0, nil, fmt.Errorf("scaffold: %w", peelReadErr(err))
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		n, ok := pattern.Number(e.Name())
		if !ok {
			continue
		}

		if pattern.ID(n) == step {
			return e.Name(), n, entries, nil
		}
	}

	return "", 0, entries, ErrNoSuchStep
}

// knownStepIDs returns pattern.ID(n) for every entry in entries that
// pattern recognizes as a step file, in ascending step-number order, for
// the unknown-step refusal's own "known:" list.
func knownStepIDs(pattern stepfile.Pattern, entries []os.DirEntry) []string {
	type numbered struct {
		n  int
		id string
	}

	var found []numbered

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		n, ok := pattern.Number(e.Name())
		if !ok {
			continue
		}

		found = append(found, numbered{n: n, id: pattern.ID(n)})
	}

	sort.Slice(found, func(i, j int) bool { return found[i].n < found[j].n })

	ids := make([]string, len(found))
	for i, f := range found {
		ids[i] = f.id
	}

	return ids
}

// knownStepsFix renders the unknown-step refusal's "known:" fix segment:
// the known ids joined by ", " when non-empty, else the suggestion to
// scaffold one.
func knownStepsFix(known []string, feature string) string {
	if len(known) == 0 {
		return fmt.Sprintf("known: none; run 'brief new step %s' to create one", feature)
	}

	return "known: " + strings.Join(known, ", ")
}

// nextOpenStep returns the lowest-numbered step file among entries whose
// frontmatter status is not "done", excluding finishedNumber and ignoring
// depends-on entirely, so a blocked step is still eligible. It returns the
// zero FinishNext when no step is open.
func nextOpenStep(fsys rwfs.FS, pattern stepfile.Pattern, entries []os.DirEntry, finishedNumber int, featurePath string) FinishNext {
	best := -1
	bestName := ""

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		n, ok := pattern.Number(e.Name())
		if !ok || n == finishedNumber {
			continue
		}

		if best != -1 && n >= best {
			continue
		}

		if siblingFrontmatter(fsys, e.Name()).Done() {
			continue
		}

		best = n
		bestName = e.Name()
	}

	if best == -1 {
		return FinishNext{}
	}

	return FinishNext{ID: pattern.ID(best), Title: stepTitleFromFile(fsys, bestName), Path: filepath.Join(featurePath, bestName)}
}

// stepTitleFromFile reads name's body through fsys and returns
// markdown.Title of its frontmatter remainder, so a "#" inside YAML
// frontmatter is never read as a heading. It returns "" when name cannot
// be read or its frontmatter does not parse, degrading FinishNext.Title
// rather than losing the id/path a caller still needs.
func stepTitleFromFile(fsys rwfs.FS, name string) string {
	body, err := fsys.ReadFile(name)
	if err != nil {
		return ""
	}

	_, rest, err := stepfile.ParseFrontmatter(body)
	if err != nil {
		return ""
	}

	title, _ := markdown.Title(string(rest))

	return title
}

// progressRefusal renders tickProgressEntry's error as a refusal naming
// specPath: ErrNoProgressHeading when the configured progress heading is
// missing, ErrNoProgressEntry when the list has no entry for step, and a
// wrapped error otherwise.
func progressRefusal(specPath string, cfg config.Config, feature, step string, err error) error {
	switch {
	case errors.Is(err, ErrNoProgressHeading):
		return &RefusalError{
			Path:    specPath,
			Problem: fmt.Sprintf("no %q heading found", cfg.ProgressHeading),
			Fix:     fmt.Sprintf("add a %q heading to the specification", cfg.ProgressHeading),
			Err:     ErrNoProgressHeading,
		}
	case errors.Is(err, ErrNoProgressEntry):
		return &RefusalError{
			Path:    specPath,
			Problem: fmt.Sprintf("no progress entry found for %q", step),
			Fix:     fmt.Sprintf("run 'brief new step %s' to add it, or check the id", feature),
			Err:     ErrNoProgressEntry,
		}
	default:
		return fmt.Errorf("scaffold: %w", err)
	}
}

// refinishVerdict is the outcome (refinish).verdict decides a re-finish of
// a done step into.
type refinishVerdict int

const (
	// refinishWrite means Finish proceeds through its normal four writes.
	refinishWrite refinishVerdict = iota
	// refinishNoop means every input matches what is already on disk:
	// Finish writes nothing.
	refinishNoop
	// refinishHandoffDiverged means the step is done, its handoff file is
	// recorded, and the supplied handoff differs from it.
	refinishHandoffDiverged
	// refinishStateDiverged means the step is done, the supplied handoff
	// matches what is recorded, and the supplied state differs from what
	// is recorded.
	refinishStateDiverged
)

// refinish carries the five facts (refinish).verdict decides a re-finish
// of a done step from, all read at the same point in Finish where the
// four writes' bodies are computed.
type refinish struct {
	// done is fm.Done() — the step's frontmatter status before this call.
	done bool
	// handoffRecorded is true when the step's handoff file exists and was
	// read without error.
	handoffRecorded bool
	// handoffMatches is true when handoffRecorded and its bytes equal the
	// supplied handoff.
	handoffMatches bool
	// stateMatches is true when the supplied state equals the state
	// bytes already on disk.
	stateMatches bool
	// specTicked is true when the specification, with step's progress
	// entry ticked, already equals what is on disk.
	specTicked bool
}

// verdict decides a re-finish of a done step, checked in order so a call
// where both the handoff and the state diverge reports the handoff first,
// matching the write order. specTicked participates only in the
// noop-vs-write split, never as a divergence trigger: the progress tick is
// derived from the specification, not a caller input, so an un-ticked
// entry means "half-applied write to repair" and must converge on a retry
// with the same arguments rather than being refused.
func (r refinish) verdict() refinishVerdict {
	if !r.done {
		return refinishWrite
	}

	// A missing or unreadable handoff file exempts a done step from the
	// divergence refusal entirely: it is this command's own output, not an
	// input to repair, and with nothing recorded there is nothing to
	// diverge from — refusing would be a dead end for a crash-then-hand-
	// edit tree, or a tree migrated before handoff files existed.
	if !r.handoffRecorded {
		return refinishWrite
	}

	if !r.handoffMatches {
		return refinishHandoffDiverged
	}

	if !r.stateMatches {
		return refinishStateDiverged
	}

	if r.specTicked {
		return refinishNoop
	}

	return refinishWrite
}

// alreadyFinishedRefusal renders the ErrAlreadyFinished refusal for a
// re-finish of step whose label ("handoff" or "state") diverges from what
// is recorded at path.
func alreadyFinishedRefusal(path, step, label string) *RefusalError {
	return &RefusalError{
		Path:    path,
		Problem: fmt.Sprintf("step %q is already done and the given %s differs from the one recorded here", step, label),
		Fix:     fmt.Sprintf("diff the %s you passed against it, then edit this file directly if the new %s is correct", label, label),
		Err:     ErrAlreadyFinished,
	}
}
