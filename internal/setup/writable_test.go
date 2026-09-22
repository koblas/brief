package setup_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_init_refuses_an_unwritable_target_before_writing_anything pins R10:
// a target whose nearest existing ancestor is not a directory, or is a
// directory that cannot be written to, refuses before any write — a
// *setup.RefusalError wrapping ErrUnwritable, naming that ancestor, with
// Result.Print populated alongside the error — and the tree stays
// byte-identical. The chmod case is skipped under root, which ignores
// directory write permission.
func Test_init_refuses_an_unwritable_target_before_writing_anything(t *testing.T) {
	t.Run("an existing regular file blocks a directory ancestor", func(t *testing.T) {
		wd := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(wd, ".claude"), 0o755))
		blocker := filepath.Join(wd, ".claude", "skills")
		require.NoError(t, os.WriteFile(blocker, []byte("not a directory"), 0o600))
		before := snapshotTree(t, wd)
		srv := newServer(t)

		res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

		require.ErrorIs(t, err, setup.ErrUnwritable)
		refusal, ok := errors.AsType[*setup.RefusalError](err)
		require.True(t, ok)
		assert.Equal(t, blocker, refusal.Path)
		assert.Equal(t, "not a directory", refusal.Problem)
		assert.Equal(t, "apply the output below by hand", refusal.Fix)
		assert.NotEmpty(t, res.Print)

		assert.Equal(t, before, snapshotTree(t, wd))

		_, statErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("an existing directory that cannot be written to", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root ignores directory write permission")
		}

		wd := t.TempDir()
		blocker := filepath.Join(wd, ".claude")
		require.NoError(t, os.Mkdir(blocker, 0o500))
		t.Cleanup(func() { _ = os.Chmod(blocker, 0o755) })
		before := snapshotTree(t, wd)
		srv := newServer(t)

		_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

		require.ErrorIs(t, err, setup.ErrUnwritable)
		refusal, ok := errors.AsType[*setup.RefusalError](err)
		require.True(t, ok)
		assert.Equal(t, blocker, refusal.Path)
		assert.Equal(t, "not writable", refusal.Problem)

		assert.Equal(t, before, snapshotTree(t, wd))
	})
}

// Test_the_writability_probe_never_runs_under_dry_run_or_print is the
// control arm for the two refusal cases above: the identical portable
// fixture (a regular file blocking ".claude/skills") never refuses under
// DryRun or Print — the probe only ever runs when Init is about to write —
// proving the refusal above comes from the pre-write check and not from
// some other, always-on validation.
func Test_the_writability_probe_never_runs_under_dry_run_or_print(t *testing.T) {
	cases := []struct {
		name string
		req  setup.InitRequest
	}{
		{name: "dry run", req: setup.InitRequest{Host: setup.HostClaudeCode, DryRun: true}},
		{name: "print", req: setup.InitRequest{Host: setup.HostClaudeCode, Print: true}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(wd, ".claude"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude", "skills"), []byte("not a directory"), 0o600))
			srv := newServer(t)

			_, err := srv.Init(t.Context(), wd, c.req)

			require.NoError(t, err)
			require.NotErrorIs(t, err, setup.ErrUnwritable)
		})
	}
}
