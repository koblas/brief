// OS-subject: bound_agent.go always reads and writes agent files through
// real disk regardless of setup.WithFSRoot, so these cases cannot move onto
// init_internal_test.go's rwfs.Mem seam. White-box: reaches run directly so
// a bare role binding resolves against an injected, empty home directory
// instead of the developer's own "~/.claude/agents".

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emptyHomeSeam returns a runSeam pinning setup.WithHomeDir to a fresh,
// empty t.TempDir() carrying no ".claude" of its own.
func emptyHomeSeam(t *testing.T) runSeam {
	t.Helper()

	home := t.TempDir()

	return withSetupOpts(setup.WithHomeDir(func() (string, error) { return home, nil }))
}

// missingSkillHeaderLine is the exact stderr header rendered when at least
// one listed agent is one --edit-agents could still reach.
const missingSkillHeaderLine = `brief init: bound agents do not preload the brief-workflow skill; add "brief-workflow" to the "skills:" list in each, or rerun with --edit-agents:`

// missingSkillHeaderLinePlain is rendered instead when suggesting
// --edit-agents could not help.
const missingSkillHeaderLinePlain = `brief init: bound agents do not preload the brief-workflow skill; add "brief-workflow" to the "skills:" list in each:`

// installedNextActionLine is the exact stderr next-action line an explicit
// "--host claude-code" run reports once at least one artifact changed and
// root == wd.
const installedNextActionLine = "brief init: installed for claude-code; start Claude Code in this directory " +
	"(or run /reload-plugins in a session already here), then 'brief new feature <name>'"

// Display groups project-scope rows ahead of user-scope ones. The
// --edit-agents sub-case merges the fixable project row and re-checks the
// header: once this run already carries the flag, it never suggests it
// again even though the user-level row remains.
func Test_init_lists_bound_agents_missing_the_workflow_skill(t *testing.T) {
	wd := t.TempDir()
	home := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
		"feature-directory: docs/specifications\nroles:\n  planner: planner\n  implementer: developer\n",
	), 0o600))

	homePlanner := filepath.Join(home, ".claude", "agents", "planner.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(homePlanner), 0o755))
	plannerBody := []byte("---\nname: planner\n---\n\nbody\n")
	require.NoError(t, os.WriteFile(homePlanner, plannerBody, 0o600))

	projectDeveloper := filepath.Join(wd, ".claude", "agents", "developer", "Agent.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(projectDeveloper), 0o755))
	developerBody := []byte("---\nname: developer\n---\n\nbody\n")
	require.NoError(t, os.WriteFile(projectDeveloper, developerBody, 0o600))

	seam := withSetupOpts(setup.WithHomeDir(func() (string, error) { return home, nil }))

	wantBlock := missingSkillHeaderLine + "\n" +
		"  .claude/agents/developer/Agent.md (implementer)\n" +
		"  ~/.claude/agents/planner.md (planner; user-level, edit by hand)\n"

	t.Run("without --dry-run", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)

		require.NoError(t, err)
		assert.Equal(t, wantBlock+installedNextActionLine+"\n", stderr.String())

		after, readErr := os.ReadFile(projectDeveloper)
		require.NoError(t, readErr)
		assert.Equal(t, developerBody, after)

		homeAfter, readErr := os.ReadFile(homePlanner)
		require.NoError(t, readErr)
		assert.Equal(t, plannerBody, homeAfter)
	})

	t.Run("--dry-run reports the same block", func(t *testing.T) {
		wd := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
			"feature-directory: docs/specifications\nroles:\n  planner: planner\n  implementer: developer\n",
		), 0o600))

		projectDeveloper := filepath.Join(wd, ".claude", "agents", "developer", "Agent.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(projectDeveloper), 0o755))
		require.NoError(t, os.WriteFile(projectDeveloper, developerBody, 0o600))

		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--dry-run"}, nil, &stdout, &stderr, noBuildInfo, seam)

		require.NoError(t, err)
		assert.Equal(t, wantBlock+"brief init: dry run, no files changed; rerun without --dry-run to apply\n", stderr.String())

		entries, readErr := os.ReadDir(filepath.Join(wd, ".claude"))
		require.NoError(t, readErr)
		assert.Len(t, entries, 1, "--dry-run must write nothing beyond the fixture's own agents directory")
	})

	t.Run("--edit-agents merges the project agent and leaves the user-level one alone", func(t *testing.T) {
		wd := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
			"feature-directory: docs/specifications\nroles:\n  planner: planner\n  implementer: developer\n",
		), 0o600))

		projectDeveloper := filepath.Join(wd, ".claude", "agents", "developer", "Agent.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(projectDeveloper), 0o755))
		require.NoError(t, os.WriteFile(projectDeveloper, developerBody, 0o600))

		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--edit-agents"}, nil, &stdout, &stderr, noBuildInfo, seam)

		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "merged .claude/agents/developer/Agent.md (brief-workflow added to skills)\n")

		wantBlock := missingSkillHeaderLinePlain + "\n" +
			"  ~/.claude/agents/planner.md (planner; user-level, edit by hand)\n"
		assert.Equal(t, wantBlock+installedNextActionLine+"\n", stderr.String())

		after, readErr := os.ReadFile(projectDeveloper)
		require.NoError(t, readErr)
		assert.Equal(t, "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n", string(after))

		homeAfter, readErr := os.ReadFile(homePlanner)
		require.NoError(t, readErr)
		assert.Equal(t, plannerBody, homeAfter)
	})
}

