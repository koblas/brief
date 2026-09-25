// White-box: reaches the unexported run so doctor's environment seams
// (WithLookPath, WithExecutable) can be pinned deterministically.

package cli

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/doctor"
	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/host"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDoctorFixture builds a wd with a valid ".brief.yaml", its default
// feature root, a ".git" directory, and a "self-brief" file.
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

// doctorFakeSeams points both the PATH lookup and the running binary at
// self, and WithHomeDir at a fresh, empty directory.
func doctorFakeSeams(t *testing.T, self string) []runSeam {
	t.Helper()

	return []runSeam{withDoctorOpts(
		doctor.WithLookPath(func(string) (string, error) { return self, nil }),
		doctor.WithExecutable(func() (string, error) { return self, nil }),
		doctor.WithHomeDir(func() (string, error) { return t.TempDir(), nil }),
	)}
}

func noBuildInfo() (*debug.BuildInfo, bool) { return nil, false }

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
		"SKIP  host-skill  .claude/skills/brief-workflow/SKILL.md  not installed; fix: run 'brief init --host claude-code'\n" +
		"SKIP  host-snippet  CLAUDE.md  not installed; fix: run 'brief init --host claude-code'\n" +
		"SKIP  host-agents  .claude/skills/brief/agents  not installed; fix: run 'brief init --with-agents'\n" +
		"SKIP  roles  .brief.yaml  no roles bound; fix: run 'brief init --with-agents'\n" +
		"SKIP  roles-skill  .brief.yaml  no bound planner or implementer brief can check\n"
	assert.Equal(t, want, stdout.String())
	assert.Equal(t, "brief doctor: setup ok; run 'brief check' for feature content\n", stderr.String())
}

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

// An invalid ".brief.yaml" never routes through resolveRoot: it still
// prints every row rather than the single-line refusal other commands give.
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

// Every *config.ValueError becomes its own row, never collapsed to the
// first the way config.Resolve's refusal is.
func Test_doctor_reports_one_row_per_invalid_value_and_exits_1(t *testing.T) {
	wd, self := newDoctorFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte("handoff-cap-lines: 0\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(t, self)...)

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Contains(t, stdout.String(), "ERROR  config-values  .brief.yaml  handoff-cap-lines is 0, must be at least 1; fix: correct the value, or delete the key to use its default\n")
}

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
	assert.Equal(t, 7, doc.Counts.Skip)
	assert.Equal(t, 0, doc.Counts.Error)
	assert.Len(t, doc.Checks, 14)

	configFile := doc.Checks[0]
	assert.Equal(t, "config-file", configFile.ID)
	require.NotNil(t, configFile.Path)
	assert.Equal(t, filepath.Join(wd, ".brief.yaml"), *configFile.Path)
	assert.Nil(t, configFile.Fix)
}

// --json is decided before any text-mode line is written, even on a run
// that would otherwise print WARN rows.
func Test_doctor_json_writes_zero_stderr_bytes(t *testing.T) {
	wd, self := newDoctorFixture(t)
	require.NoError(t, os.Remove(filepath.Join(wd, ".brief.yaml")))
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), wd, []string{"doctor", "--json"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(t, self)...)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.NotEmpty(t, stdout.String())
}

// newFullyInstalledDoctorFixture builds newDoctorFixture's baseline plus
// a complete Claude Code integration: plugin, agents, skill and snippet.
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

	for _, f := range append(append(h.Plugin(true), h.Agents()...), h.Skills()...) {
		path := filepath.Join(wd, filepath.FromSlash(f.RelPath))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, artifact.Render(f.Kind), 0o600))
	}

	block := append(append([]byte{}, artifact.SnippetBlock("docs/specifications")...), '\n')
	require.NoError(t, os.WriteFile(filepath.Join(wd, "CLAUDE.md"), block, 0o600))

	return wd, self
}

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

