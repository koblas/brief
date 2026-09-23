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

// Test_find_decodes_skills_and_omit_claude_md pins Frontmatter's own
// Skills and OmitClaudeMd fields (S05): a block-list "skills:", a
// one-line flow-list "skills: [brief-workflow]" and "omitClaudeMd: true"
// all surface on the returned Definition's own Frontmatter.
func Test_find_decodes_skills_and_omit_claude_md(t *testing.T) {
	cases := []struct {
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
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			home := t.TempDir()
			path := filepath.Join(root, ".claude", "agents", "developer.md")
			writeAgentFile(t, path, c.body)

			defs := agentfile.Find(root, home, "developer")

			require.Len(t, defs, 1)
			assert.Equal(t, c.wantSkills, defs[0].Frontmatter.Skills)
			assert.Equal(t, c.wantOmitClaudeMd, defs[0].Frontmatter.OmitClaudeMd)
		})
	}
}

// Test_find_still_resolves_an_agent_whose_skills_or_omit_claude_md_is_malformed
// pins the loose-decode contract (S05): a "skills:" or "omitClaudeMd:"
// value in an unexpected shape must never drop the agent out of Rule 5
// resolution — only Skills/OmitClaudeMd themselves fall back to their own
// zero values. Green on arrival: findIn already ignores unknown-shaped
// values for a field it does not yet decode at all; this pins the loose
// decode as a guard against a stricter one being introduced later, not a
// behavior change of its own.
func Test_find_still_resolves_an_agent_whose_skills_or_omit_claude_md_is_malformed(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "scalar skills", body: "---\nname: developer\nskills: brief-workflow\n---\n\nbody\n"},
		{name: "mapping skills", body: "---\nname: developer\nskills:\n  brief-workflow: true\n---\n\nbody\n"},
		{name: "non-bool omitClaudeMd", body: "---\nname: developer\nomitClaudeMd: yes please\n---\n\nbody\n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			home := t.TempDir()
			path := filepath.Join(root, ".claude", "agents", "developer.md")
			writeAgentFile(t, path, c.body)

			defs := agentfile.Find(root, home, "developer")

			require.Len(t, defs, 1, "a malformed skills/omitClaudeMd value must not drop the agent out of resolution")
			assert.Nil(t, defs[0].Frontmatter.Skills)
			assert.False(t, defs[0].Frontmatter.OmitClaudeMd)
		})
	}
}

// Test_parse_decodes_frontmatter_from_bytes pins Parse's own contract
// (S07): the bytes-level twin of Load, for a caller (setup's own bound-agent
// edit path) that already holds an agent file's own content in memory
// rather than a path — it decodes the same Frontmatter, loose Skills
// included, that Load returns for the identical bytes on disk, and errors
// the same way Load does on a missing closing delimiter.
func Test_parse_decodes_frontmatter_from_bytes(t *testing.T) {
	body := "---\nname: planner\nskills: [\"brief-workflow\"]\n---\n\nbody\n"

	dir := t.TempDir()
	path := filepath.Join(dir, "agent.md")
	writeAgentFile(t, path, body)

	want, loadErr := agentfile.Load(path)
	require.NoError(t, loadErr)

	got, err := agentfile.Parse([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, want, got)

	t.Run("errors on missing closing delimiter", func(t *testing.T) {
		_, err := agentfile.Parse([]byte("---\nname: planner\n\nno closing delimiter here\n"))
		require.Error(t, err)
	})
}

// Test_load_decodes_one_agent_files_frontmatter pins Load's own contract
// (S05): the single-file entry point a "brief:*" role binding's own
// resolved file uses, sharing findIn's decode.
func Test_load_decodes_one_agent_files_frontmatter(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantErr   bool
		wantName  string
		wantSkill []string
	}{
		{
			name:      "block-list skills",
			body:      "---\nname: planner\nskills:\n  - brief-workflow\n---\n\nbody\n",
			wantName:  "planner",
			wantSkill: []string{"brief-workflow"},
		},
		{
			name:     "scalar skills decodes loosely, no error",
			body:     "---\nname: planner\nskills: brief-workflow\n---\n\nbody\n",
			wantName: "planner",
		},
		{
			name:    "no frontmatter",
			body:    "just a plain agent file\n",
			wantErr: true,
		},
		{
			name:    "unparseable yaml",
			body:    "---\nname: [this is not: valid\n---\n\nbody\n",
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "agent.md")
			require.NoError(t, os.WriteFile(path, []byte(c.body), 0o600))

			fm, err := agentfile.Load(path)

			if c.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, c.wantName, fm.Name)
			assert.Equal(t, c.wantSkill, fm.Skills)
		})
	}

	t.Run("missing file", func(t *testing.T) {
		_, err := agentfile.Load(filepath.Join(t.TempDir(), "missing.md"))
		require.Error(t, err)
	})
}

