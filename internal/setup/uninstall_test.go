package setup_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_uninstall_removes_an_unedited_config_and_keeps_the_feature_root pins
// R6's ownership rule for the happy path: a config whose bytes still equal
// this binary's own render is removed, reported ActionRemoved with an empty
// Detail, and Result.Removed names its absolute path — while the feature
// root Init created, and a file placed under it, survive untouched.
func Test_uninstall_removes_an_unedited_config_and_keeps_the_feature_root(t *testing.T) {
	wd := t.TempDir()
	srv := setup.NewServer()

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	featureRoot := filepath.Join(wd, "docs", "specifications")
	marker := filepath.Join(featureRoot, "marker.txt")
	require.NoError(t, os.WriteFile(marker, []byte("keep me"), 0o600))

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	configPath := filepath.Join(wd, ".brief.yaml")

	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionRemoved}, res.Artifacts[0])
	assert.Equal(t, []string{configPath}, res.Removed)
	assert.Empty(t, res.Created)
	assert.Empty(t, res.Modified)

	_, statErr := os.Stat(configPath)
	assert.True(t, os.IsNotExist(statErr))

	info, statErr := os.Stat(featureRoot)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())

	body, readErr := os.ReadFile(marker)
	require.NoError(t, readErr)
	assert.Equal(t, "keep me", string(body))
}

// Test_uninstall_keeps_an_edited_config_and_reports_it pins R6's "edited
// locally" branch: a config whose bytes decode without violation but
// differ from artifact.ConfigFile() is left byte-identical (ActionKept)
// and Result.Removed stays empty.
func Test_uninstall_keeps_an_edited_config_and_reports_it(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	original := []byte("feature-directory: specs\n")
	require.NoError(t, os.WriteFile(configPath, original, 0o600))
	srv := setup.NewServer()

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionKept, Detail: "edited locally"}, res.Artifacts[0])
	assert.Empty(t, res.Removed)

	body, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, original, body)
}

// Test_uninstall_force_removes_an_edited_config pins --force's own
// override of the "edited locally" branch: the same fixture the test above
// keeps, --force instead removes, still reporting the "edited locally"
// detail.
func Test_uninstall_force_removes_an_edited_config(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("feature-directory: specs\n"), 0o600))
	srv := setup.NewServer()

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone, Force: true})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionRemoved, Detail: "edited locally"}, res.Artifacts[0])
	assert.Equal(t, []string{configPath}, res.Removed)

	_, statErr := os.Stat(configPath)
	assert.True(t, os.IsNotExist(statErr))
}

// Test_uninstall_treats_an_unparseable_or_invalid_config_as_edited pins the
// no-refusal-class divergence from Init: recognition is digest-only, so
// neither an unparseable file nor an R1-invalid value ever produces an
// error — both are simply "edited locally", kept without --force and
// removed with it.
func Test_uninstall_treats_an_unparseable_or_invalid_config_as_edited(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{name: "unparseable yaml", body: []byte("feature-directory: [unterminated\n")},
		{name: "R1-invalid value", body: []byte("handoff-cap-lines: 0\n")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			configPath := filepath.Join(wd, ".brief.yaml")
			require.NoError(t, os.WriteFile(configPath, tt.body, 0o600))
			srv := setup.NewServer()

			kept, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})
			require.NoError(t, err)
			require.Len(t, kept.Artifacts, 1)
			assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionKept, Detail: "edited locally"}, kept.Artifacts[0])
			assert.Empty(t, kept.Removed)

			body, readErr := os.ReadFile(configPath)
			require.NoError(t, readErr)
			assert.Equal(t, tt.body, body)

			removed, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone, Force: true})
			require.NoError(t, err)
			require.Len(t, removed.Artifacts, 1)
			assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionRemoved, Detail: "edited locally"}, removed.Artifacts[0])
			assert.Equal(t, []string{configPath}, removed.Removed)

			_, statErr := os.Stat(configPath)
			assert.True(t, os.IsNotExist(statErr))
		})
	}
}

