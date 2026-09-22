package scaffold_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// snapshotTree returns the contents of every regular file directly under
// dir, keyed by name, so a test can prove a refusal left the directory
// byte-identical by comparing two snapshots.
func snapshotTree(t *testing.T, dir string) map[string][]byte {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	snap := make(map[string][]byte, len(entries))

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		snap[e.Name()] = data
	}

	return snap
}

// finishFixture is the on-disk feature newFinishFixture builds, plus the
// two inputs a Finish call under test supplies.
type finishFixture struct {
	root       string
	cfg        config.Config
	newHandoff []byte
	newState   []byte
}

// featureDir returns the path of the fixture's "widgets" feature directory.
func (fx finishFixture) featureDir() string {
	return filepath.Join(fx.root, fx.cfg.FeatureDirectory, "widgets")
}

// stepPath returns the path of one of the fixture's step files.
func (fx finishFixture) stepPath(name string) string {
	return filepath.Join(fx.featureDir(), name)
}

// handoffPath returns the path of the fixture's target step's handoff
// file, deriving the name by literal string concatenation rather than
// through stepfile.CompileHandoff, so a production naming bug cannot hide
// behind the test's own derivation.
func (fx finishFixture) handoffPath() string {
	return filepath.Join(fx.featureDir(), "STEP-02"+fx.cfg.HandoffFileSuffix)
}

// oldStateBody is NOTES.md's body before Finish runs: the four configured
// state headings, each carrying a marker distinct from newState's, so no
// "unchanged" assertion in a test built against this fixture can pass
// vacuously.
func oldStateBody(cfg config.Config) string {
	var body strings.Builder
	for _, h := range cfg.StateHeadings.Ordered() {
		body.WriteString(h + "\n\nOLD-STATE-ENTRY\n\n")
	}

	return body.String()
}

// newStateBody is the replacement state body Finish is asked to write: the
// same four headings, each carrying a marker distinct from oldStateBody's.
func newStateBody(cfg config.Config) []byte {
	var body strings.Builder
	for _, h := range cfg.StateHeadings.Ordered() {
		body.WriteString(h + "\n\nNEW-STATE-ENTRY\n\n")
	}

	return []byte(body.String())
}

// differentStateBody is a replacement state body carrying the four
// configured headings, each holding a marker distinct from both
// oldStateBody's and newStateBody's, for tests that need a state argument
// that both passes SCENARIO-19's heading check and still diverges from
// whatever is already recorded on disk.
func differentStateBody(cfg config.Config) []byte {
	var body strings.Builder
	for _, h := range cfg.StateHeadings.Ordered() {
		body.WriteString(h + "\n\nDIFFERENT-STATE-ENTRY\n\n")
	}

	return []byte(body.String())
}

// step02Body is the target step's body, in stepSkeleton's shape plus one
// key Frontmatter does not know (owner: planner) and a depends-on
// satisfied by STEP-01. Its checklist is fully ticked and holds a fenced
// block containing a decoy "## Fixture Handoff" line, so a naive anchor
// search or a checklist-completeness bug would misfire on this fixture
// rather than on a contrived edge case. A bare handoff heading is followed
// by a legacy "## Notes" section, so every "preserved outside the span"
// assertion built against this fixture has content to lose.
func step02Body(cfg config.Config) string {
	return "---\n" +
		"id: STEP-02\n" +
		"status: open\n" +
		"depends-on: [STEP-01]\n" +
		"owner: planner\n" +
		"---\n" +
		"\n" +
		"# STEP-02 Assemble the thing\n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		"- [x] first thing\n" +
		"- [x] second thing\n" +
		"\n" +
		"```\n" +
		"fenced decoy\n" +
		"## Fixture Handoff" + "\n" +
		"not a real anchor\n" +
		"```\n" +
		"\n" +
		"## Fixture Handoff" + "\n" +
		"\n" +
		"## Notes\n" +
		"\n" +
		"PLEASE KEEP THIS\n"
}

