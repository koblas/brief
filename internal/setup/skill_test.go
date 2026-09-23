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

// skillFilePath returns root's own brief-workflow SKILL.md absolute path.
func skillFilePath(root string) string {
	return filepath.Join(root, ".claude", "skills", "brief-workflow", "SKILL.md")
}

// Test_init_for_claude_code_writes_the_workflow_skill_after_the_hook pins
// the skill's own row position (Rule 1): config, feature-root, plugin
// files, hook (or, under NoHook, nothing in its place), skill, then agent
// files (under WithAgents) or the snippet — by table over the three shapes
// that change what sits immediately before or after the skill row.
func Test_init_for_claude_code_writes_the_workflow_skill_after_the_hook(t *testing.T) {
	cases := []struct {
		name      string
		req       setup.InitRequest
		wantKinds []setup.Kind
		wantIndex int
	}{
		{
			name: "plain claude-code install",
			req:  setup.InitRequest{Host: setup.HostClaudeCode},
			wantKinds: []setup.Kind{
				setup.KindConfig, setup.KindFeatureRoot,
				setup.KindPlugin, setup.KindPlugin, setup.KindPlugin, setup.KindHook,
				setup.KindSkill,
				setup.KindSnippet,
			},
			wantIndex: 6,
		},
		{
			name: "no-hook install",
			req:  setup.InitRequest{Host: setup.HostClaudeCode, NoHook: true},
			wantKinds: []setup.Kind{
				setup.KindConfig, setup.KindFeatureRoot,
				setup.KindPlugin, setup.KindPlugin, setup.KindPlugin,
				setup.KindSkill,
				setup.KindSnippet,
			},
			wantIndex: 5,
		},
		{
			name: "with-agents install",
			req:  setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true},
			wantKinds: []setup.Kind{
				setup.KindConfig, setup.KindFeatureRoot,
				setup.KindPlugin, setup.KindPlugin, setup.KindPlugin, setup.KindHook,
				setup.KindSkill,
				setup.KindAgent, setup.KindAgent, setup.KindAgent,
				setup.KindSnippet,
			},
			wantIndex: 6,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := t.TempDir()
			srv := newServer(t)

			res, err := srv.Init(t.Context(), wd, c.req)

			require.NoError(t, err)
			require.Len(t, res.Artifacts, len(c.wantKinds))

			kinds := make([]setup.Kind, len(res.Artifacts))
			for i, a := range res.Artifacts {
				kinds[i] = a.Kind
			}
			assert.Equal(t, c.wantKinds, kinds)

			skillArt := res.Artifacts[c.wantIndex]
			assert.Equal(t, setup.KindSkill, skillArt.Kind)
			assert.Equal(t, skillFilePath(wd), skillArt.Path)
			assert.Equal(t, setup.ActionCreated, skillArt.Action)

			body, readErr := os.ReadFile(skillFilePath(wd))
			require.NoError(t, readErr)
			assert.Equal(t, artifact.SkillWorkflow(), body)
		})
	}
}

// Test_init_for_host_none_writes_no_skill_row pins the host-gated planning
// rule for the skill: --host none plans no skill row and writes no file,
// while the control arm — the identical repository, initialized under
// claude-code — does.
func Test_init_for_host_none_writes_no_skill_row(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})

	require.NoError(t, err)
	for _, a := range res.Artifacts {
		assert.NotEqual(t, setup.KindSkill, a.Kind)
	}

	_, statErr := os.Stat(skillFilePath(wd))
	assert.True(t, os.IsNotExist(statErr))

	control := t.TempDir()
	controlRes, err := srv.Init(t.Context(), control, setup.InitRequest{Host: setup.HostClaudeCode})
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
	wd := t.TempDir()
	srv := newServer(t)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	var skillArt setup.Artifact
	for _, a := range res.Artifacts {
		if a.Kind == setup.KindSkill {
			skillArt = a
		}
	}
	assert.Equal(t, setup.ActionUnchanged, skillArt.Action)
	assert.NotContains(t, res.Created, skillFilePath(wd))
}

