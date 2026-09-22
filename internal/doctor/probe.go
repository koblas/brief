package doctor

import (
	"debug/buildinfo"
	"os"
)

// probeWritable reports whether dir can be written to: it creates a
// temporary file inside dir, closes it, and removes it immediately —
// the only write doctor ever performs, and the one root-dir's own
// writability check relies on rather than syscall.Access (unix-only, and
// go.mod carries no golang.org/x/sys) or inspecting permission bits by
// hand, which root-owned directories and ACLs can make misleading.
func probeWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".brief-doctor-probe-*")
	if err != nil {
		return false
	}

	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)

	return true
}

// readBinaryVersion reads path's own embedded Go module version via
// debug/buildinfo.ReadFile, which reads the binary's build metadata
// without executing it — production's own WithBinaryVersion adapter, so
// env-path never runs the PATH binary just to ask its version.
func readBinaryVersion(path string) (string, bool) {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return "", false
	}

	return info.Main.Version, true
}
