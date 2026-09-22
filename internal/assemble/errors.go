package assemble

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// ErrNoSuchFeature is returned when the named feature has no directory
// under the configured feature directory.
var ErrNoSuchFeature = errors.New("no such feature")

// ErrMalformedFeature is returned when a feature directory exists but is
// missing a file Start requires, such as its specification or state file,
// or that file cannot be read as intended — including a fenced code block
// that never closes, which would otherwise make every configured section
// after the fence opens read as absent.
var ErrMalformedFeature = errors.New("malformed feature")

// Problem describes why Status could not read a feature directory or one
// of its step files, or why Start refused to assemble a Brief: Path is the
// absolute path of the offending directory or file — the step file itself
// when the fault is that file's own frontmatter, never the feature
// directory it lives in — Detail is the underlying failure's own message,
// and Fix is the one-line remedy printed beside it.
type Problem struct {
	Path   string
	Detail string
	Fix    string
}

// RefusalError is the read-side refusal Start returns when it declines to
// assemble a Brief rather than return one that silently omits context: the
// embedded Problem names the absolute path, what was wrong with it and how
// to fix it; Line is the 1-based line number within Path the refusal points
// at (0 when it names the whole file); and Err is the sentinel this
// refusal wraps for errors.Is — cli/refusal.go renders these fields into
// R14a's one-line refusal template, appending ":Line" to the path when Line
// is set, with no "(no files changed)" tail: a read refusal changes
// nothing on disk by construction. It is the read-side counterpart of
// scaffold.RefusalError, duplicated rather than shared because assemble
// must not import scaffold.
type RefusalError struct {
	Problem

	Line int
	Err  error
}

// Error renders "<path>: <detail>; <fix>", or "<path>:<line>: <detail>;
// <fix>" when Line is set.
func (e *RefusalError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s; %s", e.Path, e.Line, e.Detail, e.Fix)
	}

	return fmt.Sprintf("%s: %s; %s", e.Path, e.Detail, e.Fix)
}

// Unwrap exposes Err so errors.Is reaches the sentinel this refusal wraps.
func (e *RefusalError) Unwrap() error {
	return e.Err
}

// readClassFix is the remedy offered when a feature directory or step file
// could not be opened or read at all, as opposed to being read and found
// unparseable.
const readClassFix = "make it readable and re-run"

// stepFrontmatterError is the error readSteps wraps a
// stepfile.ParseFrontmatter failure in: name is the step file's own
// filename within the feature directory, the one piece of information
// ParseFrontmatter's own error carries nothing of. Unwrap returns err
// unchanged, so errors.Is(err, stepfile.ErrNoFrontmatter) still reaches the
// sentinel through this wrapper.
type stepFrontmatterError struct {
	name string
	err  error
}

// Error renders "assemble: <name>: <err>".
func (e *stepFrontmatterError) Error() string {
	return fmt.Sprintf("assemble: %s: %s", e.name, e.err)
}

// Unwrap exposes err so errors.Is/errors.As reach the sentinel this error
// wraps.
func (e *stepFrontmatterError) Unwrap() error {
	return e.err
}

// newProblem converts err, returned while opening or listing a feature's
// own directory, while reading its specification or state file, or while
// reading and parsing one of its step files, into the Problem the caller
// reports.
//
// An open/read failure surfaces as a wrapped *fs.PathError; when nameable
// is true, that error's own Path field (relative to the feature's root) is
// joined onto base to name the offending file, and the PathError's own
// wrapped message becomes Detail, with Fix pointing at making the file
// readable.
//
// A step-file frontmatter parse failure surfaces as a *stepFrontmatterError:
// its own name is joined onto base to name the step file, its wrapped
// error's own message becomes Detail, prefixed "frontmatter does not
// parse: " — the same prefix assemble.Check's own C6 finding and
// scaffold.Finish's own frontmatter refusal both already carry, one wording
// for the one fault regardless of which of the three read paths meets it —
// and Fix always names 'brief check <feature>' — filepath.Base(base), the
// feature directory, never the step file's own name — as the one place
// every fault in the feature is listed.
//
// Every other error falls to the third, unreached branch: no current caller
// passes newProblem anything but a *fs.PathError or a *stepFrontmatterError,
// so this stays a documented default rather than a tested one.
func newProblem(base string, err error, nameable bool) *Problem {
	if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
		path := base
		if nameable {
			path = filepath.Join(base, pathErr.Path)
		}

		return &Problem{Path: path, Detail: pathErr.Err.Error(), Fix: readClassFix}
	}

	if fmErr, ok := errors.AsType[*stepFrontmatterError](err); ok {
		return &Problem{
			Path:   filepath.Join(base, fmErr.name),
			Detail: "frontmatter does not parse: " + fmErr.err.Error(),
			Fix:    fmt.Sprintf("run 'brief check %s' to list every fault", filepath.Base(base)),
		}
	}

	detail := strings.TrimPrefix(err.Error(), "assemble: ")

	return &Problem{
		Path:   base,
		Detail: detail,
		Fix:    fmt.Sprintf("fix its frontmatter, or run 'brief new step %s' to scaffold a conforming step file", filepath.Base(base)),
	}
}
