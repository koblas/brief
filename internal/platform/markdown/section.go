package markdown

import (
	"regexp"
	"strings"
)

// headingRe matches an ATX heading line: one to six leading "#" characters
// followed by whitespace or end of line.
var headingRe = regexp.MustCompile(`^(#{1,6})(\s|$)`)

// fenceRe matches a fenced-code-block delimiter: a run of three or more
// backticks or tildes at the start of the line.
var fenceRe = regexp.MustCompile("^(`{3,}|~{3,})")

// Section returns the body under the first line in body that equals
// heading after right-trimming, excluding the heading line itself. The
// section ends at the next heading of the same or higher level — a "##"
// heading ends a "##" section but a "###" subheading does not — or at end
// of file. Leading and trailing blank lines in the returned body are
// trimmed. A line inside a fenced code block (``` or ~~~) is never treated
// as a heading, whether it is the heading being searched for or the
// heading that would end the section. Section returns ("", false) when no
// line equals heading.
func Section(body, heading string) (string, bool) {
	start, end, ok := SectionRange(body, heading)
	if !ok {
		return "", false
	}

	return trimBlankLines(body[start:end]), true
}

// SectionRange returns the byte offsets of the section body under the
// first line in body that equals heading after right-trimming: start is
// the byte just past the heading line's own newline (or len(body) when the
// heading is the last line and carries no trailing newline), and end is
// the byte offset of the start of the next heading line of the same or
// higher level — including that line's leading whitespace, so
// body[start:end] never truncates the terminating line's indentation — or
// len(body) when no such heading follows. The anchor search and the end
// scan are both fence-aware: a line inside a fenced code block (``` or
// ~~~) is never treated as a heading in either role. Section is
// implemented on top of SectionRange so the two can never disagree about
// where a section ends. SectionRange returns (0, 0, false) when no line
// equals heading.
func SectionRange(body, heading string) (int, int, bool) {
	lines := strings.Split(body, "\n")
	offsets := lineOffsets(lines)

	headingIdx, headingLevel := findHeading(lines, heading)
	if headingIdx == -1 {
		return 0, 0, false
	}

	sectionEnd := len(lines)
	inFence := false

	for i := headingIdx + 1; i < len(lines); i++ {
		line := lines[i]

		if fenceRe.MatchString(strings.TrimRight(line, " \t")) {
			inFence = !inFence
			continue
		}

		if inFence {
			continue
		}

		if lvl := headingLevelOf(line); lvl > 0 && lvl <= headingLevel {
			sectionEnd = i
			break
		}
	}

	return offsets[headingIdx+1], offsets[sectionEnd], true
}

// lineOffsets returns, for each index i in 0..len(lines), the byte offset
// at which lines[i] begins within the body it was split from
// (offsets[len(lines)] is that body's length). lines must be
// strings.Split(body, "\n"): every line except the last is followed by one
// "\n" byte that lineOffsets accounts for; the last is not, since
// strings.Split never manufactures a trailing separator.
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

// Title returns the text of the first level-1 ("# ") heading in body, with
// the leading "#" and surrounding whitespace stripped. A line inside a
// fenced code block is never treated as a heading. Title returns
// ("", false) when body has no level-1 heading.
func Title(body string) (string, bool) {
	for _, line := range lines(body) {
		if headingLevelOf(line) == 1 {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimRight(line, " \t"), "#")), true
		}
	}

	return "", false
}

// lines returns the non-fenced lines of body, in order, skipping every
// line inside a fenced code block (``` or ~~~) including the fence
// delimiters themselves.
func lines(body string) []string {
	var out []string

	inFence := false

	for line := range strings.SplitSeq(body, "\n") {
		if fenceRe.MatchString(strings.TrimRight(line, " \t")) {
			inFence = !inFence
			continue
		}

		if inFence {
			continue
		}

		out = append(out, line)
	}

	return out
}

// findHeading returns the index and heading level of the first line in
// lines, outside any fenced code block, that equals heading after
// right-trimming. It returns (-1, 0) when no such line exists.
func findHeading(lines []string, heading string) (int, int) {
	inFence := false

	for i, line := range lines {
		if fenceRe.MatchString(strings.TrimRight(line, " \t")) {
			inFence = !inFence
			continue
		}

		if inFence {
			continue
		}

		if strings.TrimRight(line, " \t") == heading {
			return i, headingLevelOf(line)
		}
	}

	return -1, 0
}

// headingLevelOf returns the ATX heading level of line — the number of
// leading "#" characters — or 0 when line is not a heading. Up to three
// leading spaces are tolerated before the "#" run, per CommonMark; four or
// more makes the line an indented code block, not a heading.
func headingLevelOf(line string) int {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 {
		return 0
	}

	m := headingRe.FindStringSubmatch(trimmed)
	if m == nil {
		return 0
	}

	return len(m[1])
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
