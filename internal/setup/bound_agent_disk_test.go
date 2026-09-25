package setup_test

// Init's --edit-agents write path always reads and writes through
// bound_agent.go's real os.Lstat/os.ReadFile; several cases here seed a
// real symlink an rwfs.Mem cannot reproduce faithfully. writeConfigWithRoles,
// writeMissingSkillAgent and newServerWithHome are shared from
// missing_skill_disk_test.go.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findBoundAgentRow returns res's own KindBoundAgent artifact for path,
// failing the test if there is none.
func findBoundAgentRow(t *testing.T, res setup.Result, path string) setup.Artifact {
	t.Helper()

	for _, a := range res.Artifacts {
		if a.Kind == setup.KindBoundAgent && a.Path == path {
			return a
		}
	}

	t.Fatalf("no bound-agent row for %s", path)

	return setup.Artifact{}
}

// assertNoBoundAgentRow reports a failure if res carries a KindBoundAgent
// row for path.
func assertNoBoundAgentRow(t *testing.T, res setup.Result, path string) {
	t.Helper()

	for _, a := range res.Artifacts {
		if a.Kind == setup.KindBoundAgent && a.Path == path {
			t.Fatalf("unexpected bound-agent row for %s", path)
		}
	}
}

// missingSkillPaths extracts res.AgentsMissingSkill's own paths.
func missingSkillPaths(res setup.Result) []string {
	out := make([]string, 0, len(res.AgentsMissingSkill))
	for _, a := range res.AgentsMissingSkill {
		out = append(out, a.Path)
	}

	return out
}

func Test_init_edit_agents_adds_the_skill_to_bound_project_agents(t *testing.T) {
	wd := t.TempDir()
	home := t.TempDir()
	writeConfigWithRoles(t, wd, "", "developer", "")

	path := filepath.Join(wd, ".claude", "agents", "developer", "Agent.md")
	body := "---\nname: developer\n---\n\nbody\n"
	writeMissingSkillAgent(t, path, body)
	require.NoError(t, os.Chmod(path, 0o640))

	srv := newServerWithHome(t, home)
	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
	require.NoError(t, err)

	row := findBoundAgentRow(t, res, path)
	assert.Equal(t, setup.ActionMerged, row.Action)
	assert.Equal(t, `brief-workflow added to skills`, row.Detail)

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n", string(after))

	info, statErr := os.Stat(path)
	require.NoError(t, statErr)
	assert.Equal(t, fs.FileMode(0o640), info.Mode().Perm())

	assert.Contains(t, res.Modified, path)
	assert.Empty(t, res.AgentsMissingSkill)
}

func Test_init_edit_agents_orders_bound_agent_rows(t *testing.T) {
	wd := t.TempDir()
	home := t.TempDir()
	writeConfigWithRoles(t, wd, "", "developer", "")

	path := filepath.Join(wd, ".claude", "agents", "developer.md")
	writeMissingSkillAgent(t, path, "---\nname: developer\n---\n\nbody\n")

	srv := newServerWithHome(t, home)
	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true, EditAgents: true})
	require.NoError(t, err)

	var (
		skillIdx   = -1
		agentIdxs  []int
		boundIdx   = -1
		snippetIdx = -1
	)

	for i, a := range res.Artifacts {
		switch a.Kind {
		case setup.KindSkill:
			skillIdx = i
		case setup.KindAgent:
			agentIdxs = append(agentIdxs, i)
		case setup.KindBoundAgent:
			boundIdx = i
		case setup.KindSnippet:
			snippetIdx = i
		}
	}

	require.NotEqual(t, -1, skillIdx)
	require.Len(t, agentIdxs, 3)
	require.NotEqual(t, -1, boundIdx)
	require.NotEqual(t, -1, snippetIdx)

	assert.Less(t, skillIdx, agentIdxs[0])
	assert.Less(t, agentIdxs[len(agentIdxs)-1], boundIdx)
	assert.Less(t, boundIdx, snippetIdx)
}