// newFinishFixture builds a "widgets" feature under t.TempDir() with three
// step files (STEP-01 done, STEP-02 the target, STEP-03 untouched), a
// state file and a specification carrying a progress entry for each step,
// using fixtureConfig so every field this package reads differs from
// config.Default(). It returns the fixture together with the handoff and
// state bytes a happy-path Finish("widgets", "STEP-02", ...) call supplies.
func newFinishFixture(t *testing.T) finishFixture {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	fx := finishFixture{root: root, cfg: cfg}

	require.NoError(t, os.MkdirAll(fx.featureDir(), 0o755))

	step01 := "---\n" +
		"id: STEP-01\n" +
		"status: done\n" +
		"depends-on: []\n" +
		"---\n" +
		"\n" +
		"# STEP-01\n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		"- [x] did the first thing\n" +
		"\n" +
		"## Fixture Handoff" + "\n" +
		"\n" +
		"OLD-HANDOFF-01\n"
	require.NoError(t, os.WriteFile(fx.stepPath("STEP-01.md"), []byte(step01), 0o600))

	require.NoError(t, os.WriteFile(fx.stepPath("STEP-02.md"), []byte(step02Body(cfg)), 0o600))

	step03 := "---\n" +
		"id: STEP-03\n" +
		"status: open\n" +
		"depends-on: []\n" +
		"---\n" +
		"\n" +
		"# STEP-03\n" +
		"\n" +
		cfg.ChecklistHeading + "\n" +
		"\n" +
		"- [ ] not done yet\n" +
		"\n" +
		"## Fixture Handoff" + "\n"
	require.NoError(t, os.WriteFile(fx.stepPath("STEP-03.md"), []byte(step03), 0o600))

	require.NoError(t, os.WriteFile(filepath.Join(fx.featureDir(), cfg.StateFile), []byte(oldStateBody(cfg)), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n" +
		"- [x] STEP-01: already done\n" +
		"- [ ] STEP-02: Assemble the thing\n" +
		"- [ ] STEP-03\n" +
		"\n" +
		"## Notes\n" +
		"\n" +
		"Something else entirely.\n"
	require.NoError(t, os.WriteFile(filepath.Join(fx.featureDir(), cfg.SpecificationFile), []byte(spec), 0o600))

	fx.newHandoff = []byte("NEW-HANDOFF-02\n\n```\n# not a heading\n```\n")
	fx.newState = newStateBody(cfg)

	return fx
}

// Test_writes_the_supplied_handoff_to_its_own_file pins the amended
// SCENARIO-05 contract directly: the handoff no longer lives inside the
// step file, it lives at handoffPath(), and it holds fx.newHandoff
// verbatim — including its trailing newline and its nested fence.
func Test_writes_the_supplied_handoff_to_its_own_file(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(fx.handoffPath())
	require.NoError(t, readErr)

	assert.Equal(t, string(fx.newHandoff), string(got))
}

// Test_leaves_the_step_file_byte_identical_apart_from_the_status_line
// replaces Test_leaves_the_rest_of_the_step_file_byte_identical: this
// single assertion is R21's second clause at the Server level — the
// legacy-"## Notes"-section preservation proof — and the assertion the old
// splice could never pass.
func Test_leaves_the_step_file_byte_identical_apart_from_the_status_line(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)

	want := strings.Replace(step02Body(fx.cfg), "status: open\n", "status: done\n", 1)

	assert.Equal(t, want, string(got))
}

// Test_leaves_the_specification_byte_identical_apart_from_the_finished_step_s_progress_line
// proves the trailing "## Notes" section in the specification survives
// finish's progress-checkbox edit untouched.
func Test_leaves_the_specification_byte_identical_apart_from_the_finished_step_s_progress_line(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	specPath := filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile)
	before, readErr := os.ReadFile(specPath)
	require.NoError(t, readErr)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(specPath)
	require.NoError(t, readErr)

	want := strings.Replace(string(before), "- [ ] STEP-02: Assemble the thing\n", "- [x] STEP-02: Assemble the thing\n", 1)

	assert.Equal(t, want, string(got))
}

