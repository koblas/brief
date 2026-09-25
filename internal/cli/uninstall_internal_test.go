// This file reaches the unexported run directly, injecting newMemSetupSeam
// so the plain install/removal path never touches real disk. Bound-agent
// cases stay in uninstall_bound_agent_internal_test.go; the real
// writability pre-check stays in uninstall_test.go.

package cli

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_uninstall_removes_the_config_init_wrote(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"uninstall", "--host", "none"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, "removed .brief.yaml\n", stdout.String())
	assert.Equal(t, "brief uninstall: removed brief's config; the feature root and its contents were left in place\n", stderr.String())

	_, statErr := mem.Stat(memKey(filepath.Join(wd, ".brief.yaml")))
	assert.ErrorIs(t, statErr, fs.ErrNotExist)
}

// A repository set up with "init --host none" never had a claude-code
// install to remove, so the default-host "removed" line must name the
// config, never claim a host install that was never there.
func Test_uninstall_default_host_after_init_host_none_names_the_config_not_a_host_install(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"uninstall"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, "removed .brief.yaml\n", stdout.String())
	assert.Equal(t, "brief uninstall: removed brief's config; the feature root and its contents were left in place\n", stderr.String())
}

func Test_uninstall_keeps_an_edited_config_and_reports_it(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	require.NoError(t, mem.WriteFile(memKey(filepath.Join(wd, ".brief.yaml")), []byte("feature-directory: specs\n"), 0o600))
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall", "--host", "none"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, "kept .brief.yaml (edited locally)\n", stdout.String())
	assert.Equal(t, "brief uninstall: nothing removed; 1 file(s) edited locally were kept; run 'brief uninstall --force' to remove them\n", stderr.String())

	body, readErr := mem.ReadFile(memKey(filepath.Join(wd, ".brief.yaml")))
	require.NoError(t, readErr)
	assert.Equal(t, "feature-directory: specs\n", string(body))
}

// The dry-run twin of the test above: it promises what a real run would
// do rather than reporting what this call did, and the file survives byte-identical.
func Test_uninstall_dry_run_reports_edited_files_would_be_kept(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configKey := memKey(filepath.Join(wd, ".brief.yaml"))
	original := []byte("feature-directory: specs\n")
	require.NoError(t, mem.WriteFile(configKey, original, 0o600))
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall", "--host", "none", "--dry-run"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, "kept .brief.yaml (edited locally)\n", stdout.String())
	assert.Equal(t, "brief uninstall: dry run, nothing removed; 1 file(s) edited locally would be kept; run 'brief uninstall --force' to remove them\n", stderr.String())

	body, readErr := mem.ReadFile(configKey)
	require.NoError(t, readErr)
	assert.Equal(t, original, body)
}

func Test_uninstall_force_removes_an_edited_config(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	require.NoError(t, mem.WriteFile(memKey(filepath.Join(wd, ".brief.yaml")), []byte("feature-directory: specs\n"), 0o600))
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall", "--host", "none", "--force"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, "removed .brief.yaml (edited locally)\n", stdout.String())
	assert.Equal(t, "brief uninstall: removed brief's config; the feature root and its contents were left in place\n", stderr.String())

	_, statErr := mem.Stat(memKey(filepath.Join(wd, ".brief.yaml")))
	assert.ErrorIs(t, statErr, fs.ErrNotExist)
}

func Test_uninstall_with_nothing_installed_reports_it(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief uninstall: nothing installed for claude-code\n", stderr.String())
}

// Control arm: scoped to --host none, the suffix naming the host is absent.
func Test_uninstall_host_none_with_nothing_installed_omits_the_host_suffix(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall", "--host", "none"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief uninstall: nothing installed\n", stderr.String())
}

func Test_uninstall_dry_run_prints_the_plan_and_removes_nothing(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"uninstall", "--host", "none", "--dry-run"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, "removed .brief.yaml\n", stdout.String())
	assert.Equal(t, "brief uninstall: dry run, nothing removed; rerun without --dry-run to remove brief's config\n", stderr.String())

	_, statErr := mem.Stat(memKey(filepath.Join(wd, ".brief.yaml")))
	assert.NoError(t, statErr)
}