// A path is listed after the run iff it was lacking before and its row is
// not "merged".
func Test_init_edit_agents_rows_for_unchanged_and_kept_shapes(t *testing.T) {
	t.Run("already listed reports unchanged and stays byte-identical", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "", "developer", "")
		path := filepath.Join(wd, ".claude", "agents", "developer.md")
		body := "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n"
		writeMissingSkillAgent(t, path, body)

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
		require.NoError(t, err)

		row := findBoundAgentRow(t, res, path)
		assert.Equal(t, setup.ActionUnchanged, row.Action)
		assert.Empty(t, row.Detail)

		after, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, body, string(after))
		assert.Empty(t, res.AgentsMissingSkill)
	})

	t.Run("other shape reports kept, stays byte-identical, stays in the missing-skill report", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "", "developer", "")
		path := filepath.Join(wd, ".claude", "agents", "developer.md")
		body := "---\nname: developer\nskills: brief-workflow\n---\n\nbody\n"
		writeMissingSkillAgent(t, path, body)

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
		require.NoError(t, err)

		row := findBoundAgentRow(t, res, path)
		assert.Equal(t, setup.ActionKept, row.Action)
		assert.Equal(t, `skills: is not a list brief can edit; add brief-workflow by hand`, row.Detail)

		after, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, body, string(after))

		require.Len(t, res.AgentsMissingSkill, 1)
		assert.Equal(t, path, res.AgentsMissingSkill[0].Path)
	})

	t.Run("agents_missing_skill after the run iff lacking before and not merged", func(t *testing.T) {
		cases := []struct {
			name          string
			body          string
			lackingBefore bool
			merged        bool
		}{
			{name: "no key", body: "---\nname: developer\n---\n\nbody\n", lackingBefore: true, merged: true},
			{name: "block list", body: "---\nname: developer\nskills:\n  - other\n---\n\nbody\n", lackingBefore: true, merged: true},
			{name: "flow list", body: "---\nname: developer\nskills: [other]\n---\n\nbody\n", lackingBefore: true, merged: true},
			{name: "other shape", body: "---\nname: developer\nskills: brief-workflow\n---\n\nbody\n", lackingBefore: true, merged: false},
			{name: "already listed", body: "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n", lackingBefore: false, merged: false},
		}

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				wd := t.TempDir()
				home := t.TempDir()
				writeConfigWithRoles(t, wd, "", "developer", "")
				path := filepath.Join(wd, ".claude", "agents", "developer.md")
				writeMissingSkillAgent(t, path, c.body)

				srv := newServerWithHome(t, home)
				res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
				require.NoError(t, err)

				row := findBoundAgentRow(t, res, path)
				assert.Equal(t, c.merged, row.Action == setup.ActionMerged)

				wantListed := c.lackingBefore && !c.merged
				assert.Equal(t, wantListed, contains(missingSkillPaths(res), path))
			})
		}
	})
}

// contains reports whether list holds want.
func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}

	return false
}

