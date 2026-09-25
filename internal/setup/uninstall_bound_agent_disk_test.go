package setup_test

// Uninstall's bound-agent rows always read and write through
// bound_agent.go's real os.Lstat/os.ReadFile. Every test here starts from a
// real claude-code install via installClaudeCode, then writes one
// bound-agent fixture and calls Uninstall directly. findBoundAgentRow,
// assertNoBoundAgentRow, writeConfigWithRoles, writeMissingSkillAgent and
// newServerWithHome are shared helpers from sibling files in this package.

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/host"
	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installClaudeCode writes a config binding planner/implementer/reviewer,
// then runs a real claude-code Init against it. withAgents also installs
// the three role-agent files, needed only by the row-order test.
func installClaudeCode(t *testing.T, wd, home, planner, implementer, reviewer string, withAgents bool) *setup.Server {
	t.Helper()

	writeConfigWithRoles(t, wd, planner, implementer, reviewer)

	srv := newServerWithHome(t, home)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: withAgents})
	require.NoError(t, err)

	return srv
}

func Test_uninstall_removes_brief_workflow_from_bound_project_agents(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantBody string
	}{
		{
			name:     "no key: the merged single-item flow list is dropped entirely",
			body:     "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n",
			wantBody: "---\nname: developer\n---\n\nbody\n",
		},
		{
			name:     "block list: the item is removed, the key stays",
			body:     "---\nname: developer\nskills:\n  - other\n  - brief-workflow\n---\n\nbody\n",
			wantBody: "---\nname: developer\nskills:\n  - other\n---\n\nbody\n",
		},
		{
			name:     "flow list: the token is removed, the key stays",
			body:     "---\nname: developer\nskills: [other, brief-workflow]\n---\n\nbody\n",
			wantBody: "---\nname: developer\nskills: [other]\n---\n\nbody\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := t.TempDir()
			home := t.TempDir()
			srv := installClaudeCode(t, wd, home, "", "developer", "", false)

			path := filepath.Join(wd, ".claude", "agents", "developer.md")
			writeMissingSkillAgent(t, path, c.body)
			require.NoError(t, os.Chmod(path, 0o640))

			res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
			require.NoError(t, err)

			row := findBoundAgentRow(t, res, path)
			assert.Equal(t, setup.ActionRemoved, row.Action)
			assert.Equal(t, "brief-workflow from skills", row.Detail)

			after, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, c.wantBody, string(after))

			info, statErr := os.Stat(path)
			require.NoError(t, statErr)
			assert.Equal(t, fs.FileMode(0o640), info.Mode().Perm())

			assert.Contains(t, res.Modified, path)
			assert.NotContains(t, res.Removed, path)
		})
	}
}

func Test_uninstall_orders_bound_agent_rows_after_the_snippet(t *testing.T) {
	wd := t.TempDir()
	home := t.TempDir()
	srv := installClaudeCode(t, wd, home, "planner-agent", "implementer-agent", "", true)

	plannerPath := filepath.Join(wd, ".claude", "agents", "planner-agent.md")
	implementerPath := filepath.Join(wd, ".claude", "agents", "implementer-agent.md")
	writeMissingSkillAgent(t, plannerPath, "---\nname: planner-agent\nskills: [brief-workflow]\n---\n\nbody\n")
	writeMissingSkillAgent(t, implementerPath, "---\nname: implementer-agent\nskills: [brief-workflow]\n---\n\nbody\n")

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	var (
		snippetIdx     = -1
		implementerIdx = -1
		plannerIdx     = -1
		agentIdx       = -1
	)

	for i, a := range res.Artifacts {
		switch {
		case a.Kind == setup.KindSnippet && snippetIdx == -1:
			snippetIdx = i
		case a.Kind == setup.KindBoundAgent && a.Path == implementerPath:
			implementerIdx = i
		case a.Kind == setup.KindBoundAgent && a.Path == plannerPath:
			plannerIdx = i
		case a.Kind == setup.KindAgent && agentIdx == -1:
			agentIdx = i
		}
	}

	require.NotEqual(t, -1, snippetIdx)
	require.NotEqual(t, -1, implementerIdx)
	require.NotEqual(t, -1, plannerIdx)
	require.NotEqual(t, -1, agentIdx)

	assert.Less(t, snippetIdx, implementerIdx)
	assert.Less(t, implementerIdx, plannerIdx)
	assert.Less(t, plannerIdx, agentIdx)
}

