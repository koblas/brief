package setup

// These tests construct a snippetArtifact or pluginArtifact whose Action and
// existing bytes were decided from stale planning-time content, to prove
// the read-modify-write guard runs inside apply/applyUninstall's real write
// path, not merely as the isolated function snippet_internal_test.go pins.

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_apply_refuses_when_CLAUDE_md_changed_since_planning(t *testing.T) {
	claudePath := "/repo/CLAUDE.md"
	featureRoot := "/repo/docs/specifications"

	mem := rwfs.NewMem(fstest.MapFS{
		"repo/CLAUDE.md":           &fstest.MapFile{Data: []byte("edited after planning")},
		"repo/docs/specifications": &fstest.MapFile{Mode: fs.ModeDir | 0o755},
	})

	res := Result{Created: []string{}, Modified: []string{}, Removed: []string{}}
	featureArt := Artifact{Kind: KindFeatureRoot, Path: featureRoot, Action: ActionUnchanged}
	snippetArt := snippetArtifact{
		Kind: KindSnippet, Path: claudePath, Action: ActionMerged,
		existing: []byte("stale planning-time bytes"),
		dir:      "docs/specifications",
	}
	configArt := Artifact{Kind: KindConfig, Path: "/repo/.brief.yaml", Action: ActionUnchanged}

	_, err := apply(mem, res, featureArt, nil, nil, snippetArt, true, configArt, nil)

	require.ErrorIs(t, err, ErrConcurrentEdit)

	assert.Equal(t, "edited after planning", string(mem.Snapshot()["repo/CLAUDE.md"].Data))
}

// Unlike the test above, featureArt is ActionCreated here, so its MkdirAll
// lands before the concurrent-edit guard runs; the error must still wrap a
// *RefusalError, since cli's classifyRefusal branches on that type.
func Test_apply_wraps_ErrPartialWrite_when_an_earlier_write_already_landed(t *testing.T) {
	claudePath := "/repo/CLAUDE.md"
	featureRoot := "/repo/docs/specifications"

	mem := rwfs.NewMem(fstest.MapFS{
		"repo/CLAUDE.md": &fstest.MapFile{Data: []byte("edited after planning")},
	})

	res := Result{Created: []string{}, Modified: []string{}, Removed: []string{}}
	featureArt := Artifact{Kind: KindFeatureRoot, Path: featureRoot, Action: ActionCreated}
	snippetArt := snippetArtifact{
		Kind: KindSnippet, Path: claudePath, Action: ActionMerged,
		existing: []byte("stale planning-time bytes"),
		dir:      "docs/specifications",
	}
	configArt := Artifact{Kind: KindConfig, Path: "/repo/.brief.yaml", Action: ActionUnchanged}

	_, err := apply(mem, res, featureArt, nil, nil, snippetArt, true, configArt, nil)

	require.ErrorIs(t, err, ErrConcurrentEdit)
	require.ErrorIs(t, err, ErrPartialWrite)

	var refusal *RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, claudePath, refusal.Path)
	require.ErrorIs(t, refusal.Err, ErrConcurrentEdit)

	snap := mem.Snapshot()
	info, ok := snap["repo/docs/specifications"]
	require.True(t, ok)
	assert.True(t, info.Mode.IsDir())

	assert.Equal(t, "edited after planning", string(snap["repo/CLAUDE.md"].Data))
}

