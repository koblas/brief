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

// pluginPaths names the four claude-code plugin files' absolute paths
// under one root, in R11's own stdout order: manifest, start skill,
// finish skill, hook wiring, the brief-workflow skill (outside the plugin
// directory), plus the CLAUDE.md instruction block's own default (root)
// location.
type pluginPaths struct {
	Manifest, Start, Finish, Hooks, Skill, ClaudeMD string
}

// pluginFilePaths returns root's own pluginPaths.
func pluginFilePaths(root string) pluginPaths {
	base := filepath.Join(root, ".claude", "skills", "brief")

	return pluginPaths{
		Manifest: filepath.Join(base, ".claude-plugin", "plugin.json"),
		Start:    filepath.Join(base, "skills", "start", "SKILL.md"),
		Finish:   filepath.Join(base, "skills", "finish", "SKILL.md"),
		Hooks:    filepath.Join(base, "hooks", "hooks.json"),
		Skill:    skillFilePath(root),
		ClaudeMD: filepath.Join(root, "CLAUDE.md"),
	}
}

// Test_init_for_claude_code_writes_the_plugin_after_the_feature_root_and_before_the_config
// pins R4/R11 for a fresh repository: the plugin's own four rows and the
// CLAUDE.md block land after config and feature root, each reports
// ActionCreated with KindPlugin (KindHook for hooks.json, KindSnippet for
// CLAUDE.md), the bytes written equal their own artifact.Render (or
// artifact.SnippetBlock for CLAUDE.md), Created lists files only in write
// order, and Result.Root is the install root. The package's own
// index-by-index full-row-order pin is agents_test.go's --with-agents Init
// (Test_init_with_agents_writes_three_agents_and_a_config_binding_them);
// this test still pins the same order for the plain, no-agents case, but
// as one whole-slice equality rather than per-index assertions.
func Test_init_for_claude_code_writes_the_plugin_after_the_feature_root_and_before_the_config(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	assert.Equal(t, wd, res.Root)

	configPath := filepath.Join(wd, ".brief.yaml")
	featureRoot := filepath.Join(wd, "docs", "specifications")
	paths := pluginFilePaths(wd)

	assert.Equal(t, []setup.Artifact{
		{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated},
		{Kind: setup.KindFeatureRoot, Path: featureRoot, Action: setup.ActionCreated},
		{Kind: setup.KindPlugin, Path: paths.Manifest, Action: setup.ActionCreated},
		{Kind: setup.KindPlugin, Path: paths.Start, Action: setup.ActionCreated},
		{Kind: setup.KindPlugin, Path: paths.Finish, Action: setup.ActionCreated},
		{Kind: setup.KindHook, Path: paths.Hooks, Action: setup.ActionCreated},
		{Kind: setup.KindSkill, Path: paths.Skill, Action: setup.ActionCreated},
		{Kind: setup.KindSnippet, Path: paths.ClaudeMD, Action: setup.ActionCreated},
	}, res.Artifacts)

	assert.Equal(t, []string{featureRoot, paths.Manifest, paths.Start, paths.Finish, paths.Hooks, paths.Skill, paths.ClaudeMD, configPath}, res.Created)

	snap := mem.Snapshot()
	assert.Equal(t, artifact.PluginManifest(), snap[memKey(paths.Manifest)].Data)
	assert.Equal(t, artifact.SkillStart(), snap[memKey(paths.Start)].Data)
	assert.Equal(t, artifact.SkillFinish(), snap[memKey(paths.Finish)].Data)
	assert.Equal(t, artifact.ClaudeHooks(), snap[memKey(paths.Hooks)].Data)
	assert.Equal(t, artifact.SkillWorkflow(), snap[memKey(paths.Skill)].Data)
}

// Test_init_from_a_subdirectory_installs_the_plugin_at_the_config_root pins
// the binding decision that the plugin shares the config's own root rule:
// a config found walking up from wd plants the plugin at that ancestor,
// never under wd itself — Claude Code loads a project skills-dir plugin
// only from the session's own working directory, no walk-up, so a plugin
// under wd here would never be found.
func Test_init_from_a_subdirectory_installs_the_plugin_at_the_config_root(t *testing.T) {
	parent := fsAbs("repo")
	mem := newVirtualMem(parent)
	require.NoError(t, mem.WriteFile(memKey(parent)+"/.brief.yaml", []byte("feature-directory: specs\n"), 0o600))
	require.NoError(t, mem.Mkdir(memKey(parent)+"/child", 0o755))
	child := filepath.Join(parent, "child")

	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), child, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	assert.Equal(t, parent, res.Root)

	snap := mem.Snapshot()
	assert.Equal(t, artifact.PluginManifest(), snap[memKey(pluginFilePaths(parent).Manifest)].Data)
	assert.NotContains(t, snap, memKey(child)+"/.claude")
}

