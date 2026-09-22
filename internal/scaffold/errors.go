package scaffold

import (
	"errors"
	"fmt"

	"github.com/koblas/brief/internal/platform/conform"
)

// ErrInvalidFeatureName is returned when NewFeature is asked to create a
// name that would break the whitespace-separated four-field status
// contract: an empty name, or one containing a rune unicode.IsSpace
// reports true for — the same predicate strings.Fields splits that
// contract's fields on. It is a bare sentinel, not a *RefusalError: an
// invalid name is an invocation defect, reported by cli as a usage error
// rather than as a write refusal.
var ErrInvalidFeatureName = errors.New("invalid feature name")

// ErrNoSuchFeature is returned when the named feature has no directory
// under the configured feature directory.
var ErrNoSuchFeature = errors.New("no such feature")

// ErrFeatureExists is returned when NewFeature is asked to create a
// feature whose directory already exists. It travels inside a
// *RefusalError naming that directory.
var ErrFeatureExists = errors.New("feature already exists")

// ErrMalformedFeature is returned when a feature directory exists but is
// missing a file this package requires, such as its specification.
var ErrMalformedFeature = errors.New("malformed feature")

// ErrNoProgressHeading is returned when a feature's specification has no
// line matching the configured progress heading.
var ErrNoProgressHeading = errors.New("no progress heading found")

// ErrNoSuchStep is returned when a feature directory exists but has no
// step file whose id equals the one Finish was asked to close.
var ErrNoSuchStep = errors.New("no such step")

// ErrNoProgressEntry is returned when a feature's progress list has no
// entry for a step whose file does exist.
var ErrNoProgressEntry = errors.New("no progress entry found")

// ErrUnterminatedFence is returned when a replacement state body given to
// Finish opens a fenced code block it never closes: assemble.Start finds
// every configured heading by scanning forward for a terminator, so an
// open fence there would leave every heading after it unreadable. It is
// conform.ErrUnterminatedFence: assemble.Check reports the same fault as a
// Finding against a state file the write path never validated.
var ErrUnterminatedFence = conform.ErrUnterminatedFence

// ErrAlreadyFinished is returned when Finish is asked to close a step whose
// frontmatter already says done, but the supplied handoff or state differs
// from what is recorded on disk. It travels inside a *RefusalError naming
// the specific divergent file — the recorded handoff file or the state
// file — so the caller can read the recorded bytes and compare rather than
// having their new input silently discarded or the record silently
// replaced. A done step whose handoff file is missing or unreadable is
// exempt from this refusal: with no recorded handoff there is nothing to
// diverge from, and Finish is the only path to a done step, so refusing
// would leave a crash-then-hand-edit tree, or a tree migrated before
// handoff files existed, with no way forward.
var ErrAlreadyFinished = errors.New("step already finished with different inputs")

// ErrOverCap is returned when a body Finish is asked to write measures more
// lines, by markdown.CountLines, than its configured cap
// (cfg.HandoffCapLines for the handoff argument, cfg.StateCapLines for the
// state argument — both share this one sentinel). RefusalError.Path names
// which body: HandoffSource or StateSource. It is conform.ErrOverCap:
// assemble.Check reports the same fault as a Finding against a handoff or
// state file that predates the cap.
var ErrOverCap = conform.ErrOverCap

// ErrOpenChecklistItem is returned when Finish is asked to close a step
// whose own checklist section — the section under cfg.ChecklistHeading —
// still carries an item not ticked with "[x]"/"[X]" (markdown.FirstUnchecked).
// It travels inside a *RefusalError naming the step file and the item's
// line. A checklist with no items, or a step file with no checklist
// heading at all, is never refused this way — the freshly scaffolded step
// NewStep writes is exactly that shape. It is conform.ErrOpenChecklistItem:
// assemble.Check reports the same fault as a Finding, but only against a
// done step — an open step's unticked item is ordinary in-progress work.
var ErrOpenChecklistItem = conform.ErrOpenChecklistItem

