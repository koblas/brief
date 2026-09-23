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
// ActionKept artifact are excluded — except the CLAUDE.md snippet's own
// ActionKept "not a regular file" row (snippetArt.notRegular), which prints
// as PrintMerge anyway: apply never writes through it either, but unlike
// every other ActionKept artifact there is no existing content on disk to
// leave alone, so the adopter still needs the block's own bytes to add by
// hand. configBody is the variant apply would write (ConfigFile, or under
// WithAgents ConfigFileWithRoles); writeArts supplies every plugin and
// agent file's own render body, keyed by path; boundAgentArts supplies
// every KindBoundAgent row's own inserted or rewritten line (the whole
// file's bytes are never printed for one, only that line); the snippet's
// own body always comes from artifact.SnippetBlock(dir), not from
// writeArts. The result is never nil.
func printArtifacts(artifacts []Artifact, configBody []byte, writeArts []pluginArtifact, boundAgentArts []boundAgentArtifact, snippetArt snippetArtifact) []PrintArtifact {
	bodies := make(map[string][]byte, len(writeArts))
	for _, w := range writeArts {
		bodies[w.Path] = artifact.Render(w.renderKind)
	}

	boundAgentLines := make(map[string]string, len(boundAgentArts))
	for _, ba := range boundAgentArts {
		boundAgentLines[ba.Path] = ba.line
	}

	out := make([]PrintArtifact, 0, len(artifacts))

	for _, a := range artifacts {
		if a.Kind == KindFeatureRoot {
			continue
		}

		var action PrintAction

		switch {
		case a.Action == ActionCreated:
			action = PrintCreate
		case a.Action == ActionMerged:
			action = PrintMerge
		case a.Kind == KindSnippet && a.Action == ActionKept && snippetArt.notRegular:
			action = PrintMerge
		default:
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
		case KindPlugin, KindHook, KindSkill, KindAgent:
			body = bodies[a.Path]
		case KindBoundAgent:
			body = []byte(boundAgentLines[a.Path])
		}

		out = append(out, PrintArtifact{Path: a.Path, Action: action, Body: string(body)})
	}

	return out
}
