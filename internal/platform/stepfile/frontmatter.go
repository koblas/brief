package stepfile

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrNoFrontmatter is returned by ParseFrontmatter when body does not begin
// with a "---" YAML frontmatter delimiter.
var ErrNoFrontmatter = errors.New("no frontmatter found")

// ErrNoStatusField is returned by SetStatus when body's frontmatter has no
// "status:" line to replace.
var ErrNoStatusField = errors.New("no status field found in frontmatter")

// statusFieldPrefix is the literal text that opens a frontmatter "status:"
// line, matched without decoding the surrounding YAML.
const statusFieldPrefix = "status:"

// frontmatterDelim is the line that opens and closes a step file's YAML
// frontmatter block.
const frontmatterDelim = "---"

// frontmatterOpenLen returns the byte length of an opening frontmatter
// delimiter line ("---\n" or "---\r\n") at the start of s, or 0 if s does
// not begin with one.
func frontmatterOpenLen(s string) int {
	switch {
	case strings.HasPrefix(s, frontmatterDelim+"\r\n"):
		return len(frontmatterDelim) + 2
	case strings.HasPrefix(s, frontmatterDelim+"\n"):
		return len(frontmatterDelim) + 1
	default:
		return 0
	}
}

// Frontmatter holds the machine fields a step file's YAML frontmatter
// carries: its id, its status, and the ids of the steps it depends on.
// ID and DependsOn are read but not validated here; finish and check are
// the write-path owners of what a valid value looks like.
type Frontmatter struct {
	ID        string   `yaml:"id"`
	Status    string   `yaml:"status"`
	DependsOn []string `yaml:"depends-on"`
}

// Done reports whether fm's Status, trimmed and case-folded, is exactly
// "done". Every other value — including "open", "blocked" and the empty
// string — counts as open.
func (fm Frontmatter) Done() bool {
	return strings.EqualFold(strings.TrimSpace(fm.Status), "done")
}

// DecodeFrontmatter splits body into its YAML frontmatter and the
// remaining body, decoding the frontmatter onto out (a pointer). It
// returns ErrNoFrontmatter when body does not start with a "---"
// delimiter line, or a wrapped error when the YAML does not decode onto
// out. rest is body's bytes after the closing delimiter line.
func DecodeFrontmatter(body []byte, out any) ([]byte, error) {
	s := string(body)

	openLen := frontmatterOpenLen(s)
	if openLen == 0 {
		return nil, ErrNoFrontmatter
	}

	afterOpen := s[openLen:]

	yamlPart, afterClose, found := strings.Cut(afterOpen, "\n"+frontmatterDelim)
	if !found {
		return nil, fmt.Errorf("stepfile: %w: no closing frontmatter delimiter", ErrNoFrontmatter)
	}

	rest := strings.TrimPrefix(afterClose, "\n")

	if err := yaml.Unmarshal([]byte(yamlPart), out); err != nil {
		return nil, fmt.Errorf("stepfile: parse frontmatter: %w", err)
	}

	return []byte(rest), nil
}

// ParseFrontmatter splits body into its YAML frontmatter and the
// remaining body. It returns ErrNoFrontmatter when body does not start
// with a "---" delimiter line, or a wrapped error when the YAML does not
// decode onto Frontmatter. rest is body's bytes after the closing
// delimiter line.
func ParseFrontmatter(body []byte) (Frontmatter, []byte, error) {
	var fm Frontmatter

	rest, err := DecodeFrontmatter(body, &fm)
	if err != nil {
		return Frontmatter{}, nil, err
	}

	return fm, rest, nil
}

// SetStatus replaces the first "status:" line inside body's YAML
// frontmatter delimiters with "status: <status>", leaving every other
// byte identical. It returns ErrNoStatusField, body unchanged, when the
// frontmatter has no "status:" line to replace.
func SetStatus(body []byte, status string) ([]byte, error) {
	s := string(body)

	openLen := frontmatterOpenLen(s)
	if openLen == 0 {
		return nil, ErrNoStatusField
	}

	afterOpen := s[openLen:]

	yamlPart, afterClose, found := strings.Cut(afterOpen, "\n"+frontmatterDelim)
	if !found {
		return nil, ErrNoStatusField
	}

	lines := strings.Split(yamlPart, "\n")

	statusIdx := -1

	for i, line := range lines {
		if strings.HasPrefix(line, statusFieldPrefix) {
			statusIdx = i

			break
		}
	}

	if statusIdx == -1 {
		return nil, ErrNoStatusField
	}

	newLine := statusFieldPrefix + " " + status
	if strings.HasSuffix(lines[statusIdx], "\r") {
		newLine += "\r"
	}

	lines[statusIdx] = newLine

	result := s[:openLen] + strings.Join(lines, "\n") + "\n" + frontmatterDelim + afterClose

	return []byte(result), nil
}