// A SKILL.md replaced by a directory is the one new ERROR row host-skill
// grants, and exits 1.
func Test_doctor_reports_a_non_regular_host_skill_as_an_error_and_exits_1(t *testing.T) {
	wd, self := newFullyInstalledDoctorFixture(t)
	skillPath := filepath.Join(wd, filepath.FromSlash(host.WorkflowSkillDir), "SKILL.md")
	require.NoError(t, os.RemoveAll(skillPath))
	require.NoError(t, os.MkdirAll(skillPath, 0o755))

	var stdout, stderr bytes.Buffer
	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(t, self)...)

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Contains(t, stdout.String(), "ERROR  host-skill  .claude/skills/brief-workflow/SKILL.md  not a regular file; fix: remove .claude/skills/brief-workflow/SKILL.md, then run 'brief init'\n")
	assert.Equal(t, "brief doctor: 1 ERROR, 0 WARN; this checks setup only, run 'brief check' for feature content\n", stderr.String())
}

// A repository set up by "brief init --host claude-code --with-agents"
// reports the exact roles-skill row, and doctor still exits 0.
func Test_doctor_reports_roles_skill_ok_after_init_with_agents(t *testing.T) {
	wd := t.TempDir()
	var initStdout, initStderr bytes.Buffer

	initErr := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--with-agents"}, nil, &initStdout, &initStderr, noBuildInfo, emptyHomeSeam(t))
	require.NoError(t, initErr)

	self := filepath.Join(wd, "self-brief")
	require.NoError(t, os.WriteFile(self, []byte("self"), 0o600))

	var stdout, stderr bytes.Buffer
	err := run(t.Context(), wd, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, doctorFakeSeams(t, self)...)

	require.NoError(t, err)
	assert.Equal(t, 0, ExitCode(err))
	assert.Contains(t, stdout.String(), "OK  roles-skill  .brief.yaml  planner, implementer preload brief-workflow\n")
}

// brief missing from PATH in a fully installed repository is exit 1, with
// the stderr summary counting the one ERROR.
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

// chmodUnreadableDir chmods dir to 0o000 and restores it to 0o755 on
// cleanup, before TempDir's own removal runs. Skips under euid 0.
func chmodUnreadableDir(t *testing.T, dir string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("chmod has no effect as root")
	}

	require.NoError(t, os.Chmod(dir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

// A ".claude" directory doctor cannot even Lstat into (mode 0o000) must
// not silently flip doctor's exit code from 1 to 0.
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
	assert.Contains(t, stdout.String(), "ERROR  host-plugin")
	assert.Contains(t, stdout.String(), "WARN  host-snippet")
	assert.NotContains(t, stdout.String(), "SKIP  host-snippet")
	assert.NotContains(t, stderr.String(), "0 ERROR", "an unreadable .claude must not report zero ERROR rows")
}

// An unreadable host-plugin subject file must surface as ERROR: a bare
// "brief doctor" run against it exits non-zero. Control: readable exits 0.
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

// A ".claude" that is a plain file makes every Lstat through it fail with
// ENOTDIR; classifyProbeError must read that as absent, not unreadable.
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

func Test_doctor_too_many_arguments_is_a_usage_error(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), "/repo", []string{"doctor", "extra"}, nil, &stdout, &stderr, noBuildInfo)

	require.ErrorIs(t, err, ErrUsage)
	assert.Equal(t, 2, ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief doctor: too many arguments; run 'brief doctor'\n", stderr.String())
}

// The refusal runDoctor emits on its production path (no withRootFS
// seam), checked ahead of Diagnose.
func Test_doctor_refuses_a_working_directory_that_does_not_exist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), missing, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo)

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), missing)
}

// Same refusal on runDoctor's withRootFS branch, over an rwfs.Mem holding
// nothing at all.
func Test_doctor_refuses_a_working_directory_that_does_not_exist_on_a_seamed_fsys(t *testing.T) {
	missing := "/repo/does-not-exist"
	mem := rwfs.NewMem(fstest.MapFS{})
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), missing, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, withRootFS(mem))

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), missing)
}

// Control arm for the two refusal tests above: wd is present on the
// seamed rwfs.Mem, so the pre-check's refusal never fires.
func Test_doctor_reaches_diagnose_when_the_seamed_wd_exists_on_the_fsys(t *testing.T) {
	present := "/repo"
	mem := rwfs.NewMem(fstest.MapFS{"repo": &fstest.MapFile{Mode: fs.ModeDir | 0o755}})
	var stdout, stderr bytes.Buffer

	err := run(t.Context(), present, []string{"doctor"}, nil, &stdout, &stderr, noBuildInfo, withRootFS(mem))

	require.Error(t, err)
	assert.Equal(t, 1, ExitCode(err))
	assert.Contains(t, stdout.String(), "root-dir")
}