func Test_uninstall_keeps_bound_agent_entries_while_the_skill_is_kept(t *testing.T) {
	const body = "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n"
	const unedited = "---\nname: developer\n---\n\nbody\n"

	newFixture := func(t *testing.T) (wd string, srv *setup.Server, skillPath, agentPath string) {
		t.Helper()

		wd = t.TempDir()
		home := t.TempDir()
		srv = installClaudeCode(t, wd, home, "", "developer", "", false)

		skillPath = filepath.Join(wd, host.WorkflowSkillDir, "SKILL.md")
		agentPath = filepath.Join(wd, ".claude", "agents", "developer.md")
		writeMissingSkillAgent(t, agentPath, body)

		return wd, srv, skillPath, agentPath
	}

	t.Run("SKILL.md edited without force: entries kept, no row", func(t *testing.T) {
		wd, srv, skillPath, agentPath := newFixture(t)
		require.NoError(t, os.WriteFile(skillPath, []byte("---\nedited: true\n---\n"), 0o600))

		res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		assertNoBoundAgentRow(t, res, agentPath)

		after, readErr := os.ReadFile(agentPath)
		require.NoError(t, readErr)
		assert.Equal(t, body, string(after))
	})

	t.Run("SKILL.md replaced by a directory under --force: entries kept, no row", func(t *testing.T) {
		wd, srv, skillPath, agentPath := newFixture(t)
		require.NoError(t, os.Remove(skillPath))
		require.NoError(t, os.Mkdir(skillPath, 0o755))

		res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode, Force: true})
		require.NoError(t, err)

		assertNoBoundAgentRow(t, res, agentPath)

		after, readErr := os.ReadFile(agentPath)
		require.NoError(t, readErr)
		assert.Equal(t, body, string(after))
	})

	t.Run("control: --force with a still-regular, edited SKILL.md opens the gate", func(t *testing.T) {
		wd, srv, skillPath, agentPath := newFixture(t)
		require.NoError(t, os.WriteFile(skillPath, []byte("---\nedited: true\n---\n"), 0o600))

		res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode, Force: true})
		require.NoError(t, err)

		row := findBoundAgentRow(t, res, agentPath)
		assert.Equal(t, setup.ActionRemoved, row.Action)

		after, readErr := os.ReadFile(agentPath)
		require.NoError(t, readErr)
		assert.Equal(t, unedited, string(after))
	})

	t.Run("control: no SKILL.md at all opens the gate", func(t *testing.T) {
		wd, srv, skillPath, agentPath := newFixture(t)
		require.NoError(t, os.Remove(skillPath))

		res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		row := findBoundAgentRow(t, res, agentPath)
		assert.Equal(t, setup.ActionRemoved, row.Action)

		after, readErr := os.ReadFile(agentPath)
		require.NoError(t, readErr)
		assert.Equal(t, unedited, string(after))
	})
}

