package setup_test

import (
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Init_ignores_an_ancestor_config_outside_the_enclosing_git_repository(t *testing.T) {
	home := fsAbs("home")
	proj := fsAbs("home", "proj")
	mem := newVirtualMem(proj)

	claudeBefore := []byte("# Home notes\n")
	require.NoError(t, mem.WriteFile(memKey(home)+"/.brief.yaml", []byte("feature-directory: home-specs\n"), 0o600))
	require.NoError(t, mem.WriteFile(memKey(home)+"/CLAUDE.md", claudeBefore, 0o600))
	require.NoError(t, mem.Mkdir(memKey(proj)+"/.git", 0o755))

	unrelatedHome := fsAbs("unrelated-home")
	srv := newMemServer(mem, setup.WithHomeDir(func() (string, error) { return unrelatedHome, nil }))

	res, err := srv.Init(t.Context(), proj, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	assert.Equal(t, proj, res.Root)

	projConfig := filepath.Join(proj, ".brief.yaml")
	assert.Contains(t, res.Created, projConfig)

	// init wrote its own CLAUDE.md at proj, never touching the ancestor's.
	projClaude := filepath.Join(proj, "CLAUDE.md")
	assert.Contains(t, res.Created, projClaude)

	snap := mem.Snapshot()

	require.Contains(t, snap, memKey(projConfig))

	require.Contains(t, snap, memKey(home)+"/CLAUDE.md")
	assert.Equal(t, claudeBefore, snap[memKey(home)+"/CLAUDE.md"].Data)

	require.Contains(t, snap, memKey(home)+"/.brief.yaml")
	assert.Equal(t, "feature-directory: home-specs\n", string(snap[memKey(home)+"/.brief.yaml"].Data))

	require.Contains(t, snap, memKey(proj)+"/.claude/skills/brief")
	assert.True(t, snap[memKey(proj)+"/.claude/skills/brief"].Mode.IsDir())

	assert.NotContains(t, snap, memKey(home)+"/.claude", "init must never create the ancestor's own .claude/")
}

func Test_Init_still_adopts_a_config_at_the_enclosing_git_repository_root(t *testing.T) {
	root := fsAbs("repo")
	mem := newVirtualMem(root)
	require.NoError(t, mem.Mkdir(memKey(root)+"/.git", 0o755))
	require.NoError(t, mem.WriteFile(memKey(root)+"/.brief.yaml", []byte("feature-directory: specs\n"), 0o600))
	require.NoError(t, mem.Mkdir(memKey(root)+"/sub", 0o755))
	sub := filepath.Join(root, "sub")

	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), sub, setup.InitRequest{Host: setup.HostNone})

	require.NoError(t, err)
	assert.Equal(t, root, res.Root)

	featureRoot := filepath.Join(root, "specs")
	row := findArtifact(t, res, setup.KindFeatureRoot)
	assert.Equal(t, setup.ActionCreated, row.Action)
	assert.Equal(t, featureRoot, row.Path)
}

func Test_Uninstall_still_adopts_a_config_at_the_enclosing_git_repository_root(t *testing.T) {
	root := fsAbs("repo")
	mem := newVirtualMem(root)
	require.NoError(t, mem.Mkdir(memKey(root)+"/.git", 0o755))

	srv := newMemServer(mem)

	_, err := srv.Init(t.Context(), root, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	require.NoError(t, mem.Mkdir(memKey(root)+"/sub", 0o755))
	sub := filepath.Join(root, "sub")

	res, err := srv.Uninstall(t.Context(), sub, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	assert.Equal(t, root, res.Root)

	configPath := filepath.Join(root, ".brief.yaml")
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionRemoved}, res.Artifacts[0])
	assert.Equal(t, []string{configPath}, res.Removed)

	assert.NotContains(t, mem.Snapshot(), memKey(configPath))
}

func Test_Uninstall_ignores_an_ancestor_config_outside_the_enclosing_git_repository(t *testing.T) {
	home := fsAbs("home")
	proj := fsAbs("home", "proj")
	mem := newVirtualMem(proj)
	homeConfig := filepath.Join(home, ".brief.yaml")
	require.NoError(t, mem.WriteFile(memKey(homeConfig), []byte("feature-directory: home-specs\n"), 0o600))
	require.NoError(t, mem.Mkdir(memKey(proj)+"/.git", 0o755))

	srv := newMemServer(mem)

	res, err := srv.Uninstall(t.Context(), proj, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	assert.Empty(t, res.Artifacts)
	assert.Equal(t, proj, res.Root)

	snap := mem.Snapshot()
	require.Contains(t, snap, memKey(homeConfig))
	assert.Equal(t, "feature-directory: home-specs\n", string(snap[memKey(homeConfig)].Data))
}
