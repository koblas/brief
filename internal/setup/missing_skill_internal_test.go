package setup

// These tests call agentsMissingSkill and missingSkillReach directly: the
// race between agentfile.Find's read and this package's own re-read cannot
// be forced through srv.Init's real flow.

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_agents_missing_skill_root_resolution_error(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")

	list, err := agentsMissingSkill(filepath.EvalSymlinks, root, "", config.RoleBindings{Implementer: "developer"})

	require.ErrorIs(t, err, fs.ErrNotExist)
	assert.Nil(t, list)
}

// d.Path was already read once by agentfile.Find; a fresh Lstat failure
// here means the file vanished between that read and this one.
func Test_missing_skill_reach_propagates_a_plan_bound_agent_error(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	d := agentfile.Definition{
		Path:  filepath.Join(root, "developer.md"),
		Scope: agentfile.ScopeProject,
	}

	reach, keep, reachErr := missingSkillReach(d, root)

	require.ErrorIs(t, reachErr, fs.ErrNotExist)
	assert.False(t, keep)
	assert.Empty(t, reach)
}

// d is a Definition Find already read as lacking the skill; the file on
// disk now lists it, simulating a change between Find's read and this one.
func Test_missing_skill_reach_drops_a_row_the_file_already_lists(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	path := filepath.Join(root, "developer.md")
	require.NoError(t, os.WriteFile(path, []byte("---\nname: developer\nskills: [brief-workflow]\n---\n\nbody\n"), 0o600))

	d := agentfile.Definition{Path: path, Scope: agentfile.ScopeProject}

	reach, keep, reachErr := missingSkillReach(d, root)

	require.NoError(t, reachErr)
	assert.False(t, keep)
	assert.Empty(t, reach)
}
