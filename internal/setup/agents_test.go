package setup_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// agentPaths names the three role-agent files' absolute paths under one
// root, in install order: planner, implementer, reviewer.
type agentPaths struct {
	Planner, Implementer, Reviewer string
}

// agentFilePaths returns root's own agentPaths.
func agentFilePaths(root string) agentPaths {
	base := filepath.Join(root, ".claude", "skills", "brief", "agents")

	return agentPaths{
		Planner:     filepath.Join(base, "planner.md"),
		Implementer: filepath.Join(base, "implementer.md"),
		Reviewer:    filepath.Join(base, "reviewer.md"),
	}
}

func Test_init_with_agents_writes_three_agents_and_a_config_binding_them(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 11)

	kinds := make([]setup.Kind, len(res.Artifacts))
	for i, a := range res.Artifacts {
		kinds[i] = a.Kind
	}
	assert.Equal(t, []setup.Kind{
		setup.KindConfig, setup.KindFeatureRoot,
		setup.KindPlugin, setup.KindPlugin, setup.KindPlugin, setup.KindHook,
		setup.KindSkill,
		setup.KindAgent, setup.KindAgent, setup.KindAgent,
		setup.KindSnippet,
	}, kinds)

	paths := agentFilePaths(wd)
	assert.Equal(t, paths.Planner, res.Artifacts[7].Path)
	assert.Equal(t, paths.Implementer, res.Artifacts[8].Path)
	assert.Equal(t, paths.Reviewer, res.Artifacts[9].Path)

	for _, i := range []int{7, 8, 9} {
		assert.Equalf(t, setup.ActionCreated, res.Artifacts[i].Action, "artifact %d must be created", i)
	}

	assert.Equal(t, []string{}, res.RolesToAdd)

	snap := mem.Snapshot()
	assert.Equal(t, artifact.ConfigFileWithRoles(), snap[memKey(filepath.Join(wd, ".brief.yaml"))].Data)
	assert.Equal(t, artifact.AgentPlanner(), snap[memKey(paths.Planner)].Data)
	assert.Equal(t, artifact.AgentImplementer(), snap[memKey(paths.Implementer)].Data)
	assert.Equal(t, artifact.AgentReviewer(), snap[memKey(paths.Reviewer)].Data)
}

func Test_rerunning_init_with_agents_reports_every_agent_unchanged(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})

	require.NoError(t, err)
	assert.Empty(t, res.Created)

	configPath := filepath.Join(wd, ".brief.yaml")
	featureRoot := filepath.Join(wd, "docs", "specifications")
	pluginPaths := pluginFilePaths(wd)
	agentPaths := agentFilePaths(wd)
	wantActions := map[string]setup.Action{
		configPath:             setup.ActionUnchanged,
		featureRoot:            setup.ActionUnchanged,
		pluginPaths.Manifest:   setup.ActionUnchanged,
		pluginPaths.Start:      setup.ActionUnchanged,
		pluginPaths.Finish:     setup.ActionUnchanged,
		pluginPaths.Hooks:      setup.ActionUnchanged,
		pluginPaths.Skill:      setup.ActionUnchanged,
		pluginPaths.ClaudeMD:   setup.ActionUnchanged,
		agentPaths.Planner:     setup.ActionUnchanged,
		agentPaths.Implementer: setup.ActionUnchanged,
		agentPaths.Reviewer:    setup.ActionUnchanged,
	}

	gotActions := make(map[string]setup.Action, len(res.Artifacts))
	for _, a := range res.Artifacts {
		gotActions[a.Path] = a.Action
	}
	assert.Equal(t, wantActions, gotActions)
	assert.Equal(t, []string{}, res.RolesToAdd)
}

func Test_init_keeps_an_unrecognized_agent_file_even_under_force(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	planner := agentFilePaths(wd).Planner
	edited := []byte("---\nname: planner\nedited: true\n---\n")
	require.NoError(t, mem.WriteFile(memKey(planner), edited, 0o600))

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true, Force: true})

	require.NoError(t, err)

	plannerArt := findArtifactByPath(t, res, planner)
	assert.Equal(t, setup.Artifact{Kind: setup.KindAgent, Path: planner, Action: setup.ActionKept, Detail: "edited locally"}, plannerArt)

	assert.Equal(t, edited, mem.Snapshot()[memKey(planner)].Data)
}

func Test_init_with_agents_never_edits_an_existing_config(t *testing.T) {
	cases := []struct {
		name       string
		body       []byte
		wantAction setup.Action
	}{
		{name: "plain render", body: artifact.ConfigFile(), wantAction: setup.ActionUnchanged},
		{name: "edited but valid", body: []byte("feature-directory: specs\n"), wantAction: setup.ActionKept},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := fsAbs("repo")
			mem := newVirtualMem(wd)
			configPath := filepath.Join(wd, ".brief.yaml")
			require.NoError(t, mem.WriteFile(memKey(configPath), c.body, 0o600))
			srv := newMemServer(mem)

			res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})

			require.NoError(t, err)
			assert.Equal(t, c.wantAction, findArtifact(t, res, setup.KindConfig).Action)
			assert.NotContains(t, res.Created, configPath)
			assert.NotContains(t, res.Modified, configPath)

			assert.Equal(t, c.body, mem.Snapshot()[memKey(configPath)].Data)

			assert.Equal(t, []string{
				"roles:",
				"  planner: brief:planner",
				"  implementer: brief:implementer",
				"  reviewer: brief:reviewer",
			}, res.RolesToAdd)
		})
	}
}

