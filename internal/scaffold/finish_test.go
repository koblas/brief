package scaffold_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// Test_writes_the_supplied_handoff_to_its_own_file pins the amended
// SCENARIO-05 contract directly: the handoff no longer lives inside the
// step file, it lives at handoffPath(), and it holds fx.newHandoff
// verbatim — including its trailing newline and its nested fence.
func Test_writes_the_supplied_handoff_to_its_own_file(t *testing.T) {
	fx := newFinishFixtureFS(t)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile(fx.handoffName())
	require.NoError(t, readErr)

	assert.Equal(t, string(fx.newHandoff), string(got))
}

// Test_leaves_the_step_file_byte_identical_apart_from_the_status_line is
// R21's second clause at the Server level — the legacy-"## Notes"-section
// preservation proof.
func Test_leaves_the_step_file_byte_identical_apart_from_the_status_line(t *testing.T) {
	fx := newFinishFixtureFS(t)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile("STEP-02.md")
	require.NoError(t, readErr)

	want := strings.Replace(step02Body(fx.cfg), "status: open\n", "status: done\n", 1)

	assert.Equal(t, want, string(got))
}

// Test_leaves_the_specification_byte_identical_apart_from_the_finished_step_s_progress_line
// proves the trailing "## Notes" section in the specification survives
// finish's progress-checkbox edit untouched.
func Test_leaves_the_specification_byte_identical_apart_from_the_finished_step_s_progress_line(t *testing.T) {
	fx := newFinishFixtureFS(t)
	before, readErr := fx.mem.ReadFile(fx.cfg.SpecificationFile)
	require.NoError(t, readErr)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile(fx.cfg.SpecificationFile)
	require.NoError(t, readErr)

	want := strings.Replace(string(before), "- [ ] STEP-02: Assemble the thing\n", "- [x] STEP-02: Assemble the thing\n", 1)

	assert.Equal(t, want, string(got))
}

func Test_replaces_the_state_file_with_exactly_the_supplied_body(t *testing.T) {
	fx := newFinishFixtureFS(t)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile(fx.cfg.StateFile)
	require.NoError(t, readErr)

	assert.Equal(t, string(fx.newState), string(got))
}

func Test_marks_the_step_done_in_its_frontmatter(t *testing.T) {
	fx := newFinishFixtureFS(t)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile("STEP-02.md")
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
	pattern, patternErr := stepfilePattern(cfg)
	require.NoError(t, patternErr)
	handoffPattern, handoffPatternErr := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, handoffPatternErr)

	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" +
		"## Fixture Handoff" + "\n"
	oldState := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-02\n"

	mem := rwfs.NewMem(fstest.MapFS{
		"STEP-02.md":          &fstest.MapFile{Data: []byte(step), Mode: 0o600},
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte(spec), Mode: 0o600},
		cfg.StateFile:         &fstest.MapFile{Data: []byte(oldState), Mode: 0o600},
	})
	before := mem.Snapshot()

	srv := scaffold.NewServer(cfg, "")
	handoff := []byte("NEW-HANDOFF\n")
	badState := []byte("## Decisions Fixture\n\n```\nunterminated\n")

	_, err := srv.FinishFS(mem, testFeaturePath, "widgets", "STEP-02", handoff, badState, pattern, handoffPattern)

	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrUnterminatedFence)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, scaffold.StateSource, refusal.Path)
	assert.Equal(t, 3, refusal.Line, "the fence opens on the state body's own 3rd line")

	assert.Equal(t, before, mem.Snapshot(), "a refused finish must leave every file byte-identical")
}