// nothingToEditLine is --edit-agents' own exit-0 "nothing to edit" stderr
// line.
const nothingToEditLine = `brief init: --edit-agents: no planner or implementer bound to an agent under .claude/agents; nothing to edit`

func Test_init_edit_agents_says_nothing_to_edit(t *testing.T) {
	t.Run("no bare planner or implementer bound", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--edit-agents"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Contains(t, stderr.String(), nothingToEditLine+"\n")
	})

	t.Run("only bare binding is user-level: nothing-to-edit precedes the missing-skill block", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
			"feature-directory: docs/specifications\nroles:\n  implementer: planner\n",
		), 0o600))

		homePlanner := filepath.Join(home, ".claude", "agents", "planner.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(homePlanner), 0o755))
		require.NoError(t, os.WriteFile(homePlanner, []byte("---\nname: planner\n---\n\nbody\n"), 0o600))

		seam := withSetupOpts(setup.WithHomeDir(func() (string, error) { return home, nil }))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--edit-agents"}, nil, &stdout, &stderr, noBuildInfo, seam)

		require.NoError(t, err)

		nothingIdx := strings.Index(stderr.String(), nothingToEditLine)
		missingIdx := strings.Index(stderr.String(), missingSkillHeaderLinePlain)
		require.NotEqual(t, -1, nothingIdx)
		require.NotEqual(t, -1, missingIdx)
		assert.Less(t, nothingIdx, missingIdx)
	})

	t.Run("absent under --json", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--edit-agents", "--json"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Empty(t, stderr.String())
		assert.NotContains(t, stdout.String(), "nothing to edit")
	})
}

// ", or rerun with --edit-agents:" is appended only when this run did not
// already carry the flag AND at least one listed agent is one --edit-agents
// could still reach.
func Test_init_missing_skill_header_omits_edit_agents_suggestion_when_it_cannot_help(t *testing.T) {
	t.Run("no --edit-agents given, only a user-level agent remains", func(t *testing.T) {
		wd := t.TempDir()
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
			"feature-directory: docs/specifications\nroles:\n  planner: planner\n",
		), 0o600))

		homePlanner := filepath.Join(home, ".claude", "agents", "planner.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(homePlanner), 0o755))
		require.NoError(t, os.WriteFile(homePlanner, []byte("---\nname: planner\n---\n\nbody\n"), 0o600))

		seam := withSetupOpts(setup.WithHomeDir(func() (string, error) { return home, nil }))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)

		require.NoError(t, err)
		assert.Equal(t,
			missingSkillHeaderLinePlain+"\n"+
				"  ~/.claude/agents/planner.md (planner; user-level, edit by hand)\n"+
				installedNextActionLine+"\n",
			stderr.String())
	})

	t.Run("--edit-agents given, already kept its one bound agent as uneditable", func(t *testing.T) {
		wd := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
			"feature-directory: docs/specifications\nroles:\n  implementer: developer\n",
		), 0o600))

		agentPath := filepath.Join(wd, ".claude", "agents", "developer.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(agentPath), 0o755))
		require.NoError(t, os.WriteFile(agentPath, []byte("---\nname: developer\nskills: brief-workflow\n---\n\nbody\n"), 0o600))

		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--edit-agents"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "kept .claude/agents/developer.md (skills: is not a list brief can edit; add brief-workflow by hand)\n")
		assert.Equal(t,
			missingSkillHeaderLinePlain+"\n"+
				"  .claude/agents/developer.md (implementer; skills: is not a list brief can edit, edit by hand)\n"+
				installedNextActionLine+"\n",
			stderr.String())
	})
}

