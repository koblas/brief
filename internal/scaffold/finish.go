package scaffold

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/conform"
	"github.com/koblas/brief/internal/platform/stepfile"
)

// FinishResult is what Finish returns on success — either after its four
// writes land, or as R11's no-op, which still returns the populated result
// rather than a zero value. Feature and Step are the arguments Finish was
// called with. Changed is false only for R11's no-op: every input already
// matched what was on disk, so nothing was written. HandoffPath and
// StatePath are absolute — the step's own handoff file and the feature's
// state file Finish wrote or, on a no-op, would have — never the caller's
// own --state/--handoff input path, which Finish never learns. Next is the
// id of the lowest-numbered step file (by stepfile.Pattern.Number) whose
// frontmatter status is not "done", counting the just-finished step as
// done and ignoring depends-on, so a blocked step can still be Next; ""
// when every other step is done. A sibling whose frontmatter cannot be
// read or does not parse counts as not done, so it can be named Next too.
type FinishResult struct {
	Feature, Step          string
	Changed                bool
	HandoffPath, StatePath string
	Next                   string
}

// Finish closes feature's step: it writes handoff to that step's own
// handoff file, replaces the feature's state file with state, then marks
// the step file's frontmatter status "done" (R8, R21). handoff is checked
// against cfg.HandoffCapLines; state is checked against cfg.StateCapLines
// and against cfg.StateHeadings, which it must carry all four of (any
// order, an empty section valid) — the write-side counterpart of
// assemble.Start's read-side shortfall degrade. handoff carries no such
// heading check: it is written verbatim to its own file and nothing reads
// it structurally. Neither argument is spliced into an existing document —
// every write here is a whole-file write, so none has a boundary inferred
// from prose to get wrong.
//
// The handoff file is named stepPattern.ID(n) + cfg.HandoffFileSuffix
// (stepfile.CompileHandoff); a "## Handoff" section left behind in a step
// file by an unmigrated tree is ordinary prose Finish never reads.
//
// Every check runs, and every write body is computed, before the first
// byte reaches disk — in the order a refusal must name the first thing
// wrong (R14a): the step-file pattern compiles; the handoff-file-suffix
// compiles against it (stepfile.ErrInvalidHandoffSuffix); the feature
// directory opens; a step file exists whose id equals step; its
// frontmatter parses; the handoff argument measures no more than
// cfg.HandoffCapLines lines (ErrOverCap, named against HandoffSource,
// counted by markdown.CountLines), immediately followed by the same check
// against cfg.StateCapLines for the replacement state body (named against
// StateSource) — a done step over either cap reports the cap rather than
// falling through to the re-finish verdict below, since both checks run
// ahead of it; the replacement state body then closes every fence it opens
// (ErrUnterminatedFence, named against StateSource) — state's configured
// headings are read by a terminator scan on every later Start, so an open
// fence there is not merely untidy, it is unreadable; the replacement state
// body then carries a section for every one of cfg.StateHeadings.Ordered()'s
// four headings (ErrMissingStateHeading, named against StateSource,
// checkArgumentHeadings) — checked only after the fence closes, since an
// open fence would leave a heading after it unreadable; the step's own
// checklist section — under cfg.ChecklistHeading — carries no item left
// unticked (ErrOpenChecklistItem, checkStepChecklist, naming stepPath and
// the item's line) — checked only after the state argument band, and
// ahead of the specification read and (refinish).verdict below, so a step
// that is both un-ticked and a divergent re-finish reports the open item;
// a checklist with no items, or no checklist heading at all, is never
// refused this way, matching assemble.Start's read-side degrade for the
// same heading; the step's own frontmatter then carries no depends-on id
// that is not a done step (ErrUnmetDependency, checkStepDependencies,
// naming stepPath) — checked immediately after the checklist and ahead of
// the specification read, so a step both un-ticked and blocked reports the
// checklist item first; the check is skipped entirely when depends-on is
// empty, so an unrelated broken sibling step file never affects an
// ordinary finish; a sibling that cannot be read or whose frontmatter does
// not parse is recorded as a known, not-done step rather than skipped, so
// it blocks with the "is not finished" copy rather than the wrong "names
// no step file" one; a done step is never refused this way, whatever its
// dependencies say — stepfile.DependencyIndex.FirstUnmet short-circuits on
// the step's own doneness, the same exemption assemble.Status's blocked
// count applies, keeping a re-finish of a done step whose dependency was
// reopened by hand a true no-op; the specification is
// readable; the specification carries the
// configured progress heading and an entry for step; the state file exists
// as a regular file. Computing the frontmatter's "status: done" line during
// this phase, rather than at write time, means a step file with no
// "status:" field (stepfile.ErrNoStatusField) is refused before any write
// lands, not discovered half way through the sequence. The handoff
// argument is never fence-checked, only line-counted: it is written
// verbatim to its own file and nothing reads it structurally.
//
// The four writes then land in a fixed order — handoff file, state file,
// step file, specification — chosen so a crash between them always
// converges on retry. "status: done" must never land before the handoff
// file exists: every earlier prefix in this order leaves the step's
// frontmatter status "open", the sole doneness authority a reader trusts,
// so a retried Finish takes the full path again and rewrites each earlier
// write with a byte-identical body. The reverse order does not converge:
// a step file already marked done, beside a missing or stale handoff
// file, gives a retry no accurate signal that anything is still wrong. A
// write failure after validation is returned as-is, never wrapped in
// *RefusalError — the template's "(no files changed)" tail would
// misreport a half-applied write.
//
// A re-finish of a step whose frontmatter already says done resolves to
// one of three outcomes, decided by (refinish).verdict — see its doc
// comment for the five facts and six rows that make up the decision:
//
//   - A true no-op, writing nothing and preserving mtime on all four
//     files, when the handoff file exists and its bytes equal handoff, the
//     state bytes equal state, and the spec-with-tick already equals what
//     is on disk (R11).
//   - A refusal wrapping ErrAlreadyFinished, naming the recorded handoff
//     file, when the handoff file exists but its bytes differ from
//     handoff.
//   - A refusal wrapping ErrAlreadyFinished, naming cfg.StateFile, when
//     the handoff matches but state differs from the recorded state
//     bytes.
//
// A done step whose handoff file is missing or unreadable is exempt from
// both refusals and writes as normal, the same as an un-ticked progress
// entry — see (refinish).verdict for why neither is a divergence trigger.
// The step-file conjunct is fm.Done() rather than a byte comparison of the
// step body: with the splice gone the step-file write body is a pure
// function of the on-disk body, so a byte comparison would hold in almost
// exactly the cases fm.Done() holds, and where they differ fm.Done() is
// the correct predicate — the doneness authority is the parsed value, not
// the byte shape, and R11 requires mtime preserved.
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

	topRoot, err := os.OpenRoot(featureDirPath)
	if err != nil {
		return FinishResult{}, noSuchFeatureRefusal(featurePath, feature)
	}
	defer func() { _ = topRoot.Close() }()

	root, err := topRoot.OpenRoot(feature)
	if err != nil {
		return FinishResult{}, noSuchFeatureRefusal(featurePath, feature)
	}
	defer func() { _ = root.Close() }()

	stepFileName, stepNumber, entries, err := findStepFile(root, pattern, step)
	if err != nil {
		return FinishResult{}, &RefusalError{
			Path:    featurePath,
			Problem: fmt.Sprintf("no step file found for %q", step),
			Fix:     fmt.Sprintf("run 'brief new step %s' to see the next step, or check the id", feature),
			Err:     ErrNoSuchStep,
		}
	}

	stepPath := filepath.Join(featurePath, stepFileName)
	handoffName := handoffPattern.Name(stepNumber)

	stepBody, err := root.ReadFile(stepFileName)
	if err != nil {
		return FinishResult{}, fmt.Errorf("scaffold: %w", err)
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

	if err := checkStepDependencies(root, pattern, fm, step, stepPath); err != nil {
		return FinishResult{}, err
	}

	specPath := filepath.Join(featurePath, s.cfg.SpecificationFile)

	specBytes, err := root.ReadFile(s.cfg.SpecificationFile)
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

	stateInfo, err := root.Lstat(s.cfg.StateFile)
	if err != nil || !stateInfo.Mode().IsRegular() {
		return FinishResult{}, &RefusalError{
			Path:    statePath,
			Problem: "state file is missing",
			Fix:     "scaffold the feature again to restore it",
			Err:     ErrMalformedFeature,
		}
	}

	stateBytes, err := root.ReadFile(s.cfg.StateFile)
	if err != nil {
		return FinishResult{}, fmt.Errorf("scaffold: %w", err)
	}

	// An unreadable handoff file — including one that does not exist —
	// exempts a done step from the divergence refusal entirely: the
	// handoff file is this command's own output, not an input the caller
	// must repair, and a done step is only ever reached through Finish
	// (R10), so refusing here would be a dead end for a crash-then-hand-
	// edit tree or a tree migrated before handoff files existed.
	handoffPath := filepath.Join(featurePath, handoffName)
	existingHandoff, handoffReadErr := root.ReadFile(handoffName)
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

	// next is computed here, in the validation phase and before any write,
	// so a caller learns brief start's post-finish next step even on R11's
	// no-op, and so no error path exists for it after the writes land.
	next := nextOpenStep(root, pattern, entries, stepNumber)

	switch r.verdict() {
	case refinishNoop:
		return FinishResult{Feature: feature, Step: step, Changed: false, HandoffPath: handoffPath, StatePath: statePath, Next: next}, nil
	case refinishHandoffDiverged:
		return FinishResult{}, alreadyFinishedRefusal(handoffPath, step, "handoff")
	case refinishStateDiverged:
		return FinishResult{}, alreadyFinishedRefusal(statePath, step, "state")
	case refinishWrite:
		// Falls through to the four writes below.
	}

	if err := applyFinishWrites(root, s.cfg, feature, step, handoffName, handoff, stepFileName, newStepBody, newSpec, state); err != nil {
		return FinishResult{}, err
	}

	return FinishResult{Feature: feature, Step: step, Changed: true, HandoffPath: handoffPath, StatePath: statePath, Next: next}, nil
}

