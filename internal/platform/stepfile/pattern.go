package stepfile

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ErrInvalidPattern is returned by Compile when a step-file-pattern
// configuration value cannot be used to name or recognize step files.
var ErrInvalidPattern = errors.New("invalid step-file pattern")

// verbRe matches a single-integer fmt verb restricted to the forms Number
// can read back: %d and a zero-padded width, %0Nd (including %0d). %3d
// and %-4d pad with spaces, which Number's digits-only scan can never
// read back.
var verbRe = regexp.MustCompile(`%(0[0-9]*)?d`)

// Pattern is a compiled step-file-pattern: a single-integer fmt pattern
// split around its one integer verb, so a filename can be checked against
// the literal prefix and suffix before a candidate number is round-tripped
// back through the pattern.
type Pattern struct {
	raw    string
	prefix string
	suffix string
}

// Compile validates pattern and returns the compiled form, refusing with
// ErrInvalidPattern when pattern is empty or is "." or ".."; contains a
// path separator; does not contain exactly one integer verb matching %d
// or %0Nd; or contains any other "%", including %s, %v or %%.
func Compile(pattern string) (Pattern, error) {
	if pattern == "" || pattern == "." || pattern == ".." {
		return Pattern{}, fmt.Errorf("%q: %w", pattern, ErrInvalidPattern)
	}

	if strings.ContainsAny(pattern, `/\`) {
		return Pattern{}, fmt.Errorf("%q: %w", pattern, ErrInvalidPattern)
	}

	matches := verbRe.FindAllStringIndex(pattern, -1)
	if len(matches) != 1 {
		return Pattern{}, fmt.Errorf("%q: %w", pattern, ErrInvalidPattern)
	}

	start, end := matches[0][0], matches[0][1]
	prefix, suffix := pattern[:start], pattern[end:]

	if strings.Contains(prefix+suffix, "%") {
		return Pattern{}, fmt.Errorf("%q: %w", pattern, ErrInvalidPattern)
	}

	return Pattern{raw: pattern, prefix: prefix, suffix: suffix}, nil
}

// Name renders the filename for step number n.
func (p Pattern) Name(n int) string {
	return fmt.Sprintf(p.raw, n)
}

// ID returns the step id for step number n: Name(n) with its extension
// stripped.
func (p Pattern) ID(n int) string {
	name := p.Name(n)

	return strings.TrimSuffix(name, filepath.Ext(name))
}

// Number reports the step number filename encodes, and whether it is a
// step file at all. filename is a step file for n only if it round-trips
// exactly: Name(n) == filename. A near-miss, such as "SCENARIO-7.md"
// against "SCENARIO-%02d.md", is not a step file even though its digits parse.
func (p Pattern) Number(filename string) (int, bool) {
	if !strings.HasPrefix(filename, p.prefix) || !strings.HasSuffix(filename, p.suffix) {
		return 0, false
	}

	if len(filename) < len(p.prefix)+len(p.suffix) {
		return 0, false
	}

	middle := filename[len(p.prefix) : len(filename)-len(p.suffix)]
	if middle == "" {
		return 0, false
	}

	for _, c := range middle {
		if c < '0' || c > '9' {
			return 0, false
		}
	}

	n, err := strconv.Atoi(middle)
	if err != nil {
		return 0, false
	}

	if p.Name(n) != filename {
		return 0, false
	}

	return n, true
}
