// White-box: reaches run directly to inject a runSeam substituting rwfs.Mem
// (newMemSetupSeam) so host detection and the plain install path never touch
// real disk. Bound-agent and missing-workflow-skill tests needing a real
// agent file stay in init_bound_agent_internal_test.go.

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

// Pins the four-group render order every Reach combination must sort by:
// fixable, then not-regular/uneditable combined, then escaping, then
// user-level. Within the combined group, rows keep their relative order
// rather than being resorted by Reach.
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

// A ScopeProject row should never carry ReachNone in production, but
// missingSkillLines must not silently drop one if that invariant breaks; the
// control proves the fallback is scoped to ScopeProject alone.
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

// A config that decodes without violation but differs from the shipped
// render is reported "kept", and the feature root it names is created.
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

// Control arm for the refusal test above: proves the directory-listing
// probe is capable of catching a write, not merely one that sees none.
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

func Test_init_force_rewrites_an_existing_config_from_defaults(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	require.NoError(t, mem.WriteFile(memKey(filepath.Join(wd, ".brief.yaml")), []byte("handoff-cap-lines: 0\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none", "--force"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Equal(t, "created .brief.yaml (rewritten from defaults)\ncreated docs/specifications/\n", stdout.String())
}

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

func Test_init_refuses_an_unknown_host(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "bogus"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	assert.Equal(t, 2, ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief init: unknown host "bogus"; expected one of: claude-code, none; run 'brief init --print' to wire it by hand`+"\n", stderr.String())
}

func Test_init_refuses_a_stray_positional_argument(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), fsAbs("repo"), []string{"init", "extra"}, nil, &stdout, &stderr, noBuildInfo)

	assert.Equal(t, 2, ExitCode(err))
	assert.Contains(t, stderr.String(), "too many arguments")
}

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

func Test_no_hook_with_host_none_is_accepted_and_changes_nothing(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none", "--no-hook"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Equal(t, "created .brief.yaml\ncreated docs/specifications/\n", stdout.String())
}

// A second run reports every row "unchanged" except the finish skill,
// edited between runs, reported "kept (edited locally)" — even under
// --force, which only ever rewrites the config.
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

// Every other artifact already converged (unchanged); only the CLAUDE.md
// block needs replacing, but the next-action line must still say
// "installed", not "already installed; nothing changed".
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

// A repository already carrying every other claude-code artifact but no
// brief-workflow skill reruns to print exactly one "created" row.
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

// A planner file holding older bytes is reported "merged … (updated)" and
// rewritten to today's render.
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
