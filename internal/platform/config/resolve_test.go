package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/stepfile"
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

func Test_a_config_file_overrides_the_checklist_heading_and_keeps_other_headings_default(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "checklist-heading: \"## Fixture Checklist\"\n")

	cfg, _, err := config.Resolve(root)

	require.NoError(t, err)
	assert.Equal(t, "## Fixture Checklist", cfg.ChecklistHeading)
	assert.Equal(t, config.Default().ProgressHeading, cfg.ProgressHeading)
	assert.Equal(t, config.Default().HandoffFileSuffix, cfg.HandoffFileSuffix)
}

func Test_Resolve_reads_the_handoff_file_suffix_from_the_config_file(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "handoff-file-suffix: \".fixture-handoff.md\"\n")

	cfg, _, err := config.Resolve(root)

	require.NoError(t, err)
	assert.Equal(t, ".fixture-handoff.md", cfg.HandoffFileSuffix)
	assert.Equal(t, config.Default().StepFilePattern, cfg.StepFilePattern)
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

// Test_Resolve_refuses_an_invalid_config_value covers every R1 value rule:
// each case's own config.yaml violates exactly one rule, and Resolve must
// report it as a *config.ValueError naming the offending key, the value
// that failed, and the rule's own reason text, wrapped in
// *config.InvalidConfigError naming the config file. Every case shares one
// assertion tuple (ErrInvalidConfig, Key, Value, the exact Error() text,
// Path) because every rule here is the same family — "this configuration
// value fails its own rule" — differing only in which rule and which
// value; see agent-briefs.md's table-mutation protocol for the per-case
// discrimination this table is verified against.
func Test_Resolve_refuses_an_invalid_config_value(t *testing.T) {
	cases := []struct {
		name       string
		configYAML string
		wantKey    string
		wantValue  any
		wantError  string
	}{
		{
			name:       "handoff-cap-lines at 0",
			configYAML: "handoff-cap-lines: 0\n",
			wantKey:    "handoff-cap-lines",
			wantValue:  0,
			wantError:  "handoff-cap-lines is 0, must be at least 1",
		},
		{
			name:       "handoff-cap-lines at -1",
			configYAML: "handoff-cap-lines: -1\n",
			wantKey:    "handoff-cap-lines",
			wantValue:  -1,
			wantError:  "handoff-cap-lines is -1, must be at least 1",
		},
		{
			name:       "state-cap-lines at 0",
			configYAML: "state-cap-lines: 0\n",
			wantKey:    "state-cap-lines",
			wantValue:  0,
			wantError:  "state-cap-lines is 0, must be at least 1",
		},
		{
			name:       "state-cap-lines at -1",
			configYAML: "state-cap-lines: -1\n",
			wantKey:    "state-cap-lines",
			wantValue:  -1,
			wantError:  "state-cap-lines is -1, must be at least 1",
		},
		{
			name:       "default-output-budget-bytes at 0",
			configYAML: "default-output-budget-bytes: 0\n",
			wantKey:    "default-output-budget-bytes",
			wantValue:  0,
			wantError:  "default-output-budget-bytes is 0, must be at least 1",
		},
		{
			name:       "default-output-budget-bytes at -1",
			configYAML: "default-output-budget-bytes: -1\n",
			wantKey:    "default-output-budget-bytes",
			wantValue:  -1,
			wantError:  "default-output-budget-bytes is -1, must be at least 1",
		},
		{
			name:       "progress-heading blank",
			configYAML: "progress-heading: \"\"\n",
			wantKey:    "progress-heading",
			wantValue:  "",
			wantError:  `progress-heading is "", must not be empty`,
		},
		{
			name:       "checklist-heading blank",
			configYAML: "checklist-heading: \"\"\n",
			wantKey:    "checklist-heading",
			wantValue:  "",
			wantError:  `checklist-heading is "", must not be empty`,
		},
		{
			name:       "acceptance-heading blank",
			configYAML: "acceptance-heading: \"\"\n",
			wantKey:    "acceptance-heading",
			wantValue:  "",
			wantError:  `acceptance-heading is "", must not be empty`,
		},
		{
			name:       "state-headings.binding-decisions blank",
			configYAML: "state-headings:\n  binding-decisions: \"\"\n",
			wantKey:    "state-headings.binding-decisions",
			wantValue:  "",
			wantError:  `state-headings.binding-decisions is "", must not be empty`,
		},
		{
			name:       "state-headings.left-unbuilt blank",
			configYAML: "state-headings:\n  left-unbuilt: \"\"\n",
			wantKey:    "state-headings.left-unbuilt",
			wantValue:  "",
			wantError:  `state-headings.left-unbuilt is "", must not be empty`,
		},
		{
			name:       "state-headings.traps blank",
			configYAML: "state-headings:\n  traps: \"\"\n",
			wantKey:    "state-headings.traps",
			wantValue:  "",
			wantError:  `state-headings.traps is "", must not be empty`,
		},
		{
			name:       "state-headings.open-debts blank",
			configYAML: "state-headings:\n  open-debts: \"\"\n",
			wantKey:    "state-headings.open-debts",
			wantValue:  "",
			wantError:  `state-headings.open-debts is "", must not be empty`,
		},
		{
			name:       "checklist-heading duplicates progress-heading",
			configYAML: "checklist-heading: \"## BDD Acceptance Progress\"\n",
			wantKey:    "checklist-heading",
			wantValue:  "## BDD Acceptance Progress",
			wantError:  `checklist-heading is "## BDD Acceptance Progress", must differ from progress-heading`,
		},
		{
			name:       "specification-file with a forward slash",
			configYAML: "specification-file: sub/SPEC.md\n",
			wantKey:    "specification-file",
			wantValue:  "sub/SPEC.md",
			wantError:  `specification-file is "sub/SPEC.md", must be a plain file name with no path separator`,
		},
		{
			name:       "specification-file is the current-directory dot",
			configYAML: "specification-file: \".\"\n",
			wantKey:    "specification-file",
			wantValue:  ".",
			wantError:  `specification-file is ".", must be a plain file name with no path separator`,
		},
		{
			name:       "state-file with a backslash",
			configYAML: "state-file: \"sub\\\\STATE.md\"\n",
			wantKey:    "state-file",
			wantValue:  `sub\STATE.md`,
			wantError:  `state-file is "sub\\STATE.md", must be a plain file name with no path separator`,
		},
		{
			name:       "state-file equal to specification-file",
			configYAML: "state-file: specification.md\n",
			wantKey:    "state-file",
			wantValue:  "specification.md",
			wantError:  `state-file is "specification.md", must differ from specification-file`,
		},
		{
			name:       "state-file case-only differs from specification-file",
			configYAML: "state-file: SPECIFICATION.MD\n",
			wantKey:    "state-file",
			wantValue:  "SPECIFICATION.MD",
			wantError:  `state-file is "SPECIFICATION.MD", must differ from specification-file`,
		},
		{
			name:       "step-file-pattern with no integer verb",
			configYAML: "step-file-pattern: \"SCENARIO-%s.md\"\n",
			wantKey:    "step-file-pattern",
			wantValue:  "SCENARIO-%s.md",
			wantError:  `step-file-pattern is "SCENARIO-%s.md", must be a plain file name with exactly one %d or %0Nd verb and no other %`,
		},
		{
			name:       "step-file-pattern with two integer verbs",
			configYAML: "step-file-pattern: \"SCENARIO-%d-%d.md\"\n",
			wantKey:    "step-file-pattern",
			wantValue:  "SCENARIO-%d-%d.md",
			wantError:  `step-file-pattern is "SCENARIO-%d-%d.md", must be a plain file name with exactly one %d or %0Nd verb and no other %`,
		},
		{
			name:       "handoff-file-suffix with a path separator",
			configYAML: "handoff-file-suffix: \"sub/HANDOFF.md\"\n",
			wantKey:    "handoff-file-suffix",
			wantValue:  "sub/HANDOFF.md",
			wantError: `handoff-file-suffix is "sub/HANDOFF.md", must be a file-name suffix with no path separator, ` +
				`digit or %, naming a file distinct from the step, state and specification files`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			configPath := writeConfig(t, root, c.configYAML)

			_, _, err := config.Resolve(root)

			require.ErrorIs(t, err, config.ErrInvalidConfig)

			var invalidCfg *config.InvalidConfigError
			require.ErrorAs(t, err, &invalidCfg)
			assert.Equal(t, configPath, invalidCfg.Path)

			var valueErr *config.ValueError
			require.ErrorAs(t, err, &valueErr)
			assert.Equal(t, c.wantKey, valueErr.Key)
			assert.Equal(t, c.wantValue, valueErr.Value)
			assert.Equal(t, c.wantError, valueErr.Error())
		})
	}
}

