package setup_test

import (
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_uninstall_removes_an_unedited_config_and_keeps_the_feature_root(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	featureRoot := filepath.Join(wd, "docs", "specifications")
	marker := filepath.Join(featureRoot, "marker.txt")
	require.NoError(t, mem.WriteFile(memKey(marker), []byte("keep me"), 0o600))

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	configPath := filepath.Join(wd, ".brief.yaml")

	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionRemoved}, res.Artifacts[0])
	assert.Equal(t, []string{configPath}, res.Removed)
	assert.Empty(t, res.Created)
	assert.Empty(t, res.Modified)

	snap := mem.Snapshot()
	assert.NotContains(t, snap, memKey(configPath))

	info, ok := snap[memKey(featureRoot)]
	require.True(t, ok)
	assert.True(t, info.Mode.IsDir())

	assert.Equal(t, "keep me", string(snap[memKey(marker)].Data))
}

func Test_uninstall_keeps_an_edited_config_and_reports_it(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	original := []byte("feature-directory: specs\n")
	require.NoError(t, mem.WriteFile(memKey(configPath), original, 0o600))
	srv := newMemServer(mem)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionKept, Detail: "edited locally", ForceRemovable: true}, res.Artifacts[0])
	assert.Empty(t, res.Removed)

	assert.Equal(t, original, mem.Snapshot()[memKey(configPath)].Data)
}

func Test_uninstall_force_removes_an_edited_config(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, mem.WriteFile(memKey(configPath), []byte("feature-directory: specs\n"), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone, Force: true})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionRemoved, Detail: "edited locally"}, res.Artifacts[0])
	assert.Equal(t, []string{configPath}, res.Removed)

	assert.NotContains(t, mem.Snapshot(), memKey(configPath))
}

func Test_uninstall_treats_an_unparseable_or_invalid_config_as_edited(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{name: "unparseable yaml", body: []byte("feature-directory: [unterminated\n")},
		{name: "out-of-range value", body: []byte("handoff-cap-lines: 0\n")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := fsAbs("repo")
			mem := newVirtualMem(wd)
			configPath := filepath.Join(wd, ".brief.yaml")
			require.NoError(t, mem.WriteFile(memKey(configPath), tt.body, 0o600))
			srv := newMemServer(mem)

			kept, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})
			require.NoError(t, err)
			require.Len(t, kept.Artifacts, 1)
			assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionKept, Detail: "edited locally", ForceRemovable: true}, kept.Artifacts[0])
			assert.Empty(t, kept.Removed)

			assert.Equal(t, tt.body, mem.Snapshot()[memKey(configPath)].Data)

			removed, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone, Force: true})
			require.NoError(t, err)
			require.Len(t, removed.Artifacts, 1)
			assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionRemoved, Detail: "edited locally"}, removed.Artifacts[0])
			assert.Equal(t, []string{configPath}, removed.Removed)

			assert.NotContains(t, mem.Snapshot(), memKey(configPath))
		})
	}
}

func Test_uninstall_with_no_config_reports_nothing_installed(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	featureRoot := filepath.Join(wd, "docs", "specifications")
	require.NoError(t, mem.MkdirAll(memKey(featureRoot), 0o755))
	srv := newMemServer(mem)

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

	info, ok := mem.Snapshot()[memKey(featureRoot)]
	require.True(t, ok)
	assert.True(t, info.Mode.IsDir())
}

func Test_uninstall_dry_run_plans_removal_and_removes_nothing(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	before := mem.Snapshot()

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone, DryRun: true})

	require.NoError(t, err)
	assert.True(t, res.DryRun)
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.ActionRemoved, res.Artifacts[0].Action)
	assert.Empty(t, res.Removed)

	assert.Equal(t, before, mem.Snapshot())
}

func Test_uninstall_keeps_a_config_path_that_is_not_a_regular_file(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, mem.Mkdir(memKey(configPath), 0o755))
	srv := newMemServer(mem)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})
	require.NoError(t, err)
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionKept, Detail: "not a regular file"}, res.Artifacts[0])

	res, err = srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone, Force: true})
	require.NoError(t, err)
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionKept, Detail: "not a regular file"}, res.Artifacts[0])

	info, ok := mem.Snapshot()[memKey(configPath)]
	require.True(t, ok)
	assert.True(t, info.Mode.IsDir())
}

