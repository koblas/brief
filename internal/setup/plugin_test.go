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
// under one root: manifest, start skill, finish skill, hook wiring, the
// brief-workflow skill, and the CLAUDE.md block's default location.
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

// Created is asserted in write order; Artifacts is compared unordered
// (ElementsMatch) since row order is pinned elsewhere.
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

	assert.ElementsMatch(t, []setup.Artifact{
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

// Claude Code loads a project skills-dir plugin only from the session's own
// working directory, no walk-up, so a plugin under wd would never be found.
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

func Test_init_with_no_hook_installs_everything_but_the_hook(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, NoHook: true})

	require.NoError(t, err)

	configPath := filepath.Join(wd, ".brief.yaml")
	featureRoot := filepath.Join(wd, "docs", "specifications")
	paths := pluginFilePaths(wd)

	assert.ElementsMatch(t, []setup.Artifact{
		{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated},
		{Kind: setup.KindFeatureRoot, Path: featureRoot, Action: setup.ActionCreated},
		{Kind: setup.KindPlugin, Path: paths.Manifest, Action: setup.ActionCreated},
		{Kind: setup.KindPlugin, Path: paths.Start, Action: setup.ActionCreated},
		{Kind: setup.KindPlugin, Path: paths.Finish, Action: setup.ActionCreated},
		{Kind: setup.KindSkill, Path: paths.Skill, Action: setup.ActionCreated},
		{Kind: setup.KindSnippet, Path: paths.ClaudeMD, Action: setup.ActionCreated},
	}, res.Artifacts)

	hooks := paths.Hooks
	assert.NotContains(t, mem.Snapshot(), memKey(hooks))
	assert.NotContains(t, res.Created, hooks)
}

func Test_rerunning_init_for_claude_code_reports_every_plugin_file_unchanged(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	assert.Empty(t, res.Created)

	configPath := filepath.Join(wd, ".brief.yaml")
	featureRoot := filepath.Join(wd, "docs", "specifications")
	paths := pluginFilePaths(wd)
	wantActions := map[string]setup.Action{
		configPath:     setup.ActionUnchanged,
		featureRoot:    setup.ActionUnchanged,
		paths.Manifest: setup.ActionUnchanged,
		paths.Start:    setup.ActionUnchanged,
		paths.Finish:   setup.ActionUnchanged,
		paths.Hooks:    setup.ActionUnchanged,
		paths.Skill:    setup.ActionUnchanged,
		paths.ClaudeMD: setup.ActionUnchanged,
	}

	gotActions := make(map[string]setup.Action, len(res.Artifacts))
	for _, a := range res.Artifacts {
		gotActions[a.Path] = a.Action
	}
	assert.Equal(t, wantActions, gotActions)
}

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

	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated, Detail: "rewritten from defaults"}, findArtifactByPath(t, res, configPath))
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: start, Action: setup.ActionKept, Detail: "edited locally"}, findArtifactByPath(t, res, start))

	assert.Equal(t, edited, mem.Snapshot()[memKey(start)].Data)
}

// The symlink is seeded at construction: rwfs.Mem has no post-construction
// symlink-creating method.
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
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: manifest, Action: setup.ActionKept, Detail: "not a regular file"}, findArtifactByPath(t, res, manifest))
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: finish, Action: setup.ActionKept, Detail: "not a regular file"}, findArtifactByPath(t, res, finish))

	info, statErr := mem.Lstat(memKey(finish))
	require.NoError(t, statErr)
	assert.Equal(t, fs.ModeSymlink, info.Mode()&fs.ModeSymlink)
}

func Test_init_dry_run_for_claude_code_writes_nothing(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	before := mem.Snapshot()
	srv, rec := newMemServerRecording(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, DryRun: true})

	require.NoError(t, err)

	configPath := filepath.Join(wd, ".brief.yaml")
	featureRoot := filepath.Join(wd, "docs", "specifications")
	paths := pluginFilePaths(wd)
	assert.ElementsMatch(t, []setup.Artifact{
		{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated},
		{Kind: setup.KindFeatureRoot, Path: featureRoot, Action: setup.ActionCreated},
		{Kind: setup.KindPlugin, Path: paths.Manifest, Action: setup.ActionCreated},
		{Kind: setup.KindPlugin, Path: paths.Start, Action: setup.ActionCreated},
		{Kind: setup.KindPlugin, Path: paths.Finish, Action: setup.ActionCreated},
		{Kind: setup.KindHook, Path: paths.Hooks, Action: setup.ActionCreated},
		{Kind: setup.KindSkill, Path: paths.Skill, Action: setup.ActionCreated},
		{Kind: setup.KindSnippet, Path: paths.ClaudeMD, Action: setup.ActionCreated},
	}, res.Artifacts)
	assert.Empty(t, res.Created)
	assert.Equal(t, before, mem.Snapshot())
	assert.Empty(t, rec.calls, "DryRun must never reach R10's own writability pre-check")

	srv2, rec2 := newMemServerRecording(newVirtualMem(wd))
	_, err = srv2.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)
	assert.Len(t, rec2.calls, 1, "a real run must call R10's own writability pre-check exactly once")
}
