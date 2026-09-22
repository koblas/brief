package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newStartFixture writes one step, "SCENARIO-01", for feature "demo" under
// the default profile's layout, with its frontmatter status field set to
// status, and returns the working directory Run should be called with.
func newStartFixture(t *testing.T, status string) string {
	t.Helper()

	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: " + status + "\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Scenario\n\n" +
		"the acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		"- [ ] do the thing\n\n" +
		"## Handoff\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte(step), 0o600))

	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(state), 0o600))

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))

	return wd
}

// Test_start_refuses_a_specification_with_no_progress_heading is
// SCENARIO-13's "no partial brief" proof: it differs from
// Test_prints_the_brief_and_writes_nothing_to_stderr's control arm in
// exactly one variable — the specification body has no progress heading —
// so a mutation that deleted the progress-heading check entirely would
// make this test see the same 46-byte brief that control arm proves exists
// on stdout at exit 0, not the stdout-empty refusal this test asserts.
func Test_start_refuses_a_specification_with_no_progress_heading(t *testing.T) {
	wd := newStartFixture(t, "open")
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	spec := "# demo\n\nno progress list here\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Zero(t, stdout.Len())

	lines := strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n")
	require.Len(t, lines, 1)
	assert.NotContains(t, lines[0], "(no files changed)")
	assert.Contains(t, lines[0], filepath.Join("docs", "specifications", "demo", "specification.md"))
	assert.Contains(t, lines[0], "## BDD Acceptance Progress")
}

// Test_start_names_an_absent_acceptance_heading_and_still_prints_the_brief
// is SCENARIO-14's core case: a step file missing only its "## Scenario"
// heading degrades rather than refuses. It reuses newStartFixture for a
// baseline run, then overwrites SCENARIO-01.md with the same content minus
// its acceptance section — never editing newStartFixture itself, which
// every other test in this file shares.
func Test_start_names_an_absent_acceptance_heading_and_still_prints_the_brief(t *testing.T) {
	baselineWD := newStartFixture(t, "open")
	var baselineStdout, baselineStderr bytes.Buffer
	require.NoError(t, cli.Run(t.Context(), baselineWD, []string{"start", "demo"}, nil, &baselineStdout, &baselineStderr))

	wd := newStartFixture(t, "open")
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Implementation Plan\n\n" +
		"- [ ] do the thing\n\n" +
		"## Handoff\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte(step), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n")
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], filepath.Join("docs", "specifications", "demo", "SCENARIO-01.md"))

	wantStdout := strings.Replace(baselineStdout.String(), "\n## Scenario\n\nthe acceptance criteria\n", "", 1)
	assert.Equal(t, wantStdout, stdout.String())
}

// Test_start_names_an_absent_state_heading_and_still_prints_the_brief is the
// state-file twin of the acceptance case above: STATE.md is overwritten
// without its "## Traps" section, and the brief still prints minus that
// one block.
func Test_start_names_an_absent_state_heading_and_still_prints_the_brief(t *testing.T) {
	baselineWD := newStartFixture(t, "open")
	var baselineStdout, baselineStderr bytes.Buffer
	require.NoError(t, cli.Run(t.Context(), baselineWD, []string{"start", "demo"}, nil, &baselineStdout, &baselineStderr))

	wd := newStartFixture(t, "open")
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Open debts\n\na debt\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(state), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n")
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], filepath.Join("docs", "specifications", "demo", "STATE.md"))

	wantStdout := strings.Replace(baselineStdout.String(), "\n## Traps\n\na trap\n", "", 1)
	assert.Equal(t, wantStdout, stdout.String())
}

