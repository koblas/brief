package setup_test

import (
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_a_second_run_reports_every_artifact_unchanged(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})

	require.NoError(t, err)
	assert.Empty(t, res.Created)
	assert.Empty(t, res.Modified)
	assert.Equal(t, setup.ActionUnchanged, findArtifact(t, res, setup.KindConfig).Action)
	assert.Equal(t, setup.ActionUnchanged, findArtifact(t, res, setup.KindFeatureRoot).Action)
}

func Test_a_valid_existing_config_is_kept_and_its_own_feature_directory_wins(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	original := []byte("feature-directory: specs\n")
	require.NoError(t, mem.WriteFile(memKey(configPath), original, 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})

	require.NoError(t, err)
	featureRoot := filepath.Join(wd, "specs")
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionKept, Detail: "edited locally"}, findArtifact(t, res, setup.KindConfig))
	assert.Equal(t, setup.Artifact{Kind: setup.KindFeatureRoot, Path: featureRoot, Action: setup.ActionCreated}, findArtifact(t, res, setup.KindFeatureRoot))
	assert.Equal(t, []string{featureRoot}, res.Created)

	assert.Equal(t, original, mem.Snapshot()[memKey(configPath)].Data)
}

func Test_an_unparseable_existing_config_refuses_and_changes_nothing(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	original := []byte("feature-directory: [unterminated\n")
	require.NoError(t, mem.WriteFile(memKey(configPath), original, 0o600))
	before := mem.Snapshot()
	srv := newMemServer(mem)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})

	var refusal *setup.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, configPath, refusal.Path)

	assert.Equal(t, before, mem.Snapshot())
}

func Test_an_invalid_config_value_refuses_naming_the_key_and_value(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, mem.WriteFile(memKey(configPath), []byte("handoff-cap-lines: 0\n"), 0o600))
	srv := newMemServer(mem)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})

	var refusal *setup.RefusalError
	require.ErrorAs(t, err, &refusal)

	var valueErr *config.ValueError
	require.ErrorAs(t, err, &valueErr)
	assert.Equal(t, "handoff-cap-lines", valueErr.Key)
	assert.Equal(t, 0, valueErr.Value)
}

func Test_force_over_an_invalid_config_rewrites_it_from_defaults(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, mem.WriteFile(memKey(configPath), []byte("handoff-cap-lines: 0\n"), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, Force: true})

	require.NoError(t, err)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated, Detail: "rewritten from defaults"}, findArtifact(t, res, setup.KindConfig))

	assert.Equal(t, artifact.ConfigFile(), mem.Snapshot()[memKey(configPath)].Data)
}

// The feature root is also created in this fixture, so proving the config
// alone stayed untouched needs it excluded from Created, not an empty one.
func Test_force_over_the_current_render_reports_unchanged(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, mem.WriteFile(memKey(configPath), artifact.ConfigFile(), 0o600))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, Force: true})

	require.NoError(t, err)
	assert.Equal(t, setup.ActionUnchanged, findArtifact(t, res, setup.KindConfig).Action)
	assert.NotContains(t, res.Created, configPath)
}

func Test_a_feature_root_that_is_a_file_refuses_before_writing_the_config(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	featureRoot := filepath.Join(wd, "docs", "specifications")
	require.NoError(t, mem.MkdirAll(memKey(filepath.Dir(featureRoot)), 0o755))
	require.NoError(t, mem.WriteFile(memKey(featureRoot), []byte("not a directory"), 0o600))
	srv := newMemServer(mem)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})

	require.ErrorIs(t, err, setup.ErrNotADirectory)

	assert.NotContains(t, mem.Snapshot(), memKey(filepath.Join(wd, ".brief.yaml")))
}

// The returned Result is populated, not the zero value, so a caller can
// still see what landed before the partial-write failure.
func Test_force_with_the_config_path_as_a_directory_reports_a_partial_write(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, mem.Mkdir(memKey(configPath), 0o755))
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, Force: true})

	require.ErrorIs(t, err, setup.ErrPartialWrite)

	snap := mem.Snapshot()
	featureRoot := filepath.Join(wd, "docs", "specifications")
	info, ok := snap[memKey(featureRoot)]
	require.True(t, ok)
	assert.True(t, info.Mode.IsDir())

	assert.Contains(t, res.Created, featureRoot)
	assert.ElementsMatch(t, []setup.Artifact{
		{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated, Detail: "rewritten from defaults"},
		{Kind: setup.KindFeatureRoot, Path: featureRoot, Action: setup.ActionCreated},
	}, res.Artifacts)
}

func Test_dry_run_returns_the_plan_and_writes_nothing(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	before := mem.Snapshot()
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, DryRun: true})

	require.NoError(t, err)
	assert.True(t, res.DryRun)
	assert.Empty(t, res.Created)
	assert.Empty(t, res.Modified)
	assert.Equal(t, setup.ActionCreated, findArtifact(t, res, setup.KindConfig).Action)
	assert.Equal(t, setup.ActionCreated, findArtifact(t, res, setup.KindFeatureRoot).Action)

	assert.Equal(t, before, mem.Snapshot())
}

func Test_an_unknown_host_reports_ErrUnknownHost(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	srv := newMemServer(mem)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: "bogus"})

	assert.ErrorIs(t, err, setup.ErrUnknownHost)
}

func Test_operates_in_the_directory_of_a_config_found_in_an_ancestor(t *testing.T) {
	parent := fsAbs("repo")
	mem := newVirtualMem(parent)
	require.NoError(t, mem.WriteFile(memKey(parent)+"/.brief.yaml", []byte("feature-directory: specs\n"), 0o600))
	require.NoError(t, mem.Mkdir(memKey(parent)+"/child", 0o755))
	child := filepath.Join(parent, "child")
	srv := newMemServer(mem)

	res, err := srv.Init(t.Context(), child, setup.InitRequest{Host: setup.HostNone})

	require.NoError(t, err)
	featureRoot := filepath.Join(parent, "specs")
	row := findArtifact(t, res, setup.KindFeatureRoot)
	assert.Equal(t, setup.ActionCreated, row.Action)
	assert.Equal(t, featureRoot, row.Path)
}
