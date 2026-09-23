package agentfile

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/koblas/brief/internal/platform/stepfile"
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

// Frontmatter holds the agent-file fields Find reads from an agent
// definition's own YAML frontmatter.
type Frontmatter struct {
	Name string `yaml:"name"`
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

		var fm Frontmatter
		if _, decodeErr := stepfile.DecodeFrontmatter(body, &fm); decodeErr != nil {
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
