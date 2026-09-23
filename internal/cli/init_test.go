package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/cli"
	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_init_reports_created_then_unchanged pins R2/R3/R11's happy path: a
// fresh repository's first "brief init --host none" reports both artifacts
// created, with the feature root's row carrying a trailing "/", and the
// "installed" next-action line on stderr; a second, identical run reports
// both "unchanged" and the "already installed" next-action line, exit 0.
func Test_init_reports_created_then_unchanged(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Equal(t, "created .brief.yaml\ncreated docs/specifications/\n", stdout.String())
	assert.Equal(t, "brief init: installed config and feature root; run 'brief new feature <name>'\n", stderr.String())

	body, readErr := os.ReadFile(filepath.Join(wd, ".brief.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, artifact.ConfigFile(), body)

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
	assert.Equal(t, "unchanged .brief.yaml\nunchanged docs/specifications/\n", stdout.String())
	assert.Equal(t, "brief init: already installed; nothing changed\n", stderr.String())
}

// Test_init_keeps_a_valid_existing_config_and_reports_it pins R6's "edited
// locally" branch at the CLI boundary: a config that decodes without
// violation but differs from the shipped render is reported "kept", and
// the feature root it names — not Default()'s — is what gets created.
func Test_init_keeps_a_valid_existing_config_and_reports_it(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("feature-directory: specs\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "kept .brief.yaml (edited locally)\ncreated specs/\n", stdout.String())
	assert.Equal(t, "brief init: installed config and feature root; run 'brief new feature <name>'\n", stderr.String())
}

// Test_init_refuses_an_unparseable_config_leaving_the_tree_untouched pins
// R3's refusal branch: exit 1, a stderr line naming the "(no files
// changed)" promise, and the working directory carrying exactly the one
// file that was already there.
func Test_init_refuses_an_unparseable_config_leaving_the_tree_untouched(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("feature-directory: [unterminated\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "(no files changed)")
	assert.Contains(t, stderr.String(), "brief init --force")

	entries, readErr := os.ReadDir(wd)
	require.NoError(t, readErr)
	assert.Len(t, entries, 1)
}

// Test_the_directory_probe_sees_new_entries_on_a_successful_init is the
// control arm for the refusal test above: the identical os.ReadDir probe,
// against a fresh repository instead of the refusing fixture, does grow a
// second entry — proving the probe is capable of catching a write, not
// merely one that happens to see none.
func Test_the_directory_probe_sees_new_entries_on_a_successful_init(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)

	require.NoError(t, err)

	entries, readErr := os.ReadDir(wd)
	require.NoError(t, readErr)
	assert.Len(t, entries, 2)
}

// Test_init_refuses_an_invalid_config_value_naming_the_key_value_and_force_fix
// pins the STATE.md open debt this scenario closes at the CLI boundary:
// the stderr line names the offending key, its value, and 'brief init
// --force' as the fix.
func Test_init_refuses_an_invalid_config_value_naming_the_key_value_and_force_fix(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("handoff-cap-lines: 0\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Contains(t, stderr.String(), "handoff-cap-lines is 0")
	assert.Contains(t, stderr.String(), "brief init --force")
}

// Test_init_force_rewrites_an_existing_config_from_defaults pins --force's
// own report shape: "created", detail "rewritten from defaults", against
// the same fixture the refusal tests above refuse on.
func Test_init_force_rewrites_an_existing_config_from_defaults(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("handoff-cap-lines: 0\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--force"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "created .brief.yaml (rewritten from defaults)\ncreated docs/specifications/\n", stdout.String())
}

// Test_init_dry_run_prints_the_plan_and_writes_nothing pins R9: the same
// rows a real run would print, the dry-run stderr line, and an unchanged
// working directory.
func Test_init_dry_run_prints_the_plan_and_writes_nothing(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--dry-run"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "created .brief.yaml\ncreated docs/specifications/\n", stdout.String())
	assert.Equal(t, "brief init: dry run, no files changed; rerun without --dry-run to apply\n", stderr.String())

	entries, readErr := os.ReadDir(wd)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

// Test_init_refuses_an_unknown_host pins R8's usage-error branch: exit 2,
// naming the given value, the accepted list, and --print as the by-hand
// route. "bogus" is used rather than "claude-code" — S03 does not accept
// it yet.
func Test_init_refuses_an_unknown_host(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "bogus"}, nil, &stdout, &stderr)

	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief init: unknown host "bogus"; expected one of: claude-code, none; run 'brief init --print' to wire it by hand`+"\n", stderr.String())
}

// Test_init_refuses_a_stray_positional_argument pins the usage-error
// branch for an argument init takes none of.
func Test_init_refuses_a_stray_positional_argument(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "extra"}, nil, &stdout, &stderr)

	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Contains(t, stderr.String(), "too many arguments")
}

// Test_init_for_claude_code_installs_the_plugin_and_says_where_to_start_claude_code
// pins the user-visible contract for a fresh repository: all six rows in
// order, the claude-code next-action line naming "this directory" since
// the install root is wd itself, and every plugin file's bytes on disk
// equal to its own artifact render.
func Test_init_for_claude_code_installs_the_plugin_and_says_where_to_start_claude_code(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
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

	manifest, err2 := os.ReadFile(filepath.Join(wd, ".claude", "skills", "brief", ".claude-plugin", "plugin.json"))
	require.NoError(t, err2)
	assert.Equal(t, artifact.PluginManifest(), manifest)
}

// Test_init_from_a_subdirectory_names_the_install_root_in_the_next_action
// pins the root ≠ wd form: run from a child directory of a repository
// already configured at the parent, the next-action line names the
// parent, relative to wd, in both places the root=wd control arm above
// says "this directory"/"here".
func Test_init_from_a_subdirectory_names_the_install_root_in_the_next_action(t *testing.T) {
	parent := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(parent, ".brief.yaml"), []byte("feature-directory: specs\n"), 0o600))
	child := filepath.Join(parent, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), child, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "brief init: installed for claude-code in ..; start Claude Code in .. (or run /reload-plugins in a session already there), then 'brief new feature <name>'\n", stderr.String())
}

// Test_init_no_hook_omits_the_hook_row pins --no-hook: the same five rows
// minus hooks.json, and no hooks.json file on disk.
func Test_init_no_hook_omits_the_hook_row(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--no-hook"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"created .brief.yaml\n"+
		"created docs/specifications/\n"+
		"created .claude/skills/brief/.claude-plugin/plugin.json\n"+
		"created .claude/skills/brief/skills/start/SKILL.md\n"+
		"created .claude/skills/brief/skills/finish/SKILL.md\n"+
		"created .claude/skills/brief-workflow/SKILL.md\n"+
		"created CLAUDE.md\n", stdout.String())

	_, statErr := os.Stat(filepath.Join(wd, ".claude", "skills", "brief", "hooks", "hooks.json"))
	assert.True(t, os.IsNotExist(statErr))
}

// Test_no_hook_with_host_none_is_accepted_and_changes_nothing pins
// --no-hook's own no-op under --host none: no plugin was ever planned, so
// --no-hook has nothing to omit, and init still installs just the config
// and feature root.
func Test_no_hook_with_host_none_is_accepted_and_changes_nothing(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--no-hook"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "created .brief.yaml\ncreated docs/specifications/\n", stdout.String())
}

// Test_init_rerunning_for_claude_code_reports_unchanged_and_edited_files_kept
// pins convergence and "edited locally" together at the CLI boundary: a
// second run reports every row "unchanged" except the finish skill, edited
// between runs, reported "kept (edited locally)" — even under --force,
// which only ever rewrites the config.
func Test_init_rerunning_for_claude_code_reports_unchanged_and_edited_files_kept(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	finish := filepath.Join(wd, ".claude", "skills", "brief", "skills", "finish", "SKILL.md")
	require.NoError(t, os.WriteFile(finish, []byte("---\nedited: true\n---\n"), 0o600))

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--force"}, nil, &stdout, &stderr)

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
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), artifact.SnippetBlock("elsewhere"), 0o600))

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)

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

	body, readErr := os.ReadFile(filepath.Join(wd, "CLAUDE.md"))
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
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(wd, ".claude", "skills", "brief-workflow", "SKILL.md")))

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
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
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--with-agents"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, 0, cli.ExitCode(err))
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

	body, readErr := os.ReadFile(filepath.Join(wd, ".brief.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, artifact.ConfigFileWithRoles(), body)
}

// Test_init_with_agents_over_an_existing_config_prints_the_roles_lines_to_add
// pins R7's stderr hint, exact copy: the config is never edited, and the
// hint block — "was not edited" line, then "roles:" and the three bare
// "  <role>: brief:<role>" lines, no "brief init: " prefix on those since
// they are meant to be pasted verbatim into .brief.yaml — lands before the
// ordinary next-action line.
func Test_init_with_agents_over_an_existing_config_prints_the_roles_lines_to_add(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	original := []byte("feature-directory: specs\n")
	require.NoError(t, os.WriteFile(configPath, original, 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--with-agents"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"brief init: .brief.yaml was not edited; to bind brief's agents, add these lines to it:\n"+
		"roles:\n"+
		"  planner: brief:planner\n"+
		"  implementer: brief:implementer\n"+
		"  reviewer: brief:reviewer\n"+
		"brief init: installed for claude-code; start Claude Code in this directory (or run /reload-plugins in a session already here), then 'brief new feature <name>'\n", stderr.String())

	body, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, original, body)
}

// Test_init_with_agents_and_host_none_is_a_usage_error pins the
// flag-combination rule (checked on the resolved host): an explicit
// "--host none" alongside "--with-agents" refuses, exit 2, tree unchanged.
// A bare "--with-agents" (host resolved by detection) is covered in
// init_internal_test.go, where the home directory is injected — through
// cli.Run here it would read the developer's own "~/.claude".
func Test_init_with_agents_and_host_none_is_a_usage_error(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--with-agents"}, nil, &stdout, &stderr)

	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief init: --with-agents requires --host claude-code; run 'brief init --host claude-code --with-agents'`+"\n", stderr.String())

	entries, readErr := os.ReadDir(wd)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

// Test_init_keeps_a_plugin_path_that_is_a_directory_instead_of_a_file pins
// the "not a regular file" row at the CLI boundary: a directory already
// occupying the manifest's own path is kept, never followed, never
// written.
func Test_init_keeps_a_plugin_path_that_is_a_directory_instead_of_a_file(t *testing.T) {
	wd := t.TempDir()
	manifest := filepath.Join(wd, ".claude", "skills", "brief", ".claude-plugin", "plugin.json")
	require.NoError(t, os.MkdirAll(manifest, 0o755))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)

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
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := cli.Run(t.Context(), wd, c.args, nil, &stdout, &stderr)

			assert.Equal(t, 2, cli.ExitCode(err))
			assert.Empty(t, stdout.String())
			assert.Equal(t, "brief init: --dry-run and --print cannot be combined; run 'brief init --print'\n", stderr.String())

			entries, readErr := os.ReadDir(wd)
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
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--print"}, nil, &stdout, &stderr)

		require.NoError(t, err)
		assert.Equal(t, 0, cli.ExitCode(err))
		assert.Equal(t, "brief init: printed only, no files changed; apply the output above by hand, or rerun without --print\n", stderr.String())

		want := "# .brief.yaml (create)\n" + string(artifact.ConfigFile()) +
			"\n# .claude/skills/brief/.claude-plugin/plugin.json (create)\n" + string(artifact.PluginManifest()) +
			"\n# .claude/skills/brief/skills/start/SKILL.md (create)\n" + string(artifact.SkillStart()) +
			"\n# .claude/skills/brief/skills/finish/SKILL.md (create)\n" + string(artifact.SkillFinish()) +
			"\n# .claude/skills/brief/hooks/hooks.json (create)\n" + string(artifact.ClaudeHooks()) +
			"\n# .claude/skills/brief-workflow/SKILL.md (create)\n" + string(artifact.SkillWorkflow()) +
			"\n# CLAUDE.md (create)\n" + string(artifact.SnippetBlock("docs/specifications")) + "\n"
		assert.Equal(t, want, stdout.String())

		entries, readErr := os.ReadDir(wd)
		require.NoError(t, readErr)
		assert.Empty(t, entries)
	})

	t.Run("merges into an existing CLAUDE.md", func(t *testing.T) {
		wd := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), []byte("# hello\n"), 0o600))
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--print"}, nil, &stdout, &stderr)

		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "# CLAUDE.md (merge)\n")

		body, readErr := os.ReadFile(filepath.Join(wd, "CLAUDE.md"))
		require.NoError(t, readErr)
		assert.Equal(t, []byte("# hello\n"), body)
	})

	t.Run("already installed prints nothing pending", func(t *testing.T) {
		wd := t.TempDir()
		var initStdout, initStderr bytes.Buffer
		require.NoError(t, cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &initStdout, &initStderr))

		var stdout, stderr bytes.Buffer
		err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--print"}, nil, &stdout, &stderr)

		require.NoError(t, err)
		assert.Empty(t, stdout.String())
		assert.Equal(t, "brief init: already installed; nothing changed\n", stderr.String())
	})

	t.Run("a non-regular CLAUDE.md still prints the block to add by hand", func(t *testing.T) {
		wd := t.TempDir()
		var initStdout, initStderr bytes.Buffer
		require.NoError(t, cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--no-hook"}, nil, &initStdout, &initStderr))
		require.NoError(t, os.Remove(filepath.Join(wd, "CLAUDE.md")))
		require.NoError(t, os.Mkdir(filepath.Join(wd, "CLAUDE.md"), 0o755))

		var stdout, stderr bytes.Buffer
		err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--no-hook", "--print"}, nil, &stdout, &stderr)

		require.NoError(t, err)
		assert.Equal(t, "# CLAUDE.md (merge)\n"+string(artifact.SnippetBlock("docs/specifications"))+"\n", stdout.String())
		assert.Equal(t, "brief init: printed only, no files changed; apply the output above by hand, or rerun without --print\n", stderr.String())
	})
}

