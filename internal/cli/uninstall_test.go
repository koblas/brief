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

// Test_uninstall_removes_the_config_init_wrote pins R6/R11's happy path
// through cli.Run: after "init", "uninstall" removes the config it wrote,
// stdout carries exactly one "removed" row, stderr names R11's third line,
// and exit is nil (0).
func Test_uninstall_removes_the_config_init_wrote(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "removed .brief.yaml\n", stdout.String())
	assert.Equal(t, "brief uninstall: removed brief's config; the feature root and its contents were left in place\n", stderr.String())

	_, statErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
	assert.True(t, os.IsNotExist(statErr))
}

// Test_uninstall_default_host_after_init_host_none_names_the_config_not_a_host_install
// pins the fix for a MAJOR finding: uninstall's own default host is
// claude-code (a superset of none's own plan), but a repository set up
// with "init --host none" never had a claude-code install to remove — only
// the config file is present, so the "removed" line must name the config,
// never claim a claude-code install that was never there.
func Test_uninstall_default_host_after_init_host_none_names_the_config_not_a_host_install(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "removed .brief.yaml\n", stdout.String())
	assert.Equal(t, "brief uninstall: removed brief's config; the feature root and its contents were left in place\n", stderr.String())
}

// Test_uninstall_keeps_an_edited_config_and_reports_it pins R11's stderr
// line 4: a config that was never brief's own render is "kept", and
// stderr names --force as the way to remove it.
func Test_uninstall_keeps_an_edited_config_and_reports_it(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("feature-directory: specs\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "kept .brief.yaml (edited locally)\n", stdout.String())
	assert.Equal(t, "brief uninstall: nothing removed; 1 file(s) edited locally were kept; run 'brief uninstall --force' to remove them\n", stderr.String())

	body, readErr := os.ReadFile(filepath.Join(wd, ".brief.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "feature-directory: specs\n", string(body))
}

// Test_uninstall_dry_run_reports_edited_files_would_be_kept is the dry-run
// twin of Test_uninstall_keeps_an_edited_config_and_reports_it: the same
// edited config, under --dry-run, promises what a real run would do
// ("would be kept") rather than reporting what this call did, and the file
// survives byte-identical. Mutation-verified: deleting uninstallNextAction's
// own dry-run "case editedKept > 0" arm reddens this test alone, restored
// after.
func Test_uninstall_dry_run_reports_edited_files_would_be_kept(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	original := []byte("feature-directory: specs\n")
	require.NoError(t, os.WriteFile(configPath, original, 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none", "--dry-run"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "kept .brief.yaml (edited locally)\n", stdout.String())
	assert.Equal(t, "brief uninstall: dry run, nothing removed; 1 file(s) edited locally would be kept; run 'brief uninstall --force' to remove them\n", stderr.String())

	body, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, original, body)
}

// Test_uninstall_force_removes_an_edited_config pins --force's own
// override at the CLI boundary.
func Test_uninstall_force_removes_an_edited_config(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("feature-directory: specs\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none", "--force"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "removed .brief.yaml (edited locally)\n", stdout.String())
	assert.Equal(t, "brief uninstall: removed brief's config; the feature root and its contents were left in place\n", stderr.String())

	_, statErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
	assert.True(t, os.IsNotExist(statErr))
}

// Test_uninstall_with_nothing_installed_reports_it pins R11's zero-artifact
// branch for uninstall's own default host, claude-code: empty stdout,
// "nothing installed for claude-code" on stderr, exit nil.
func Test_uninstall_with_nothing_installed_reports_it(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief uninstall: nothing installed for claude-code\n", stderr.String())
}

// Test_uninstall_host_none_with_nothing_installed_omits_the_host_suffix is
// the control arm for the test above: scoped to --host none, the same
// zero-artifact outcome reports "nothing installed" with no " for <host>"
// suffix, proving the suffix names the host rather than always appearing.
func Test_uninstall_host_none_with_nothing_installed_omits_the_host_suffix(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief uninstall: nothing installed\n", stderr.String())
}

// Test_uninstall_dry_run_prints_the_plan_and_removes_nothing pins R9's own
// dry-run promise: the row says "removed", the dry-run stderr line names
// what the plan actually holds — here a lone config removal, never a host
// install that was never there (C2's fix: the dry-run arm used to name
// installLabel(host) unconditionally, regardless of what was planned) —
// and the file survives byte-identical.
func Test_uninstall_dry_run_prints_the_plan_and_removes_nothing(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall", "--host", "none", "--dry-run"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "removed .brief.yaml\n", stdout.String())
	assert.Equal(t, "brief uninstall: dry run, nothing removed; rerun without --dry-run to remove brief's config\n", stderr.String())

	_, statErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
	assert.NoError(t, statErr)
}

// Test_uninstall_dry_run_for_claude_code_names_the_host_install is the
// control arm for the test above: a plan holding a non-config removed
// artifact (the CLAUDE.md block, here) names the host install under
// --dry-run, exactly as a real run's own "removed" line would.
func Test_uninstall_dry_run_for_claude_code_names_the_host_install(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code", "--dry-run"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "brief uninstall: dry run, nothing removed; rerun without --dry-run to remove brief's claude-code install\n", stderr.String())

	_, statErr := os.Stat(filepath.Join(wd, ".claude", "skills", "brief"))
	assert.NoError(t, statErr)
}

// Test_uninstall_dry_run_with_nothing_installed_reports_it is the third
// arm: an empty plan under --dry-run names "nothing installed", never a
// promise to remove something the plan never found.
func Test_uninstall_dry_run_with_nothing_installed_reports_it(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--dry-run"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief uninstall: dry run, nothing installed for claude-code\n", stderr.String())
}

// Test_uninstall_for_claude_code_removes_the_plugin_then_the_config pins
// the user-visible contract: rows in removal order — the CLAUDE.md block
// first, then hooks.json, finish skill, start skill, manifest, then
// ".brief.yaml" last — every one "removed", and the claude-code
// next-action suffix.
func Test_uninstall_for_claude_code_removes_the_plugin_then_the_config(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"removed CLAUDE.md\n"+
		"removed .claude/skills/brief/hooks/hooks.json\n"+
		"removed .claude/skills/brief/skills/finish/SKILL.md\n"+
		"removed .claude/skills/brief/skills/start/SKILL.md\n"+
		"removed .claude/skills/brief/.claude-plugin/plugin.json\n"+
		"removed .brief.yaml\n", stdout.String())
	assert.Equal(t, "brief uninstall: removed brief's claude-code install; the feature root and its contents were left in place\n", stderr.String())

	_, statErr := os.Stat(filepath.Join(wd, ".claude", "skills", "brief"))
	assert.True(t, os.IsNotExist(statErr))
}

// Test_uninstall_for_claude_code_keeps_an_edited_skill_and_removes_it_under_force
// pins the mixed-row case at the CLI boundary: one file kept, the rest
// removed, then --force removes it too.
func Test_uninstall_for_claude_code_keeps_an_edited_skill_and_removes_it_under_force(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	start := filepath.Join(wd, ".claude", "skills", "brief", "skills", "start", "SKILL.md")
	require.NoError(t, os.WriteFile(start, []byte("---\nedited: true\n---\n"), 0o600))

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "kept .claude/skills/brief/skills/start/SKILL.md (edited locally)\n")

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code", "--force"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "removed .claude/skills/brief/skills/start/SKILL.md (edited locally)\n")

	_, statErr := os.Stat(filepath.Join(wd, ".claude", "skills", "brief"))
	assert.True(t, os.IsNotExist(statErr))
}

// Test_uninstall_counts_two_force_removable_kept_artifacts pins
// uninstallNextAction's own "N file(s) edited locally were kept" count at
// N=2, not just N=1 (every other case in this file edits exactly one
// artifact, which a hardcoded "1" in place of editedKept would also
// satisfy): a hand-written plugin file and an edited config, both host
// claude-code, present with nothing else installed — planPluginRemoval
// plans no row at all for a missing path, so every other plugin/agent file
// is simply absent from the plan — are both kept and both counted, with no
// ActionRemoved artifact to take the "removed" branch ahead of editedKept.
// Mutation-verified: hardcoding the real-run "N file(s) … kept" Sprintf's
// own count argument to 1 reddens this test alone, restored after.
func Test_uninstall_counts_two_force_removable_kept_artifacts(t *testing.T) {
	wd := t.TempDir()
	start := filepath.Join(wd, ".claude", "skills", "brief", "skills", "start", "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(start), 0o755))
	require.NoError(t, os.WriteFile(start, []byte("---\nedited: true\n---\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("feature-directory: specs\n"), 0o600))

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "kept .claude/skills/brief/skills/start/SKILL.md (edited locally)\n")
	assert.Contains(t, stdout.String(), "kept .brief.yaml (edited locally)\n")
	assert.Equal(t, "brief uninstall: nothing removed; 2 file(s) edited locally were kept; run 'brief uninstall --force' to remove them\n", stderr.String())

	_, startErr := os.Stat(start)
	require.NoError(t, startErr)
	_, configErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
	require.NoError(t, configErr)
}

// Test_uninstall_removes_the_block_leaving_unrelated_content pins the
// "removed CLAUDE.md (brief block)" row at the CLI boundary: a CLAUDE.md
// carrying unrelated prose alongside the block loses only the block, is
// reported "removed" with that detail, stays on disk, and lands in
// modified[] never removed[].
func Test_uninstall_removes_the_block_leaving_unrelated_content(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	claudeMD := filepath.Join(wd, "CLAUDE.md")
	before, readErr := os.ReadFile(claudeMD)
	require.NoError(t, readErr)
	require.NoError(t, os.WriteFile(claudeMD, append([]byte("# My project\n\n"), before...), 0o600)) //nolint:gosec // claudeMD is t.TempDir() joined with a fixed literal, not user input

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code", "--json"}, nil, &stdout, &stderr)

	require.NoError(t, err)

	var doc struct {
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	assert.Contains(t, doc.Modified, claudeMD)
	assert.NotContains(t, doc.Removed, claudeMD)

	body, readErr := os.ReadFile(claudeMD)
	require.NoError(t, readErr)
	assert.Equal(t, "# My project\n", string(body))
}

// Test_uninstall_text_row_for_a_kept_content_block pins the text-mode row
// naming: "removed CLAUDE.md (brief block)" for the block-stripped case.
func Test_uninstall_text_row_for_a_kept_content_block(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr)
	require.NoError(t, err)

	claudeMD := filepath.Join(wd, "CLAUDE.md")
	before, readErr := os.ReadFile(claudeMD)
	require.NoError(t, readErr)
	require.NoError(t, os.WriteFile(claudeMD, append([]byte("# My project\n\n"), before...), 0o600)) //nolint:gosec // claudeMD is t.TempDir() joined with a fixed literal, not user input

	stdout.Reset()
	stderr.Reset()

	err = cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "removed CLAUDE.md (brief block)\n")
}

// Test_uninstall_reaches_the_removed_line_from_the_snippet_alone pins the
// snippet-only uninstall path: with no plugin files and no config
// installed — only a brief-written CLAUDE.md block, --host none never
// having touched it — uninstall still reaches R11's "removed brief's
// install" stderr line off the snippet's own ActionRemoved alone.
func Test_uninstall_reaches_the_removed_line_from_the_snippet_alone(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), artifact.SnippetBlock("docs/specifications"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "removed CLAUDE.md\n", stdout.String())
	assert.Equal(t, "brief uninstall: removed brief's claude-code install; the feature root and its contents were left in place\n", stderr.String())
}

// Test_uninstall_nonregular_claude_md_alone_reports_plain_nothing_removed
// pins the boundary of uninstallNextAction's own "N file(s) edited locally
// were kept" count (MAJOR fix): a CLAUDE.md that is a directory, not a
// regular file, is the sole artifact and reports ActionKept, detail "not a
// regular file" — never "edited locally", since --force cannot remove it
// either (planSnippetRemoval). That must not be counted toward the
// force-removable tally, or the line would promise --force can remove a
// file it never touches; it must fall to the plain "nothing removed" line
// instead. The stdout row is pinned exactly (C3's fix): planSnippetRemoval's
// own kept detail is the plain "not a regular file", never planSnippet's
// longer install-side "…; add the block by hand, see 'brief init --print'"
// — there is nothing to add by hand on a removal.
func Test_uninstall_nonregular_claude_md_alone_reports_plain_nothing_removed(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(wd, "CLAUDE.md"), 0o755))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "kept CLAUDE.md (not a regular file)\n", stdout.String())
	assert.Equal(t, "brief uninstall: nothing removed for claude-code\n", stderr.String())
}

