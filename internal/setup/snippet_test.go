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

// Only Uninstall --force ever removes an edited block.
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

// The symlink is seeded at construction: rwfs.Mem has no post-construction
// symlink-creating method.
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
