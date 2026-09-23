package artifact

import (
	"embed"
	"fmt"
)

// files holds every fixed artifact brief installs, byte for byte as it is
// written to disk: the plugin manifest and hook wiring, the start, finish
// and brief-workflow skills, the three role agents, and — under older/ —
// earlier releases' renders that Recognize still reports as older. Their
// bytes are the contract Recognize's digests are computed from, so
// .gitattributes pins the directory against line-ending conversion.
//
//go:embed files
var files embed.FS

// mustReadFile returns name's own bytes from files. name is always one of
// this package's own fixed paths, so a missing file is a build-time
// packaging bug, not a runtime condition a caller can act on.
func mustReadFile(name string) []byte {
	body, err := files.ReadFile("files/" + name)
	if err != nil {
		panic(fmt.Sprintf("artifact: read embedded %s: %v", name, err))
	}

	return body
}
