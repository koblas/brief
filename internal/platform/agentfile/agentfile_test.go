package agentfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeAgentFile writes body at path, creating every parent directory it
// needs.
func writeAgentFile(t *testing.T, path, body string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

func Test_find_matches_frontmatter_name_anywhere_under_project_agents(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()

	path := filepath.Join(root, ".claude", "agents", "developer", "Agent.md")
	writeAgentFile(t, path, "---\nname: developer\n---\n\nDeveloper agent.\n")

	defs := agentfile.Find(root, home, "developer")

	require.Len(t, defs, 1)
	assert.Equal(t, agentfile.ScopeProject, defs[0].Scope)
	assert.Equal(t, path, defs[0].Path)
	assert.Equal(t, "developer", defs[0].Frontmatter.Name)
}

func Test_find_ignores_a_filename_match_whose_name_differs(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()

	path := filepath.Join(root, ".claude", "agents", "developer.md")
	writeAgentFile(t, path, "---\nname: someone-else\n---\n\nNot the developer.\n")

	defs := agentfile.Find(root, home, "developer")

	assert.Empty(t, defs, "the filename must play no part in the match")
}

func Test_find_skips_files_whose_frontmatter_does_not_parse(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "no frontmatter delimiter", body: "just a plain agent file\n"},
		{name: "unclosed frontmatter delimiter", body: "---\nname: developer\n\nno closing delimiter here\n"},
		{name: "yaml that fails to decode", body: "---\nname: [this is not: valid\n---\n\nbody\n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			home := t.TempDir()
			path := filepath.Join(root, ".claude", "agents", "developer.md")
			writeAgentFile(t, path, c.body)

			defs := agentfile.Find(root, home, "developer")
			assert.Empty(t, defs, "invalid frontmatter must not be a candidate")

			// Control: the same path with valid frontmatter is found —
			// proves the empty result above came from the parse failure,
			// not from an unrelated resolution bug.
			writeAgentFile(t, path, "---\nname: developer\n---\n\nvalid body\n")

			defs = agentfile.Find(root, home, "developer")
			require.Len(t, defs, 1)
			assert.Equal(t, "developer", defs[0].Frontmatter.Name)
		})
	}
}

func Test_find_prefers_project_definitions_over_user_level(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()

	projectPath := filepath.Join(root, ".claude", "agents", "developer.md")
	homePath := filepath.Join(home, ".claude", "agents", "developer.md")
	writeAgentFile(t, projectPath, "---\nname: developer\n---\n\nproject body\n")
	writeAgentFile(t, homePath, "---\nname: developer\n---\n\nhome body\n")

	defs := agentfile.Find(root, home, "developer")

	require.Len(t, defs, 1)
	assert.Equal(t, agentfile.ScopeProject, defs[0].Scope)
	assert.Equal(t, projectPath, defs[0].Path)

	// Control: with no project definition, the same name resolves to the
	// home definition rather than being skipped outright.
	require.NoError(t, os.Remove(projectPath))

	defs = agentfile.Find(root, home, "developer")

	require.Len(t, defs, 1)
	assert.Equal(t, agentfile.ScopeUser, defs[0].Scope)
	assert.Equal(t, homePath, defs[0].Path)
}

func Test_find_returns_every_project_duplicate_in_lexical_order(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()

	first := filepath.Join(root, ".claude", "agents", "my-reviewer.md")
	second := filepath.Join(root, ".claude", "agents", "team", "r.md")
	// Written out of lexical order on purpose — Find's own order must not
	// depend on write order or directory creation order.
	writeAgentFile(t, second, "---\nname: my-reviewer\n---\n\nsecond\n")
	writeAgentFile(t, first, "---\nname: my-reviewer\n---\n\nfirst\n")

	defs := agentfile.Find(root, home, "my-reviewer")

	require.Len(t, defs, 2)
	assert.Equal(t, first, defs[0].Path)
	assert.Equal(t, second, defs[1].Path)
}

func Test_find_edge_cases(t *testing.T) {
	t.Run("an empty home skips the user scope", func(t *testing.T) {
		root := t.TempDir()

		defs := agentfile.Find(root, "", "developer")

		assert.Empty(t, defs)
	})

	t.Run("a missing .claude/agents returns nothing", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()

		defs := agentfile.Find(root, home, "developer")

		assert.Empty(t, defs)
	})

	t.Run("a non-.md file with a matching name is ignored", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()
		path := filepath.Join(root, ".claude", "agents", "developer.txt")
		writeAgentFile(t, path, "---\nname: developer\n---\n\nbody\n")

		defs := agentfile.Find(root, home, "developer")

		assert.Empty(t, defs)
	})
}
