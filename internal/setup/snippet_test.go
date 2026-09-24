package setup_test

import (
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_init_for_claude_code_creates_CLAUDE_md_with_the_block_alone pins the
// create case (R5): neither CLAUDE.md nor .claude/CLAUDE.md exists, so
// CLAUDE.md is created holding exactly the block.
func Test_init_for_claude_code_creates_CLAUDE_md_with_the_block_alone(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	claudeMD := filepath.Join(wd, "CLAUDE.md")

	snippetArt := findArtifact(t, res, setup.KindSnippet)
	assert.Equal(t, setup.Artifact{Kind: setup.KindSnippet, Path: claudeMD, Action: setup.ActionCreated}, snippetArt)
	assert.Contains(t, res.Created, claudeMD)

	assert.Equal(t, string(artifact.SnippetBlock("docs/specifications"))+"\n", string(mem.Snapshot()[memKey(claudeMD)].Data))
}

// Test_init_for_claude_code_appends_the_block_to_an_existing_CLAUDE_md pins
// the merge-by-append case: a pre-existing CLAUDE.md with no brief block is
// reported ActionMerged and the block lands at the end, in Result.Modified
// rather than Created.
func Test_init_for_claude_code_appends_the_block_to_an_existing_CLAUDE_md(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	claudeMD := filepath.Join(wd, "CLAUDE.md")
	require.NoError(t, mem.WriteFile(memKey(claudeMD), []byte("# My project\n"), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	snippetArt := findArtifact(t, res, setup.KindSnippet)
	assert.Equal(t, setup.Artifact{Kind: setup.KindSnippet, Path: claudeMD, Action: setup.ActionMerged}, snippetArt)
	assert.Contains(t, res.Modified, claudeMD)
	assert.NotContains(t, res.Created, claudeMD)

	assert.Equal(t, "# My project\n\n"+string(artifact.SnippetBlock("docs/specifications"))+"\n", string(mem.Snapshot()[memKey(claudeMD)].Data))
}

// Test_init_for_claude_code_reports_unchanged_on_rerun pins convergence: a
// second run against a repository already carrying the current block
// reports ActionUnchanged and writes nothing further.
func Test_init_for_claude_code_reports_unchanged_on_rerun(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	snippetArt := findArtifact(t, res, setup.KindSnippet)
	assert.Equal(t, setup.ActionUnchanged, snippetArt.Action)
	assert.Empty(t, res.Created)
	assert.Empty(t, res.Modified)
}

// Test_init_for_claude_code_replaces_a_block_written_for_a_different_dir
// pins the "merged (block updated)" branch: a block brief-written for a
// feature directory other than the one now configured is replaced in place
// — Recognize is config-independent (OriginCurrent for its own dir), but
// the dir mismatch still means the bytes must change.
func Test_init_for_claude_code_replaces_a_block_written_for_a_different_dir(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	claudeMD := filepath.Join(wd, "CLAUDE.md")
	require.NoError(t, mem.WriteFile(memKey(claudeMD), artifact.SnippetBlock("elsewhere"), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	snippetArt := findArtifact(t, res, setup.KindSnippet)
	assert.Equal(t, setup.Artifact{Kind: setup.KindSnippet, Path: claudeMD, Action: setup.ActionMerged, Detail: "block updated"}, snippetArt)
	assert.Contains(t, res.Modified, claudeMD)

	assert.Equal(t, artifact.SnippetBlock("docs/specifications"), mem.Snapshot()[memKey(claudeMD)].Data)
}

// Test_init_for_claude_code_keeps_a_block_that_matches_no_render pins the
// "edited locally" branch: bytes between the markers that are not any known
// snippet render are kept, --force included — only Uninstall --force ever
// removes an edited block.
func Test_init_for_claude_code_keeps_a_block_that_matches_no_render(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	claudeMD := filepath.Join(wd, "CLAUDE.md")
	edited := artifact.SnippetBegin + "\nhand-written notes\n" + artifact.SnippetEnd + "\n"
	require.NoError(t, mem.WriteFile(memKey(claudeMD), []byte(edited), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, Force: true})

	require.NoError(t, err)

	snippetArt := findArtifact(t, res, setup.KindSnippet)
	assert.Equal(t, setup.Artifact{Kind: setup.KindSnippet, Path: claudeMD, Action: setup.ActionKept, Detail: "edited locally"}, snippetArt)

	assert.Equal(t, edited, string(mem.Snapshot()[memKey(claudeMD)].Data))
}

// Test_init_for_claude_code_keeps_a_CLAUDE_md_that_is_not_a_regular_file
// pins the Lstat guard, mirroring the plugin files' own: a symlink at
// CLAUDE.md's own path is kept, never followed, never written — but its
// body still prints (Result.Print), so a run this can never write through
// still shows the adopter what to add by hand. Mem.Lstat reports a
// fs.ModeSymlink entry as non-regular without following it, the same as
// real disk for a leaf symlink, so this is Mem-backed; the symlink can
// only be seeded at construction.
func Test_init_for_claude_code_keeps_a_CLAUDE_md_that_is_not_a_regular_file(t *testing.T) {
	wd := fsAbs("repo")
	claudeMD := filepath.Join(wd, "CLAUDE.md")
	elsewhere := filepath.Join(wd, "elsewhere.md")

	mem := rwfs.NewMem(fstest.MapFS{
		memKey(wd):        &fstest.MapFile{Mode: fs.ModeDir | 0o755},
		memKey(elsewhere): &fstest.MapFile{Data: []byte("elsewhere")},
		memKey(claudeMD):  &fstest.MapFile{Mode: fs.ModeSymlink, Data: []byte(elsewhere)},
	})
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	snippetArt := findArtifact(t, res, setup.KindSnippet)
	assert.Equal(t, setup.Artifact{
		Kind: setup.KindSnippet, Path: claudeMD, Action: setup.ActionKept,
		Detail: "not a regular file; add the block by hand, see 'brief init --print'",
	}, snippetArt)

	require.Contains(t, res.Print, setup.PrintArtifact{
		Path: claudeMD, Action: setup.PrintMerge, Body: string(artifact.SnippetBlock("docs/specifications")),
	})

	info, statErr := mem.Lstat(memKey(claudeMD))
	require.NoError(t, statErr)
	assert.Equal(t, fs.ModeSymlink, info.Mode()&fs.ModeSymlink)
}

// Test_init_for_claude_code_falls_back_to_dot_claude_CLAUDE_md pins the
// location rule's existence fallback: with no root CLAUDE.md but an
// existing ".claude/CLAUDE.md", the block installs there instead of
// creating a new root file.
func Test_init_for_claude_code_falls_back_to_dot_claude_CLAUDE_md(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	dotClaudeMD := filepath.Join(wd, ".claude", "CLAUDE.md")
	require.NoError(t, mem.MkdirAll(memKey(filepath.Dir(dotClaudeMD)), 0o755))
	require.NoError(t, mem.WriteFile(memKey(dotClaudeMD), []byte("notes\n"), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	snippetArt := findArtifact(t, res, setup.KindSnippet)
	assert.Equal(t, dotClaudeMD, snippetArt.Path)
	assert.Equal(t, setup.ActionMerged, snippetArt.Action)

	assert.NotContains(t, mem.Snapshot(), memKey(filepath.Join(wd, "CLAUDE.md")), "root CLAUDE.md must not be created when .claude/CLAUDE.md already exists")
}

// Test_init_for_claude_code_prefers_the_candidate_already_holding_a_block
// pins the "block wins" location rule: a block already installed in
// ".claude/CLAUDE.md" is updated in place, even though a root CLAUDE.md now
// also exists (created after the fact, e.g. by Claude Code's own /init) —
// R5 amended: root is preferred only as a fallback, never over a candidate
// that already holds the block.
func Test_init_for_claude_code_prefers_the_candidate_already_holding_a_block(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	dotClaudeMD := filepath.Join(wd, ".claude", "CLAUDE.md")
	require.NoError(t, mem.MkdirAll(memKey(filepath.Dir(dotClaudeMD)), 0o755))
	require.NoError(t, mem.WriteFile(memKey(dotClaudeMD), artifact.SnippetBlock("docs/specifications"), 0o600))
	rootClaudeMD := filepath.Join(wd, "CLAUDE.md")
	require.NoError(t, mem.WriteFile(memKey(rootClaudeMD), []byte("# root notes\n"), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	snippetArt := findArtifact(t, res, setup.KindSnippet)
	assert.Equal(t, dotClaudeMD, snippetArt.Path)
	assert.Equal(t, setup.ActionUnchanged, snippetArt.Action)

	assert.Equal(t, "# root notes\n", string(mem.Snapshot()[memKey(rootClaudeMD)].Data), "root CLAUDE.md must be left untouched")
}

// Test_init_for_claude_code_refuses_when_both_candidates_hold_a_block pins
// the cross-candidate refusal: naming ".claude/CLAUDE.md" (root is the
// preferred location) and its own begin line, and leaving both files
// byte-identical.
func Test_init_for_claude_code_refuses_when_both_candidates_hold_a_block(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	dotClaudeMD := filepath.Join(wd, ".claude", "CLAUDE.md")
	require.NoError(t, mem.MkdirAll(memKey(filepath.Dir(dotClaudeMD)), 0o755))
	dotClaudeBody := artifact.SnippetBlock("elsewhere")
	require.NoError(t, mem.WriteFile(memKey(dotClaudeMD), dotClaudeBody, 0o600))
	rootClaudeMD := filepath.Join(wd, "CLAUDE.md")
	rootBody := artifact.SnippetBlock("docs/specifications")
	require.NoError(t, mem.WriteFile(memKey(rootClaudeMD), rootBody, 0o600))
	srv := newMemServer(mem)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	var refusal *setup.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, dotClaudeMD, refusal.Path)
	assert.Equal(t, 1, refusal.Line)

	snap := mem.Snapshot()
	assert.Equal(t, dotClaudeBody, snap[memKey(dotClaudeMD)].Data)
	assert.Equal(t, rootBody, snap[memKey(rootClaudeMD)].Data)
	assert.NotContains(t, snap, memKey(filepath.Join(wd, ".brief.yaml")), "no files changed")
}

// Test_init_for_claude_code_refuses_on_a_marker_defect pins every marker
// defect the plan names, each citing its own path and 1-based line, with no
// files written.
func Test_init_for_claude_code_refuses_on_a_marker_defect(t *testing.T) {
	begin := artifact.SnippetBegin
	end := artifact.SnippetEnd

	cases := []struct {
		name     string
		body     string
		wantLine int
	}{
		{name: "lone begin", body: "notes\n" + begin + "\n", wantLine: 2},
		{name: "lone end", body: "notes\n" + end + "\n", wantLine: 2},
		{name: "end before begin", body: end + "\n" + begin + "\n" + end + "\n", wantLine: 1},
		{name: "second begin, one file", body: begin + "\n" + end + "\n" + begin + "\n" + end + "\n", wantLine: 3},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := fsAbs("repo")
			mem := newVirtualMem(wd)
			claudeMD := filepath.Join(wd, "CLAUDE.md")
			require.NoError(t, mem.WriteFile(memKey(claudeMD), []byte(c.body), 0o600))
			srv := newMemServer(mem)

			_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

			var refusal *setup.RefusalError
			require.ErrorAs(t, err, &refusal)
			assert.Equal(t, claudeMD, refusal.Path)
			assert.Equal(t, c.wantLine, refusal.Line)

			snap := mem.Snapshot()
			assert.Equal(t, c.body, string(snap[memKey(claudeMD)].Data))
			assert.NotContains(t, snap, memKey(filepath.Join(wd, ".brief.yaml")), "no files changed")
		})
	}
}

// Test_init_for_claude_code_refuses_a_CRLF_chosen_candidate pins the CRLF
// refusal scoped to the one candidate Init would actually write to: no
// line number, and nothing written.
func Test_init_for_claude_code_refuses_a_CRLF_chosen_candidate(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	claudeMD := filepath.Join(wd, "CLAUDE.md")
	crlf := "# notes\r\nmore\r\n"
	require.NoError(t, mem.WriteFile(memKey(claudeMD), []byte(crlf), 0o600))
	srv := newMemServer(mem)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	var refusal *setup.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, claudeMD, refusal.Path)
	assert.Equal(t, 0, refusal.Line)

	snap := mem.Snapshot()
	assert.Equal(t, crlf, string(snap[memKey(claudeMD)].Data))
	assert.NotContains(t, snap, memKey(filepath.Join(wd, ".brief.yaml")), "no files changed")
}

// Test_init_for_claude_code_does_not_refuse_on_an_untouched_CRLF_candidate
// is the control arm proving CRLF is scoped to the chosen candidate, not
// blanket across both: root CLAUDE.md exists (clean, no block) and is the
// one Init actually writes to; a CRLF ".claude/CLAUDE.md" it never touches
// must not block the run.
func Test_init_for_claude_code_does_not_refuse_on_an_untouched_CRLF_candidate(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	rootClaudeMD := filepath.Join(wd, "CLAUDE.md")
	require.NoError(t, mem.WriteFile(memKey(rootClaudeMD), []byte("# root notes\n"), 0o600))
	dotClaudeMD := filepath.Join(wd, ".claude", "CLAUDE.md")
	require.NoError(t, mem.MkdirAll(memKey(filepath.Dir(dotClaudeMD)), 0o755))
	require.NoError(t, mem.WriteFile(memKey(dotClaudeMD), []byte("host notes\r\n"), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	snippetArt := findArtifact(t, res, setup.KindSnippet)
	assert.Equal(t, rootClaudeMD, snippetArt.Path)
	assert.Equal(t, setup.ActionMerged, snippetArt.Action)

	assert.Equal(t, "host notes\r\n", string(mem.Snapshot()[memKey(dotClaudeMD)].Data), "the untouched CRLF candidate must survive byte-identical")
}

// Test_uninstall_for_claude_code_removes_the_block_leaving_the_rest pins
// the "removed CLAUDE.md (brief block)" branch: a pre-existing file with
// unrelated content plus the current block is stripped down to exactly the
// unrelated content, reported ActionRemoved and listed in Result.Modified,
// never Removed — the file itself survives.
func Test_uninstall_for_claude_code_removes_the_block_leaving_the_rest(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	claudeMD := filepath.Join(wd, "CLAUDE.md")
	before := mem.Snapshot()[memKey(claudeMD)].Data
	require.NoError(t, mem.WriteFile(memKey(claudeMD), append([]byte("# My project\n\n"), before...), 0o600))

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	snippetArt := findArtifact(t, res, setup.KindSnippet)
	assert.Equal(t, setup.Artifact{Kind: setup.KindSnippet, Path: claudeMD, Action: setup.ActionRemoved, Detail: "brief block"}, snippetArt)
	assert.Contains(t, res.Modified, claudeMD)
	assert.NotContains(t, res.Removed, claudeMD)

	assert.Equal(t, "# My project\n", string(mem.Snapshot()[memKey(claudeMD)].Data))
}

// Test_uninstall_for_claude_code_deletes_a_CLAUDE_md_that_is_only_the_block
// pins the "removed CLAUDE.md" (file deleted) branch: a CLAUDE.md holding
// only the block, as Init itself creates it, is deleted entirely once the
// block comes out — the only "brief created it" signal this package keeps.
func Test_uninstall_for_claude_code_deletes_a_CLAUDE_md_that_is_only_the_block(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)
	claudeMD := filepath.Join(wd, "CLAUDE.md")

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	snippetArt := findArtifact(t, res, setup.KindSnippet)
	assert.Equal(t, setup.Artifact{Kind: setup.KindSnippet, Path: claudeMD, Action: setup.ActionRemoved}, snippetArt)
	assert.Contains(t, res.Removed, claudeMD)
	assert.NotContains(t, res.Modified, claudeMD)

	assert.NotContains(t, mem.Snapshot(), memKey(claudeMD))
}

// Test_uninstall_for_claude_code_keeps_an_edited_block_unless_forced pins
// the "edited locally" kept branch: without --force the block survives
// byte-identical.
func Test_uninstall_for_claude_code_keeps_an_edited_block_unless_forced(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	claudeMD := filepath.Join(wd, "CLAUDE.md")
	edited := artifact.SnippetBegin + "\nhand-written notes\n" + artifact.SnippetEnd + "\n"
	require.NoError(t, mem.WriteFile(memKey(claudeMD), []byte(edited), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	snippetArt := findArtifact(t, res, setup.KindSnippet)
	assert.Equal(t, setup.Artifact{Kind: setup.KindSnippet, Path: claudeMD, Action: setup.ActionKept, Detail: "edited locally", ForceRemovable: true}, snippetArt)

	assert.Equal(t, edited, string(mem.Snapshot()[memKey(claudeMD)].Data))
}

// Test_uninstall_force_removes_an_edited_block_following_the_same_removed_shape
// pins --force's own override: an edited block is still removed, and the
// resulting row follows the same "brief block kept" / "file deleted" shape
// as an unedited removal — "edited locally" only ever describes ActionKept,
// never a forced ActionRemoved.
func Test_uninstall_force_removes_an_edited_block_following_the_same_removed_shape(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	claudeMD := filepath.Join(wd, "CLAUDE.md")
	edited := "# notes\n\n" + artifact.SnippetBegin + "\nhand-written notes\n" + artifact.SnippetEnd + "\n"
	require.NoError(t, mem.WriteFile(memKey(claudeMD), []byte(edited), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode, Force: true})

	require.NoError(t, err)

	snippetArt := findArtifact(t, res, setup.KindSnippet)
	assert.Equal(t, setup.Artifact{Kind: setup.KindSnippet, Path: claudeMD, Action: setup.ActionRemoved, Detail: "brief block"}, snippetArt)
	assert.Contains(t, res.Modified, claudeMD)

	assert.Equal(t, "# notes\n", string(mem.Snapshot()[memKey(claudeMD)].Data))
}

// Test_uninstall_for_claude_code_plans_no_row_when_no_block_is_installed
// pins the missing-block branch: a CLAUDE.md that exists but carries no
// brief block plans no row at all and is left untouched.
func Test_uninstall_for_claude_code_plans_no_row_when_no_block_is_installed(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	claudeMD := filepath.Join(wd, "CLAUDE.md")
	require.NoError(t, mem.WriteFile(memKey(claudeMD), []byte("# My project\n"), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	for _, a := range res.Artifacts {
		assert.NotEqual(t, setup.KindSnippet, a.Kind)
	}

	assert.Equal(t, "# My project\n", string(mem.Snapshot()[memKey(claudeMD)].Data))
}

// Test_uninstall_for_claude_code_refuses_on_a_marker_defect pins the same
// refusal set Init reports, reached through Uninstall instead.
func Test_uninstall_for_claude_code_refuses_on_a_marker_defect(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	claudeMD := filepath.Join(wd, "CLAUDE.md")
	body := "notes\n" + artifact.SnippetBegin + "\n"
	require.NoError(t, mem.WriteFile(memKey(claudeMD), []byte(body), 0o600))
	srv := newMemServer(mem)

	_, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	var refusal *setup.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, claudeMD, refusal.Path)
	assert.Equal(t, 2, refusal.Line)
}

// Test_uninstall_for_host_none_leaves_the_CLAUDE_md_block_in_place pins
// R5's own host gate: --host none never touches CLAUDE.md.
func Test_uninstall_for_host_none_leaves_the_CLAUDE_md_block_in_place(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	for _, a := range res.Artifacts {
		assert.NotEqual(t, setup.KindSnippet, a.Kind)
	}

	require.Contains(t, mem.Snapshot(), memKey(filepath.Join(wd, "CLAUDE.md")), "the CLAUDE.md block must survive an uninstall scoped to --host none")
}