// A bound implementer resolved only through a ".claude" symlinked outside
// the repository is listed "; outside the repository, edit by hand", and
// keeps the header plain since --edit-agents cannot reach it either.
func Test_init_missing_skill_lists_an_escaping_bound_agent_separately(t *testing.T) {
	wd := t.TempDir()
	home := t.TempDir()
	outsideClaude := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
		"feature-directory: docs/specifications\nroles:\n  implementer: developer\n",
	), 0o600))

	agentPath := filepath.Join(outsideClaude, "agents", "developer.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(agentPath), 0o755))
	require.NoError(t, os.WriteFile(agentPath, []byte("---\nname: developer\n---\n\nbody\n"), 0o600))
	require.NoError(t, os.Symlink(outsideClaude, filepath.Join(wd, ".claude")))

	seam := withSetupOpts(setup.WithHomeDir(func() (string, error) { return home, nil }))
	var stdout, stderr bytes.Buffer

	// --dry-run: a real run would also trip the writability pre-check on the symlinked ".claude", unrelated to this test.
	err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--dry-run"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t,
		missingSkillHeaderLinePlain+"\n"+
			"  .claude/agents/developer.md (implementer; outside the repository, edit by hand)\n"+
			"brief init: dry run, no files changed; rerun without --dry-run to apply\n",
		stderr.String())
}

// A bound implementer resolved to a symlinked leaf whose target still
// resolves inside the repository is listed "; not a regular file, edit by
// hand". The control proves the symlink itself disqualifies the row: the
// identical frontmatter as a plain regular file resolves fixable instead.
func Test_init_missing_skill_lists_a_not_regular_leaf_separately(t *testing.T) {
	body := []byte("---\nname: developer\n---\n\nbody\n")

	t.Run("symlinked leaf, target inside the repository", func(t *testing.T) {
		wd := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
			"feature-directory: docs/specifications\nroles:\n  implementer: developer\n",
		), 0o600))

		agentsDir := filepath.Join(wd, ".claude", "agents")
		require.NoError(t, os.MkdirAll(agentsDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "developer-body.txt"), body, 0o600))
		require.NoError(t, os.Symlink("developer-body.txt", filepath.Join(agentsDir, "developer.md")))

		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Equal(t,
			missingSkillHeaderLinePlain+"\n"+
				"  .claude/agents/developer.md (implementer; not a regular file, edit by hand)\n"+
				installedNextActionLine+"\n",
			stderr.String())
	})

	t.Run("control: identical bytes as a regular file resolve fixable", func(t *testing.T) {
		wd := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
			"feature-directory: docs/specifications\nroles:\n  implementer: developer\n",
		), 0o600))

		agentPath := filepath.Join(wd, ".claude", "agents", "developer.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(agentPath), 0o755))
		require.NoError(t, os.WriteFile(agentPath, body, 0o600))

		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Equal(t,
			missingSkillHeaderLine+"\n"+
				"  .claude/agents/developer.md (implementer)\n"+
				installedNextActionLine+"\n",
			stderr.String())
	})
}

// A row's reach is decided by its shape, never by whether an edit was
// attempted, so a bound agent with an unrecognized "skills:" shape is
// listed uneditable even when --edit-agents was never given.
func Test_init_missing_skill_lists_an_uneditable_shape_without_edit_agents(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
		"feature-directory: docs/specifications\nroles:\n  implementer: developer\n",
	), 0o600))

	agentPath := filepath.Join(wd, ".claude", "agents", "developer.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(agentPath), 0o755))
	require.NoError(t, os.WriteFile(agentPath, []byte("---\nname: developer\nskills: tdd\n---\n\nbody\n"), 0o600))

	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

	require.NoError(t, err)
	assert.Equal(t,
		missingSkillHeaderLinePlain+"\n"+
			"  .claude/agents/developer.md (implementer; skills: is not a list brief can edit, edit by hand)\n"+
			installedNextActionLine+"\n",
		stderr.String())
}

