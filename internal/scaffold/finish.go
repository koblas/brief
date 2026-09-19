package scaffold

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/koblas/brief/internal/platform/atomicfile"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/koblas/brief/internal/platform/stepfile"
)

// Finish closes feature's step, replacing its handoff block with handoff
// and the feature's state file with state, then marking the step file's
// frontmatter status "done" (R8). handoff and state are written verbatim,
// neither checked against a length cap or a required-heading schema.
// depends-on and the step's own checklist are read but not enforced:
// Finish writes regardless of either.
//
// The handoff anchor's section runs from the end of its own heading line
// to end of the step body, unconditionally — never by scanning for a
// terminating heading, per the default profile's "## Handoff is the last
// section of every step file". That contract, not a scan, is what makes
// re-splicing a fixed point: no scan means no fence state a nested fence
// could leave wrongly open, and no heading anywhere in handoff's own text
// — however handoff distills its content — can be misread as an early
// terminator. See spliceHandoff.
//
// Every check runs, and every write body is computed, before the first
// byte reaches disk — in the order a refusal must name the first thing
// wrong (R14a): the step-file pattern compiles; the feature directory
// opens; a step file exists whose id equals step; its frontmatter parses;
// its handoff anchor is present — or, when absent because a fence opened
// earlier in the body never closed before the anchor's own line would
// have been reached, ErrUnterminatedFence names that instead of the
// otherwise-untrue "no heading found"; the anchor is the last heading in
// the step file, checked only while the step is not yet done, since once
// done everything after the anchor is Finish's own prior output and R11
// requires it to survive an identical re-finish untouched
// (ErrHandoffNotLast); handoff itself closes every fence it opens
// (ErrUnterminatedFence, named against HandoffSource); state closes every
// fence it opens (ErrUnterminatedFence, named against StateSource) —
// state's configured headings are read by a terminator scan on every
// later Start, so an open fence there is not merely untidy, it is
// unreadable; the specification is readable; the specification carries
// the configured progress heading and an entry for step; the state file
// exists as a regular file. Computing the spliced step body and the
// frontmatter's "status: done" line during this phase, rather than at
// write time, means a step file with no "status:" field
// (stepfile.ErrNoStatusField) is refused before any write lands, not
// discovered half way through the sequence.
//
// The three writes then land in a fixed order — state file, step file,
// specification — chosen so a crash between them always converges on
// retry: a crash after the state file leaves the step open, so a retried
// Finish takes the full path again; a crash after the step file leaves
// only the progress checkbox stale, and frontmatter's status is the sole
// doneness authority a reader trusts. The reverse order does not converge:
// a step file already marked done, beside the state file's old body,
// gives a retry no accurate signal that anything is still wrong — the
// frontmatter already says done, so nothing distinguishes "this step
// still needs its state written" from "this step is genuinely finished",
// and the gap becomes unrepairable through the tool. A write failure
// after validation is returned as-is, never wrapped in *RefusalError —
// the template's "(no files changed)" tail would misreport a
// half-applied write.
//
// A re-finish of a step whose frontmatter already says done is a true
// no-op — Finish writes nothing and mtime on all three files is
// preserved — when the spliced step body, the state bytes and the
// spec-with-tick all already equal what is on disk; any single
// divergence writes as normal. The step-body half of that comparison is
// taken before "status:" is set to "done", excluding the status line
// itself: a comparison taken after would only ever hold on an
// already-done step, making the frontmatter gate unreachable. The spec
// half is required even though the frontmatter gate alone looks
// sufficient: after a crash between the step-file write and the
// specification write, frontmatter already says done and the handoff
// and state both match, so without the spec conjunct the progress
// checkbox would stay stale forever.
func (s *Server) Finish(_ context.Context, feature, step string, handoff, state []byte) error {
	pattern, err := stepfile.Compile(s.cfg.StepFilePattern)
	if err != nil {
		return &RefusalError{
			Path:    filepath.Join(s.root, s.cfg.FeatureDirectory, feature),
			Problem: fmt.Sprintf("step-file-pattern %q is invalid: %v", s.cfg.StepFilePattern, err),
			Fix:     "fix step-file-pattern in .brief.yaml",
			Err:     stepfile.ErrInvalidPattern,
		}
	}

	featureDirPath := filepath.Join(s.root, s.cfg.FeatureDirectory)
	featurePath := filepath.Join(featureDirPath, feature)

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

	stepFileName, err := findStepFile(root, pattern, step)
	if err != nil {
		return &RefusalError{
			Path:    featurePath,
			Problem: fmt.Sprintf("no step file found for %q", step),
			Fix:     fmt.Sprintf("run 'brief new step %s' to see the next step, or check the id", feature),
			Err:     ErrNoSuchStep,
		}
	}

	stepPath := filepath.Join(featurePath, stepFileName)

	stepBody, err := root.ReadFile(stepFileName)
	if err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}

	fm, rest, err := stepfile.ParseFrontmatter(stepBody)
	if err != nil {
		return &RefusalError{
			Path:    stepPath,
			Problem: fmt.Sprintf("frontmatter does not parse: %v", err),
			Fix:     "fix the step file's YAML frontmatter",
			Err:     ErrMalformedFeature,
		}
	}

	frontLen := len(stepBody) - len(rest)
	// frontLines converts a line number relative to rest (what findHeading
	// and UnterminatedFence report) into the step file's own absolute line
	// number, so a refusal naming stepPath always points at the line a
	// reader opening that file would need.
	frontLines := strings.Count(string(stepBody[:frontLen]), "\n")
	restStr := string(rest)

	if refusal := validateHandoffAnchor(restStr, s.cfg.HandoffHeading, frontLines, stepPath, fm.Done()); refusal != nil {
		return refusal
	}

	if refusal := checkArgumentFence(handoff, HandoffSource, "handoff"); refusal != nil {
		return refusal
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

	splicedRest, ok := spliceHandoff(rest, s.cfg.HandoffHeading, handoff)
	if !ok {
		return fmt.Errorf("scaffold: %w", ErrMalformedFeature)
	}

	splicedStepBody := make([]byte, 0, frontLen+len(splicedRest))
	splicedStepBody = append(splicedStepBody, stepBody[:frontLen]...)
	splicedStepBody = append(splicedStepBody, splicedRest...)

	// Taken before SetStatus, so the status: line never enters the
	// comparison — otherwise a done step would be the only state in which
	// identity could hold, and the fm.Done() gate below would be dead code.
	identical := string(splicedStepBody) == string(stepBody) &&
		string(state) == string(stateBytes) &&
		newSpec == string(specBytes)

	newStepBody, err := stepfile.SetStatus(splicedStepBody, "done")
	if err != nil {
		return &RefusalError{
			Path:    stepPath,
			Problem: `frontmatter has no "status:" field`,
			Fix:     `add a "status:" key to the step file's frontmatter`,
			Err:     err,
		}
	}

	if fm.Done() && identical {
		return nil
	}

	if err := atomicfile.WriteFile(root, s.cfg.StateFile, state, 0o644); err != nil {
		return writeFailure(err, feature, step)
	}

	if err := atomicfile.WriteFile(root, stepFileName, newStepBody, 0o644); err != nil {
		return writeFailure(err, feature, step)
	}

	if err := atomicfile.WriteFile(root, s.cfg.SpecificationFile, []byte(newSpec), 0o644); err != nil {
		return writeFailure(err, feature, step)
	}

	return nil
}