// Test_resolve_binding_classifies_by_prefix pins ResolveBinding's own
// shape/state split (S06): a "" value is unbound; a "brief:x" value
// resolves through the repository override before the plugin path, and is
// unresolved when neither is a regular file; any other "<plugin>:x" value
// is unverified, never touching the filesystem; a bare name resolves
// through Find, carrying its Path and Defs.
func Test_resolve_binding_classifies_by_prefix(t *testing.T) {
	t.Run("empty value is unbound", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()

		b := agentfile.ResolveBinding(root, home, "")

		assert.Equal(t, agentfile.BindingUnbound, b.State)
		assert.Empty(t, b.Path)
		assert.Empty(t, b.Defs)
	})

	t.Run("brief prefix prefers the repository override over the plugin path", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()

		pluginPath := filepath.Join(root, ".claude", "skills", "brief", "agents", "implementer.md")
		overridePath := filepath.Join(root, ".claude", "agents", "implementer.md")
		writeAgentFile(t, pluginPath, "---\nname: implementer\n---\n\nplugin body\n")
		writeAgentFile(t, overridePath, "---\nname: implementer\n---\n\noverride body\n")

		b := agentfile.ResolveBinding(root, home, "brief:implementer")

		require.Equal(t, agentfile.BindingBrief, b.Kind)
		require.Equal(t, agentfile.BindingResolved, b.State)
		assert.Equal(t, overridePath, b.Path)

		// Control: with only the plugin path present, that one resolves.
		require.NoError(t, os.Remove(overridePath))

		b = agentfile.ResolveBinding(root, home, "brief:implementer")
		require.Equal(t, agentfile.BindingResolved, b.State)
		assert.Equal(t, pluginPath, b.Path)
	})

	t.Run("brief prefix with neither file present is unresolved", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()

		b := agentfile.ResolveBinding(root, home, "brief:implementer")

		assert.Equal(t, agentfile.BindingBrief, b.Kind)
		assert.Equal(t, agentfile.BindingUnresolved, b.State)
	})

	t.Run("a directory at the override path is not regular; falls back to the plugin path", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()

		overridePath := filepath.Join(root, ".claude", "agents", "implementer.md")
		require.NoError(t, os.MkdirAll(overridePath, 0o755))

		pluginPath := filepath.Join(root, ".claude", "skills", "brief", "agents", "implementer.md")
		writeAgentFile(t, pluginPath, "---\nname: implementer\n---\n\nplugin body\n")

		b := agentfile.ResolveBinding(root, home, "brief:implementer")

		require.Equal(t, agentfile.BindingResolved, b.State)
		assert.Equal(t, pluginPath, b.Path)
	})

	t.Run("another plugin prefix is unverified", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()

		b := agentfile.ResolveBinding(root, home, "mine:implementer")

		assert.Equal(t, agentfile.BindingPlugin, b.Kind)
		assert.Equal(t, agentfile.BindingUnverified, b.State)
	})

	t.Run("bare name resolved carries Path and Defs", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()
		path := filepath.Join(root, ".claude", "agents", "developer.md")
		writeAgentFile(t, path, "---\nname: developer\n---\n\nbody\n")

		b := agentfile.ResolveBinding(root, home, "developer")

		assert.Equal(t, agentfile.BindingBare, b.Kind)
		assert.Equal(t, agentfile.BindingResolved, b.State)
		assert.Equal(t, path, b.Path)
		require.Len(t, b.Defs, 1)
		assert.Equal(t, path, b.Defs[0].Path)
	})

	t.Run("bare name unresolved carries neither", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()

		b := agentfile.ResolveBinding(root, home, "developer")

		assert.Equal(t, agentfile.BindingBare, b.Kind)
		assert.Equal(t, agentfile.BindingUnresolved, b.State)
		assert.Empty(t, b.Path)
		assert.Empty(t, b.Defs)
	})
}

