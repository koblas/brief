package agentfile_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// briefFile is a minimal frontmatter body ResolveBindingIn's own
// "brief:<agent>" case resolves against — the agent name plays no part in
// its own match, unlike FindIn's own "name:" match, so its value is
// unconstrained here.
const briefFile = "---\nname: implementer\n---\n\nbody\n"

// Test_resolve_binding_in_classifies_by_prefix pins ResolveBindingIn's own
// shape/state split (S06), consistent with FindIn: a "" value is unbound;
// a "brief:x" value resolves through the project's own override before its
// own plugin path, and is unresolved when neither is a regular file, or
// project itself carries no FS; any other "<plugin>:x" value is
// unverified, never touching the filesystem; a bare name resolves through
// FindIn, carrying its Path and Defs.
func Test_resolve_binding_in_classifies_by_prefix(t *testing.T) {
	t.Run("empty value is unbound", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{})
		user := memTree(userDir, map[string]string{})

		b := agentfile.ResolveBindingIn(project, user, "")

		assert.Equal(t, agentfile.BindingUnbound, b.State)
		assert.Empty(t, b.Path)
		assert.Empty(t, b.Defs)
	})

	t.Run("brief prefix prefers the project override over the plugin path", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{
			".claude/skills/brief/agents/implementer.md": "---\nname: implementer\n---\n\nplugin body\n",
			".claude/agents/implementer.md":               "---\nname: implementer\n---\n\noverride body\n",
		})
		user := memTree(userDir, map[string]string{})

		b := agentfile.ResolveBindingIn(project, user, "brief:implementer")

		require.Equal(t, agentfile.BindingBrief, b.Kind)
		require.Equal(t, agentfile.BindingResolved, b.State)
		assert.Equal(t, at(projectDir, ".claude/agents/implementer.md"), b.Path)

		// Control: with only the plugin path present, that one resolves.
		onlyPlugin := memTree(projectDir, map[string]string{
			".claude/skills/brief/agents/implementer.md": "---\nname: implementer\n---\n\nplugin body\n",
		})

		b = agentfile.ResolveBindingIn(onlyPlugin, user, "brief:implementer")
		require.Equal(t, agentfile.BindingResolved, b.State)
		assert.Equal(t, at(projectDir, ".claude/skills/brief/agents/implementer.md"), b.Path)
	})

	t.Run("brief prefix with neither file present is unresolved", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{})
		user := memTree(userDir, map[string]string{})

		b := agentfile.ResolveBindingIn(project, user, "brief:implementer")

		assert.Equal(t, agentfile.BindingBrief, b.Kind)
		assert.Equal(t, agentfile.BindingUnresolved, b.State)
	})

	t.Run("brief prefix with a zero project Tree is unresolved", func(t *testing.T) {
		user := memTree(userDir, map[string]string{})

		b := agentfile.ResolveBindingIn(agentfile.Tree{}, user, "brief:implementer")

		assert.Equal(t, agentfile.BindingBrief, b.Kind)
		assert.Equal(t, agentfile.BindingUnresolved, b.State)
	})

	t.Run("a directory at the override path is not regular; falls back to the plugin path", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{
			".claude/skills/brief/agents/implementer.md": "---\nname: implementer\n---\n\nplugin body\n",
		})
		project.FS.(fstest.MapFS)[".claude/agents/implementer.md"] = &fstest.MapFile{Mode: fs.ModeDir}

		b := agentfile.ResolveBindingIn(project, memTree(userDir, map[string]string{}), "brief:implementer")

		require.Equal(t, agentfile.BindingResolved, b.State)
		assert.Equal(t, at(projectDir, ".claude/skills/brief/agents/implementer.md"), b.Path)
	})

	t.Run("another plugin prefix is unverified", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{})
		user := memTree(userDir, map[string]string{})

		b := agentfile.ResolveBindingIn(project, user, "mine:implementer")

		assert.Equal(t, agentfile.BindingPlugin, b.Kind)
		assert.Equal(t, agentfile.BindingUnverified, b.State)
	})

	t.Run("bare name resolved carries Path and Defs", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{".claude/agents/developer.md": developerAgent})
		user := memTree(userDir, map[string]string{})

		b := agentfile.ResolveBindingIn(project, user, "developer")

		assert.Equal(t, agentfile.BindingBare, b.Kind)
		assert.Equal(t, agentfile.BindingResolved, b.State)
		assert.Equal(t, at(projectDir, ".claude/agents/developer.md"), b.Path)
		require.Len(t, b.Defs, 1)
		assert.Equal(t, at(projectDir, ".claude/agents/developer.md"), b.Defs[0].Path)
	})

	t.Run("bare name unresolved carries neither", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{})
		user := memTree(userDir, map[string]string{})

		b := agentfile.ResolveBindingIn(project, user, "developer")

		assert.Equal(t, agentfile.BindingBare, b.Kind)
		assert.Equal(t, agentfile.BindingUnresolved, b.State)
		assert.Empty(t, b.Path)
		assert.Empty(t, b.Defs)
	})

	// This subtest re-confirms, at ResolveBindingIn's own level, the
	// project-over-user precedence FindIn already owns
	// (find_test.go's own Test_find_in_prefers_project_definitions_over_user_level):
	// a bare name found in both trees resolves to the project one.
	// Mutation-verified together with that test: swapping FindIn's own
	// search order (project first, then user) reddens both this subtest
	// and the FindIn one on the same change, since ResolveBindingIn holds
	// no separate precedence logic of its own — it delegates to FindIn
	// directly.
	t.Run("bare name project shadows a same-named user definition", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{".claude/agents/developer.md": developerAgent})
		user := memTree(userDir, map[string]string{".claude/agents/developer.md": developerAgent})

		b := agentfile.ResolveBindingIn(project, user, "developer")

		require.Len(t, b.Defs, 1)
		assert.Equal(t, agentfile.ScopeProject, b.Defs[0].Scope)
	})
}