func Test_init_edit_agents_dry_run_and_print(t *testing.T) {
	newFixture := func(t *testing.T, crlf bool) (string, runSeam, string) {
		t.Helper()

		wd := t.TempDir()
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
			"feature-directory: docs/specifications\nroles:\n  implementer: developer\n",
		), 0o600))

		agentPath := filepath.Join(wd, ".claude", "agents", "developer", "Agent.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(agentPath), 0o755))

		body := "---\nname: developer\n---\n\nbody\n"
		if crlf {
			body = "---\r\nname: developer\r\n---\r\n\r\nbody\r\n"
		}

		require.NoError(t, os.WriteFile(agentPath, []byte(body), 0o600))

		seam := withSetupOpts(setup.WithHomeDir(func() (string, error) { return home, nil }))

		return wd, seam, agentPath
	}

	t.Run("--dry-run shows the merged row and leaves the file unchanged", func(t *testing.T) {
		wd, seam, agentPath := newFixture(t, false)
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--edit-agents", "--dry-run"}, nil, &stdout, &stderr, noBuildInfo, seam)
		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "merged .claude/agents/developer/Agent.md (brief-workflow added to skills)\n")

		after, readErr := os.ReadFile(agentPath)
		require.NoError(t, readErr)
		assert.Equal(t, "---\nname: developer\n---\n\nbody\n", string(after))
	})

	t.Run("--print shows the merge header and only the inserted line, no CR from a CRLF fixture", func(t *testing.T) {
		wd, seam, agentPath := newFixture(t, true)
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--edit-agents", "--print"}, nil, &stdout, &stderr, noBuildInfo, seam)
		require.NoError(t, err)

		assert.Contains(t, stdout.String(), "# .claude/agents/developer/Agent.md (merge)\nskills: [brief-workflow]\n")
		assert.NotContains(t, stdout.String(), "skills: [brief-workflow]\r")

		after, readErr := os.ReadFile(agentPath)
		require.NoError(t, readErr)
		assert.Equal(t, "---\r\nname: developer\r\n---\r\n\r\nbody\r\n", string(after))
	})

	t.Run("--print --json carries the merge body", func(t *testing.T) {
		wd, seam, agentPath := newFixture(t, false)
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--edit-agents", "--print", "--json"}, nil, &stdout, &stderr, noBuildInfo, seam)
		require.NoError(t, err)

		var doc struct {
			Artifacts []struct {
				Path   string `json:"path"`
				Action string `json:"action"`
				Body   string `json:"body"`
			} `json:"artifacts"`
		}
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))

		var found bool
		for _, a := range doc.Artifacts {
			if a.Path == agentPath {
				found = true
				assert.Equal(t, "merge", a.Action)
				assert.Equal(t, "skills: [brief-workflow]", a.Body)
			}
		}
		assert.True(t, found)
	})
}

