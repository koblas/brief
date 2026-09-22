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
	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/host"
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

// doctorFakeSeams returns the runSeam every test below passes to run,
// pointing both the PATH lookup and the running binary at self — the same
// file, so env-path reports OK deterministically — and WithHomeDir at a
// fresh, empty directory, so the roles check never reads the developer's
// own real "~/.claude/agents".
func doctorFakeSeams(t *testing.T, self string) []runSeam {
	t.Helper()

	return []runSeam{withDoctorOpts(
		doctor.WithLookPath(func(string) (string, error) { return self, nil }),
		doctor.WithExecutable(func() (string, error) { return self, nil }),
		doctor.WithHomeDir(func() (string, error) { return t.TempDir(), nil }),
	)}
}

func noBuildInfo() (*debug.BuildInfo, bool) { return nil, false }

// Test_doctor_prints_one_row_per_check_and_exits_0_in_a_healthy_repo pins
// R13's row format ("<SEVERITY>  <id>  <path>  <detail>", two-space
// joined, paths relative to wd) and the "setup ok" stderr summary.
func Test_doctor_prints_one_row_per_check_and_exits_0_in_a_healthy_repo(t *testing.T) {
	wd, self := newDoctorFixture(t)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(t, self)...)

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	want := "OK  config-file  .brief.yaml  found\n" +
		"OK  config-parse  .brief.yaml  parses\n" +
		"OK  config-values  .brief.yaml  no invalid values\n" +
		"OK  config-shadow  .brief.yaml  no ancestor configs shadowed\n" +
		"OK  root-dir  docs/specifications  exists, readable and writable\n" +
		"OK  env-git  .git  found\n" +
		"OK  env-path  self-brief  matches the running binary\n" +
		"SKIP  host-plugin  .claude/skills/brief  not installed; fix: run 'brief init --host claude-code'\n" +
		"SKIP  host-hook  .claude/skills/brief/hooks/hooks.json  not installed; fix: run 'brief init --host claude-code'\n" +
		"SKIP  host-snippet  CLAUDE.md  not installed; fix: run 'brief init --host claude-code'\n" +
		"SKIP  host-agents  .claude/skills/brief/agents  not installed; fix: run 'brief init --with-agents'\n" +
		"SKIP  roles  .brief.yaml  no roles bound; fix: run 'brief init --with-agents'\n"
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

	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(t, self)...)

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

	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(t, self)...)

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Contains(t, stdout.String(), "ERROR  config-parse  .brief.yaml")
	assert.Contains(t, stdout.String(), "SKIP  config-values  .brief.yaml  .brief.yaml did not parse\n")
	assert.NotContains(t, stderr.String(), "no files changed", "an unparseable config must not render as a refusal")
}

// Test_doctor_reports_one_row_per_invalid_value_and_exits_1 pins that
// every *config.ValueError becomes its own row (never collapsed to the
// first, the way config.Resolve's own refusal is).
func Test_doctor_reports_one_row_per_invalid_value_and_exits_1(t *testing.T) {
	wd, self := newDoctorFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("handoff-cap-lines: 0\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(t, self)...)

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Contains(t, stdout.String(), "ERROR  config-values  .brief.yaml  handoff-cap-lines is 0, must be at least 1; fix: correct the value, or delete the key to use its default\n")
}

// Test_doctor_json_reports_absolute_paths_null_fix_and_counts pins
// doctor --json's own document shape: absolute paths, null path/fix where
// none apply, and counts summing to len(checks).
func Test_doctor_json_reports_absolute_paths_null_fix_and_counts(t *testing.T) {
	wd, self := newDoctorFixture(t)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor", "--json"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(t, self)...)

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
	assert.Equal(t, 5, doc.Counts.Skip)
	assert.Equal(t, 0, doc.Counts.Error)
	assert.Len(t, doc.Checks, 12)

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

	err := run(t.Context(), wd, []string{"doctor", "--json"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(t, self)...)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.NotEmpty(t, stdout.String())
}

// newFullyInstalledDoctorFixture builds newDoctorFixture's own baseline
// plus a complete Claude Code integration — every Plugin(true) file, the
// three role agents, a role-bound ".brief.yaml" and a root CLAUDE.md
// snippet for the default feature directory — written from
// internal/platform/artifact renders directly, mirroring
// internal/doctor's own fixture.
func newFullyInstalledDoctorFixture(t *testing.T) (string, string) {
	t.Helper()

	wd, self := newDoctorFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(
		"roles:\n"+
			"  planner: brief:planner\n"+
			"  implementer: brief:implementer\n"+
			"  reviewer: brief:reviewer\n"), 0o600))

	h, ok := host.Lookup(host.ClaudeCode)
	require.True(t, ok)

	for _, f := range append(h.Plugin(true), h.Agents()...) {
		path := filepath.Join(wd, filepath.FromSlash(f.RelPath))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, artifact.Render(f.Kind), 0o600))
	}

	block := append(append([]byte{}, artifact.SnippetBlock("docs/specifications")...), '\n')
	require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), block, 0o600))

	return wd, self
}

