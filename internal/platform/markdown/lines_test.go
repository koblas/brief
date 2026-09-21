package markdown_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/stretchr/testify/assert"
)

// Test_CountLines covers the whole contract in one table: the unit is
// "lines", a trailing newline terminates the last line rather than opening
// a new one, and a CRLF body counts the same as an LF one. Each case is a
// single input and a single expected count, so a table states the rule more
// plainly than seven near-identical functions did.
func Test_CountLines(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{name: "empty body", body: "", want: 0},
		{name: "only a newline", body: "\n", want: 1},
		{name: "one line, no trailing newline", body: "a", want: 1},
		{name: "one line, trailing newline", body: "a\n", want: 1},
		{name: "two lines, no trailing newline", body: "a\nb", want: 2},
		{name: "two lines, trailing newline", body: "a\nb\n", want: 2},
		{name: "CRLF counts as LF", body: "a\r\nb\r\n", want: 2},
		{name: "blank interior line still counts", body: "a\n\nb\n", want: 3},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, markdown.CountLines(c.body))
		})
	}
}
