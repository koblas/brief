// This file reaches the unexported run directly so host detection (R8) can
// be pinned against an injected, empty home directory: through cli.Run a
// bare "init" would detect against the developer's own "~/.claude", which
// exists for nearly every Claude Code user and would make these tests
// depend on the machine they happen to run on.

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
// empty t.TempDir() — a home directory that exists but carries no
// ".claude" of its own, so detection never finds anything through it.
func emptyHomeSeam(t *testing.T) runSeam {
	t.Helper()

	home := t.TempDir()

	return withSetupOpts(setup.WithHomeDir(func() (string, error) { return home, nil }))
}

// Test_init_without_host_detects_the_host_from_the_tree pins R8's
// detection rule at the cli boundary: nothing present resolves to
// HostNone with the R8 stderr line replacing the next action; a root
// "CLAUDE.md" resolves to claude-code and installs the plugin; under
// --json a detected-none run leaves stderr empty and reports "host":"none".
func Test_init_without_host_detects_the_host_from_the_tree(t *testing.T) {
	t.Run("nothing present detects none", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Equal(t, "created .brief.yaml\ncreated docs/specifications/\n", stdout.String())
		assert.Equal(t, "brief init: no agent host detected; run 'brief init --host claude-code' to install integration\n", stderr.String())
	})

	t.Run("a root CLAUDE.md detects claude-code", func(t *testing.T) {
		wd := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), []byte("# hi\n"), 0o600))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "created .claude/skills/brief/.claude-plugin/plugin.json\n")
		assert.Equal(t, "brief init: installed for claude-code (detected CLAUDE.md; use --host none to skip); "+
			"start Claude Code in this directory (or run /reload-plugins in a session already here), "+
			"then 'brief new feature <name>'\n", stderr.String())
	})

	t.Run("a root .claude directory detects claude-code and names it", func(t *testing.T) {
		wd := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(wd, ".claude"), 0o755))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Equal(t, "brief init: installed for claude-code (detected .claude; use --host none to skip); "+
			"start Claude Code in this directory (or run /reload-plugins in a session already here), "+
			"then 'brief new feature <name>'\n", stderr.String())
	})

	t.Run("an explicit --host claude-code names no detection signal", func(t *testing.T) {
		wd := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(wd, ".claude"), 0o755))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Equal(t, "brief init: installed for claude-code; start Claude Code in this directory (or run /reload-plugins in a session already here), then 'brief new feature <name>'\n", stderr.String())
	})

	t.Run("detected none under --json leaves stderr empty", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--json"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Empty(t, stderr.String())
		assert.Contains(t, stdout.String(), `"host":"none"`)
		assert.Contains(t, stdout.String(), `"detected_by":null`)
	})

	t.Run("a detected host reports detected_by under --json", func(t *testing.T) {
		wd := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(wd, ".claude"), 0o755))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--json"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Empty(t, stderr.String())
		assert.Contains(t, stdout.String(), `"detected_by":".claude"`)
	})
}

// Test_init_with_agents_follows_the_detected_host pins --with-agents
// against a detected (rather than explicit) host: nothing detected still
// refuses ErrAgentsNeedHost's own usage error with the tree left empty; a
// detected claude-code root installs the three agents.
func Test_init_with_agents_follows_the_detected_host(t *testing.T) {
	t.Run("nothing detected refuses", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--with-agents"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.Error(t, err)
		assert.Equal(t, 2, ExitCode(err))
		assert.Equal(t, "brief init: --with-agents requires --host claude-code; run 'brief init --host claude-code --with-agents'\n", stderr.String())

		entries, readErr := os.ReadDir(wd)
		require.NoError(t, readErr)
		assert.Empty(t, entries)
	})

	t.Run("a detected claude-code root installs the agents", func(t *testing.T) {
		wd := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(wd, ".claude"), 0o755))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--with-agents"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "created .claude/skills/brief/agents/planner.md\n")
	})
}

// missingSkillHeaderLine is the exact stderr header init's own missing-skill
// block renders (Surface & Copy) when at least one listed agent is one
// "--edit-agents" could still reach and this run did not already pass it,
// including the "brief init: " prefix.
const missingSkillHeaderLine = `brief init: bound agents do not preload the brief-workflow skill; add "brief-workflow" to the "skills:" list in each, or rerun with --edit-agents:`

// missingSkillHeaderLinePlain is missingSkillHeaderLine's own counterpart
// (Surface & Copy) rendered instead whenever suggesting "--edit-agents"
// could not help: this run already passed it, or every listed agent
// already carries its own "edit by hand" annotation.
const missingSkillHeaderLinePlain = `brief init: bound agents do not preload the brief-workflow skill; add "brief-workflow" to the "skills:" list in each:`

// installedNextActionLine is the exact stderr next-action line an explicit
// "--host claude-code" run reports once at least one artifact changed and
// root == wd — no detection clause, since the host was given explicitly.
const installedNextActionLine = "brief init: installed for claude-code; start Claude Code in this directory " +
	"(or run /reload-plugins in a session already here), then 'brief new feature <name>'"

