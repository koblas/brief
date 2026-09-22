package setup_test

import (
	"os"
	"path/filepath"
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

// Test_init_with_agents_writes_three_agents_and_a_config_binding_them pins
// the fresh-repository happy path: ten rows in order (config, feature
// root, the four plugin files, the three agents, then the snippet), each
// agent ActionCreated with KindAgent, the config's own bytes equal
// artifact.ConfigFileWithRoles(), the three agent files' own bytes equal
// their own render, and RolesToAdd is empty — the config this run wrote
// already binds every role.
func Test_init_with_agents_writes_three_agents_and_a_config_binding_them(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})

	require.NoError(t, err)
	require.Len(t, res.Artifacts, 10)

	kinds := make([]setup.Kind, len(res.Artifacts))
	for i, a := range res.Artifacts {
		kinds[i] = a.Kind
	}
	assert.Equal(t, []setup.Kind{
		setup.KindConfig, setup.KindFeatureRoot,
		setup.KindPlugin, setup.KindPlugin, setup.KindPlugin, setup.KindHook,
		setup.KindAgent, setup.KindAgent, setup.KindAgent,
		setup.KindSnippet,
	}, kinds)

	paths := agentFilePaths(wd)
	assert.Equal(t, paths.Planner, res.Artifacts[6].Path)
	assert.Equal(t, paths.Implementer, res.Artifacts[7].Path)
	assert.Equal(t, paths.Reviewer, res.Artifacts[8].Path)

	for _, i := range []int{6, 7, 8} {
		assert.Equalf(t, setup.ActionCreated, res.Artifacts[i].Action, "artifact %d must be created", i)
	}

	assert.Equal(t, []string{}, res.RolesToAdd)

	configBody, readErr := os.ReadFile(filepath.Join(wd, ".brief.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, artifact.ConfigFileWithRoles(), configBody)

	plannerBody, readErr := os.ReadFile(paths.Planner)
	require.NoError(t, readErr)
	assert.Equal(t, artifact.AgentPlanner(), plannerBody)

	implementerBody, readErr := os.ReadFile(paths.Implementer)
	require.NoError(t, readErr)
	assert.Equal(t, artifact.AgentImplementer(), implementerBody)

	reviewerBody, readErr := os.ReadFile(paths.Reviewer)
	require.NoError(t, readErr)
	assert.Equal(t, artifact.AgentReviewer(), reviewerBody)
}

// Test_rerunning_init_with_agents_reports_every_agent_unchanged pins R3's
// convergence for the three agent files: a second, identical run reports
// every one of the ten artifacts ActionUnchanged and writes nothing
// further.
func Test_rerunning_init_with_agents_reports_every_agent_unchanged(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})

	require.NoError(t, err)
	assert.Empty(t, res.Created)
	require.Len(t, res.Artifacts, 10)
	for _, a := range res.Artifacts {
		assert.Equalf(t, setup.ActionUnchanged, a.Action, "artifact %s must report unchanged", a.Path)
	}
	assert.Equal(t, []string{}, res.RolesToAdd)
}

// Test_init_keeps_an_unrecognized_agent_file_even_under_force pins R7's
// last sentence: an agents/<role>.md file whose bytes match no known
// render is the adopter's own customisation, kept even under --force,
// mirroring a plugin file's own "edited locally" branch exactly.
func Test_init_keeps_an_unrecognized_agent_file_even_under_force(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	planner := agentFilePaths(wd).Planner
	edited := []byte("---\nname: planner\nedited: true\n---\n")
	require.NoError(t, os.WriteFile(planner, edited, 0o600))

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true, Force: true})

	require.NoError(t, err)

	var plannerArt setup.Artifact
	for _, a := range res.Artifacts {
		if a.Path == planner {
			plannerArt = a
		}
	}
	assert.Equal(t, setup.Artifact{Kind: setup.KindAgent, Path: planner, Action: setup.ActionKept, Detail: "edited locally"}, plannerArt)

	body, readErr := os.ReadFile(planner)
	require.NoError(t, readErr)
	assert.Equal(t, edited, body)
}

// Test_init_with_agents_never_edits_an_existing_config pins R7's core
// promise by table: a config already present before this run — whether it
// is the plain unbound render or a validly edited one — is never rewritten
// by --with-agents; its bytes on disk are byte-identical before and after,
// and RolesToAdd lists the "roles:" header plus all three unbound roles
// (neither fixture binds any of them).
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
			wd := t.TempDir()
			configPath := filepath.Join(wd, ".brief.yaml")
			require.NoError(t, os.WriteFile(configPath, c.body, 0o600))
			srv := newServer(t)

			res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})

			require.NoError(t, err)
			assert.Equal(t, c.wantAction, res.Artifacts[0].Action)
			assert.NotContains(t, res.Created, configPath)
			assert.NotContains(t, res.Modified, configPath)

			body, readErr := os.ReadFile(configPath)
			require.NoError(t, readErr)
			assert.Equal(t, c.body, body)

			assert.Equal(t, []string{
				"roles:",
				"  planner: brief:planner",
				"  implementer: brief:implementer",
				"  reviewer: brief:reviewer",
			}, res.RolesToAdd)
		})
	}
}