// Test_lacking_skill_returns_each_definition_without_it pins
// (Binding).LackingSkill's own single decision point (S06): a bare-name
// binding filters b.Defs to the ones missing the skill; a "brief:*"
// binding synthesizes one Definition via Load; an unbound, unresolved or
// unverified binding always returns nil.
func Test_lacking_skill_returns_each_definition_without_it(t *testing.T) {
	t.Run("bare name duplicates: only the one lacking the skill is returned", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()

		first := filepath.Join(root, ".claude", "agents", "a.md")
		second := filepath.Join(root, ".claude", "agents", "b.md")
		writeAgentFile(t, first, "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n")
		writeAgentFile(t, second, "---\nname: developer\n---\n\nbody\n")

		b := agentfile.ResolveBinding(root, home, "developer")
		defs := b.LackingSkill("brief-workflow")

		require.Len(t, defs, 1)
		assert.Equal(t, second, defs[0].Path)
	})

	t.Run("brief prefix file without the skill returns one project-scope Definition", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()
		path := filepath.Join(root, ".claude", "agents", "implementer.md")
		writeAgentFile(t, path, "---\nname: implementer\n---\n\nbody\n")

		b := agentfile.ResolveBinding(root, home, "brief:implementer")
		defs := b.LackingSkill("brief-workflow")

		require.Len(t, defs, 1)
		assert.Equal(t, path, defs[0].Path)
		assert.Equal(t, agentfile.ScopeProject, defs[0].Scope)
	})

	t.Run("brief prefix path with no frontmatter returns one Definition with zero Frontmatter", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()
		path := filepath.Join(root, ".claude", "agents", "implementer.md")
		writeAgentFile(t, path, "no frontmatter here\n")

		b := agentfile.ResolveBinding(root, home, "brief:implementer")
		defs := b.LackingSkill("brief-workflow")

		require.Len(t, defs, 1)
		assert.Equal(t, path, defs[0].Path)
		assert.Equal(t, agentfile.Frontmatter{}, defs[0].Frontmatter)
	})

	t.Run("unbound, unresolved or unverified is nil", func(t *testing.T) {
		root := t.TempDir()
		home := t.TempDir()

		cases := []string{"", "developer", "mine:developer"}
		for _, value := range cases {
			b := agentfile.ResolveBinding(root, home, value)
			assert.Nil(t, b.LackingSkill("brief-workflow"))
		}
	})
}

func Test_find_edge_cases(t *testing.T) {
	t.Run("an empty home skips the user scope", func(t *testing.T) {
		root := t.TempDir()

		// A relative ".claude/agents" resolves against the process's own
		// working directory when home is joined unguarded — seeding that
		// directory with a real match proves the empty-home guard, not
		// merely the absence of anything to find.
		cwd := t.TempDir()
		t.Chdir(cwd)
		writeAgentFile(t, filepath.Join(cwd, ".claude", "agents", "developer.md"), "---\nname: developer\n---\n\nbody\n")

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