func Test_roles_to_add_lists_only_unbound_roles(t *testing.T) {
	cases := []struct {
		name string
		body []byte
		want []string
	}{
		{
			name: "one bound to the adopter's own agent, one to brief's",
			body: []byte("roles:\n  planner: mine:planner\n  implementer: brief:implementer\n"),
			want: []string{"roles:", "  reviewer: brief:reviewer"},
		},
		{
			name: "every role already bound",
			body: []byte("roles:\n  planner: brief:planner\n  implementer: brief:implementer\n  reviewer: brief:reviewer\n"),
			want: []string{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := fsAbs("repo")
			mem := newVirtualMem(wd)
			require.NoError(t, mem.WriteFile(memKey(wd)+"/.brief.yaml", c.body, 0o600))
			srv := newMemServer(mem)

			res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})

			require.NoError(t, err)
			assert.Equal(t, c.want, res.RolesToAdd)
		})
	}
}

// Init runs against the in-memory store; the final config.Inspect check
// uses a real file instead, since config.Inspect always reads disk.
func Test_roles_to_add_lines_parse_into_brief_bindings(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	base := "feature-directory: docs/specifications\n"
	require.NoError(t, mem.WriteFile(memKey(wd)+"/.brief.yaml", []byte(base), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)
	require.NotEmpty(t, res.RolesToAdd)

	appended := base
	for _, line := range res.RolesToAdd {
		appended += line + "\n"
	}

	realDir := t.TempDir()
	configPath := filepath.Join(realDir, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(appended), 0o600))

	cfg, violations, inspectErr := config.Inspect(configPath)

	require.NoError(t, inspectErr)
	assert.Empty(t, violations)
	assert.Equal(t, artifact.AgentBindings(), cfg.Roles)
}

func Test_init_with_agents_on_a_bound_config_reports_it_unchanged_with_nothing_to_add(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, mem.WriteFile(memKey(configPath), artifact.ConfigFileWithRoles(), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})

	require.NoError(t, err)
	assert.Equal(t, setup.ActionUnchanged, findArtifact(t, res, setup.KindConfig).Action)
	assert.Equal(t, []string{}, res.RolesToAdd)

	assert.Equal(t, artifact.ConfigFileWithRoles(), mem.Snapshot()[memKey(configPath)].Data)
}

func Test_force_init_with_agents_rewrites_a_plain_config_to_the_bound_variant(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, mem.WriteFile(memKey(configPath), artifact.ConfigFile(), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true, Force: true})

	require.NoError(t, err)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated, Detail: "rewritten from defaults"}, findArtifact(t, res, setup.KindConfig))

	assert.Equal(t, artifact.ConfigFileWithRoles(), mem.Snapshot()[memKey(configPath)].Data)
}

func Test_init_without_agents_leaves_installed_agents_alone(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	for _, a := range res.Artifacts {
		assert.NotEqual(t, setup.KindAgent, a.Kind)
	}

	control, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	var agentCount int
	for _, a := range control.Artifacts {
		if a.Kind == setup.KindAgent {
			agentCount++
		}
	}
	assert.Equal(t, 3, agentCount)
}

func Test_init_with_agents_for_host_none_refuses_and_writes_nothing(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	before := mem.Snapshot()
	srv := newMemServer(mem)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, WithAgents: true})

	require.ErrorIs(t, err, setup.ErrAgentsNeedHost)
	assert.Equal(t, before, mem.Snapshot())
}

// olderPlannerBytes and olderImplementerBytes are agent renders that predate
// the current format, used to exercise the upgrade path.
const (
	olderPlannerBytes     = "---\nname: planner\ndescription: Turn a feature's specification into ordered scenario plans.\ntools: Read, Grep, Glob, Bash, Edit, Write\n---\n\nTurn the feature's specification into ordered scenario plans: run `brief new step <feature>` for the next scenario, then fill its plan file. Never write production or test code.\n"
	olderImplementerBytes = "---\nname: implementer\ndescription: Implement a feature's next open step, from brief start through brief finish.\n---\n\nRun `brief start <feature>` and implement its next open step, working from its output rather than reading the specification or earlier steps whole. Close the step with `brief finish <feature> <step> --handoff <path> --state <path>`.\n"
)