// Test_init_refuses_an_unwritable_target_and_prints_the_manual_output pins
// R10 at the CLI boundary: the exact stderr refusal line, stdout
// byte-equal to a --print run captured on the same tree beforehand, exit
// 1, and a byte-identical tree; the chmod case is skipped under root. The
// portable fixture uses an explicit --host claude-code — a bare "init"
// would also trip host detection on the same ".claude/skills" path.
func Test_init_refuses_an_unwritable_target_and_prints_the_manual_output(t *testing.T) {
	t.Run("a regular file blocks a plugin directory ancestor", func(t *testing.T) {
		control := t.TempDir()
		var printStdout, printStderr bytes.Buffer
		require.NoError(t, cli.Run(t.Context(), control, []string{"init", "--host", "claude-code", "--print"}, nil, &printStdout, &printStderr))

		wd := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(wd, ".claude"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude", "skills"), []byte("not a directory"), 0o600))
		entriesBefore, readErr := os.ReadDir(wd)
		require.NoError(t, readErr)
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)

		assert.Equal(t, 1, cli.ExitCode(err))
		assert.Equal(t, "brief init: .claude/skills: not a directory; apply the output below by hand (no files changed)\n", stderr.String())
		assert.Equal(t, printStdout.String(), stdout.String())

		entriesAfter, readErr := os.ReadDir(wd)
		require.NoError(t, readErr)
		assert.Equal(t, entriesBefore, entriesAfter)
	})

	t.Run("an unwritable directory, --json", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root ignores directory write permission")
		}

		wd := t.TempDir()
		blocker := filepath.Join(wd, ".claude")
		require.NoError(t, os.Mkdir(blocker, 0o500))
		t.Cleanup(func() { _ = os.Chmod(blocker, 0o755) })
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--json"}, nil, &stdout, &stderr)

		assert.Equal(t, 1, cli.ExitCode(err))
		assert.Empty(t, stderr.String())

		decoded := decodeErrorDocument(t, stdout.Bytes(), "init")
		assert.Equal(t, "refusal", decoded.Kind)
		require.NotNil(t, decoded.FilesChanged)
		assert.False(t, *decoded.FilesChanged)
		assert.Equal(t, "run 'brief init --print --json' and apply the artifacts by hand", decoded.Fix)

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &raw))
		assert.NotContains(t, raw, "artifacts")
	})
}