// Every subtest but the baseline changes exactly one variable from it and
// asserts no bound-agent row and byte-identical agent bytes.
func Test_uninstall_leaves_non_targets_alone(t *testing.T) {
	const baselineBody = "---\nname: developer\nskills: [other, brief-workflow]\n---\n\nbody\n"

	t.Run("baseline: bare-name project agent in a flow list is removed (control)", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		srv := installClaudeCode(t, wd, home, "", "developer", "", false)

		path := filepath.Join(wd, ".claude", "agents", "developer.md")
		writeMissingSkillAgent(t, path, baselineBody)

		res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		row := findBoundAgentRow(t, res, path)
		assert.Equal(t, setup.ActionRemoved, row.Action)
	})

	t.Run("brief:planner binding gets no row", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		srv := installClaudeCode(t, wd, home, "", "brief:planner", "", false)

		path := filepath.Join(wd, ".claude", "agents", "planner.md")
		writeMissingSkillAgent(t, path, "---\nname: planner\nskills: [other, brief-workflow]\n---\n\nbody\n")

		res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		assertNoBoundAgentRow(t, res, path)

		after, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, "---\nname: planner\nskills: [other, brief-workflow]\n---\n\nbody\n", string(after))
	})

	t.Run("agent defined only under home (nested inside root) gets no row", func(t *testing.T) {
		wd := t.TempDir()
		home := filepath.Join(wd, "home")
		srv := installClaudeCode(t, wd, home, "", "developer", "", false)

		path := filepath.Join(home, ".claude", "agents", "developer.md")
		writeMissingSkillAgent(t, path, baselineBody)

		res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		assertNoBoundAgentRow(t, res, path)

		after, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, baselineBody, string(after))
	})

	t.Run("a symlinked leaf gets no row", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		outside := t.TempDir()
		srv := installClaudeCode(t, wd, home, "", "developer", "", false)

		targetPath := filepath.Join(outside, "real.md")
		require.NoError(t, os.WriteFile(targetPath, []byte(baselineBody), 0o600))

		linkPath := filepath.Join(wd, ".claude", "agents", "developer.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(linkPath), 0o755))
		require.NoError(t, os.Symlink(targetPath, linkPath))

		res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		assertNoBoundAgentRow(t, res, linkPath)

		targetAfter, readErr := os.ReadFile(targetPath)
		require.NoError(t, readErr)
		assert.Equal(t, baselineBody, string(targetAfter))
	})

	t.Run("a symlink to a regular file inside root gets no row (isolates the leaf-mode check from the escape check)", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		srv := installClaudeCode(t, wd, home, "", "developer", "", false)

		targetPath := filepath.Join(wd, "real.md")
		require.NoError(t, os.WriteFile(targetPath, []byte(baselineBody), 0o600))

		linkPath := filepath.Join(wd, ".claude", "agents", "developer.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(linkPath), 0o755))
		require.NoError(t, os.Symlink(targetPath, linkPath))

		res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		assertNoBoundAgentRow(t, res, linkPath)

		targetAfter, readErr := os.ReadFile(targetPath)
		require.NoError(t, readErr)
		assert.Equal(t, baselineBody, string(targetAfter))
	})

	t.Run("a .claude symlinked outside the repo gets no row (dry run)", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		outsideClaude := t.TempDir()
		srv := installClaudeCode(t, wd, home, "", "developer", "", false)

		agentPath := filepath.Join(outsideClaude, "agents", "developer.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(agentPath), 0o755))
		require.NoError(t, os.WriteFile(agentPath, []byte(baselineBody), 0o600))

		require.NoError(t, os.RemoveAll(filepath.Join(wd, ".claude")))
		require.NoError(t, os.Symlink(outsideClaude, filepath.Join(wd, ".claude")))

		res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode, DryRun: true})
		require.NoError(t, err)

		assertNoBoundAgentRow(t, res, filepath.Join(wd, ".claude", "agents", "developer.md"))

		after, readErr := os.ReadFile(agentPath)
		require.NoError(t, readErr)
		assert.Equal(t, baselineBody, string(after))
	})

	t.Run("a quoted skill entry gets no row", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		srv := installClaudeCode(t, wd, home, "", "developer", "", false)

		path := filepath.Join(wd, ".claude", "agents", "developer.md")
		body := "---\nname: developer\nskills:\n  - \"brief-workflow\"\n---\n\nbody\n"
		writeMissingSkillAgent(t, path, body)

		res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		assertNoBoundAgentRow(t, res, path)

		after, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, body, string(after))
	})

	t.Run("skill not listed gets no row", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		srv := installClaudeCode(t, wd, home, "", "developer", "", false)

		path := filepath.Join(wd, ".claude", "agents", "developer.md")
		body := "---\nname: developer\nskills: [other]\n---\n\nbody\n"
		writeMissingSkillAgent(t, path, body)

		res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		assertNoBoundAgentRow(t, res, path)

		after, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, body, string(after))
	})

	t.Run("invalid .brief.yaml gets no bound rows and no refusal", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		srv := installClaudeCode(t, wd, home, "", "developer", "", false)

		path := filepath.Join(wd, ".claude", "agents", "developer.md")
		writeMissingSkillAgent(t, path, baselineBody)

		configPath := filepath.Join(wd, ".brief.yaml")
		require.NoError(t, os.WriteFile(configPath, []byte("feature-directory: [unterminated\n"), 0o600))

		res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		assertNoBoundAgentRow(t, res, path)

		after, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, baselineBody, string(after))
	})

	t.Run("--host none plans no bound rows", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		srv := installClaudeCode(t, wd, home, "", "developer", "", false)

		path := filepath.Join(wd, ".claude", "agents", "developer.md")
		writeMissingSkillAgent(t, path, baselineBody)

		res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})
		require.NoError(t, err)

		assertNoBoundAgentRow(t, res, path)

		after, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, baselineBody, string(after))
	})
}

