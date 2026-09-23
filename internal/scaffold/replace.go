package scaffold

import (
	"github.com/koblas/brief/internal/platform/rwfs"
)

// replaceBytes atomically replaces name under fsys with data.
//
// The 0o600 perm matches what writeExclusive creates the specification,
// state and step files with, so the handoff file — the only file Finish
// creates rather than replaces — does not land wider than the siblings
// beside it. It applies only on a fresh create: for a file that already
// exists, fsys.WriteFile preserves the mode it already has.
//
// It returns peelWriteErr(fsys.WriteFile's error) on purpose: Finish's
// writeFailure wraps it once for that boundary and adds the retry hint,
// and wrapping here as well would put "scaffold:" in the message twice.
// Peeling rwfs's own "writefile <name>: " framing off first means that
// wrap sees the same underlying atomicfile/OS error scaffold has always
// surfaced, on both adapters.
func replaceBytes(fsys rwfs.FS, name string, data []byte) error {
	if err := fsys.WriteFile(name, data, 0o600); err != nil {
		return peelWriteErr(err)
	}

	return nil
}

// replaceString atomically replaces name under fsys with data. Its perm
// argument carries the same 0o600 as replaceBytes'.
//
// It returns peelWriteErr(fsys.WriteFile's error) on purpose, for the same
// reason as replaceBytes: the caller owns the one place this boundary is
// wrapped.
func replaceString(fsys rwfs.FS, name, data string) error {
	if err := fsys.WriteFile(name, []byte(data), 0o600); err != nil {
		return peelWriteErr(err)
	}

	return nil
}
