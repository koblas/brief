// This file reaches the unexported run directly to inject a runSeam: the
// rwfs.Mem-backed cases below substitute setup.WithFSRoot (plus
// WithResolveRoot, WithHomeDir and WithWritableCheck) via newMemSetupSeam
// (mem_internal_test.go) so host detection (R8) and the plain install path never
// touch real disk or the developer's own "~/.claude". init's bound-agent
// and missing-workflow-skill tests stay in
// init_bound_agent_internal_test.go: bound_agent.go's own confinedAgentFile
// always reads and writes real agent files through real disk regardless of
// setup.WithFSRoot (internal/setup's own doc.go), so no rwfs.Mem fixture
// can stand in for one. This file also calls missingSkillHeader and
// missingSkillLines directly, unexported: both are pure functions of a
// []setup.MissingSkillAgent, and their own combined four-group rendering
// is pinned against hand-built rows rather than through real agent files
// on disk.

package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_init_without_host_detects_the_host_from_the_tree pins R8's
// detection rule at the cli boundary: nothing present resolves to
// HostNone with the R8 stderr line replacing the next action; a root
// "CLAUDE.md" resolves to claude-code and installs the plugin; under
// --json a detected-none run leaves stderr empty and reports "host":"none".
func Test_init_without_host_detects_the_host_from_the_tree(t *testing.T) {
	t.Run("nothing present detects none", func(t *testing.T) {
		wd := fsAbs("repo")
		mem := newVirtualMem(wd)
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

		require.NoError(t, err)
		assert.Equal(t, "created .brief.yaml\ncreated docs/specifications/\n", stdout.String())
		assert.Equal(t, "brief init: no agent host detected; run 'brief init --host claude-code' to install integration\n", stderr.String())
	})

	t.Run("a root CLAUDE.md detects claude-code", func(t *testing.T) {
		wd := fsAbs("repo")
		mem := newVirtualMem(wd)
		require.NoError(t, mem.WriteFile(memKey(filepath.Join(wd, "CLAUDE.md")), []byte("# hi\n"), 0o600))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "created .claude/skills/brief/.claude-plugin/plugin.json\n")
		assert.Equal(t, "brief init: installed for claude-code (detected CLAUDE.md; use --host none to skip); "+
			"start Claude Code in this directory (or run /reload-plugins in a session already here), "+
			"then 'brief new feature <name>'\n", stderr.String())
	})

	t.Run("a root .claude directory detects claude-code and names it", func(t *testing.T) {
		wd := fsAbs("repo")
		mem := newVirtualMem(wd)
		require.NoError(t, mem.Mkdir(memKey(filepath.Join(wd, ".claude")), 0o755))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

		require.NoError(t, err)
		assert.Equal(t, "brief init: installed for claude-code (detected .claude; use --host none to skip); "+
			"start Claude Code in this directory (or run /reload-plugins in a session already here), "+
			"then 'brief new feature <name>'\n", stderr.String())
	})

	t.Run("an explicit --host claude-code names no detection signal", func(t *testing.T) {
		wd := fsAbs("repo")
		mem := newVirtualMem(wd)
		require.NoError(t, mem.Mkdir(memKey(filepath.Join(wd, ".claude")), 0o755))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

		require.NoError(t, err)
		assert.Equal(t, "brief init: installed for claude-code; start Claude Code in this directory (or run /reload-plugins in a session already here), then 'brief new feature <name>'\n", stderr.String())
	})

	t.Run("detected none under --json leaves stderr empty", func(t *testing.T) {
		wd := fsAbs("repo")
		mem := newVirtualMem(wd)
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--json"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

		require.NoError(t, err)
		assert.Empty(t, stderr.String())
		assert.Contains(t, stdout.String(), `"host":"none"`)
		assert.Contains(t, stdout.String(), `"detected_by":null`)
	})

	t.Run("a detected host reports detected_by under --json", func(t *testing.T) {
		wd := fsAbs("repo")
		mem := newVirtualMem(wd)
		require.NoError(t, mem.Mkdir(memKey(filepath.Join(wd, ".claude")), 0o755))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--json"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

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
		wd := fsAbs("repo")
		mem := newVirtualMem(wd)
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--with-agents"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

		require.Error(t, err)
		assert.Equal(t, 2, ExitCode(err))
		assert.Equal(t, "brief init: --with-agents requires --host claude-code; run 'brief init --host claude-code --with-agents'\n", stderr.String())

		entries, readErr := mem.ReadDir(memKey(wd))
		require.NoError(t, readErr)
		assert.Empty(t, entries)
	})

	t.Run("a detected claude-code root installs the agents", func(t *testing.T) {
		wd := fsAbs("repo")
		mem := newVirtualMem(wd)
		require.NoError(t, mem.Mkdir(memKey(filepath.Join(wd, ".claude")), 0o755))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--with-agents"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "created .claude/skills/brief/agents/planner.md\n")
	})
}

