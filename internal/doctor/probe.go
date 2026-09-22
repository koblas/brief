package doctor

import (
	"debug/buildinfo"
)

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