func Test_replaces_the_state_file_with_exactly_the_supplied_body(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, readErr)

	assert.Equal(t, string(fx.newState), string(got))
}

func Test_marks_the_step_done_in_its_frontmatter(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)

	fm, _, parseErr := stepfile.ParseFrontmatter(got)
	require.NoError(t, parseErr)

	assert.True(t, fm.Done())
}

// Test_finish_refuses_a_replacement_state_body_with_an_unterminated_fence
// is the last surviving argument-fence check: a replacement state body
// whose fence never closes would leave every configured state heading
// unreadable on the next Start, since assemble.stateSections finds each
// one by scanning forward for a terminator. Named against
// scaffold.StateSource, since Finish never learns the argument's real
// path — a caller that does, such as cli's --state flag, upgrades the
// placeholder before rendering. The handoff argument carries no such
// check: it is written verbatim to its own file and nothing reads it
// structurally.
func Test_finish_refuses_a_replacement_state_body_with_an_unterminated_fence(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	stepPath := filepath.Join(featureDir, "STEP-02.md")
	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" +
		"## Fixture Handoff" + "\n"
	require.NoError(t, os.WriteFile(stepPath, []byte(step), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-02\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	oldState := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(oldState), 0o600))

	before := snapshotTree(t, featureDir)

	srv := scaffold.NewServer(cfg, root)
	handoff := []byte("NEW-HANDOFF\n")
	badState := []byte("## Decisions Fixture\n\n```\nunterminated\n")

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", handoff, badState)

	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrUnterminatedFence)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, scaffold.StateSource, refusal.Path)
	assert.Equal(t, 3, refusal.Line, "the fence opens on the state body's own 3rd line")

	after := snapshotTree(t, featureDir)
	assert.Equal(t, before, after, "a refused finish must leave every file byte-identical")
}

// Test_Finish_ticks_a_progress_entry_when_the_specification_uses_CRLF
// reproduces the reviewer's finding directly: tickProgressEntry's heading
// match, like insertProgressEntry's, used to right-trim only " \t", so a
// CRLF specification's progress heading never matched and Finish refused
// with ErrNoProgressHeading on every CRLF feature.
func Test_Finish_ticks_a_progress_entry_when_the_specification_uses_CRLF(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + "## Fixture Handoff" + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-02.md"), []byte(step), 0o600))

	spec := "# widgets\r\n\r\n" + cfg.ProgressHeading + "\r\n\r\n- [ ] STEP-02\r\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))

	srv := scaffold.NewServer(cfg, root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", []byte("NEW-HANDOFF"), []byte(state))
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(featureDir, cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(got), "- [x] STEP-02")
}

func Test_ticks_the_progress_entry_for_the_finished_step(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)

	assert.Contains(t, string(got), "- [x] STEP-02: Assemble the thing")
}

func Test_leaves_every_other_progress_entry_unchanged(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)

	assert.Contains(t, string(got), "- [x] STEP-01: already done")
	assert.Contains(t, string(got), "- [ ] STEP-03")
}

func Test_does_not_tick_an_entry_whose_id_merely_starts_with_the_finished_id(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%d.md"
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step1 := "---\nid: STEP-1\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-1\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + "## Fixture Handoff" + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-1.md"), []byte(step1), 0o600))

	step10 := "---\nid: STEP-10\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-10\n\n" + cfg.ChecklistHeading + "\n\n- [ ] pending\n\n" + "## Fixture Handoff" + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-10.md"), []byte(step10), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-10\n- [ ] STEP-1\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))

	srv := scaffold.NewServer(cfg, root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-1", []byte("handoff body"), []byte(state))
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(featureDir, cfg.SpecificationFile))
	require.NoError(t, readErr)

	assert.Contains(t, string(got), "- [ ] STEP-10")
	assert.Contains(t, string(got), "- [x] STEP-1")
}

