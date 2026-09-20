package markdown

import "strings"

// CountLines returns the number of lines in body: 0 for an empty body,
// otherwise strings.Count(body, "\n") plus 1 when body does not end in
// "\n" — a trailing newline never adds a phantom line. "\r\n" counts once,
// like every other line ending, since the count is driven by "\n" alone.
func CountLines(body string) int {
	if body == "" {
		return 0
	}

	n := strings.Count(body, "\n")
	if !strings.HasSuffix(body, "\n") {
		n++
	}

	return n
}
