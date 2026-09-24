// This file reaches the unexported run directly to inject a runSeam: every
// case below substitutes setup.WithFSRoot (plus WithResolveRoot,
// WithHomeDir and WithWritableCheck) via newMemSetupSeam
// (mem_internal_test.go) so the plain install/removal path — no bound
// agent file, no real permission bit — never touches real disk.
// uninstall's own bound-agent tests stay in
// uninstall_bound_agent_internal_test.go, the same split
// init_internal_test.go and init_bound_agent_internal_test.go make, for
// the same reason (internal/setup's own bound_agent.go always reads and
// writes those files through real disk regardless of setup.WithFSRoot).
// R10's own writability pre-check (a real chmod'd, unwritable directory)
// stays in uninstall_test.go: setup.WithWritableCheck's own no-op here
// exists so a target that only exists on mem is never asked about on real
// disk, which also means this file cannot exercise R10 itself.

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

// Test_uninstall_removes_the_config_init_wrote pins R6/R11's happy path:
// after "init", "uninstall" removes the config it wrote, stdout carries
// exactly one "removed" row, stderr names R11's third line, and exit is
// nil (0).
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

// Test_uninstall_default_host_after_init_host_none_names_the_config_not_a_host_install
// pins the fix for a MAJOR finding: uninstall's own default host is
// claude-code (a superset of none's own plan), but a repository set up
// with "init --host none" never had a claude-code install to remove — only
// the config file is present, so the "removed" line must name the config,
// never claim a claude-code install that was never there.
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

// Test_uninstall_keeps_an_edited_config_and_reports_it pins R11's stderr
// line 4: a config that was never brief's own render is "kept", and
// stderr names --force as the way to remove it.
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

// Test_uninstall_dry_run_reports_edited_files_would_be_kept is the dry-run
// twin of Test_uninstall_keeps_an_edited_config_and_reports_it: the same
// edited config, under --dry-run, promises what a real run would do
// ("would be kept") rather than reporting what this call did, and the file
// survives byte-identical.
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

// Test_uninstall_force_removes_an_edited_config pins --force's own
// override.
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

// Test_uninstall_with_nothing_installed_reports_it pins R11's zero-artifact
// branch for uninstall's own default host, claude-code: empty stdout,
// "nothing installed for claude-code" on stderr, exit nil.
func Test_uninstall_with_nothing_installed_reports_it(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief uninstall: nothing installed for claude-code\n", stderr.String())
}

// Test_uninstall_host_none_with_nothing_installed_omits_the_host_suffix is
// the control arm for the test above: scoped to --host none, the same
// zero-artifact outcome reports "nothing installed" with no " for <host>"
// suffix, proving the suffix names the host rather than always appearing.
func Test_uninstall_host_none_with_nothing_installed_omits_the_host_suffix(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall", "--host", "none"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief uninstall: nothing installed\n", stderr.String())
}

// Test_uninstall_dry_run_prints_the_plan_and_removes_nothing pins R9's own
// dry-run promise: the row says "removed", the dry-run stderr line names
// what the plan actually holds — here a lone config removal, never a host
// install that was never there — and the file survives byte-identical.
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

// Test_uninstall_dry_run_for_claude_code_names_the_host_install is the
// control arm for the test above: a plan holding a non-config removed
// artifact (the CLAUDE.md block, here) names the host install under
// --dry-run, exactly as a real run's own "removed" line would.
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

// Test_uninstall_dry_run_with_nothing_installed_reports_it is the third
// arm: an empty plan under --dry-run names "nothing installed", never a
// promise to remove something the plan never found.
func Test_uninstall_dry_run_with_nothing_installed_reports_it(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall", "--dry-run"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief uninstall: dry run, nothing installed for claude-code\n", stderr.String())
}

// Test_uninstall_for_claude_code_removes_the_plugin_then_the_config pins
// the user-visible contract: rows in removal order — the CLAUDE.md block
// first, then the brief-workflow skill, then hooks.json, finish skill,
// start skill, manifest, then ".brief.yaml" last — every one "removed",
// and the claude-code next-action suffix.
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