// Test_Finish_ticks_a_progress_entry_when_the_specification_uses_CRLF pins
// that tickProgressEntry's heading match, like insertProgressEntry's, must
// right-trim "\r" along with " \t": a CRLF specification's progress
// heading must still match, or Finish would refuse every CRLF feature with
// ErrNoProgressHeading.
func Test_Finish_ticks_a_progress_entry_when_the_specification_uses_CRLF(t *testing.T) {
	cfg := fixtureConfig()
	pattern, patternErr := stepfilePattern(cfg)
	require.NoError(t, patternErr)
	handoffPattern, handoffPatternErr := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, handoffPatternErr)

	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + "## Fixture Handoff" + "\n"
	spec := "# widgets\r\n\r\n" + cfg.ProgressHeading + "\r\n\r\n- [ ] STEP-02\r\n"
	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"

	mem := rwfs.NewMem(fstest.MapFS{
		"STEP-02.md":          &fstest.MapFile{Data: []byte(step), Mode: 0o600},
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte(spec), Mode: 0o600},
		cfg.StateFile:         &fstest.MapFile{Data: []byte(state), Mode: 0o600},
	})

	srv := scaffold.NewServer(cfg, "")

	_, err := srv.FinishFS(mem, testFeaturePath, "widgets", "STEP-02", []byte("NEW-HANDOFF"), []byte(state), pattern, handoffPattern)
	require.NoError(t, err)

	got, readErr := mem.ReadFile(cfg.SpecificationFile)
	require.NoError(t, readErr)
	assert.Contains(t, string(got), "- [x] STEP-02")
}

func Test_ticks_the_progress_entry_for_the_finished_step(t *testing.T) {
	fx := newFinishFixtureFS(t)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile(fx.cfg.SpecificationFile)
	require.NoError(t, readErr)

	assert.Contains(t, string(got), "- [x] STEP-02: Assemble the thing")
}

func Test_leaves_every_other_progress_entry_unchanged(t *testing.T) {
	fx := newFinishFixtureFS(t)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := fx.mem.ReadFile(fx.cfg.SpecificationFile)
	require.NoError(t, readErr)

	assert.Contains(t, string(got), "- [x] STEP-01: already done")
	assert.Contains(t, string(got), "- [ ] STEP-03")
}

func Test_does_not_tick_an_entry_whose_id_merely_starts_with_the_finished_id(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%d.md"
	pattern, patternErr := stepfilePattern(cfg)
	require.NoError(t, patternErr)
	handoffPattern, handoffPatternErr := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, handoffPatternErr)

	step1 := "---\nid: STEP-1\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-1\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + "## Fixture Handoff" + "\n"
	step10 := "---\nid: STEP-10\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-10\n\n" + cfg.ChecklistHeading + "\n\n- [ ] pending\n\n" + "## Fixture Handoff" + "\n"
	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-10\n- [ ] STEP-1\n"
	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"

	mem := rwfs.NewMem(fstest.MapFS{
		"STEP-1.md":           &fstest.MapFile{Data: []byte(step1), Mode: 0o600},
		"STEP-10.md":          &fstest.MapFile{Data: []byte(step10), Mode: 0o600},
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte(spec), Mode: 0o600},
		cfg.StateFile:         &fstest.MapFile{Data: []byte(state), Mode: 0o600},
	})

	srv := scaffold.NewServer(cfg, "")

	_, err := srv.FinishFS(mem, testFeaturePath, "widgets", "STEP-1", []byte("handoff body"), []byte(state), pattern, handoffPattern)
	require.NoError(t, err)

	got, readErr := mem.ReadFile(cfg.SpecificationFile)
	require.NoError(t, readErr)

	assert.Contains(t, string(got), "- [ ] STEP-10")
	assert.Contains(t, string(got), "- [x] STEP-1")
}