// Test_start_names_every_absent_convention_on_its_own_line is the
// one-line-per-shortfall and ordering proof: STATE.md is replaced by prose
// carrying none of the four required headings and the step's "## Scenario"
// heading is dropped too, so all five conventions are missing at once.
// Order is pinned: acceptance, then binding decisions, left unbuilt,
// traps, open debts — the same order RenderText emits sections in.
func Test_start_names_every_absent_convention_on_its_own_line(t *testing.T) {
	baselineWD := newStartFixture(t, "open")
	var baselineStdout, baselineStderr bytes.Buffer
	require.NoError(t, cli.Run(t.Context(), baselineWD, []string{"start", "demo"}, nil, &baselineStdout, &baselineStderr))

	wd := newStartFixture(t, "open")
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Implementation Plan\n\n" +
		"- [ ] do the thing\n\n" +
		"## Handoff\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte(step), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte("just some prose, no headings here\n"), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n")
	require.Len(t, lines, 5)
	assert.Contains(t, lines[0], filepath.Join("docs", "specifications", "demo", "SCENARIO-01.md"))
	assert.Contains(t, lines[0], "## Scenario")
	assert.Contains(t, lines[1], filepath.Join("docs", "specifications", "demo", "STATE.md"))
	assert.Contains(t, lines[1], "## Binding decisions")
	assert.Contains(t, lines[2], filepath.Join("docs", "specifications", "demo", "STATE.md"))
	assert.Contains(t, lines[2], "## Left unbuilt")
	assert.Contains(t, lines[3], filepath.Join("docs", "specifications", "demo", "STATE.md"))
	assert.Contains(t, lines[3], "## Traps")
	assert.Contains(t, lines[4], filepath.Join("docs", "specifications", "demo", "STATE.md"))
	assert.Contains(t, lines[4], "## Open debts")

	wantStdout := baselineStdout.String()
	for _, block := range []string{
		"\n## Scenario\n\nthe acceptance criteria\n",
		"\n## Binding decisions\n\nsome decision\n",
		"\n## Left unbuilt\n\nsomething left\n",
		"\n## Traps\n\na trap\n",
		"\n## Open debts\n\na debt\n",
	} {
		wantStdout = strings.Replace(wantStdout, block, "", 1)
	}

	assert.Equal(t, wantStdout, stdout.String())
}

// Test_start_says_nothing_about_a_present_but_empty_convention is the
// !Found vs Body == "" discriminator: every heading is present but carries
// no body, differing from the previous test in exactly one variable — the
// headings are there. No shortfall may fire.
func Test_start_says_nothing_about_a_present_but_empty_convention(t *testing.T) {
	wd := newStartFixture(t, "open")
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Scenario\n\n" +
		"## Implementation Plan\n\n" +
		"- [ ] do the thing\n\n" +
		"## Handoff\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte(step), 0o600))
	state := "## Binding decisions\n\n## Left unbuilt\n\n## Traps\n\n## Open debts\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(state), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
}

// Test_start_on_a_freshly_scaffolded_feature_names_only_the_absent_acceptance_heading
// is the end-to-end proof of the !Found rule on the shape "new feature" and
// "new step" actually produce: stateSkeleton writes all four state
// headings bare, so only the step's missing acceptance heading may fire —
// a Body == "" implementation would emit five lines here instead of one.
func Test_start_on_a_freshly_scaffolded_feature_names_only_the_absent_acceptance_heading(t *testing.T) {
	wd := t.TempDir()
	var discard bytes.Buffer

	require.NoError(t, cli.Run(t.Context(), wd, []string{"new", "feature", "demo"}, nil, &discard, &discard))
	require.NoError(t, cli.Run(t.Context(), wd, []string{"new", "step", "demo"}, nil, &discard, &discard))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.NotEmpty(t, stdout.String())

	lines := strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n")
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "## Scenario")
}

// Test_start_still_refuses_a_state_file_whose_fence_is_unterminated is the
// control arm keeping SCENARIO-13's refusal and SCENARIO-14's degrade
// apart: the same state file, present and readable, with one variable
// different from a conforming fixture — an unclosed fence instead of an
// absent heading. Green on arrival: SCENARIO-13 already refuses this.
func Test_start_still_refuses_a_state_file_whose_fence_is_unterminated(t *testing.T) {
	wd := newStartFixture(t, "open")
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	state := "```\nunterminated\n## Binding decisions\n\nsome decision\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(state), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())

	lines := strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n")
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], filepath.Join("docs", "specifications", "demo", "STATE.md"))
}

func Test_prints_the_brief_and_writes_nothing_to_stderr(t *testing.T) {
	wd := newStartFixture(t, "open")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.NotEmpty(t, stdout.String())
	assert.Contains(t, stdout.String(), "SCENARIO-01 — 0 done, 1 open")
	assert.Contains(t, stdout.String(), "the acceptance criteria")
	assert.Contains(t, stdout.String(), "some decision")
}

