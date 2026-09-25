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

// artifactRow renders one artifact as a text-mode line (no trailing
// newline): "<action> <relative path>[/][ (<detail>)]".
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

// landedArtifacts filters res.Artifacts to the ones res.Created,
// res.Modified or res.Removed actually name, in res.Artifacts' order.
func landedArtifacts(res setup.Result) []setup.Artifact {
	landed := make(map[string]bool, len(res.Created)+len(res.Modified)+len(res.Removed))

	for _, p := range res.Created {
		landed[p] = true
	}

	for _, p := range res.Modified {
		landed[p] = true
	}

	for _, p := range res.Removed {
		landed[p] = true
	}

	out := make([]setup.Artifact, 0, len(landed))

	for _, a := range res.Artifacts {
		if landed[a.Path] {
			out = append(out, a)
		}
	}

	return out
}

// renderPartialWrite reports setup.ErrPartialWrite: in text mode it prints
// every artifact that actually landed on disk, in the same shape a
// successful run would use, before the refusal line; it returns the
// standard refusal document either way.
func renderPartialWrite(res setup.Result, err error, wd string, out reporter) error {
	if !out.json {
		// The artifact for the write that failed still lists what its
		// Action would have been had the write succeeded.
		for _, a := range landedArtifacts(res) {
			fmt.Fprintln(out.stdout, artifactRow(wd, a))
		}
	}

	return out.refusal(err)
}
