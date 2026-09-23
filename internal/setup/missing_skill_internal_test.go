package setup

// White-box package: agentsMissingSkill's own root-resolution error
// (filepath.EvalSymlinks(root)) and missingSkillReach's own
// planBoundAgent-prologue error propagation are both reachable through
// srv.Init's real flow only by racing the filesystem between
// agentfile.Find's own read (building the Definition LackingSkill filters
// on) and this package's own next read — the same kind of TOCTOU window
// bound_agent_internal_test.go's own header describes for planBoundAgent
// and planBoundAgentRemoval. Driving them directly here, unexported, is
// the only economical way to exercise either.

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

// Test_agents_missing_skill_root_resolution_error pins agentsMissingSkill's
// own root-resolution error: root that does not exist at all fails
// filepath.EvalSymlinks, wrapped and returned rather than treated as "no
// bindings" — nothing meaningful can be classified against a root that
// cannot even be resolved.
func Test_agents_missing_skill_root_resolution_error(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")

	list, err := agentsMissingSkill(root, "", config.RoleBindings{Implementer: "developer"})

	require.ErrorIs(t, err, fs.ErrNotExist)
	assert.Nil(t, list)
}

// Test_missing_skill_reach_propagates_a_plan_bound_agent_error pins
// missingSkillReach's own `return "", err` branch: a ScopeProject
// Definition whose Path planBoundAgent's own os.Lstat prologue fails on —
// here, a path that no longer exists — propagates that error unchanged
// rather than folding it into any MissingSkillReach value. d.Path having
// already been read once by agentfile.Find to build the Definition in the
// first place, a fresh failure here means it changed underneath this run.
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

// Test_missing_skill_reach_drops_a_row_the_file_already_lists pins
// missingSkillReach's own ActionUnchanged branch: d — a Definition
// agentfile.Find already read as lacking the skill, the same shape
// agentsMissingSkill's own LackingSkill filter hands it — whose file a
// fresh planBoundAgent read now finds already lists it (fm.HasSkill true)
// is dropped (keep false) rather than reported ReachFixable: the file
// changed to already carry the skill in the window between Find's own
// read and this call, so suggesting --edit-agents for it would be a dead
// end for a row that no longer lacks anything.
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