// Test_start_says_the_feature_is_complete_when_every_step_is_done is the
// R14 "the feature has no next step because it is finished" case: the same
// newStartFixture as the open-step control arm above, differing in exactly
// one variable — the frontmatter status field.
func Test_start_says_the_feature_is_complete_when_every_step_is_done(t *testing.T) {
	wd := newStartFixture(t, "done")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t,
		"brief start: "+filepath.Join("docs", "specifications", "demo")+
			": feature is complete, 1 of 1 steps done; run 'brief new step demo' to add the next one\n",
		stderr.String())
}

// Test_start_says_there_are_no_step_files_yet_for_an_empty_feature is R14's
// second discriminated case: a feature directory holding a state file but
// no step files at all, the shape a bare "brief new feature demo" leaves
// behind. It hits the same nil-Step branch as the all-done case above but
// must not be told "complete" — SCENARIO-11 already ruled this directory
// conforming, not malformed.
func Test_start_says_there_are_no_step_files_yet_for_an_empty_feature(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(state), 0o600))
	spec := "# demo\n\n## BDD Acceptance Progress\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stdout.String())
	assert.Equal(t,
		"brief start: "+filepath.Join("docs", "specifications", "demo")+
			": no step files yet; run 'brief new step demo' to scaffold the first one\n",
		stderr.String())
}

// Test_prints_the_full_checklist_when_it_contains_a_nested_fence
// reproduces the reviewer's read-path finding: markdown.Section's old
// boolean fence toggle let a ``` line close an open ~~~ block, so the
// "## Not a heading" line inside it was read as the real terminating
// heading and everything after it — here "- [ ] real task" — was silently
// dropped from brief start's output, with exit 0 and nothing on stderr.
func Test_prints_the_full_checklist_when_it_contains_a_nested_fence(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Scenario\n\n" +
		"the acceptance criteria\n\n" +
		"## Implementation Plan\n\n" +
		"~~~\n```\n## Not a heading\n~~~\n\n" +
		"- [ ] real task\n\n" +
		"## Handoff\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte(step), 0o600))

	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(state), 0o600))

	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Contains(t, stdout.String(), "real task")
}

// Test_prints_the_brief_from_a_CRLF_step_file reproduces the reviewer's
// R10 finding: under core.autocrlf=true, strings.Split(body, "\n") leaves
// a trailing "\r" on every heading line, and the old equality check
// right-trimmed only " \t", so no heading — not the title, not the
// acceptance criteria, not the checklist, not one of the four state
// sections — was ever found. brief start used to print only the bare
// "<id> — <done> done, <open> open" line: 47 bytes, exit 0, nothing on
// stderr, none of R10's required sections. The frontmatter delimiter
// itself stays LF here — YAML frontmatter's own CRLF tolerance is
// stepfile.ParseFrontmatter's contract, not named by this finding, and
// stepSkeleton always writes it with a literal "\n".
func Test_prints_the_brief_from_a_CRLF_step_file(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\r\n\r\n" +
		"## Scenario\r\n\r\n" +
		"the acceptance criteria\r\n\r\n" +
		"## Implementation Plan\r\n\r\n" +
		"- [ ] do the thing\r\n\r\n" +
		"## Handoff\r\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte(step), 0o600))

	state := "## Binding decisions\r\n\r\nsome decision\r\n\r\n" +
		"## Left unbuilt\r\n\r\nsomething left\r\n\r\n" +
		"## Traps\r\n\r\na trap\r\n\r\n" +
		"## Open debts\r\n\r\na debt\r\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(state), 0o600))

	spec := "# demo\r\n\r\n## BDD Acceptance Progress\r\n\r\n- [ ] SCENARIO-01\r\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))

	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Contains(t, stdout.String(), "SCENARIO-01 Demo step")
	assert.Contains(t, stdout.String(), "the acceptance criteria")
	assert.Contains(t, stdout.String(), "do the thing")
	assert.Contains(t, stdout.String(), "some decision")
	assert.Contains(t, stdout.String(), "something left")
	assert.Contains(t, stdout.String(), "a trap")
	assert.Contains(t, stdout.String(), "a debt")
}

