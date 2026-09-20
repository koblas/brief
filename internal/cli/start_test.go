package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	assert.Contains(t, lines[0], filepath.Join(featureDir, "specification.md"))
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
	assert.Contains(t, lines[0], filepath.Join(featureDir, "SCENARIO-01.md"))

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
	assert.Contains(t, lines[0], filepath.Join(featureDir, "STATE.md"))

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
	assert.Contains(t, lines[0], filepath.Join(featureDir, "SCENARIO-01.md"))
	assert.Contains(t, lines[0], "## Scenario")
	assert.Contains(t, lines[1], filepath.Join(featureDir, "STATE.md"))
	assert.Contains(t, lines[1], "## Binding decisions")
	assert.Contains(t, lines[2], filepath.Join(featureDir, "STATE.md"))
	assert.Contains(t, lines[2], "## Left unbuilt")
	assert.Contains(t, lines[3], filepath.Join(featureDir, "STATE.md"))
	assert.Contains(t, lines[3], "## Traps")
	assert.Contains(t, lines[4], filepath.Join(featureDir, "STATE.md"))
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
	assert.Contains(t, lines[0], filepath.Join(featureDir, "STATE.md"))
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
		"brief start: "+filepath.Join(wd, "docs", "specifications", "demo")+
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
		"brief start: "+filepath.Join(wd, "docs", "specifications", "demo")+
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
	assert.Equal(t, "brief start: flag provided but not defined: -bogus; run 'brief start <feature>'\n", stderr.String())
}

func Test_prints_the_start_usage_for_help(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "--help"}, nil, &stdout, &stderr)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())
	assert.Contains(t, stdout.String(), "brief start reads; it never writes.")
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
	assert.Contains(t, stderr.String(), filepath.Join(featureDir, "STATE.md"))
}

func Test_returns_an_error_for_an_unknown_feature_on_start(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer

	err := cli.Run(t.Context(), wd, []string{"start", "ghost"}, nil, &stdout, &stderr)

	assert.Equal(t, 1, cli.ExitCode(err))
	assert.Empty(t, stdout.String())
	assert.NotEmpty(t, stderr.String())
}
