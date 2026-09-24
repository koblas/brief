package setup

// OS-subject, white-box: a boundAgentArtifact's own agentFile() always
// opens a real os.Root at resolvedRoot, confined to rel, regardless of the
// fsys apply/applyUninstall themselves were given (bound_agent.go's own
// confinedAgentFile) — see doc.go and fs.go's own doc comments. Both cases
// here need a real resolvedRoot to open.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_apply_refuses_a_bound_agent_changed_since_planning pins the same
// read-modify-write guard for a boundAgentArts entry (an --edit-agents
// merge, planning-time bytes carried on boundAgentArtifact.existing): the
// file on disk now holds something else, so apply refuses, wrapping
// ErrConcurrentEdit, and the file's own bytes are unchanged afterward,
// proving apply never reached ba.agentFile().write.
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

	_, err := apply(diskFS{}, res, featureArt, nil, boundAgentArts, snippetArtifact{}, false, configArt, nil)

	require.ErrorIs(t, err, ErrConcurrentEdit)

	body, readErr := os.ReadFile(agentPath)
	require.NoError(t, readErr)
	assert.Equal(t, "edited after planning", string(body))
}

// Test_apply_uninstall_refuses_a_bound_agent_edited_after_planning pins
// applyUninstall's own read-modify-write guard for a boundAgentArts entry
// (Rule 8's own removal edit, planning-time bytes carried on
// boundAgentArtifact.existing): the file on disk now holds something else,
// so applyUninstall refuses, wrapping ErrConcurrentEdit, and the file's own
// bytes are unchanged afterward, proving applyUninstall never reached
// ba.agentFile().write. The control arm is the identical fixture with the file
// still holding exactly what planning read: applyUninstall proceeds and
// rewrites it to the edited bytes.
func Test_apply_uninstall_refuses_a_bound_agent_edited_after_planning(t *testing.T) {
	wd := t.TempDir()
	agentPath := filepath.Join(wd, "developer.md")
	staleBytes := []byte("---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n")
	editedBytes := []byte("---\nname: developer\n---\n\nbody\n")

	boundAgentArts := []boundAgentArtifact{{
		Kind: KindBoundAgent, Path: agentPath, Action: ActionRemoved, Detail: "brief-workflow from skills",
		existing:     staleBytes,
		edited:       editedBytes,
		perm:         0o600,
		resolvedRoot: wd,
		rel:          "developer.md",
	}}

	t.Run("changed since planning: refused, untouched", func(t *testing.T) {
		require.NoError(t, os.WriteFile(agentPath, []byte("edited after planning"), 0o600))

		res := Result{Created: []string{}, Modified: []string{}, Removed: []string{}}

		_, err := applyUninstall(diskFS{}, res, wd, HostNone, snippetArtifact{}, false, boundAgentArts)

		require.ErrorIs(t, err, ErrConcurrentEdit)

		body, readErr := os.ReadFile(agentPath)
		require.NoError(t, readErr)
		assert.Equal(t, "edited after planning", string(body))
	})

	t.Run("control: unchanged since planning is edited", func(t *testing.T) {
		require.NoError(t, os.WriteFile(agentPath, staleBytes, 0o600))

		res := Result{Created: []string{}, Modified: []string{}, Removed: []string{}}

		out, err := applyUninstall(diskFS{}, res, wd, HostNone, snippetArtifact{}, false, boundAgentArts)

		require.NoError(t, err)
		assert.Contains(t, out.Modified, agentPath)

		body, readErr := os.ReadFile(agentPath)
		require.NoError(t, readErr)
		assert.Equal(t, editedBytes, body)
	})
}