func Test_returns_a_usage_error_when_no_feature_is_given_to_start(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief start: no feature given; run 'brief start <feature>'\n", stderr.String())
}

func Test_returns_a_usage_error_when_start_is_given_too_many_arguments(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "a", "b"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief start: too many arguments; run 'brief start <feature>'\n", stderr.String())
}

func Test_returns_a_usage_error_for_an_unknown_start_flag(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--bogus", "demo"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief start: unknown flag: --bogus; run 'brief start <feature>'\n", stderr.String())
}

func Test_prints_the_start_usage_for_help(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Contains(t, stdout.String(), "brief start reads; it never writes.")
}

// Test_returns_a_usage_error_when_help_precedes_an_undefined_flag and its
// control arm below pin that an undefined flag is a usage error regardless
// of where --help falls relative to it: flag parsing rejects --bogus before
// either position of --help is ever considered.
func Test_returns_a_usage_error_when_help_precedes_an_undefined_flag(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--help", "--bogus"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief start: unknown flag: --bogus; run 'brief start <feature>'\n", stderr.String())
}

func Test_returns_a_usage_error_when_an_undefined_flag_precedes_help(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--bogus", "--help"}, nil, &stdout, &stderr)

	require.ErrorIs(t, err, cli.ErrUsage)
	assert.Empty(t, stdout.String())
	assert.Equal(t, "brief start: unknown flag: --bogus; run 'brief start <feature>'\n", stderr.String())
}

// Test_start_still_refuses_a_feature_with_no_state_file is the control arm
// proving the nil-Step notice above did not swallow the pre-existing
// malformed-feature refusal: a feature directory with a step file but no
// state file still exits 1 with a message on stderr, never the "no step
// files yet" notice or a silent success.
func Test_start_still_refuses_a_feature_with_no_state_file(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte(step), 0o600))
	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), filepath.Join("docs", "specifications", "demo", "STATE.md"))
}

func Test_returns_an_error_for_an_unknown_feature_on_start(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "ghost"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.NotEmpty(t, stderr.String())
}

// writeStartMalformedFixture writes a conforming specification and state
// file for "demo" under wd's default layout, plus one step file,
// SCENARIO-01.md, with no frontmatter at all — the SCENARIO-04 fixture
// every subtest of Test_start_names_the_malformed_step_file_relative_to_the_working_directory
// shares.
func writeStartMalformedFixture(t *testing.T, featureDir string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	spec := "# demo\n\n## BDD Acceptance Progress\n\n- [ ] SCENARIO-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(""), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte("no frontmatter here\n"), 0o600))
}

// Test_start_names_the_malformed_step_file_relative_to_the_working_directory
// is SCENARIO-04's CLI slice: a step file with no frontmatter is named by
// its own path, relative to the working directory — not the feature
// directory — with a fix pointing at 'brief check'. The json subtest pins
// R6's split on the same fixture: "path" stays absolute while "message" is
// byte-identical to the text-mode line. The subdirectory subtest pins that
// a working directory below the config root renders a "../"-prefixed path.
func Test_start_names_the_malformed_step_file_relative_to_the_working_directory(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		wd := t.TempDir()
		featureDir := filepath.Join(wd, "docs", "specifications", "demo")
		writeStartMalformedFixture(t, featureDir)
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

		assert.Equal(t, 1, cli.ExitCode(err))
		assert.Empty(t, stdout.String())
		assert.Equal(t,
			"brief start: "+filepath.Join("docs", "specifications", "demo", "SCENARIO-01.md")+
				": frontmatter does not parse: no frontmatter found; run 'brief check demo' to list every fault\n",
			stderr.String())
	})

	t.Run("json", func(t *testing.T) {
		wd := t.TempDir()
		featureDir := filepath.Join(wd, "docs", "specifications", "demo")
		writeStartMalformedFixture(t, featureDir)
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"start", "--json", "demo"}, nil, &stdout, &stderr)

		assert.Equal(t, 1, cli.ExitCode(err))
		assert.Empty(t, stderr.String())

		got := decodeErrorDocument(t, stdout.Bytes(), "start")
		require.NotNil(t, got.Path)
		assert.Equal(t, filepath.Join(featureDir, "SCENARIO-01.md"), *got.Path)
		assert.Equal(t,
			"brief start: "+filepath.Join("docs", "specifications", "demo", "SCENARIO-01.md")+
				": frontmatter does not parse: no frontmatter found; run 'brief check demo' to list every fault",
			got.Message)
	})

	t.Run("from a subdirectory", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, ".brief.yaml"), []byte(""), 0o600))
		featureDir := filepath.Join(root, "docs", "specifications", "demo")
		writeStartMalformedFixture(t, featureDir)
		wd := filepath.Join(root, "a")
		require.NoError(t, os.MkdirAll(wd, 0o755))
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

		assert.Equal(t, 1, cli.ExitCode(err))
		assert.Equal(t,
			"brief start: "+filepath.Join("..", "docs", "specifications", "demo", "SCENARIO-01.md")+
				": frontmatter does not parse: no frontmatter found; run 'brief check demo' to list every fault\n",
			stderr.String())
	})
}

