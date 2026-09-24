package setup_test

import (
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skillFilePath returns root's own brief-workflow SKILL.md absolute path.
func skillFilePath(root string) string {
	return filepath.Join(root, ".claude", "skills", "brief-workflow", "SKILL.md")
}

// Test_init_for_claude_code_writes_the_workflow_skill_after_the_hook pins
// the skill's own row position (Rule 1) by relative position rather than a
// hardcoded full Kind sequence, so it holds regardless of which other flags
// change what sits immediately around the skill row: the skill row comes
// after every KindPlugin/KindHook row and before every KindAgent/KindSnippet
// row, over the three shapes that change what those neighbors are.
func Test_init_for_claude_code_writes_the_workflow_skill_after_the_hook(t *testing.T) {
	cases := []struct {
		name string
		req  setup.InitRequest
	}{
		{name: "plain claude-code install", req: setup.InitRequest{Host: setup.HostClaudeCode}},
		{name: "no-hook install", req: setup.InitRequest{Host: setup.HostClaudeCode, NoHook: true}},
		{name: "with-agents install", req: setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := fsAbs("repo")
			mem := newVirtualMem(wd)
			srv := newMemServer(mem)

			res, err := srv.Init(t.Context(), wd, c.req)

			require.NoError(t, err)

			skillIdx, lastPluginOrHookIdx, firstAgentOrSnippetIdx := -1, -1, len(res.Artifacts)

			for i, a := range res.Artifacts {
				switch a.Kind {
				case setup.KindSkill:
					skillIdx = i
				case setup.KindPlugin, setup.KindHook:
					lastPluginOrHookIdx = i
				case setup.KindAgent, setup.KindSnippet:
					if firstAgentOrSnippetIdx == len(res.Artifacts) {
						firstAgentOrSnippetIdx = i
					}
				case setup.KindConfig, setup.KindFeatureRoot, setup.KindBoundAgent:
				}
			}

			require.NotEqualf(t, -1, skillIdx, "no KindSkill row in %v", res.Artifacts)
			assert.Greater(t, skillIdx, lastPluginOrHookIdx, "the skill row must come after every plugin/hook row")
			assert.Less(t, skillIdx, firstAgentOrSnippetIdx, "the skill row must come before every agent/snippet row")

			skillArt := res.Artifacts[skillIdx]
			assert.Equal(t, skillFilePath(wd), skillArt.Path)
			assert.Equal(t, setup.ActionCreated, skillArt.Action)

			assert.Equal(t, artifact.SkillWorkflow(), mem.Snapshot()[memKey(skillFilePath(wd))].Data)
		})
	}
}

// Test_init_for_host_none_writes_no_skill_row pins the host-gated planning
// rule for the skill: --host none plans no skill row and writes no file,
// while the control arm — the identical repository, initialized under
// claude-code — does.
func Test_init_for_host_none_writes_no_skill_row(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})

	require.NoError(t, err)
	for _, a := range res.Artifacts {
		assert.NotEqual(t, setup.KindSkill, a.Kind)
	}

	assert.NotContains(t, mem.Snapshot(), memKey(skillFilePath(wd)))

	control := fsAbs("control")
	controlMem := newVirtualMem(control)
	controlSrv := newMemServer(controlMem)
	controlRes, err := controlSrv.Init(t.Context(), control, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	var sawSkill bool
	for _, a := range controlRes.Artifacts {
		if a.Kind == setup.KindSkill {
			sawSkill = true
		}
	}
	assert.True(t, sawSkill, "the control install must have planned a skill row")
}

// Test_rerunning_init_for_claude_code_reports_the_skill_unchanged pins R3's
// convergence for the skill file: a second, identical run reports it
// ActionUnchanged and writes nothing further.
func Test_rerunning_init_for_claude_code_reports_the_skill_unchanged(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	skillArt := findArtifact(t, res, setup.KindSkill)
	assert.Equal(t, setup.ActionUnchanged, skillArt.Action)
	assert.NotContains(t, res.Created, skillFilePath(wd))
}

