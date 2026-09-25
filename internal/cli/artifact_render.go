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

// landedArtifacts filters res.Artifacts down to the ones res.Created,
// res.Modified or res.Removed actually names, in res.Artifacts' own order —
// init's or uninstall's own partial-write report (renderPartialWrite) must
// print only what really landed on disk, never the full plan: res.Artifacts
// on a partial write still carries the row for the write that failed,
// Action and all, exactly as it would have read had that write succeeded.
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

// renderPartialWrite renders a write command's own partial-write failure
// (setup.ErrPartialWrite, R3/R11): text mode prints every row that
// actually landed (landedArtifacts) on stdout — the same shape a
// successful run would print — before the ordinary refusal line on
// stderr; JSON mode is the standard error document (out.refusal),
// unchanged: files_changed is already true through filesChangedFor's own
// errors.Is(setup.ErrPartialWrite) check, and R3's own envelope carries no
// artifacts field for a failing run, matching R10's own ErrUnwritable
// contract above.
func renderPartialWrite(res setup.Result, err error, wd string, out reporter) error {
	if !out.json {
		for _, a := range landedArtifacts(res) {
			fmt.Fprintln(out.stdout, artifactRow(wd, a))
		}
	}

	return out.refusal(err)
}

// changedFiles reports whether an init or uninstall run changed any file:
// created, modified or removed one. A dry run, a print-only run and a
// re-run with nothing left to do all report false.
func changedFiles(res setup.Result) bool {
	return len(res.Created)+len(res.Modified)+len(res.Removed) > 0
}
