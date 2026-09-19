package scaffold

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/atomicfile"
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
// A re-finish of a step whose frontmatter already says done is a true
// no-op — Finish writes nothing and mtime on all four files is
// preserved — when the handoff file exists and its bytes equal handoff,
// the state bytes equal state, and the spec-with-tick already equals what
// is on disk; any single divergence, including an absent handoff file,
// writes as normal. The step-file conjunct is fm.Done() rather than a
// byte comparison of the step body: with the splice gone the step-file
// write body is a pure function of the on-disk body, so a byte comparison
// would hold in almost exactly the cases fm.Done() holds, and where they
// differ fm.Done() is the correct predicate — the doneness authority is
// the parsed value, not the byte shape, and R11 requires mtime preserved.
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

	handoffPattern, err := stepfile.CompileHandoff(pattern, s.cfg.HandoffFileSuffix)
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
	// is treated as not matching, never as a refusal: the handoff file is
	// this command's own output, not an input the caller must repair.
	existingHandoff, handoffReadErr := root.ReadFile(handoffName)
	handoffMatches := handoffReadErr == nil && string(existingHandoff) == string(handoff)

	identical := handoffMatches &&
		string(state) == string(stateBytes) &&
		newSpec == string(specBytes)

	newStepBody, err := stepfile.SetStatus(stepBody, "done")
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

	if err := atomicfile.WriteFile(root, handoffName, handoff, 0o644); err != nil {
		return writeFailure(err, feature, step)
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