// fixtureFileNames is the exact set of files newFinishFixture writes plus
// the handoff file finish creates, against which the temp-file sweep
// below checks.
func fixtureFileNames(fx finishFixture) []string {
	return []string{
		fx.cfg.SpecificationFile, fx.cfg.StateFile,
		"STEP-01.md", "STEP-02.md", "STEP-03.md",
		"STEP-02" + fx.cfg.HandoffFileSuffix,
	}
}

func Test_finish_leaves_no_temp_file_in_the_feature_directory(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	entries, readErr := os.ReadDir(fx.featureDir())
	require.NoError(t, readErr)

	assert.ElementsMatch(t, fixtureFileNames(fx), namesOf(entries))
}

// Test_reports_a_handoff_write_that_cannot_be_committed guards the contract
// each write site now carries by hand: the rename happens in Close, so a site
// that checks only the write and not Close reports success on a write that
// never landed. A directory at the handoff's path makes the rename fail with
// nothing else disturbed — asserted file by file: the step file still
// says open, the state file still holds its old body, and the
// specification's STEP-02 entry is still unticked.
//
// It also pins convergence: the step file must still say status: open, since
// that is the sole doneness authority and a retry has to be able to repair
// the half-finished state — and then, clearing the obstruction and retrying
// with the same arguments, that the retry actually lands the handoff and
// marks the step done, not merely that the precondition for a retry holds.
func Test_reports_a_handoff_write_that_cannot_be_committed(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	blocked := filepath.Join(fx.featureDir(), "STEP-02"+fx.cfg.HandoffFileSuffix)
	require.NoError(t, os.Mkdir(blocked, 0o755))

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)

	require.Error(t, err)
	require.NotErrorIs(t, err, scaffold.ErrPartialWrite,
		"the handoff write is the first of the four; nothing landed before it failed")

	stepBody, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(stepBody), "status: open",
		"a write that could not be committed must leave the step retryable")

	// The three writes after the blocked one must not have run. Their
	// control arm is the retry below: it changes both of these files, which
	// is what proves these two probes would have seen a write had one
	// happened, rather than being satisfied by nothing occurring at all.
	gotState, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, readErr)
	assert.Equal(t, oldStateBody(fx.cfg), string(gotState),
		"the state write follows the blocked handoff write, so it must not have run")

	gotSpec, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotSpec), "- [ ] STEP-02: Assemble the thing",
		"the specification write follows the blocked handoff write, so STEP-02 must still be unticked")

	require.NoError(t, os.RemoveAll(blocked))

	_, retryErr := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, retryErr, "a retry once the obstruction is cleared must converge")

	gotHandoff, readErr := os.ReadFile(fx.handoffPath())
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newHandoff), string(gotHandoff), "the retry must finish the write the blocked attempt left undone")

	gotStepAfterRetry, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotStepAfterRetry), "status: done")

	// The control arm for the two "did not run" assertions above: the same
	// two probes, on the same two files, now see the writes the blocked
	// attempt left undone.
	stateAfterRetry, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newState), string(stateAfterRetry))

	specAfterRetry, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(specAfterRetry), "- [x] STEP-02: Assemble the thing")
}

// Test_reports_a_state_write_that_cannot_be_committed covers the second of
// Finish's four writes: a directory planted at the state file's own temp
// sibling — the same seam Test_the_temp_file_sweep_sees_a_temp_file uses as
// a decoy — blocks Create's OpenFile outright, before Close is ever reached.
//
// It carries the same convergence proof as the specification and step-file
// siblings below it: the handoff write ahead of the blocked one has landed,
// the state write itself has not (the old state is still on disk), and the
// step file and specification — both after the blocked write in the fixed
// order — are untouched. Clearing the obstruction and retrying with the same
// arguments must then finish the whole sequence.
func Test_reports_a_state_write_that_cannot_be_committed(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	blocked := filepath.Join(fx.featureDir(), "."+fx.cfg.StateFile+".brief-tmp")
	require.NoError(t, os.Mkdir(blocked, 0o755))

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrPartialWrite,
		"the handoff write ahead of the blocked state write already landed")

	gotHandoff, readErr := os.ReadFile(fx.handoffPath())
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newHandoff), string(gotHandoff), "the handoff write ahead of the blocked one must have landed")

	gotState, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, readErr)
	assert.Equal(t, oldStateBody(fx.cfg), string(gotState), "the state write itself was blocked, so the old state must still be on disk")

	gotStep, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotStep), "status: open",
		"a write that could not be committed must leave the step retryable")

	gotSpec, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotSpec), "- [ ] STEP-02: Assemble the thing",
		"the specification write must not have run past the blocked state write")

	require.NoError(t, os.RemoveAll(blocked))

	_, retryErr := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, retryErr, "a retry once the obstruction is cleared must converge")

	gotStateAfterRetry, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newState), string(gotStateAfterRetry))

	gotSpecAfterRetry, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotSpecAfterRetry), "- [x] STEP-02: Assemble the thing")
}