// Test_init_keeps_an_edited_skill_file_even_under_force pins R6's own
// asymmetry for the skill file, mirroring a plugin file's own branch: an
// edited copy is kept, ActionKept, detail "edited locally", byte-identical,
// even under --force — which only ever rewrites the config.
func Test_init_keeps_an_edited_skill_file_even_under_force(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	edited := []byte("---\nedited by hand\n---\n")
	require.NoError(t, os.WriteFile(skillFilePath(wd), edited, 0o600))

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, Force: true})

	require.NoError(t, err)

	var skillArt setup.Artifact
	for _, a := range res.Artifacts {
		if a.Kind == setup.KindSkill {
			skillArt = a
		}
	}
	assert.Equal(t, setup.Artifact{Kind: setup.KindSkill, Path: skillFilePath(wd), Action: setup.ActionKept, Detail: "edited locally"}, skillArt)

	body, readErr := os.ReadFile(skillFilePath(wd))
	require.NoError(t, readErr)
	assert.Equal(t, edited, body)
}

// Test_init_keeps_a_skill_path_that_is_not_a_regular_file pins the Lstat
// guard mirroring a plugin file's own: a directory at the skill's own path
// is kept, detail "not a regular file", never followed nor written.
func Test_init_keeps_a_skill_path_that_is_not_a_regular_file(t *testing.T) {
	wd := t.TempDir()
	require.NoError(t, os.MkdirAll(skillFilePath(wd), 0o755))
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)

	var skillArt setup.Artifact
	for _, a := range res.Artifacts {
		if a.Kind == setup.KindSkill {
			skillArt = a
		}
	}
	assert.Equal(t, setup.Artifact{Kind: setup.KindSkill, Path: skillFilePath(wd), Action: setup.ActionKept, Detail: "not a regular file"}, skillArt)
}

// Test_init_dry_run_for_claude_code_reports_the_skill_row_and_writes_nothing
// pins R9 for the skill: the same pending row a real run would report, and
// no file on disk afterward.
func Test_init_dry_run_for_claude_code_reports_the_skill_row_and_writes_nothing(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, DryRun: true})

	require.NoError(t, err)

	var skillArt setup.Artifact
	for _, a := range res.Artifacts {
		if a.Kind == setup.KindSkill {
			skillArt = a
		}
	}
	assert.Equal(t, setup.ActionCreated, skillArt.Action)

	_, statErr := os.Stat(skillFilePath(wd))
	assert.True(t, os.IsNotExist(statErr))
}

// Test_init_print_reports_the_skill_body pins R9's --print shape for the
// skill: a PrintCreate entry whose body equals artifact.SkillWorkflow(),
// and nothing written to disk.
func Test_init_print_reports_the_skill_body(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, Print: true})

	require.NoError(t, err)

	var skillPrint setup.PrintArtifact
	for _, a := range res.Print {
		if a.Path == skillFilePath(wd) {
			skillPrint = a
		}
	}
	assert.Equal(t, setup.PrintArtifact{Path: skillFilePath(wd), Action: setup.PrintCreate, Body: string(artifact.SkillWorkflow())}, skillPrint)

	_, statErr := os.Stat(skillFilePath(wd))
	assert.True(t, os.IsNotExist(statErr))
}

// Test_init_over_an_install_without_the_skill_creates_only_it pins the
// upgrade path every current adopter hits: a repository already carrying
// every other claude-code artifact but no skill file (the pre-S01 shape)
// reruns to create only the skill row — everything else already converged
// to ActionUnchanged.
func Test_init_over_an_install_without_the_skill_creates_only_it(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)
	require.NoError(t, os.Remove(skillFilePath(wd)))

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	assert.Equal(t, []string{skillFilePath(wd)}, res.Created)

	for _, a := range res.Artifacts {
		if a.Kind == setup.KindSkill {
			assert.Equal(t, setup.ActionCreated, a.Action)

			continue
		}

		assert.Equalf(t, setup.ActionUnchanged, a.Action, "artifact %s must already have converged", a.Path)
	}
}