func Test_uninstall_operates_on_a_config_found_in_an_ancestor(t *testing.T) {
	parent := fsAbs("repo")
	mem := newVirtualMem(parent)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), parent, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	require.NoError(t, mem.Mkdir(memKey(parent)+"/child", 0o755))
	child := filepath.Join(parent, "child")

	res, err := srv.Uninstall(t.Context(), child, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	configPath := filepath.Join(parent, ".brief.yaml")
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, configPath, res.Artifacts[0].Path)
	assert.Equal(t, setup.ActionRemoved, res.Artifacts[0].Action)

	assert.NotContains(t, mem.Snapshot(), memKey(configPath))
}

func Test_uninstall_never_removes_either_feature_root_after_force_init(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, mem.WriteFile(memKey(configPath), []byte("feature-directory: specs\n"), 0o600))
	srv := newMemServer(mem)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)
	customRoot := filepath.Join(wd, "specs")
	info, ok := mem.Snapshot()[memKey(customRoot)]
	require.True(t, ok)
	require.True(t, info.Mode.IsDir())

	_, err = srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, Force: true})
	require.NoError(t, err)
	defaultRoot := filepath.Join(wd, "docs", "specifications")
	info, ok = mem.Snapshot()[memKey(defaultRoot)]
	require.True(t, ok)
	require.True(t, info.Mode.IsDir())

	_, err = srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone, Force: true})
	require.NoError(t, err)

	snap := mem.Snapshot()
	info, ok = snap[memKey(customRoot)]
	require.True(t, ok)
	assert.True(t, info.Mode.IsDir())

	info, ok = snap[memKey(defaultRoot)]
	require.True(t, ok)
	assert.True(t, info.Mode.IsDir())
}

func Test_uninstall_rejects_an_unknown_host(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)

	_, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: "bogus"})

	assert.ErrorIs(t, err, setup.ErrUnknownHost)
}

func Test_uninstall_for_claude_code_removes_the_unedited_plugin_and_its_empty_directories(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	paths := pluginFilePaths(wd)
	configPath := filepath.Join(wd, ".brief.yaml")

	require.Len(t, res.Artifacts, 7)
	assert.Equal(t, setup.Artifact{Kind: setup.KindSnippet, Path: paths.ClaudeMD, Action: setup.ActionRemoved}, res.Artifacts[0])
	assert.Equal(t, setup.Artifact{Kind: setup.KindSkill, Path: paths.Skill, Action: setup.ActionRemoved}, res.Artifacts[1])
	assert.Equal(t, setup.Artifact{Kind: setup.KindHook, Path: paths.Hooks, Action: setup.ActionRemoved}, res.Artifacts[2])
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: paths.Finish, Action: setup.ActionRemoved}, res.Artifacts[3])
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: paths.Start, Action: setup.ActionRemoved}, res.Artifacts[4])
	assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: paths.Manifest, Action: setup.ActionRemoved}, res.Artifacts[5])
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionRemoved}, res.Artifacts[6])
	assert.Equal(t, []string{paths.ClaudeMD, paths.Skill, paths.Hooks, paths.Finish, paths.Start, paths.Manifest, configPath}, res.Removed)

	snap := mem.Snapshot()
	assert.NotContains(t, snap, memKey(filepath.Join(wd, ".claude", "skills", "brief")))

	info, ok := snap[memKey(filepath.Join(wd, ".claude", "skills"))]
	require.True(t, ok)
	assert.True(t, info.Mode.IsDir())

	info, ok = snap[memKey(filepath.Join(wd, ".claude"))]
	require.True(t, ok)
	assert.True(t, info.Mode.IsDir())
}