func Test_init_edit_agents_json(t *testing.T) {
	wd := t.TempDir()
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
		"feature-directory: docs/specifications\nroles:\n  implementer: developer\n",
	), 0o600))

	agentPath := filepath.Join(wd, ".claude", "agents", "developer.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(agentPath), 0o755))
	require.NoError(t, os.WriteFile(agentPath, []byte("---\nname: developer\n---\n\nbody\n"), 0o600))

	seam := withSetupOpts(setup.WithHomeDir(func() (string, error) { return home, nil }))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--edit-agents", "--json"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	var doc struct {
		Modified  []string `json:"modified"`
		Artifacts []struct {
			Kind   string `json:"kind"`
			Path   string `json:"path"`
			Action string `json:"action"`
		} `json:"artifacts"`
		AgentsMissingSkill []struct {
			Path string `json:"path"`
		} `json:"agents_missing_skill"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))

	assert.Contains(t, doc.Modified, agentPath)

	var found bool
	for _, a := range doc.Artifacts {
		if a.Path == agentPath {
			found = true
			assert.Equal(t, "bound-agent", a.Kind)
			assert.Equal(t, "merged", a.Action)
		}
	}
	assert.True(t, found)

	for _, m := range doc.AgentsMissingSkill {
		assert.NotEqual(t, agentPath, m.Path)
	}
}

func Test_init_prints_roles_to_add_before_the_missing_skill_block(t *testing.T) {
	wd := t.TempDir()
	home := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
		"feature-directory: docs/specifications\nroles:\n  implementer: developer\n",
	), 0o600))

	developer := filepath.Join(wd, ".claude", "agents", "developer.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(developer), 0o755))
	require.NoError(t, os.WriteFile(developer, []byte("---\nname: developer\n---\n\nbody\n"), 0o600))

	seam := withSetupOpts(setup.WithHomeDir(func() (string, error) { return home, nil }))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--with-agents"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"brief init: .brief.yaml was not edited; to bind brief's agents, add these lines to it:\n"+
		"roles:\n"+
		"  planner: brief:planner\n"+
		"  reviewer: brief:reviewer\n"+
		missingSkillHeaderLine+"\n"+
		"  .claude/agents/developer.md (implementer)\n"+
		installedNextActionLine+"\n", stderr.String())
}

func Test_init_json_lists_agents_missing_the_skill(t *testing.T) {
	wd := t.TempDir()
	home := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
		"feature-directory: docs/specifications\nroles:\n  planner: planner\n  implementer: developer\n",
	), 0o600))

	homePlanner := filepath.Join(home, ".claude", "agents", "planner.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(homePlanner), 0o755))
	require.NoError(t, os.WriteFile(homePlanner, []byte("---\nname: planner\n---\n\nbody\n"), 0o600))

	projectDeveloper := filepath.Join(wd, ".claude", "agents", "developer", "Agent.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(projectDeveloper), 0o755))
	require.NoError(t, os.WriteFile(projectDeveloper, []byte("---\nname: developer\n---\n\nbody\n"), 0o600))

	seam := withSetupOpts(setup.WithHomeDir(func() (string, error) { return home, nil }))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--json"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	var doc struct {
		AgentsMissingSkill []struct {
			Role  string `json:"role"`
			Agent string `json:"agent"`
			Path  string `json:"path"`
			Scope string `json:"scope"`
		} `json:"agents_missing_skill"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))

	require.Len(t, doc.AgentsMissingSkill, 2)

	byRole := map[string]struct {
		Role  string
		Agent string
		Path  string
		Scope string
	}{}
	for _, a := range doc.AgentsMissingSkill {
		byRole[a.Role] = struct {
			Role  string
			Agent string
			Path  string
			Scope string
		}{a.Role, a.Agent, a.Path, a.Scope}
	}

	require.Contains(t, byRole, "planner")
	require.Contains(t, byRole, "implementer")

	assert.Equal(t, "planner", byRole["planner"].Agent)
	assert.Equal(t, homePlanner, byRole["planner"].Path)
	assert.True(t, filepath.IsAbs(byRole["planner"].Path))
	assert.Equal(t, "user", byRole["planner"].Scope)

	assert.Equal(t, "developer", byRole["implementer"].Agent)
	assert.Equal(t, projectDeveloper, byRole["implementer"].Path)
	assert.True(t, filepath.IsAbs(byRole["implementer"].Path))
	assert.Equal(t, "project", byRole["implementer"].Scope)

	rawKeys := extractFirstObjectKeys(t, stdout.Bytes(), "agents_missing_skill")
	assert.Equal(t, []string{"role", "agent", "path", "scope"}, rawKeys)
}

// extractFirstObjectKeys decodes doc[field]'s first array element as an
// ordered list of JSON object keys, via json.Decoder's token stream.
func extractFirstObjectKeys(t *testing.T, doc []byte, field string) []string {
	t.Helper()

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc, &raw))

	var items []json.RawMessage
	require.NoError(t, json.Unmarshal(raw[field], &items))
	require.NotEmpty(t, items)

	dec := json.NewDecoder(bytes.NewReader(items[0]))

	tok, err := dec.Token()
	require.NoError(t, err)
	require.Equal(t, json.Delim('{'), tok)

	var keys []string

	for dec.More() {
		keyTok, err := dec.Token()
		require.NoError(t, err)

		key, ok := keyTok.(string)
		require.True(t, ok, "an object key token must decode as a string")
		keys = append(keys, key)

		var discard json.RawMessage
		require.NoError(t, dec.Decode(&discard))
	}

	return keys
}