// Test_doctor_reports_every_row_ok_when_fully_installed pins that a
// repository init already set up end to end reports the five host rows
// and roles all OK, alongside the original seven.
func Test_doctor_reports_every_row_ok_when_fully_installed(t *testing.T) {
	wd, self := newFullyInstalledDoctorFixture(t)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(t, self)...)

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.NotContains(t, stdout.String(), "ERROR")
	assert.NotContains(t, stdout.String(), "WARN")
	assert.NotContains(t, stdout.String(), "SKIP")
	assert.Equal(t, "brief doctor: setup ok; run 'brief check' for feature content\n", stderr.String())
}

// Test_doctor_reports_env_path_error_when_not_on_path_and_the_plugin_is_installed
// pins env-path's own ERROR arm reaching the CLI: brief missing from PATH
// in a fully installed repository is exit 1, with the stderr summary
// counting the one ERROR.
func Test_doctor_reports_env_path_error_when_not_on_path_and_the_plugin_is_installed(t *testing.T) {
	wd, _ := newFullyInstalledDoctorFixture(t)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, withDoctorOpts(
		doctor.WithLookPath(func(string) (string, error) { return "", os.ErrNotExist }),
		doctor.WithHomeDir(func() (string, error) { return t.TempDir(), nil }),
	))

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Contains(t, stdout.String(), "ERROR  env-path")
	assert.Equal(t, "brief doctor: 1 ERROR, 0 WARN; this checks setup only, run 'brief check' for feature content\n", stderr.String())
}

// chmodUnreadableDir chmods dir to 0o000 and registers a t.Cleanup that
// restores it to 0o755 before TempDir's own removal runs — an inaccessible
// directory left at 0o000 (no execute/search bit) would otherwise make
// RemoveAll unable to traverse into it. Skips under euid 0, where chmod's
// permission bits have no effect and every Lstat underneath would silently
// succeed.
func chmodUnreadableDir(t *testing.T, dir string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("chmod has no effect as root")
	}

	require.NoError(t, os.Chmod(dir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

// Test_doctor_env_path_stays_error_when_the_host_snippet_directory_is_unreadable
// pins fix pass 8's M2 fix at the CLI boundary: a ".claude" directory
// doctor cannot even Lstat into (mode 0o000) must not silently flip
// doctor's own exit code from 1 to 0. Repro: ".claude/CLAUDE.md" holds the
// current brief block and brief is missing from PATH — with ".claude"
// readable that is ERROR env-path, exit 1 (doctorLong's own "exits 1 when
// any check is ERROR"); the same tree with ".claude" at 0o000 must keep
// exiting 1, host-snippet must keep discriminating "not readable" from
// "not installed" (never SKIP), and the ERROR count must not read zero —
// a stat failure other than "not found" must never read as "nothing
// installed" one layer up from host-snippet's own row. host-plugin's own
// subject files, equally present-but-unreadable under the same ".claude",
// turn ERROR too (fix pass 10), so this repro's non-zero ERROR count no
// longer rests on env-path alone; host-hook stays WARN regardless, since
// doctor cannot tell a lost hook file from --no-hook either way.
func Test_doctor_env_path_stays_error_when_the_host_snippet_directory_is_unreadable(t *testing.T) {
	wd, _ := newDoctorFixture(t)
	claudeDir := filepath.Join(wd, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0o755))
	block := append(append([]byte{}, artifact.SnippetBlock("docs/specifications")...), '\n')
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), block, 0o600))

	lookPathNotFound := doctor.WithLookPath(func(string) (string, error) { return "", os.ErrNotExist })
	homeDir := doctor.WithHomeDir(func() (string, error) { return t.TempDir(), nil })

	var beforeOut, beforeErr bytes.Buffer
	beforeRunErr := run(t.Context(), wd, []string{"doctor"}, nil, &beforeOut, &beforeErr, noBuildInfo, withDoctorOpts(lookPathNotFound, homeDir))
	require.Error(t, beforeRunErr)
	require.Equal(t, 1, ExitCode(beforeRunErr), "control arm: a readable .claude must report ERROR/exit 1")
	require.Contains(t, beforeOut.String(), "OK  host-snippet")

	chmodUnreadableDir(t, claudeDir)

	var stdout, stderr bytes.Buffer
	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, withDoctorOpts(lookPathNotFound, homeDir))

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Contains(t, stdout.String(), "ERROR  env-path")
	assert.Contains(t, stdout.String(), "WARN  host-snippet")
	assert.NotContains(t, stdout.String(), "SKIP  host-snippet")
	assert.NotContains(t, stderr.String(), "0 ERROR", "an unreadable .claude must not report zero ERROR rows")
}

