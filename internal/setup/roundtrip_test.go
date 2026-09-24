package setup_test

import (
	"maps"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_init_then_uninstall_leaves_the_tree_as_before_except_the_feature_root
// pins R6's own asymmetry end to end: against a repository already
// carrying unrelated files (a root README, a nested source file, a
// ".claude/" file), Init then Uninstall (default, unedited config)
// reproduces the exact pre-existing tree, plus exactly the empty feature
// root directory Init created and Uninstall deliberately never removes —
// asserted present, not merely tolerated if it happened to still be there.
func Test_init_then_uninstall_leaves_the_tree_as_before_except_the_feature_root(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	require.NoError(t, mem.WriteFile(memKey(wd)+"/README.md", []byte("hello\n"), 0o600))
	require.NoError(t, mem.MkdirAll(memKey(wd)+"/src/pkg", 0o755))
	require.NoError(t, mem.WriteFile(memKey(wd)+"/src/pkg/main.go", []byte("package pkg\n"), 0o600))
	require.NoError(t, mem.MkdirAll(memKey(wd)+"/.claude", 0o755))
	require.NoError(t, mem.WriteFile(memKey(wd)+"/.claude/notes.md", []byte("notes\n"), 0o600))

	before := memTree(mem.Snapshot(), wd)

	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	_, err = srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})
	require.NoError(t, err)

	after := memTree(mem.Snapshot(), wd)

	featureRootRel := filepath.Join("docs", "specifications")

	expected := map[string]memTreeEntry{}
	maps.Copy(expected, before)
	expected["docs"] = memTreeEntry{isDir: true}
	expected[featureRootRel] = memTreeEntry{isDir: true}

	assert.Equal(t, expected, after)

	entry, ok := after[featureRootRel]
	require.True(t, ok, "feature root must survive uninstall")
	assert.True(t, entry.isDir)
}

// Test_init_then_uninstall_for_claude_code_leaves_pre_existing_claude_files_byte_identical
// pins R6 end to end for the plugin: against a repository already carrying
// a host's own files at ".claude/settings.json" and
// ".claude/skills/other/SKILL.md" (both above host.PluginDir, so brief
// never touches them), Init then Uninstall for claude-code reproduces the
// exact pre-existing tree plus the feature root Init created — the control
// proving the snapshot actually changed after Init, so "unchanged after
// round trip" is not vacuously true of a run that wrote nothing.
func Test_init_then_uninstall_for_claude_code_leaves_pre_existing_claude_files_byte_identical(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	require.NoError(t, mem.MkdirAll(memKey(wd)+"/.claude/skills/other", 0o755))
	require.NoError(t, mem.WriteFile(memKey(wd)+"/.claude/settings.json", []byte(`{"env":{}}`+"\n"), 0o600))
	require.NoError(t, mem.WriteFile(memKey(wd)+"/.claude/skills/other/SKILL.md", []byte("---\ndescription: mine\n---\nhello\n"), 0o600))

	before := memTree(mem.Snapshot(), wd)

	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	afterInit := memTree(mem.Snapshot(), wd)
	assert.NotEqual(t, before, afterInit, "init must actually have written the plugin")
	assert.Contains(t, afterInit, filepath.Join(".claude", "skills", "brief", ".claude-plugin", "plugin.json"))

	_, err = srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	after := memTree(mem.Snapshot(), wd)

	featureRootRel := filepath.Join("docs", "specifications")

	expected := map[string]memTreeEntry{}
	maps.Copy(expected, before)
	expected["docs"] = memTreeEntry{isDir: true}
	expected[featureRootRel] = memTreeEntry{isDir: true}

	assert.Equal(t, expected, after)
}

