package artifact

import (
	"bytes"
	"strings"
)

// SnippetBegin and SnippetEnd are the marker lines delimiting the CLAUDE.md
// instruction block (R5). A marker line is a whole line exactly equal to
// one of these — internal/setup owns the byte-level rules for locating and
// replacing the span between them.
const (
	SnippetBegin = "<!-- brief:begin -->"
	SnippetEnd   = "<!-- brief:end -->"
)

// snippetTemplatePrefix and snippetTemplateSuffix are SnippetBlock's own
// fixed text, split around the one place the feature directory appears.
// They double as RecognizeSnippet's anchors: a block starting with the
// prefix and ending with the suffix has its feature directory in the bytes
// between them.
const (
	snippetTemplatePrefix = SnippetBegin + "\n" +
		"## brief\n" +
		"Features under `"
	snippetTemplateSuffix = "/` are tracked by `brief`. To work on a step, run\n" +
		"`brief start <feature>` and work from its output rather than reading the specification\n" +
		"or earlier steps whole. Close the step with\n" +
		"`brief finish <feature> <step> --handoff <path> --state <path>` — never write a handoff\n" +
		"or tick the progress list by hand. `brief --help` for the rest.\n" +
		SnippetEnd
)

// SnippetBlock renders the CLAUDE.md instruction block (R5) naming dir as
// the feature directory: the begin marker, the block's fixed prose with dir
// embedded once, and the end marker, with no trailing newline. dir has any
// trailing "/" trimmed first, so the rendered text reads the directory with
// exactly one slash.
func SnippetBlock(dir string) []byte {
	dir = strings.TrimRight(dir, "/")

	var b strings.Builder
	b.WriteString(snippetTemplatePrefix)
	b.WriteString(dir)
	b.WriteString(snippetTemplateSuffix)

	return []byte(b.String())
}

// SnippetMatch is RecognizeSnippet's own result.
type SnippetMatch struct {
	// Origin is OriginCurrent when block's bytes equal a known snippet
	// template rendered for some feature directory, OriginEdited otherwise.
	Origin Origin
	// Dir is the feature directory extracted from block's own text — the
	// directory that template was rendered for, not necessarily a
	// repository's currently configured one. Empty when Origin is
	// OriginEdited.
	Dir string
}

// snippetTemplates lists every snippet template this package can
// recognize, in the order RecognizeSnippet tries them: today only
// SnippetBlock's own current render. A future release appends an older
// template here without removing this one, so an earlier release's block is
// still recognized rather than misclassified as edited.
var snippetTemplates = []func(dir string) []byte{
	SnippetBlock,
}

// RecognizeSnippet reports block's Origin against every known snippet
// template (snippetTemplates): a block matching a template's own fixed
// prefix and suffix, with the bytes between them re-rendering, through that
// same template, to exactly block, is OriginCurrent — recognition is
// config-independent, so a block brief-written for any feature directory
// matches, not only the caller's currently configured one — with Dir
// reporting the directory it was rendered for. Anything else, including a
// correctly worded block whose line endings were changed, is OriginEdited.
func RecognizeSnippet(block []byte) SnippetMatch {
	for _, render := range snippetTemplates {
		if dir, ok := extractSnippetDir(render, block); ok {
			return SnippetMatch{Origin: OriginCurrent, Dir: dir}
		}
	}

	return SnippetMatch{Origin: OriginEdited}
}

// extractSnippetDir reports the feature directory embedded in block, using
// render's own fixed prefix/suffix as anchors — SnippetBlock's prefix and
// suffix specifically, today the only render this package tries. The
// extracted directory is re-rendered through render and compared back
// against block byte-for-byte, so an extraction that only coincidentally
// matches the anchors never reports a false positive.
func extractSnippetDir(render func(string) []byte, block []byte) (string, bool) {
	prefix := []byte(snippetTemplatePrefix)
	suffix := []byte(snippetTemplateSuffix)

	if !bytes.HasPrefix(block, prefix) || !bytes.HasSuffix(block, suffix) {
		return "", false
	}

	dir := string(block[len(prefix) : len(block)-len(suffix)])

	if !bytes.Equal(render(dir), block) {
		return "", false
	}

	return dir, true
}
