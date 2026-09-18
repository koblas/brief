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

// frontmatterDelim is the line that opens and closes a step file's YAML
// frontmatter block.
const frontmatterDelim = "---"

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

// ParseFrontmatter splits body into its YAML frontmatter and the body that
// follows it. It returns ErrNoFrontmatter when body does not start with a
// "---" delimiter line, and a wrapped error when the enclosed YAML does
// not decode onto Frontmatter. The returned rest is body's bytes after the
// closing delimiter line, with the delimiter's own trailing newline
// consumed.
func ParseFrontmatter(body []byte) (Frontmatter, []byte, error) {
	s := string(body)

	if !strings.HasPrefix(s, frontmatterDelim+"\n") {
		return Frontmatter{}, nil, ErrNoFrontmatter
	}

	afterOpen := s[len(frontmatterDelim)+1:]

	yamlPart, afterClose, found := strings.Cut(afterOpen, "\n"+frontmatterDelim)
	if !found {
		return Frontmatter{}, nil, fmt.Errorf("stepfile: %w: no closing frontmatter delimiter", ErrNoFrontmatter)
	}

	rest := strings.TrimPrefix(afterClose, "\n")

	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(yamlPart), &fm); err != nil {
		return Frontmatter{}, nil, fmt.Errorf("stepfile: parse frontmatter: %w", err)
	}

	return fm, []byte(rest), nil
}
