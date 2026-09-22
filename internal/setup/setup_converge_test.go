package setup_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_a_second_run_reports_every_artifact_unchanged pins R3's convergence:
// once installed, a second Init against the same repository reports both
// artifacts ActionUnchanged and writes nothing further.
func Test_a_second_run_reports_every_artifact_unchanged(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)
	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})
	require.NoError(t, err)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})

	require.NoError(t, err)
	assert.Empty(t, res.Created)
	assert.Empty(t, res.Modified)
	assert.Equal(t, setup.ActionUnchanged, res.Artifacts[0].Action)
	assert.Equal(t, setup.ActionUnchanged, res.Artifacts[1].Action)
}

// Test_a_valid_existing_config_is_kept_and_its_own_feature_directory_wins
// pins R6's "edited locally" branch: a config whose bytes decode without
// violation but differ from artifact.ConfigFile() is left untouched
// (ActionKept) and its own feature-directory value, not Default()'s,
// governs where the feature root is planned.
func Test_a_valid_existing_config_is_kept_and_its_own_feature_directory_wins(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	original := []byte("feature-directory: specs\n")
	require.NoError(t, os.WriteFile(configPath, original, 0o600))
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})

	require.NoError(t, err)
	featureRoot := filepath.Join(wd, "specs")
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionKept, Detail: "edited locally"}, res.Artifacts[0])
	assert.Equal(t, setup.Artifact{Kind: setup.KindFeatureRoot, Path: featureRoot, Action: setup.ActionCreated}, res.Artifacts[1])
	assert.Equal(t, []string{featureRoot}, res.Created)

	body, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, original, body)
}

// Test_an_unparseable_existing_config_refuses_and_changes_nothing pins R3's
// refusal branch for a config config.Inspect cannot decode at all: Init
// returns a *setup.RefusalError naming the config path, and the tree is
// byte-identical to before the call — no feature root, no rewrite.
func Test_an_unparseable_existing_config_refuses_and_changes_nothing(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	original := []byte("feature-directory: [unterminated\n")
	require.NoError(t, os.WriteFile(configPath, original, 0o600))
	srv := newServer(t)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})

	var refusal *setup.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, configPath, refusal.Path)

	entries, readErr := os.ReadDir(wd)
	require.NoError(t, readErr)
	assert.Len(t, entries, 1)

	body, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, original, body)
}

// Test_an_invalid_config_value_refuses_naming_the_key_and_value pins the
// STATE.md open debt S03 closes: the refusal's chain reaches a
// *config.ValueError via errors.As, naming the offending key and value
// exactly as violations() decoded them.
func Test_an_invalid_config_value_refuses_naming_the_key_and_value(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("handoff-cap-lines: 0\n"), 0o600))
	srv := newServer(t)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})

	var refusal *setup.RefusalError
	require.ErrorAs(t, err, &refusal)

	var valueErr *config.ValueError
	require.ErrorAs(t, err, &valueErr)
	assert.Equal(t, "handoff-cap-lines", valueErr.Key)
	assert.Equal(t, 0, valueErr.Value)
}

// Test_force_over_an_invalid_config_rewrites_it_from_defaults pins R3's
// promise that --force never refuses on the old config's content: the same
// fixture Test_an_invalid_config_value_refuses_naming_the_key_and_value
// refuses on, --force instead rewrites to artifact.ConfigFile() verbatim.
func Test_force_over_an_invalid_config_rewrites_it_from_defaults(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("handoff-cap-lines: 0\n"), 0o600))
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, Force: true})

	require.NoError(t, err)
	assert.Equal(t, setup.Artifact{Kind: setup.KindConfig, Path: configPath, Action: setup.ActionCreated, Detail: "rewritten from defaults"}, res.Artifacts[0])

	body, readErr := os.ReadFile(configPath)
	require.NoError(t, readErr)
	assert.Equal(t, artifact.ConfigFile(), body)
}

