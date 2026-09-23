package setup_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newServerWithHome returns a Server whose own home directory is pinned to
// home — every test in this file binds a bare-name role, so leaving the
// default os.UserHomeDir in place would make the result depend on the
// developer's own "~/.claude/agents".
func newServerWithHome(t *testing.T, home string) *setup.Server {
	t.Helper()

	return setup.NewServer(setup.WithHomeDir(func() (string, error) { return home, nil }))
}

// writeMissingSkillAgent writes body at path, creating every parent
// directory it needs.
func writeMissingSkillAgent(t *testing.T, path, body string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

// writeConfigWithRoles writes a minimal, valid ".brief.yaml" at wd binding
// roles — one line per non-empty field, in RoleBindings' own order — the
// same "edited but valid" shape Test_init_with_agents_never_edits_an_existing_config
// already relies on to keep planConfig's own ActionKept branch.
func writeConfigWithRoles(t *testing.T, wd string, planner, implementer, reviewer string) {
	t.Helper()

	body := "feature-directory: docs/specifications\nroles:\n"
	if planner != "" {
		body += "  planner: " + planner + "\n"
	}

	if implementer != "" {
		body += "  implementer: " + implementer + "\n"
	}

	if reviewer != "" {
		body += "  reviewer: " + reviewer + "\n"
	}

	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(body), 0o600))
}

