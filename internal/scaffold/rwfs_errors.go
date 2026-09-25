package scaffold

import (
	"errors"
	"io/fs"
)

// peelWriteErr returns err's own Err field when err is an *fs.PathError —
// the shape every rwfs.FS write method wraps every failure in — so a
// caller that adds its own context does not stack rwfs's own "<op> <name>:
// " prefix on top of it. It is deliberately not applied to the two write
// paths rwfs classifies without preserving the underlying OS error (a
// traversing name, or "exists but is not a directory"), where peeling
// would drop the name entirely; those keep rwfs's wrapping.
func peelWriteErr(err error) error {
	if pe, ok := errors.AsType[*fs.PathError](err); ok {
		return pe.Err
	}

	return err
}

// peelReadErr returns err's immediate cause when err carries rwfs's own
// read-side wrap, so a caller's own "scaffold: ..." context does not stack
// a second "rwfs:" segment. It returns err unchanged otherwise.
func peelReadErr(err error) error {
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return unwrapped
	}

	return err
}
