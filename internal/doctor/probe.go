package doctor

import (
	"debug/buildinfo"
)

// readBinaryVersion reads path's embedded Go module version via
// debug/buildinfo.ReadFile, which reads the binary's build metadata
// without executing it.
func readBinaryVersion(path string) (string, bool) {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return "", false
	}

	return info.Main.Version, true
}