// Test_init_keeps_an_edited_skill_file_even_under_force pins R6's own
// asymmetry for the skill file, mirroring a plugin file's own branch: an
// edited copy is kept, ActionKept, detail "edited locally", byte-identical,
// even under --force — which only ever rewrites the config.
func Test_init_keeps_an_edited_skill_file_even_under_force(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	edited := []byte("---\nedited by hand\n---\n")
	require.NoError(t, mem.WriteFile(memKey(skillFilePath(wd)), edited, 0o600))

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, Force: true})

	require.NoError(t, err)

	skillArt := findArtifact(t, res, setup.KindSkill)
	assert.Equal(t, setup.Artifact{Kind: setup.KindSkill, Path: skillFilePath(wd), Action: setup.ActionKept, Detail: "edited locally"}, skillArt)

	assert.Equal(t, edited, mem.Snapshot()[memKey(skillFilePath(wd))].Data)
}

// Test_init_keeps_a_skill_path_that_is_not_a_regular_file pins the Lstat
// guard mirroring a plugin file's own: a directory at the skill's own path
// is kept, detail "not a regular file", never followed nor written.
func Test_init_keeps_a_skill_path_that_is_not_a_regular_file(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	require.NoError(t, mem.MkdirAll(memKey(skillFilePath(wd)), 0o755))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	skillArt := findArtifact(t, res, setup.KindSkill)
	assert.Equal(t, setup.Artifact{Kind: setup.KindSkill, Path: skillFilePath(wd), Action: setup.ActionKept, Detail: "not a regular file"}, skillArt)
}

// Test_init_dry_run_for_claude_code_reports_the_skill_row_and_writes_nothing
// pins R9 for the skill: the same pending row a real run would report, and
// no bytes written to mem afterward.
func Test_init_dry_run_for_claude_code_reports_the_skill_row_and_writes_nothing(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	before := mem.Snapshot()
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, DryRun: true})

	require.NoError(t, err)

	skillArt := findArtifact(t, res, setup.KindSkill)
	assert.Equal(t, setup.ActionCreated, skillArt.Action)

	assert.Equal(t, before, mem.Snapshot())
}

// Test_init_print_reports_the_skill_body pins R9's --print shape for the
// skill: a PrintCreate entry whose body equals artifact.SkillWorkflow(),
// and nothing written to mem.
func Test_init_print_reports_the_skill_body(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	before := mem.Snapshot()
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, Print: true})

	require.NoError(t, err)

	var skillPrint setup.PrintArtifact
	for _, a := range res.Print {
		if a.Path == skillFilePath(wd) {
			skillPrint = a
		}
	}
	assert.Equal(t, setup.PrintArtifact{Path: skillFilePath(wd), Action: setup.PrintCreate, Body: string(artifact.SkillWorkflow())}, skillPrint)

	assert.Equal(t, before, mem.Snapshot())
}

// Test_init_over_an_install_without_the_skill_creates_only_it pins the
// upgrade path every current adopter hits: a repository already carrying
// every other claude-code artifact but no skill file (the pre-S01 shape)
// reruns to create only the skill row — everything else already converged
// to ActionUnchanged.
func Test_init_over_an_install_without_the_skill_creates_only_it(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)
	require.NoError(t, mem.Remove(memKey(skillFilePath(wd))))

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	assert.Equal(t, []string{skillFilePath(wd)}, res.Created)

	paths := pluginFilePaths(wd)
	wantActions := map[string]setup.Action{
		filepath.Join(wd, ".brief.yaml"):            setup.ActionUnchanged,
		filepath.Join(wd, "docs", "specifications"): setup.ActionUnchanged,
		paths.Manifest: setup.ActionUnchanged,
		paths.Start:    setup.ActionUnchanged,
		paths.Finish:   setup.ActionUnchanged,
		paths.Hooks:    setup.ActionUnchanged,
		paths.Skill:    setup.ActionCreated,
		paths.ClaudeMD: setup.ActionUnchanged,
	}

	gotActions := make(map[string]setup.Action, len(res.Artifacts))
	for _, a := range res.Artifacts {
		gotActions[a.Path] = a.Action
	}
	assert.Equal(t, wantActions, gotActions)
}

