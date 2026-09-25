package setup_test

import (
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_init_print_returns_pending_bodies_and_writes_nothing(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	before := mem.Snapshot()
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, Print: true})

	require.NoError(t, err)
	assert.Equal(t, before, mem.Snapshot())

	configPath := filepath.Join(wd, ".brief.yaml")
	paths := pluginFilePaths(wd)

	want := []setup.PrintArtifact{
		{Path: configPath, Action: setup.PrintCreate, Body: string(artifact.ConfigFile())},
		{Path: paths.Manifest, Action: setup.PrintCreate, Body: string(artifact.PluginManifest())},
		{Path: paths.Start, Action: setup.PrintCreate, Body: string(artifact.SkillStart())},
		{Path: paths.Finish, Action: setup.PrintCreate, Body: string(artifact.SkillFinish())},
		{Path: paths.Hooks, Action: setup.PrintCreate, Body: string(artifact.ClaudeHooks())},
		{Path: paths.Skill, Action: setup.PrintCreate, Body: string(artifact.SkillWorkflow())},
		{Path: paths.ClaudeMD, Action: setup.PrintCreate, Body: string(artifact.SnippetBlock("docs/specifications"))},
	}
	assert.Equal(t, want, res.Print)
}

func Test_init_print_reports_merge_for_an_existing_claude_md(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	claudeMD := filepath.Join(wd, "CLAUDE.md")
	require.NoError(t, mem.WriteFile(memKey(claudeMD), []byte("# hello\n"), 0o600))
	before := mem.Snapshot()
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, Print: true})

	require.NoError(t, err)
	assert.Equal(t, before, mem.Snapshot())

	var snippet setup.PrintArtifact
	for _, a := range res.Print {
		if a.Path == claudeMD {
			snippet = a
		}
	}
	assert.Equal(t, setup.PrintMerge, snippet.Action)
	assert.Equal(t, string(artifact.SnippetBlock("docs/specifications")), snippet.Body)
}

func Test_init_print_with_agents_reports_the_bound_config_and_three_agents(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true, Print: true})

	require.NoError(t, err)

	byPath := make(map[string]setup.PrintArtifact, len(res.Print))
	for _, a := range res.Print {
		byPath[a.Path] = a
	}

	configPath := filepath.Join(wd, ".brief.yaml")
	require.Contains(t, byPath, configPath)
	assert.Equal(t, string(artifact.ConfigFileWithRoles()), byPath[configPath].Body)

	agents := agentFilePaths(wd)
	require.Contains(t, byPath, agents.Planner)
	require.Contains(t, byPath, agents.Implementer)
	require.Contains(t, byPath, agents.Reviewer)
	assert.Equal(t, string(artifact.AgentPlanner()), byPath[agents.Planner].Body)
	assert.Equal(t, string(artifact.AgentImplementer()), byPath[agents.Implementer].Body)
	assert.Equal(t, string(artifact.AgentReviewer()), byPath[agents.Reviewer].Body)
}

func Test_init_print_force_over_a_kept_config_reports_create_with_the_desired_variant(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, mem.WriteFile(memKey(configPath), []byte("feature-directory: specs\n"), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, Force: true, Print: true})

	require.NoError(t, err)
	require.Len(t, res.Print, 1)
	assert.Equal(t, setup.PrintArtifact{Path: configPath, Action: setup.PrintCreate, Body: string(artifact.ConfigFile())}, res.Print[0])
}

func Test_init_print_after_a_real_init_reports_nothing_pending(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, Print: true})

	require.NoError(t, err)
	assert.NotNil(t, res.Print)
	assert.Empty(t, res.Print)
}
