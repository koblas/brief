package scaffold

import (
	"github.com/koblas/brief/internal/platform/rwfs"
)

// replaceBytes atomically replaces name under fsys with data, at 0o600 —
// applied only on a fresh create, matching writeExclusive's siblings — so
// the caller-created handoff file never lands wider than its siblings. It
// returns peelWriteErr's result unwrapped so the caller owns the one place
// this boundary is wrapped.
func replaceBytes(fsys rwfs.FS, name string, data []byte) error {
	if err := fsys.WriteFile(name, data, 0o600); err != nil {
		return peelWriteErr(err)
	}

	return nil
}

// replaceString is replaceBytes' string-data counterpart.
func replaceString(fsys rwfs.FS, name, data string) error {
	if err := fsys.WriteFile(name, []byte(data), 0o600); err != nil {
		return peelWriteErr(err)
	}

	return nil
}