func Test_init_edit_agents_leaves_non_targets_alone(t *testing.T) {
	t.Run("a user-level binding is left alone; the identical agent at project scope is merged (control)", func(t *testing.T) {
		wd := t.TempDir()
		// home nests under wd, isolating the ScopeProject filter from the
		// containment check, which this subtest is not pinning.
		home := filepath.Join(wd, "home")
		writeConfigWithRoles(t, wd, "", "developer", "")

		homePath := filepath.Join(home, ".claude", "agents", "developer.md")
		body := "---\nname: developer\n---\n\nbody\n"
		writeMissingSkillAgent(t, homePath, body)

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
		require.NoError(t, err)

		assertNoBoundAgentRow(t, res, homePath)

		after, readErr := os.ReadFile(homePath)
		require.NoError(t, readErr)
		assert.Equal(t, body, string(after))

		require.Len(t, res.AgentsMissingSkill, 1)
		assert.Equal(t, homePath, res.AgentsMissingSkill[0].Path)

		projectPath := filepath.Join(wd, ".claude", "agents", "developer.md")
		writeMissingSkillAgent(t, projectPath, body)

		res2, err2 := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
		require.NoError(t, err2)

		row := findBoundAgentRow(t, res2, projectPath)
		assert.Equal(t, setup.ActionMerged, row.Action)
	})

	t.Run("brief:*, other-plugin and reviewer bindings get no row", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "brief:planner", "mine:implementer", "developer")

		reviewerPath := filepath.Join(wd, ".claude", "agents", "developer.md")
		writeMissingSkillAgent(t, reviewerPath, "---\nname: developer\n---\n\nbody\n")

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
		require.NoError(t, err)

		for _, a := range res.Artifacts {
			assert.NotEqual(t, setup.KindBoundAgent, a.Kind)
		}
	})

	t.Run("a symlinked leaf is kept as not a regular file; the identical bytes as a regular file are merged (control)", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		outside := t.TempDir()
		writeConfigWithRoles(t, wd, "", "developer", "")

		targetPath := filepath.Join(outside, "real.md")
		body := "---\nname: developer\n---\n\nbody\n"
		require.NoError(t, os.WriteFile(targetPath, []byte(body), 0o600))

		linkPath := filepath.Join(wd, ".claude", "agents", "developer.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(linkPath), 0o755))
		require.NoError(t, os.Symlink(targetPath, linkPath))

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
		require.NoError(t, err)

		row := findBoundAgentRow(t, res, linkPath)
		assert.Equal(t, setup.ActionKept, row.Action)
		assert.Equal(t, "not a regular file", row.Detail)

		targetAfter, readErr := os.ReadFile(targetPath)
		require.NoError(t, readErr)
		assert.Equal(t, body, string(targetAfter))

		linkInfo, lstatErr := os.Lstat(linkPath)
		require.NoError(t, lstatErr)
		assert.True(t, linkInfo.Mode()&os.ModeSymlink != 0, "the leaf must still be a symlink")

		require.NoError(t, os.Remove(linkPath))
		require.NoError(t, os.WriteFile(linkPath, []byte(body), 0o600))

		res2, err2 := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
		require.NoError(t, err2)

		row2 := findBoundAgentRow(t, res2, linkPath)
		assert.Equal(t, setup.ActionMerged, row2.Action)
	})

	t.Run("a symlink to a regular file inside root is still kept as not a regular file (isolates the leaf-mode check from the escape check)", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		writeConfigWithRoles(t, wd, "", "developer", "")

		targetPath := filepath.Join(wd, "real.md")
		body := "---\nname: developer\n---\n\nbody\n"
		require.NoError(t, os.WriteFile(targetPath, []byte(body), 0o600))

		linkPath := filepath.Join(wd, ".claude", "agents", "developer.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(linkPath), 0o755))
		require.NoError(t, os.Symlink(targetPath, linkPath))

		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
		require.NoError(t, err)

		row := findBoundAgentRow(t, res, linkPath)
		assert.Equal(t, setup.ActionKept, row.Action)
		assert.Equal(t, "not a regular file", row.Detail)

		targetAfter, readErr := os.ReadFile(targetPath)
		require.NoError(t, readErr)
		assert.Equal(t, body, string(targetAfter))
	})

	t.Run("a .claude symlinked outside the repo gets no row; a real .claude is merged (control)", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		outsideClaude := t.TempDir()
		writeConfigWithRoles(t, wd, "", "developer", "")

		agentPath := filepath.Join(outsideClaude, "agents", "developer.md")
		body := "---\nname: developer\n---\n\nbody\n"
		require.NoError(t, os.MkdirAll(filepath.Dir(agentPath), 0o755))
		require.NoError(t, os.WriteFile(agentPath, []byte(body), 0o600))

		require.NoError(t, os.Symlink(outsideClaude, filepath.Join(wd, ".claude")))

		// DryRun avoids tripping the writability pre-check on the symlinked
		// ".claude", unrelated to what this subtest proves.
		srv := newServerWithHome(t, home)
		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true, DryRun: true})
		require.NoError(t, err)

		for _, a := range res.Artifacts {
			assert.NotEqual(t, setup.KindBoundAgent, a.Kind)
		}

		after, readErr := os.ReadFile(agentPath)
		require.NoError(t, readErr)
		assert.Equal(t, body, string(after))

		require.Len(t, res.AgentsMissingSkill, 1)
		assert.Equal(t, filepath.Join(wd, ".claude", "agents", "developer.md"), res.AgentsMissingSkill[0].Path)
		assert.Equal(t, setup.ReachEscaped, res.AgentsMissingSkill[0].Reach, "a binding resolved only through a symlinked .claude must be reported as escaping the repository")

		wd2 := t.TempDir()
		writeConfigWithRoles(t, wd2, "", "developer", "")
		realPath := filepath.Join(wd2, ".claude", "agents", "developer.md")
		writeMissingSkillAgent(t, realPath, body)

		res2, err2 := srv.Init(t.Context(), wd2, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
		require.NoError(t, err2)

		row := findBoundAgentRow(t, res2, realPath)
		assert.Equal(t, setup.ActionMerged, row.Action)
	})
}