// Test_Resolve_keeps_the_stepfile_sentinel_reachable_for_a_bad_pattern
// pins that ValueError.Err wraps the stepfile package's own sentinel
// rather than replacing it, for both the step-file-pattern and the
// handoff-file-suffix rules: a caller branching with errors.Is against
// stepfile.ErrInvalidPattern or stepfile.ErrInvalidHandoffSuffix must
// still see through Resolve's *config.InvalidConfigError /
// *config.ValueError wrapping.
func Test_Resolve_keeps_the_stepfile_sentinel_reachable_for_a_bad_pattern(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "step-file-pattern: \"SCENARIO-%s.md\"\n")

	_, _, err := config.Resolve(root)

	assert.ErrorIs(t, err, stepfile.ErrInvalidPattern)
}

// Test_Resolve_keeps_the_handoff_suffix_sentinel_reachable_for_a_bad_suffix
// is Test_Resolve_keeps_the_stepfile_sentinel_reachable_for_a_bad_pattern's
// sibling case for the handoff-file-suffix rule.
func Test_Resolve_keeps_the_handoff_suffix_sentinel_reachable_for_a_bad_suffix(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "handoff-file-suffix: \"sub/HANDOFF.md\"\n")

	_, _, err := config.Resolve(root)

	assert.ErrorIs(t, err, stepfile.ErrInvalidHandoffSuffix)
}

