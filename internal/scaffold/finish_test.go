package scaffold_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unexpectedFiles returns the names present in dir but not in want, so a
// "leaves no temp file" test can assert this is empty and its control arm
// can assert it is not — the same probe, used both ways, so neither
// assertion can pass vacuously.
func unexpectedFiles(t *testing.T, dir string, want []string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	wantSet := make(map[string]bool, len(want))
	for _, w := range want {
		wantSet[w] = true
	}

	var extra []string

	for _, e := range entries {
		if !wantSet[e.Name()] {
			extra = append(extra, e.Name())
		}
	}

	return extra
}

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

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
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

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
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

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(specPath)
	require.NoError(t, readErr)

	want := strings.Replace(string(before), "- [ ] STEP-02: Assemble the thing\n", "- [x] STEP-02: Assemble the thing\n", 1)

	assert.Equal(t, want, string(got))
}

func Test_replaces_the_state_file_with_exactly_the_supplied_body(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.StateFile))
	require.NoError(t, readErr)

	assert.Equal(t, string(fx.newState), string(got))
	assert.NotContains(t, string(got), "OLD-STATE-ENTRY")
}

func Test_marks_the_step_done_in_its_frontmatter(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)

	fm, _, parseErr := stepfile.ParseFrontmatter(got)
	require.NoError(t, parseErr)

	assert.True(t, fm.Done())
}

func Test_leaves_the_frontmatter_byte_identical_apart_from_the_status_line(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)

	assert.Contains(t, string(got), "owner: planner\n")
	assert.Contains(t, string(got), "depends-on: [STEP-01]\n")
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

	err := srv.Finish(context.Background(), "widgets", "STEP-02", handoff, badState)

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

	err := srv.Finish(context.Background(), "widgets", "STEP-02", []byte("NEW-HANDOFF"), []byte(state))
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(featureDir, cfg.SpecificationFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(got), "- [x] STEP-02")
}

func Test_ticks_the_progress_entry_for_the_finished_step(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(filepath.Join(fx.featureDir(), fx.cfg.SpecificationFile))
	require.NoError(t, readErr)

	assert.Contains(t, string(got), "- [x] STEP-02: Assemble the thing")
}

func Test_leaves_every_other_progress_entry_unchanged(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
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

	err := srv.Finish(context.Background(), "widgets", "STEP-1", []byte("handoff body"), []byte(state))
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

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	entries, readErr := os.ReadDir(fx.featureDir())
	require.NoError(t, readErr)

	assert.ElementsMatch(t, fixtureFileNames(fx), namesOf(entries))
}

func Test_the_temp_file_sweep_sees_a_temp_file(t *testing.T) {
	fx := newFinishFixture(t)
	decoy := "." + fx.cfg.StateFile + ".brief-tmp"
	require.NoError(t, os.WriteFile(filepath.Join(fx.featureDir(), decoy), []byte("leftover"), 0o600))

	extra := unexpectedFiles(t, fx.featureDir(), fixtureFileNames(fx))

	assert.Contains(t, extra, decoy)
}

func Test_refuses_an_unknown_feature_on_finish(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)

	err := srv.Finish(context.Background(), "ghost", "STEP-02", []byte("h"), []byte("s"))

	require.ErrorIs(t, err, scaffold.ErrNoSuchFeature)
	assert.NoDirExists(t, filepath.Join(root, cfg.FeatureDirectory, "ghost"))
}

func Test_refuses_an_unknown_step(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	before := snapshotTree(t, fx.featureDir())

	err := srv.Finish(context.Background(), "widgets", "STEP-99", fx.newHandoff, fx.newState)

	require.ErrorIs(t, err, scaffold.ErrNoSuchStep)
	assert.Equal(t, before, snapshotTree(t, fx.featureDir()))
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

	err := srv.Finish(context.Background(), "widgets", "STEP-02", []byte("h"), []byte("s"))

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

	err := srv.Finish(context.Background(), "widgets", "STEP-02", []byte("h"), []byte("s"))

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

	err := srv.Finish(context.Background(), "widgets", "STEP-02", []byte("h"), []byte("s"))

	require.ErrorIs(t, err, scaffold.ErrMalformedFeature)
	assert.Equal(t, before, snapshotTree(t, featureDir))
}

func Test_refuses_a_feature_name_that_escapes_the_feature_root_on_finish(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)

	err := srv.Finish(context.Background(), "../escaped", "STEP-02", []byte("h"), []byte("s"))

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

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)

	require.Error(t, err)

	var refusal *scaffold.RefusalError
	assert.NotErrorAs(t, err, &refusal, "a mid-sequence write failure must not be a *RefusalError")
	assert.Contains(t, err.Error(), "run 'brief finish widgets STEP-02")
	assert.Equal(t, before, snapshotTree(t, fx.featureDir()))
	assert.NoFileExists(t, fx.handoffPath(), "the first of the four writes failing must leave nothing partial")
}