func Test_uninstall_keeps_an_edited_plugin_file_and_the_directories_holding_it_unless_forced(t *testing.T) {
	edited := []byte("---\nedited by hand\n---\n")

	tests := []struct {
		name                string
		force               bool
		wantAction          setup.Action
		wantForceRemovable  bool
		wantExists          bool
		wantBytes           []byte
		wantPluginDirExists bool
	}{
		{name: "no force: kept", force: false, wantAction: setup.ActionKept, wantForceRemovable: true, wantExists: true, wantBytes: edited, wantPluginDirExists: true},
		{name: "force: removed", force: true, wantAction: setup.ActionRemoved, wantForceRemovable: false, wantExists: false, wantBytes: nil, wantPluginDirExists: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := fsAbs("repo")
			mem := newVirtualMem(wd)
			srv := newMemServer(mem)
			_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
			require.NoError(t, err)

			start := pluginFilePaths(wd).Start
			require.NoError(t, mem.WriteFile(memKey(start), edited, 0o600))

			res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode, Force: tt.force})

			require.NoError(t, err)

			startArt := findArtifactByPath(t, res, start)
			assert.Equal(t, setup.Artifact{Kind: setup.KindPlugin, Path: start, Action: tt.wantAction, Detail: "edited locally", ForceRemovable: tt.wantForceRemovable}, startArt)

			data, exists := memData(mem.Snapshot(), memKey(start))
			assert.Equal(t, tt.wantExists, exists)
			assert.Equal(t, tt.wantBytes, data)

			_, pluginDirExists := mem.Snapshot()[memKey(filepath.Join(wd, ".claude", "skills", "brief"))]
			assert.Equal(t, tt.wantPluginDirExists, pluginDirExists)
		})
	}
}

func Test_uninstall_after_a_no_hook_init_removes_the_three_files_and_the_directory(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, NoHook: true})
	require.NoError(t, err)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	configPath := filepath.Join(wd, ".brief.yaml")
	paths := pluginFilePaths(wd)
	assert.ElementsMatch(t, []setup.Artifact{
		{Kind: setup.KindSnippet, Path: paths.ClaudeMD, Action: setup.ActionRemoved},
		{Kind: setup.KindSkill, Path: paths.Skill, Action: setup.ActionRemoved},
		{Kind: setup.KindPlugin, Path: paths.Finish, Action: setup.ActionRemoved},
		{Kind: setup.KindPlugin, Path: paths.Start, Action: setup.ActionRemoved},
		{Kind: setup.KindPlugin, Path: paths.Manifest, Action: setup.ActionRemoved},
		{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionRemoved},
	}, res.Artifacts)

	assert.NotContains(t, mem.Snapshot(), memKey(filepath.Join(wd, ".claude", "skills", "brief")))
}

func Test_uninstall_keeps_a_plugin_directory_holding_a_file_brief_did_not_write(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	extraDir := filepath.Join(wd, ".claude", "skills", "brief", "skills", "extra")
	require.NoError(t, mem.MkdirAll(memKey(extraDir), 0o755))
	require.NoError(t, mem.WriteFile(memKey(extraDir)+"/notes.md", []byte("mine"), 0o600))

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	for _, a := range res.Artifacts {
		assert.Equal(t, setup.ActionRemoved, a.Action, "brief's own file %s must still be removed", a.Path)
	}

	snap := mem.Snapshot()
	require.Contains(t, snap, memKey(extraDir)+"/notes.md", "the adopter's own file must survive")

	info, ok := snap[memKey(filepath.Join(wd, ".claude", "skills", "brief", "skills"))]
	require.True(t, ok, "skills/ must survive: it still holds extra/")
	assert.True(t, info.Mode.IsDir())

	info, ok = snap[memKey(filepath.Join(wd, ".claude", "skills", "brief"))]
	require.True(t, ok, "brief/ must survive: it still holds skills/")
	assert.True(t, info.Mode.IsDir())
}

func Test_uninstall_removes_agents_and_prunes_the_agents_directory(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
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

	snap := mem.Snapshot()
	for _, p := range agentPathList {
		assert.NotContains(t, snap, memKey(p), "%s must be removed", p)
	}

	assert.NotContains(t, snap, memKey(filepath.Join(wd, ".claude", "skills", "brief", "agents")), "the now-empty agents/ directory must be pruned")
}

