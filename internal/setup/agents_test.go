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

// Test_init_with_agents_writes_three_agents_and_a_config_binding_them pins
// the fresh-repository happy path: eleven rows in order (config, feature
// root, the four plugin files, the brief-workflow skill, the three agents,
// then the snippet), each agent ActionCreated with KindAgent, the config's
// own bytes equal artifact.ConfigFileWithRoles(), the three agent files'
// own bytes equal their own render, and RolesToAdd is empty — the config
// this run wrote already binds every role. This is the package's own
// full-row-order pin for a --with-agents Init.
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

// Test_rerunning_init_with_agents_reports_every_agent_unchanged pins R3's
// convergence for the three agent files: a second, identical run reports
// every one of the eleven artifacts ActionUnchanged and writes nothing
// further.
func Test_rerunning_init_with_agents_reports_every_agent_unchanged(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})

	require.NoError(t, err)
	assert.Empty(t, res.Created)
	require.Len(t, res.Artifacts, 11)
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

// Test_init_with_agents_never_edits_an_existing_config pins R7's core
// promise by table: a config already present before this run — whether it
// is the plain unbound render or a validly edited one — is never rewritten
// by --with-agents; its bytes are byte-identical before and after, and
// RolesToAdd lists the "roles:" header plus all three unbound roles
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
			wd := fsAbs("repo")
			mem := newVirtualMem(wd)
			configPath := filepath.Join(wd, ".brief.yaml")
			require.NoError(t, mem.WriteFile(memKey(configPath), c.body, 0o600))
			srv := newMemServer(mem)

			res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})

			require.NoError(t, err)
			assert.Equal(t, c.wantAction, res.Artifacts[0].Action)
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

// Test_roles_to_add_lines_parse_into_brief_bindings is the control arm for
// the stderr/JSON hint itself: appending RolesToAdd's own lines to a
// config carrying no "roles:" key at all and decoding the result
// (config.Inspect, which always reads real disk — internal/platform/config
// has no fsys seam of its own) must yield exactly artifact.AgentBindings(),
// with no violation — proving the advice the hint prints actually works.
// Init's own run is Mem-backed; only this last verification step touches a
// second, unrelated real file, since config.Inspect is what is under test
// here, not setup's own planning.
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

// Test_init_with_agents_on_a_bound_config_reports_it_unchanged_with_nothing_to_add
// pins the fully-converged case: a config already carrying today's own
// bound render (ConfigFileWithRoles) reports ActionUnchanged, never
// rewritten, and RolesToAdd is empty — every role is already bound.
func Test_init_with_agents_on_a_bound_config_reports_it_unchanged_with_nothing_to_add(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, mem.WriteFile(memKey(configPath), artifact.ConfigFileWithRoles(), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})

	require.NoError(t, err)
	assert.Equal(t, setup.ActionUnchanged, res.Artifacts[0].Action)
	assert.Equal(t, []string{}, res.RolesToAdd)

	assert.Equal(t, artifact.ConfigFileWithRoles(), mem.Snapshot()[memKey(configPath)].Data)
}

// Test_force_init_with_agents_rewrites_a_plain_config_to_the_bound_variant
// pins --force --with-agents' own target: a config holding exactly the
// plain render (a recognized, but unbound, current render — never
// rewritten without --force) is rewritten to ConfigFileWithRoles() under
// --force, reported ActionCreated, detail "rewritten from defaults".
func Test_force_init_with_agents_rewrites_a_plain_config_to_the_bound_variant(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, mem.WriteFile(memKey(configPath), artifact.ConfigFile(), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true, Force: true})

	require.NoError(t, err)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated, Detail: "rewritten from defaults"}, res.Artifacts[0])

	assert.Equal(t, artifact.ConfigFileWithRoles(), mem.Snapshot()[memKey(configPath)].Data)
}

// Test_init_without_agents_leaves_installed_agents_alone pins the "no
// flag, no plan, no row" rule (mirroring --no-hook): against a repository
// that already has the three agent files installed, a plain "init
// --host claude-code" plans no agent row at all and never reads or writes
// them, while the control arm — the identical fixture, rerun with
// WithAgents — still reports all three.
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

// Test_init_with_agents_for_host_none_refuses_and_writes_nothing pins the
// flag-combination rule: --with-agents is checked against the resolved
// host, so --host none (or the still-unresolved bare default before S09)
// refuses with setup.ErrAgentsNeedHost and changes nothing.
func Test_init_with_agents_for_host_none_refuses_and_writes_nothing(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	before := mem.Snapshot()
	srv := newMemServer(mem)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, WithAgents: true})

	require.ErrorIs(t, err, setup.ErrAgentsNeedHost)
	assert.Equal(t, before, mem.Snapshot())
}

// olderPlannerBytes, olderImplementerBytes are the pre-SCENARIO-02 planner
// and implementer renders, captured mechanically (%q dump) before agents.go
// changed — the same literal artifact's own agents_test.go pins against
// Recognize, copied here since setup_test cannot import an unexported
// artifact fixture.
const (
	olderPlannerBytes     = "---\nname: planner\ndescription: Turn a feature's specification into ordered scenario plans.\ntools: Read, Grep, Glob, Bash, Edit, Write\n---\n\nTurn the feature's specification into ordered scenario plans: run `brief new step <feature>` for the next scenario, then fill its plan file. Never write production or test code.\n"
	olderImplementerBytes = "---\nname: implementer\ndescription: Implement a feature's next open step, from brief start through brief finish.\n---\n\nRun `brief start <feature>` and implement its next open step, working from its output rather than reading the specification or earlier steps whole. Close the step with `brief finish <feature> <step> --handoff <path> --state <path>`.\n"
)

// Test_init_with_agents_upgrades_an_older_agent_file pins Rule 6 at Init's
// own write path: a planner or implementer file holding the pre-SCENARIO-02
// bytes is upgraded — ActionMerged, detail "updated", bytes rewritten to
// today's own ruled render, path in Result.Modified — the generic
// OriginOlder branch planPluginFile/apply share with every plugin Kind.
// The control rows pin the two branches that must NOT move: an edited file
// stays ActionKept "edited locally", untouched, and today's own current
// render stays ActionUnchanged, untouched.
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

// Test_init_with_agents_dry_run_and_print_show_an_older_agent_upgrade pins
// R9 for an ActionMerged agent row: --dry-run reports the merged row and
// leaves the bytes byte-identical to the older bytes it seeded; --print
// emits a PrintMerge PrintArtifact carrying today's own full render as its
// Body, mirroring the snippet's own bare-block precedent, and also writes
// nothing.
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

// Test_init_without_agents_leaves_an_older_agent_file_alone pins the same
// "no flag, no plan, no row" rule this file already pins for an
// unrecognized agent file: a planner holding the pre-SCENARIO-02 bytes is
// never read or rewritten by a plain "init --host claude-code" (no
// --with-agents) — no KindAgent row at all — and its bytes stay
// byte-identical.
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

// Test_init_with_agents_dry_run_writes_nothing_but_reports_roles_to_add
// pins R9 for --with-agents: the same eleven rows a real run would report,
// RolesToAdd still populated against the pre-existing config the fixture
// seeds, and nothing written.
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
	require.Len(t, res.Artifacts, 11)
	assert.Empty(t, res.Created)
	assert.Equal(t, []string{
		"roles:",
		"  planner: brief:planner",
		"  implementer: brief:implementer",
		"  reviewer: brief:reviewer",
	}, res.RolesToAdd)

	assert.Equal(t, before, mem.Snapshot())
}