// Test_uninstall_removes_an_unedited_skill_and_prunes_its_directory pins
// R6's removal contract for the skill file: an unedited copy is removed and
// ".claude/skills/brief-workflow/" is pruned once empty, while a sibling
// ".claude/skills/other/" (never brief's) survives, and the skill row sits
// between the last agent row and the hook row (Uninstall's own reverse of
// Init's write order).
func Test_uninstall_removes_an_unedited_skill_and_prunes_its_directory(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	otherSkillDir := filepath.Join(wd, ".claude", "skills", "other")
	require.NoError(t, mem.MkdirAll(memKey(otherSkillDir), 0o755))
	require.NoError(t, mem.WriteFile(memKey(otherSkillDir)+"/SKILL.md", []byte("mine"), 0o600))

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	var skillIdx, lastAgentIdx, hookIdx int
	for i, a := range res.Artifacts {
		if a.Kind == setup.KindSkill {
			skillIdx = i
		}

		if a.Kind == setup.KindAgent {
			lastAgentIdx = i
		}

		if a.Kind == setup.KindHook {
			hookIdx = i
		}
	}
	assert.Equal(t, setup.ActionRemoved, res.Artifacts[skillIdx].Action)
	assert.Greater(t, skillIdx, lastAgentIdx, "the skill row must come after every agent row")
	assert.Less(t, skillIdx, hookIdx, "the skill row must come before the hook row")

	snap := mem.Snapshot()
	assert.NotContains(t, snap, memKey(skillFilePath(wd)))
	assert.NotContains(t, snap, memKey(filepath.Join(wd, ".claude", "skills", "brief-workflow")), "the now-empty brief-workflow/ directory must be pruned")

	info, ok := snap[memKey(otherSkillDir)]
	require.True(t, ok, "a sibling skill directory brief never wrote must survive")
	assert.True(t, info.Mode.IsDir())

	info, ok = snap[memKey(filepath.Join(wd, ".claude", "skills"))]
	require.True(t, ok, ".claude/skills/ itself must never be pruned")
	assert.True(t, info.Mode.IsDir())
}

// Test_uninstall_keeps_an_edited_skill_unless_forced pins the "edited
// locally" branch for the skill file at Uninstall, mirroring a plugin
// file's own: kept without --force, removed (still detail "edited
// locally") with it.
func Test_uninstall_keeps_an_edited_skill_unless_forced(t *testing.T) {
	edited := []byte("---\nedited by hand\n---\n")

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
			_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
			require.NoError(t, err)

			require.NoError(t, mem.WriteFile(memKey(skillFilePath(wd)), edited, 0o600))

			res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode, Force: tt.force})

			require.NoError(t, err)

			skillArt := findArtifact(t, res, setup.KindSkill)
			assert.Equal(t, setup.Artifact{Kind: setup.KindSkill, Path: skillFilePath(wd), Action: tt.wantAction, Detail: "edited locally", ForceRemovable: tt.wantForceRemovable}, skillArt)

			data, exists := memData(mem.Snapshot(), memKey(skillFilePath(wd)))
			assert.Equal(t, tt.wantExists, exists)
			assert.Equal(t, tt.wantBytes, data)
		})
	}
}

// Test_uninstall_for_host_none_leaves_the_skill_in_place pins the
// host-gated removal planning for the skill, mirroring the plugin's own:
// --host none never plans the skill row, so it survives an uninstall scoped
// to none even though a claude-code install wrote it.
func Test_uninstall_for_host_none_leaves_the_skill_in_place(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	for _, a := range res.Artifacts {
		assert.NotEqual(t, setup.KindSkill, a.Kind)
	}

	require.Contains(t, mem.Snapshot(), memKey(skillFilePath(wd)), "the skill file must survive an uninstall scoped to --host none")
}