func Test_init_with_agents_upgrades_an_older_agent_file(t *testing.T) {
	cases := []struct {
		name         string
		role         string
		seedBytes    []byte
		wantAction   setup.Action
		wantDetail   string
		wantBytes    []byte
		wantModified bool
	}{
		{
			name: "older planner is merged and updated", role: "planner",
			seedBytes:  []byte(olderPlannerBytes),
			wantAction: setup.ActionMerged, wantDetail: "updated",
			wantBytes: artifact.AgentPlanner(), wantModified: true,
		},
		{
			name: "older implementer is merged and updated", role: "implementer",
			seedBytes:  []byte(olderImplementerBytes),
			wantAction: setup.ActionMerged, wantDetail: "updated",
			wantBytes: artifact.AgentImplementer(), wantModified: true,
		},
		{
			name: "edited planner stays kept", role: "planner",
			seedBytes:  []byte("---\nname: planner\nedited: true\n---\n"),
			wantAction: setup.ActionKept, wantDetail: "edited locally",
			wantBytes: []byte("---\nname: planner\nedited: true\n---\n"), wantModified: false,
		},
		{
			name: "current implementer stays unchanged", role: "implementer",
			seedBytes:  artifact.AgentImplementer(),
			wantAction: setup.ActionUnchanged, wantDetail: "",
			wantBytes: artifact.AgentImplementer(), wantModified: false,
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

			res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
			require.NoError(t, err)

			art := findArtifactByPath(t, res, path)
			assert.Equal(t, c.wantAction, art.Action)
			assert.Equal(t, c.wantDetail, art.Detail)
			assert.Equal(t, c.wantModified, slices.Contains(res.Modified, path))

			assert.Equal(t, c.wantBytes, mem.Snapshot()[memKey(path)].Data)
		})
	}
}

func Test_init_with_agents_dry_run_and_print_show_an_older_agent_upgrade(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	planner := agentFilePaths(wd).Planner
	require.NoError(t, mem.WriteFile(memKey(planner), []byte(olderPlannerBytes), 0o600))

	before := mem.Snapshot()

	dryRes, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true, DryRun: true})
	require.NoError(t, err)

	dryArt := findArtifactByPath(t, dryRes, planner)
	assert.Equal(t, setup.ActionMerged, dryArt.Action)
	assert.Equal(t, "updated", dryArt.Detail)

	assert.Equal(t, before, mem.Snapshot())

	printRes, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true, Print: true})
	require.NoError(t, err)

	var printArt setup.PrintArtifact
	var found bool
	for _, p := range printRes.Print {
		if p.Path == planner {
			printArt = p
			found = true
		}
	}
	require.True(t, found, "planner.md must appear in Result.Print")
	assert.Equal(t, setup.PrintMerge, printArt.Action)
	assert.Equal(t, string(artifact.AgentPlanner()), printArt.Body)

	assert.Equal(t, before, mem.Snapshot())
}

func Test_init_without_agents_leaves_an_older_agent_file_alone(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	planner := agentFilePaths(wd).Planner
	require.NoError(t, mem.WriteFile(memKey(planner), []byte(olderPlannerBytes), 0o600))

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	for _, a := range res.Artifacts {
		assert.NotEqual(t, setup.KindAgent, a.Kind)
	}

	assert.Equal(t, []byte(olderPlannerBytes), mem.Snapshot()[memKey(planner)].Data)
}

func Test_init_with_agents_dry_run_writes_nothing_but_reports_roles_to_add(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, mem.WriteFile(memKey(configPath), artifact.ConfigFile(), 0o600))
	before := mem.Snapshot()
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true, DryRun: true})

	require.NoError(t, err)
	assert.True(t, res.DryRun)

	featureRoot := filepath.Join(wd, "docs", "specifications")
	pluginPaths := pluginFilePaths(wd)
	agentPaths := agentFilePaths(wd)
	assert.ElementsMatch(t, []setup.Artifact{
		{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionUnchanged},
		{Kind: setup.KindFeatureRoot, Path: featureRoot, Action: setup.ActionCreated},
		{Kind: setup.KindPlugin, Path: pluginPaths.Manifest, Action: setup.ActionCreated},
		{Kind: setup.KindPlugin, Path: pluginPaths.Start, Action: setup.ActionCreated},
		{Kind: setup.KindPlugin, Path: pluginPaths.Finish, Action: setup.ActionCreated},
		{Kind: setup.KindHook, Path: pluginPaths.Hooks, Action: setup.ActionCreated},
		{Kind: setup.KindSkill, Path: pluginPaths.Skill, Action: setup.ActionCreated},
		{Kind: setup.KindAgent, Path: agentPaths.Planner, Action: setup.ActionCreated},
		{Kind: setup.KindAgent, Path: agentPaths.Implementer, Action: setup.ActionCreated},
		{Kind: setup.KindAgent, Path: agentPaths.Reviewer, Action: setup.ActionCreated},
		{Kind: setup.KindSnippet, Path: pluginPaths.ClaudeMD, Action: setup.ActionCreated},
	}, res.Artifacts)

	assert.Empty(t, res.Created)
	assert.Equal(t, []string{
		"roles:",
		"  planner: brief:planner",
		"  implementer: brief:implementer",
		"  reviewer: brief:reviewer",
	}, res.RolesToAdd)

	assert.Equal(t, before, mem.Snapshot())
}