// Test_uninstall_dry_run_nonregular_claude_md_alone_reports_plain_nothing_removed
// is the dry-run twin of the test above: the same non-regular CLAUDE.md,
// under --dry-run, reaches uninstallNextAction's own plain "nothing
// removed" default arm — a non-empty plan with nothing removed and nothing
// force-removable — rather than a promise to remove or keep anything.
// Mutation-verified: dropping the "&& a.ForceRemovable" guard from
// uninstallNextAction's own tally loop (counting every ActionKept, not
// only a force-removable one) reddens this test and its non-dry-run
// sibling above, restored after.
func Test_uninstall_dry_run_nonregular_claude_md_alone_reports_plain_nothing_removed(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(wd, "CLAUDE.md"), 0o755))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code", "--dry-run"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Equal(t, "kept CLAUDE.md (not a regular file)\n", stdout.String())
	assert.Equal(t, "brief uninstall: dry run, nothing removed for claude-code\n", stderr.String())
}

// Test_uninstall_refuses_a_lone_marker_naming_the_file_and_line pins R5's
// own refusal text at the CLI boundary: "<rel>:<line>: <problem>; <fix>",
// exit 1, nothing removed.
func Test_uninstall_refuses_a_lone_marker_naming_the_file_and_line(t *testing.T) {
	wd := t.TempDir()
	claudeMD := filepath.Join(wd, "CLAUDE.md")
	require.NoError(t, os.WriteFile(claudeMD, []byte("notes\n"+artifact.SnippetBegin+"\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "CLAUDE.md:2:")
	assert.Contains(t, stderr.String(), "(no files changed)")

	body, readErr := os.ReadFile(claudeMD)
	require.NoError(t, readErr)
	assert.Equal(t, "notes\n"+artifact.SnippetBegin+"\n", string(body))
}

// Test_uninstall_refuses_an_unknown_host pins R8's usage-error branch,
// mirrored from init: exit 2, naming the given value and the accepted
// list.
func Test_uninstall_refuses_an_unknown_host(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "bogus"}, nil, &stdout, &stderr)

	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief uninstall: unknown host "bogus"; expected one of: claude-code, none; run 'brief uninstall --host claude-code'`+"\n", stderr.String())
}

// Test_uninstall_refuses_a_stray_positional_argument pins the usage-error
// branch for an argument uninstall takes none of.
func Test_uninstall_refuses_a_stray_positional_argument(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"uninstall", "extra"}, nil, &stdout, &stderr)

	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Contains(t, stderr.String(), "too many arguments")
}

// Test_uninstall_partial_write_prints_the_rows_that_landed pins the
// partial-write case (setup.ErrPartialWrite): the CLAUDE.md block is
// removed before the agents directory — made unwritable — blocks the next
// removal, so text mode prints the CLAUDE.md "removed" row that actually
// landed on stdout before the refusal line on stderr; no agent row, which
// never landed, is ever printed. Skipped under root, which ignores
// directory write permission.
func Test_uninstall_partial_write_prints_the_rows_that_landed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	wd := t.TempDir()
	var initStdout, initStderr bytes.Buffer
	require.NoError(t, cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--with-agents"}, nil, &initStdout, &initStderr))

	agentsDir := filepath.Join(wd, ".claude", "skills", "brief", "agents")
	require.NoError(t, os.Chmod(agentsDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(agentsDir, 0o755) })

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Equal(t, "removed CLAUDE.md\n", stdout.String())
	assert.Contains(t, stderr.String(), "brief uninstall: ")
	assert.NotContains(t, stdout.String(), "agents")

	_, statErr := os.Stat(filepath.Join(wd, "CLAUDE.md"))
	assert.True(t, os.IsNotExist(statErr))
}

// Test_uninstall_partial_write_with_json is
// Test_uninstall_partial_write_prints_the_rows_that_landed's own --json
// sibling: the same partial write reports the standard error document —
// files_changed true, since the CLAUDE.md block did land, and no
// "artifacts" field, the same contract a partial write's text mode observes
// by printing landed rows instead. Skipped under root, which ignores
// directory write permission.
func Test_uninstall_partial_write_with_json(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	wd := t.TempDir()
	var initStdout, initStderr bytes.Buffer
	require.NoError(t, cli.Run(t.Context(), wd, []string{"init", "--host", "claude-code", "--with-agents"}, nil, &initStdout, &initStderr))

	agentsDir := filepath.Join(wd, ".claude", "skills", "brief", "agents")
	require.NoError(t, os.Chmod(agentsDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(agentsDir, 0o755) })

	var stdout, stderr bytes.Buffer
	err := cli.Run(t.Context(), wd, []string{"uninstall", "--host", "claude-code", "--json"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stderr.String())

	decoded := decodeErrorDocument(t, stdout.Bytes(), "uninstall")
	assert.Equal(t, "failure", decoded.Kind)
	require.NotNil(t, decoded.FilesChanged)
	assert.True(t, *decoded.FilesChanged)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &raw))
	assert.NotContains(t, raw, "artifacts")

	_, statErr := os.Stat(filepath.Join(wd, "CLAUDE.md"))
	assert.True(t, os.IsNotExist(statErr))
}