// startJSONDocument is start's --json success document, decoded field by
// field in the tests below: the common header (schema, command, ok,
// exit_code) precedes assemble.Brief's own fields, flattened by embedding.
type startJSONDocument struct {
	assemble.Brief

	Schema   int    `json:"schema"`
	Command  string `json:"command"`
	OK       bool   `json:"ok"`
	ExitCode int    `json:"exit_code"`
}

// Test_start_json_success_document_golden is SCENARIO-03's exact-bytes
// proof: the open-step fixture with no shortfalls, decoded as nothing more
// than the header's four fields prepended to the same payload
// Test_start_without_json_is_byte_identical_to_the_text_brief's control arm
// renders as markdown, key order pinned so a later field reordering in
// either jsonHeader or assemble.Brief is caught here rather than by an
// unmarshal that would silently accept it.
func Test_start_json_success_document_golden(t *testing.T) {
	wd := newStartFixture(t, "open")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--json", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	want := `{"schema":1,"command":"start","ok":true,"exit_code":0,` +
		`"done":0,"open":1,` +
		`"step":{"id":"SCENARIO-01","title":"SCENARIO-01 Demo step",` +
		`"acceptance":{"heading":"## Scenario","body":"the acceptance criteria","found":true},` +
		`"checklist":{"heading":"## Implementation Plan","body":"- [ ] do the thing","found":true}},` +
		`"inherited":[` +
		`{"heading":"## Binding decisions","body":"some decision","found":true},` +
		`{"heading":"## Left unbuilt","body":"something left","found":true},` +
		`{"heading":"## Traps","body":"a trap","found":true},` +
		`{"heading":"## Open debts","body":"a debt","found":true}],` +
		`"shortfalls":null}` + "\n"

	assert.Equal(t, want, stdout.String())
}

// Test_start_prints_the_brief_as_json_when_asked is SCENARIO-03's core
// case: --json against newStartFixture's open-step feature decodes to the
// common header (schema 1, command "start", ok true, exit_code 0) plus the
// same Brief fields RenderText already proves through
// Test_prints_the_brief_and_writes_nothing_to_stderr, with stderr still
// empty and the command still succeeding.
func Test_start_prints_the_brief_as_json_when_asked(t *testing.T) {
	wd := newStartFixture(t, "open")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--json", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	var got startJSONDocument
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))

	assert.Equal(t, 1, got.Schema)
	assert.Equal(t, "start", got.Command)
	assert.True(t, got.OK)
	assert.Equal(t, 0, got.ExitCode)

	require.NotNil(t, got.Step)
	assert.Equal(t, 0, got.Done)
	assert.Equal(t, 1, got.Open)
	assert.Equal(t, "SCENARIO-01", got.Step.ID)
	assert.Equal(t, "the acceptance criteria", got.Step.Acceptance.Body)
	assert.Equal(t, "- [ ] do the thing", got.Step.Checklist.Body)
	assert.Equal(t, "some decision", got.Inherited[0].Body)
}

