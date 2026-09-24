package setup_test

import (
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newServer builds the Server every test in this file exercises. Init
// takes no seams today, so the helper carries no options of its own — it
// exists so every test constructs the same way and a later option never
// requires shotgun surgery across this file.
func newServer(t *testing.T) *setup.Server {
	t.Helper()

	return setup.NewServer()
}

// Test_init_creates_the_config_and_feature_root_in_a_fresh_repo pins R2 for
// a repository with nothing installed yet: both artifacts report created,
// the config file's bytes are exactly artifact.ConfigFile(), and the
// feature root exists as a directory afterward. Init's own planning and
// apply here never touch a bound-agent path or a symlink, so this is
// Mem-backed rather than disk.
func Test_init_creates_the_config_and_feature_root_in_a_fresh_repo(t *testing.T) {
	wd, mem := newRealRootMem(t)
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})

	require.NoError(t, err)
	assert.Equal(t, setup.HostNone, res.Host)
	assert.False(t, res.DryRun)

	configPath := filepath.Join(wd, ".brief.yaml")
	featureRoot := filepath.Join(wd, "docs", "specifications")

	require.Len(t, res.Artifacts, 2)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated}, res.Artifacts[0])
	assert.Equal(t, setup.Artifact{Kind: setup.KindFeatureRoot, Path: featureRoot, Action: setup.ActionCreated}, res.Artifacts[1])
	assert.ElementsMatch(t, []string{featureRoot, configPath}, res.Created)
	assert.Empty(t, res.Modified)

	snap := mem.Snapshot()
	body := snap[memKey(configPath)]
	require.NotNil(t, body)
	assert.Equal(t, artifact.ConfigFile(), body.Data)

	info := snap[memKey(featureRoot)]
	require.NotNil(t, info)
	assert.True(t, info.Mode.IsDir())
}
