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

// Test_init_print_returns_pending_bodies_and_writes_nothing pins R9's own
// artifact set at the Server boundary: a fresh, plain claude-code install
// reports the config, every plugin file, the brief-workflow skill and the
// CLAUDE.md block, each PrintCreate, bodies equal to what a real run would
// write, and the tree byte-identical before and after.
func Test_init_print_returns_pending_bodies_and_writes_nothing(t *testing.T) {
	wd := t.TempDir()
	before := snapshotTree(t, wd)
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, Print: true})

	require.NoError(t, err)
	assert.Equal(t, before, snapshotTree(t, wd))

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

// Test_init_print_reports_merge_for_an_existing_claude_md pins the
// "merge" verb: a CLAUDE.md already present without a brief block plans
// ActionMerged, so its own PrintArtifact reports PrintMerge, body still
// the bare block rather than the merged file.
func Test_init_print_reports_merge_for_an_existing_claude_md(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), []byte("# hello\n"), 0o600))
	before := snapshotTree(t, wd)
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, Print: true})

	require.NoError(t, err)
	assert.Equal(t, before, snapshotTree(t, wd))

	var snippet setup.PrintArtifact
	for _, a := range res.Print {
		if a.Path == filepath.Join(wd, "CLAUDE.md") {
			snippet = a
		}
	}
	assert.Equal(t, setup.PrintMerge, snippet.Action)
	assert.Equal(t, string(artifact.SnippetBlock("docs/specifications")), snippet.Body)
}

// Test_init_print_with_agents_reports_the_bound_config_and_three_agents
// pins --with-agents' own contribution to Print: the config's own body is
// ConfigFileWithRoles(), and the three agent files each report
// PrintCreate with their own render.
func Test_init_print_with_agents_reports_the_bound_config_and_three_agents(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)

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

// Test_init_print_force_over_a_kept_config_reports_create_with_the_desired_variant
// pins --force --print together: a valid but non-current config is
// planned ActionCreated ("rewritten from defaults"), so Print reports it
// PrintCreate with the desired variant's own bytes — the plain render
// here, since WithAgents is not set.
func Test_init_print_force_over_a_kept_config_reports_create_with_the_desired_variant(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("feature-directory: specs\n"), 0o600))
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, Force: true, Print: true})

	require.NoError(t, err)
	require.Len(t, res.Print, 1)
	assert.Equal(t, setup.PrintArtifact{Path: configPath, Action: setup.PrintCreate, Body: string(artifact.ConfigFile())}, res.Print[0])
}

// Test_init_print_after_a_real_init_reports_nothing_pending pins "second
// pass after a real Init → empty, non-nil": every artifact already
// converged, so Print carries no entries but is still a non-nil, empty
// slice rather than nil.
func Test_init_print_after_a_real_init_reports_nothing_pending(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, Print: true})

	require.NoError(t, err)
	assert.NotNil(t, res.Print)
	assert.Empty(t, res.Print)
}
