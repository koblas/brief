package doctor_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/doctor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newHealthyDoctorFixture builds a wd with a valid ".brief.yaml", its
// default feature root ("docs/specifications"), and a ".git" directory —
// the baseline every case in this file starts from, mutated by exactly
// one deviation per test.
func newHealthyDoctorFixture(t *testing.T) string {
	t.Helper()

	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("progress-heading: \"## Progress\"\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, ".git"), 0o755))

	return wd
}

// findCheck returns the first Check in report carrying id, failing the
// test if none does.
func findCheck(t *testing.T, report doctor.Report, id string) doctor.Check {
	t.Helper()

	for _, c := range report.Checks {
		if c.ID == id {
			return c
		}
	}

	t.Fatalf("no check with id %q in report %+v", id, report.Checks)

	return doctor.Check{}
}

// checkIDs returns report's own Check.ID values, in report order.
func checkIDs(report doctor.Report) []string {
	ids := make([]string, len(report.Checks))
	for i, c := range report.Checks {
		ids[i] = c.ID
	}

	return ids
}

// Test_diagnose_reports_every_check_ok_in_a_healthy_repository pins the
// fixed row order (config-file, config-parse, config-values,
// config-shadow, root-dir, env-git, env-path) and that a fully healthy
// repository reports every row OK — not merely "not ERROR or WARN", which
// a SKIP row (the no-config arm's own shape) would also satisfy.
func Test_diagnose_reports_every_check_ok_in_a_healthy_repository(t *testing.T) {
	wd := newHealthyDoctorFixture(t)
	self := filepath.Join(wd, "self-brief")
	require.NoError(t, os.WriteFile(self, []byte("self"), 0o600))

	srv := doctor.NewServer(
		doctor.WithLookPath(func(string) (string, error) { return self, nil }),
		doctor.WithExecutable(func() (string, error) { return self, nil }),
	)

	report := srv.Diagnose(t.Context(), wd)

	assert.Equal(t, []string{"config-file", "config-parse", "config-values", "config-shadow", "root-dir", "env-git", "env-path"}, checkIDs(report))

	for _, c := range report.Checks {
		assert.Equal(t, doctor.SeverityOK, c.Severity, "check %q must be OK in a healthy repo", c.ID)
	}
}

// diagnoseCase is one row of Test_diagnose_classifies_common_setup_problems:
// mutate deviates newHealthyDoctorFixture's baseline by exactly one
// change, and Diagnose's own report must carry checkID at wantSeverity.
type diagnoseCase struct {
	name         string
	mutate       func(t *testing.T, wd string)
	checkID      string
	wantSeverity doctor.Severity
}

// Test_diagnose_classifies_common_setup_problems sweeps the setup faults
// Diagnose must classify: an absent config warns config-file and skips
// config-parse; an unparseable config errors config-parse and skips both
// config-values and root-dir (the feature directory is unknown); a
// missing or non-directory feature root errors root-dir; a ".git" file
// (a worktree) satisfies env-git the same as a directory would, while no
// ".git" at all warns it.
func Test_diagnose_classifies_common_setup_problems(t *testing.T) {
	cases := []diagnoseCase{
		{
			name: "no .brief.yaml anywhere warns config-file",
			mutate: func(t *testing.T, wd string) {
				t.Helper()
				require.NoError(t, os.Remove(filepath.Join(wd, ".brief.yaml")))
			},
			checkID:      "config-file",
			wantSeverity: doctor.SeverityWarn,
		},
		{
			name: "no .brief.yaml anywhere skips config-parse",
			mutate: func(t *testing.T, wd string) {
				t.Helper()
				require.NoError(t, os.Remove(filepath.Join(wd, ".brief.yaml")))
			},
			checkID:      "config-parse",
			wantSeverity: doctor.SeveritySkip,
		},
		{
			name: "no .brief.yaml anywhere still reports config-shadow ok",
			mutate: func(t *testing.T, wd string) {
				t.Helper()
				require.NoError(t, os.Remove(filepath.Join(wd, ".brief.yaml")))
			},
			checkID:      "config-shadow",
			wantSeverity: doctor.SeverityOK,
		},
		{
			name: "an unparseable config errors config-parse",
			mutate: func(t *testing.T, wd string) {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("progress-heading: [not a scalar\n"), 0o600))
			},
			checkID:      "config-parse",
			wantSeverity: doctor.SeverityError,
		},
		{
			name: "an unparseable config skips config-values",
			mutate: func(t *testing.T, wd string) {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("progress-heading: [not a scalar\n"), 0o600))
			},
			checkID:      "config-values",
			wantSeverity: doctor.SeveritySkip,
		},
		{
			name: "an unparseable config skips root-dir",
			mutate: func(t *testing.T, wd string) {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("progress-heading: [not a scalar\n"), 0o600))
			},
			checkID:      "root-dir",
			wantSeverity: doctor.SeveritySkip,
		},
		{
			name: "a missing feature root errors root-dir",
			mutate: func(t *testing.T, wd string) {
				t.Helper()
				require.NoError(t, os.RemoveAll(filepath.Join(wd, "docs", "specifications")))
			},
			checkID:      "root-dir",
			wantSeverity: doctor.SeverityError,
		},
		{
			name: "a feature root that is a file errors root-dir",
			mutate: func(t *testing.T, wd string) {
				t.Helper()
				require.NoError(t, os.RemoveAll(filepath.Join(wd, "docs", "specifications")))
				require.NoError(t, os.WriteFile(filepath.Join(wd, "docs", "specifications"), []byte("x"), 0o600))
			},
			checkID:      "root-dir",
			wantSeverity: doctor.SeverityError,
		},
		{
			name: "a .git file rather than directory satisfies env-git",
			mutate: func(t *testing.T, wd string) {
				t.Helper()
				require.NoError(t, os.RemoveAll(filepath.Join(wd, ".git")))
				require.NoError(t, os.WriteFile(filepath.Join(wd, ".git"), []byte("gitdir: ../.git/worktrees/x\n"), 0o600))
			},
			checkID:      "env-git",
			wantSeverity: doctor.SeverityOK,
		},
		{
			name: "no .git anywhere warns env-git",
			mutate: func(t *testing.T, wd string) {
				t.Helper()
				require.NoError(t, os.RemoveAll(filepath.Join(wd, ".git")))
			},
			checkID:      "env-git",
			wantSeverity: doctor.SeverityWarn,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := newHealthyDoctorFixture(t)
			c.mutate(t, wd)

			srv := doctor.NewServer()
			report := srv.Diagnose(t.Context(), wd)

			check := findCheck(t, report, c.checkID)
			assert.Equal(t, c.wantSeverity, check.Severity)
		})
	}
}