func Test_uninstall_keeps_an_edited_agent_unless_forced(t *testing.T) {
	edited := []byte("---\nname: planner\nedited: true\n---\n")

	tests := []struct {
		name               string
		force              bool
		wantAction         setup.Action
		wantForceRemovable bool
		wantExists         bool
		wantBytes          []byte
	}{
		{name: "no force: kept", force: false, wantAction: setup.ActionKept, wantForceRemovable: true, wantExists: true, wantBytes: edited},
		{name: "force: removed", force: true, wantAction: setup.ActionRemoved, wantForceRemovable: false, wantExists: false, wantBytes: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := fsAbs("repo")
			mem := newVirtualMem(wd)
			srv := newMemServer(mem)
			_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
			require.NoError(t, err)

			planner := agentFilePaths(wd).Planner
			require.NoError(t, mem.WriteFile(memKey(planner), edited, 0o600))

			res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode, Force: tt.force})

			require.NoError(t, err)

			plannerArt := findArtifactByPath(t, res, planner)
			assert.Equal(t, setup.Artifact{Kind: setup.KindAgent, Path: planner, Action: tt.wantAction, Detail: "edited locally", ForceRemovable: tt.wantForceRemovable}, plannerArt)

			data, exists := memData(mem.Snapshot(), memKey(planner))
			assert.Equal(t, tt.wantExists, exists)
			assert.Equal(t, tt.wantBytes, data)
		})
	}
}

// olderPlannerBytesForUninstall and olderImplementerBytesForUninstall are
// agent renders that predate the current format, used to exercise removal.
const (
	olderPlannerBytesForUninstall = "---\nname: planner\ndescription: Turn a feature's specification into ordered scenario " +
		"plans.\ntools: Read, Grep, Glob, Bash, Edit, Write\n---\n\nTurn the feature's " +
		"specification into ordered scenario plans: run `brief new step <feature>` for the next " +
		"scenario, then fill its plan file. Never write production or test code.\n"
	olderImplementerBytesForUninstall = "---\nname: implementer\ndescription: Implement a feature's next open step, from brief " +
		"start through brief finish.\n---\n\nRun `brief start <feature>` and implement its next " +
		"open step, working from its output rather than reading the specification or earlier steps " +
		"whole. Close the step with `brief finish <feature> <step> --handoff <path> --state " +
		"<path>`.\n"
)

func Test_uninstall_removes_an_older_agent_file_without_force(t *testing.T) {
	cases := []struct {
		name               string
		role               string
		seedBytes          []byte
		wantAction         setup.Action
		wantDetail         string
		wantForceRemovable bool
		wantExists         bool
		wantBytes          []byte
	}{
		{
			name: "older planner is removed without force", role: "planner",
			seedBytes:  []byte(olderPlannerBytesForUninstall),
			wantAction: setup.ActionRemoved, wantDetail: "", wantForceRemovable: false, wantExists: false, wantBytes: nil,
		},
		{
			name: "older implementer is removed without force", role: "implementer",
			seedBytes:  []byte(olderImplementerBytesForUninstall),
			wantAction: setup.ActionRemoved, wantDetail: "", wantForceRemovable: false, wantExists: false, wantBytes: nil,
		},
		{
			name: "edited planner is kept without force", role: "planner",
			seedBytes:  []byte("---\nname: planner\nedited: true\n---\n"),
			wantAction: setup.ActionKept, wantDetail: "edited locally", wantForceRemovable: true,
			wantExists: true, wantBytes: []byte("---\nname: planner\nedited: true\n---\n"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := fsAbs("repo")
			mem := newVirtualMem(wd)
			srv := newMemServer(mem)
			_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
			require.NoError(t, err)

			paths := agentFilePaths(wd)
			path := paths.Planner
			if c.role == "implementer" {
				path = paths.Implementer
			}
			require.NoError(t, mem.WriteFile(memKey(path), c.seedBytes, 0o600))

			res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
			require.NoError(t, err)

			art := findArtifactByPath(t, res, path)
			assert.Equal(t, c.wantAction, art.Action)
			assert.Equal(t, c.wantDetail, art.Detail)
			assert.Equal(t, c.wantForceRemovable, art.ForceRemovable)

			data, exists := memData(mem.Snapshot(), memKey(path))
			assert.Equal(t, c.wantExists, exists)
			assert.Equal(t, c.wantBytes, data)
		})
	}
}

func Test_uninstall_removes_a_bound_config(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	configPath := filepath.Join(wd, ".brief.yaml")

	configArt := findArtifactByPath(t, res, configPath)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionRemoved}, configArt)

	assert.NotContains(t, mem.Snapshot(), memKey(configPath))
}

func Test_uninstall_for_host_none_leaves_the_plugin_in_place(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.KindConfig, res.Artifacts[0].Kind)

	require.Contains(t, mem.Snapshot(), memKey(pluginFilePaths(wd).Manifest), "the plugin must survive an uninstall scoped to --host none")
}
