package artifact

import (
	"bytes"
	"strings"
)

// SnippetBegin and SnippetEnd are the marker lines delimiting the CLAUDE.md
// instruction block (R5). A marker line is a whole line exactly equal to
// one of these. ScanSnippetMarkers (this package) locates the span between
// them in an existing file's bytes; internal/setup owns merging a new block
// into that span and removing it, and internal/doctor reads the same span
// to classify a CLAUDE.md's own health.
const (
	SnippetBegin = "<!-- brief:begin -->"
	SnippetEnd   = "<!-- brief:end -->"
)

// snippetTemplatePrefix and snippetTemplateSuffix are SnippetBlock's own
// fixed text, split around the one place the feature directory appears.
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
	// Origin is OriginCurrent when block's bytes equal a current snippet
	// template rendered for some feature directory, OriginOlder for an
	// earlier release's own template, OriginEdited otherwise.
	Origin Origin
	// Dir is the feature directory extracted from block's own text — the
	// directory that template was rendered for, not necessarily a
	// repository's currently configured one. Empty when Origin is
	// OriginEdited.
	Dir string
}

// snippetTemplates lists every snippet template this package renders
// today, in the order RecognizeSnippet tries them: today only SnippetBlock's
// own current render.
var snippetTemplates = []func(dir string) []byte{
	SnippetBlock,
}

// olderSnippetTemplates lists every earlier release's own snippet template
// no longer in snippetTemplates, empty until a release changes
// SnippetBlock's own fixed prose.
var olderSnippetTemplates = []func(dir string) []byte{}

// RecognizeSnippet reports block's Origin against snippetTemplates and
// olderSnippetTemplates: OriginCurrent for a match against the current
// template, OriginOlder for a match against an earlier one, OriginEdited
// otherwise — recognizeSnippetWith's own precedence. Recognition is
// config-independent, so a block brief-written for any feature directory
// matches, not only the caller's currently configured one, with Dir
// reporting the directory it was rendered for.
func RecognizeSnippet(block []byte) SnippetMatch {
	return recognizeSnippetWith(snippetTemplates, olderSnippetTemplates, block)
}

// recognizeSnippetWith is RecognizeSnippet's own decision logic: a block
// matching a current template's own fixed prefix and suffix, with the bytes
// between them re-rendering, through that same template, to exactly block,
// is OriginCurrent; the same match against an older template is OriginOlder;
// anything else, including a correctly worded block whose line endings were
// changed, is OriginEdited. current is tried before older, so a block a
// current template happens to also match is always OriginCurrent.
func recognizeSnippetWith(current, older []func(dir string) []byte, block []byte) SnippetMatch {
	for _, render := range current {
		if dir, ok := extractSnippetDir(render, block); ok {
			return SnippetMatch{Origin: OriginCurrent, Dir: dir}
		}
	}

	for _, render := range older {
		if dir, ok := extractSnippetDir(render, block); ok {
			return SnippetMatch{Origin: OriginOlder, Dir: dir}
		}
	}

	return SnippetMatch{Origin: OriginEdited}
}

// snippetDirSentinel is an arbitrary marker with no meaning of its own,
// rendered through a template once to locate the one place it embeds its
// dir argument — never written to disk, never compared against a real
// block's own bytes.
const snippetDirSentinel = "\x00brief-snippet-dir-sentinel\x00"

// extractSnippetDir reports the feature directory embedded in block, using
// render's own fixed prefix and suffix as anchors: rendering render with
// snippetDirSentinel locates that template's own prefix (the bytes before
// the sentinel) and suffix (the bytes after it), so each template in
// snippetTemplates/olderSnippetTemplates supplies its own anchors rather
// than sharing one global pair. The extracted directory is re-rendered
// through render and compared back against block byte-for-byte, so an
// extraction that only coincidentally matches the anchors never reports a
// false positive.
func extractSnippetDir(render func(string) []byte, block []byte) (string, bool) {
	anchor := render(snippetDirSentinel)

	prefix, suffix, ok := bytes.Cut(anchor, []byte(snippetDirSentinel))
	if !ok {
		return "", false
	}

	if !bytes.HasPrefix(block, prefix) || !bytes.HasSuffix(block, suffix) {
		return "", false
	}

	dir := string(block[len(prefix) : len(block)-len(suffix)])

	if !bytes.Equal(render(dir), block) {
		return "", false
	}

	return dir, true
}