// Test_uninstall_with_no_config_reports_nothing_installed pins the
// "nothing installed" equivalence: zero artifacts, every slice empty but
// non-nil, no error. The control arm is an existing feature root with no
// config nearby — still zero artifacts, and untouched.
func Test_uninstall_with_no_config_reports_nothing_installed(t *testing.T) {
	wd := t.TempDir()
	featureRoot := filepath.Join(wd, "docs", "specifications")
	require.NoError(t, os.MkdirAll(featureRoot, 0o755))
	srv := setup.NewServer()

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	assert.NotNil(t, res.Artifacts)
	assert.Empty(t, res.Artifacts)
	assert.NotNil(t, res.Removed)
	assert.Empty(t, res.Removed)
	assert.NotNil(t, res.Created)
	assert.Empty(t, res.Created)
	assert.NotNil(t, res.Modified)
	assert.Empty(t, res.Modified)

	info, statErr := os.Stat(featureRoot)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

// Test_uninstall_dry_run_plans_removal_and_removes_nothing pins R9's own
// DryRun promise for Uninstall: the row says "removed", Result.Removed
// stays empty, and the file on disk is untouched, byte-identical.
func Test_uninstall_dry_run_plans_removal_and_removes_nothing(t *testing.T) {
	wd := t.TempDir()
	srv := setup.NewServer()
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone, DryRun: true})

	require.NoError(t, err)
	assert.True(t, res.DryRun)
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.ActionRemoved, res.Artifacts[0].Action)
	assert.Empty(t, res.Removed)

	configPath := filepath.Join(wd, ".brief.yaml")
	body, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, artifact.ConfigFile(), body)
}

// Test_uninstall_keeps_a_config_path_that_is_not_a_regular_file pins the
// Lstat guard: ".brief.yaml" as an empty directory (os.Remove fails on a
// non-empty one regardless, so an empty one is the fixture that would
// actually catch a dropped guard) is kept, detail "not a regular file",
// with or without --force, and the directory survives.
func Test_uninstall_keeps_a_config_path_that_is_not_a_regular_file(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.Mkdir(configPath, 0o755))
	srv := setup.NewServer()

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})
	require.NoError(t, err)
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionKept, Detail: "not a regular file"}, res.Artifacts[0])

	res, err = srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone, Force: true})
	require.NoError(t, err)
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionKept, Detail: "not a regular file"}, res.Artifacts[0])

	info, statErr := os.Stat(configPath)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

// Test_uninstall_operates_on_a_config_found_in_an_ancestor pins the shared
// root rule: a config found walking up from wd (config.Locate) is the one
// Uninstall removes, not anything relative to wd itself.
func Test_uninstall_operates_on_a_config_found_in_an_ancestor(t *testing.T) {
	parent := t.TempDir()
	srv := setup.NewServer()
	_, err := srv.Init(t.Context(), parent, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	child := filepath.Join(parent, "child")
	require.NoError(t, os.Mkdir(child, 0o755))

	res, err := srv.Uninstall(t.Context(), child, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	configPath := filepath.Join(parent, ".brief.yaml")
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, configPath, res.Artifacts[0].Path)
	assert.Equal(t, setup.ActionRemoved, res.Artifacts[0].Action)

	_, statErr := os.Stat(configPath)
	assert.True(t, os.IsNotExist(statErr))
}

// Test_uninstall_never_removes_either_feature_root_after_force_init pins
// R6's last sentence: a non-default feature-directory, then "init --force"
// (which leaves both the old custom root and the new default one on disk),
// then "uninstall --force" — neither root is ever removed, including the
// default one, which is empty.
func Test_uninstall_never_removes_either_feature_root_after_force_init(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("feature-directory: specs\n"), 0o600))
	srv := setup.NewServer()

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)
	customRoot := filepath.Join(wd, "specs")
	info, statErr := os.Stat(customRoot)
	require.NoError(t, statErr)
	require.True(t, info.IsDir())

	_, err = srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, Force: true})
	require.NoError(t, err)
	defaultRoot := filepath.Join(wd, "docs", "specifications")
	info, statErr = os.Stat(defaultRoot)
	require.NoError(t, statErr)
	require.True(t, info.IsDir())

	_, err = srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone, Force: true})
	require.NoError(t, err)

	info, statErr = os.Stat(customRoot)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())

	info, statErr = os.Stat(defaultRoot)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

