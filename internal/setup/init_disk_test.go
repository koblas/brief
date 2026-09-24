package setup_test

// OS-subject: this file pins two things the fsys seam (internal/setup/fs.go's
// diskFS) must not change, neither of which a Mem-backed fixture can stand
// in for.
//
// First: Init's own write path stays unconfined — a ".claude" symlinked to
// a directory outside the repository, itself already holding a "skills/"
// directory, is still followed when Init writes the claude-code plugin
// files into it, exactly as a plain os.MkdirAll + os.OpenRoot(dir) always
// did. This is the counterexample R10's own writability pre-check
// (checkWritable, still real disk, unconfined) does not gate:
// nearestExistingAncestor stops at the first ancestor that exists — here
// "wd/.claude/skills", resolved transparently through the symlink to the
// real, writable "outside/skills" — so a caller confining the write
// adapter at the repository root instead of leaving it unconfined would
// first pass R10 and then fail at apply's own MkdirAll, turning this into
// ErrPartialWrite (the feature root already landed) even though nothing
// about this fixture is a concurrent edit. This can only be driven with a
// real symlink, so it belongs on disk regardless of any future
// test-conversion pass.
//
// Second: a relative wd still resolves against the process's own current
// directory, the way every raw os.* call this seam replaced always did —
// diskFS's own "/"-rooted mapping (fsName) must not silently reinterpret a
// relative wd as rooted at "/" instead. This needs a real process cwd
// (t.Chdir), so it belongs on disk too.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/require"
)

// Test_init_writes_plugin_files_through_a_claude_symlinked_outside_the_repo
// is the disk characterization test fs.go's own diskFS doc comment cites:
// a real, non-DryRun Init succeeds and lands the claude-code plugin
// manifest under the symlink's own target, never refusing on the ancestor
// symlink the way a repository-root-confined write adapter would.
func Test_init_writes_plugin_files_through_a_claude_symlinked_outside_the_repo(t *testing.T) {
	wd := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "skills"), 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(wd, ".claude")))

	srv := setup.NewServer()

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostClaudeCode})
	require.NoError(t, err)

	manifest := filepath.Join(outside, "skills", "brief", ".claude-plugin", "plugin.json")
	_, statErr := os.Stat(manifest)
	require.NoError(t, statErr, "plugin manifest must land under the symlink's own target, not be refused as an escape")
}

// Test_init_resolves_a_relative_wd_against_the_process_cwd pins that a
// relative wd is still resolved against the process's own current
// directory — not reinterpreted as rooted at "/" by diskFS's own fsName
// mapping — exactly as every raw os.* call this seam replaced always did.
func Test_init_resolves_a_relative_wd_against_the_process_cwd(t *testing.T) {
	parent := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(parent, "child"), 0o755))
	t.Chdir(parent)

	srv := setup.NewServer()

	_, err := srv.Init(t.Context(), "child", setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	configPath := filepath.Join(parent, "child", ".brief.yaml")
	_, statErr := os.Stat(configPath)
	require.NoError(t, statErr, "relative wd must resolve against the process cwd, not diskFS's own \"/\" root")
}