// Test_reports_a_step_file_write_that_cannot_be_committed covers the third
// of Finish's four writes: a directory planted at the step file's own temp
// sibling blocks it. This is the doc's own load-bearing boundary — the last
// write whose failure still leaves the step's frontmatter status "open", the
// sole doneness authority a retry relies on; a failure one write later (the
// specification) would leave status already "done".
//
// It asserts the same convergence shape as its siblings: the handoff and
// state writes ahead of the blocked one have landed, the step file itself
// still says status: open, the specification is still unticked, and clearing
// the obstruction and retrying finishes the sequence.
func Test_reports_a_step_file_write_that_cannot_be_committed(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	blocked := filepath.Join(fx.featureDir(), "."+"STEP-02.md"+".brief-tmp")
	require.NoError(t, os.Mkdir(blocked, 0o755))

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrPartialWrite,
		"the handoff and state writes ahead of the blocked step-file write already landed")

	gotHandoff, readErr := os.ReadFile(fx.handoffPath())
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newHandoff), string(gotHandoff), "the handoff write ahead of the blocked one must have landed")

	gotState, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newState), string(gotState), "the state write ahead of the blocked one must have landed")

	gotStep, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotStep), "status: open",
		"the step-file write itself was blocked, so status must still be open — the last prefix that leaves it that way")

	gotSpec, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotSpec), "- [ ] STEP-02: Assemble the thing",
		"the specification write must not have run past the blocked step-file write")

	require.NoError(t, os.RemoveAll(blocked))

	_, retryErr := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, retryErr, "a retry once the obstruction is cleared must converge")

	gotStepAfterRetry, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotStepAfterRetry), "status: done")

	gotSpecAfterRetry, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotSpecAfterRetry), "- [x] STEP-02: Assemble the thing")
}

// Test_reports_a_specification_write_that_cannot_be_committed covers the
// io.StringWriter path the same way, at the last write in the sequence: a
// directory planted at the specification's temp sibling blocks it.
//
// It also carries the only proof in this suite that Finish's crash-
// convergence claim holds past the first write: the handoff, state and step
// writes ahead of the blocked one have already landed, so it asserts each
// of those three lands correctly and the specification does not, then
// clears the obstruction and calls Finish again with the same arguments —
// the retry a crash at this exact point would need — and asserts that
// second call both succeeds and finishes the tick the first one left undone.
func Test_reports_a_specification_write_that_cannot_be_committed(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	blocked := filepath.Join(fx.featureDir(), "."+fx.cfg.SpecificationFile+".brief-tmp")
	require.NoError(t, os.Mkdir(blocked, 0o755))

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrPartialWrite,
		"the handoff, state and step-file writes ahead of the blocked specification write already landed")

	gotHandoff, readErr := os.ReadFile(fx.handoffPath())
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newHandoff), string(gotHandoff), "the handoff write ahead of the blocked one must have landed")

	gotState, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newState), string(gotState), "the state write ahead of the blocked one must have landed")

	gotStep, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotStep), "status: done", "the step-file write ahead of the blocked one must have landed")

	gotSpec, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotSpec), "- [ ] STEP-02: Assemble the thing",
		"the specification write itself was blocked, so the progress entry must still be unticked")

	require.NoError(t, os.RemoveAll(blocked))

	_, retryErr := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, retryErr, "a retry once the obstruction is cleared must converge")

	gotSpecAfterRetry, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(gotSpecAfterRetry), "- [x] STEP-02: Assemble the thing",
		"the retry must finish the tick the blocked write left undone")
}

