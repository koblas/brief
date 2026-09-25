// White-box: run()'s withRootFS seam is unexported. check --hook's case
// reads real disk and lives in stdout_failure_disk_test.go.

package cli

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingWriter refuses every Write, so a multi-piece renderer fails on each.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, &fs.PathError{Op: "write", Path: "/dev/stdout", Err: syscall.ENOSPC}
}

// errReadOnly wraps setup.ErrUnwritable to drive init's unwritable refusal.
var errReadOnly = fmt.Errorf("read-only file system: %w", setup.ErrUnwritable)

// writeFailureCause is failingWriter's error text.
const writeFailureCause = "write /dev/stdout: no space left on device"

// runMemFailingStdout runs args against tree with a failing stdout.
func runMemFailingStdout(t *testing.T, tree *memTree, args []string) (string, error) {
	t.Helper()

	var stderr strings.Builder

	err := run(t.Context(), memRoot, args, nil, failingWriter{}, &stderr, noBuildInfo, withRootFS(tree.mem()))

	return stderr.String(), err
}

// wantReadFailure is the stdout-failure line for a run that changed no files.
func wantReadFailure(command, hint string) string {
	return "brief " + command + ": writing to stdout failed: " + writeFailureCause +
		"; fix the output destination, then run '" + hint + "' again\n"
}

// wantWriteFailure is the stdout-failure line for a run that changed files.
func wantWriteFailure(command, hint string) string {
	return "brief " + command + ": writing to stdout failed: " + writeFailureCause +
		"; files were already changed, check them with 'git status' before running '" + hint + "' again\n"
}