// Test_roles_to_add_lists_only_unbound_roles pins the "never shadows an
// existing binding" rule (R15): a config that already binds one role to
// the adopter's own agent and a second to brief's own agent lists only the
// third, still-unbound role; a config binding all three lists nothing.
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
			wd := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), c.body, 0o600))
			srv := newServer(t)

			res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})

			require.NoError(t, err)
			assert.Equal(t, c.want, res.RolesToAdd)
		})
	}
}

// Test_roles_to_add_lines_parse_into_brief_bindings is the control arm for
// the stderr/JSON hint itself: appending RolesToAdd's own lines to a
// config carrying no "roles:" key at all and decoding the result
// (config.Inspect) must yield exactly artifact.AgentBindings(), with no
// violation — proving the advice the hint prints actually works.
func Test_roles_to_add_lines_parse_into_brief_bindings(t *testing.T) {
	wd := t.TempDir()
	base := "feature-directory: docs/specifications\n"
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(base), 0o600))
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)
	require.NotEmpty(t, res.RolesToAdd)

	appended := base
	for _, line := range res.RolesToAdd {
		appended += line + "\n"
	}
	require.NoError(t, os.WriteFile(configPath, []byte(appended), 0o600))

	cfg, violations, inspectErr := config.Inspect(configPath)

	require.NoError(t, inspectErr)
	assert.Empty(t, violations)
	assert.Equal(t, artifact.AgentBindings(), cfg.Roles)
}

// Test_init_with_agents_on_a_bound_config_reports_it_unchanged_with_nothing_to_add
// pins the fully-converged case: a config already carrying today's own
// bound render (ConfigFileWithRoles) reports ActionUnchanged, never
// rewritten, and RolesToAdd is empty — every role is already bound.
func Test_init_with_agents_on_a_bound_config_reports_it_unchanged_with_nothing_to_add(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, artifact.ConfigFileWithRoles(), 0o600))
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})

	require.NoError(t, err)
	assert.Equal(t, setup.ActionUnchanged, res.Artifacts[0].Action)
	assert.Equal(t, []string{}, res.RolesToAdd)

	body, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, artifact.ConfigFileWithRoles(), body)
}

// Test_force_init_with_agents_rewrites_a_plain_config_to_the_bound_variant
// pins --force --with-agents' own target: a config holding exactly the
// plain render (a recognized, but unbound, current render — never
// rewritten without --force) is rewritten to ConfigFileWithRoles() under
// --force, reported ActionCreated, detail "rewritten from defaults".
func Test_force_init_with_agents_rewrites_a_plain_config_to_the_bound_variant(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, artifact.ConfigFile(), 0o600))
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true, Force: true})

	require.NoError(t, err)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated, Detail: "rewritten from defaults"}, res.Artifacts[0])

	body, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, artifact.ConfigFileWithRoles(), body)
}

// Test_init_without_agents_leaves_installed_agents_alone pins the "no
// flag, no plan, no row" rule (mirroring --no-hook): against a repository
// that already has the three agent files installed, a plain "init
// --host claude-code" plans no agent row at all and never reads or writes
// them, while the control arm — the identical fixture, rerun with
// WithAgents — still reports all three.
func Test_init_without_agents_leaves_installed_agents_alone(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)
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

// Test_init_with_agents_for_host_none_refuses_and_writes_nothing pins the
// flag-combination rule: --with-agents is checked against the resolved
// host, so --host none (or the still-unresolved bare default before S09)
// refuses with setup.ErrAgentsNeedHost and changes nothing on disk.
func Test_init_with_agents_for_host_none_refuses_and_writes_nothing(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, WithAgents: true})

	require.ErrorIs(t, err, setup.ErrAgentsNeedHost)

	entries, readErr := os.ReadDir(wd)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

// Test_init_with_agents_dry_run_writes_nothing_but_reports_roles_to_add
// pins R9 for --with-agents: the same ten rows a real run would report,
// RolesToAdd still populated against the pre-existing config the fixture
// seeds, and nothing written to disk.
func Test_init_with_agents_dry_run_writes_nothing_but_reports_roles_to_add(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, artifact.ConfigFile(), 0o600))
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true, DryRun: true})

	require.NoError(t, err)
	assert.True(t, res.DryRun)
	require.Len(t, res.Artifacts, 10)
	assert.Empty(t, res.Created)
	assert.Equal(t, []string{
		"roles:",
		"  planner: brief:planner",
		"  implementer: brief:implementer",
		"  reviewer: brief:reviewer",
	}, res.RolesToAdd)

	_, statErr := os.Stat(agentFilePaths(wd).Planner)
	assert.True(t, os.IsNotExist(statErr))

	body, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, artifact.ConfigFile(), body)
}
