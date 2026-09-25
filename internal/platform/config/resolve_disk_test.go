// This file holds config's disk-only tests: Resolve's decision logic that
// InspectFS alone cannot reach, plus Inspect, Locate and LocateInRepo's
// real-file wiring. InspectFS's decode contract and LocateWithinFS's walk
// are pinned in memory in resolve_test.go.
package config_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

func Test_resolve_wires_the_root_fs_over_real_files(t *testing.T) {
	root := t.TempDir()
	configPath := writeConfig(t, root, "progress-heading: \"## Custom Progress\"\n")
	startDir := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(startDir, 0o755))

	cfg, source, err := config.Resolve(startDir)

	require.NoError(t, err)
	assert.Equal(t, "## Custom Progress", cfg.ProgressHeading)
	assert.Equal(t, configPath, source)
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

// The farther, shadowed config's values (handoff-cap-lines) must not merge
// in, even though Locate itself reports the shadowed file.
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

// For a config carrying two bad values, Resolve's refusal must name the
// same key InspectFS's violations[0] would.
func Test_resolve_refuses_with_the_first_violation_inspect_reports(t *testing.T) {
	root := t.TempDir()
	configPath := writeConfig(t, root, "step-file-pattern: \"SCENARIO-%s.md\"\nhandoff-cap-lines: 0\n")

	_, inspectViolations, inspectErr := config.Inspect(configPath)
	require.NoError(t, inspectErr)
	require.NotEmpty(t, inspectViolations)

	_, _, resolveErr := config.Resolve(root)

	require.ErrorIs(t, resolveErr, config.ErrInvalidConfig)

	var valueErr *config.ValueError
	require.ErrorAs(t, resolveErr, &valueErr)
	assert.Equal(t, inspectViolations[0].Key, valueErr.Key)
}

// Inspect's "resolve config:" prefix must appear exactly once — never
// doubled by InspectFS's error passing back through it.
func Test_inspect_refuses_a_config_that_cannot_be_decoded_with_the_original_path(t *testing.T) {
	root := t.TempDir()
	configPath := writeConfig(t, root, "progress-heading: [this is not a scalar\n")

	_, violations, err := config.Inspect(configPath)

	require.ErrorIs(t, err, config.ErrInvalidConfig)
	assert.Nil(t, violations)
	assert.Equal(t, 1, strings.Count(err.Error(), "resolve config:"))

	var invalidCfg *config.InvalidConfigError
	require.ErrorAs(t, err, &invalidCfg)
	assert.Equal(t, configPath, invalidCfg.Path)
}

// Control for the decode-failure case above: a missing file is a plain
// error naming the real path twice, never *InvalidConfigError.
func Test_inspect_refuses_a_path_it_cannot_open_with_the_original_path_text(t *testing.T) {
	root := t.TempDir()
	missingPath := filepath.Join(root, ".brief.yaml")

	_, violations, err := config.Inspect(missingPath)

	assert.Nil(t, violations)
	require.NotErrorIs(t, err, config.ErrInvalidConfig)

	var pathErr *fs.PathError
	require.ErrorAs(t, err, &pathErr)
	assert.Equal(t, missingPath, pathErr.Path)

	wantErr := fmt.Sprintf("resolve config: %s: open %s: no such file or directory", missingPath, missingPath)
	assert.Equal(t, wantErr, err.Error())
}

func Test_locate_wires_the_root_fs_over_real_files(t *testing.T) {
	root := t.TempDir()
	rootConfig := writeConfig(t, root, "progress-heading: \"## Root Progress\"\n")
	nearDir := filepath.Join(root, "near")
	require.NoError(t, os.MkdirAll(nearDir, 0o755))
	nearConfig := writeConfig(t, nearDir, "progress-heading: \"## Near Progress\"\n")

	nearest, shadowed, err := config.Locate(nearDir)

	require.NoError(t, err)
	assert.Equal(t, nearConfig, nearest)
	assert.Equal(t, []string{rootConfig}, shadowed)
}

// Pins the exact byte-for-byte error text rewritePathError must reproduce,
// as though os.Stat(missingDir) itself had produced it.
func Test_locate_refuses_a_nonexistent_start_directory_with_the_original_error_text(t *testing.T) {
	root := t.TempDir()
	missingDir := filepath.Join(root, "does-not-exist")

	_, _, err := config.Locate(missingDir)

	require.ErrorIs(t, err, config.ErrInvalidConfig)

	wantErr := fmt.Sprintf("resolve config: %s: invalid brief config: stat %s: no such file or directory", missingDir, missingDir)
	assert.Equal(t, wantErr, err.Error())
}

func Test_LocateInRepo_rejects_an_ancestor_config_outside_the_enclosing_git_repository(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, "progress-heading: \"## Home Progress\"\n")
	proj := filepath.Join(home, "proj")
	require.NoError(t, os.MkdirAll(filepath.Join(proj, ".git"), 0o755))

	plainNearest, plainShadowed, plainErr := config.Locate(proj)
	require.NoError(t, plainErr)
	require.NotEmpty(t, plainNearest, "control: plain Locate must find the ancestor config")
	require.Empty(t, plainShadowed)

	nearest, shadowed, err := config.LocateInRepo(proj)

	require.NoError(t, err)
	assert.Empty(t, nearest)
	assert.Empty(t, shadowed)
}

// Control for the case above: a config at the enclosing git repository
// root is still adopted, walking up from a subdirectory with neither.
func Test_LocateInRepo_adopts_a_config_at_the_enclosing_git_repository_root(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	configPath := writeConfig(t, root, "progress-heading: \"## Repo Progress\"\n")
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	nearest, shadowed, err := config.LocateInRepo(sub)

	require.NoError(t, err)
	assert.Equal(t, configPath, nearest)
	assert.Empty(t, shadowed)
}

func Test_LocateInRepo_behaves_like_Locate_with_no_enclosing_git_repository(t *testing.T) {
	root := t.TempDir()
	configPath := writeConfig(t, root, "progress-heading: \"## No Git Progress\"\n")
	start := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(start, 0o755))

	nearest, shadowed, err := config.LocateInRepo(start)

	require.NoError(t, err)
	assert.Equal(t, configPath, nearest)
	assert.Empty(t, shadowed)
}