// addWorkflowSkill is a surgical text scan, not a YAML parser; each case
// here is a shape it misjudges, caught by re-decoding the edit afterward.
func Test_init_edit_agents_falls_back_when_the_text_edit_cannot_be_verified(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "space before the colon: a second skills: line would be inserted, producing a duplicate key",
			body: "---\nname: developer\nskills : [a]\n---\n\nbody\n",
		},
		{
			name: "quoted key: a second skills: line would be inserted, producing a duplicate key",
			body: "---\nname: developer\n\"skills\": [a]\n---\n\nbody\n",
		},
		{
			name: "block item folded onto the next line: the inserted item absorbs the continuation",
			body: "---\nname: developer\nskills:\n  - a\n    continued\n---\n\nbody\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := t.TempDir()
			home := t.TempDir()
			writeConfigWithRoles(t, wd, "", "developer", "")
			path := filepath.Join(wd, ".claude", "agents", "developer.md")
			writeMissingSkillAgent(t, path, c.body)

			srv := newServerWithHome(t, home)
			res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
			require.NoError(t, err)

			row := findBoundAgentRow(t, res, path)
			assert.Equal(t, setup.ActionKept, row.Action)
			assert.Equal(t, `skills: is not a list brief can edit; add brief-workflow by hand`, row.Detail)

			after, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, c.body, string(after))

			require.Len(t, res.AgentsMissingSkill, 1)
			assert.Equal(t, path, res.AgentsMissingSkill[0].Path)
		})
	}
}

func Test_init_edit_agents_edits_a_shared_agent_once(t *testing.T) {
	wd := t.TempDir()
	home := t.TempDir()
	writeConfigWithRoles(t, wd, "developer", "developer", "")

	path := filepath.Join(wd, ".claude", "agents", "developer.md")
	writeMissingSkillAgent(t, path, "---\nname: developer\n---\n\nbody\n")

	srv := newServerWithHome(t, home)
	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
	require.NoError(t, err)

	var rows []setup.Artifact
	for _, a := range res.Artifacts {
		if a.Kind == setup.KindBoundAgent {
			rows = append(rows, a)
		}
	}
	require.Len(t, rows, 1)
	assert.Equal(t, setup.ActionMerged, rows[0].Action)

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n", string(after))

	assert.Empty(t, res.AgentsMissingSkill)
}

