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
	assert.Equal(t, "brief uninstall: removed brief's install; the feature root and its contents were left in place\n", stderr.String())

	_, statErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
	assert.True(t, os.IsNotExist(statErr))
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
	assert.Equal(t, "brief uninstall: nothing removed; run 'brief uninstall --force' to remove edited files\n", stderr.String())

	body, readErr := os.ReadFile(filepath.Join(wd, ".brief.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "feature-directory: specs\n", string(body))
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
	assert.Equal(t, "brief uninstall: removed brief's install; the feature root and its contents were left in place\n", stderr.String())

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
// dry-run promise: the row says "removed", the dry-run stderr line, and
// the file survives byte-identical.
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
	assert.Equal(t, "brief uninstall: dry run, no files changed; rerun without --dry-run to apply\n", stderr.String())

	_, statErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
	assert.NoError(t, statErr)
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
