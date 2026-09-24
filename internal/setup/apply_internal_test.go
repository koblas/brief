package setup

// White-box package: apply and applyUninstall are unexported and take
// several already-planned artifacts as arguments, letting these tests
// construct a snippetArtifact or pluginArtifact whose own Action and
// existing bytes were decided from stale (planning-time) file content —
// proving the read-modify-write guard (verifyFileUnchanged) actually runs
// inside the real write path Init and Uninstall call, not merely as an
// isolated function (see snippet_internal_test.go's own
// Test_verifyFileUnchanged for that). The bound-agent guard's own twin
// tests are apply_bound_agent_internal_test.go — a boundAgentArtifact
// always reads and writes through bound_agent.go's own real disk,
// regardless of the fsys this file's own cases inject.

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_apply_refuses_when_CLAUDE_md_changed_since_planning pins the guard
// at Init's own write path: snippetArt.existing carries bytes planning
// read at some earlier point; mem now holds something else (an edit by
// another process, in this test's stand-in for that race). apply refuses,
// wrapping ErrConcurrentEdit, and — the control this claim needs — the
// bytes are unchanged afterward, proving apply never reached
// writeSnippetFile.
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

// Test_apply_wraps_ErrPartialWrite_when_an_earlier_write_already_landed
// pins apply's own ordering guarantee at the write path directly: unlike
// the test above, featureArt is ActionCreated here, so its own MkdirAll
// lands before the concurrent-edit guard ever runs. The returned error
// must wrap both ErrConcurrentEdit (what went wrong) and ErrPartialWrite
// (that something already changed) — cli's own files_changed and
// refusal-tail rendering both key off ErrPartialWrite being reachable
// here, not merely off the sentinel this markPartial call itself wraps.
// It must also still carry a *RefusalError through markPartial's
// wrapping: cli's classifyRefusal (internal/cli/refusal.go) branches on
// errors.AsType[*RefusalError], not on the sentinels alone, so a
// markPartial that erased the type would silently fall through to the
// generic errorKindFailure case even though every errors.Is assertion
// here still holds.
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

// Test_apply_refuses_an_older_plugin_file_changed_since_planning pins the
// same read-modify-write guard for an ActionMerged pluginArts entry (an
// OriginOlder agent file, planning-time bytes carried on
// pluginArtifact.existing): mem now holds something else — a concurrent
// edit apply must not silently overwrite — so apply refuses, wrapping
// ErrConcurrentEdit, and the bytes are unchanged afterward, proving apply
// never reached writePluginFile. The control arm is the identical fixture
// with mem still holding exactly what planning read: apply proceeds and
// rewrites it to today's own render, proving the guard — not some other
// check — is what refused above.
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

// Test_applyUninstall_refuses_when_CLAUDE_md_changed_since_planning is
// Test_apply_refuses_when_CLAUDE_md_changed_since_planning's own sibling
// for Uninstall's write path.
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

// Test_applyUninstall_skips_a_KindBoundAgent_row_in_its_own_removal_loop
// pins applyUninstall's own removal loop guard: a hand-built
// KindBoundAgent ActionRemoved row in res.Artifacts, with boundAgentArts
// itself empty, must never be reached by the generic removal loop below —
// only the earlier boundAgentArts loop (Rule 8) ever rewrites a bound
// agent file, in place, never by deleting it — so P survives untouched. A
// sibling KindPlugin ActionRemoved row (Q) is the control, proving the
// loop still removes everything it is supposed to.
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
