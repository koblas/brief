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
// absolute path of the offending directory or file, Detail is the
// underlying failure's own message, and Fix is the one-line remedy printed
// beside it.
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

// newProblem converts err, returned while opening or listing a feature's
// own directory, while reading its specification or state file, or while
// reading and parsing one of its step files, into the Problem the caller
// reports. Every open/read failure surfaces as a wrapped *fs.PathError;
// when nameable is true, that error's own Path field (relative to the
// feature's root) is joined onto base to name the offending file, and the
// PathError's own wrapped message becomes Detail. stepfile.ParseFrontmatter's
// errors carry no file name of their own, so a frontmatter parse failure —
// the only case that is not a *fs.PathError — keeps base as Path and its
// full message, stripped of the "assemble: " wrap, as Detail. nameable is
// false when base already names the exact file err concerns, so joining a
// relative path onto it would repeat it.
func newProblem(base string, err error, nameable bool) *Problem {
	if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
		path := base
		if nameable {
			path = filepath.Join(base, pathErr.Path)
		}

		return &Problem{Path: path, Detail: pathErr.Err.Error(), Fix: readClassFix}
	}

	detail := strings.TrimPrefix(err.Error(), "assemble: ")

	return &Problem{
		Path:   base,
		Detail: detail,
		Fix:    fmt.Sprintf("fix its frontmatter, or run 'brief new step %s' to scaffold a conforming step file", filepath.Base(base)),
	}
}
