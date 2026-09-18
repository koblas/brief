package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeConfig writes a ".brief.yaml" file with the given body under dir and
// returns its path.
func writeConfig(t *testing.T, dir, body string) string {
	t.Helper()

	path := filepath.Join(dir, ".brief.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	return path
}

func Test_Resolve_finds_the_config_at_the_repository_root_from_three_levels_below(t *testing.T) {
	root := t.TempDir()
	configPath := writeConfig(t, root, "progress-heading: \"## Custom Progress\"\n")
	startDir := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(startDir, 0o755))

	cfg, source, err := config.Resolve(startDir)

	require.NoError(t, err)
	require.Equal(t, "## Custom Progress", cfg.ProgressHeading)
	require.Equal(t, configPath, source)
}

func Test_Resolve_falls_back_to_the_shipped_profile_when_no_config_file_exists(t *testing.T) {
	root := t.TempDir()
	startDir := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(startDir, 0o755))

	cfg, source, err := config.Resolve(startDir)

	require.NoError(t, err)
	assert.Equal(t, config.Default(), cfg)
	assert.Empty(t, source)
}

func Test_Resolve_reads_the_four_state_headings_from_the_config_file(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `state-headings:
  binding-decisions: "## Fixture decisions"
  left-unbuilt: "## Fixture left unbuilt"
  traps: "## Fixture traps"
  open-debts: "## Fixture open debts"
`)

	cfg, _, err := config.Resolve(root)

	require.NoError(t, err)
	assert.Equal(t, []string{
		"## Fixture decisions",
		"## Fixture left unbuilt",
		"## Fixture traps",
		"## Fixture open debts",
	}, cfg.StateHeadings.Ordered())
}

func Test_Resolve_keeps_the_shipped_state_headings_when_the_config_omits_them(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "handoff-cap-lines: 42\n")

	cfg, _, err := config.Resolve(root)

	require.NoError(t, err)
	assert.Equal(t, config.Default().StateHeadings.Ordered(), cfg.StateHeadings.Ordered())
}

func Test_Resolve_prefers_the_nearest_config_when_two_exist(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "progress-heading: \"## Root Progress\"\nhandoff-cap-lines: 99\n")
	nearDir := filepath.Join(root, "near")
	require.NoError(t, os.MkdirAll(nearDir, 0o755))
	writeConfig(t, nearDir, "progress-heading: \"## Near Progress\"\n")

	cfg, source, err := config.Resolve(nearDir)

	require.NoError(t, err)
	assert.Equal(t, "## Near Progress", cfg.ProgressHeading)
	assert.Equal(t, config.Default().HandoffCapLines, cfg.HandoffCapLines)
	assert.Equal(t, filepath.Join(nearDir, ".brief.yaml"), source)
}

func Test_Resolve_keeps_shipped_defaults_for_keys_the_config_omits(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "handoff-cap-lines: 12\n")

	cfg, _, err := config.Resolve(root)

	require.NoError(t, err)
	want := config.Default()
	want.HandoffCapLines = 12
	assert.Equal(t, want, cfg)
}

func Test_a_config_file_overrides_the_state_file_name(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "state-file: NOTES.md\n")

	cfg, _, err := config.Resolve(root)

	require.NoError(t, err)
	assert.Equal(t, "NOTES.md", cfg.StateFile)
	assert.Equal(t, config.Default().SpecificationFile, cfg.SpecificationFile)
}

func Test_Resolve_refuses_a_config_with_malformed_yaml(t *testing.T) {
	root := t.TempDir()
	configPath := writeConfig(t, root, "progress-heading: [this is not a scalar\n")

	_, _, err := config.Resolve(root)

	require.ErrorIs(t, err, config.ErrInvalidConfig)
	assert.ErrorContains(t, err, configPath)
}

func Test_Resolve_refuses_a_config_carrying_an_unknown_key(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "not-a-real-key: true\n")

	_, _, err := config.Resolve(root)

	require.ErrorIs(t, err, config.ErrInvalidConfig)
	assert.ErrorContains(t, err, "not-a-real-key")
}

func Test_Resolve_treats_an_empty_config_file_as_the_shipped_profile(t *testing.T) {
	root := t.TempDir()
	configPath := writeConfig(t, root, "")

	cfg, source, err := config.Resolve(root)

	require.NoError(t, err)
	assert.Equal(t, config.Default(), cfg)
	assert.Equal(t, configPath, source)
}

func Test_names_the_offending_file_when_the_config_is_invalid(t *testing.T) {
	root := t.TempDir()
	configPath := writeConfig(t, root, "not-a-real-key: true\n")

	_, _, err := config.Resolve(root)

	require.ErrorIs(t, err, config.ErrInvalidConfig)

	var target *config.InvalidConfigError
	require.ErrorAs(t, err, &target)
	assert.Equal(t, configPath, target.Path)
}

func Test_Resolve_refuses_a_start_directory_that_does_not_exist(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "progress-heading: \"## Root Progress\"\n")
	missingDir := filepath.Join(root, "does-not-exist")

	_, _, err := config.Resolve(missingDir)

	require.ErrorIs(t, err, config.ErrInvalidConfig)
	assert.ErrorContains(t, err, missingDir)
}

func Test_Resolve_accepts_a_relative_start_directory(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "progress-heading: \"## Relative Progress\"\n")
	cwd, err := os.Getwd()
	require.NoError(t, err)
	relDir, err := filepath.Rel(cwd, root)
	require.NoError(t, err)

	absCfg, absSource, absErr := config.Resolve(root)
	require.NoError(t, absErr)

	relCfg, relSource, relErr := config.Resolve(relDir)

	require.NoError(t, relErr)
	assert.Equal(t, absCfg, relCfg)
	assert.Equal(t, absSource, relSource)
}