// Test_diagnose_reports_root_dir_readability_and_writability_separately
// pins that root-dir checks readability before writability, each with its
// own detail: an unreadable directory (0o000) never reaches the write
// probe, while a readable-but-unwritable one (0o555) does. Both arms skip
// under euid 0, where chmod's permission bits have no effect.
func Test_diagnose_reports_root_dir_readability_and_writability_separately(t *testing.T) {
	cases := []struct {
		name       string
		mode       os.FileMode
		wantDetail string
	}{
		{name: "not readable", mode: 0o000, wantDetail: "not readable"},
		{name: "readable but not writable", mode: 0o555, wantDetail: "not writable"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("chmod has no effect as root")
			}

			wd := newHealthyDoctorFixture(t)
			root := filepath.Join(wd, "docs", "specifications")
			require.NoError(t, os.Chmod(root, c.mode))
			t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

			srv := doctor.NewServer()
			report := srv.Diagnose(t.Context(), wd)

			check := findCheck(t, report, "root-dir")
			assert.Equal(t, doctor.SeverityError, check.Severity)
			assert.Contains(t, check.Detail, c.wantDetail)
		})
	}
}

// Test_diagnose_reports_one_config_values_row_per_violation_in_field_order
// pins doctor's config-values fan-out: every *config.ValueError Inspect
// reports becomes its own ERROR row, in Config's own field-declaration
// order — step-file-pattern ahead of handoff-cap-lines — never collapsed
// to the first violation the way config.Resolve's own refusal is.
func Test_diagnose_reports_one_config_values_row_per_violation_in_field_order(t *testing.T) {
	wd := newHealthyDoctorFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"),
		[]byte("step-file-pattern: \"SCENARIO-%s.md\"\nhandoff-cap-lines: 0\n"), 0o600))

	srv := doctor.NewServer()
	report := srv.Diagnose(t.Context(), wd)

	var rows []doctor.Check

	for _, c := range report.Checks {
		if c.ID == "config-values" {
			rows = append(rows, c)
		}
	}

	require.Len(t, rows, 2)
	assert.Equal(t, doctor.SeverityError, rows[0].Severity)
	assert.Contains(t, rows[0].Detail, "step-file-pattern")
	assert.Equal(t, doctor.SeverityError, rows[1].Severity)
	assert.Contains(t, rows[1].Detail, "handoff-cap-lines")
}

// Test_diagnose_names_shadowed_ancestor_configs_in_config_shadow_detail
// pins that a farther ancestor ".brief.yaml" — shadowed by the nearer one
// Resolve/Inspect actually read — is still named, so a repository is
// never left wondering which of two configs is in effect. config-shadow
// itself stays OK: a shadowed ancestor is not a fault.
func Test_diagnose_names_shadowed_ancestor_configs_in_config_shadow_detail(t *testing.T) {
	root := t.TempDir()
	rootConfig := filepath.Join(root, ".brief.yaml")
	require.NoError(t, os.WriteFile(rootConfig, []byte("progress-heading: \"## Root\"\n"), 0o600))
	wd := filepath.Join(root, "near")
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("progress-heading: \"## Near\"\n"), 0o600))

	srv := doctor.NewServer()
	report := srv.Diagnose(t.Context(), wd)

	check := findCheck(t, report, "config-shadow")
	assert.Equal(t, doctor.SeverityOK, check.Severity)
	assert.Contains(t, check.Detail, rootConfig)
}