// Test_init_partial_write_prints_the_rows_that_landed pins the partial-write
// case (setup.ErrPartialWrite): apply's own write order lands the feature
// root before the config write — a directory at ".brief.yaml" — fails, so
// text mode prints the feature root's own "created" row on stdout, the same
// row a successful run would, before the refusal line on stderr; the config
// row, which never landed, is never printed.
func Test_init_partial_write_prints_the_rows_that_landed(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(wd, ".brief.yaml"), 0o755))

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--force"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Equal(t, "created docs/specifications/\n", stdout.String())
	assert.Contains(t, stderr.String(), "brief init: ")
	assert.NotContains(t, stdout.String(), ".brief.yaml")

	info, statErr := os.Stat(filepath.Join(wd, "docs", "specifications"))
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

// Test_init_partial_write_with_json is
// Test_init_partial_write_prints_the_rows_that_landed's own --json sibling:
// the same partial write reports the standard error document — files_changed
// true, since the feature root did land, and no "artifacts" field, the same
// contract a partial write's text mode observes by printing landed rows
// instead.
func Test_init_partial_write_with_json(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(wd, ".brief.yaml"), 0o755))

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none", "--force", "--json"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	decoded := decodeErrorDocument(t, stdout.Bytes(), "init")
	assert.Equal(t, "failure", decoded.Kind)
	require.NotNil(t, decoded.FilesChanged)
	assert.True(t, *decoded.FilesChanged)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &raw))
	assert.NotContains(t, raw, "artifacts")

	info, statErr := os.Stat(filepath.Join(wd, "docs", "specifications"))
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}
