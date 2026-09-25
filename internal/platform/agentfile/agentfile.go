package agentfile

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/koblas/brief/internal/platform/stepfile"
	"gopkg.in/yaml.v3"
)

// Scope is a Definition's own origin: the repository ("project") or the
// current user's home directory ("user").
type Scope string

const (
	// ScopeProject marks a Definition found under the project's ".claude/agents/".
	ScopeProject Scope = "project"
	// ScopeUser marks a Definition found under the home directory's ".claude/agents/".
	ScopeUser Scope = "user"
)

// Frontmatter holds the fields Find and Load read from an agent file's
// YAML frontmatter: Name, Skills and OmitClaudeMd. Skills and OmitClaudeMd
// decode loosely (see frontmatterSkills, frontmatterOmitClaudeMd) so an
// unexpected shape falls back to zero value instead of failing the decode.
type Frontmatter struct {
	Name         string
	Skills       []string
	OmitClaudeMd bool
}

// rawFrontmatter decodes Skills and OmitClaudeMd into yaml.Node, which
// accepts any shape, deferring interpretation to frontmatterSkills and
// frontmatterOmitClaudeMd.
type rawFrontmatter struct {
	Name         string    `yaml:"name"`
	Skills       yaml.Node `yaml:"skills"`
	OmitClaudeMd yaml.Node `yaml:"omitClaudeMd"`
}

// decode parses body's YAML frontmatter into a Frontmatter, returning the
// error stepfile.DecodeFrontmatter returns for a missing or unparseable block.
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

// frontmatterSkills returns a "skills:" sequence of scalars in document
// order; any other shape (scalar, mapping, non-scalar item, or absent key) is nil.
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

// frontmatterOmitClaudeMd returns an "omitClaudeMd:" bool scalar's value;
// any other shape or an absent key is false.
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

// Load reads path and decodes its YAML frontmatter into a Frontmatter. It
// returns an error when path cannot be read or its frontmatter does not decode.
func Load(path string) (Frontmatter, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Frontmatter{}, err
	}

	return decode(body)
}

// LoadFS is Load's fs.FS-backed twin: it reads name's bytes through fsys
// instead of os.ReadFile, so a resolved binding can be decoded in-memory
// against the same Tree it was resolved from. Errors match Load.
func LoadFS(fsys fs.FS, name string) (Frontmatter, error) {
	body, err := fs.ReadFile(fsys, name)
	if err != nil {
		return Frontmatter{}, err
	}

	return decode(body)
}

// HasSkill reports whether fm's Skills names skill.
func (fm Frontmatter) HasSkill(skill string) bool {
	return slices.Contains(fm.Skills, skill)
}

// Parse decodes body's YAML frontmatter into a Frontmatter — Load's
// bytes-level twin, for a caller that already holds an agent file's content
// in memory rather than a path.
func Parse(body []byte) (Frontmatter, error) {
	return decode(body)
}

// Definition is one agent file whose frontmatter "name:" matched Find's
// query: Path is absolute, Scope names which root it came from, and
// Frontmatter carries its decoded fields.
type Definition struct {
	Path        string
	Scope       Scope
	Frontmatter Frontmatter
}

// Tree is one scope's agent search root: FS holds the tree Find walks,
// rooted so ".claude/agents" names the agents directory, and Dir is the
// absolute OS path FS is rooted at, the prefix joined onto every returned
// Definition.Path. The zero Tree is never searched.
type Tree struct {
	FS  fs.FS
	Dir string
}

// DirTree returns the Tree for an OS directory: FS is os.DirFS of dir's
// absolute form and Dir is that absolute path. An empty dir (an unknown
// home directory) returns the zero Tree.
func DirTree(dir string) Tree {
	if dir == "" {
		return Tree{}
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}

	return Tree{FS: os.DirFS(abs), Dir: abs}
}

// Find matches a bare agent name against frontmatter name: for OS
// directories: FindIn over DirTree(root) and DirTree(home).
func Find(root, home, name string) []Definition {
	return FindIn(DirTree(root), DirTree(home), name)
}

// FindIn returns every "*.md" file under project's ".claude/agents/" whose
// frontmatter "name:" equals name; the filename plays no part in the match.
// It searches user's tree only when project's has no match. A file with no
// frontmatter or unparseable YAML is skipped, not reported; FindIn returns
// no error. Results are sorted by Path.
func FindIn(project, user Tree, name string) []Definition {
	if defs := findIn(project, name, ScopeProject); len(defs) > 0 {
		return defs
	}

	return findIn(user, name, ScopeUser)
}

// findIn walks tree's ".claude/agents" and returns every "*.md" file whose
// frontmatter "name:" equals name, tagged with scope and sorted by Path.
func findIn(tree Tree, name string, scope Scope) []Definition {
	if tree.FS == nil {
		return nil
	}

	var defs []Definition

	_ = fs.WalkDir(tree.FS, ".claude/agents", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			// Unreadable entry: skip it, not the rest of the walk.
			return nil //nolint:nilerr
		}

		if d.IsDir() || !strings.EqualFold(path.Ext(p), ".md") {
			return nil
		}

		body, readErr := fs.ReadFile(tree.FS, p)
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

		defs = append(defs, Definition{
			Path:        filepath.Join(tree.Dir, filepath.FromSlash(p)),
			Scope:       scope,
			Frontmatter: fm,
		})

		return nil
	})

	sort.Slice(defs, func(i, j int) bool { return defs[i].Path < defs[j].Path })

	return defs
}