// Test_uninstall_reports_a_remove_failure_without_partial_write pins the
// single-artifact failure path: an unwritable parent directory makes
// os.Remove fail, and because the config is the only artifact this release
// plans, nothing was ever removed before that failure — so the returned
// error does not wrap ErrPartialWrite, and the file survives. Skipped under
// root, which ignores directory write permission.
func Test_uninstall_reports_a_remove_failure_without_partial_write(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	wd := t.TempDir()
	srv := setup.NewServer()
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	require.NoError(t, os.Chmod(wd, 0o555))
	t.Cleanup(func() { _ = os.Chmod(wd, 0o755) })

	_, err = srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})

	require.Error(t, err)
	require.NotErrorIs(t, err, setup.ErrPartialWrite)

	_, statErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
	assert.NoError(t, statErr)
}

// Test_uninstall_rejects_an_unknown_host pins the same usage-error branch
// Init reports: a host outside Hosts() never reaches config.Locate at all.
func Test_uninstall_rejects_an_unknown_host(t *testing.T) {
	wd := t.TempDir()
	srv := setup.NewServer()

	_, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: "bogus"})

	assert.ErrorIs(t, err, setup.ErrUnknownHost)
}

// Test_uninstall_for_claude_code_removes_the_unedited_plugin_and_its_empty_directories
// pins R6's own removal order and directory pruning: the CLAUDE.md block
// first, then the plugin's four rows report hooks.json, finish skill,
// start skill, manifest — the reverse of Init's own write order — then the
// config last, every row ActionRemoved, and afterward ".claude/skills/brief/"
// is gone while ".claude/skills/" and ".claude/" (the host's own
// directories, never brief's to remove) still stand.
func Test_uninstall_for_claude_code_removes_the_unedited_plugin_and_its_empty_directories(t *testing.T) {
	wd := t.TempDir()
	srv := setup.NewServer()
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	paths := pluginFilePaths(wd)
	configPath := filepath.Join(wd, ".brief.yaml")

	require.Len(t, res.Artifacts, 6)
	assert.Equal(t, setup.Artifact{Kind: setup.KindSnippet, Path: paths.ClaudeMD, Action: setup.ActionRemoved}, res.Artifacts[0])
	assert.Equal(t, setup.Artifact{Kind: setup.KindHook, Path: paths.Hooks, Action: setup.ActionRemoved}, res.Artifacts[1])
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: paths.Finish, Action: setup.ActionRemoved}, res.Artifacts[2])
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: paths.Start, Action: setup.ActionRemoved}, res.Artifacts[3])
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: paths.Manifest, Action: setup.ActionRemoved}, res.Artifacts[4])
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionRemoved}, res.Artifacts[5])
	assert.Equal(t, []string{paths.ClaudeMD, paths.Hooks, paths.Finish, paths.Start, paths.Manifest, configPath}, res.Removed)

	_, statErr := os.Stat(filepath.Join(wd, ".claude", "skills", "brief"))
	assert.True(t, os.IsNotExist(statErr))

	info, statErr := os.Stat(filepath.Join(wd, ".claude", "skills"))
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())

	info, statErr = os.Stat(filepath.Join(wd, ".claude"))
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