// Test_reports_a_handoff_write_that_cannot_be_committed guards the
// contract Finish's write sequence carries: a directory at the handoff's
// own path makes Mem.WriteFile refuse with fs.ErrExist, nothing else
// disturbed — asserted file by file: the step file still says open, the
// state file still holds its old body, and the specification's STEP-02
// entry is still unticked.
//
// It also pins convergence: the step file must still say status: open,
// since that is the sole doneness authority and a retry has to be able to
// repair the half-finished state — and then, clearing the obstruction and
// retrying with the same arguments, that the retry actually lands the
// handoff and marks the step done, not merely that the precondition for a
// retry holds.
func Test_reports_a_handoff_write_that_cannot_be_committed(t *testing.T) {
	fx := newFinishFixtureFS(t)
	require.NoError(t, fx.mem.Mkdir(fx.handoffName(), 0o755))

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)

	require.Error(t, err)
	require.NotErrorIs(t, err, scaffold.ErrPartialWrite,
		"the handoff write is the first of the four; nothing landed before it failed")

	stepBody, readErr := fx.mem.ReadFile("STEP-02.md")
	require.NoError(t, readErr)
	assert.Contains(t, string(stepBody), "status: open",
		"a write that could not be committed must leave the step retryable")

	// The three writes after the blocked one must not have run. Their
	// control arm is the retry below: it changes both of these files, which
	// is what proves these two probes would have seen a write had one
	// happened, rather than being satisfied by nothing occurring at all.
	gotState, readErr := fx.mem.ReadFile(fx.cfg.StateFile)
	require.NoError(t, readErr)
	assert.Equal(t, oldStateBody(fx.cfg), string(gotState),
		"the state write follows the blocked handoff write, so it must not have run")

	gotSpec, readErr := fx.mem.ReadFile(fx.cfg.SpecificationFile)
	require.NoError(t, readErr)
	assert.Contains(t, string(gotSpec), "- [ ] STEP-02: Assemble the thing",
		"the specification write follows the blocked handoff write, so STEP-02 must still be unticked")

	require.NoError(t, fx.mem.Remove(fx.handoffName()))

	_, retryErr := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, retryErr, "a retry once the obstruction is cleared must converge")

	gotHandoff, readErr := fx.mem.ReadFile(fx.handoffName())
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newHandoff), string(gotHandoff), "the retry must finish the write the blocked attempt left undone")

	gotStepAfterRetry, readErr := fx.mem.ReadFile("STEP-02.md")
	require.NoError(t, readErr)
	assert.Contains(t, string(gotStepAfterRetry), "status: done")

	// The control arm for the two "did not run" assertions above: the same
	// two probes, on the same two files, now see the writes the blocked
	// attempt left undone.
	stateAfterRetry, readErr := fx.mem.ReadFile(fx.cfg.StateFile)
	require.NoError(t, readErr)
	assert.Equal(t, string(fx.newState), string(stateAfterRetry))

	specAfterRetry, readErr := fx.mem.ReadFile(fx.cfg.SpecificationFile)
	require.NoError(t, readErr)
	assert.Contains(t, string(specAfterRetry), "- [x] STEP-02: Assemble the thing")
}

// Test_refuses_an_unknown_step pins the unknown-step refusal's shape: it
// names the id and feature, and its Fix lists every step id
// newFinishFixtureFS wrote (STEP-01..03, ascending by number) — the same
// "known:" convention cli's own unknown-feature refusal carries, rather
// than a "run 'brief new step'" suggestion that writes files on what is
// otherwise a read-only refusal.
func Test_refuses_an_unknown_step(t *testing.T) {
	fx := newFinishFixtureFS(t)
	before := fx.mem.Snapshot()

	_, err := fx.finish(t, "STEP-99", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrNoSuchStep)
	assert.Equal(t, before, fx.mem.Snapshot())

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
	pattern, patternErr := stepfilePattern(cfg)
	require.NoError(t, patternErr)
	handoffPattern, handoffPatternErr := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, handoffPatternErr)
	mem := rwfs.NewMem(fstest.MapFS{})
	srv := scaffold.NewServer(cfg, "")

	_, err := srv.FinishFS(mem, testFeaturePath, "widgets", "STEP-01", []byte("h"), []byte("s"), pattern, handoffPattern)

	require.ErrorIs(t, err, scaffold.ErrNoSuchStep)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, `no step "STEP-01" in widgets`, refusal.Problem)
	assert.Equal(t, "known: none; run 'brief new step widgets' to create one", refusal.Fix)
}