// removeWorkflowSkill is a surgical text scan; each case here is a shape it
// misjudges, caught by re-decoding the edit and confirming it afterward.
func Test_uninstall_leaves_bound_agent_untouched_when_the_removal_cannot_be_verified(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "quoted flow item holding a comma: the naive comma-split cuts inside it",
			body: "---\nname: developer\nskills: [\"x, brief-workflow, y\", 'brief-workflow']\n---\n\nbody\n",
		},
		{
			name: "duplicate flow entries: only the first is removed, the skill is still listed",
			body: "---\nname: developer\nskills: [brief-workflow, brief-workflow]\n---\n\nbody\n",
		},
		{
			name: "duplicate block entries: only the first is removed, the skill is still listed",
			body: "---\nname: developer\nskills:\n  - brief-workflow\n  - brief-workflow\n---\n\nbody\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := t.TempDir()
			home := t.TempDir()
			srv := installClaudeCode(t, wd, home, "", "developer", "", false)

			path := filepath.Join(wd, ".claude", "agents", "developer.md")
			writeMissingSkillAgent(t, path, c.body)

			res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
			require.NoError(t, err)

			assertNoBoundAgentRow(t, res, path)

			after, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, c.body, string(after))
		})
	}
}

func Test_uninstall_edits_a_shared_bound_agent_once(t *testing.T) {
	wd := t.TempDir()
	home := t.TempDir()
	srv := installClaudeCode(t, wd, home, "developer", "developer", "", false)

	path := filepath.Join(wd, ".claude", "agents", "developer.md")
	writeMissingSkillAgent(t, path, "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n")

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	var rows []setup.Artifact
	for _, a := range res.Artifacts {
		if a.Kind == setup.KindBoundAgent {
			rows = append(rows, a)
		}
	}
	require.Len(t, rows, 1)
	assert.Equal(t, setup.ActionRemoved, rows[0].Action)

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "---\nname: developer\n---\n\nbody\n", string(after))
}

func Test_uninstall_dry_run_plans_bound_agent_rows_and_writes_nothing(t *testing.T) {
	wd := t.TempDir()
	home := t.TempDir()
	srv := installClaudeCode(t, wd, home, "", "developer", "", false)

	path := filepath.Join(wd, ".claude", "agents", "developer.md")
	body := "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n"
	writeMissingSkillAgent(t, path, body)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode, DryRun: true})
	require.NoError(t, err)

	row := findBoundAgentRow(t, res, path)
	assert.Equal(t, setup.ActionRemoved, row.Action)
	assert.Empty(t, res.Modified)

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, body, string(after))
}
