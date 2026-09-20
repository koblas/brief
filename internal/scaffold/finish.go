package scaffold

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/koblas/brief/internal/platform/stepfile"
)

// Finish closes feature's step: it writes handoff to that step's own
// handoff file, replaces the feature's state file with state, then marks
// the step file's frontmatter status "done" (R8, R21). handoff and state
// are written verbatim, neither checked against a length cap or a
// required-heading schema, and neither is spliced into an existing
// document — every write here is a whole-file write, so none has a
// boundary inferred from prose to get wrong.
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
// frontmatter parses; the replacement state body closes every fence it
// opens (ErrUnterminatedFence, named against StateSource) — state's
// configured headings are read by a terminator scan on every later Start,
// so an open fence there is not merely untidy, it is unreadable; the
// specification is readable; the specification carries the configured
// progress heading and an entry for step; the state file exists as a
// regular file. Computing the frontmatter's "status: done" line during
// this phase, rather than at write time, means a step file with no
// "status:" field (stepfile.ErrNoStatusField) is refused before any write
// lands, not discovered half way through the sequence. The handoff
// argument itself is never fence-checked: it is written verbatim to its
// own file and nothing reads it structurally.
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
func (s *Server) Finish(_ context.Context, feature, step string, handoff, state []byte) error {
	featureDirPath := filepath.Join(s.root, s.cfg.FeatureDirectory)
	featurePath := filepath.Join(featureDirPath, feature)

	pattern, err := stepfile.Compile(s.cfg.StepFilePattern)
	if err != nil {
		return &RefusalError{
			Path:    featurePath,
			Problem: fmt.Sprintf("step-file-pattern %q is invalid: %v", s.cfg.StepFilePattern, err),
			Fix:     "fix step-file-pattern in .brief.yaml",
			Err:     stepfile.ErrInvalidPattern,
		}
	}

	handoffPattern, err := stepfile.CompileHandoff(pattern, s.cfg.HandoffFileSuffix, s.cfg.StateFile, s.cfg.SpecificationFile)
	if err != nil {
		return &RefusalError{
			Path:    featurePath,
			Problem: fmt.Sprintf("handoff-file-suffix %q is invalid: %v", s.cfg.HandoffFileSuffix, err),
			Fix:     "fix handoff-file-suffix in .brief.yaml",
			Err:     stepfile.ErrInvalidHandoffSuffix,
		}
	}

	topRoot, err := os.OpenRoot(featureDirPath)
	if err != nil {
		return noSuchFeatureRefusal(featurePath, feature)
	}
	defer func() { _ = topRoot.Close() }()

	root, err := topRoot.OpenRoot(feature)
	if err != nil {
		return noSuchFeatureRefusal(featurePath, feature)
	}
	defer func() { _ = root.Close() }()

	stepFileName, stepNumber, err := findStepFile(root, pattern, step)
	if err != nil {
		return &RefusalError{
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
		return fmt.Errorf("scaffold: %w", err)
	}

	fm, _, err := stepfile.ParseFrontmatter(stepBody)
	if err != nil {
		return &RefusalError{
			Path:    stepPath,
			Problem: fmt.Sprintf("frontmatter does not parse: %v", err),
			Fix:     "fix the step file's YAML frontmatter",
			Err:     ErrMalformedFeature,
		}
	}

	if refusal := checkArgumentFence(state, StateSource, "state"); refusal != nil {
		return refusal
	}

	specPath := filepath.Join(featurePath, s.cfg.SpecificationFile)

	specBytes, err := root.ReadFile(s.cfg.SpecificationFile)
	if err != nil {
		return &RefusalError{
			Path:    specPath,
			Problem: "specification file is missing",
			Fix:     "scaffold the feature again to restore it",
			Err:     ErrMalformedFeature,
		}
	}

	newSpec, err := tickProgressEntry(string(specBytes), s.cfg.ProgressHeading, step)
	if err != nil {
		return progressRefusal(specPath, s.cfg, feature, step, err)
	}

	statePath := filepath.Join(featurePath, s.cfg.StateFile)

	stateInfo, err := root.Lstat(s.cfg.StateFile)
	if err != nil || !stateInfo.Mode().IsRegular() {
		return &RefusalError{
			Path:    statePath,
			Problem: "state file is missing",
			Fix:     "scaffold the feature again to restore it",
			Err:     ErrMalformedFeature,
		}
	}

	stateBytes, err := root.ReadFile(s.cfg.StateFile)
	if err != nil {
		return fmt.Errorf("scaffold: %w", err)
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
		return &RefusalError{
			Path:    stepPath,
			Problem: `frontmatter has no "status:" field`,
			Fix:     `add a "status:" key to the step file's frontmatter`,
			Err:     err,
		}
	}

	switch r.verdict() {
	case refinishNoop:
		return nil
	case refinishHandoffDiverged:
		return alreadyFinishedRefusal(handoffPath, step, "handoff")
	case refinishStateDiverged:
		return alreadyFinishedRefusal(statePath, step, "state")
	case refinishWrite:
		// Falls through to the four writes below.
	}

	// The four writes below are ordered, and the order is load-bearing:
	// "status: done" must not land before the handoff file exists, or the
	// divergence refusal would refuse the very retry that repairs a crash.
	// Every prefix up to and including the state write leaves the step's
	// frontmatter status "open", the sole doneness authority a reader
	// trusts, so a retry from one of those takes the full path again. The
	// remaining prefix — handoff, state, and the step file itself — already
	// leaves status "done"; a retry from there still converges by one of two
	// mechanisms. Ordinarily newSpec differs from what is on disk (identical
	// stays false), so the retry falls through and re-runs tickProgressEntry
	// and SetStatus, both idempotent, rewriting each file with the same
	// bytes it already holds. But if the progress entry was ticked by some
	// other means between the crash and the retry, newSpec already equals
	// what is on disk, identical stays true, and fm.Done() is already true —
	// so the retry instead converges by hitting the no-op gate above and
	// returning before touching any file at all.
	if err := replaceBytes(root, handoffName, handoff); err != nil {
		return writeFailure(err, feature, step)
	}

	if err := replaceBytes(root, s.cfg.StateFile, state); err != nil {
		return writeFailure(err, feature, step)
	}

	if err := replaceBytes(root, stepFileName, newStepBody); err != nil {
		return writeFailure(err, feature, step)
	}

	if err := replaceString(root, s.cfg.SpecificationFile, newSpec); err != nil {
		return writeFailure(err, feature, step)
	}

	return nil
}

// checkArgumentFence refuses when body — Finish's state argument, named
// for the error message by label — opens a fenced code block it never
// closes. It names source (StateSource) rather than any file, since
// Finish never learns which file, or whether there was one at all, body's
// bytes came from. It returns nil when every fence body opens is closed.
func checkArgumentFence(body []byte, source, label string) *RefusalError {
	line, delim, unterminated := markdown.UnterminatedFence(string(body))
	if !unterminated {
		return nil
	}

	return &RefusalError{
		Path:    source,
		Line:    line,
		Problem: fmt.Sprintf("%s has an unclosed %s fence", label, delim),
		Fix:     "close the fence, or remove the unmatched delimiter, and retry",
		Err:     ErrUnterminatedFence,
	}
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
// handoff filename.
func findStepFile(root *os.Root, pattern stepfile.Pattern, step string) (string, int, error) {
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return "", 0, fmt.Errorf("scaffold: %w", err)
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
			return e.Name(), n, nil
		}
	}

	return "", 0, ErrNoSuchStep
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
	// (R11, SCENARIO-06): Finish writes nothing.
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
//  5. done && specTicked             -> refinishNoop (R11, SCENARIO-06)
//  6. done && !specTicked            -> refinishWrite
//
// specTicked participates only in the noop-vs-write split (rows 5/6),
// never in a diverged arm: the progress tick is derived from the
// specification, not a caller input, so an un-ticked entry means
// "half-applied write to repair", not "different inputs". Test_reports_a_specification_write_that_cannot_be_committed
// in finish_test.go blocks the fourth write and then retries with the
// same arguments, leaving the tree at done + handoff matches + state
// matches + spec un-ticked and requiring that retry to converge; folding
// specTicked into the divergence trigger would refuse that retry.
//
// verdict is a strict refinement of SCENARIO-06: row 5 is reached only
// when done, handoffRecorded, handoffMatches, stateMatches and specTicked
// all hold, which is bit-for-bit the no-op's original truth condition —
// the only behavioural delta this scenario adds is rows 3 and 4.
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
