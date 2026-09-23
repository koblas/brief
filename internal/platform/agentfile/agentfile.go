package agentfile

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/koblas/brief/internal/platform/stepfile"
	"gopkg.in/yaml.v3"
)

// Scope is a Definition's own origin: the repository ("project") or the
// current user's home directory ("user").
type Scope string

const (
	// ScopeProject marks a Definition found under root's own
	// ".claude/agents/".
	ScopeProject Scope = "project"
	// ScopeUser marks a Definition found under the injected home's own
	// ".claude/agents/", searched only when root has no match.
	ScopeUser Scope = "user"
)

// Frontmatter holds the agent-file fields Find and Load read from an
// agent definition's own YAML frontmatter: Name from "name:"; Skills from
// "skills:", populated only when it decodes as a plain sequence of scalars
// (nil for any other shape — a scalar, a mapping, or the key's absence);
// OmitClaudeMd from "omitClaudeMd:", populated only when it decodes as a
// bare bool (false for any other shape). Skills and OmitClaudeMd decode
// loosely so a value in an unexpected shape never fails the whole decode
// and drops the agent out of Rule 5 resolution — only these two fields
// fall back to their own zero value.
type Frontmatter struct {
	Name         string
	Skills       []string
	OmitClaudeMd bool
}

// rawFrontmatter is the shape decode reads before building a Frontmatter
// from it: Skills and OmitClaudeMd decode into yaml.Node rather than their
// own final Go types, since a yaml.Node accepts whatever shape is present
// without ever failing the decode, deferring interpretation to
// frontmatterSkills and frontmatterOmitClaudeMd.
type rawFrontmatter struct {
	Name         string    `yaml:"name"`
	Skills       yaml.Node `yaml:"skills"`
	OmitClaudeMd yaml.Node `yaml:"omitClaudeMd"`
}

// decode parses body's own YAML frontmatter into a Frontmatter through
// rawFrontmatter's loose shape, returning the same error
// stepfile.DecodeFrontmatter would for a missing or unparseable
// frontmatter block.
func decode(body []byte) (Frontmatter, error) {
	var raw rawFrontmatter
	if _, err := stepfile.DecodeFrontmatter(body, &raw); err != nil {
		return Frontmatter{}, err
	}

	return Frontmatter{
		Name:         raw.Name,
		Skills:       frontmatterSkills(raw.Skills),
		OmitClaudeMd: frontmatterOmitClaudeMd(raw.OmitClaudeMd),
	}, nil
}

// frontmatterSkills classifies a "skills:" node: a sequence whose every
// item is a scalar becomes the list, in document order; anything else — a
// scalar, a mapping, a sequence holding a non-scalar item, or the key's
// absence (a zero yaml.Node) — is nil.
func frontmatterSkills(node yaml.Node) []string {
	if node.Kind != yaml.SequenceNode {
		return nil
	}

	out := make([]string, 0, len(node.Content))

	for _, item := range node.Content {
		var s string
		if item.Kind != yaml.ScalarNode || item.Decode(&s) != nil {
			return nil
		}

		out = append(out, s)
	}

	return out
}

// frontmatterOmitClaudeMd classifies an "omitClaudeMd:" node: a bool
// scalar decodes to its own value; anything else — a non-bool scalar, a
// sequence, a mapping, or the key's absence — is false.
func frontmatterOmitClaudeMd(node yaml.Node) bool {
	if node.Kind != yaml.ScalarNode {
		return false
	}

	var b bool
	if node.Decode(&b) != nil {
		return false
	}

	return b
}

// Load reads path and decodes its own YAML frontmatter into a
// Frontmatter — the single-file entry point a "brief:*" role binding's own
// resolved file uses (doctor's roles-skill row), sharing findIn's own
// decode. It returns an error when path cannot be read, carries no
// frontmatter, or its frontmatter does not decode; Skills and
// OmitClaudeMd decode loosely, the same rule findIn applies.
func Load(path string) (Frontmatter, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Frontmatter{}, err
	}

	return decode(body)
}

// Parse decodes body's own YAML frontmatter into a Frontmatter — the
// bytes-level twin of Load, for a caller that already holds an agent file's
// own content in memory (setup's own bound-agent edit path) rather than a
// path. It decodes through decode, the same loose Skills/OmitClaudeMd rule
// Load and findIn apply, and returns the same error Load would for the
// identical bytes.
func Parse(body []byte) (Frontmatter, error) {
	return decode(body)
}

// Definition is one agent file whose frontmatter "name:" matched Find's
// own query: Path is absolute, Scope names which root it came from, and
// Frontmatter carries its decoded fields.
type Definition struct {
	Path        string
	Scope       Scope
	Frontmatter Frontmatter
}

// Find matches Claude Code's own agent identification (Rule 5): it
// returns every "*.md" file anywhere under "<root>/.claude/agents/",
// recursively, whose frontmatter "name:" equals name — the filename plays
// no part in the match. It searches "<home>/.claude/agents/" only when
// root's own tree has no match; an empty home, or one whose tree has no
// match either, yields no user-scope results at all. A file with no
// frontmatter, no closing delimiter, or YAML that fails to decode is not
// a candidate — it is skipped, not reported. Find returns no error:
// unreadable directories and files are skipped the same way. Results from
// whichever scope matched are absolute paths in lexical walk order.
func Find(root, home, name string) []Definition {
	if defs := findIn(root, name, ScopeProject); len(defs) > 0 {
		return defs
	}

	if home == "" {
		return nil
	}

	return findIn(home, name, ScopeUser)
}

// findIn walks "<base>/.claude/agents/" recursively and returns every
// "*.md" file, in lexical walk order, whose frontmatter "name:" equals
// name, tagged with scope.
func findIn(base, name string, scope Scope) []Definition {
	agentsDir := filepath.Join(base, ".claude", "agents")

	var defs []Definition

	_ = filepath.WalkDir(agentsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Unreadable entry: skip it, not the rest of the walk.
			return nil //nolint:nilerr
		}

		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}

		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		fm, decodeErr := decode(body)
		if decodeErr != nil {
			return nil
		}

		if fm.Name != name {
			return nil
		}

		abs, absErr := filepath.Abs(path)
		if absErr != nil {
			abs = path
		}

		defs = append(defs, Definition{Path: abs, Scope: scope, Frontmatter: fm})

		return nil
	})

	sort.Slice(defs, func(i, j int) bool { return defs[i].Path < defs[j].Path })

	return defs
}
