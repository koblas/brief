package agentfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