// Test_doctor_reports_host_plugin_error_when_the_plugin_directory_is_unreadable
// pins fix pass 10's restore: an unreadable host-plugin subject file must
// surface as ERROR, not the WARN fix pass 9 gave it, since a Claude Code
// install "brief doctor" cannot read is one it cannot load either — a
// bare "brief doctor" run against it must exit non-zero, unlike env-path,
// which stays OK throughout this test (brief is found on PATH). Control
// arm: the identical, readable install exits 0.
func Test_doctor_reports_host_plugin_error_when_the_plugin_directory_is_unreadable(t *testing.T) {
	wd := t.TempDir()
	var initStdout, initStderr bytes.Buffer
	require.NoError(t, Run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &initStdout, &initStderr))

	self := filepath.Join(wd, "self-brief")
	require.NoError(t, os.WriteFile(self, []byte("self"), 0o600))
	seams := doctorFakeSeams(t, self)

	var controlOut, controlErr bytes.Buffer
	controlRunErr := run(t.Context(), wd, []string{"doctor"}, nil, &controlOut, &controlErr, noBuildInfo, seams...)
	require.NoError(t, controlRunErr, "control arm: a readable .claude must exit 0")
	assert.Equal(t, 0, ExitCode(controlRunErr))

	chmodUnreadableDir(t, filepath.Join(wd, ".claude"))

	var stdout, stderr bytes.Buffer
	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, seams...)

	require.Error(t, err)
	assert.NotEqual(t, 0, ExitCode(err))
	assert.Contains(t, stdout.String(), "ERROR  host-plugin")
	assert.NotContains(t, stdout.String(), "ERROR  env-path")
}

// Test_doctor_reports_every_host_row_skip_when_dot_claude_is_a_regular_file
// pins P1's ENOTDIR fix at the CLI boundary: a ".claude" that is a plain
// file, not a directory, makes every Lstat through it fail with ENOTDIR —
// classifyProbeError must read that as absent, the same as no ".claude" at
// all, never as present-but-unreadable. Control arm: newDoctorFixture's
// own bare baseline (no ".claude" whatsoever) reports the identical rows
// and exit code, proving the file-in-the-way case is not distinguishable
// from plain absence. Mutation-verified alongside the doctor-level ENOTDIR
// cases (internal/doctor/host_test.go): the same classifyProbeError arm
// backs every row asserted here.
func Test_doctor_reports_every_host_row_skip_when_dot_claude_is_a_regular_file(t *testing.T) {
	wd, _ := newDoctorFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".claude"), []byte("not a directory\n"), 0o600))

	lookPathNotFound := doctor.WithLookPath(func(string) (string, error) { return "", os.ErrNotExist })
	homeDir := doctor.WithHomeDir(func() (string, error) { return t.TempDir(), nil })

	var stdout, stderr bytes.Buffer
	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, withDoctorOpts(lookPathNotFound, homeDir))

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Contains(t, stdout.String(), "SKIP  host-plugin")
	assert.Contains(t, stdout.String(), "SKIP  host-hook")
	assert.Contains(t, stdout.String(), "SKIP  host-snippet")
	assert.Contains(t, stdout.String(), "SKIP  host-agents")
	assert.Contains(t, stdout.String(), "WARN  env-path")
	assert.NotContains(t, stdout.String(), "ERROR")

	controlWd, _ := newDoctorFixture(t)
	var controlOut, controlErr bytes.Buffer
	controlRunErr := run(t.Context(), controlWd, []string{"doctor"}, nil, &controlOut, &controlErr, noBuildInfo, withDoctorOpts(lookPathNotFound, homeDir))

	require.NoError(t, controlRunErr)
	assert.Equal(t, stdout.String(), controlOut.String(), "a '.claude' regular file must report identically to no '.claude' at all")
}

// Test_doctor_too_many_arguments_is_a_usage_error pins that "brief
// doctor" takes no positional argument.
func Test_doctor_too_many_arguments_is_a_usage_error(t *testing.T) {
	wd, self := newDoctorFixture(t)
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor", "extra"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(t, self)...)

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