// ErrUnmetDependency is returned when Finish is asked to close a step whose
// frontmatter declares a depends-on id that is not a done step, by
// stepfile.DependencyIndex.FirstUnmet. It travels inside a *RefusalError
// naming the step file being finished (Line 0), and covers two distinct
// causes rendered as different copy: the dependency names a step file that
// exists but is not done — including a self-dependency, which can never
// become done through this check alone since the tool refuses rather than
// writes — or the dependency names no step file at all
// (stepfile.DependencyIndex.Known is false). A done step is never refused
// this way, whatever its dependencies say: FirstUnmet short-circuits on
// the dependant's own doneness, so a re-finish of a done step whose
// dependency was later reopened stays a no-op. A step file this scenario
// depends on that cannot be read or whose frontmatter does not parse is
// recorded as a known, not-done step rather than skipped, so it takes the
// "is not finished" branch, never the "names no step file" one — finish
// grows no separate malformed-sibling refusal for that case.
var ErrUnmetDependency = errors.New("step depends on a step that is not finished")

// ErrMissingStateHeading is returned when a replacement state body given to
// Finish carries no section for one of cfg.StateHeadings.Ordered()'s four
// required headings. The check is presence-only (markdown.Section's found
// return), in any order, and a section with an empty body is valid — a
// freshly scaffolded state file with every section empty is accepted; only
// a missing heading line itself is refused. It is
// conform.ErrMissingStateHeading: assemble.Check reports the same fault as
// a Finding against a state file the write path never validated.
var ErrMissingStateHeading = conform.ErrMissingStateHeading

// ErrPartialWrite marks a write-path error returned after at least one of
// Finish's, NewFeature's, or NewStep's own writes already landed on disk —
// distinct from one returned before any of them did. cli's files_changed
// (R3) reads this through errors.Is rather than assuming every write
// command's own failure always changed nothing.
var ErrPartialWrite = errors.New("partial write")

// StateSource is the RefusalError.Path placeholder a refusal carries when
// it concerns the bytes of Finish's state argument rather than a file
// Finish opened itself. Finish never learns where those bytes came from —
// a file, or standard input — so a caller that does know, such as cli's
// --state flag, is expected to replace the placeholder with the real
// source before rendering the refusal.
const StateSource = "<state>"

// HandoffSource is StateSource's counterpart for Finish's handoff argument.
const HandoffSource = "<handoff>"

// RefusalError reports a refusal that changed nothing on disk: the path
// it concerns, what was wrong with it, and how to fix it. cli renders
// these fields into R14a's one-line refusal template, appending ":Line"
// to the path when Line is set. Line is 0 when the refusal names a whole
// file or path rather than one line inside it.
type RefusalError struct {
	Path    string
	Line    int
	Problem string
	Fix     string
	Err     error
}

// Error renders "<path>: <problem>; <fix>", or "<path>:<line>: <problem>;
// <fix>" when Line is set.
func (e *RefusalError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s; %s", e.Path, e.Line, e.Problem, e.Fix)
	}

	return fmt.Sprintf("%s: %s; %s", e.Path, e.Problem, e.Fix)
}

// Unwrap exposes Err so errors.Is reaches the sentinel this refusal wraps.
func (e *RefusalError) Unwrap() error {
	return e.Err
}

// partialWriteError marks err as ErrPartialWrite without changing what
// Error() reports: Go's multi-error Unwrap lets errors.Is reach both err's
// own chain and ErrPartialWrite, while Error() renders exactly what err
// alone would have, so a --json document's "message"/"problem" text is
// never affected by whether a write landed before this error was returned.
type partialWriteError struct {
	err error
}

func (e *partialWriteError) Error() string   { return e.err.Error() }
func (e *partialWriteError) Unwrap() []error { return []error{e.err, ErrPartialWrite} }

// markPartial wraps err with ErrPartialWrite, reporting that at least one
// write already landed before err was produced. It returns nil unchanged.
func markPartial(err error) error {
	if err == nil {
		return nil
	}

	return &partialWriteError{err: err}
}
