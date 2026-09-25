package setup_test

import (
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newServer builds the Server every test in this file exercises.
func newServer(t *testing.T) *setup.Server {
	t.Helper()

	return setup.NewServer()
}

func Test_init_creates_the_config_and_feature_root_in_a_fresh_repo(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})

	require.NoError(t, err)
	assert.Equal(t, setup.HostNone, res.Host)
	assert.False(t, res.DryRun)

	configPath := filepath.Join(wd, ".brief.yaml")
	featureRoot := filepath.Join(wd, "docs", "specifications")

	assert.ElementsMatch(t, []setup.Artifact{
		{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated},
		{Kind: setup.KindFeatureRoot, Path: featureRoot, Action: setup.ActionCreated},
	}, res.Artifacts)
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
