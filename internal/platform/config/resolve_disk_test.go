// This file holds config's disk-only tests: Resolve's own decision logic
// that InspectFS alone cannot reach (no config file found, only the
// nearest one read with no merge from a farther, shadowed one, the first
// violation wrapped as *InvalidConfigError, a missing start directory
// refused) — a slim OS-adapter smoke test proving Resolve's own real-file
// wiring, Inspect's own path-rewrite contract on both its error shapes,
// the Abs/cwd-dependent relative-start-directory case, and LocateInRepo's
// own integration with repo.Root, itself still disk-based. InspectFS's own
// decode contract — defaults, violations, every R1 rule, both sentinel
// wraps — is pinned in memory in resolve_test.go; LocateWithinFS's own
// walk is pinned in memory there too.
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

// Test_resolve_wires_the_root_fs_over_real_files is Resolve's own
// OS-adapter smoke test: proof that Locate's walk and Inspect's decode
// compose correctly against real files, three levels down, the nearest
// file's own values decoded and its path reported as source. Locate's own
// walk and InspectFS's own decode are each pinned exhaustively in memory.
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

// Test_Resolve_falls_back_to_the_shipped_profile_when_no_config_file_exists
// pins Resolve's own no-config decision: Default() stands, and the
// reported source is empty — Resolve's own logic, not Locate's (an empty
// nearest) nor Inspect's (never called).
func Test_Resolve_falls_back_to_the_shipped_profile_when_no_config_file_exists(t *testing.T) {
	root := t.TempDir()
	startDir := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(startDir, 0o755))

	cfg, source, err := config.Resolve(startDir)

	require.NoError(t, err)
	assert.Equal(t, config.Default(), cfg)
	assert.Empty(t, source)
}

// Test_Resolve_prefers_the_nearest_config_when_two_exist pins Resolve's own
// nearest-only-read rule: the farther, shadowed config's own values (here,
// handoff-cap-lines) never merge in, even though Locate itself reports the
// shadowed file — only Resolve decides not to read it.
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

// Test_Resolve_refuses_a_start_directory_that_does_not_exist pins Resolve's
// own propagation of Locate's missing-startDir refusal end to end through
// the real root FS.
func Test_Resolve_refuses_a_start_directory_that_does_not_exist(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "progress-heading: \"## Root Progress\"\n")
	missingDir := filepath.Join(root, "does-not-exist")

	_, _, err := config.Resolve(missingDir)

	require.ErrorIs(t, err, config.ErrInvalidConfig)
	assert.ErrorContains(t, err, missingDir)
}

// Test_Resolve_accepts_a_relative_start_directory pins the Abs/cwd-
// dependent case: a relative startDir resolves identically to its own
// absolute form.
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

// Test_resolve_refuses_with_the_first_violation_inspect_reports pins
// Resolve's own first-violation-wrapping rule: for a config carrying two
// bad values, Resolve's own refusal names the same key InspectFS's own
// violations[0] would, proving Resolve is built on Inspect's first element
// rather than a second, independent check.
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

// Test_inspect_refuses_a_config_that_cannot_be_decoded_with_the_original_path
// pins Inspect's own adapter-only contract on top of InspectFS's own
// decode-failure shape (pinned in memory in resolve_test.go): the
// *InvalidConfigError's Path is the file's own real, absolute path, and
// Inspect's own "resolve config:" prefix appears exactly once — never
// doubled by InspectFS's own error passing back through it.
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

// Test_inspect_refuses_a_path_it_cannot_open_with_the_original_path_text
// pins Inspect's own path-rewrite contract on its open-failure shape, the
// control for the decode-failure case above: a missing file is a plain
// error naming the real path twice — Inspect's own "resolve config: <abs>:"
// prefix, then the rewritten *fs.PathError's own "open <abs>: ..." text —
// never *InvalidConfigError.
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

// Test_locate_wires_the_root_fs_over_real_files is Locate's own OS-adapter
// smoke test: proof the production rootFS() (os.DirFS("/")) plus fsName's
// own mapping resolves a real ".brief.yaml" the same way the disk-backed
// walk always did, nearest-wins and shadowed included. LocateWithinFS's own
// walk logic is pinned exhaustively in memory in resolve_test.go.
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

// Test_locate_refuses_a_nonexistent_start_directory_with_the_original_error_text
// pins the exact byte-for-byte error text LocateWithinFS's own
// rewritePathError must reproduce: the root-FS seam runs the existence
// check through fs.Stat, whose *fs.PathError carries the fs-relative name
// rather than missingDir — rewritePathError swaps it back so this text
// reads exactly as os.Stat(missingDir) itself would have produced, the
// contract InvalidConfigError.Error predates the seam.
func Test_locate_refuses_a_nonexistent_start_directory_with_the_original_error_text(t *testing.T) {
	root := t.TempDir()
	missingDir := filepath.Join(root, "does-not-exist")

	_, _, err := config.Locate(missingDir)

	require.ErrorIs(t, err, config.ErrInvalidConfig)

	wantErr := fmt.Sprintf("resolve config: %s: invalid brief config: stat %s: no such file or directory", missingDir, missingDir)
	assert.Equal(t, wantErr, err.Error())
}

// Test_LocateInRepo_rejects_an_ancestor_config_outside_the_enclosing_git_repository
// pins the boundary rule: a ".brief.yaml" that sits above the nearest
// enclosing git repository root is never adopted — reported exactly as if
// none existed, empty nearest, no shadowed ancestors — even though plain
// Locate would find it, since it is a HOME-level (or otherwise unrelated)
// repository's own config, not this one's.
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

// Test_LocateInRepo_adopts_a_config_at_the_enclosing_git_repository_root is
// the control for the case above: a config sitting exactly at, or below,
// the nearest enclosing git repository root is still adopted, walking up
// from a subdirectory that holds neither a config nor a ".git" of its own.
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

// Test_LocateInRepo_behaves_like_Locate_with_no_enclosing_git_repository
// pins the "no boundary" arm: when no ".git" exists anywhere above
// startDir, LocateInRepo keeps today's plain ancestor walk — an ancestor
// config is still adopted exactly as Locate itself would report it.
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
