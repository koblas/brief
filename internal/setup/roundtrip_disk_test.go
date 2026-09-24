package setup_test

// OS-subject: --edit-agents binds a bare-name planner or implementer role
// to a project agent file, which reaches bound_agent.go's own real
// os.Lstat/os.ReadFile and confinedAgentFile regardless of the fsRoot seam
// (see doc.go) — a Mem fixture would silently resolve against nothing.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readFileT reads path, failing the test on any error — a small helper
// this file's own round-trip snapshots use for readability.
func readFileT(t *testing.T, path string) []byte {
	t.Helper()

	body, err := os.ReadFile(path)
	require.NoError(t, err)

	return body
}

// Test_init_edit_agents_then_uninstall_leaves_bound_agents_byte_identical
// pins R16's own round trip across every shape it holds for: a bare-name
// role bound to an agent carrying no top-level "skills:" key, one bound to
// an agent carrying a non-empty block list, and one bound to an agent
// already carrying a canonical single-entry flow list — Init --edit-agents
// then Uninstall reproduces each one exactly byte-identical. The control
// (afterEdit != before) proves the edit actually landed before the round
// trip claims to undo it.
func Test_init_edit_agents_then_uninstall_leaves_bound_agents_byte_identical(t *testing.T) {
	roundTrip := func(t *testing.T, planner, plannerBody, implementer, implementerBody string) {
		t.Helper()

		wd := t.TempDir()
		home := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
			"feature-directory: docs/specifications\nroles:\n"+
				"  planner: "+planner+"\n  implementer: "+implementer+"\n",
		), 0o600))

		plannerPath := filepath.Join(wd, ".claude", "agents", planner+".md")
		implementerPath := filepath.Join(wd, ".claude", "agents", implementer+".md")

		require.NoError(t, os.MkdirAll(filepath.Dir(plannerPath), 0o755))
		require.NoError(t, os.WriteFile(plannerPath, []byte(plannerBody), 0o600))
		require.NoError(t, os.WriteFile(implementerPath, []byte(implementerBody), 0o600))

		before := map[string][]byte{plannerPath: []byte(plannerBody), implementerPath: []byte(implementerBody)}

		srv := setup.NewServer(setup.WithHomeDir(func() (string, error) { return home, nil }))
		_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, EditAgents: true})
		require.NoError(t, err)

		afterEdit := map[string][]byte{plannerPath: readFileT(t, plannerPath), implementerPath: readFileT(t, implementerPath)}
		assert.NotEqual(t, before, afterEdit, "init --edit-agents must actually have changed both files")

		_, err = srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})
		require.NoError(t, err)

		afterUninstall := map[string][]byte{plannerPath: readFileT(t, plannerPath), implementerPath: readFileT(t, implementerPath)}
		assert.Equal(t, before, afterUninstall)
	}

	t.Run("no key and a non-empty block list", func(t *testing.T) {
		roundTrip(t,
			"no-key-agent", "---\nname: no-key-agent\n---\n\nbody\n",
			"block-agent", "---\nname: block-agent\nskills:\n  - other\n---\n\nbody\n",
		)
	})

	t.Run("canonical flow list", func(t *testing.T) {
		roundTrip(t,
			"flow-agent", "---\nname: flow-agent\nskills: [a]\n---\n\nbody\n",
			"no-key-agent-2", "---\nname: no-key-agent-2\n---\n\nbody\n",
		)
	})
}