// Test_init_edit_agents_requires_claude_code pins the exit-2 refusal when
// the resolved host is not claude-code, whether explicit or detected. This
// refusal fires before Init ever reads a bound agent file, so it needs no
// real disk.
func Test_init_edit_agents_requires_claude_code(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none", "--edit-agents"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.Error(t, err)
	assert.Equal(t, 2, ExitCode(err))
	assert.Equal(t, "brief init: --edit-agents requires --host claude-code; run 'brief init --host claude-code --edit-agents'\n", stderr.String())

	entries, readErr := mem.ReadDir(memKey(wd))
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

// Test_missing_skill_lines_group_order pins missingSkillHeader and
// missingSkillLines' own combined rendering across the four-group order
// (Surface & Copy) every Reach combination must sort by: fixable, then a
// project row setup.planBoundAgent itself cannot reach — not regular, an
// uneditable "skills:" shape — combined, then an escaping project row,
// then every user-level row. The group-1/2, group-2/3 and group-3/4 cases
// below each pin one adjacent transition; since the render order is a
// fixed sequence of passes, those three adjacent transitions together
// already pin every non-adjacent one too (a group-1/3 case adds nothing a
// group-1/2 and group-2/3 pair does not already catch — confirmed by
// swapping the fixable and escaped passes, which reddens both). Within the
// combined not-regular/uneditable group, rows keep agents' own relative
// order rather than being resorted by Reach — the two same-group cases
// below each put one of each Reach in both role orders, so a rewrite that
// splits the group into two Reach-ordered passes reddens exactly one of
// the two, whichever order the split renders first. wd is a fixed string;
// missingSkillHeader and missingSkillLines never touch the filesystem, so
// every Reach combination can be pinned directly without constructing real
// agent files. headerSuffixed and headerPlain are missingSkillHeaderLine
// and missingSkillHeaderLinePlain's own unprefixed counterparts (both
// defined in init_bound_agent_internal_test.go, same package) — missingSkillHeader itself
// renders without the "brief init: " prefix runInit prepends — so a direct
// call can be compared against them as-is.
func Test_missing_skill_lines_group_order(t *testing.T) {
	const wd = "/repo"

	headerSuffixed := strings.TrimPrefix(missingSkillHeaderLine, "brief init: ")
	headerPlain := strings.TrimPrefix(missingSkillHeaderLinePlain, "brief init: ")

	renderBlock := func(editAgents bool, agents []setup.MissingSkillAgent) string {
		lines := append([]string{missingSkillHeader(editAgents, agents)}, missingSkillLines(wd, agents)...)

		return strings.Join(lines, "\n") + "\n"
	}

	planner := func(reach setup.MissingSkillReach) setup.MissingSkillAgent {
		return setup.MissingSkillAgent{Role: "planner", Agent: "planner", Path: filepath.Join(wd, ".claude", "agents", "planner.md"), Scope: agentfile.ScopeProject, Reach: reach}
	}
	implementer := func(reach setup.MissingSkillReach) setup.MissingSkillAgent {
		return setup.MissingSkillAgent{Role: "implementer", Agent: "developer", Path: filepath.Join(wd, ".claude", "agents", "developer.md"), Scope: agentfile.ScopeProject, Reach: reach}
	}
	userPlanner := setup.MissingSkillAgent{Role: "planner", Agent: "planner", Scope: agentfile.ScopeUser, ScopeRelPath: ".claude/agents/planner.md", Reach: setup.ReachNone}

	cases := []struct {
		name   string
		agents []setup.MissingSkillAgent
		want   string
	}{
		{
			name:   "escaping implementer, user-level planner: group 3 before group 4, header plain",
			agents: []setup.MissingSkillAgent{userPlanner, implementer(setup.ReachEscaped)},
			want: headerPlain + "\n" +
				"  .claude/agents/developer.md (implementer; outside the repository, edit by hand)\n" +
				"  ~/.claude/agents/planner.md (planner; user-level, edit by hand)\n",
		},
		{
			name:   "in-repo unfixable planner, fixable implementer: group 1 before group 2, header suffixed",
			agents: []setup.MissingSkillAgent{planner(setup.ReachUneditable), implementer(setup.ReachFixable)},
			want: headerSuffixed + "\n" +
				"  .claude/agents/developer.md (implementer)\n" +
				"  .claude/agents/planner.md (planner; skills: is not a list brief can edit, edit by hand)\n",
		},
		{
			name:   "in-repo unfixable planner, escaping implementer: group 2 before group 3, header plain",
			agents: []setup.MissingSkillAgent{planner(setup.ReachNotRegular), implementer(setup.ReachEscaped)},
			want: headerPlain + "\n" +
				"  .claude/agents/planner.md (planner; not a regular file, edit by hand)\n" +
				"  .claude/agents/developer.md (implementer; outside the repository, edit by hand)\n",
		},
		{
			name:   "not-regular planner, uneditable implementer: both in group 2, role order kept, not resorted by Reach",
			agents: []setup.MissingSkillAgent{planner(setup.ReachNotRegular), implementer(setup.ReachUneditable)},
			want: headerPlain + "\n" +
				"  .claude/agents/planner.md (planner; not a regular file, edit by hand)\n" +
				"  .claude/agents/developer.md (implementer; skills: is not a list brief can edit, edit by hand)\n",
		},
		{
			name:   "uneditable planner, not-regular implementer: both in group 2, role order kept, not resorted by Reach",
			agents: []setup.MissingSkillAgent{planner(setup.ReachUneditable), implementer(setup.ReachNotRegular)},
			want: headerPlain + "\n" +
				"  .claude/agents/planner.md (planner; skills: is not a list brief can edit, edit by hand)\n" +
				"  .claude/agents/developer.md (implementer; not a regular file, edit by hand)\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, renderBlock(false, c.agents))
		})
	}
}