// applyFinishWrites lands Finish's four writes — handoff file, state file,
// step file, specification — in that fixed order, chosen so a crash
// between them always converges on a retry: "status: done" must not land
// before the handoff file exists, or the divergence refusal would refuse
// the very retry that repairs a crash. Every prefix up to and including
// the state write leaves the step's frontmatter status "open", the sole
// doneness authority a reader trusts, so a retry from one of those takes
// the full path again. The remaining prefix — handoff, state, and the step
// file itself — already leaves status "done"; a retry from there still
// converges by one of two mechanisms. Ordinarily newSpec differs from what
// is on disk, so the retry falls through and re-runs tickProgressEntry and
// SetStatus, both idempotent, rewriting each file with the same bytes it
// already holds. But if the progress entry was ticked by some other means
// between the crash and the retry, newSpec already equals what is on disk
// and fm.Done() is already true — so the retry instead converges by
// hitting Finish's no-op gate and returning before touching any file at
// all. It returns writeFailure(err, feature, step) for whichever write
// fails first, never a *RefusalError, since every earlier write in this
// call has already landed.
func applyFinishWrites(root *os.Root, cfg config.Config, feature, step, handoffName string, handoff []byte, stepFileName string, newStepBody []byte, newSpec string, state []byte) error {
	if err := replaceBytes(root, handoffName, handoff); err != nil {
		return writeFailure(err, feature, step)
	}

	if err := replaceBytes(root, cfg.StateFile, state); err != nil {
		return writeFailure(err, feature, step)
	}

	if err := replaceBytes(root, stepFileName, newStepBody); err != nil {
		return writeFailure(err, feature, step)
	}

	if err := replaceString(root, cfg.SpecificationFile, newSpec); err != nil {
		return writeFailure(err, feature, step)
	}

	return nil
}