// Test_the_temp_file_sweep_sees_a_temp_file is the control for
// Test_finish_leaves_no_temp_file_in_the_feature_directory's claim, and uses
// the exact probe that test does — namesOf(entries) compared against
// fixtureFileNames(fx) — rather than a separate helper: a decoy leftover
// temp file shows up in the directory listing but not in the fixture's
// expected set, which is exactly what would fail the real test's
// assert.ElementsMatch if finish ever left one behind.
func Test_the_temp_file_sweep_sees_a_temp_file(t *testing.T) {
	fx := newFinishFixture(t)
	decoy := "." + fx.cfg.StateFile + ".brief-tmp"
	require.NoError(t, os.WriteFile(filepath.Join(fx.featureDir(), decoy), []byte("leftover"), 0o600))

	entries, err := os.ReadDir(fx.featureDir())
	require.NoError(t, err)

	assert.Contains(t, namesOf(entries), decoy, "the probe must see the decoy")
	assert.NotContains(t, fixtureFileNames(fx), decoy, "the fixture's expected set must not already include it")
}

func Test_refuses_an_unknown_feature_on_finish(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.Finish(context.Background(), "ghost", "STEP-02", []byte("h"), []byte("s"))

	require.ErrorIs(t, err, scaffold.ErrNoSuchFeature)
	assert.NoDirExists(t, filepath.Join(root, cfg.FeatureDirectory, "ghost"))
}

// Test_refuses_an_unknown_step is MAJOR 3: the refusal names the id and
// feature, and its Fix lists every step id newFinishFixture wrote
// (STEP-01..03, ascending by number) — the same "known:" convention cli's
// own unknown-feature refusal carries, rather than a "run 'brief new step'"
// suggestion that writes files on what is otherwise a read-only refusal.
func Test_refuses_an_unknown_step(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	before := snapshotTree(t, fx.featureDir())

	_, err := srv.Finish(context.Background(), "widgets", "STEP-99", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrNoSuchStep)
	assert.Equal(t, before, snapshotTree(t, fx.featureDir()))

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, `no step "STEP-99" in widgets`, refusal.Problem)
	assert.Equal(t, "known: STEP-01, STEP-02, STEP-03", refusal.Fix)
}

// Test_refuses_an_unknown_step_with_no_step_files_suggests_creating_one is
// Test_refuses_an_unknown_step's empty-list companion: a feature directory
// with no step files at all gets the "known: none; run '...' to create
// one" suggestion instead of an empty list.
func Test_refuses_an_unknown_step_with_no_step_files_suggests_creating_one(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	srv := scaffold.NewServer(cfg, root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-01", []byte("h"), []byte("s"))

	require.ErrorIs(t, err, scaffold.ErrNoSuchStep)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, `no step "STEP-01" in widgets`, refusal.Problem)
	assert.Equal(t, "known: none; run 'brief new step widgets' to create one", refusal.Fix)
}

func Test_refuses_a_specification_with_no_progress_heading_on_finish(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + "## Fixture Handoff" + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-02.md"), []byte(step), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte("# widgets\n\nno progress list here.\n"), 0o600))

	srv := scaffold.NewServer(cfg, root)
	before := snapshotTree(t, featureDir)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", []byte("h"), []byte(oldStateBody(cfg)))

	require.ErrorIs(t, err, scaffold.ErrNoProgressHeading)
	assert.Equal(t, before, snapshotTree(t, featureDir))
}

func Test_refuses_a_progress_list_with_no_entry_for_the_step(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + "## Fixture Handoff" + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-02.md"), []byte(step), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-99\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	srv := scaffold.NewServer(cfg, root)
	before := snapshotTree(t, featureDir)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", []byte("h"), []byte(oldStateBody(cfg)))

	require.ErrorIs(t, err, scaffold.ErrNoProgressEntry)
	assert.Equal(t, before, snapshotTree(t, featureDir))
}