// Test_start_json_emits_a_null_step_for_a_completed_feature asserts the
// raw stdout bytes carry "step":null and a non-zero "done" for a feature
// whose only step is done — unmarshalling would collapse null and absent
// into the same value, so the claim is made on bytes. Under --json the
// text-mode "feature is complete" notice is not written anywhere (R1):
// step:null plus a non-zero done is the document's own discriminator, so
// stderr stays empty.
func Test_start_json_emits_a_null_step_for_a_completed_feature(t *testing.T) {
	wd := newStartFixture(t, "done")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--json", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Contains(t, stdout.String(), `"step":null`)
	assert.Contains(t, stdout.String(), `"done":1`)

	var got startJSONDocument
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
	assert.Equal(t, "start", got.Command)
	assert.True(t, got.OK)
}

// Test_start_json_emits_zero_counts_for_a_feature_with_no_step_files is
// R14's second discriminated case under --json: a feature directory with
// no step files at all still writes a document, with "done" and "open"
// both zero so a structured caller can tell it apart from a completed
// feature — both share "step":null. Under --json the text-mode "no step
// files yet" notice is not written anywhere (R1): stderr stays empty.
func Test_start_json_emits_zero_counts_for_a_feature_with_no_step_files(t *testing.T) {
	wd := t.TempDir()
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	state := "## Binding decisions\n\nsome decision\n\n" +
		"## Left unbuilt\n\nsomething left\n\n" +
		"## Traps\n\na trap\n\n" +
		"## Open debts\n\na debt\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STATE.md"), []byte(state), 0o600))
	spec := "# demo\n\n## BDD Acceptance Progress\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--json", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Contains(t, stdout.String(), `"step":null`)
	assert.Contains(t, stdout.String(), `"done":0`)
	assert.Contains(t, stdout.String(), `"open":0`)

	var got startJSONDocument
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
	assert.Equal(t, "start", got.Command)
	assert.True(t, got.OK)
}

// Test_start_json_keeps_stdout_parseable_when_a_convention_is_missing is
// SCENARIO-03's shortfall case under --json: the shortfall is already
// payload (Brief.Shortfalls), so the text-mode stderr line for it is
// dropped rather than duplicated — stderr is empty, stdout stays a single
// parseable document carrying the shortfall with an absolute path (R6).
func Test_start_json_keeps_stdout_parseable_when_a_convention_is_missing(t *testing.T) {
	wd := newStartFixture(t, "open")
	featureDir := filepath.Join(wd, "docs", "specifications", "demo")
	step := "---\n" +
		"id: SCENARIO-01\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n\n" +
		"# SCENARIO-01 Demo step\n\n" +
		"## Implementation Plan\n\n" +
		"- [ ] do the thing\n\n" +
		"## Handoff\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "SCENARIO-01.md"), []byte(step), 0o600))
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--json", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	stdoutLines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	require.Len(t, stdoutLines, 1)

	var got assemble.Brief
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
	assert.False(t, got.Step.Acceptance.Found)
	require.Len(t, got.Shortfalls, 1)
	assert.Contains(t, got.Shortfalls[0].Path, "SCENARIO-01.md")
	assert.True(t, filepath.IsAbs(got.Shortfalls[0].Path))
}

// Test_start_json_writes_the_error_document_when_it_refuses asserts a
// refusal under --json renders R3's error document on stdout, empty
// stderr, exit 1 — not R14a's plain stderr line: runStart calls Start
// before it renders anything, so its refusal reaches out.refusal the same
// way every other command's does (SCENARIO-02).
func Test_start_json_writes_the_error_document_when_it_refuses(t *testing.T) {
	t.Run("malformed feature", func(t *testing.T) {
		wd := newStartFixture(t, "open")
		featureDir := filepath.Join(wd, "docs", "specifications", "demo")
		spec := "# demo\n\nno progress list here\n"
		require.NoError(t, os.WriteFile(filepath.Join(featureDir, "specification.md"), []byte(spec), 0o600))
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"start", "--json", "demo"}, nil, &stdout, &stderr)

		assert.Equal(t, 1, cli.ExitCode(err))
		assert.Empty(t, stderr.String())

		got := decodeErrorDocument(t, stdout.Bytes(), "start")
		assert.Equal(t, "refusal", got.Kind)
		assert.NotContains(t, got.Message, "(no files changed)")
	})

	t.Run("unknown feature", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"start", "--json", "ghost"}, nil, &stdout, &stderr)

		assert.Equal(t, 1, cli.ExitCode(err))
		assert.Empty(t, stderr.String())

		got := decodeErrorDocument(t, stdout.Bytes(), "start")
		assert.Equal(t, "refusal", got.Kind)
		assert.NotEmpty(t, got.Message)
	})
}