// Test_init_then_uninstall_leaves_claude_md_byte_identical pins R5's own
// round trip across every append shape the byte rules distinguish, plus
// the ".claude/CLAUDE.md" fallback and the "nothing there at all" case:
// each asserts the tree actually changed after Init (so "restored after
// round trip" is never vacuously true of a run that wrote nothing) before
// asserting Uninstall restores it byte-identical — or, when nothing
// preceded Init, absent again afterward.
func Test_init_then_uninstall_leaves_claude_md_byte_identical(t *testing.T) {
	cases := []struct {
		name    string
		relPath string // "" means no pre-existing CLAUDE.md at all
		body    string
	}{
		{name: "no trailing newline", relPath: "CLAUDE.md", body: "# notes"},
		{name: "one trailing newline", relPath: "CLAUDE.md", body: "# notes\n"},
		{name: "trailing blank line", relPath: "CLAUDE.md", body: "# notes\n\n"},
		{name: ".claude/CLAUDE.md fallback", relPath: filepath.Join(".claude", "CLAUDE.md"), body: "host notes\n"},
		{name: "no CLAUDE.md at all", relPath: "", body: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := fsAbs("repo")
			mem := newVirtualMem(wd)

			if c.relPath != "" {
				if dir := filepath.Dir(c.relPath); dir != "." {
					require.NoError(t, mem.MkdirAll(memKey(wd)+"/"+filepath.ToSlash(dir), 0o755))
				}
				require.NoError(t, mem.WriteFile(memKey(wd)+"/"+filepath.ToSlash(c.relPath), []byte(c.body), 0o600))
			}

			before := memTree(mem.Snapshot(), wd)

			srv := newMemServer(mem)
			_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
			require.NoError(t, err)

			afterInit := memTree(mem.Snapshot(), wd)
			assert.NotEqual(t, before, afterInit, "init must actually have written the CLAUDE.md block")

			_, err = srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
			require.NoError(t, err)

			after := memTree(mem.Snapshot(), wd)

			featureRootRel := filepath.Join("docs", "specifications")

			expected := map[string]memTreeEntry{}
			maps.Copy(expected, before)
			expected["docs"] = memTreeEntry{isDir: true}
			expected[featureRootRel] = memTreeEntry{isDir: true}
			// A claude-code Init always creates ".claude/skills/brief/..."; Uninstall
			// prunes empty directories only down to and including host.PluginDir
			// (R6 never removes ".claude/skills/" or ".claude/" themselves), so both
			// survive, empty, regardless of what the CLAUDE.md fixture pre-created.
			expected[".claude"] = memTreeEntry{isDir: true}
			expected[filepath.Join(".claude", "skills")] = memTreeEntry{isDir: true}

			assert.Equal(t, expected, after)
		})
	}
}

// Test_init_with_agents_then_uninstall_leaves_the_tree_as_before pins the
// round trip end to end for --with-agents: against a repository with
// nothing pre-existing, Init --with-agents then Uninstall reproduces the
// exact pre-existing tree, plus exactly the empty feature root Init
// created — the control (afterInit != before) proves this run actually
// wrote something, and ".claude/skills/brief/" is gone entirely while
// ".claude/skills/" and ".claude/" themselves — never brief's to remove —
// survive empty, the same boundary a plain claude-code round trip already
// pins.
func Test_init_with_agents_then_uninstall_leaves_the_tree_as_before(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	before := memTree(mem.Snapshot(), wd)

	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	afterInit := memTree(mem.Snapshot(), wd)
	assert.NotEqual(t, before, afterInit, "init --with-agents must actually have written something")
	assert.Contains(t, afterInit, filepath.Join(".claude", "skills", "brief", "agents", "planner.md"))

	_, err = srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	after := memTree(mem.Snapshot(), wd)

	featureRootRel := filepath.Join("docs", "specifications")

	expected := map[string]memTreeEntry{}
	maps.Copy(expected, before)
	expected["docs"] = memTreeEntry{isDir: true}
	expected[featureRootRel] = memTreeEntry{isDir: true}
	expected[".claude"] = memTreeEntry{isDir: true}
	expected[filepath.Join(".claude", "skills")] = memTreeEntry{isDir: true}

	assert.Equal(t, expected, after)

	assert.NotContains(t, after, filepath.Join(".claude", "skills", "brief"), "the plugin directory, agents/ included, must be fully removed")
}

// Test_init_then_uninstall_deletes_a_pre_existing_empty_CLAUDE_md pins the
// accepted exception R6's own "brief created it" signal is exposed to: an
// emptied CLAUDE.md is deleted on uninstall, and that rule cannot tell a
// file brief emptied apart from one that started empty — so a pre-existing,
// already-empty CLAUDE.md is deleted too, not restored as an empty file.
func Test_init_then_uninstall_deletes_a_pre_existing_empty_CLAUDE_md(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	claudeMD := filepath.Join(wd, "CLAUDE.md")
	require.NoError(t, mem.WriteFile(memKey(claudeMD), []byte{}, 0o600))

	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	_, err = srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	assert.NotContains(t, mem.Snapshot(), memKey(claudeMD), "a pre-existing empty CLAUDE.md is deleted, not restored")
}
