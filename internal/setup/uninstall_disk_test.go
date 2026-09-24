package setup_test

// OS-subject: both cases here inject a real-permission failure (os.Chmod)
// into applyUninstall's own os.Remove call — a Mem fixture has no
// permission model for Remove to fail against (rwfs.Mem.Remove only ever
// fails on fs.ErrNotExist or syscall.ENOTEMPTY), so this can only be driven
// on real disk.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_uninstall_reports_a_remove_failure_without_partial_write pins the
// single-artifact failure path: an unwritable parent directory makes
// os.Remove fail, and because the config is the only artifact this release
// plans, nothing was ever removed before that failure — so the returned
// error does not wrap ErrPartialWrite, and the file survives. Skipped under
// root, which ignores directory write permission.
func Test_uninstall_reports_a_remove_failure_without_partial_write(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	wd := t.TempDir()
	srv := setup.NewServer()
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	require.NoError(t, os.Chmod(wd, 0o555))
	t.Cleanup(func() { _ = os.Chmod(wd, 0o755) })

	_, err = srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostNone})

	require.Error(t, err)
	require.NotErrorIs(t, err, setup.ErrPartialWrite)

	_, statErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
	assert.NoError(t, statErr)
}

// Test_uninstall_reports_the_populated_result_on_a_partial_write pins the
// multi-artifact failure path: the CLAUDE.md block (removedAny's own first
// write) is removed before the agents directory — made unwritable — blocks
// the next removal, so the returned error wraps ErrPartialWrite and the
// returned Result is populated, not the zero value: it still names the
// CLAUDE.md removal that actually landed. Skipped under root, which
// ignores directory write permission.
func Test_uninstall_reports_the_populated_result_on_a_partial_write(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	wd := t.TempDir()
	srv := setup.NewServer()
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	agentsDir := filepath.Join(wd, ".claude", "skills", "brief", "agents")
	require.NoError(t, os.Chmod(agentsDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(agentsDir, 0o755) })

	res, err := srv.Uninstall(t.Context(), wd, setup.UninstallRequest{Host: setup.HostClaudeCode})

	require.ErrorIs(t, err, setup.ErrPartialWrite)

	claudePath := filepath.Join(wd, "CLAUDE.md")
	assert.Contains(t, res.Removed, claudePath)

	_, statErr := os.Stat(claudePath)
	assert.True(t, os.IsNotExist(statErr))
}
