package agentfile_test

import (
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectDir and userDir are the Dirs memTree roots its in-memory trees
// at. Nothing reads them from disk; they only prove every Definition.Path
// is the Tree's own Dir joined with the file's path inside FS.
var (
	projectDir = filepath.FromSlash("/repo")
	userDir    = filepath.FromSlash("/home/me")
)

// memTree returns a Tree over an in-memory FS holding files — each key a
// slash-separated path inside the tree, each value that file's contents —
// rooted at dir.
func memTree(dir string, files map[string]string) agentfile.Tree {
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}

	return agentfile.Tree{FS: fsys, Dir: dir}
}

// at returns the Path FindIn reports for rel, a slash-separated path inside
// a Tree rooted at dir.
func at(dir, rel string) string {
	return filepath.Join(dir, filepath.FromSlash(rel))
}

const developerAgent = "---\nname: developer\n---\n\nbody\n"

func Test_find_in_matches_frontmatter_name_anywhere_under_project_agents(t *testing.T) {
	project := memTree(projectDir, map[string]string{
		".claude/agents/developer/Agent.md": developerAgent,
	})

	defs := agentfile.FindIn(project, agentfile.Tree{}, "developer")

	require.Len(t, defs, 1)
	assert.Equal(t, agentfile.ScopeProject, defs[0].Scope)
	assert.Equal(t, at(projectDir, ".claude/agents/developer/Agent.md"), defs[0].Path)
	assert.Equal(t, "developer", defs[0].Frontmatter.Name)
}

func Test_find_in_ignores_a_filename_match_whose_name_differs(t *testing.T) {
	project := memTree(projectDir, map[string]string{
		".claude/agents/developer.md": "---\nname: someone-else\n---\n\nbody\n",
	})

	defs := agentfile.FindIn(project, agentfile.Tree{}, "developer")

	assert.Empty(t, defs, "the filename must play no part in the match")
}

func Test_find_in_skips_files_whose_frontmatter_does_not_parse(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "no frontmatter delimiter", body: "just a plain agent file\n", want: 0},
		{name: "unclosed frontmatter delimiter", body: "---\nname: developer\n\nno closing delimiter here\n", want: 0},
		{name: "yaml that fails to decode", body: "---\nname: [this is not: valid\n---\n\nbody\n", want: 0},
		{name: "valid frontmatter (control)", body: developerAgent, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := memTree(projectDir, map[string]string{".claude/agents/developer.md": tt.body})

			defs := agentfile.FindIn(project, agentfile.Tree{}, "developer")

			assert.Len(t, defs, tt.want)
		})
	}
}

func Test_find_in_prefers_project_definitions_over_user_level(t *testing.T) {
	user := memTree(userDir, map[string]string{".claude/agents/developer.md": developerAgent})

	t.Run("project defines it", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{".claude/agents/developer.md": developerAgent})

		defs := agentfile.FindIn(project, user, "developer")

		require.Len(t, defs, 1)
		assert.Equal(t, agentfile.ScopeProject, defs[0].Scope)
		assert.Equal(t, at(projectDir, ".claude/agents/developer.md"), defs[0].Path)
	})

	t.Run("project does not (control)", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{})

		defs := agentfile.FindIn(project, user, "developer")

		require.Len(t, defs, 1)
		assert.Equal(t, agentfile.ScopeUser, defs[0].Scope)
		assert.Equal(t, at(userDir, ".claude/agents/developer.md"), defs[0].Path)
	})
}

func Test_find_in_returns_every_project_duplicate_sorted_by_path(t *testing.T) {
	project := memTree(projectDir, map[string]string{
		".claude/agents/team/r.md":       "---\nname: my-reviewer\n---\n\nsecond\n",
		".claude/agents/my-reviewer.md": "---\nname: my-reviewer\n---\n\nfirst\n",
	})

	defs := agentfile.FindIn(project, agentfile.Tree{}, "my-reviewer")

	require.Len(t, defs, 2)
	assert.Equal(t, at(projectDir, ".claude/agents/my-reviewer.md"), defs[0].Path)
	assert.Equal(t, at(projectDir, ".claude/agents/team/r.md"), defs[1].Path)
}

// Test_find_in_decodes_skills_and_omit_claude_md pins Frontmatter's own
// Skills and OmitClaudeMd fields, and the loose-decode contract: a value in
// an unexpected shape falls back to that field's zero value and never drops
// the agent out of resolution.
func Test_find_in_decodes_skills_and_omit_claude_md(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		wantSkills       []string
		wantOmitClaudeMd bool
	}{
		{
			name:       "block-list skills",
			body:       "---\nname: developer\nskills:\n  - brief-workflow\n  - other-skill\n---\n\nbody\n",
			wantSkills: []string{"brief-workflow", "other-skill"},
		},
		{
			name:       "flow-list skills",
			body:       "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n",
			wantSkills: []string{"brief-workflow"},
		},
		{
			name:             "omitClaudeMd true",
			body:             "---\nname: developer\nomitClaudeMd: true\n---\n\nbody\n",
			wantOmitClaudeMd: true,
		},
		{name: "scalar skills", body: "---\nname: developer\nskills: brief-workflow\n---\n\nbody\n"},
		{name: "mapping skills", body: "---\nname: developer\nskills:\n  brief-workflow: true\n---\n\nbody\n"},
		{name: "non-bool omitClaudeMd", body: "---\nname: developer\nomitClaudeMd: yes please\n---\n\nbody\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := memTree(projectDir, map[string]string{".claude/agents/developer.md": tt.body})

			defs := agentfile.FindIn(project, agentfile.Tree{}, "developer")

			require.Len(t, defs, 1)
			assert.Equal(t, tt.wantSkills, defs[0].Frontmatter.Skills)
			assert.Equal(t, tt.wantOmitClaudeMd, defs[0].Frontmatter.OmitClaudeMd)
		})
	}
}

func Test_find_in_edge_cases(t *testing.T) {
	t.Run("a zero user Tree is never searched", func(t *testing.T) {
		defs := agentfile.FindIn(memTree(projectDir, map[string]string{}), agentfile.Tree{}, "developer")

		assert.Empty(t, defs)
	})

	t.Run("a zero project Tree falls through to the user tree", func(t *testing.T) {
		user := memTree(userDir, map[string]string{".claude/agents/developer.md": developerAgent})

		defs := agentfile.FindIn(agentfile.Tree{}, user, "developer")

		require.Len(t, defs, 1)
		assert.Equal(t, agentfile.ScopeUser, defs[0].Scope)
	})

	t.Run("a missing .claude/agents returns nothing", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{"README.md": developerAgent})

		defs := agentfile.FindIn(project, agentfile.Tree{}, "developer")

		assert.Empty(t, defs)
	})

	t.Run("a non-.md file with a matching name is ignored", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{".claude/agents/developer.txt": developerAgent})

		defs := agentfile.FindIn(project, agentfile.Tree{}, "developer")

		assert.Empty(t, defs)
	})

	t.Run("an upper-case .MD extension still matches", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{".claude/agents/developer.MD": developerAgent})

		defs := agentfile.FindIn(project, agentfile.Tree{}, "developer")

		assert.Len(t, defs, 1)
	})
}
