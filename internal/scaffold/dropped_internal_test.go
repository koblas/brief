package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_trailing_tag_extraction pins D3's grammar for the unexported
// trailing-tag extractor: a trailing "(<token>)" with a non-empty,
// whitespace-free token is a tag; anything else is untagged.
func Test_trailing_tag_extraction(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{name: "a trailing token in parens is the tag", text: "X (SCENARIO-02)", want: "SCENARIO-02"},
		{name: "no parens at all", text: "X", want: ""},
		{name: "empty parens carry no token", text: "X ()", want: ""},
		{name: "whitespace inside the parens is not a token", text: "X (a b)", want: ""},
		{name: "parens in the middle of the text are not trailing", text: "X (x) tail", want: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, trailingTag(c.text))
		})
	}
}