// Test_force_over_the_current_render_reports_unchanged pins --force's own
// no-op: bytes already equal to artifact.ConfigFile() report
// ActionUnchanged rather than being rewritten. The feature root is created
// alongside it in this fixture — proving the config artifact alone stayed
// untouched needs it excluded from Created, not an empty Created.
func Test_force_over_the_current_render_reports_unchanged(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.WriteFile(configPath, artifact.ConfigFile(), 0o600))
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, Force: true})

	require.NoError(t, err)
	assert.Equal(t, setup.ActionUnchanged, res.Artifacts[0].Action)
	assert.NotContains(t, res.Created, configPath)
}

// Test_a_feature_root_that_is_a_file_refuses_before_writing_the_config pins
// plan-then-apply: the feature-root refusal is decided before the config
// file — a fresh install in this fixture — is ever written.
func Test_a_feature_root_that_is_a_file_refuses_before_writing_the_config(t *testing.T) {
	wd := t.TempDir()
	featureRoot := filepath.Join(wd, "docs", "specifications")
	require.NoError(t, os.MkdirAll(filepath.Dir(featureRoot), 0o755))
	require.NoError(t, os.WriteFile(featureRoot, []byte("not a directory"), 0o600))
	srv := newServer(t)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone})

	require.ErrorIs(t, err, setup.ErrNotADirectory)

	_, statErr := os.Stat(filepath.Join(wd, ".brief.yaml"))
	assert.True(t, os.IsNotExist(statErr))
}

// Test_force_with_the_config_path_as_a_directory_reports_a_partial_write
// pins the apply-order guarantee: the feature root, planned ActionCreated
// in this fixture, lands before the config write is attempted and fails —
// ".brief.yaml" is itself a directory, so nothing can be renamed over it —
// so the returned error wraps setup.ErrPartialWrite rather than reporting
// as if nothing were written.
func Test_force_with_the_config_path_as_a_directory_reports_a_partial_write(t *testing.T) {
	wd := t.TempDir()
	configPath := filepath.Join(wd, ".brief.yaml")
	require.NoError(t, os.Mkdir(configPath, 0o755))
	srv := newServer(t)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, Force: true})

	require.ErrorIs(t, err, setup.ErrPartialWrite)

	info, statErr := os.Stat(filepath.Join(wd, "docs", "specifications"))
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

// Test_dry_run_returns_the_plan_and_writes_nothing pins R9: DryRun reports
// the identical plan a real run would apply — both artifacts ActionCreated
// in this fresh-repo fixture — and the working directory carries no new
// entry afterward.
func Test_dry_run_returns_the_plan_and_writes_nothing(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)

	res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: setup.HostNone, DryRun: true})

	require.NoError(t, err)
	assert.True(t, res.DryRun)
	assert.Empty(t, res.Created)
	assert.Empty(t, res.Modified)
	assert.Equal(t, setup.ActionCreated, res.Artifacts[0].Action)
	assert.Equal(t, setup.ActionCreated, res.Artifacts[1].Action)

	entries, readErr := os.ReadDir(wd)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

// Test_an_unknown_host_reports_ErrUnknownHost pins the usage-error branch
// cli classifies before any refusal rendering: a host outside Hosts()
// never reaches planning at all.
func Test_an_unknown_host_reports_ErrUnknownHost(t *testing.T) {
	wd := t.TempDir()
	srv := newServer(t)

	_, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: "bogus"})

	assert.ErrorIs(t, err, setup.ErrUnknownHost)
}

// Test_operates_in_the_directory_of_a_config_found_in_an_ancestor pins the
// binding decision that init shares resolveRoot's own rule: a config found
// walking up from wd decides the root every path is planned against, not
// wd itself.
func Test_operates_in_the_directory_of_a_config_found_in_an_ancestor(t *testing.T) {
	parent := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(parent, ".brief.yaml"), []byte("feature-directory: specs\n"), 0o600))
	child := filepath.Join(parent, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	srv := newServer(t)

	res, err := srv.Init(t.Context(), child, setup.InitRequest{Host: setup.HostNone})

	require.NoError(t, err)
	featureRoot := filepath.Join(parent, "specs")
	assert.Equal(t, setup.ActionCreated, res.Artifacts[1].Action)
	assert.Equal(t, featureRoot, res.Artifacts[1].Path)
}
