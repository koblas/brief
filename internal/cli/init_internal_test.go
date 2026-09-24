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
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/koblas/brief/internal/platform/artifact"
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

// Test_init_reports_created_then_unchanged pins R2/R3/R11's happy path: a
// fresh repository's first "brief init --host none" reports both artifacts
// created, with the feature root's row carrying a trailing "/", and the
// "installed" next-action line on stderr; a second, identical run reports
// both "unchanged" and the "already installed" next-action line, exit 0.
func Test_init_reports_created_then_unchanged(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Equal(t, "created .brief.yaml\ncreated docs/specifications/\n", stdout.String())
	assert.Equal(t, "brief init: installed config and feature root; run 'brief new feature <name>'\n", stderr.String())

	body, readErr := mem.ReadFile(memKey(filepath.Join(wd, ".brief.yaml")))
	require.NoError(t, readErr)
	assert.Equal(t, artifact.ConfigFile(), body)

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Equal(t, "unchanged .brief.yaml\nunchanged docs/specifications/\n", stdout.String())
	assert.Equal(t, "brief init: already installed; nothing changed\n", stderr.String())
}

// Test_init_keeps_a_valid_existing_config_and_reports_it pins R6's "edited
// locally" branch: a config that decodes without violation but differs
// from the shipped render is reported "kept", and the feature root it
// names — not Default()'s — is what gets created.
func Test_init_keeps_a_valid_existing_config_and_reports_it(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	require.NoError(t, mem.WriteFile(memKey(filepath.Join(wd, ".brief.yaml")), []byte("feature-directory: specs\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Equal(t, "kept .brief.yaml (edited locally)\ncreated specs/\n", stdout.String())
	assert.Equal(t, "brief init: installed config and feature root; run 'brief new feature <name>'\n", stderr.String())
}

// Test_init_refuses_an_unparseable_config_leaving_the_tree_untouched pins
// R3's refusal branch: exit 1, a stderr line naming the "(no files
// changed)" promise, and the working directory carrying exactly the one
// file that was already there.
func Test_init_refuses_an_unparseable_config_leaving_the_tree_untouched(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	require.NoError(t, mem.WriteFile(memKey(filepath.Join(wd, ".brief.yaml")), []byte("feature-directory: [unterminated\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "(no files changed)")
	assert.Contains(t, stderr.String(), "brief init --force")

	entries, readErr := mem.ReadDir(memKey(wd))
	require.NoError(t, readErr)
	assert.Len(t, entries, 1)
}

// Test_the_directory_probe_sees_new_entries_on_a_successful_init is the
// control arm for the refusal test above: the identical directory-listing
// probe, against a fresh repository instead of the refusing fixture, does
// grow a second entry — proving the probe is capable of catching a write,
// not merely one that happens to see none.
func Test_the_directory_probe_sees_new_entries_on_a_successful_init(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)

	entries, readErr := mem.ReadDir(memKey(wd))
	require.NoError(t, readErr)
	assert.Len(t, entries, 2)
}

// Test_init_refuses_an_invalid_config_value_naming_the_key_value_and_force_fix
// pins the STATE.md open debt this scenario closes: the stderr line names
// the offending key, its value, and 'brief init --force' as the fix.
func Test_init_refuses_an_invalid_config_value_naming_the_key_value_and_force_fix(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	require.NoError(t, mem.WriteFile(memKey(filepath.Join(wd, ".brief.yaml")), []byte("handoff-cap-lines: 0\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	assert.Equal(t, 1, ExitCode(err))
	assert.Contains(t, stderr.String(), "handoff-cap-lines is 0")
	assert.Contains(t, stderr.String(), "brief init --force")
}

// Test_init_force_rewrites_an_existing_config_from_defaults pins --force's
// own report shape: "created", detail "rewritten from defaults", against
// the same fixture the refusal tests above refuse on.
func Test_init_force_rewrites_an_existing_config_from_defaults(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	require.NoError(t, mem.WriteFile(memKey(filepath.Join(wd, ".brief.yaml")), []byte("handoff-cap-lines: 0\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none", "--force"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Equal(t, "created .brief.yaml (rewritten from defaults)\ncreated docs/specifications/\n", stdout.String())
}

// Test_init_dry_run_prints_the_plan_and_writes_nothing pins R9: the same
// rows a real run would print, the dry-run stderr line, and an unchanged
// working directory.
func Test_init_dry_run_prints_the_plan_and_writes_nothing(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none", "--dry-run"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Equal(t, "created .brief.yaml\ncreated docs/specifications/\n", stdout.String())
	assert.Equal(t, "brief init: dry run, no files changed; rerun without --dry-run to apply\n", stderr.String())

	entries, readErr := mem.ReadDir(memKey(wd))
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

// Test_init_refuses_an_unknown_host pins R8's usage-error branch: exit 2,
// naming the given value, the accepted list, and --print as the by-hand
// route. "bogus" is used rather than "claude-code" — S03 does not accept
// it yet.
func Test_init_refuses_an_unknown_host(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "bogus"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	assert.Equal(t, 2, ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief init: unknown host "bogus"; expected one of: claude-code, none; run 'brief init --print' to wire it by hand`+"\n", stderr.String())
}

// Test_init_refuses_a_stray_positional_argument pins the usage-error
// branch for an argument init takes none of — checked before wd is ever
// read, so it needs no fixture at all.
func Test_init_refuses_a_stray_positional_argument(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), fsAbs("repo"), []string{"init", "extra"}, nil, &stdout, &stderr, noBuildInfo)

	assert.Equal(t, 2, ExitCode(err))
	assert.Contains(t, stderr.String(), "too many arguments")
}

// Test_init_for_claude_code_installs_the_plugin_and_says_where_to_start_claude_code
// pins the user-visible contract for a fresh repository: all six rows in
// order, the claude-code next-action line naming "this directory" since
// the install root is wd itself, and every plugin file's bytes equal to
// its own artifact render.
func Test_init_for_claude_code_installs_the_plugin_and_says_where_to_start_claude_code(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Equal(t, ""+
		"created .brief.yaml\n"+
		"created docs/specifications/\n"+
		"created .claude/skills/brief/.claude-plugin/plugin.json\n"+
		"created .claude/skills/brief/skills/start/SKILL.md\n"+
		"created .claude/skills/brief/skills/finish/SKILL.md\n"+
		"created .claude/skills/brief/hooks/hooks.json\n"+
		"created .claude/skills/brief-workflow/SKILL.md\n"+
		"created CLAUDE.md\n", stdout.String())
	assert.Equal(t, "brief init: installed for claude-code; start Claude Code in this directory (or run /reload-plugins in a session already here), then 'brief new feature <name>'\n", stderr.String())

	manifest, err2 := mem.ReadFile(memKey(filepath.Join(wd, ".claude", "skills", "brief", ".claude-plugin", "plugin.json")))
	require.NoError(t, err2)
	assert.Equal(t, artifact.PluginManifest(), manifest)
}

// Test_init_from_a_subdirectory_names_the_install_root_in_the_next_action
// pins the root ≠ wd form: run from a child directory of a repository
// already configured at the parent, the next-action line names the
// parent, relative to wd, in both places the root=wd control arm above
// says "this directory"/"here".
func Test_init_from_a_subdirectory_names_the_install_root_in_the_next_action(t *testing.T) {
	parent := fsAbs("repo")
	mem := newVirtualMem(parent)
	require.NoError(t, mem.WriteFile(memKey(filepath.Join(parent, ".brief.yaml")), []byte("feature-directory: specs\n"), 0o600))
	child := filepath.Join(parent, "child")
	require.NoError(t, mem.Mkdir(memKey(child), 0o755))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), child, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Equal(t, "brief init: installed for claude-code in ..; start Claude Code in .. (or run /reload-plugins in a session already there), then 'brief new feature <name>'\n", stderr.String())
}

// Test_init_no_hook_omits_the_hook_row pins --no-hook: the same five rows
// minus hooks.json, and no hooks.json file on disk.
func Test_init_no_hook_omits_the_hook_row(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--no-hook"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Equal(t, ""+
		"created .brief.yaml\n"+
		"created docs/specifications/\n"+
		"created .claude/skills/brief/.claude-plugin/plugin.json\n"+
		"created .claude/skills/brief/skills/start/SKILL.md\n"+
		"created .claude/skills/brief/skills/finish/SKILL.md\n"+
		"created .claude/skills/brief-workflow/SKILL.md\n"+
		"created CLAUDE.md\n", stdout.String())

	_, statErr := mem.Stat(memKey(filepath.Join(wd, ".claude", "skills", "brief", "hooks", "hooks.json")))
	assert.ErrorIs(t, statErr, fs.ErrNotExist)
}

// Test_no_hook_with_host_none_is_accepted_and_changes_nothing pins
// --no-hook's own no-op under --host none: no plugin was ever planned, so
// --no-hook has nothing to omit, and init still installs just the config
// and feature root.
func Test_no_hook_with_host_none_is_accepted_and_changes_nothing(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none", "--no-hook"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Equal(t, "created .brief.yaml\ncreated docs/specifications/\n", stdout.String())
}

// Test_init_rerunning_for_claude_code_reports_unchanged_and_edited_files_kept
// pins convergence and "edited locally" together: a second run reports
// every row "unchanged" except the finish skill, edited between runs,
// reported "kept (edited locally)" — even under --force, which only ever
// rewrites the config.
func Test_init_rerunning_for_claude_code_reports_unchanged_and_edited_files_kept(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer
	err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	finish := filepath.Join(wd, ".claude", "skills", "brief", "skills", "finish", "SKILL.md")
	require.NoError(t, mem.WriteFile(memKey(finish), []byte("---\nedited: true\n---\n"), 0o600))

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"init", "--host", "claude-code", "--force"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"unchanged .brief.yaml\n"+
		"unchanged docs/specifications/\n"+
		"unchanged .claude/skills/brief/.claude-plugin/plugin.json\n"+
		"unchanged .claude/skills/brief/skills/start/SKILL.md\n"+
		"kept .claude/skills/brief/skills/finish/SKILL.md (edited locally)\n"+
		"unchanged .claude/skills/brief/hooks/hooks.json\n"+
		"unchanged .claude/skills/brief-workflow/SKILL.md\n"+
		"unchanged CLAUDE.md\n", stdout.String())
	assert.Equal(t, "brief init: already installed; nothing changed\n", stderr.String())
}

// Test_init_merging_only_the_snippet_reports_installed_not_nothing_changed
// pins the trap a merge-only run is exposed to: every other artifact
// already converged (unchanged), only the CLAUDE.md block needs replacing
// (it was brief-written for a different feature directory) — the
// next-action line must still say "installed", not "already installed;
// nothing changed", so initNextAction has to treat ActionMerged as a
// change alongside ActionCreated.
func Test_init_merging_only_the_snippet_reports_installed_not_nothing_changed(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer
	err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	require.NoError(t, mem.WriteFile(memKey(filepath.Join(wd, "CLAUDE.md")), artifact.SnippetBlock("elsewhere"), 0o600))

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"unchanged .brief.yaml\n"+
		"unchanged docs/specifications/\n"+
		"unchanged .claude/skills/brief/.claude-plugin/plugin.json\n"+
		"unchanged .claude/skills/brief/skills/start/SKILL.md\n"+
		"unchanged .claude/skills/brief/skills/finish/SKILL.md\n"+
		"unchanged .claude/skills/brief/hooks/hooks.json\n"+
		"unchanged .claude/skills/brief-workflow/SKILL.md\n"+
		"merged CLAUDE.md (block updated)\n", stdout.String())
	assert.Equal(t, "brief init: installed for claude-code; start Claude Code in this directory (or run /reload-plugins in a session already here), then 'brief new feature <name>'\n", stderr.String())

	body, readErr := mem.ReadFile(memKey(filepath.Join(wd, "CLAUDE.md")))
	require.NoError(t, readErr)
	assert.Equal(t, artifact.SnippetBlock("docs/specifications"), body)
}

// Test_init_over_an_install_without_the_workflow_skill_creates_only_it pins
// the upgrade path every current adopter hits: a repository already
// carrying every other claude-code artifact but no brief-workflow skill
// (the pre-S01 shape) reruns to print exactly one "created" row, and stderr
// still reads the ordinary "installed for claude-code; …" line —
// initNextAction is kind-generic, not skill-specific.
func Test_init_over_an_install_without_the_workflow_skill_creates_only_it(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer
	err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)
	require.NoError(t, mem.Remove(memKey(filepath.Join(wd, ".claude", "skills", "brief-workflow", "SKILL.md"))))

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Equal(t, ""+
		"unchanged .brief.yaml\n"+
		"unchanged docs/specifications/\n"+
		"unchanged .claude/skills/brief/.claude-plugin/plugin.json\n"+
		"unchanged .claude/skills/brief/skills/start/SKILL.md\n"+
		"unchanged .claude/skills/brief/skills/finish/SKILL.md\n"+
		"unchanged .claude/skills/brief/hooks/hooks.json\n"+
		"created .claude/skills/brief-workflow/SKILL.md\n"+
		"unchanged CLAUDE.md\n", stdout.String())
	assert.Equal(t, "brief init: installed for claude-code; start Claude Code in this directory (or run /reload-plugins in a session already here), then 'brief new feature <name>'\n", stderr.String())
}

// Test_init_with_agents_installs_three_agents_and_binds_roles pins the
// fresh-repository happy path for --with-agents: ten rows in order, the
// three agents created under "agents/", the config's own bytes equal
// artifact.ConfigFileWithRoles(), and the ordinary "installed for
// claude-code" next-action line — no roles hint, since this run authored
// the bindings itself.
func Test_init_with_agents_installs_three_agents_and_binds_roles(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--with-agents"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Equal(t, ""+
		"created .brief.yaml\n"+
		"created docs/specifications/\n"+
		"created .claude/skills/brief/.claude-plugin/plugin.json\n"+
		"created .claude/skills/brief/skills/start/SKILL.md\n"+
		"created .claude/skills/brief/skills/finish/SKILL.md\n"+
		"created .claude/skills/brief/hooks/hooks.json\n"+
		"created .claude/skills/brief-workflow/SKILL.md\n"+
		"created .claude/skills/brief/agents/planner.md\n"+
		"created .claude/skills/brief/agents/implementer.md\n"+
		"created .claude/skills/brief/agents/reviewer.md\n"+
		"created CLAUDE.md\n", stdout.String())
	assert.Equal(t, "brief init: installed for claude-code; start Claude Code in this directory (or run /reload-plugins in a session already here), then 'brief new feature <name>'\n", stderr.String())

	body, readErr := mem.ReadFile(memKey(filepath.Join(wd, ".brief.yaml")))
	require.NoError(t, readErr)
	assert.Equal(t, artifact.ConfigFileWithRoles(), body)
}

// Test_init_with_agents_reports_an_older_agent_as_merged_updated pins R11's
// stdout row for Rule 6's upgrade path: a planner file holding the
// pre-SCENARIO-02 bytes is reported "merged … (updated)", exit 0, and its
// bytes are rewritten to today's own render.
func Test_init_with_agents_reports_an_older_agent_as_merged_updated(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--with-agents"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	plannerPath := filepath.Join(wd, ".claude", "skills", "brief", "agents", "planner.md")
	older := []byte("---\nname: planner\ndescription: Turn a feature's specification into ordered scenario " +
		"plans.\ntools: Read, Grep, Glob, Bash, Edit, Write\n---\n\nTurn the feature's " +
		"specification into ordered scenario plans: run `brief new step <feature>` for the next " +
		"scenario, then fill its plan file. Never write production or test code.\n")
	require.NoError(t, mem.WriteFile(memKey(plannerPath), older, 0o600))

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"init", "--host", "claude-code", "--with-agents"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Contains(t, stdout.String(), "merged .claude/skills/brief/agents/planner.md (updated)\n")

	body, readErr := mem.ReadFile(memKey(plannerPath))
	require.NoError(t, readErr)
	assert.Equal(t, artifact.AgentPlanner(), body)
}

// Test_init_with_agents_over_an_existing_config_prints_the_roles_lines_to_add
// pins R7's stderr hint, exact copy: the config is never edited, and the
// hint block — "was not edited" line, then "roles:" and the three bare
// "  <role>: brief:<role>" lines, no "brief init: " prefix on those since
// they are meant to be pasted verbatim into .brief.yaml — lands before the
// ordinary next-action line.
func Test_init_with_agents_over_an_existing_config_prints_the_roles_lines_to_add(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configKey := memKey(filepath.Join(wd, ".brief.yaml"))
	original := []byte("feature-directory: specs\n")
	require.NoError(t, mem.WriteFile(configKey, original, 0o600))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--with-agents"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Equal(t, ""+
		"brief init: .brief.yaml was not edited; to bind brief's agents, add these lines to it:\n"+
		"roles:\n"+
		"  planner: brief:planner\n"+
		"  implementer: brief:implementer\n"+
		"  reviewer: brief:reviewer\n"+
		"brief init: installed for claude-code; start Claude Code in this directory (or run /reload-plugins in a session already here), then 'brief new feature <name>'\n", stderr.String())

	body, readErr := mem.ReadFile(configKey)
	require.NoError(t, readErr)
	assert.Equal(t, original, body)
}

// Test_init_with_agents_and_host_none_is_a_usage_error pins the
// flag-combination rule (checked on the resolved host): an explicit
// "--host none" alongside "--with-agents" refuses, exit 2, tree unchanged.
// A bare "--with-agents" (host resolved by detection) is
// Test_init_with_agents_follows_the_detected_host, above.
func Test_init_with_agents_and_host_none_is_a_usage_error(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none", "--with-agents"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	assert.Equal(t, 2, ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief init: --with-agents requires --host claude-code; run 'brief init --host claude-code --with-agents'`+"\n", stderr.String())

	entries, readErr := mem.ReadDir(memKey(wd))
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

// Test_init_keeps_a_plugin_path_that_is_a_directory_instead_of_a_file pins
// the "not a regular file" row: a directory already occupying the
// manifest's own path is kept, never followed, never written.
func Test_init_keeps_a_plugin_path_that_is_a_directory_instead_of_a_file(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	manifest := filepath.Join(wd, ".claude", "skills", "brief", ".claude-plugin", "plugin.json")
	require.NoError(t, mem.MkdirAll(memKey(manifest), 0o755))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "kept .claude/skills/brief/.claude-plugin/plugin.json (not a regular file)\n")
}

// Test_init_dry_run_with_print_is_a_usage_error pins R9's own
// flag-combination rule, checked before setup ever runs, in either flag
// order: exit 2, the exact stderr line, stdout empty, tree unchanged.
func Test_init_dry_run_with_print_is_a_usage_error(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "--dry-run before --print", args: []string{"init", "--dry-run", "--print"}},
		{name: "--print before --dry-run", args: []string{"init", "--print", "--dry-run"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := fsAbs("repo")
			mem := newVirtualMem(wd)
			var stdout, stderr bytes.Buffer

			err := run(t.Context(), wd, c.args, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

			assert.Equal(t, 2, ExitCode(err))
			assert.Empty(t, stdout.String())
			assert.Equal(t, "brief init: --dry-run and --print cannot be combined; run 'brief init --print'\n", stderr.String())

			entries, readErr := mem.ReadDir(memKey(wd))
			require.NoError(t, readErr)
			assert.Empty(t, entries)
		})
	}
}

// Test_init_print_writes_bodies_to_stdout_and_nothing_to_disk pins R9's
// own text-mode shape: a fresh claude-code install prints one
// "# <path> (create)" header plus body per artifact, blank-line
// separated, none after the last, the exact stderr line, exit 0, and an
// unchanged tree; a CLAUDE.md merge case reports "(merge)"; an
// already-installed tree reports empty stdout and the "already installed"
// stderr line instead.
func Test_init_print_writes_bodies_to_stdout_and_nothing_to_disk(t *testing.T) {
	t.Run("fresh install", func(t *testing.T) {
		wd := fsAbs("repo")
		mem := newVirtualMem(wd)
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--print"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

		require.NoError(t, err)
		assert.Equal(t, 0, ExitCode(err))
		assert.Equal(t, "brief init: printed only, no files changed; apply the output above by hand, or rerun without --print\n", stderr.String())

		want := "# .brief.yaml (create)\n" + string(artifact.ConfigFile()) +
			"\n# .claude/skills/brief/.claude-plugin/plugin.json (create)\n" + string(artifact.PluginManifest()) +
			"\n# .claude/skills/brief/skills/start/SKILL.md (create)\n" + string(artifact.SkillStart()) +
			"\n# .claude/skills/brief/skills/finish/SKILL.md (create)\n" + string(artifact.SkillFinish()) +
			"\n# .claude/skills/brief/hooks/hooks.json (create)\n" + string(artifact.ClaudeHooks()) +
			"\n# .claude/skills/brief-workflow/SKILL.md (create)\n" + string(artifact.SkillWorkflow()) +
			"\n# CLAUDE.md (create)\n" + string(artifact.SnippetBlock("docs/specifications")) + "\n"
		assert.Equal(t, want, stdout.String())

		entries, readErr := mem.ReadDir(memKey(wd))
		require.NoError(t, readErr)
		assert.Empty(t, entries)
	})

	t.Run("merges into an existing CLAUDE.md", func(t *testing.T) {
		wd := fsAbs("repo")
		mem := newVirtualMem(wd)
		require.NoError(t, mem.WriteFile(memKey(filepath.Join(wd, "CLAUDE.md")), []byte("# hello\n"), 0o600))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--print"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "# CLAUDE.md (merge)\n")

		body, readErr := mem.ReadFile(memKey(filepath.Join(wd, "CLAUDE.md")))
		require.NoError(t, readErr)
		assert.Equal(t, []byte("# hello\n"), body)
	})

	t.Run("already installed prints nothing pending", func(t *testing.T) {
		wd := fsAbs("repo")
		mem := newVirtualMem(wd)
		seam := newMemSetupSeam(mem)
		var initStdout, initStderr bytes.Buffer
		require.NoError(t, run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &initStdout, &initStderr, noBuildInfo, seam))

		var stdout, stderr bytes.Buffer
		err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--print"}, nil, &stdout, &stderr, noBuildInfo, seam)

		require.NoError(t, err)
		assert.Empty(t, stdout.String())
		assert.Equal(t, "brief init: already installed; nothing changed\n", stderr.String())
	})

	t.Run("a non-regular CLAUDE.md still prints the block to add by hand", func(t *testing.T) {
		wd := fsAbs("repo")
		mem := newVirtualMem(wd)
		seam := newMemSetupSeam(mem)
		var initStdout, initStderr bytes.Buffer
		require.NoError(t, run(t.Context(), wd, []string{"init", "--host", "claude-code", "--no-hook"}, nil, &initStdout, &initStderr, noBuildInfo, seam))
		require.NoError(t, mem.Remove(memKey(filepath.Join(wd, "CLAUDE.md"))))
		require.NoError(t, mem.Mkdir(memKey(filepath.Join(wd, "CLAUDE.md")), 0o755))

		var stdout, stderr bytes.Buffer
		err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--no-hook", "--print"}, nil, &stdout, &stderr, noBuildInfo, seam)

		require.NoError(t, err)
		assert.Equal(t, "# CLAUDE.md (merge)\n"+string(artifact.SnippetBlock("docs/specifications"))+"\n", stdout.String())
		assert.Equal(t, "brief init: printed only, no files changed; apply the output above by hand, or rerun without --print\n", stderr.String())
	})
}