func Test_status_reports_a_stdout_write_failure_on_stderr_mem(t *testing.T) {
	stderr, err := runMemFailingStdout(t, newMemStatusFixture(), []string{"status"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantReadFailure("status", statusInvocation), stderr)
}

func Test_status_json_reports_a_stdout_write_failure_on_stderr_mem(t *testing.T) {
	stderr, err := runMemFailingStdout(t, newMemStatusFixture(), []string{"status", "--json"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantReadFailure("status", statusInvocation), stderr)
}

func Test_start_reports_a_stdout_write_failure_on_stderr_mem(t *testing.T) {
	stderr, err := runMemFailingStdout(t, newMemStartFixture("open"), []string{"start", "demo"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantReadFailure("start", startInvocation), stderr)
}

func Test_check_reports_a_stdout_write_failure_on_stderr_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.
		file(filepath.Join(featureDir, "specification.md"), memConformingSpec).
		file(filepath.Join(featureDir, "STATE.md"), memConformingState)
	memCheckStep(tree, featureDir, "SCENARIO-01", "open", []string{"- [ ] do the thing"})
	tree.file(filepath.Join(featureDir, "SCENARIO-01-HANDOFF.md"), memCheckBodyOfLines(61))

	stderr, err := runMemFailingStdout(t, tree, []string{"check"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantReadFailure("check", checkInvocation), stderr)
}

func Test_finish_json_reports_a_stdout_write_failure_after_replacing_the_state_file_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	newState := "## Binding decisions\n\nnew decision\n\n## Left unbuilt\n\nnothing\n\n## Traps\n\nnone\n\n## Open debts\n\nnone\n"
	statePath := memWriteInput(tree, "state.md", newState)
	mem := tree.mem()

	var stderr strings.Builder

	err := run(t.Context(), memRoot, []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"},
		nil, failingWriter{}, &stderr, noBuildInfo, withRootFS(mem))

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantWriteFailure("finish", finishInvocation), stderr.String())

	got, readErr := fs.ReadFile(mem, fsName(filepath.Join(memRoot, "docs", "specifications", "demo", "STATE.md")))
	require.NoError(t, readErr)
	assert.Equal(t, newState, string(got))
}

func Test_start_reports_a_stdout_write_failure_after_its_convention_notices_mem(t *testing.T) {
	tree := newMemStartFixture("open")
	featureDir := filepath.Join(memRoot, "docs", "specifications", "demo")
	tree.file(filepath.Join(featureDir, "STATE.md"), "## Binding decisions\n\nd\n\n## Left unbuilt\n\nl\n\n## Traps\n\nt\n")

	stderr, err := runMemFailingStdout(t, tree, []string{"start", "demo"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, "brief start: "+filepath.Join("docs", "specifications", "demo", "STATE.md")+
		": no \"## Open debts\" heading found; add a \"## Open debts\" heading to the state file\n"+
		wantReadFailure("start", startInvocation), stderr)
}

func Test_finish_json_no_op_reports_a_stdout_write_failure_as_safe_to_re_run_mem(t *testing.T) {
	tree := newMemFinishFixture("- [x] do the thing")
	handoffPath := memWriteInput(tree, "handoff.md", "NEW-HANDOFF\n")
	statePath := memWriteInput(tree, "state.md", "## Binding decisions\n\nd\n\n## Left unbuilt\n\nn\n\n## Traps\n\nn\n\n## Open debts\n\nn\n")
	mem := tree.mem()
	args := []string{"finish", "demo", "SCENARIO-01", "--handoff", handoffPath, "--state", statePath, "--json"}
	_, _, firstErr := runFinishArgsMem(t, mem, args)
	require.NoError(t, firstErr)

	var stderr strings.Builder

	err := run(t.Context(), memRoot, args, nil, failingWriter{}, &stderr, noBuildInfo, withRootFS(mem))

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantReadFailure("finish", finishInvocation), stderr.String())
}

func Test_new_feature_json_reports_a_stdout_write_failure_after_creating_files_mem(t *testing.T) {
	stderr, err := runMemFailingStdout(t, newMemTree(memRoot), []string{"new", "feature", "payments", "--json"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantWriteFailure("new feature", newFeatureInvocation), stderr)
}

// runSetupFailingStdout runs each of before with a working stdout, then args
// with a failing one, in a fresh virtual repository.
func runSetupFailingStdout(t *testing.T, args []string, before ...[]string) (string, error) {
	t.Helper()

	wd := fsAbs("repo")
	seam := newMemSetupSeam(newVirtualMem(wd))

	for _, b := range before {
		var stdout, stderr strings.Builder
		require.NoError(t, run(t.Context(), wd, b, nil, &stdout, &stderr, noBuildInfo, seam))
	}

	var stderr strings.Builder

	err := run(t.Context(), wd, args, nil, failingWriter{}, &stderr, noBuildInfo, seam)

	return stderr.String(), err
}

func Test_init_json_reports_a_stdout_write_failure_after_writing_files(t *testing.T) {
	stderr, err := runSetupFailingStdout(t, []string{"init", "--host", "none", "--json"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantWriteFailure("init", initInvocation), stderr)
}

func Test_init_dry_run_json_reports_a_stdout_write_failure_as_safe_to_re_run(t *testing.T) {
	stderr, err := runSetupFailingStdout(t, []string{"init", "--host", "none", "--dry-run", "--json"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantReadFailure("init", initInvocation), stderr)
}

func Test_uninstall_json_reports_a_stdout_write_failure_after_removing_files(t *testing.T) {
	stderr, err := runSetupFailingStdout(t, []string{"uninstall", "--host", "none", "--json"},
		[]string{"init", "--host", "none"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantWriteFailure("uninstall", uninstallInvocation), stderr)
}

func Test_uninstall_json_with_nothing_installed_reports_a_stdout_write_failure_as_safe_to_re_run(t *testing.T) {
	stderr, err := runSetupFailingStdout(t, []string{"uninstall", "--host", "none", "--json"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantReadFailure("uninstall", uninstallInvocation), stderr)
}

func Test_completion_reports_a_stdout_write_failure_on_stderr(t *testing.T) {
	var stderr strings.Builder

	err := run(t.Context(), memRoot, []string{"completion", "bash"}, nil, failingWriter{}, &stderr, noBuildInfo)

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantReadFailure("completion", completionInvocation), stderr.String())
}

func Test_version_json_reports_a_stdout_write_failure_on_stderr(t *testing.T) {
	var stderr strings.Builder

	err := run(t.Context(), memRoot, []string{"--version", "--json"}, nil, failingWriter{}, &stderr, noBuildInfo)

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantReadFailure("--version", "brief --version --json"), stderr.String())
}

func Test_init_json_re_run_reports_a_stdout_write_failure_as_safe_to_re_run(t *testing.T) {
	stderr, err := runSetupFailingStdout(t, []string{"init", "--host", "none", "--json"},
		[]string{"init", "--host", "none"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantReadFailure("init", initInvocation), stderr)
}

func Test_start_json_reports_a_stdout_write_failure_on_stderr_mem(t *testing.T) {
	stderr, err := runMemFailingStdout(t, newMemStartFixture("open"), []string{"start", "demo", "--json"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantReadFailure("start", startInvocation), stderr)
}

func Test_check_json_reports_a_stdout_write_failure_on_stderr_mem(t *testing.T) {
	stderr, err := runMemFailingStdout(t, newMemCheckJSONFixture(), []string{"check", "--json"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantReadFailure("check", checkInvocation), stderr)
}

func Test_new_step_json_reports_a_stdout_write_failure_after_creating_the_step_mem(t *testing.T) {
	tree := newMemTree(memRoot, filepath.Join(memRoot, "docs", "specifications"))
	memConformingFeatureFiles(tree, filepath.Join(memRoot, "docs", "specifications", "payments"))

	stderr, err := runMemFailingStdout(t, tree, []string{"new", "step", "payments", "--json"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantWriteFailure("new step", newStepInvocation), stderr)
}

func Test_init_print_json_reports_a_stdout_write_failure_as_safe_to_re_run(t *testing.T) {
	stderr, err := runSetupFailingStdout(t, []string{"init", "--host", "none", "--print", "--json"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantReadFailure("init", initInvocation), stderr)
}

// CLAUDE.md lost its snippet, so the re-run modifies exactly that one file.
func Test_init_json_that_only_modifies_a_file_reports_a_stdout_write_failure_after_writing(t *testing.T) {
	wd := fsAbs("repo")
	mem := newVirtualMem(wd)
	seam := newMemSetupSeam(mem)
	var seedOut, seedErr strings.Builder
	require.NoError(t, run(t.Context(), wd, []string{"init", "--host", "claude-code"}, nil, &seedOut, &seedErr, noBuildInfo, seam))
	require.NoError(t, mem.WriteFile(memKey(filepath.Join(wd, "CLAUDE.md")), []byte("# notes\n"), 0o600))
	var stderr strings.Builder

	err := run(t.Context(), wd, []string{"init", "--host", "claude-code", "--json"}, nil, failingWriter{}, &stderr, noBuildInfo, seam)

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantWriteFailure("init", initInvocation), stderr.String())
}

func Test_doctor_json_reports_a_stdout_write_failure_on_stderr(t *testing.T) {
	wd, self := newDoctorFixture(t)
	var stderr strings.Builder

	err := run(t.Context(), wd, []string{"doctor", "--json"}, nil, failingWriter{}, &stderr, noBuildInfo, doctorFakeSeams(t, self)...)

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, wantReadFailure("doctor", doctorInvocation), stderr.String())
}

func Test_json_refusal_falls_back_to_its_message_on_stderr_when_stdout_fails_mem(t *testing.T) {
	stderr, err := runMemFailingStdout(t, newMemStartFixture("open"), []string{"start", "nosuch", "--json"})

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, "brief start: no feature \"nosuch\" in "+filepath.Join("docs", "specifications")+"; known: demo\n", stderr)
}

func Test_json_usage_error_falls_back_to_its_message_on_stderr_when_stdout_fails_mem(t *testing.T) {
	stderr, err := runMemFailingStdout(t, newMemStatusFixture(), []string{"status", "extra", "--json"})

	assert.Equal(t, 2, ExitCode(err))
	assert.Equal(t, "brief status: too many arguments; run '"+statusInvocation+"'\n", stderr)
}

func Test_init_json_unwritable_refusal_falls_back_to_its_message_on_stderr_when_stdout_fails(t *testing.T) {
	wd := fsAbs("repo")
	seam := newMemSetupSeam(newVirtualMem(wd), setup.WithWritableCheck(func([]string) error { return errReadOnly }))
	var textStderr, textStdout strings.Builder
	textErr := run(t.Context(), wd, []string{"init", "--host", "none"}, nil, &textStdout, &textStderr, noBuildInfo, seam)
	require.Error(t, textErr)
	var stderr strings.Builder

	err := run(t.Context(), wd, []string{"init", "--host", "none", "--json"}, nil, failingWriter{}, &stderr, noBuildInfo, seam)

	assert.Equal(t, 1, ExitCode(err))
	assert.Equal(t, textStderr.String(), stderr.String())
}

func Test_help_json_reports_a_stdout_write_failure_on_stderr(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "help --json", args: []string{"help", "--json"}, want: wantReadFailure("help", "brief help --json")},
		{name: "--help --json", args: []string{"--help", "--json"}, want: wantReadFailure("help", "brief help --json")},
		{name: "help <command> --json", args: []string{"help", "status", "--json"}, want: wantReadFailure("help", "brief help status --json")},
		{name: "<command> --help --json", args: []string{"status", "--help", "--json"}, want: wantReadFailure("help", "brief help status --json")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stderr strings.Builder

			err := run(t.Context(), memRoot, c.args, nil, failingWriter{}, &stderr, noBuildInfo)

			assert.Equal(t, 1, ExitCode(err))
			assert.Equal(t, c.want, stderr.String())
		})
	}
}
