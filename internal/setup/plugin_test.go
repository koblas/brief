package setup_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
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
// CLAUDE.md), the bytes on disk equal their own artifact.Render (or
// artifact.SnippetBlock for CLAUDE.md), Created lists files only in write
// order, and Result.Root is the install root.
func Test_init_for_claude_code_writes_the_plugin_after_the_feature_root_and_before_the_config(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	assert.Equal(t, wd, res.Root)

	configPath := filepath.Join(wd, ".brief.yaml")
	featureRoot := filepath.Join(wd, "docs", "specifications")
	paths := pluginFilePaths(wd)

	require.Len(t, res.Artifacts, 8)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated}, res.Artifacts[0])
	assert.Equal(t, setup.Artifact{Kind: setup.KindFeatureRoot, Path: featureRoot, Action: setup.ActionCreated}, res.Artifacts[1])
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: paths.Manifest, Action: setup.ActionCreated}, res.Artifacts[2])
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: paths.Start, Action: setup.ActionCreated}, res.Artifacts[3])
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: paths.Finish, Action: setup.ActionCreated}, res.Artifacts[4])
	assert.Equal(t, setup.Artifact{Kind: setup.KindHook, Path: paths.Hooks, Action: setup.ActionCreated}, res.Artifacts[5])
	assert.Equal(t, setup.Artifact{Kind: setup.KindSkill, Path: paths.Skill, Action: setup.ActionCreated}, res.Artifacts[6])
	assert.Equal(t, setup.Artifact{Kind: setup.KindSnippet, Path: paths.ClaudeMD, Action: setup.ActionCreated}, res.Artifacts[7])

	assert.Equal(t, []string{featureRoot, paths.Manifest, paths.Start, paths.Finish, paths.Hooks, paths.Skill, paths.ClaudeMD, configPath}, res.Created)

	body, readErr := os.ReadFile(paths.Manifest)
	require.NoError(t, readErr)
	assert.Equal(t, artifact.PluginManifest(), body)

	body, readErr = os.ReadFile(paths.Start)
	require.NoError(t, readErr)
	assert.Equal(t, artifact.SkillStart(), body)

	body, readErr = os.ReadFile(paths.Finish)
	require.NoError(t, readErr)
	assert.Equal(t, artifact.SkillFinish(), body)

	body, readErr = os.ReadFile(paths.Hooks)
	require.NoError(t, readErr)
	assert.Equal(t, artifact.ClaudeHooks(), body)

	body, readErr = os.ReadFile(paths.Skill)
	require.NoError(t, readErr)
	assert.Equal(t, artifact.SkillWorkflow(), body)
}

// Test_init_from_a_subdirectory_installs_the_plugin_at_the_config_root pins
// the binding decision that the plugin shares the config's own root rule:
// a config found walking up from wd plants the plugin at that ancestor,
// never under wd itself — Claude Code loads a project skills-dir plugin
// only from the session's own working directory, no walk-up, so a plugin
// under wd here would never be found.
func Test_init_from_a_subdirectory_installs_the_plugin_at_the_config_root(t *testing.T) {
	parent := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(parent, ".brief.yaml"), []byte("feature-directory: specs\n"), 0o600))
	child := filepath.Join(parent, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	srv := newServer(t)

	res, err := srv.Init(t.Context(), child, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	assert.Equal(t, parent, res.Root)

	body, readErr := os.ReadFile(pluginFilePaths(parent).Manifest)
	require.NoError(t, readErr)
	assert.Equal(t, artifact.PluginManifest(), body)

	_, statErr := os.Stat(filepath.Join(child, ".claude"))
	assert.True(t, os.IsNotExist(statErr))
}

// Test_init_with_no_hook_installs_everything_but_the_hook pins NoHook: the
// control arm is Test_init_for_claude_code_writes_the_plugin_after_the_feature_root_and_before_the_config's
// own eight-row run; with NoHook set, the manifest, both skills, the
// brief-workflow skill and the CLAUDE.md block still install identically
// but hooks.json is planned, written, or reported not at all — R5's block
// is never a host.File, so --no-hook cannot affect it.
func Test_init_with_no_hook_installs_everything_but_the_hook(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, NoHook: true})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 7)

	kinds := make([]setup.Kind, len(res.Artifacts))
	for i, a := range res.Artifacts {
		kinds[i] = a.Kind
	}
	assert.Equal(t, []setup.Kind{setup.KindConfig, setup.KindFeatureRoot, setup.KindPlugin, setup.KindPlugin, setup.KindPlugin, setup.KindSkill, setup.KindSnippet}, kinds)

	hooks := pluginFilePaths(wd).Hooks
	_, statErr := os.Stat(hooks)
	assert.True(t, os.IsNotExist(statErr))
	assert.NotContains(t, res.Created, hooks)
}