func Test_init_edit_agents_dry_run_and_print_write_nothing(t *testing.T) {
	newFixture := func(t *testing.T) (wd, home, mergePath, otherPath string) {
		t.Helper()
		wd = t.TempDir()
		home = t.TempDir()
		writeConfigWithRoles(t, wd, "other-shape-agent", "developer", "")

		mergePath = filepath.Join(wd, ".claude", "agents", "developer.md")
		writeMissingSkillAgent(t, mergePath, "---\nname: developer\n---\n\nbody\n")

		otherPath = filepath.Join(wd, ".claude", "agents", "other-shape-agent.md")
		writeMissingSkillAgent(t, otherPath, "---\nname: other-shape-agent\nskills: brief-workflow\n---\n\nbody\n")

		return wd, home, mergePath, otherPath
	}

	t.Run("dry run", func(t *testing.T) {
		wd, home, mergePath, otherPath := newFixture(t)
		srv := newServerWithHome(t, home)

		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true, DryRun: true})
		require.NoError(t, err)

		row := findBoundAgentRow(t, res, mergePath)
		assert.Equal(t, setup.ActionMerged, row.Action)

		after, readErr := os.ReadFile(mergePath)
		require.NoError(t, readErr)
		assert.Equal(t, "---\nname: developer\n---\n\nbody\n", string(after))

		paths := missingSkillPaths(res)
		assert.NotContains(t, paths, mergePath)
		assert.Contains(t, paths, otherPath)
	})

	t.Run("print", func(t *testing.T) {
		wd, home, mergePath, otherPath := newFixture(t)
		srv := newServerWithHome(t, home)

		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true, Print: true})
		require.NoError(t, err)

		var found *setup.PrintArtifact
		for i := range res.Print {
			if res.Print[i].Path == mergePath {
				found = &res.Print[i]
			}

			assert.NotEqual(t, otherPath, res.Print[i].Path, "a kept other-shape row must never print")
		}
		require.NotNil(t, found)
		assert.Equal(t, setup.PrintMerge, found.Action)
		assert.Equal(t, "skills: [brief-workflow]", found.Body)

		after, readErr := os.ReadFile(mergePath)
		require.NoError(t, readErr)
		assert.Equal(t, "---\nname: developer\n---\n\nbody\n", string(after))

		paths := missingSkillPaths(res)
		assert.NotContains(t, paths, mergePath)
		assert.Contains(t, paths, otherPath)
	})
}

func Test_init_edit_agents_refuses_without_claude_code(t *testing.T) {
	t.Run("explicit HostNone", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		srv := newServerWithHome(t, home)

		_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, EditAgents: true})
		require.ErrorIs(t, err, setup.ErrEditAgentsNeedHost)

		_, statErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("detection finds nothing", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		srv := newServerWithHome(t, home)

		_, err := srv.Init(t.Context(), wd, setup.InitRequest{EditAgents: true})
		require.ErrorIs(t, err, setup.ErrEditAgentsNeedHost)

		_, statErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("with-agents also set: ErrAgentsNeedHost wins", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		srv := newServerWithHome(t, home)

		_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, WithAgents: true, EditAgents: true})
		require.ErrorIs(t, err, setup.ErrAgentsNeedHost)
		assert.False(t, errors.Is(err, setup.ErrEditAgentsNeedHost))
	})
}

func Test_init_edit_agents_joins_the_writability_precheck(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	wd := t.TempDir()
	home := t.TempDir()
	writeConfigWithRoles(t, wd, "", "developer", "")

	agentsDir := filepath.Join(wd, ".claude", "agents")
	path := filepath.Join(agentsDir, "developer.md")
	body := "---\nname: developer\n---\n\nbody\n"
	writeMissingSkillAgent(t, path, body)

	require.NoError(t, os.Chmod(agentsDir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(agentsDir, 0o755) })

	configPath := filepath.Join(wd, ".brief.yaml")
	configBefore, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)

	srv := newServerWithHome(t, home)
	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})

	require.ErrorIs(t, err, setup.ErrUnwritable)
	refusal, ok := errors.AsType[*setup.RefusalError](err)
	require.True(t, ok)
	assert.Equal(t, agentsDir, refusal.Path)

	assert.Empty(t, res.Created)
	assert.Empty(t, res.Modified)

	configAfter, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, configBefore, configAfter)

	after, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, body, string(after))
}