// Test_uninstall_keeps_an_edited_plugin_file_and_the_directories_holding_it_unless_forced
// pins R6's "edited locally" branch for a plugin file: without --force the
// edited start skill (and the directories holding it) survive; with
// --force it is removed, detail "edited locally", and the plugin's own
// directory tree is pruned same as the happy path.
func Test_uninstall_keeps_an_edited_plugin_file_and_the_directories_holding_it_unless_forced(t *testing.T) {
	tests := []struct {
		name       string
		force      bool
		wantAction setup.Action
	}{
		{name: "no force: kept", force: false, wantAction: setup.ActionKept},
		{name: "force: removed", force: true, wantAction: setup.ActionRemoved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			srv := setup.NewServer()
			_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
			require.NoError(t, err)

			start := pluginFilePaths(wd).Start
			edited := []byte("---\nedited by hand\n---\n")
			require.NoError(t, os.WriteFile(start, edited, 0o600))

			res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode, Force: tt.force})

			require.NoError(t, err)

			var startArt setup.Artifact
			for _, a := range res.Artifacts {
				if a.Path == start {
					startArt = a
				}
			}
			assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: start, Action: tt.wantAction, Detail: "edited locally"}, startArt)

			_, statErr := os.Stat(start)
			if tt.force {
				assert.True(t, os.IsNotExist(statErr))

				_, statErr = os.Stat(filepath.Join(wd, ".claude", "skills", "brief"))
				assert.True(t, os.IsNotExist(statErr))

				return
			}

			require.NoError(t, statErr)
			body, readErr := os.ReadFile(start)
			require.NoError(t, readErr)
			assert.Equal(t, edited, body)

			info, statErr := os.Stat(filepath.Dir(start))
			require.NoError(t, statErr, "the directory holding the kept file must survive")
			assert.True(t, info.IsDir())
		})
	}
}

// Test_uninstall_after_a_no_hook_init_removes_the_three_files_and_the_directory
// pins the missing-file branch: hooks.json never existed (a --no-hook
// init), so it plans no row and no error, and the remaining three plugin
// files plus the CLAUDE.md block still remove cleanly with the plugin
// directory pruned.
func Test_uninstall_after_a_no_hook_init_removes_the_three_files_and_the_directory(t *testing.T) {
	wd := t.TempDir()
	srv := setup.NewServer()
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, NoHook: true})
	require.NoError(t, err)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 5)
	for _, a := range res.Artifacts {
		assert.NotEqual(t, setup.KindHook, a.Kind)
		assert.Equal(t, setup.ActionRemoved, a.Action)
	}

	_, statErr := os.Stat(filepath.Join(wd, ".claude", "skills", "brief"))
	assert.True(t, os.IsNotExist(statErr))
}

// Test_uninstall_keeps_a_plugin_directory_holding_a_file_brief_did_not_write
// pins the pruning boundary: an adopter's own file under
// "skills/extra/notes.md" keeps "skills/" and "brief/" standing even
// though every one of brief's own files in this same run is removed — the
// control proving pruning runs at all.
func Test_uninstall_keeps_a_plugin_directory_holding_a_file_brief_did_not_write(t *testing.T) {
	wd := t.TempDir()
	srv := setup.NewServer()
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	extraDir := filepath.Join(wd, ".claude", "skills", "brief", "skills", "extra")
	require.NoError(t, os.MkdirAll(extraDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(extraDir, "notes.md"), []byte("mine"), 0o600))

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	for _, a := range res.Artifacts {
		assert.Equal(t, setup.ActionRemoved, a.Action, "brief's own file %s must still be removed", a.Path)
	}

	_, statErr := os.Stat(filepath.Join(extraDir, "notes.md"))
	require.NoError(t, statErr, "the adopter's own file must survive")

	info, statErr := os.Stat(filepath.Join(wd, ".claude", "skills", "brief", "skills"))
	require.NoError(t, statErr, "skills/ must survive: it still holds extra/")
	assert.True(t, info.IsDir())

	info, statErr = os.Stat(filepath.Join(wd, ".claude", "skills", "brief"))
	require.NoError(t, statErr, "brief/ must survive: it still holds skills/")
	assert.True(t, info.IsDir())
}