// Test_start_json_still_reports_usage_errors asserts --json turns start's
// own usage errors into R3's error document (SCENARIO-01), not stdout
// text: a missing feature, and an undefined flag alongside --json, both
// still exit 2, now with the text-mode line carried as the document's
// "error.message" and stderr empty.
func Test_start_json_still_reports_usage_errors(t *testing.T) {
	t.Run("no feature given", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"start", "--json"}, nil, &stdout, &stderr)

		require.ErrorIs(t, err, cli.ErrUsage)
		assert.Equal(t, 2, cli.ExitCode(err))
		assert.Empty(t, stderr.String())

		message, _ := decodeUsageErrorDocument(t, stdout.Bytes(), "start", nil)
		assert.Equal(t, "brief start: no feature given; run 'brief start <feature>'", message)
	})

	t.Run("undefined flag alongside json", func(t *testing.T) {
		wd := t.TempDir()
		var stdout, stderr bytes.Buffer

		err := cli.Run(t.Context(), wd, []string{"start", "--bogus", "--json", "demo"}, nil, &stdout, &stderr)

		require.ErrorIs(t, err, cli.ErrUsage)
		assert.Equal(t, 2, cli.ExitCode(err))
		assert.Empty(t, stderr.String())

		message, _ := decodeUsageErrorDocument(t, stdout.Bytes(), "start", nil)
		assert.Equal(t, "brief start: unknown flag: --bogus; run 'brief start <feature>'", message)
	})
}

// Test_start_accepts_the_json_flag_after_the_feature asserts --json works
// in either argument position: "demo --json" produces byte-identical
// output to "--json demo".
func Test_start_accepts_the_json_flag_after_the_feature(t *testing.T) {
	wdBefore := newStartFixture(t, "open")
	var beforeOut, beforeErr bytes.Buffer
	require.NoError(t, cli.Run(t.Context(), wdBefore, []string{"start", "--json", "demo"}, nil, &beforeOut, &beforeErr))

	wdAfter := newStartFixture(t, "open")
	var afterOut, afterErr bytes.Buffer

	err := cli.Run(t.Context(), wdAfter, []string{"start", "demo", "--json"}, nil, &afterOut, &afterErr)

	require.NoError(t, err)
	assert.Equal(t, beforeOut.String(), afterOut.String())
	assert.Equal(t, beforeErr.String(), afterErr.String())
}

// Test_start_json_help_prints_usage_not_json asserts --json --help prints
// start's help to stdout rather than a JSON document — help wins
// regardless of --json's position or value.
func Test_start_json_help_prints_usage_not_json(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--json", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Contains(t, stdout.String(), "brief start reads; it never writes.")
}

// Test_start_without_json_is_byte_identical_to_the_text_brief pins that
// 15 left the non-JSON path untouched: the same fixture, without --json,
// still produces exactly the markdown RenderText produced before this
// scenario.
func Test_start_without_json_is_byte_identical_to_the_text_brief(t *testing.T) {
	wd := newStartFixture(t, "open")
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "demo"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Equal(t, ""+
		"SCENARIO-01 — 0 done, 1 open\n\n"+
		"# SCENARIO-01 Demo step\n\n"+
		"## Scenario\n\n"+
		"the acceptance criteria\n\n"+
		"## Implementation Plan\n\n"+
		"- [ ] do the thing\n\n"+
		"## Binding decisions\n\n"+
		"some decision\n\n"+
		"## Left unbuilt\n\n"+
		"something left\n\n"+
		"## Traps\n\n"+
		"a trap\n\n"+
		"## Open debts\n\n"+
		"a debt\n",
		stdout.String())
}
