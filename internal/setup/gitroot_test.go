package setup_test

import (
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_Init_ignores_an_ancestor_config_outside_the_enclosing_git_repository
// pins the boundary rule: an ancestor ".brief.yaml" (and CLAUDE.md) sitting
// above the nearest enclosing git repository — a HOME-level config, say —
// is never adopted. Init writes a fresh config at wd instead of merging
// into the ancestor's own CLAUDE.md or installing the plugin under the
// ancestor's own ".claude/". The ancestor CLAUDE.md is proven
// byte-identical afterward, which is vacuous unless the same run also
// proves it wrote *something* — the fresh config and plugin at wd. The
// ancestor walk (repo.RootFS, config.LocateWithinFS) only ever stats a
// ".git" or ".brief.yaml" candidate, so this is Mem-backed; proj alone
// stays a real t.TempDir() for checkWritable (R10, unconverted).
func Test_Init_ignores_an_ancestor_config_outside_the_enclosing_git_repository(t *testing.T) {
	proj, mem := newRealRootMem(t)
	home := filepath.Dir(proj)

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

// Test_Init_still_adopts_a_config_at_the_enclosing_git_repository_root is
// the control for the case above: a config sitting exactly at the nearest
// enclosing git repository root — not above it — is still adopted, even
// though wd is a subdirectory holding neither a config nor a ".git" of its
// own.
func Test_Init_still_adopts_a_config_at_the_enclosing_git_repository_root(t *testing.T) {
	root, mem := newRealRootMem(t)
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

// Test_Uninstall_still_adopts_a_config_at_the_enclosing_git_repository_root
// is Test_Init_still_adopts_a_config_at_the_enclosing_git_repository_root's
// own sibling for Uninstall: a config sitting exactly at the nearest
// enclosing git repository root is still adopted and removed, even though
// wd is a subdirectory holding neither a config nor a ".git" of its own.
func Test_Uninstall_still_adopts_a_config_at_the_enclosing_git_repository_root(t *testing.T) {
	root, mem := newRealRootMem(t)
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

// Test_Uninstall_ignores_an_ancestor_config_outside_the_enclosing_git_repository
// is Test_Init_ignores_an_ancestor_config_outside_the_enclosing_git_repository's
// own sibling for Uninstall: an ancestor config above the enclosing git
// repository is never removed, and Uninstall reports nothing installed
// rather than reaching outside the repository.
func Test_Uninstall_ignores_an_ancestor_config_outside_the_enclosing_git_repository(t *testing.T) {
	proj, mem := newRealRootMem(t)
	home := filepath.Dir(proj)
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
