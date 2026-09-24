// OS-subject: every case in this file removes a real bound agent file (a
// repository ".claude/agents/*.md" file --edit-agents merged, or resolved
// through a bare-name role binding) — internal/setup's own bound_agent.go
// (confinedAgentFile) always reads and writes those files through real
// disk regardless of setup.WithFSRoot, so no rwfs.Mem fixture in
// uninstall_internal_test.go can stand in for one (the same reason
// internal/cli/init_bound_agent_internal_test.go stays on disk).
//
// White-box package: this file reaches the unexported run directly so a
// bound-agent role can be resolved against an injected home directory —
// through cli.Run a bare "developer" binding would also search the
// developer's own "~/.claude/agents", making these tests depend on the
// machine they happen to run on.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// boundAgentUninstallFixture installs a claude-code plugin via "init" — the
// snippet, skill and plugin files this file's own row and stderr
// assertions rely on being present — with an injected, empty home
// directory, then rebinds ".brief.yaml" to a bare-name implementer and
// writes one bound agent file already carrying "brief-workflow".
func boundAgentUninstallFixture(t *testing.T) (string, runSeam, string) {
	t.Helper()

	wd := t.TempDir()
	home := t.TempDir()
	seam := withSetupOpts(setup.WithHomeDir(func() (string, error) { return home, nil }))

	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
		"feature-directory: docs/specifications\nroles:\n  implementer: developer\n",
	), 0o600))

	agentPath := filepath.Join(wd, ".claude", "agents", "developer.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(agentPath), 0o755))
	require.NoError(t, os.WriteFile(agentPath, []byte("---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n"), 0o600))

	return wd, seam, agentPath
}

// Test_uninstall_reports_the_bound_agent_row pins Surface & Copy's own
// stdout row, its stderr next-action line, --json's own modified/removed
// split, and --dry-run leaving the file untouched.
func Test_uninstall_reports_the_bound_agent_row(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		wd, seam, agentPath := boundAgentUninstallFixture(t)
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"uninstall", "--host", "claude-code"}, nil, &stdout, &stderr, noBuildInfo, seam)
		require.NoError(t, err)

		assert.Contains(t, stdout.String(), "removed .claude/agents/developer.md (brief-workflow from skills)\n")
		assert.Equal(t, "brief uninstall: removed brief's claude-code install; the feature root and its contents were left in place\n", stderr.String())

		after, readErr := os.ReadFile(agentPath)
		require.NoError(t, readErr)
		assert.Equal(t, "---\nname: developer\n---\n\nbody\n", string(after))
	})

	t.Run("json", func(t *testing.T) {
		wd, seam, agentPath := boundAgentUninstallFixture(t)
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"uninstall", "--host", "claude-code", "--json"}, nil, &stdout, &stderr, noBuildInfo, seam)
		require.NoError(t, err)
		assert.Empty(t, stderr.String())

		var doc struct {
			Modified  []string `json:"modified"`
			Removed   []string `json:"removed"`
			Artifacts []struct {
				Kind   string `json:"kind"`
				Path   string `json:"path"`
				Action string `json:"action"`
				Detail string `json:"detail"`
			} `json:"artifacts"`
		}
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))

		assert.Contains(t, doc.Modified, agentPath)
		assert.NotContains(t, doc.Removed, agentPath)

		var found bool

		for _, a := range doc.Artifacts {
			if a.Path == agentPath {
				found = true

				assert.Equal(t, "bound-agent", a.Kind)
				assert.Equal(t, "removed", a.Action)
				assert.Equal(t, "brief-workflow from skills", a.Detail)
			}
		}

		assert.True(t, found)
	})

	t.Run("dry run", func(t *testing.T) {
		wd, seam, agentPath := boundAgentUninstallFixture(t)
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"uninstall", "--host", "claude-code", "--dry-run"}, nil, &stdout, &stderr, noBuildInfo, seam)
		require.NoError(t, err)

		assert.Contains(t, stdout.String(), "removed .claude/agents/developer.md (brief-workflow from skills)\n")

		after, readErr := os.ReadFile(agentPath)
		require.NoError(t, readErr)
		assert.Equal(t, "---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n", string(after))
	})
}
