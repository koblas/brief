package cli

import (
	"fmt"

	"github.com/koblas/brief/internal/setup"
)

// artifactJSON is one setup.Artifact rendered as init's or uninstall's own
// "artifacts" row: kind, path, action and detail exactly as setup.Artifact
// carries them, path always absolute, detail null when empty.
type artifactJSON struct {
	Kind   string  `json:"kind"`
	Path   string  `json:"path"`
	Action string  `json:"action"`
	Detail *string `json:"detail"`
}

// artifactsJSON maps artifacts to initDocument's or uninstallDocument's own
// "artifacts" rows, in setup.Result's own order.
func artifactsJSON(artifacts []setup.Artifact) []artifactJSON {
	out := make([]artifactJSON, 0, len(artifacts))

	for _, a := range artifacts {
		var detail *string
		if a.Detail != "" {
			d := a.Detail
			detail = &d
		}

		out = append(out, artifactJSON{Kind: string(a.Kind), Path: a.Path, Action: string(a.Action), Detail: detail})
	}

	return out
}

// artifactRow renders one artifact as R11's text-mode line, minus the
// trailing newline: "<action> <relative path>[/][ (<detail>)]" — a
// trailing "/" on the feature root's own row, never on the config file's.
func artifactRow(wd string, a setup.Artifact) string {
	path := displayPath(wd, a.Path)
	if a.Kind == setup.KindFeatureRoot {
		path += "/"
	}

	line := fmt.Sprintf("%s %s", a.Action, path)
	if a.Detail != "" {
		line += fmt.Sprintf(" (%s)", a.Detail)
	}

	return line
}