// Test_inspect_reports_every_invalid_value_in_field_declaration_order pins
// Inspect's own contract, doctor's config-values row source: every bad
// value in the file, not only the first, each reported in Config's own
// field-declaration order, alongside the fully decoded Config; a clean
// config and an empty file both decode with no violations; a file that
// cannot be decoded at all refuses with the same *InvalidConfigError shape
// Resolve's own decode failure carries, "resolve config:" prefixed exactly
// once.
func Test_inspect_reports_every_invalid_value_in_field_declaration_order(t *testing.T) {
	t.Run("multiple bad values report in field-declaration order", func(t *testing.T) {
		root := t.TempDir()
		configPath := writeConfig(t, root, "step-file-pattern: \"SCENARIO-%s.md\"\nhandoff-cap-lines: 0\n")

		cfg, violations, err := config.Inspect(configPath)

		require.NoError(t, err)
		require.Len(t, violations, 2)
		assert.Equal(t, "step-file-pattern", violations[0].Key)
		assert.Equal(t, "handoff-cap-lines", violations[1].Key)
		assert.Equal(t, "SCENARIO-%s.md", cfg.StepFilePattern)
	})

	t.Run("a clean config reports no violations", func(t *testing.T) {
		root := t.TempDir()
		configPath := writeConfig(t, root, "progress-heading: \"## Custom Progress\"\n")

		cfg, violations, err := config.Inspect(configPath)

		require.NoError(t, err)
		assert.Empty(t, violations)
		assert.Equal(t, "## Custom Progress", cfg.ProgressHeading)
	})

	t.Run("an empty file is valid and decodes to the shipped defaults", func(t *testing.T) {
		root := t.TempDir()
		configPath := writeConfig(t, root, "")

		cfg, violations, err := config.Inspect(configPath)

		require.NoError(t, err)
		assert.Empty(t, violations)
		assert.Equal(t, config.Default(), cfg)
	})

	t.Run("a file that cannot be decoded refuses with one resolve config prefix", func(t *testing.T) {
		root := t.TempDir()
		configPath := writeConfig(t, root, "progress-heading: [this is not a scalar\n")

		_, violations, err := config.Inspect(configPath)

		require.ErrorIs(t, err, config.ErrInvalidConfig)
		assert.Nil(t, violations)
		assert.Equal(t, 1, strings.Count(err.Error(), "resolve config:"))

		var invalidCfg *config.InvalidConfigError
		require.ErrorAs(t, err, &invalidCfg)
		assert.Equal(t, configPath, invalidCfg.Path)
	})
}

// Test_locate_returns_the_nearest_config_and_the_ancestors_it_shadows pins
// Locate's own contract, doctor's config-shadow row source: the nearest
// ".brief.yaml" wins, every farther ancestor config is reported as
// shadowed (nearest-first — Resolve/Inspect never see them), a repository
// with none reports an empty nearest and no shadowed ancestors, and a
// nonexistent startDir refuses as ErrInvalidConfig, the same guard Resolve
// itself relies on.
func Test_locate_returns_the_nearest_config_and_the_ancestors_it_shadows(t *testing.T) {
	t.Run("the nearest of two configs wins, the farther one is shadowed", func(t *testing.T) {
		root := t.TempDir()
		rootConfig := writeConfig(t, root, "progress-heading: \"## Root Progress\"\n")
		nearDir := filepath.Join(root, "near")
		require.NoError(t, os.MkdirAll(nearDir, 0o755))
		nearConfig := writeConfig(t, nearDir, "progress-heading: \"## Near Progress\"\n")

		nearest, shadowed, err := config.Locate(nearDir)

		require.NoError(t, err)
		assert.Equal(t, nearConfig, nearest)
		assert.Equal(t, []string{rootConfig}, shadowed)
	})

	t.Run("no config anywhere reports an empty nearest and no shadowed ancestors", func(t *testing.T) {
		root := t.TempDir()
		startDir := filepath.Join(root, "a", "b")
		require.NoError(t, os.MkdirAll(startDir, 0o755))

		nearest, shadowed, err := config.Locate(startDir)

		require.NoError(t, err)
		assert.Empty(t, nearest)
		assert.Empty(t, shadowed)
	})

	t.Run("a nonexistent start directory refuses as ErrInvalidConfig", func(t *testing.T) {
		root := t.TempDir()
		missingDir := filepath.Join(root, "does-not-exist")

		_, _, err := config.Locate(missingDir)

		require.ErrorIs(t, err, config.ErrInvalidConfig)
		assert.ErrorContains(t, err, missingDir)
	})
}

// Test_resolve_refuses_with_the_first_violation_inspect_reports pins the
// agreement between Resolve and Inspect: Resolve's own refusal names the
// same key Inspect's own violations[0] would, for a config carrying two
// bad values — proving Resolve is built on Inspect's first element rather
// than a second, independent check. This agreement arm alone cannot catch
// a doubled "resolve config:" prefix, since both sides move together; the
// byte-level proof is internal/cli/invalid_config_test.go and every other
// SCENARIO-01 test passing unmodified.
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
