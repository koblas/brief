// This file reaches the unexported run directly so doctor's own
// environment seams (WithLookPath, WithExecutable, WithBinaryVersion) can
// be pinned deterministically: under `go test`, os.Executable resolves to
// the test binary and exec.LookPath("brief") depends on the runner's own
// PATH, neither of which a black-box cli_test file can control. Every
// other doctor behavior — usage errors, exit codes, JSON shape — is
// exercised the same way other leaves' own internal test files are.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/koblas/brief/internal/doctor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDoctorFixture builds a wd with a valid ".brief.yaml", its default
// feature root ("docs/specifications"), and a ".git" directory, plus a
// "self-brief" file used as both the PATH binary and the running
// binary's own path — the baseline every test below starts from.
func newDoctorFixture(t *testing.T) (string, string) {
	t.Helper()

	wd := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(""), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, "docs", "specifications"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wd, ".git"), 0o755))
	self := filepath.Join(wd, "self-brief")
	require.NoError(t, os.WriteFile(self, []byte("self"), 0o600))

	return wd, self
}

// doctorFakeSeams returns the doctor.Option pair every test below passes
// to run, pointing both the PATH lookup and the running binary at self —
// the same file, so env-path reports OK deterministically.
func doctorFakeSeams(self string) []doctor.Option {
	return []doctor.Option{
		doctor.WithLookPath(func(string) (string, error) { return self, nil }),
		doctor.WithExecutable(func() (string, error) { return self, nil }),
	}
}

func noBuildInfo() (*debug.BuildInfo, bool) { return nil, false }

// Test_doctor_prints_one_row_per_check_and_exits_0_in_a_healthy_repo pins
// R13's row format ("<SEVERITY>  <id>  <path>  <detail>", two-space
// joined, paths relative to wd) and the "setup ok" stderr summary.
func Test_doctor_prints_one_row_per_check_and_exits_0_in_a_healthy_repo(t *testing.T) {
	wd, self := newDoctorFixture(t)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(self)...)

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	want := "OK  config-file  .brief.yaml  found\n" +
		"OK  config-parse  .brief.yaml  parses\n" +
		"OK  config-values  .brief.yaml  no invalid values\n" +
		"OK  config-shadow  .brief.yaml  no ancestor configs shadowed\n" +
		"OK  root-dir  docs/specifications  exists, readable and writable\n" +
		"OK  env-git  .git  found\n" +
		"OK  env-path  self-brief  matches the running binary\n"
	assert.Equal(t, want, stdout.String())
	assert.Equal(t, "brief doctor: setup ok; run 'brief check' for feature content\n", stderr.String())
}

// Test_doctor_reports_a_missing_feature_root_as_an_error_and_exits_1 pins
// R13's exit contract: a missing feature root is ERROR with a fix, the
// stderr summary counts it, and brief doctor exits 1.
func Test_doctor_reports_a_missing_feature_root_as_an_error_and_exits_1(t *testing.T) {
	wd, self := newDoctorFixture(t)
	require.NoError(t, os.RemoveAll(filepath.Join(wd, "docs", "specifications")))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(self)...)

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Contains(t, stdout.String(), "ERROR  root-dir  docs/specifications  does not exist; fix: mkdir -p docs/specifications\n")
	assert.Equal(t, "brief doctor: 1 ERROR, 0 WARN; this checks setup only, run 'brief check' for feature content\n", stderr.String())
}

// Test_doctor_reports_an_unparseable_config_as_rows_not_a_refusal pins
// that an invalid ".brief.yaml" never routes through resolveRoot: it
// still prints every row (config-parse ERROR, config-values SKIP) rather
// than the single-line refusal every other command renders for the same
// file.
func Test_doctor_reports_an_unparseable_config_as_rows_not_a_refusal(t *testing.T) {
	wd, self := newDoctorFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("progress-heading: [not a scalar\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(self)...)

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Contains(t, stdout.String(), "ERROR  config-parse  .brief.yaml")
	assert.Contains(t, stdout.String(), "SKIP  config-values  .brief.yaml  skipped: .brief.yaml did not parse\n")
	assert.NotContains(t, stderr.String(), "no files changed", "an unparseable config must not render as a refusal")
}

// Test_doctor_reports_one_row_per_invalid_value_and_exits_1 pins that
// every *config.ValueError becomes its own row (never collapsed to the
// first, the way config.Resolve's own refusal is).
func Test_doctor_reports_one_row_per_invalid_value_and_exits_1(t *testing.T) {
	wd, self := newDoctorFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("handoff-cap-lines: 0\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(self)...)

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Contains(t, stdout.String(), "ERROR  config-values  .brief.yaml  handoff-cap-lines is 0, must be at least 1; fix: fix it or remove it to fall back to the shipped defaults\n")
}

// Test_doctor_json_reports_absolute_paths_null_fix_and_counts pins
// doctor --json's own document shape: absolute paths, null path/fix where
// none apply, and counts summing to len(checks).
func Test_doctor_json_reports_absolute_paths_null_fix_and_counts(t *testing.T) {
	wd, self := newDoctorFixture(t)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor", "--json"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(self)...)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	var doc struct {
		Schema   int    `json:"schema"`
		Command  string `json:"command"`
		OK       bool   `json:"ok"`
		ExitCode int    `json:"exit_code"`
		Counts   struct {
			Error int `json:"error"`
			Warn  int `json:"warn"`
			OK    int `json:"ok"`
			Skip  int `json:"skip"`
		} `json:"counts"`
		Checks []struct {
			ID       string  `json:"id"`
			Severity string  `json:"severity"`
			Path     *string `json:"path"`
			Detail   string  `json:"detail"`
			Fix      *string `json:"fix"`
		} `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))

	assert.Equal(t, "doctor", doc.Command)
	assert.True(t, doc.OK)
	assert.Equal(t, 0, doc.ExitCode)
	assert.Equal(t, 7, doc.Counts.OK)
	assert.Equal(t, 0, doc.Counts.Error)
	assert.Len(t, doc.Checks, 7)

	configFile := doc.Checks[0]
	assert.Equal(t, "config-file", configFile.ID)
	require.NotNil(t, configFile.Path)
	assert.Equal(t, filepath.Join(wd, ".brief.yaml"), *configFile.Path)
	assert.Nil(t, configFile.Fix)
}

// Test_doctor_json_writes_zero_stderr_bytes pins R1/R6: --json is decided
// before any text-mode line is written, even on a run that would
// otherwise print WARN rows.
func Test_doctor_json_writes_zero_stderr_bytes(t *testing.T) {
	wd, self := newDoctorFixture(t)
	require.NoError(t, os.Remove(filepath.Join(wd, ".brief.yaml")))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor", "--json"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(self)...)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.NotEmpty(t, stdout.String())
}

// Test_doctor_too_many_arguments_is_a_usage_error pins that "brief
// doctor" takes no positional argument.
func Test_doctor_too_many_arguments_is_a_usage_error(t *testing.T) {
	wd, self := newDoctorFixture(t)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor", "extra"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(self)...)

	require.ErrorIs(t, err, ErrUsage)
	assert.Equal(t, 2, ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief doctor: too many arguments; run 'brief doctor'\n", stderr.String())
}

// Test_doctor_refuses_a_working_directory_that_does_not_exist pins the
// one refusal runDoctor emits: config.Locate's own nonexistent-startDir
// guard, checked ahead of Diagnose, the same shape every other command's
// resolveRoot already refuses with.
func Test_doctor_refuses_a_working_directory_that_does_not_exist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), missing, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo)

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), missing)
}
