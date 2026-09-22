package cli_test

import (
	"bytes"
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
// naming the given value and the accepted list. "bogus" is used rather
// than "claude-code" — S03 does not accept it yet.
func Test_init_refuses_an_unknown_host(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "bogus"}, nil, &stdout, &stderr)

	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief init: unknown host "bogus"; expected one of: claude-code, none; run 'brief init --host claude-code'`+"\n", stderr.String())
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
		"merged CLAUDE.md (block updated)\n", stdout.String())
	assert.Equal(t, "brief init: installed for claude-code; start Claude Code in this directory (or run /reload-plugins in a session already here), then 'brief new feature <name>'\n", stderr.String())

	body, readErr := os.ReadFile(filepath.Join(wd, "CLAUDE.md"))
	require.NoError(t, readErr)
	assert.Equal(t, artifact.SnippetBlock("docs/specifications"), body)
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