// Control arm: a plan holding a non-config removed artifact names the
// host install under --dry-run.
func Test_uninstall_dry_run_for_claude_code_names_the_host_install(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"uninstall", "--host", "claude-code", "--dry-run"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, "brief uninstall: dry run, nothing removed; rerun without --dry-run to remove brief's claude-code install\n", stderr.String())

	_, statErr := mem.Stat(memKey(filepath.Join(wd, ".claude", "skills", "brief")))
	assert.NoError(t, statErr)
}

func Test_uninstall_dry_run_with_nothing_installed_reports_it(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall", "--dry-run"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief uninstall: dry run, nothing installed for claude-code\n", stderr.String())
}

func Test_uninstall_for_claude_code_removes_the_plugin_then_the_config(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Equal(t, ""+
		"removed CLAUDE.md\n"+
		"removed .claude/skills/brief-workflow/SKILL.md\n"+
		"removed .claude/skills/brief/hooks/hooks.json\n"+
		"removed .claude/skills/brief/skills/finish/SKILL.md\n"+
		"removed .claude/skills/brief/skills/start/SKILL.md\n"+
		"removed .claude/skills/brief/.claude-plugin/plugin.json\n"+
		"removed .brief.yaml\n", stdout.String())
	assert.Equal(t, "brief uninstall: removed brief's claude-code install; the feature root and its contents were left in place\n", stderr.String())

	_, statErr := mem.Stat(memKey(filepath.Join(wd, ".claude", "skills", "brief")))
	assert.ErrorIs(t, statErr, fs.ErrNotExist)
}

func Test_uninstall_for_claude_code_keeps_an_edited_skill_and_removes_it_under_force(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	start := filepath.Join(wd, ".claude", "skills", "brief", "skills", "start", "SKILL.md")
	require.NoError(t, mem.WriteFile(memKey(start), []byte("---\nedited: true\n---\n"), 0o600))

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "kept .claude/skills/brief/skills/start/SKILL.md (edited locally)\n")

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"uninstall", "--host", "claude-code", "--force"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "removed .claude/skills/brief/skills/start/SKILL.md (edited locally)\n")

	_, statErr := mem.Stat(memKey(filepath.Join(wd, ".claude", "skills", "brief")))
	assert.ErrorIs(t, statErr, fs.ErrNotExist)
}

// Two independently edited artifacts, both kept, must both be counted,
// not just the first.
func Test_uninstall_counts_two_force_removable_kept_artifacts(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	start := filepath.Join(wd, ".claude", "skills", "brief", "skills", "start", "SKILL.md")
	require.NoError(t, mem.MkdirAll(memKey(filepath.Dir(start)), 0o755))
	require.NoError(t, mem.WriteFile(memKey(start), []byte("---\nedited: true\n---\n"), 0o600))
	require.NoError(t, mem.WriteFile(memKey(filepath.Join(wd, ".brief.yaml")), []byte("feature-directory: specs\n"), 0o600))

	var stdout, stderr bytes.Buffer
	err := run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "kept .claude/skills/brief/skills/start/SKILL.md (edited locally)\n")
	assert.Contains(t, stdout.String(), "kept .brief.yaml (edited locally)\n")
	assert.Equal(t, "brief uninstall: nothing removed; 2 file(s) edited locally were kept; run 'brief uninstall --force' to remove them\n", stderr.String())

	_, startErr := mem.Stat(memKey(start))
	require.NoError(t, startErr)
	_, configErr := mem.Stat(memKey(filepath.Join(wd, ".brief.yaml")))
	require.NoError(t, configErr)
}