func Test_refuses_a_missing_state_file_on_finish(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + "## Fixture Handoff" + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-02.md"), []byte(step), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-02\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	srv := scaffold.NewServer(cfg, root)
	before := snapshotTree(t, featureDir)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", []byte("h"), []byte(oldStateBody(cfg)))

	require.ErrorIs(t, err, scaffold.ErrMalformedFeature)
	assert.Equal(t, before, snapshotTree(t, featureDir))
}

func Test_refuses_a_feature_name_that_escapes_the_feature_root_on_finish(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.Finish(context.Background(), "../escaped", "STEP-02", []byte("h"), []byte("s"))

	require.ErrorIs(t, err, scaffold.ErrNoSuchFeature)
	assert.NoDirExists(t, filepath.Join(root, "escaped"))
}

// Test_returns_an_error_and_changes_nothing_when_the_feature_directory_is_not_writable
// deliberately does not skip when running as root, unlike a typical
// permission-bit test: a require.Error failing to fire under root is the
// signal that this test stopped exercising the failure it names, and that
// signal is more valuable here than a clean skip.
func Test_returns_an_error_and_changes_nothing_when_the_feature_directory_is_not_writable(t *testing.T) {
	fx := newFinishFixture(t)
	t.Cleanup(func() { _ = os.Chmod(fx.featureDir(), 0o755) })
	require.NoError(t, os.Chmod(fx.featureDir(), 0o555))

	before := snapshotTree(t, fx.featureDir())
	srv := scaffold.NewServer(fx.cfg, fx.root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)

	require.Error(t, err)

	var refusal *scaffold.RefusalError
	assert.NotErrorAs(t, err, &refusal, "a mid-sequence write failure must not be a *RefusalError")
	assert.Contains(t, err.Error(), "run 'brief finish widgets STEP-02")
	assert.Equal(t, before, snapshotTree(t, fx.featureDir()))
	assert.NoFileExists(t, fx.handoffPath(), "the first of the four writes failing must leave nothing partial")
}

// Test_the_handoff_file_is_created_with_the_same_mode_as_its_siblings pins
// the mode of the one file Finish creates rather than replaces. The
// specification, state and step files are all created by writeExclusive at
// 0o600; the handoff file goes through atomicfile.Create, whose perm
// argument applies only on a fresh create. A wider perm there lands a
// world-readable handoff beside three owner-only siblings.
//
// That a wrong mode then sticks — every later Finish finding an existing
// file and preserving whatever mode the first one chose — is a property of
// atomicfile.Create, proven one layer down by
// Test_Create_keeps_the_existing_files_permissions_when_it_replaces_it.
// This test pins only the mode the file is born with.
//
// The umask is pinned at 0o022 — the permissive default, which would let a
// 0o644 perm through intact — so this fails on a wrong perm rather than
// passing because a stricter ambient umask happened to mask the extra bits
// away. syscall.Umask is process-global and no test in this package calls
// t.Parallel, so nothing else observes the pinned value mid-test.
//
// The state file's mode is read rather than asserted against a constant
// alone: the claim is that the handoff matches the siblings Finish writes
// beside it, and a fixture that somehow created those wider would
// otherwise go unnoticed.
func Test_the_handoff_file_is_created_with_the_same_mode_as_its_siblings(t *testing.T) {
	old := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(old) })

	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	handoffInfo, err := os.Stat(fx.handoffPath())
	require.NoError(t, err)

	stateInfo, statErr := os.Stat(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, statErr)

	assert.Equal(t, os.FileMode(0o600), stateInfo.Mode().Perm(),
		"the fixture's own siblings must be owner-only, or the comparison below proves nothing")
	assert.Equal(t, stateInfo.Mode().Perm(), handoffInfo.Mode().Perm(),
		"the handoff file must not be created wider than the siblings Finish writes beside it")
}
