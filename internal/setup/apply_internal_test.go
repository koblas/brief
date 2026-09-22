package setup

// White-box package: apply and applyUninstall are unexported and take
// several already-planned artifacts as arguments, letting these tests
// construct a snippetArtifact whose own Action and existing bytes were
// decided from stale (planning-time) file content — proving the
// read-modify-write guard (verifySnippetUnchanged) actually runs inside
// the real write path Init and Uninstall call, not merely as an isolated
// function (see snippet_internal_test.go's own Test_verifySnippetUnchanged
// for that).

import (
	"os"
	"path/filepath"
	"testing"

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

	_, err := apply(res, featureArt, nil, snippetArt, true, configArt, nil)

	require.ErrorIs(t, err, ErrConcurrentEdit)

	body, readErr := os.ReadFile(claudePath)
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

	_, err := applyUninstall(res, wd, HostNone, snippetArt, true)

	require.ErrorIs(t, err, ErrConcurrentEdit)

	body, readErr := os.ReadFile(claudePath)
	require.NoError(t, readErr)
	assert.Equal(t, "edited after planning", string(body))
}