// Test_lacking_skill_returns_each_definition_without_it pins
// (Binding).LackingSkill's own single decision point (S06): a bare-name
// binding filters b.Defs to the ones missing the skill; a "brief:*"
// binding, resolved in memory, decodes its own resolved file through the
// Tree ResolveBindingIn found it in rather than the filesystem; an
// unbound, unresolved or unverified binding always returns nil.
func Test_lacking_skill_returns_each_definition_without_it(t *testing.T) {
	t.Run("bare name duplicates: only the one lacking the skill is returned", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{
			".claude/agents/a.md": "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n",
			".claude/agents/b.md": "---\nname: developer\n---\n\nbody\n",
		})
		user := memTree(userDir, map[string]string{})

		b := agentfile.ResolveBindingIn(project, user, "developer")
		defs := b.LackingSkill("brief-workflow")

		require.Len(t, defs, 1)
		assert.Equal(t, at(projectDir, ".claude/agents/b.md"), defs[0].Path)
	})

	t.Run("brief prefix file without the skill returns one project-scope Definition", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{".claude/agents/implementer.md": briefFile})
		user := memTree(userDir, map[string]string{})

		b := agentfile.ResolveBindingIn(project, user, "brief:implementer")
		defs := b.LackingSkill("brief-workflow")

		require.Len(t, defs, 1)
		assert.Equal(t, at(projectDir, ".claude/agents/implementer.md"), defs[0].Path)
		assert.Equal(t, agentfile.ScopeProject, defs[0].Scope)
	})

	// Control for the case above: the same resolved file, this time
	// already carrying the skill, must report no shortfall — proving
	// LackingSkill actually reads the resolved file's own content rather
	// than reporting "lacking" unconditionally for every BindingBrief.
	t.Run("brief prefix file already carrying the skill returns nil (control)", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{
			".claude/agents/implementer.md": "---\nname: implementer\nskills: [brief-workflow]\n---\n\nbody\n",
		})
		user := memTree(userDir, map[string]string{})

		b := agentfile.ResolveBindingIn(project, user, "brief:implementer")
		defs := b.LackingSkill("brief-workflow")

		assert.Nil(t, defs)
	})

	t.Run("brief prefix path with no frontmatter returns one Definition with zero Frontmatter", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{".claude/agents/implementer.md": "no frontmatter here\n"})
		user := memTree(userDir, map[string]string{})

		b := agentfile.ResolveBindingIn(project, user, "brief:implementer")
		defs := b.LackingSkill("brief-workflow")

		require.Len(t, defs, 1)
		assert.Equal(t, at(projectDir, ".claude/agents/implementer.md"), defs[0].Path)
		assert.Equal(t, agentfile.Frontmatter{}, defs[0].Frontmatter)
	})

	t.Run("unbound, unresolved or unverified is nil", func(t *testing.T) {
		project := memTree(projectDir, map[string]string{})
		user := memTree(userDir, map[string]string{})

		cases := []string{"", "developer", "mine:developer"}
		for _, value := range cases {
			b := agentfile.ResolveBindingIn(project, user, value)
			assert.Nil(t, b.LackingSkill("brief-workflow"))
		}
	})
}