// direntNames returns entries' own names, for comparing a directory
// listing before and after an operation that must not change it.
func direntNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}

	return names
}

// Test_diagnose_leaves_the_feature_root_byte_identical is the mutation
// control for root-dir's write probe: the directory listing before and
// after Diagnose is identical, and the control arm — root-dir reporting
// OK — proves the probe actually ran and found the root writable, rather
// than this test passing vacuously because nothing was ever attempted.
func Test_diagnose_leaves_the_feature_root_byte_identical(t *testing.T) {
	wd := newHealthyDoctorFixture(t)
	root := filepath.Join(wd, "docs", "specifications")
	before, err := os.ReadDir(root)
	require.NoError(t, err)

	srv := doctor.NewServer()
	report := srv.Diagnose(t.Context(), wd)

	after, err := os.ReadDir(root)
	require.NoError(t, err)

	assert.Equal(t, direntNames(before), direntNames(after))

	check := findCheck(t, report, "root-dir")
	assert.Equal(t, doctor.SeverityOK, check.Severity, "control arm: the probe must have run and found the root writable")
}

// envPathCase is one row of Test_diagnose_classifies_env_path: opts
// builds the Server options standing in for lookPath, executable and the
// running version.
type envPathCase struct {
	name         string
	opts         func(t *testing.T, found string) []doctor.Option
	wantSeverity doctor.Severity
}

// Test_diagnose_classifies_env_path sweeps env-path's own rules: brief
// missing from PATH warns; the PATH binary being the same file as the
// running one is OK without ever reading a version; a different file
// carrying the same, non-"(devel)" version is OK; a different file
// carrying a different version, or one whose version cannot be read, both
// warn.
func Test_diagnose_classifies_env_path(t *testing.T) {
	cases := []envPathCase{
		{
			name: "brief is not on PATH",
			opts: func(t *testing.T, _ string) []doctor.Option {
				t.Helper()

				return []doctor.Option{
					doctor.WithLookPath(func(string) (string, error) { return "", os.ErrNotExist }),
				}
			},
			wantSeverity: doctor.SeverityWarn,
		},
		{
			name: "the PATH binary is the running binary",
			opts: func(t *testing.T, found string) []doctor.Option {
				t.Helper()

				return []doctor.Option{
					doctor.WithLookPath(func(string) (string, error) { return found, nil }),
					doctor.WithExecutable(func() (string, error) { return found, nil }),
				}
			},
			wantSeverity: doctor.SeverityOK,
		},
		{
			name: "a different file on PATH carries the same version",
			opts: func(t *testing.T, found string) []doctor.Option {
				t.Helper()

				self := found + "-self"
				require.NoError(t, os.WriteFile(self, []byte("self"), 0o600))

				return []doctor.Option{
					doctor.WithLookPath(func(string) (string, error) { return found, nil }),
					doctor.WithExecutable(func() (string, error) { return self, nil }),
					doctor.WithBinaryVersion(func(string) (string, bool) { return "v1.2.3", true }),
					doctor.WithVersion("v1.2.3"),
				}
			},
			wantSeverity: doctor.SeverityOK,
		},
		{
			name: "a different file on PATH carries a different version",
			opts: func(t *testing.T, found string) []doctor.Option {
				t.Helper()

				self := found + "-self"
				require.NoError(t, os.WriteFile(self, []byte("self"), 0o600))

				return []doctor.Option{
					doctor.WithLookPath(func(string) (string, error) { return found, nil }),
					doctor.WithExecutable(func() (string, error) { return self, nil }),
					doctor.WithBinaryVersion(func(string) (string, bool) { return "v9.9.9", true }),
					doctor.WithVersion("v1.2.3"),
				}
			},
			wantSeverity: doctor.SeverityWarn,
		},
		{
			name: "a different file on PATH has an unreadable version",
			opts: func(t *testing.T, found string) []doctor.Option {
				t.Helper()

				self := found + "-self"
				require.NoError(t, os.WriteFile(self, []byte("self"), 0o600))

				return []doctor.Option{
					doctor.WithLookPath(func(string) (string, error) { return found, nil }),
					doctor.WithExecutable(func() (string, error) { return self, nil }),
					doctor.WithBinaryVersion(func(string) (string, bool) { return "", false }),
					doctor.WithVersion("v1.2.3"),
				}
			},
			wantSeverity: doctor.SeverityWarn,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := newHealthyDoctorFixture(t)
			found := filepath.Join(wd, "found-brief")
			require.NoError(t, os.WriteFile(found, []byte("found"), 0o600))

			srv := doctor.NewServer(c.opts(t, found)...)
			report := srv.Diagnose(t.Context(), wd)

			check := findCheck(t, report, "env-path")
			assert.Equal(t, c.wantSeverity, check.Severity)
		})
	}
}
