package setup_test

import (
	"os"
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
// proves it wrote *something* — the fresh config and plugin at wd.
func Test_Init_ignores_an_ancestor_config_outside_the_enclosing_git_repository(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(home, ".brief.yaml"), []byte("feature-directory: home-specs\n"), 0o600))
	claudeBefore := []byte("# Home notes\n")
	require.NoError(t, os.WriteFile(filepath.Join(home, "CLAUDE.md"), claudeBefore, 0o600))

	proj := filepath.Join(home, "proj")
	require.NoError(t, os.MkdirAll(filepath.Join(proj, ".git"), 0o755))

	unrelatedHome := t.TempDir()
	srv := setup.NewServer(setup.WithHomeDir(func() (string, error) { return unrelatedHome, nil }))

	res, err := srv.Init(t.Context(), proj, setup.InitRequest{Host: setup.HostClaudeCode})

	require.NoError(t, err)
	assert.Equal(t, proj, res.Root)

	projConfig := filepath.Join(proj, ".brief.yaml")
	assert.Contains(t, res.Created, projConfig)
	assert.FileExists(t, projConfig)

	// init wrote its own CLAUDE.md at proj, never touching the ancestor's.
	projClaude := filepath.Join(proj, "CLAUDE.md")
	assert.Contains(t, res.Created, projClaude)

	claudeAfter, readErr := os.ReadFile(filepath.Join(home, "CLAUDE.md"))
	require.NoError(t, readErr)
	assert.Equal(t, claudeBefore, claudeAfter)

	homeConfigAfter, readErr := os.ReadFile(filepath.Join(home, ".brief.yaml"))
	require.NoError(t, readErr)
	assert.Equal(t, "feature-directory: home-specs\n", string(homeConfigAfter))

	assert.DirExists(t, filepath.Join(proj, ".claude", "skills", "brief"))

	_, statErr := os.Stat(filepath.Join(home, ".claude"))
	assert.True(t, os.IsNotExist(statErr), "init must never create the ancestor's own .claude/")
}

// Test_Init_still_adopts_a_config_at_the_enclosing_git_repository_root is
// the control for the case above: a config sitting exactly at the nearest
// enclosing git repository root — not above it — is still adopted, even
// though wd is a subdirectory holding neither a config nor a ".git" of its
// own.
func Test_Init_still_adopts_a_config_at_the_enclosing_git_repository_root(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".brief.yaml"), []byte("feature-directory: specs\n"), 0o600))
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	srv := newServer(t)

	res, err := srv.Init(t.Context(), sub, setup.InitRequest{Host: setup.HostNone})

	require.NoError(t, err)
	assert.Equal(t, root, res.Root)

	featureRoot := filepath.Join(root, "specs")
	assert.Equal(t, setup.ActionCreated, res.Artifacts[1].Action)
	assert.Equal(t, featureRoot, res.Artifacts[1].Path)
}

// Test_Uninstall_still_adopts_a_config_at_the_enclosing_git_repository_root
// is Test_Init_still_adopts_a_config_at_the_enclosing_git_repository_root's
// own sibling for Uninstall: a config sitting exactly at the nearest
// enclosing git repository root is still adopted and removed, even though
// wd is a subdirectory holding neither a config nor a ".git" of its own.
func Test_Uninstall_still_adopts_a_config_at_the_enclosing_git_repository_root(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))

	srv := newServer(t)

	_, err := srv.Init(t.Context(), root, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	sub := filepath.Join(root, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	res, err := srv.Uninstall(t.Context(), sub, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	assert.Equal(t, root, res.Root)

	configPath := filepath.Join(root, ".brief.yaml")
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionRemoved}, res.Artifacts[0])
	assert.Equal(t, []string{configPath}, res.Removed)

	_, statErr := os.Stat(configPath)
	assert.True(t, os.IsNotExist(statErr))
}

// Test_Uninstall_ignores_an_ancestor_config_outside_the_enclosing_git_repository
// is Test_Init_ignores_an_ancestor_config_outside_the_enclosing_git_repository's
// own sibling for Uninstall: an ancestor config above the enclosing git
// repository is never removed, and Uninstall reports nothing installed
// rather than reaching outside the repository.
func Test_Uninstall_ignores_an_ancestor_config_outside_the_enclosing_git_repository(t *testing.T) {
	home := t.TempDir()
	homeConfig := filepath.Join(home, ".brief.yaml")
	require.NoError(t, os.WriteFile(homeConfig, []byte("feature-directory: home-specs\n"), 0o600))

	proj := filepath.Join(home, "proj")
	require.NoError(t, os.MkdirAll(filepath.Join(proj, ".git"), 0o755))

	srv := setup.NewServer()

	res, err := srv.Uninstall(t.Context(), proj, setup.UninstallRequest{Host: setup.HostNone})

	require.NoError(t, err)
	assert.Empty(t, res.Artifacts)
	assert.Equal(t, proj, res.Root)

	assert.FileExists(t, homeConfig)
	body, readErr := os.ReadFile(homeConfig)
	require.NoError(t, readErr)
	assert.Equal(t, "feature-directory: home-specs\n", string(body))
}
