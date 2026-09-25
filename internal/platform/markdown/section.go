package markdown

import (
	"regexp"
	"strings"
)

// headingRe matches an ATX heading line: 1-6 leading "#" then whitespace or end of line.
var headingRe = regexp.MustCompile(`^(#{1,6})(\s|$)`)

// fenceRe matches a fenced-code-block delimiter: three or more backticks or tildes.
var fenceRe = regexp.MustCompile("^(`{3,}|~{3,})")

// trimEOL trims trailing spaces, tabs and a carriage return from s.
func trimEOL(s string) string {
	return strings.TrimRight(s, " \t\r")
}

// fenceDelim reports the fence character, run length, and info string of
// the delimiter line opens or closes, tolerating up to three leading
// spaces per CommonMark. The fourth return is false when line is not a
// fence delimiter.
func fenceDelim(line string) (byte, int, string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 {
		return 0, 0, "", false
	}

	trimmed = trimEOL(trimmed)

	m := fenceRe.FindString(trimmed)
	if m == "" {
		return 0, 0, "", false
	}

	return m[0], len(m), trimmed[len(m):], true
}

// fenceState tracks whether a scanner is inside a fenced code block; a
// fence closes only on a run of the same character at least as long as the one that opened it.
type fenceState struct {
	open bool
	ch   byte
	run  int
}

// step updates state for line and reports whether line is a fence
// delimiter; a closing candidate carrying an info string does not close.
func (f *fenceState) step(line string) bool {
	ch, run, info, ok := fenceDelim(line)
	if !ok {
		return false
	}

	if !f.open {
		f.open, f.ch, f.run = true, ch, run
		return true
	}

	if ch == f.ch && run >= f.run && strings.TrimSpace(info) == "" {
		f.open = false
	}

	return true
}

// Section returns the body under the first line in body that equals
// heading, up to the next heading of the same or higher level or end of
// file, with leading and trailing blank lines trimmed. Section returns
// ("", false) when no line equals heading.
func Section(body, heading string) (string, bool) {
	start, end, ok := sectionRange(body, heading)
	if !ok {
		return "", false
	}

	return trimBlankLines(body[start:end]), true
}

// sectionRange returns the byte offsets of the section body under the
// first line in body that equals heading, or (0, 0, false) when no line
// equals heading.
func sectionRange(body, heading string) (int, int, bool) {
	lines := strings.Split(body, "\n")
	offsets := lineOffsets(lines)

	headingIdx, sectionEnd, ok := sectionSpan(lines, heading)
	if !ok {
		return 0, 0, false
	}

	return offsets[headingIdx+1], offsets[sectionEnd], true
}

// sectionSpan returns the line-index boundaries of the section under the
// first line in lines that equals heading, or (0, 0, false) when none does.
func sectionSpan(lines []string, heading string) (int, int, bool) {
	headingIdx, headingLevel := findHeading(lines, heading)
	if headingIdx == -1 {
		return 0, 0, false
	}

	sectionEnd := len(lines)

	var fence fenceState

	for i := headingIdx + 1; i < len(lines); i++ {
		line := lines[i]

		if fence.step(line) {
			continue
		}

		if fence.open {
			continue
		}

		if lvl := headingLevelOf(line); lvl > 0 && lvl <= headingLevel {
			sectionEnd = i
			break
		}
	}

	return headingIdx, sectionEnd, true
}

// lineOffsets returns, for each index i in 0..len(lines), the byte offset
// at which lines[i] begins in the body strings.Split(body, "\n") produced it from.
func lineOffsets(lines []string) []int {
	offsets := make([]int, len(lines)+1)

	for i, line := range lines {
		if i == len(lines)-1 {
			offsets[i+1] = offsets[i] + len(line)
			continue
		}

		offsets[i+1] = offsets[i] + len(line) + 1
	}

	return offsets
}

// HeadingLine returns the 1-based line number of the first line in body
// that equals heading, and ok == false when no such line exists.
func HeadingLine(body, heading string) (int, bool) {
	idx, _ := findHeading(strings.Split(body, "\n"), heading)
	if idx == -1 {
		return 0, false
	}

	return idx + 1, true
}

// Title returns the text of the first level-1 ("# ") heading in body, with
// the leading "#" and surrounding whitespace stripped. A line inside a
// fenced code block is never treated as a heading. Title returns
// ("", false) when body has no level-1 heading.
func Title(body string) (string, bool) {
	for _, line := range lines(body) {
		if headingLevelOf(line) == 1 {
			return strings.TrimSpace(strings.TrimPrefix(trimEOL(line), "#")), true
		}
	}

	return "", false
}

// lines returns the non-fenced lines of body, skipping fence delimiters too.
func lines(body string) []string {
	var out []string

	var fence fenceState

	for line := range strings.SplitSeq(body, "\n") {
		if fence.step(line) {
			continue
		}

		if fence.open {
			continue
		}

		out = append(out, line)
	}

	return out
}

// findHeading returns the index and level of the first line in lines that
// equals heading, or (-1, 0) when no such line exists.
func findHeading(lines []string, heading string) (int, int) {
	var fence fenceState

	for i, line := range lines {
		if fence.step(line) {
			continue
		}

		if fence.open {
			continue
		}

		if trimEOL(line) == heading {
			return i, headingLevelOf(line)
		}
	}

	return -1, 0
}

// headingLevelOf returns line's ATX heading level, or 0 when line is not
// a heading (four or more leading spaces makes it an indented code block instead).
func headingLevelOf(line string) int {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 {
		return 0
	}

	m := headingRe.FindStringSubmatch(trimEOL(trimmed))
	if m == nil {
		return 0
	}

	return len(m[1])
}

// UnterminatedFence reports the first fence delimiter in body that opens
// a fenced code block never closed before end of file: line is its
// 1-based line number and delim is the delimiter text. It returns
// ok == false when every fence opened in body is closed.
func UnterminatedFence(body string) (int, string, bool) {
	var fence fenceState

	var openLine int

	var openDelim string

	for i, l := range strings.Split(body, "\n") {
		wasOpen := fence.open
		if !fence.step(l) {
			continue
		}

		if !wasOpen && fence.open {
			ch, run, _, _ := fenceDelim(l)
			openLine, openDelim = i+1, strings.Repeat(string(ch), run)
		}
	}

	if fence.open {
		return openLine, openDelim, true
	}

	return 0, "", false
}

// trimBlankLines removes leading and trailing blank lines from s, leaving
// internal blank lines untouched.
func trimBlankLines(s string) string {
	lines := strings.Split(s, "\n")

	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}

	return strings.Join(lines[start:end], "\n")
}
