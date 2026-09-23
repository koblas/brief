package setup

// White-box package: apply and applyUninstall are unexported and take
// several already-planned artifacts as arguments, letting these tests
// construct a snippetArtifact or pluginArtifact whose own Action and
// existing bytes were decided from stale (planning-time) file content —
// proving the read-modify-write guard (verifyFileUnchanged) actually runs
// inside the real write path Init and Uninstall call, not merely as an
// isolated function (see snippet_internal_test.go's own
// Test_verifyFileUnchanged for that).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_apply_refuses_when_CLAUDE_md_changed_since_planning pins the guard
// at Init's own write path: snippetArt.existing carries bytes planning
// read at some earlier point; the file on disk now holds something else
// (an edit by another process, in this test's stand-in for that race).
// apply refuses, wrapping ErrConcurrentEdit, and — the control this claim
// needs — the file's own bytes are unchanged afterward, proving apply
// never reached writeSnippetFile.
func Test_apply_refuses_when_CLAUDE_md_changed_since_planning(t *testing.T) {
	wd := t.TempDir()
	claudePath := filepath.Join(wd, "CLAUDE.md")
	require.NoError(t, os.WriteFile(claudePath, []byte("edited after planning"), 0o600))

	featureRoot := filepath.Join(wd, "docs", "specifications")
	require.NoError(t, os.MkdirAll(featureRoot, 0o755))

	res := Result{Created: []string{}, Modified: []string{}, Removed: []string{}}
	featureArt := Artifact{Kind: KindFeatureRoot, Path: featureRoot, Action: ActionUnchanged}
	snippetArt := snippetArtifact{
		Kind: KindSnippet, Path: claudePath, Action: ActionMerged,
		existing: []byte("stale planning-time bytes"),
		dir:      "docs/specifications",
	}
	configArt := Artifact{Kind: KindConfig, Path: filepath.Join(wd, ".brief.yaml"), Action: ActionUnchanged}

	_, err := apply(res, featureArt, nil, nil, snippetArt, true, configArt, nil)

	require.ErrorIs(t, err, ErrConcurrentEdit)

	body, readErr := os.ReadFile(claudePath)
	require.NoError(t, readErr)
	assert.Equal(t, "edited after planning", string(body))
}

// Test_apply_wraps_ErrPartialWrite_when_an_earlier_write_already_landed
// pins apply's own ordering guarantee at the write path directly: unlike
// the test above, featureArt is ActionCreated here, so its own
// os.MkdirAll lands before the concurrent-edit guard ever runs. The
// returned error must wrap both ErrConcurrentEdit (what went wrong) and
// ErrPartialWrite (that something already changed) — cli's own
// files_changed and refusal-tail rendering both key off ErrPartialWrite
// being reachable here, not merely off the sentinel this markPartial call
// itself wraps. It must also still carry a *RefusalError through
// markPartial's wrapping: cli's classifyRefusal (internal/cli/refusal.go)
// branches on errors.AsType[*RefusalError], not on the sentinels alone, so
// a markPartial that erased the type would silently fall through to the
// generic errorKindFailure case even though every errors.Is assertion here
// still holds.
func Test_apply_wraps_ErrPartialWrite_when_an_earlier_write_already_landed(t *testing.T) {
	wd := t.TempDir()
	claudePath := filepath.Join(wd, "CLAUDE.md")
	require.NoError(t, os.WriteFile(claudePath, []byte("edited after planning"), 0o600))

	featureRoot := filepath.Join(wd, "docs", "specifications")

	res := Result{Created: []string{}, Modified: []string{}, Removed: []string{}}
	featureArt := Artifact{Kind: KindFeatureRoot, Path: featureRoot, Action: ActionCreated}
	snippetArt := snippetArtifact{
		Kind: KindSnippet, Path: claudePath, Action: ActionMerged,
		existing: []byte("stale planning-time bytes"),
		dir:      "docs/specifications",
	}
	configArt := Artifact{Kind: KindConfig, Path: filepath.Join(wd, ".brief.yaml"), Action: ActionUnchanged}

	_, err := apply(res, featureArt, nil, nil, snippetArt, true, configArt, nil)

	require.ErrorIs(t, err, ErrConcurrentEdit)
	require.ErrorIs(t, err, ErrPartialWrite)

	var refusal *RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, claudePath, refusal.Path)
	require.ErrorIs(t, refusal.Err, ErrConcurrentEdit)

	info, statErr := os.Stat(featureRoot)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())

	body, readErr := os.ReadFile(claudePath)
	require.NoError(t, readErr)
	assert.Equal(t, "edited after planning", string(body))
}