// Test_rerunning_init_for_claude_code_reports_every_plugin_file_unchanged
// pins convergence (R3) for the plugin, the skill and the CLAUDE.md block:
// a second, identical run reports every one of the eight artifacts
// ActionUnchanged and writes nothing further.
func Test_rerunning_init_for_claude_code_reports_every_plugin_file_unchanged(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)
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
	wd := t.TempDir()
	srv := newServer(t)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("feature-directory: specs\n"), 0o600))

	start := pluginFilePaths(wd).Start
	edited := []byte("---\nedited by hand\n---\n")
	require.NoError(t, os.WriteFile(start, edited, 0o600))

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, Force: true})

	require.NoError(t, err)

	require.Len(t, res.Artifacts, 8)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated, Detail: "rewritten from defaults"}, res.Artifacts[0])
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: start, Action: setup.ActionKept, Detail: "edited locally"}, res.Artifacts[3])

	body, readErr := os.ReadFile(start)
	require.NoError(t, readErr)
	assert.Equal(t, edited, body)
}

// Test_init_keeps_a_plugin_path_that_is_not_a_regular_file pins the Lstat
// guard, mirroring Uninstall's own: a directory at the manifest's own path
// and a symlink at the finish skill's own path are both kept, detail "not
// a regular file", neither followed nor written.
func Test_init_keeps_a_plugin_path_that_is_not_a_regular_file(t *testing.T) {
	wd := t.TempDir()
	paths := pluginFilePaths(wd)
	manifest, finish := paths.Manifest, paths.Finish
	require.NoError(t, os.MkdirAll(manifest, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(finish), 0o755))
	elsewhere := filepath.Join(wd, "elsewhere.md")
	require.NoError(t, os.WriteFile(elsewhere, []byte("elsewhere"), 0o600))
	require.NoError(t, os.Symlink(elsewhere, finish))
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 8)
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: manifest, Action: setup.ActionKept, Detail: "not a regular file"}, res.Artifacts[2])
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: finish, Action: setup.ActionKept, Detail: "not a regular file"}, res.Artifacts[4])

	info, statErr := os.Lstat(finish)
	require.NoError(t, statErr)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink)
}

// Test_init_dry_run_for_claude_code_writes_nothing pins R9 for the plugin:
// the same eight rows a real run would report, and neither ".claude/" nor
// "CLAUDE.md" present afterward.
func Test_init_dry_run_for_claude_code_writes_nothing(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, DryRun: true})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 8)
	assert.Empty(t, res.Created)

	_, statErr := os.Stat(filepath.Join(wd, ".claude"))
	assert.True(t, os.IsNotExist(statErr))

	_, statErr = os.Stat(filepath.Join(wd, "CLAUDE.md"))
	assert.True(t, os.IsNotExist(statErr))
}

// Test_an_unwritable_plugin_directory_refuses_before_the_feature_root_is_created
// pins R10's pre-write check: an unwritable ".claude/skills/brief"
// directory (already present as a directory, chmod 0o555) is caught before
// anything is written at all — ErrUnwritable, naming that directory as the
// blocking ancestor, not ErrPartialWrite — so neither the feature root nor
// ".brief.yaml" is ever created. Skipped under root, which ignores
// directory write permission.
func Test_an_unwritable_plugin_directory_refuses_before_the_feature_root_is_created(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	wd := t.TempDir()
	pluginDir := filepath.Join(wd, ".claude", "skills", "brief")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.Chmod(pluginDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(pluginDir, 0o755) })
	srv := newServer(t)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.ErrorIs(t, err, setup.ErrUnwritable)
	require.NotErrorIs(t, err, setup.ErrPartialWrite)

	refusal, ok := errors.AsType[*setup.RefusalError](err)
	require.True(t, ok)
	assert.Equal(t, pluginDir, refusal.Path)

	_, statErr := os.Stat(filepath.Join(wd, "docs", "specifications"))
	assert.True(t, os.IsNotExist(statErr))

	_, statErr = os.Stat(filepath.Join(wd, ".brief.yaml"))
	assert.True(t, os.IsNotExist(statErr))
}