func Test_apply_refuses_an_older_plugin_file_changed_since_planning(t *testing.T) {
	plannerPath := "/repo/planner.md"
	staleBytes := []byte("stale planning-time bytes")

	res := Result{Created: []string{}, Modified: []string{}, Removed: []string{}}
	featureArt := Artifact{Kind: KindFeatureRoot, Path: "/repo/docs/specifications", Action: ActionUnchanged}
	configArt := Artifact{Kind: KindConfig, Path: "/repo/.brief.yaml", Action: ActionUnchanged}

	t.Run("changed since planning: refused, untouched", func(t *testing.T) {
		mem := rwfs.NewMem(fstest.MapFS{
			"repo/planner.md": &fstest.MapFile{Data: []byte("edited after planning")},
		})

		pluginArts := []pluginArtifact{{
			Kind: KindAgent, Path: plannerPath, Action: ActionMerged, Detail: "updated",
			renderKind: artifact.KindAgentPlanner,
			existing:   staleBytes,
		}}

		_, err := apply(mem, res, featureArt, pluginArts, nil, snippetArtifact{}, false, configArt, nil)

		require.ErrorIs(t, err, ErrConcurrentEdit)

		assert.Equal(t, "edited after planning", string(mem.Snapshot()["repo/planner.md"].Data))
	})

	t.Run("control: unchanged since planning is replaced", func(t *testing.T) {
		mem := rwfs.NewMem(fstest.MapFS{
			"repo/planner.md": &fstest.MapFile{Data: staleBytes},
		})

		pluginArts := []pluginArtifact{{
			Kind: KindAgent, Path: plannerPath, Action: ActionMerged, Detail: "updated",
			renderKind: artifact.KindAgentPlanner,
			existing:   staleBytes,
		}}

		out, err := apply(mem, res, featureArt, pluginArts, nil, snippetArtifact{}, false, configArt, nil)

		require.NoError(t, err)
		assert.Contains(t, out.Modified, plannerPath)

		assert.Equal(t, artifact.AgentPlanner(), mem.Snapshot()["repo/planner.md"].Data)
	})
}

func Test_applyUninstall_refuses_when_CLAUDE_md_changed_since_planning(t *testing.T) {
	claudePath := "/repo/CLAUDE.md"

	mem := rwfs.NewMem(fstest.MapFS{
		"repo/CLAUDE.md": &fstest.MapFile{Data: []byte("edited after planning")},
	})

	res := Result{
		Artifacts: []Artifact{{Kind: KindSnippet, Path: claudePath, Action: ActionRemoved}},
		Created:   []string{}, Modified: []string{}, Removed: []string{},
	}
	snippetArt := snippetArtifact{
		Kind: KindSnippet, Path: claudePath, Action: ActionRemoved,
		existing: []byte("stale planning-time bytes"),
		remains:  []byte("stale planning-time bytes"),
	}

	_, err := applyUninstall(mem, res, "/repo", HostNone, snippetArt, true, nil)

	require.ErrorIs(t, err, ErrConcurrentEdit)

	assert.Equal(t, "edited after planning", string(mem.Snapshot()["repo/CLAUDE.md"].Data))
}

// boundAgentArts is deliberately empty, so the KindBoundAgent row must not
// be reached by the generic removal loop; the sibling KindPlugin row is the
// control, proving the loop still removes what it should.
func Test_applyUninstall_skips_a_KindBoundAgent_row_in_its_own_removal_loop(t *testing.T) {
	boundAgentPath := "/repo/bound-agent.md"
	pluginPath := "/repo/plugin.md"

	mem := rwfs.NewMem(fstest.MapFS{
		"repo/bound-agent.md": &fstest.MapFile{Data: []byte("do not touch me")},
		"repo/plugin.md":      &fstest.MapFile{Data: []byte("remove me")},
	})

	res := Result{
		Artifacts: []Artifact{
			{Kind: KindBoundAgent, Path: boundAgentPath, Action: ActionRemoved},
			{Kind: KindPlugin, Path: pluginPath, Action: ActionRemoved},
		},
		Created: []string{}, Modified: []string{}, Removed: []string{},
	}

	out, err := applyUninstall(mem, res, "/repo", HostNone, snippetArtifact{}, false, nil)

	require.NoError(t, err)

	snap := mem.Snapshot()
	assert.Contains(t, snap, "repo/bound-agent.md", "a KindBoundAgent row must never be deleted by the generic removal loop")
	assert.NotContains(t, snap, "repo/plugin.md")
	assert.NotContains(t, out.Removed, boundAgentPath)
	assert.Contains(t, out.Removed, pluginPath)
}