// Test_apply_refuses_an_older_plugin_file_changed_since_planning pins the
// same read-modify-write guard for an ActionMerged pluginArts entry (an
// OriginOlder agent file, planning-time bytes carried on
// pluginArtifact.existing): the file on disk now holds something else — a
// concurrent edit apply must not silently overwrite — so apply refuses,
// wrapping ErrConcurrentEdit, and the file's own bytes are unchanged
// afterward, proving apply never reached writePluginFile. The control arm
// is the identical fixture with the file still holding exactly what
// planning read: apply proceeds and rewrites it to today's own render,
// proving the guard — not some other check — is what refused above.
func Test_apply_refuses_an_older_plugin_file_changed_since_planning(t *testing.T) {
	wd := t.TempDir()
	plannerPath := filepath.Join(wd, "planner.md")
	staleBytes := []byte("stale planning-time bytes")

	res := Result{Created: []string{}, Modified: []string{}, Removed: []string{}}
	featureArt := Artifact{Kind: KindFeatureRoot, Path: filepath.Join(wd, "docs", "specifications"), Action: ActionUnchanged}
	configArt := Artifact{Kind: KindConfig, Path: filepath.Join(wd, ".brief.yaml"), Action: ActionUnchanged}

	t.Run("changed since planning: refused, untouched", func(t *testing.T) {
		require.NoError(t, os.WriteFile(plannerPath, []byte("edited after planning"), 0o600))

		pluginArts := []pluginArtifact{{
			Kind: KindAgent, Path: plannerPath, Action: ActionMerged, Detail: "updated",
			renderKind: artifact.KindAgentPlanner,
			existing:   staleBytes,
		}}

		_, err := apply(res, featureArt, pluginArts, nil, snippetArtifact{}, false, configArt, nil)

		require.ErrorIs(t, err, ErrConcurrentEdit)

		body, readErr := os.ReadFile(plannerPath)
		require.NoError(t, readErr)
		assert.Equal(t, "edited after planning", string(body))
	})

	t.Run("control: unchanged since planning is replaced", func(t *testing.T) {
		require.NoError(t, os.WriteFile(plannerPath, staleBytes, 0o600))

		pluginArts := []pluginArtifact{{
			Kind: KindAgent, Path: plannerPath, Action: ActionMerged, Detail: "updated",
			renderKind: artifact.KindAgentPlanner,
			existing:   staleBytes,
		}}

		out, err := apply(res, featureArt, pluginArts, nil, snippetArtifact{}, false, configArt, nil)

		require.NoError(t, err)
		assert.Contains(t, out.Modified, plannerPath)

		body, readErr := os.ReadFile(plannerPath)
		require.NoError(t, readErr)
		assert.Equal(t, artifact.AgentPlanner(), body)
	})
}

// Test_apply_refuses_a_bound_agent_changed_since_planning pins the same
// read-modify-write guard for a boundAgentArts entry (an --edit-agents
// merge, planning-time bytes carried on boundAgentArtifact.existing): the
// file on disk now holds something else, so apply refuses, wrapping
// ErrConcurrentEdit, and the file's own bytes are unchanged afterward,
// proving apply never reached writeBoundAgent.
func Test_apply_refuses_a_bound_agent_changed_since_planning(t *testing.T) {
	wd := t.TempDir()
	agentPath := filepath.Join(wd, "developer.md")
	require.NoError(t, os.WriteFile(agentPath, []byte("edited after planning"), 0o600))

	res := Result{Created: []string{}, Modified: []string{}, Removed: []string{}}
	featureArt := Artifact{Kind: KindFeatureRoot, Path: filepath.Join(wd, "docs", "specifications"), Action: ActionUnchanged}
	configArt := Artifact{Kind: KindConfig, Path: filepath.Join(wd, ".brief.yaml"), Action: ActionUnchanged}

	boundAgentArts := []boundAgentArtifact{{
		Kind: KindBoundAgent, Path: agentPath, Action: ActionMerged, Detail: "brief-workflow added to skills",
		existing: []byte("stale planning-time bytes"),
		edited:   []byte("stale planning-time bytes\nskills: [brief-workflow]"),
		line:     "skills: [brief-workflow]",
		perm:     0o600,
	}}

	_, err := apply(res, featureArt, nil, boundAgentArts, snippetArtifact{}, false, configArt, nil)

	require.ErrorIs(t, err, ErrConcurrentEdit)

	body, readErr := os.ReadFile(agentPath)
	require.NoError(t, readErr)
	assert.Equal(t, "edited after planning", string(body))
}