// Test_missing_skill_lines_treats_a_scope_project_row_carrying_reach_none_as_fixable
// pins missingSkillFixable's own defensive fallback: setup's own
// agentsMissingSkill can never actually hand cli a ScopeProject row
// carrying setup.ReachNone — the value every ScopeUser row legitimately
// carries, and the zero value any other broken invariant would leave
// behind too — since an internal error aborts the whole report rather
// than emitting a row missingSkillReach never classified. missingSkillLines
// must not silently drop such a row if that invariant is ever broken —
// every other group's own filter excludes it too, so a row landing in none
// of them vanishes from the report entirely. Rendering it in the fixable
// group reuses that group's own existing line format, no new copy. The
// control proves the fallback is scoped to ScopeProject alone: the
// identical Reach value on a ScopeUser row — setup's own legitimate case —
// still renders user-level, not fixable.
func Test_missing_skill_lines_treats_a_scope_project_row_carrying_reach_none_as_fixable(t *testing.T) {
	const wd = "/repo"

	t.Run("ScopeProject carrying ReachNone renders fixable, not dropped", func(t *testing.T) {
		agents := []setup.MissingSkillAgent{
			{Role: "implementer", Agent: "developer", Path: filepath.Join(wd, ".claude", "agents", "developer.md"), Scope: agentfile.ScopeProject, Reach: setup.MissingSkillReach("")},
		}

		assert.Equal(t, []string{"  .claude/agents/developer.md (implementer)"}, missingSkillLines(wd, agents))
	})

	t.Run("control: the identical Reach on a ScopeUser row still renders user-level", func(t *testing.T) {
		agents := []setup.MissingSkillAgent{
			{Role: "implementer", Agent: "developer", Scope: agentfile.ScopeUser, ScopeRelPath: ".claude/agents/developer.md", Reach: setup.ReachNone},
		}

		assert.Equal(t, []string{"  ~/.claude/agents/developer.md (implementer; user-level, edit by hand)"}, missingSkillLines(wd, agents))
	})
}