// checkArgumentFence refuses when body — Finish's state argument, named
// for the error message by label — opens a fenced code block it never
// closes, by conform.UnterminatedFence. It names source (StateSource)
// rather than any file, since Finish never learns which file, or whether
// there was one at all, body's bytes came from. It returns nil when every
// fence body opens is closed.
func checkArgumentFence(body []byte, source, label string) *RefusalError {
	return refusalFromViolation(source, conform.UnterminatedFence(body, label))
}

// checkArgumentCap refuses when body — one of Finish's handoff or state
// arguments, named for the error message by label — measures more lines,
// by conform.OverCap, than limit. It names source (HandoffSource or
// StateSource) rather than any file, matching checkArgumentFence: Finish
// never learns which file, or whether there was one at all, body's bytes
// came from. It returns nil when body's line count does not exceed limit —
// a body of exactly limit lines is accepted.
func checkArgumentCap(body []byte, source, label string, limit int) *RefusalError {
	return refusalFromViolation(source, conform.OverCap(body, label, limit))
}

// checkArgumentHeadings refuses when body — Finish's state argument, named
// for the error message by label — carries no section for one of headings'
// four entries, by conform.MissingHeading. It names source (StateSource)
// rather than any file, matching checkArgumentFence and checkArgumentCap.
// Only the first missing heading is reported: unlike assemble.Start's
// shortfall degrade, which completes and can report every shortfall it
// finds, Finish is a refusal that stops at the first fault. It returns nil
// when every configured heading is present.
func checkArgumentHeadings(body []byte, source, label string, headings config.StateHeadings) *RefusalError {
	return refusalFromViolation(source, conform.MissingHeading(body, label, headings))
}

