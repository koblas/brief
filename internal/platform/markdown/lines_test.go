package markdown_test

import (
	"testing"

	"github.com/koblas/brief/internal/platform/markdown"
	"github.com/stretchr/testify/assert"
)

func Test_counts_lines_in_an_empty_body_as_zero(t *testing.T) {
	assert.Equal(t, 0, markdown.CountLines(""))
}

func Test_counts_lines_in_a_body_with_no_trailing_newline_as_one(t *testing.T) {
	assert.Equal(t, 1, markdown.CountLines("a"))
}

func Test_counts_lines_in_a_body_with_a_trailing_newline_as_one(t *testing.T) {
	assert.Equal(t, 1, markdown.CountLines("a\n"))
}

func Test_counts_lines_in_a_two_line_body_with_no_trailing_newline_as_two(t *testing.T) {
	assert.Equal(t, 2, markdown.CountLines("a\nb"))
}

func Test_counts_lines_in_a_two_line_body_with_a_trailing_newline_as_two(t *testing.T) {
	assert.Equal(t, 2, markdown.CountLines("a\nb\n"))
}

func Test_counts_lines_in_a_CRLF_body_the_same_as_LF(t *testing.T) {
	assert.Equal(t, 2, markdown.CountLines("a\r\nb\r\n"))
}

func Test_counts_a_body_of_only_a_newline_as_one_line(t *testing.T) {
	assert.Equal(t, 1, markdown.CountLines("\n"))
}