// Test_applyUninstall_refuses_when_CLAUDE_md_changed_since_planning is
// Test_apply_refuses_when_CLAUDE_md_changed_since_planning's own sibling
// for Uninstall's write path.
func Test_applyUninstall_refuses_when_CLAUDE_md_changed_since_planning(t *testing.T) {
	wd := t.TempDir()
	claudePath := filepath.Join(wd, "CLAUDE.md")
	require.NoError(t, os.WriteFile(claudePath, []byte("edited after planning"), 0o600))

	res := Result{
		Artifacts: []Artifact{{Kind: KindSnippet, Path: claudePath, Action: ActionRemoved}},
		Created:   []string{}, Modified: []string{}, Removed: []string{},
	}
	snippetArt := snippetArtifact{
		Kind: KindSnippet, Path: claudePath, Action: ActionRemoved,
		existing: []byte("stale planning-time bytes"),
		remains:  []byte("stale planning-time bytes"),
	}

	_, err := applyUninstall(res, wd, HostNone, snippetArt, true, nil)

	require.ErrorIs(t, err, ErrConcurrentEdit)

	body, readErr := os.ReadFile(claudePath)
	require.NoError(t, readErr)
	assert.Equal(t, "edited after planning", string(body))
}

// Test_apply_uninstall_refuses_a_bound_agent_edited_after_planning pins
// applyUninstall's own read-modify-write guard for a boundAgentArts entry
// (Rule 8's own removal edit, planning-time bytes carried on
// boundAgentArtifact.existing): the file on disk now holds something else,
// so applyUninstall refuses, wrapping ErrConcurrentEdit, and the file's own
// bytes are unchanged afterward, proving applyUninstall never reached
// writeBoundAgent. The control arm is the identical fixture with the file
// still holding exactly what planning read: applyUninstall proceeds and
// rewrites it to the edited bytes.
func Test_apply_uninstall_refuses_a_bound_agent_edited_after_planning(t *testing.T) {
	wd := t.TempDir()
	agentPath := filepath.Join(wd, "developer.md")
	staleBytes := []byte("---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n")
	editedBytes := []byte("---\nname: developer\n---\n\nbody\n")

	boundAgentArts := []boundAgentArtifact{{
		Kind: KindBoundAgent, Path: agentPath, Action: ActionRemoved, Detail: "brief-workflow from skills",
		existing: staleBytes,
		edited:   editedBytes,
		perm:     0o600,
	}}

	t.Run("changed since planning: refused, untouched", func(t *testing.T) {
		require.NoError(t, os.WriteFile(agentPath, []byte("edited after planning"), 0o600))

		res := Result{Created: []string{}, Modified: []string{}, Removed: []string{}}

		_, err := applyUninstall(res, wd, HostNone, snippetArtifact{}, false, boundAgentArts)

		require.ErrorIs(t, err, ErrConcurrentEdit)

		body, readErr := os.ReadFile(agentPath)
		require.NoError(t, readErr)
		assert.Equal(t, "edited after planning", string(body))
	})

	t.Run("control: unchanged since planning is edited", func(t *testing.T) {
		require.NoError(t, os.WriteFile(agentPath, staleBytes, 0o600))

		res := Result{Created: []string{}, Modified: []string{}, Removed: []string{}}

		out, err := applyUninstall(res, wd, HostNone, snippetArtifact{}, false, boundAgentArts)

		require.NoError(t, err)
		assert.Contains(t, out.Modified, agentPath)

		body, readErr := os.ReadFile(agentPath)
		require.NoError(t, readErr)
		assert.Equal(t, editedBytes, body)
	})
}