// Test_uninstall_removes_agents_and_prunes_the_agents_directory pins the
// "always plans agent removal" rule (R7/R4): unlike Init, Uninstall has no
// --with-agents flag of its own — an init that installed the three agent
// files has them removed here regardless, in the reverse of Init's own
// order (reviewer, implementer, planner), ahead of the plugin's own files,
// and "agents/" is pruned alongside the plugin's other now-empty
// directories.
func Test_uninstall_removes_agents_and_prunes_the_agents_directory(t *testing.T) {
	wd := t.TempDir()
	srv := setup.NewServer()
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	paths := agentFilePaths(wd)

	var agentActions []setup.Action

	var sawReviewerBeforePlugin bool

	pluginSeen := false

	for _, a := range res.Artifacts {
		if a.Kind == setup.KindAgent {
			agentActions = append(agentActions, a.Action)
		}

		if a.Kind == setup.KindPlugin || a.Kind == setup.KindHook {
			pluginSeen = true
		}

		if a.Path == paths.Reviewer && !pluginSeen {
			sawReviewerBeforePlugin = true
		}
	}

	assert.Equal(t, []setup.Action{setup.ActionRemoved, setup.ActionRemoved, setup.ActionRemoved}, agentActions)
	assert.True(t, sawReviewerBeforePlugin, "the reviewer agent must be planned before any plugin file")

	agentPathList := []string{paths.Reviewer, paths.Implementer, paths.Planner}
	var order []string

	for _, a := range res.Artifacts {
		if a.Kind == setup.KindAgent {
			order = append(order, a.Path)
		}
	}
	assert.Equal(t, agentPathList, order)

	for _, p := range agentPathList {
		_, statErr := os.Stat(p)
		assert.True(t, os.IsNotExist(statErr), "%s must be removed", p)
	}

	_, statErr := os.Stat(filepath.Join(wd, ".claude", "skills", "brief", "agents"))
	assert.True(t, os.IsNotExist(statErr), "the now-empty agents/ directory must be pruned")
}

// Test_uninstall_keeps_an_edited_agent_unless_forced pins the same "edited
// locally" branch a plugin file gets, for an agent file: kept without
// --force, removed (still detail "edited locally") with it, mirroring
// Test_uninstall_keeps_an_edited_plugin_file_and_the_directories_holding_it_unless_forced.
func Test_uninstall_keeps_an_edited_agent_unless_forced(t *testing.T) {
	tests := []struct {
		name       string
		force      bool
		wantAction setup.Action
	}{
		{name: "no force: kept", force: false, wantAction: setup.ActionKept},
		{name: "force: removed", force: true, wantAction: setup.ActionRemoved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			srv := setup.NewServer()
			_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
			require.NoError(t, err)

			planner := agentFilePaths(wd).Planner
			edited := []byte("---\nname: planner\nedited: true\n---\n")
			require.NoError(t, os.WriteFile(planner, edited, 0o600))

			res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode, Force: tt.force})

			require.NoError(t, err)

			var plannerArt setup.Artifact
			for _, a := range res.Artifacts {
				if a.Path == planner {
					plannerArt = a
				}
			}
			assert.Equal(t, setup.Artifact{Kind: setup.KindAgent, Path: planner, Action: tt.wantAction, Detail: "edited locally"}, plannerArt)

			_, statErr := os.Stat(planner)
			if tt.force {
				assert.True(t, os.IsNotExist(statErr))

				return
			}

			require.NoError(t, statErr)
			body, readErr := os.ReadFile(planner)
			require.NoError(t, readErr)
			assert.Equal(t, edited, body)
		})
	}
}

// Test_uninstall_removes_a_bound_config pins the second KindConfig
// digest's own removal path: a config holding artifact.ConfigFileWithRoles
// — the bound variant "init --with-agents" wrote — is recognized and
// removed exactly like the plain render, ActionRemoved with no detail.
func Test_uninstall_removes_a_bound_config(t *testing.T) {
	wd := t.TempDir()
	srv := setup.NewServer()
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	configPath := filepath.Join(wd, ".brief.yaml")

	var configArt setup.Artifact
	for _, a := range res.Artifacts {
		if a.Path == configPath {
			configArt = a
		}
	}
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionRemoved}, configArt)

	_, statErr := os.Stat(configPath)
	assert.True(t, os.IsNotExist(statErr))
}

// Test_uninstall_for_host_none_leaves_the_plugin_in_place pins the
// host-gated planning: with --host none, uninstall never plans a single
// plugin file, so the tree it installed under claude-code survives
// completely — only the control arm, uninstalling the same tree under
// claude-code, actually removes it.
func Test_uninstall_for_host_none_leaves_the_plugin_in_place(t *testing.T) {
	wd := t.TempDir()
	srv := setup.NewServer()
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.KindConfig, res.Artifacts[0].Kind)

	_, statErr := os.Stat(pluginFilePaths(wd).Manifest)
	require.NoError(t, statErr, "the plugin must survive an uninstall scoped to --host none")
}