// Test_uninstall_for_claude_code_keeps_an_edited_skill_and_removes_it_under_force
// pins the mixed-row case: one file kept, the rest removed, then --force
// removes it too.
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

// Test_uninstall_counts_two_force_removable_kept_artifacts pins
// uninstallNextAction's own "N file(s) edited locally were kept" count at
// N=2, not just N=1 (every other case in this file edits exactly one
// artifact, which a hardcoded "1" in place of editedKept would also
// satisfy): a hand-written plugin file and an edited config, both host
// claude-code, present with nothing else installed, are both kept and both
// counted, with no ActionRemoved artifact to take the "removed" branch
// ahead of editedKept.
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

// Test_uninstall_dry_run_counts_two_force_removable_kept_artifacts is the
// dry-run twin of Test_uninstall_counts_two_force_removable_kept_artifacts:
// the same two-artifact fixture, run with --dry-run, must promise "2
// file(s) … would be kept" — not just "1", which a dry-run Sprintf whose
// own count argument was hardcoded to 1 would also satisfy, since every
// other dry-run case in this file edits exactly one artifact.
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

// Test_uninstall_removes_the_block_leaving_unrelated_content pins the
// "removed CLAUDE.md (brief block)" row: a CLAUDE.md carrying unrelated
// prose alongside the block loses only the block, is reported "removed"
// with that detail, stays on disk, and lands in modified[] never
// removed[].
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

// Test_uninstall_text_row_for_a_kept_content_block pins the text-mode row
// naming: "removed CLAUDE.md (brief block)" for the block-stripped case.
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

// Test_uninstall_reaches_the_removed_line_from_the_snippet_alone pins the
// snippet-only uninstall path: with no plugin files and no config
// installed — only a brief-written CLAUDE.md block, --host none never
// having touched it — uninstall still reaches R11's "removed brief's
// install" stderr line off the snippet's own ActionRemoved alone.
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

// Test_uninstall_nonregular_claude_md_alone_reports_plain_nothing_removed
// pins the boundary of uninstallNextAction's own "N file(s) edited locally
// were kept" count: a CLAUDE.md that is a directory, not a regular file,
// is the sole artifact and reports ActionKept, detail "not a regular
// file" — never "edited locally", since --force cannot remove it either
// (planSnippetRemoval). That must not be counted toward the
// force-removable tally, or the line would promise --force can remove a
// file it never touches; it must fall to the plain "nothing removed" line
// instead.
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

// Test_uninstall_dry_run_nonregular_claude_md_alone_reports_plain_nothing_removed
// is the dry-run twin of the test above: the same non-regular CLAUDE.md,
// under --dry-run, reaches uninstallNextAction's own plain "nothing
// removed" default arm — a non-empty plan with nothing removed and nothing
// force-removable — rather than a promise to remove or keep anything.
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

// Test_uninstall_refuses_a_lone_marker_naming_the_file_and_line pins R5's
// own refusal text: "<rel>:<line>: <problem>; <fix>", exit 1, nothing
// removed.
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

// Test_uninstall_refuses_an_unknown_host pins R8's usage-error branch,
// mirrored from init: exit 2, naming the given value and the accepted
// list. This refusal fires before Uninstall ever reads through fsRoot, so
// it needs no fixture beyond wd's own existence.
func Test_uninstall_refuses_an_unknown_host(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"uninstall", "--host", "bogus"}, nil, &stdout, &stderr, noBuildInfo, newMemSetupSeam(mem))

	assert.Equal(t, 2, ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, `brief uninstall: unknown host "bogus"; expected one of: claude-code, none; run 'brief uninstall --host claude-code'`+"\n", stderr.String())
}

// Test_uninstall_refuses_a_stray_positional_argument pins the usage-error
// branch for an argument uninstall takes none of — checked before wd is
// ever read, so it needs no fixture at all.
func Test_uninstall_refuses_a_stray_positional_argument(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), fsAbs("repo"), []string{"uninstall", "extra"}, nil, &stdout, &stderr, noBuildInfo)

	assert.Equal(t, 2, ExitCode(err))
	assert.Contains(t, stderr.String(), "too many arguments")
}
