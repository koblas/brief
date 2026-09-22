package artifact

// White-box package: recognizeSnippetWith is RecognizeSnippet's own
// unexported decision logic, extracted so a synthetic "older template" can
// drive the OriginOlder arm — olderSnippetTemplates ships empty today (no
// earlier release has changed SnippetBlock's own text), so that arm has
// nothing to select against through the public RecognizeSnippet surface.
// snippet_test.go (artifact_test package) covers RecognizeSnippet itself.

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// olderTestTemplate is a synthetic "earlier release" template: same
// begin/end markers, different fixed prose, so it renders bytes
// snippetTemplates' own current template never produces.
func olderTestTemplate(dir string) []byte {
	return []byte(SnippetBegin + "\nold prose about " + dir + "\n" + SnippetEnd)
}

// Test_recognizeSnippetWith_reports_older_for_an_older_template pins
// recognizeSnippetWith's own precedence: a block matching the current
// template list is OriginCurrent; one matching only the older list is
// OriginOlder, with Dir still extracted; anything else is OriginEdited.
func Test_recognizeSnippetWith_reports_older_for_an_older_template(t *testing.T) {
	current := []func(string) []byte{SnippetBlock}
	older := []func(string) []byte{olderTestTemplate}

	cases := []struct {
		name    string
		block   []byte
		want    Origin
		wantDir string
	}{
		{name: "current template", block: SnippetBlock("docs/specifications"), want: OriginCurrent, wantDir: "docs/specifications"},
		{name: "older template", block: olderTestTemplate("docs/specifications"), want: OriginOlder, wantDir: "docs/specifications"},
		{name: "unrecognized bytes", block: []byte("not a brief block"), want: OriginEdited, wantDir: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			match := recognizeSnippetWith(current, older, c.block)

			assert.Equal(t, c.want, match.Origin)
			assert.Equal(t, c.wantDir, match.Dir)
		})
	}
}
