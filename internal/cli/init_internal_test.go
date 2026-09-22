// This file reaches the unexported run directly so host detection (R8) can
// be pinned against an injected, empty home directory: through cli.Run a
// bare "init" would detect against the developer's own "~/.claude", which
// exists for nearly every Claude Code user and would make these tests
// depend on the machine they happen to run on.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emptyHomeSeam returns a runSeam pinning setup.WithHomeDir to a fresh,
// empty t.TempDir() — a home directory that exists but carries no
// ".claude" of its own, so detection never finds anything through it.
func emptyHomeSeam(t *testing.T) runSeam {
	t.Helper()

	home := t.TempDir()

	return withSetupOpts(setup.WithHomeDir(func() (string, error) { return home, nil }))
}

// Test_init_without_host_detects_the_host_from_the_tree pins R8's
// detection rule at the cli boundary: nothing present resolves to
// HostNone with the R8 stderr line replacing the next action; a root
// "CLAUDE.md" resolves to claude-code and installs the plugin; under
// --json a detected-none run leaves stderr empty and reports "host":"none".
func Test_init_without_host_detects_the_host_from_the_tree(t *testing.T) {
	t.Run("nothing present detects none", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Equal(t, "created .brief.yaml\ncreated docs/specifications/\n", stdout.String())
		assert.Equal(t, "brief init: no agent host detected; run 'brief init --host claude-code' to install integration\n", stderr.String())
	})

	t.Run("a root CLAUDE.md detects claude-code", func(t *testing.T) {
		wd := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), []byte("# hi\n"), 0o600))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "created .claude/skills/brief/.claude-plugin/plugin.json\n")
		assert.Equal(t, "brief init: installed for claude-code; start Claude Code in this directory (or run /reload-plugins in a session already here), then 'brief new feature <name>'\n", stderr.String())
	})

	t.Run("detected none under --json leaves stderr empty", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--json"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Empty(t, stderr.String())
		assert.Contains(t, stdout.String(), `"host":"none"`)
	})
}

// Test_init_with_agents_follows_the_detected_host pins --with-agents
// against a detected (rather than explicit) host: nothing detected still
// refuses ErrAgentsNeedHost's own usage error with the tree left empty; a
// detected claude-code root installs the three agents.
func Test_init_with_agents_follows_the_detected_host(t *testing.T) {
	t.Run("nothing detected refuses", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--with-agents"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.Error(t, err)
		assert.Equal(t, 2, ExitCode(err))
		assert.Equal(t, "brief init: --with-agents requires --host claude-code; run 'brief init --host claude-code --with-agents'\n", stderr.String())

		entries, readErr := os.ReadDir(wd)
		require.NoError(t, readErr)
		assert.Empty(t, entries)
	})

	t.Run("a detected claude-code root installs the agents", func(t *testing.T) {
		wd := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(wd, ".claude"), 0o755))
		var stdout, stderr bytes.Buffer

		err := run(t.Context(), wd, []string{"init", "--with-agents"}, nil, &stdout, &stderr, noBuildInfo, emptyHomeSeam(t))

		require.NoError(t, err)
		assert.Contains(t, stdout.String(), "created .claude/skills/brief/agents/planner.md\n")
	})
}
