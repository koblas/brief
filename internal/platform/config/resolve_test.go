package config_test

import (
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fsAbs joins slash-separated segments under "/", matching how fsName
// strips the leading "/" to get the fs.FS-relative name back.
func fsAbs(elem ...string) string {
	return filepath.FromSlash("/" + filepath.ToSlash(filepath.Join(elem...)))
}

// inspectFixtureName is the one file every InspectFS fixture holds.
const inspectFixtureName = "config.yaml"

// inspectFixture returns a fstest.MapFS holding a single file at
// inspectFixtureName, body its contents.
func inspectFixture(body string) fstest.MapFS {
	return fstest.MapFS{inspectFixtureName: &fstest.MapFile{Data: []byte(body)}}
}

func Test_locate_within_fs_nearest_and_shadowed(t *testing.T) {
	t.Run("the nearest of two configs wins, the farther one is shadowed", func(t *testing.T) {
		fsys := fstest.MapFS{
			"repo/.brief.yaml":      &fstest.MapFile{Data: []byte("progress-heading: \"## Root\"\n")},
			"repo/near/.brief.yaml": &fstest.MapFile{Data: []byte("progress-heading: \"## Near\"\n")},
		}

		nearest, shadowed, err := config.LocateWithinFS(fsys, fsAbs("repo", "near"), "")

		require.NoError(t, err)
		assert.Equal(t, fsAbs("repo", "near", ".brief.yaml"), nearest)
		assert.Equal(t, []string{fsAbs("repo", ".brief.yaml")}, shadowed)
	})

	t.Run("no config anywhere reports an empty nearest and no shadowed ancestors", func(t *testing.T) {
		fsys := fstest.MapFS{
			"repo/a/b/README.md": &fstest.MapFile{Data: []byte("x")},
		}

		nearest, shadowed, err := config.LocateWithinFS(fsys, fsAbs("repo", "a", "b"), "")

		require.NoError(t, err)
		assert.Empty(t, nearest)
		assert.Empty(t, shadowed)
	})
}

// Must name startAbs itself, not the fs-relative name fs.Stat saw.
func Test_locate_within_fs_missing_start_dir(t *testing.T) {
	fsys := fstest.MapFS{
		"repo/.brief.yaml": &fstest.MapFile{Data: []byte("x")},
	}

	_, _, err := config.LocateWithinFS(fsys, fsAbs("repo", "does-not-exist"), "")

	require.ErrorIs(t, err, config.ErrInvalidConfig)

	var target *config.InvalidConfigError
	require.ErrorAs(t, err, &target)
	assert.Equal(t, fsAbs("repo", "does-not-exist"), target.Path)
}

func Test_locate_within_fs_walk_terminates_at_root(t *testing.T) {
	fsys := fstest.MapFS{
		"a/b/marker.txt": &fstest.MapFile{Data: []byte("x")},
		".brief.yaml":    &fstest.MapFile{Data: []byte("progress-heading: \"## Root\"\n")},
	}

	nearest, shadowed, err := config.LocateWithinFS(fsys, fsAbs("a", "b"), "")

	require.NoError(t, err)
	assert.Equal(t, fsAbs(".brief.yaml"), nearest)
	assert.Empty(t, shadowed)
}

func Test_locate_within_fs_stops_at_boundary(t *testing.T) {
	t.Run("a config above the boundary is not found", func(t *testing.T) {
		fsys := fstest.MapFS{
			".brief.yaml":         &fstest.MapFile{Data: []byte("x")},
			"repo/sub/marker.txt": &fstest.MapFile{Data: []byte("x")},
		}

		nearest, shadowed, err := config.LocateWithinFS(fsys, fsAbs("repo", "sub"), fsAbs("repo"))

		require.NoError(t, err)
		assert.Empty(t, nearest)
		assert.Empty(t, shadowed)

		// Control: the same fixture with no boundary does find it.
		nearest, shadowed, err = config.LocateWithinFS(fsys, fsAbs("repo", "sub"), "")

		require.NoError(t, err)
		assert.Equal(t, fsAbs(".brief.yaml"), nearest)
		assert.Empty(t, shadowed)
	})

	t.Run("a config at the boundary is found", func(t *testing.T) {
		fsys := fstest.MapFS{
			"boundary/.brief.yaml":    &fstest.MapFile{Data: []byte("x")},
			"boundary/sub/marker.txt": &fstest.MapFile{Data: []byte("x")},
		}

		nearest, shadowed, err := config.LocateWithinFS(fsys, fsAbs("boundary", "sub"), fsAbs("boundary"))

		require.NoError(t, err)
		assert.Equal(t, fsAbs("boundary", ".brief.yaml"), nearest)
		assert.Empty(t, shadowed)
	})

	t.Run("an empty boundary is unbounded, identical to no boundary", func(t *testing.T) {
		fsys := fstest.MapFS{
			".brief.yaml":    &fstest.MapFile{Data: []byte("x")},
			"a/b/marker.txt": &fstest.MapFile{Data: []byte("x")},
		}

		nearest, shadowed, err := config.LocateWithinFS(fsys, fsAbs("a", "b"), "")

		require.NoError(t, err)
		assert.Equal(t, fsAbs(".brief.yaml"), nearest)
		assert.Empty(t, shadowed)
	})
}

func Test_inspect_fs_decodes_onto_defaults(t *testing.T) {
	cases := []struct {
		name string
		body string
		want func(*testing.T, config.Config)
	}{
		{
			name: "the four state headings",
			body: `state-headings:
  binding-decisions: "## Fixture decisions"
  left-unbuilt: "## Fixture left unbuilt"
  traps: "## Fixture traps"
  open-debts: "## Fixture open debts"
`,
			want: func(t *testing.T, cfg config.Config) {
				t.Helper()
				assert.Equal(t, []string{
					"## Fixture decisions",
					"## Fixture left unbuilt",
					"## Fixture traps",
					"## Fixture open debts",
				}, cfg.StateHeadings.Ordered())
			},
		},
		{
			name: "state headings omitted keep the shipped defaults",
			body: "handoff-cap-lines: 42\n",
			want: func(t *testing.T, cfg config.Config) {
				t.Helper()
				assert.Equal(t, config.Default().StateHeadings.Ordered(), cfg.StateHeadings.Ordered())
			},
		},
		{
			name: "checklist heading overridden, others stay default",
			body: "checklist-heading: \"## Fixture Checklist\"\n",
			want: func(t *testing.T, cfg config.Config) {
				t.Helper()
				assert.Equal(t, "## Fixture Checklist", cfg.ChecklistHeading)
				assert.Equal(t, config.Default().ProgressHeading, cfg.ProgressHeading)
				assert.Equal(t, config.Default().HandoffFileSuffix, cfg.HandoffFileSuffix)
			},
		},
		{
			name: "handoff file suffix overridden",
			body: "handoff-file-suffix: \".fixture-handoff.md\"\n",
			want: func(t *testing.T, cfg config.Config) {
				t.Helper()
				assert.Equal(t, ".fixture-handoff.md", cfg.HandoffFileSuffix)
				assert.Equal(t, config.Default().StepFilePattern, cfg.StepFilePattern)
			},
		},
		{
			name: "state file name overridden",
			body: "state-file: NOTES.md\n",
			want: func(t *testing.T, cfg config.Config) {
				t.Helper()
				assert.Equal(t, "NOTES.md", cfg.StateFile)
				assert.Equal(t, config.Default().SpecificationFile, cfg.SpecificationFile)
			},
		},
		{
			name: "every other key omitted keeps every other shipped default",
			body: "handoff-cap-lines: 12\n",
			want: func(t *testing.T, cfg config.Config) {
				t.Helper()
				want := config.Default()
				want.HandoffCapLines = 12
				assert.Equal(t, want, cfg)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fsys := inspectFixture(c.body)

			cfg, violations, err := config.InspectFS(fsys, fsAbs(inspectFixtureName))

			require.NoError(t, err)
			assert.Empty(t, violations)
			c.want(t, cfg)
		})
	}
}

func Test_inspect_fs_treats_an_empty_file_as_the_shipped_profile(t *testing.T) {
	fsys := inspectFixture("")

	cfg, violations, err := config.InspectFS(fsys, fsAbs(inspectFixtureName))

	require.NoError(t, err)
	assert.Empty(t, violations)
	assert.Equal(t, config.Default(), cfg)
}

func Test_inspect_fs_refuses_a_config_that_fails_to_decode(t *testing.T) {
	t.Run("malformed yaml", func(t *testing.T) {
		fsys := inspectFixture("progress-heading: [this is not a scalar\n")

		_, violations, err := config.InspectFS(fsys, fsAbs(inspectFixtureName))

		require.ErrorIs(t, err, config.ErrInvalidConfig)
		assert.Nil(t, violations)

		var invalidCfg *config.InvalidConfigError
		require.ErrorAs(t, err, &invalidCfg)
		assert.Equal(t, fsAbs(inspectFixtureName), invalidCfg.Path)
	})

	t.Run("unknown key", func(t *testing.T) {
		fsys := inspectFixture("not-a-real-key: true\n")

		_, violations, err := config.InspectFS(fsys, fsAbs(inspectFixtureName))

		require.ErrorIs(t, err, config.ErrInvalidConfig)
		assert.Nil(t, violations)
		assert.ErrorContains(t, err, "not-a-real-key")
	})
}

// Control for the decode-failure case above: a name fsys has no entry for
// is a plain *fs.PathError, never *InvalidConfigError.
func Test_inspect_fs_refuses_a_path_it_cannot_open(t *testing.T) {
	fsys := fstest.MapFS{}

	_, violations, err := config.InspectFS(fsys, fsAbs("missing.yaml"))

	assert.Nil(t, violations)
	require.NotErrorIs(t, err, config.ErrInvalidConfig)

	var pathErr *fs.PathError
	require.ErrorAs(t, err, &pathErr)
	assert.Equal(t, fsAbs("missing.yaml"), pathErr.Path)
}

func Test_inspect_fs_reports_every_invalid_value_in_field_declaration_order(t *testing.T) {
	t.Run("multiple bad values report in field-declaration order", func(t *testing.T) {
		fsys := inspectFixture("step-file-pattern: \"SCENARIO-%s.md\"\nhandoff-cap-lines: 0\n")

		cfg, violations, err := config.InspectFS(fsys, fsAbs(inspectFixtureName))

		require.NoError(t, err)
		require.Len(t, violations, 2)
		assert.Equal(t, "step-file-pattern", violations[0].Key)
		assert.Equal(t, "handoff-cap-lines", violations[1].Key)
		assert.Equal(t, "SCENARIO-%s.md", cfg.StepFilePattern)
	})

	t.Run("a clean config reports no violations", func(t *testing.T) {
		fsys := inspectFixture("progress-heading: \"## Custom Progress\"\n")

		cfg, violations, err := config.InspectFS(fsys, fsAbs(inspectFixtureName))

		require.NoError(t, err)
		assert.Empty(t, violations)
		assert.Equal(t, "## Custom Progress", cfg.ProgressHeading)
	})
}

// Each case's fixture violates exactly one value rule; InspectFS must
// report it as a *config.ValueError naming the key, value and reason.
func Test_inspect_fs_reports_a_violation_for_every_r1_rule(t *testing.T) {
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
			fsys := inspectFixture(c.configYAML)

			_, violations, err := config.InspectFS(fsys, fsAbs(inspectFixtureName))

			require.NoError(t, err)
			require.Len(t, violations, 1)
			assert.Equal(t, c.wantKey, violations[0].Key)
			assert.Equal(t, c.wantValue, violations[0].Value)
			assert.Equal(t, c.wantError, violations[0].Error())
		})
	}
}

func Test_inspect_fs_keeps_the_stepfile_sentinel_reachable_for_a_bad_pattern(t *testing.T) {
	fsys := inspectFixture("step-file-pattern: \"SCENARIO-%s.md\"\n")

	_, violations, err := config.InspectFS(fsys, fsAbs(inspectFixtureName))

	require.NoError(t, err)
	require.Len(t, violations, 1)
	assert.ErrorIs(t, violations[0], stepfile.ErrInvalidPattern)
}

func Test_inspect_fs_keeps_the_handoff_suffix_sentinel_reachable_for_a_bad_suffix(t *testing.T) {
	fsys := inspectFixture("handoff-file-suffix: \"sub/HANDOFF.md\"\n")

	_, violations, err := config.InspectFS(fsys, fsAbs(inspectFixtureName))

	require.NoError(t, err)
	require.Len(t, violations, 1)
	assert.ErrorIs(t, violations[0], stepfile.ErrInvalidHandoffSuffix)
}
