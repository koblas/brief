package setup

import "github.com/koblas/brief/internal/platform/artifact"

// PrintAction names what a PrintArtifact's own bytes would do to Path if a
// real run applied it: PrintCreate for a file that does not exist yet,
// PrintMerge for one being replaced or appended to in place.
type PrintAction string

const (
	// PrintCreate marks a PrintArtifact a real run would create.
	PrintCreate PrintAction = "create"
	// PrintMerge marks a PrintArtifact a real run would merge into an
	// existing file.
	PrintMerge PrintAction = "merge"
)

// PrintArtifact is one pending artifact InitRequest.Print (R9) reports:
// Path (absolute), Action, and Body — the exact bytes a real run would
// write there. The snippet's own Body is always artifact.SnippetBlock's
// bare block, never the full CLAUDE.md file a real run would merge it
// into.
type PrintArtifact struct {
	Path   string
	Action PrintAction
	Body   string
}

// printArtifacts derives Result.Print from artifacts — Init's own planned
// Artifacts, in that same order — and the bytes apply would write for each
// one: pending files only, ActionCreated mapped to PrintCreate and
// ActionMerged to PrintMerge; the feature root and any ActionUnchanged or
// ActionKept artifact are excluded. configBody is the variant apply would
// write (ConfigFile, or under WithAgents ConfigFileWithRoles); writeArts
// supplies every plugin and agent file's own render body, keyed by path;
// the snippet's own body always comes from artifact.SnippetBlock(dir), not
// from writeArts. The result is never nil.
func printArtifacts(artifacts []Artifact, configBody []byte, writeArts []pluginArtifact, snippetArt snippetArtifact) []PrintArtifact {
	bodies := make(map[string][]byte, len(writeArts))
	for _, w := range writeArts {
		bodies[w.Path] = artifact.Render(w.renderKind)
	}

	out := make([]PrintArtifact, 0, len(artifacts))

	for _, a := range artifacts {
		var action PrintAction

		switch a.Action {
		case ActionCreated:
			action = PrintCreate
		case ActionMerged:
			action = PrintMerge
		case ActionUnchanged, ActionKept, ActionRemoved:
			continue
		default:
			continue
		}

		if a.Kind == KindFeatureRoot {
			continue
		}

		var body []byte

		switch a.Kind {
		case KindSnippet:
			body = artifact.SnippetBlock(snippetArt.dir)
		case KindConfig:
			body = configBody
		case KindFeatureRoot:
			// Never reached: the feature root is excluded above.
		case KindPlugin, KindHook, KindAgent:
			body = bodies[a.Path]
		}

		out = append(out, PrintArtifact{Path: a.Path, Action: action, Body: string(body)})
	}

	return out
}