// checkStepChecklist refuses when stepBody's checklist section — the
// section under heading — holds an item not ticked with "[x]"/"[X]", by
// conform.OpenChecklistItem. It names stepPath and the item's 1-based line
// number in the whole file, the shape R14a uses for a fault inside a file
// rather than an argument's placeholder source. stepBody must be the whole
// step file as read from disk, not the remainder ParseFrontmatter returns:
// conform.OpenChecklistItem's line number is counted from the top of
// stepBody. A checklist with no items, or an absent heading, is never
// refused — only an unchecked item triggers, matching assemble.Start's
// read-side degrade rule for the same heading.
func checkStepChecklist(stepBody []byte, stepPath, heading string) *RefusalError {
	return refusalFromViolation(stepPath, conform.OpenChecklistItem(stepBody, heading))
}

// refusalFromViolation renders v, one of conform's four predicates' result,
// into a *RefusalError naming path — the call site's own file or argument
// placeholder, which conform never learns. It returns nil when v is nil.
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
// depends-on id that is not a done step, by stepfile.DependencyIndex.
// FirstUnmet: the same rule assemble.Status's blocked count applies, so
// status and finish never disagree about the same tree. It returns nil
// immediately when fm.DependsOn is empty, without scanning root at all, so
// an unrelated broken sibling step file can never affect an ordinary
// finish. Otherwise it scans root for every entry pattern
// recognizes as a step file and records each into a
// stepfile.DependencyIndex, keyed by pattern.ID(n) rather than the
// sibling's own frontmatter id — a sibling that cannot be read or whose
// frontmatter does not parse is still Recorded, from a zero Frontmatter,
// rather than skipped, so it blocks with the "is not finished" copy
// rather than the wrong "names no step file" one — then renders
// FirstUnmet's result through Known into one of two refusal copies: an id
// that names an existing, not-done step ("is not finished" — a
// self-dependency takes this branch too, naming stepID on both sides of
// the line, and is then permanently unfinishable until the frontmatter is
// edited by hand) or an id Known reports nothing was recorded under
// ("names no step file"). A done fm is never refused here — FirstUnmet
// short-circuits on fm.Done() before either branch is reached, which is
// what keeps a re-finish of a done step whose dependency was reopened by
// hand a no-op. checkStepDependencies returns error, not *RefusalError,
// because root's directory failing to list at all is a distinct,
// non-refusal fault from any individual sibling's read or parse failure;
// callers recover the refusal with errors.As, the same way every other
// *RefusalError in this package is recovered.
func checkStepDependencies(root *os.Root, pattern stepfile.Pattern, fm stepfile.Frontmatter, stepID, stepPath string) error {
	if len(fm.DependsOn) == 0 {
		return nil
	}

	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return fmt.Errorf("scaffold: %w", err)
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

		idx.Record(pattern.ID(n), siblingFrontmatter(root, e.Name()))
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

// siblingFrontmatter reads and parses name — one of root's own step
// files — for checkStepDependencies. A file that cannot be read or whose
// frontmatter does not parse returns a zero Frontmatter (never done)
// rather than propagating the error: that sibling is recorded as a known,
// not-done step so it blocks a dependant rather than being silently
// skipped.
func siblingFrontmatter(root *os.Root, name string) stepfile.Frontmatter {
	body, err := root.ReadFile(name)
	if err != nil {
		return stepfile.Frontmatter{}
	}

	fm, _, err := stepfile.ParseFrontmatter(body)
	if err != nil {
		return stepfile.Frontmatter{}
	}

	return fm
}

// writeFailure wraps a write-path error with the same invocation that
// would retry it. It is never a *RefusalError: the write it reports on
// already changed a file, so the refusal template's "(no files changed)"
// tail would misreport that.
func writeFailure(err error, feature, step string) error {
	return fmt.Errorf("scaffold: %w; run 'brief finish %s %s --handoff <path> --state <path>' to retry", err, feature, step)
}

// findStepFile scans root for the step file whose pattern.ID equals step,
// deriving the id from each candidate's step number rather than
// reverse-parsing it out of the filename, so a near-miss pattern.Number
// already rejects is never mistaken for a match. It returns the matching
// filename and its step number, the latter needed to derive that step's
// handoff filename, plus the directory listing it read — reused by
// nextOpenStep so Finish never lists the directory twice.
func findStepFile(root *os.Root, pattern stepfile.Pattern, step string) (string, int, []os.DirEntry, error) {
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return "", 0, nil, fmt.Errorf("scaffold: %w", err)
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

// nextOpenStep returns the id of brief start's own next-open-step rule —
// the lowest-numbered step file among entries (by stepfile.Pattern.Number)
// whose frontmatter status is not "done" — excluding finishedNumber, the
// step Finish is about to mark done, and ignoring depends-on entirely, so
// a blocked step is still eligible. It duplicates assemble.Start's own
// definition (assemble.go's readSteps/Start loop) because scaffold and
// assemble may not import each other; Test_finish_next_agrees_with_start
// (internal/cli) pins the two in agreement. A sibling that cannot be read
// or whose frontmatter does not parse is read through siblingFrontmatter,
// which reports a zero Frontmatter — never done — so it counts as not
// done and can be named here, the same tolerance
// checkStepDependencies applies. It returns "" when no step is open.
func nextOpenStep(root *os.Root, pattern stepfile.Pattern, entries []os.DirEntry, finishedNumber int) string {
	best := -1

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

		if siblingFrontmatter(root, e.Name()).Done() {
			continue
		}

		best = n
	}

	if best == -1 {
		return ""
	}

	return pattern.ID(best)
}

// progressRefusal renders tickProgressEntry's error as the R14a refusal
// naming specPath: ErrNoProgressHeading when the configured progress
// heading is missing, ErrNoProgressEntry when the list has no entry for
// step, and a wrapped error for anything else tickProgressEntry did not
// itself produce.
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
	// refinishWrite means Finish proceeds through its normal four writes:
	// either the step is not yet done, its handoff file is missing or
	// unreadable, or the progress entry has not been ticked yet (a
	// crash-after-step-file retry).
	refinishWrite refinishVerdict = iota
	// refinishNoop means every input matches what is already on disk
	// (R11): Finish writes nothing.
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

// verdict decides a re-finish of a done step among four outcomes, in the
// order below — rows 3 and 4 ordered handoff-first, matching the write
// order and R14a's "names the first thing wrong", so a call where both
// the handoff and the state diverge is reported as a handoff divergence:
//
//  1. !done                          -> refinishWrite
//  2. done && !handoffRecorded       -> refinishWrite (the exemption)
//  3. done && !handoffMatches        -> refinishHandoffDiverged
//  4. done && !stateMatches          -> refinishStateDiverged
//  5. done && specTicked             -> refinishNoop (R11)
//  6. done && !specTicked            -> refinishWrite
//
// specTicked participates only in the noop-vs-write split (rows 5/6),
// never in a diverged arm: the progress tick is derived from the
// specification, not a caller input, so an un-ticked entry means
// "half-applied write to repair", not "different inputs". A retry after a
// blocked specification write — done, handoff matches, state matches, spec
// still un-ticked — must converge on a second call with the same
// arguments; folding specTicked into the divergence trigger would refuse
// that retry instead.
//
// Row 5 is reached only when done, handoffRecorded, handoffMatches,
// stateMatches and specTicked all hold — bit-for-bit R11's no-op
// condition. Rows 3 and 4 are the only outcomes that refuse rather than
// write or no-op.
//
// The exemption in row 2 — a done step whose handoff file is missing or
// unreadable writes as normal rather than being refused — covers a crash
// between the state write and the step write followed by a hand edit, and
// a tree migrated before handoff files existed. With no recorded handoff
// there is nothing to diverge from, and Finish is the only path to a done
// step (R10), so a refusal there would be a dead end.
func (r refinish) verdict() refinishVerdict {
	if !r.done {
		return refinishWrite
	}

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
// is recorded at path. The caller's only route forward is to read the
// recorded bytes and compare, so the message names the specific divergent
// file rather than refusing generically.
func alreadyFinishedRefusal(path, step, label string) *RefusalError {
	return &RefusalError{
		Path:    path,
		Problem: fmt.Sprintf("step %q is already done and the given %s differs from the one recorded here", step, label),
		Fix:     fmt.Sprintf("diff the %s you passed against it, then edit this file directly if the new %s is correct", label, label),
		Err:     ErrAlreadyFinished,
	}
}