// Test_init_with_no_hook_installs_everything_but_the_hook pins NoHook: the
// control arm is Test_init_for_claude_code_writes_the_plugin_after_the_feature_root_and_before_the_config's
// own eight-row run; with NoHook set, the manifest, both skills, the
// brief-workflow skill and the CLAUDE.md block still install identically
// but hooks.json is planned, written, or reported not at all — R5's block
// is never a host.File, so --no-hook cannot affect it.
func Test_init_with_no_hook_installs_everything_but_the_hook(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, NoHook: true})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 7)

	kinds := make([]setup.Kind, len(res.Artifacts))
	for i, a := range res.Artifacts {
		kinds[i] = a.Kind
	}
	assert.Equal(t, []setup.Kind{setup.KindConfig, setup.KindFeatureRoot, setup.KindPlugin, setup.KindPlugin, setup.KindPlugin, setup.KindSkill, setup.KindSnippet}, kinds)

	hooks := pluginFilePaths(wd).Hooks
	assert.NotContains(t, mem.Snapshot(), memKey(hooks))
	assert.NotContains(t, res.Created, hooks)
}

// Test_rerunning_init_for_claude_code_reports_every_plugin_file_unchanged
// pins convergence (R3) for the plugin, the skill and the CLAUDE.md block:
// a second, identical run reports every one of the eight artifacts
// ActionUnchanged and writes nothing further.
func Test_rerunning_init_for_claude_code_reports_every_plugin_file_unchanged(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	assert.Empty(t, res.Created)
	require.Len(t, res.Artifacts, 8)
	for _, a := range res.Artifacts {
		assert.Equal(t, setup.ActionUnchanged, a.Action, "artifact %s must report unchanged", a.Path)
	}
}

// Test_init_keeps_an_edited_plugin_file_even_under_force pins R3's
// asymmetry: --force rewrites only the config (the control arm here,
// "rewritten from defaults") and never touches an edited plugin file —
// the start skill, edited before this run, keeps its edited bytes
// byte-identical, reported ActionKept, detail "edited locally".
func Test_init_keeps_an_edited_plugin_file_even_under_force(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, mem.WriteFile(memKey(configPath), []byte("feature-directory: specs\n"), 0o600))

	start := pluginFilePaths(wd).Start
	edited := []byte("---\nedited by hand\n---\n")
	require.NoError(t, mem.WriteFile(memKey(start), edited, 0o600))

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, Force: true})

	require.NoError(t, err)

	require.Len(t, res.Artifacts, 8)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated, Detail: "rewritten from defaults"}, findArtifactByPath(t, res, configPath))
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: start, Action: setup.ActionKept, Detail: "edited locally"}, findArtifactByPath(t, res, start))

	assert.Equal(t, edited, mem.Snapshot()[memKey(start)].Data)
}

// Test_init_keeps_a_plugin_path_that_is_not_a_regular_file pins the Lstat
// guard, mirroring Uninstall's own: a directory at the manifest's own path
// and a symlink at the finish skill's own path are both kept, detail "not
// a regular file", neither followed nor written. Mem.Lstat reports a
// fs.ModeSymlink entry as non-regular without following it, the same as
// real disk for a leaf (never an ancestor) symlink, so this is Mem-backed;
// the symlink itself can only be seeded at construction (rwfs.Mem has no
// post-construction symlink-creating method).
func Test_init_keeps_a_plugin_path_that_is_not_a_regular_file(t *testing.T) {
	wd := fsAbs("repo")
	paths := pluginFilePaths(wd)
	manifest, finish := paths.Manifest, paths.Finish
	elsewhere := filepath.Join(wd, "elsewhere.md")

	mem := rwfs.NewMem(fstest.MapFS{
		memKey(wd):        &fstest.MapFile{Mode: fs.ModeDir | 0o755},
		memKey(manifest):  &fstest.MapFile{Mode: fs.ModeDir | 0o755},
		memKey(elsewhere): &fstest.MapFile{Data: []byte("elsewhere")},
		memKey(finish):    &fstest.MapFile{Mode: fs.ModeSymlink, Data: []byte(elsewhere)},
	})
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 8)
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: manifest, Action: setup.ActionKept, Detail: "not a regular file"}, findArtifactByPath(t, res, manifest))
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: finish, Action: setup.ActionKept, Detail: "not a regular file"}, findArtifactByPath(t, res, finish))

	info, statErr := mem.Lstat(memKey(finish))
	require.NoError(t, statErr)
	assert.Equal(t, fs.ModeSymlink, info.Mode()&fs.ModeSymlink)
}

// Test_init_dry_run_for_claude_code_writes_nothing pins R9 for the plugin:
// the same eight rows a real run would report, no bytes written to mem,
// and R10's own writability pre-check never even runs (the writableRecorder
// stays empty) — the control arm, a real run, calls it exactly once.
func Test_init_dry_run_for_claude_code_writes_nothing(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	before := mem.Snapshot()
	srv, rec := newMemServerRecording(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, DryRun: true})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 8)
	assert.Empty(t, res.Created)
	assert.Equal(t, before, mem.Snapshot())
	assert.Empty(t, rec.calls, "DryRun must never reach R10's own writability pre-check")

	srv2, rec2 := newMemServerRecording(newVirtualMem(wd))
	_, err = srv2.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)
	assert.Len(t, rec2.calls, 1, "a real run must call R10's own writability pre-check exactly once")
}