// Test_init_lists_bound_agents_missing_the_workflow_skill pins S06's own
// stderr block: a home-level "planner" agent and a project, nested
// ".claude/agents/developer/Agent.md" bound as implementer, neither
// carrying the skill. The project row renders first, then the user row —
// display groups project-scope rows ahead of user-scope ones, distinct
// from Result.AgentsMissingSkill's own role-major order (setup's own
// missing_skill_test.go pins that order directly). Neither agent file is
// touched, and a --dry-run sub-case reports the identical block. The
// "--edit-agents" sub-case merges the fixable project row and re-checks
// the header itself: once this run already carries the flag, the header
// never suggests it again, even though a listed row (the user-level one)
// remains — missingSkillHeaderLinePlain, not missingSkillHeaderLine.
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
// line (Surface & Copy), including the "brief init: " prefix.
const nothingToEditLine = `brief init: --edit-agents: no planner or implementer bound to an agent under .claude/agents; nothing to edit`

// Test_init_edit_agents_says_nothing_to_edit pins the nothing-to-edit line:
// no bare planner or implementer bound at all, and a bare binding that only
// resolves at user scope, both trigger it, in text mode only, placed after
// roles_to_add and before the missing-skill block.
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

// Test_init_missing_skill_header_omits_edit_agents_suggestion_when_it_cannot_help
// pins missingSkillHeader's own conditional suffix (product-vision fix
// round): ", or rerun with --edit-agents:" is appended only when this run
// did not already carry the flag AND at least one listed agent is one
// "--edit-agents" could still reach — a bare-name project binding, not
// escaping the repository. Two ways that condition fails, both getting the
// plain header: no "--edit-agents" was given, but the only lacking agent
// is user-level (which the flag could never reach); "--edit-agents" was
// given and already tried its one bound agent, which it could not edit (an
// unrecognized "skills:" shape, left "kept").
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
		assert.Contains(t, stderr.String(), missingSkillHeaderLinePlain+"\n")
		assert.NotContains(t, stderr.String(), "rerun with --edit-agents")
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
		assert.Contains(t, stderr.String(), missingSkillHeaderLinePlain+"\n")
		assert.NotContains(t, stderr.String(), "rerun with --edit-agents")
	})
}

// Test_init_missing_skill_lists_an_escaping_bound_agent_separately pins the
// third missing-skill row shape (product-vision fix round): a bound
// implementer resolved only through a ".claude" symlinked outside the
// repository is listed with "; outside the repository, edit by hand" —
// distinct from a real project row and from a user-level one — and its
// presence alone (no in-repository row) still keeps the header plain, since
// "--edit-agents" cannot reach it either.
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

	// --dry-run: a real run would also need to create the plugin/skill
	// files under the symlinked ".claude" for the first time, which trips
	// R10's own writability pre-check (Lstat never resolves a symlink at
	// the exact path it is asked to check) — an unrelated concern this
	// test is not proving; the missing-skill report itself is computed
	// identically either way (bound_agent_test.go's own precedent).
	err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--dry-run"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Contains(t, stderr.String(), missingSkillHeaderLinePlain+"\n")
	assert.NotContains(t, stderr.String(), "rerun with --edit-agents")
	assert.Contains(t, stderr.String(), "  .claude/agents/developer.md (implementer; outside the repository, edit by hand)\n")
}

// Test_init_edit_agents_requires_claude_code pins the exit-2 refusal when
// the resolved host is not claude-code, whether explicit or detected.
func Test_init_edit_agents_requires_claude_code(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none", "--edit-agents"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

	require.Error(t, err)
	assert.Equal(t, 2, ExitCode(err))
	assert.Equal(t, "brief init: --edit-agents requires --host claude-code; run 'brief init --host claude-code --edit-agents'\n", stderr.String())

	entries, readErr := os.ReadDir(wd)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

// Test_init_edit_agents_dry_run_and_print pins --dry-run and --print's own
// rendering: the same merged row and an unwritten file under --dry-run, the
// merge header and only the inserted line under --print with no CR
// surviving from a CRLF fixture, and the merge body under --print --json.
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

// Test_init_edit_agents_json pins "--edit-agents --json": a "bound-agent"
// artifact row action "merged", the path listed in "modified", and the
// merged path excluded from "agents_missing_skill".
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

// Test_init_prints_roles_to_add_before_the_missing_skill_block pins R7's
// stderr order (Surface & Copy): the "roles_to_add" block, then the
// missing-skill block, then the next-action line — never the reverse.
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

// Test_init_json_lists_agents_missing_the_skill pins "agents_missing_skill"
// --json's own item shape (S06): exactly the keys role, agent, path
// (absolute) and scope, in that order, values "project"/"user", stderr
// empty — the same fixture Test_init_lists_bound_agents_missing_the_workflow_skill
// uses.
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

// extractFirstObjectKeys decodes doc[field]'s own first array element as an
// ordered list of its own JSON object keys, via json.Decoder's own token
// stream — the one way to observe encoding/json's own field order without
// relying on a Go struct's field order to prove it.
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