// The dry-run twin of the test above: the same two-artifact fixture must
// promise "2 file(s) … would be kept", not just "1".
func Test_uninstall_dry_run_counts_two_force_removable_kept_artifacts(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	start := filepath.Join(wd, ".claude", "skills", "brief", "skills", "start", "SKILL.md")
	require.NoError(t, mem.MkdirAll(memKey(filepath.Dir(start)), 0o755))
	require.NoError(t, mem.WriteFile(memKey(start), []byte("---\nedited: true\n---\n"), 0o600))
	require.NoError(t, mem.WriteFile(memKey(filepath.Join(wd, ".brief.yaml")), []byte("feature-directory: specs\n"), 0o600))

	var stdout, stderr bytes.Buffer
	err := run(t.Context(), wd, []string{"uninstall", "--host", "claude-code", "--dry-run"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "kept .claude/skills/brief/skills/start/SKILL.md (edited locally)\n")
	assert.Contains(t, stdout.String(), "kept .brief.yaml (edited locally)\n")
	assert.Equal(t, "brief uninstall: dry run, nothing removed; 2 file(s) edited locally would be kept; run 'brief uninstall --force' to remove them\n", stderr.String())

	_, startErr := mem.Stat(memKey(start))
	require.NoError(t, startErr)
	_, configErr := mem.Stat(memKey(filepath.Join(wd, ".brief.yaml")))
	require.NoError(t, configErr)
}

func Test_uninstall_removes_the_block_leaving_unrelated_content(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	claudeMDKey := memKey(filepath.Join(wd, "CLAUDE.md"))
	before, readErr := mem.ReadFile(claudeMDKey)
	require.NoError(t, readErr)
	require.NoError(t, mem.WriteFile(claudeMDKey, append([]byte("# My project\n\n"), before...), 0o600))

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"uninstall", "--host", "claude-code", "--json"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)

	var doc struct {
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))
	claudeMD := filepath.Join(wd, "CLAUDE.md")
	assert.Contains(t, doc.Modified, claudeMD)
	assert.NotContains(t, doc.Removed, claudeMD)

	body, readErr := mem.ReadFile(claudeMDKey)
	require.NoError(t, readErr)
	assert.Equal(t, "# My project\n", string(body))
}

func Test_uninstall_text_row_for_a_kept_content_block(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	claudeMDKey := memKey(filepath.Join(wd, "CLAUDE.md"))
	before, readErr := mem.ReadFile(claudeMDKey)
	require.NoError(t, readErr)
	require.NoError(t, mem.WriteFile(claudeMDKey, append([]byte("# My project\n\n"), before...), 0o600))

	stdout.Reset()
	stderr.Reset()

	err = run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)

	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "removed CLAUDE.md (brief block)\n")
}

// No plugin files, no config installed, only a brief-written CLAUDE.md
// block: uninstall still reaches the "removed" line off the snippet alone.
func Test_uninstall_reaches_the_removed_line_from_the_snippet_alone(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	require.NoError(t, mem.WriteFile(memKey(filepath.Join(wd, "CLAUDE.md")), artifact.SnippetBlock("docs/specifications"), 0o600))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Equal(t, "removed CLAUDE.md\n", stdout.String())
	assert.Equal(t, "brief uninstall: removed brief's claude-code install; the feature root and its contents were left in place\n", stderr.String())
}

// A CLAUDE.md that is a directory, not a regular file, is not
// force-removable, so it must not count toward the "edited locally" tally.
func Test_uninstall_nonregular_claude_md_alone_reports_plain_nothing_removed(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	require.NoError(t, mem.Mkdir(memKey(filepath.Join(wd, "CLAUDE.md")), 0o755))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Equal(t, "kept CLAUDE.md (not a regular file)\n", stdout.String())
	assert.Equal(t, "brief uninstall: nothing removed for claude-code\n", stderr.String())
}

func Test_uninstall_dry_run_nonregular_claude_md_alone_reports_plain_nothing_removed(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	require.NoError(t, mem.Mkdir(memKey(filepath.Join(wd, "CLAUDE.md")), 0o755))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall", "--host", "claude-code", "--dry-run"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Equal(t, "kept CLAUDE.md (not a regular file)\n", stdout.String())
	assert.Equal(t, "brief uninstall: dry run, nothing removed for claude-code\n", stderr.String())
}

func Test_uninstall_refuses_a_lone_marker_naming_the_file_and_line(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	claudeMDKey := memKey(filepath.Join(wd, "CLAUDE.md"))
	require.NoError(t, mem.WriteFile(claudeMDKey, []byte("notes\n"+artifact.SnippetBegin+"\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "CLAUDE.md:2:")
	assert.Contains(t, stderr.String(), "(no files changed)")

	body, readErr := mem.ReadFile(claudeMDKey)
	require.NoError(t, readErr)
	assert.Equal(t, "notes\n"+artifact.SnippetBegin+"\n", string(body))
}

func Test_uninstall_refuses_an_unknown_host(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall", "--host", "bogus"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	assert.Equal(t, 2, ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief uninstall: unknown host "bogus"; expected one of: claude-code, none; run 'brief uninstall --host claude-code'`+"\n", stderr.String())
}

func Test_uninstall_refuses_a_stray_positional_argument(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), fsAbs("repo"), []string{"uninstall", "extra"}, nil, &stdout, &stderr, noBuildInfo)

	assert.Equal(t, 2, ExitCode(err))
	assert.Contains(t, stderr.String(), "too many arguments")
}