// Test_refuses_an_unknown_step_lists_known_ids_in_numeric_not_filename_order
// pins knownStepIDs' sort by stepfile.Pattern.Number rather than
// fs.ReadDir's byte order: an unpadded "STEP-%d.md" pattern puts
// "STEP-10.md" before "STEP-2.md" in filename order, while the Fix's
// "known:" list must still name STEP-2 first.
func Test_refuses_an_unknown_step_lists_known_ids_in_numeric_not_filename_order(t *testing.T) {
	cfg := fixtureConfig()
	cfg.StepFilePattern = "STEP-%d.md"
	pattern, patternErr := stepfilePattern(cfg)
	require.NoError(t, patternErr)
	handoffPattern, handoffPatternErr := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, handoffPatternErr)

	mem := rwfs.NewMem(fstest.MapFS{
		"STEP-10.md": &fstest.MapFile{Data: []byte("x"), Mode: 0o600},
		"STEP-2.md":  &fstest.MapFile{Data: []byte("x"), Mode: 0o600},
	})
	srv := scaffold.NewServer(cfg, "")

	_, err := srv.FinishFS(mem, testFeaturePath, "widgets", "STEP-99", []byte("h"), []byte("s"), pattern, handoffPattern)

	require.ErrorIs(t, err, scaffold.ErrNoSuchStep)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, "known: STEP-2, STEP-10", refusal.Fix)
}

func Test_refuses_a_specification_with_no_progress_heading_on_finish(t *testing.T) {
	cfg := fixtureConfig()
	pattern, patternErr := stepfilePattern(cfg)
	require.NoError(t, patternErr)
	handoffPattern, handoffPatternErr := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, handoffPatternErr)

	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + "## Fixture Handoff" + "\n"

	mem := rwfs.NewMem(fstest.MapFS{
		"STEP-02.md":          &fstest.MapFile{Data: []byte(step), Mode: 0o600},
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte("# widgets\n\nno progress list here.\n"), Mode: 0o600},
	})
	before := mem.Snapshot()
	srv := scaffold.NewServer(cfg, "")

	_, err := srv.FinishFS(mem, testFeaturePath, "widgets", "STEP-02", []byte("h"), []byte(oldStateBody(cfg)), pattern, handoffPattern)

	require.ErrorIs(t, err, scaffold.ErrNoProgressHeading)
	assert.Equal(t, before, mem.Snapshot())
}

func Test_refuses_a_progress_list_with_no_entry_for_the_step(t *testing.T) {
	cfg := fixtureConfig()
	pattern, patternErr := stepfilePattern(cfg)
	require.NoError(t, patternErr)
	handoffPattern, handoffPatternErr := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, handoffPatternErr)

	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + "## Fixture Handoff" + "\n"
	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-99\n"

	mem := rwfs.NewMem(fstest.MapFS{
		"STEP-02.md":          &fstest.MapFile{Data: []byte(step), Mode: 0o600},
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte(spec), Mode: 0o600},
	})
	before := mem.Snapshot()
	srv := scaffold.NewServer(cfg, "")

	_, err := srv.FinishFS(mem, testFeaturePath, "widgets", "STEP-02", []byte("h"), []byte(oldStateBody(cfg)), pattern, handoffPattern)

	require.ErrorIs(t, err, scaffold.ErrNoProgressEntry)
	assert.Equal(t, before, mem.Snapshot())
}

func Test_refuses_a_missing_state_file_on_finish(t *testing.T) {
	cfg := fixtureConfig()
	pattern, patternErr := stepfilePattern(cfg)
	require.NoError(t, patternErr)
	handoffPattern, handoffPatternErr := stepfileHandoffPattern(cfg, pattern)
	require.NoError(t, handoffPatternErr)

	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + "## Fixture Handoff" + "\n"
	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-02\n"

	mem := rwfs.NewMem(fstest.MapFS{
		"STEP-02.md":          &fstest.MapFile{Data: []byte(step), Mode: 0o600},
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte(spec), Mode: 0o600},
	})
	before := mem.Snapshot()
	srv := scaffold.NewServer(cfg, "")

	_, err := srv.FinishFS(mem, testFeaturePath, "widgets", "STEP-02", []byte("h"), []byte(oldStateBody(cfg)), pattern, handoffPattern)

	require.ErrorIs(t, err, scaffold.ErrMalformedFeature)
	assert.Equal(t, before, mem.Snapshot())
}
