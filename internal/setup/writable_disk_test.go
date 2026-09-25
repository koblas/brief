package setup_test

// checkWritable's pre-write check runs against real disk, never through the
// fsRoot seam; every case here needs a real, unwritable or non-directory
// ancestor.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// treeEntry is one snapshotTree entry: isDir alone for a directory, body
// for a regular file's exact bytes.
type treeEntry struct {
	isDir bool
	body  []byte
}

// snapshotTree walks every path under root (root excluded), keyed by its
// relative path, recording whether it is a directory or a file's own bytes.
func snapshotTree(t *testing.T, root string) map[string]treeEntry {
	t.Helper()

	r, err := os.OpenRoot(root)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()

	fsys := r.FS()
	out := map[string]treeEntry{}

	walkErr := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, entryErr error) error {
		require.NoError(t, entryErr)

		if p == "." {
			return nil
		}

		if d.IsDir() {
			out[p] = treeEntry{isDir: true}

			return nil
		}

		body, readErr := fs.ReadFile(fsys, p)
		require.NoError(t, readErr)

		out[p] = treeEntry{body: body}

		return nil
	})
	require.NoError(t, walkErr)

	return out
}

func Test_an_unwritable_plugin_directory_refuses_before_the_feature_root_is_created(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	wd := t.TempDir()
	pluginDir := filepath.Join(wd, ".claude", "skills", "brief")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.Chmod(pluginDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(pluginDir, 0o755) })
	srv := newServer(t)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})

	require.ErrorIs(t, err, setup.ErrUnwritable)
	require.NotErrorIs(t, err, setup.ErrPartialWrite)

	refusal, ok := errors.AsType[*setup.RefusalError](err)
	require.True(t, ok)
	assert.Equal(t, pluginDir, refusal.Path)

	_, statErr := os.Stat(filepath.Join(wd, "docs", "specifications"))
	assert.True(t, os.IsNotExist(statErr))

	_, statErr = os.Stat(filepath.Join(wd, ".brief.yaml"))
	assert.True(t, os.IsNotExist(statErr))
}

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

func Test_init_refuses_when_an_older_agent_file_is_unwritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}

	wd := t.TempDir()
	srv := setup.NewServer()
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})
	require.NoError(t, err)

	agentsDir := filepath.Join(wd, ".claude", "skills", "brief", "agents")
	plannerPath := filepath.Join(agentsDir, "planner.md")
	older := []byte("---\nname: planner\ndescription: Turn a feature's specification into ordered scenario " +
		"plans.\ntools: Read, Grep, Glob, Bash, Edit, Write\n---\n\nTurn the feature's " +
		"specification into ordered scenario plans: run `brief new step <feature>` for the next " +
		"scenario, then fill its plan file. Never write production or test code.\n")
	require.NoError(t, os.WriteFile(plannerPath, older, 0o600))

	require.NoError(t, os.Chmod(agentsDir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(agentsDir, 0o755) })

	_, err = srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode, WithAgents: true})

	require.ErrorIs(t, err, setup.ErrUnwritable)
	refusal, ok := errors.AsType[*setup.RefusalError](err)
	require.True(t, ok)
	assert.Equal(t, agentsDir, refusal.Path)

	body, readErr := os.ReadFile(plannerPath)
	require.NoError(t, readErr)
	assert.Equal(t, older, body)
}

// This is the control arm for the refusal cases above: the identical
// fixture must not refuse under DryRun or Print.
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
		})
	}
}