// validateHandoffAnchor refuses restStr — the frontmatter-stripped step
// body — when it is not fit for spliceHandoff to run unconditionally to
// end of body: the anchor named by heading is missing (naming the
// fence instead, when an earlier one never closes and swallows the
// heading line rather than the untrue "no heading found"); the on-disk
// section under the anchor — the exact region spliceHandoff is about to
// replace — itself opens a fence it never closes; or, only while done is
// false, the anchor is not the last heading in restStr. The fence check
// runs before the anchor-is-last check, and is not gated on done: an
// unterminated fence there hides any heading that follows from
// markdown.TrailingHeading's own fence-aware scan (the same failure mode
// this function exists to catch), so the anchor-is-last check can only
// trust what it sees once the section it walks is known to be
// fence-balanced. A legitimate re-finish's on-disk section is always
// Finish's own prior output, already balanced by checkArgumentFence at
// the write that produced it, so this never fires on one; it fires only
// on a hand-edited file that broke the fence directly (already an R10
// protocol violation before Finish runs), which is unsafe regardless of
// doneness. frontLines converts a line number relative to restStr into
// stepPath's own absolute line number. It returns nil when restStr is fit
// to splice.
func validateHandoffAnchor(restStr, heading string, frontLines int, stepPath string, done bool) *RefusalError {
	if _, ok := markdown.Section(restStr, heading); !ok {
		if line, delim, unterminated := markdown.UnterminatedFence(restStr); unterminated {
			return &RefusalError{
				Path:    stepPath,
				Line:    frontLines + line,
				Problem: fmt.Sprintf("step file has an unclosed %s fence", delim),
				Fix:     "close the fence, or remove the unmatched delimiter, and retry",
				Err:     ErrUnterminatedFence,
			}
		}

		return &RefusalError{
			Path:    stepPath,
			Problem: fmt.Sprintf("no %q heading found", heading),
			Fix:     fmt.Sprintf("add a %q heading to the step file", heading),
			Err:     ErrMalformedFeature,
		}
	}

	// markdown.Section above already proved heading is present, so
	// HeadingStart always succeeds here.
	start, _ := markdown.HeadingStart(restStr, heading)

	if line, delim, unterminated := markdown.UnterminatedFence(restStr[start:]); unterminated {
		absLine := strings.Count(restStr[:start], "\n") + line

		return &RefusalError{
			Path:    stepPath,
			Line:    frontLines + absLine,
			Problem: fmt.Sprintf("step file has an unclosed %s fence", delim),
			Fix:     "close the fence, or remove the unmatched delimiter, and retry",
			Err:     ErrUnterminatedFence,
		}
	}

	// Checked only while the step is still open: once done, everything
	// after the anchor is Finish's own prior output — legitimately
	// containing a heading as ordinary handoff prose — and R11 requires an
	// identical re-finish to leave it untouched rather than refuse it.
	// Before the step is ever finished, anything after the anchor can only
	// have arrived by a hand edit bypassing the tool (R10), and
	// spliceHandoff's unconditional end-of-body contract would otherwise
	// silently overwrite it.
	if done {
		return nil
	}

	if trailing, line, found := markdown.TrailingHeading(restStr, heading); found {
		return &RefusalError{
			Path:    stepPath,
			Line:    frontLines + line,
			Problem: fmt.Sprintf("%q is not the last heading in the step file; found %q after it", heading, trailing),
			Fix:     fmt.Sprintf("move %q out of the step file, or fold its content into the handoff, before finishing", trailing),
			Err:     ErrHandoffNotLast,
		}
	}

	return nil
}

// checkArgumentFence refuses when body — Finish's handoff or state
// argument, named for the error message by label — opens a fenced code
// block it never closes. It names source (HandoffSource or StateSource)
// rather than any file, since Finish never learns which file, or whether
// there was one at all, body's bytes came from. It returns nil when every
// fence body opens is closed.
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
// already rejects is never mistaken for a match.
func findStepFile(root *os.Root, pattern stepfile.Pattern, step string) (string, error) {
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return "", fmt.Errorf("scaffold: %w", err)
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
			return e.Name(), nil
		}
	}

	return "", ErrNoSuchStep
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