// Test_uninstall_removes_an_unedited_skill_and_prunes_its_directory pins
// R6's removal contract for the skill file: an unedited copy is removed and
// ".claude/skills/brief-workflow/" is pruned once empty, while a sibling
// ".claude/skills/other/" (never brief's) survives, and the skill row sits
// between the last agent row and the hook row (Uninstall's own reverse of
// Init's write order).
func Test_uninstall_removes_an_unedited_skill_and_prunes_its_directory(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	otherSkillDir := filepath.Join(wd, ".claude", "skills", "other")
	require.NoError(t, os.MkdirAll(otherSkillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(otherSkillDir, "SKILL.md"), []byte("mine"), 0o600))

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

	_, statErr := os.Stat(skillFilePath(wd))
	assert.True(t, os.IsNotExist(statErr))

	_, statErr = os.Stat(filepath.Join(wd, ".claude", "skills", "brief-workflow"))
	assert.True(t, os.IsNotExist(statErr), "the now-empty brief-workflow/ directory must be pruned")

	info, statErr := os.Stat(otherSkillDir)
	require.NoError(t, statErr, "a sibling skill directory brief never wrote must survive")
	assert.True(t, info.IsDir())

	info, statErr = os.Stat(filepath.Join(wd, ".claude", "skills"))
	require.NoError(t, statErr, ".claude/skills/ itself must never be pruned")
	assert.True(t, info.IsDir())
}

// Test_uninstall_keeps_an_edited_skill_unless_forced pins the "edited
// locally" branch for the skill file at Uninstall, mirroring a plugin
// file's own: kept without --force, removed (still detail "edited
// locally") with it.
func Test_uninstall_keeps_an_edited_skill_unless_forced(t *testing.T) {
	tests := []struct {
		name               string
		force              bool
		wantAction         setup.Action
		wantForceRemovable bool
	}{
		{name: "no force: kept", force: false, wantAction: setup.ActionKept, wantForceRemovable: true},
		{name: "force: removed", force: true, wantAction: setup.ActionRemoved, wantForceRemovable: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			srv := newServer(t)
			_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
			require.NoError(t, err)

			edited := []byte("---\nedited by hand\n---\n")
			require.NoError(t, os.WriteFile(skillFilePath(wd), edited, 0o600))

			res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode, Force: tt.force})

			require.NoError(t, err)

			var skillArt setup.Artifact
			for _, a := range res.Artifacts {
				if a.Kind == setup.KindSkill {
					skillArt = a
				}
			}
			assert.Equal(t, setup.Artifact{Kind: setup.KindSkill, Path: skillFilePath(wd), Action: tt.wantAction, Detail: "edited locally", ForceRemovable: tt.wantForceRemovable}, skillArt)

			_, statErr := os.Stat(skillFilePath(wd))
			if tt.force {
				assert.True(t, os.IsNotExist(statErr))

				return
			}

			require.NoError(t, statErr)
			body, readErr := os.ReadFile(skillFilePath(wd))
			require.NoError(t, readErr)
			assert.Equal(t, edited, body)
		})
	}
}

// Test_uninstall_for_host_none_leaves_the_skill_in_place pins the
// host-gated removal planning for the skill, mirroring the plugin's own:
// --host none never plans the skill row, so it survives an uninstall scoped
// to none even though a claude-code install wrote it.
func Test_uninstall_for_host_none_leaves_the_skill_in_place(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	for _, a := range res.Artifacts {
		assert.NotEqual(t, setup.KindSkill, a.Kind)
	}

	_, statErr := os.Stat(skillFilePath(wd))
	require.NoError(t, statErr, "the skill file must survive an uninstall scoped to --host none")
}