// Test_init_reports_bound_agents_missing_the_workflow_skill pins
// Result.AgentsMissingSkill (S06): the report covers bare-name planner and
// implementer bindings only, one item per lacking agentfile.Definition,
// never touching the file itself.
func Test_init_reports_bound_agents_missing_the_workflow_skill(t *testing.T) {
	t.Run("bare project agent lacking the skill is listed and left untouched", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "", "developer", "")

		path := filepath.Join(wd, ".claude", "agents", "developer.md")
		body := "---\nname: developer\n---\n\nbody\n"
		writeMissingSkillAgent(t, path, body)

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		require.Len(t, res.AgentsMissingSkill, 1)
		assert.Equal(t, "implementer", res.AgentsMissingSkill[0].Role)
		assert.Equal(t, "developer", res.AgentsMissingSkill[0].Agent)
		assert.Equal(t, path, res.AgentsMissingSkill[0].Path)
		assert.Equal(t, agentfile.ScopeProject, res.AgentsMissingSkill[0].Scope)
		assert.False(t, res.AgentsMissingSkill[0].Escaped, "a plain project agent under root does not escape it")

		after, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, body, string(after))
	})

	t.Run("control: the identical fixture with EditAgents changes the file", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "", "developer", "")

		path := filepath.Join(wd, ".claude", "agents", "developer.md")
		body := "---\nname: developer\n---\n\nbody\n"
		writeMissingSkillAgent(t, path, body)

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
		require.NoError(t, err)

		assert.Empty(t, res.AgentsMissingSkill)

		after, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.NotEqual(t, body, string(after), `EditAgents must actually change the file — proving the base test's own "left untouched" claim is falsifiable`)
	})

	t.Run("control: the same agent already carrying the skill is not listed", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "", "developer", "")

		path := filepath.Join(wd, ".claude", "agents", "developer.md")
		writeMissingSkillAgent(t, path, "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n")

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		assert.Empty(t, res.AgentsMissingSkill)
	})

	t.Run("user-scope-only agent is listed with a home-relative ScopeRelPath", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "", "developer", "")

		path := filepath.Join(home, ".claude", "agents", "developer.md")
		writeMissingSkillAgent(t, path, "---\nname: developer\n---\n\nbody\n")

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		require.Len(t, res.AgentsMissingSkill, 1)
		assert.Equal(t, agentfile.ScopeUser, res.AgentsMissingSkill[0].Scope)
		assert.Equal(t, path, res.AgentsMissingSkill[0].Path)
		assert.Equal(t, filepath.Join(".claude", "agents", "developer.md"), filepath.FromSlash(res.AgentsMissingSkill[0].ScopeRelPath))
	})

	t.Run("a brief override lacking the skill is not listed; a bare binding to identical content is", func(t *testing.T) {
		body := "---\nname: implementer\n---\n\nbody\n"

		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "", "brief:implementer", "")
		writeMissingSkillAgent(t, filepath.Join(wd, ".claude", "agents", "implementer.md"), body)

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)
		assert.Empty(t, res.AgentsMissingSkill, "a brief:* override is never in init's own report")

		bareWd := t.TempDir()
		bareHome := t.TempDir()
		writeConfigWithRoles(t, bareWd, "", "implementer", "")
		writeMissingSkillAgent(t, filepath.Join(bareWd, ".claude", "agents", "implementer.md"), body)

		bareSrv := newServerWithHome(t, bareHome)
		bareRes, bareErr := bareSrv.Init(t.Context(), bareWd, setup.InitRequest{Host: setup.HostClaudeCode})
		require.NoError(t, bareErr)
		require.Len(t, bareRes.AgentsMissingSkill, 1, "a bare binding to the identical content must still be listed")
	})

	t.Run("unbound, unresolved and other-plugin bindings are not listed", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "mine:planner", "ghost", "")

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		assert.Empty(t, res.AgentsMissingSkill)
	})

	t.Run("reviewer bound to a lacking agent is not listed", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "", "", "developer")

		path := filepath.Join(wd, ".claude", "agents", "developer.md")
		writeMissingSkillAgent(t, path, "---\nname: developer\n---\n\nbody\n")

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		assert.Empty(t, res.AgentsMissingSkill)
	})

	t.Run("two project duplicates for one name are each listed in lexical order", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "", "my-reviewer", "")

		first := filepath.Join(wd, ".claude", "agents", "a.md")
		second := filepath.Join(wd, ".claude", "agents", "b.md")
		writeMissingSkillAgent(t, first, "---\nname: my-reviewer\n---\n\nfirst\n")
		writeMissingSkillAgent(t, second, "---\nname: my-reviewer\n---\n\nsecond\n")

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		require.Len(t, res.AgentsMissingSkill, 2)
		assert.Equal(t, first, res.AgentsMissingSkill[0].Path)
		assert.Equal(t, second, res.AgentsMissingSkill[1].Path)
	})

	t.Run("planner and implementer both bound to the same lacking agent produce two items", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "developer", "developer", "")

		path := filepath.Join(wd, ".claude", "agents", "developer.md")
		writeMissingSkillAgent(t, path, "---\nname: developer\n---\n\nbody\n")

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		require.Len(t, res.AgentsMissingSkill, 2)
		assert.Equal(t, "planner", res.AgentsMissingSkill[0].Role)
		assert.Equal(t, "implementer", res.AgentsMissingSkill[1].Role)
		assert.Equal(t, path, res.AgentsMissingSkill[0].Path)
		assert.Equal(t, path, res.AgentsMissingSkill[1].Path)
	})

	t.Run("DryRun still lists and writes nothing", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "", "developer", "")

		path := filepath.Join(wd, ".claude", "agents", "developer.md")
		body := "---\nname: developer\n---\n\nbody\n"
		writeMissingSkillAgent(t, path, body)

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, DryRun: true})
		require.NoError(t, err)

		require.Len(t, res.AgentsMissingSkill, 1)
		assert.Empty(t, res.Created)
		assert.Empty(t, res.Modified)

		after, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, body, string(after))
	})

	t.Run("Host none with a lacking bound agent is empty, non-nil", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "", "developer", "")

		writeMissingSkillAgent(t, filepath.Join(wd, ".claude", "agents", "developer.md"), "---\nname: developer\n---\n\nbody\n")

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})
		require.NoError(t, err)

		assert.NotNil(t, res.AgentsMissingSkill)
		assert.Empty(t, res.AgentsMissingSkill)
	})

	t.Run("Force rewriting an invalid config that bound a lacking agent reports empty", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("handoff-cap-lines: 0\n"), 0o600))

		writeMissingSkillAgent(t, filepath.Join(wd, ".claude", "agents", "developer.md"), "---\nname: developer\n---\n\nbody\n")

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, Force: true})
		require.NoError(t, err)

		assert.NotNil(t, res.AgentsMissingSkill)
		assert.Empty(t, res.AgentsMissingSkill)
	})
}
