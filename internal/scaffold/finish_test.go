package scaffold_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/markdown"
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
// rather than on a contrived edge case.
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
		cfg.HandoffHeading + "\n" +
		"not a real anchor\n" +
		"```\n" +
		"\n" +
		cfg.HandoffHeading + "\n"
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
		cfg.HandoffHeading + "\n" +
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
		cfg.HandoffHeading + "\n"
	require.NoError(t, os.WriteFile(fx.stepPath("STEP-03.md"), []byte(step03), 0o600))

	require.NoError(t, os.WriteFile(filepath.Join(fx.featureDir(), cfg.StateFile), []byte(oldStateBody(cfg)), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n" +
		"- [x] STEP-01: already done\n" +
		"- [ ] STEP-02: Assemble the thing\n" +
		"- [ ] STEP-03\n"
	require.NoError(t, os.WriteFile(filepath.Join(fx.featureDir(), cfg.SpecificationFile), []byte(spec), 0o600))

	fx.newHandoff = []byte("NEW-HANDOFF-02\n\n```\n# not a heading\n```\n")
	fx.newState = newStateBody(cfg)

	return fx
}

func Test_writes_the_supplied_handoff_under_the_step_s_handoff_anchor(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)

	_, rest, parseErr := stepfile.ParseFrontmatter(got)
	require.NoError(t, parseErr)

	handoff, ok := markdown.Section(string(rest), fx.cfg.HandoffHeading)
	require.True(t, ok)
	assert.Equal(t, "NEW-HANDOFF-02\n\n```\n# not a heading\n```", handoff)

	checklist, ok := markdown.Section(string(rest), fx.cfg.ChecklistHeading)
	require.True(t, ok)
	assert.Equal(t, "- [x] first thing\n- [x] second thing\n\n```\nfenced decoy\n"+fx.cfg.HandoffHeading+"\nnot a real anchor\n```", checklist)
}

func Test_leaves_the_rest_of_the_step_file_byte_identical(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	got, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)

	original := step02Body(fx.cfg)
	withStatusDone := strings.Replace(original, "status: open\n", "status: done\n", 1)
	want := withStatusDone + "\n" + strings.Trim(string(fx.newHandoff), "\n") + "\n"

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

func Test_finishing_with_a_handoff_already_in_place_reproduces_the_file_byte_for_byte(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	firstWrite, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)

	err = srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	secondWrite, readErr := os.ReadFile(fx.stepPath("STEP-02.md"))
	require.NoError(t, readErr)

	assert.Equal(t, string(firstWrite), string(secondWrite))
}

// Test_finish_refuses_a_step_file_whose_handoff_anchor_is_not_the_last_heading
// reproduces the reviewer's BLOCKER directly: a step file whose handoff
// anchor already has a "## Notes" section after it, on a step that has
// never been finished. spliceHandoff now takes everything from the anchor
// to end of file as the handoff section unconditionally — there is no
// terminator scan left to get wrong — so without this refusal a clean
// handoff would silently delete "## Notes" on the very first Finish call
// rather than growing or corrupting it. The refusal must fire before any
// write, naming the step file, the line "## Notes" starts on, and the
// heading itself.
func Test_finish_refuses_a_step_file_whose_handoff_anchor_is_not_the_last_heading(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	stepPath := filepath.Join(featureDir, "STEP-02.md")
	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" +
		cfg.HandoffHeading + "\n\n## Notes\n\nsome notes\n"
	require.NoError(t, os.WriteFile(stepPath, []byte(step), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-02\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))

	before := snapshotTree(t, featureDir)

	srv := scaffold.NewServer(cfg, root)
	handoff := []byte("NEW-HANDOFF\n")

	err := srv.Finish(context.Background(), "widgets", "STEP-02", handoff, []byte(state))

	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrHandoffNotLast)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, stepPath, refusal.Path)
	assert.Equal(t, 15, refusal.Line, "\"## Notes\" is the step file's 15th line")
	assert.Contains(t, refusal.Problem, `"## Notes"`)

	after := snapshotTree(t, featureDir)
	assert.Equal(t, before, after, "a refused finish must leave every file byte-identical")
}

// Test_finish_refuses_a_step_file_whose_on_disk_handoff_section_has_an_unterminated_fence
// reproduces the fourth-round repro directly: the fence inside the on-disk
// handoff section never closes, so markdown.TrailingHeading's own fence
// state — walked fresh from just past the anchor, exactly like this
// check's — reads "## Notes" as fenced content and never reports it,
// letting spliceHandoff's unconditional splice-to-EOF silently delete it.
// The earlier, balanced-handoff fixture in
// Test_finish_refuses_a_step_file_whose_handoff_anchor_is_not_the_last_heading
// cannot reach this path at all: its fence closes before "## Notes", so
// TrailingHeading finds the heading and ErrHandoffNotLast already refuses.
// Only an unterminated fence in the section spliceHandoff is about to
// replace can hide a trailing heading from that scan.
func Test_finish_refuses_a_step_file_whose_on_disk_handoff_section_has_an_unterminated_fence(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	stepPath := filepath.Join(featureDir, "STEP-02.md")
	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" +
		cfg.HandoffHeading + "\n\n" +
		"older binary wrote this:\n\n" +
		"```bash\n" +
		"go test ./...\n\n" +
		"## Notes\n\n" +
		"PLEASE KEEP THIS TOO\n"
	require.NoError(t, os.WriteFile(stepPath, []byte(step), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-02\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))

	before := snapshotTree(t, featureDir)

	srv := scaffold.NewServer(cfg, root)
	handoff := []byte("NEW-HANDOFF\n")

	err := srv.Finish(context.Background(), "widgets", "STEP-02", handoff, []byte(state))

	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrUnterminatedFence)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, stepPath, refusal.Path)
	assert.Equal(t, 17, refusal.Line, "the fence opens on the step file's own 17th line")
	assert.Contains(t, refusal.Problem, "```")

	after := snapshotTree(t, featureDir)
	assert.Equal(t, before, after, "a refused finish must leave every file byte-identical")
}

// Test_finish_does_not_refuse_a_re_finish_whose_own_prior_handoff_contains_a_heading
// is the control arm for the "not last heading" refusal's !fm.Done() gate:
// a handoff containing an ordinary, unfenced heading ("## Risks") is
// legitimate content once Finish has accepted it, and R11 requires an
// identical re-finish to keep succeeding rather than trip over its own
// prior output. Three identical finishes must all succeed, and the step
// file's bytes must stop changing after the first write — proving the
// splice is a structural fixed point, not merely a refusal-free one.
func Test_finish_does_not_refuse_a_re_finish_whose_own_prior_handoff_contains_a_heading(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	stepPath := filepath.Join(featureDir, "STEP-02.md")
	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" +
		cfg.HandoffHeading + "\n"
	require.NoError(t, os.WriteFile(stepPath, []byte(step), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-02\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))

	srv := scaffold.NewServer(cfg, root)
	handoff := []byte("summary line\n\n## Risks\n\nthe risk\n")

	err := srv.Finish(context.Background(), "widgets", "STEP-02", handoff, []byte(state))
	require.NoError(t, err)

	firstWrite, readErr := os.ReadFile(stepPath)
	require.NoError(t, readErr)

	err = srv.Finish(context.Background(), "widgets", "STEP-02", handoff, []byte(state))
	require.NoError(t, err)

	secondWrite, readErr := os.ReadFile(stepPath)
	require.NoError(t, readErr)

	err = srv.Finish(context.Background(), "widgets", "STEP-02", handoff, []byte(state))
	require.NoError(t, err)

	thirdWrite, readErr := os.ReadFile(stepPath)
	require.NoError(t, readErr)

	assert.Equal(t, string(firstWrite), string(secondWrite))
	assert.Equal(t, string(secondWrite), string(thirdWrite))
	assert.Contains(t, string(firstWrite), "## Risks\n\nthe risk")
}

// Test_finishing_twice_with_a_nested_fenced_handoff_is_a_fixed_point
// reproduces the reviewer's original finding's fixture — a handoff whose
// fenced block (four backticks) contains a shorter fenced-looking line
// (three backticks), the shape that used to make markdown.SectionRange's
// boolean fence toggle think the outer fence stayed open past end of
// file — against the fix that made it structurally unreachable rather
// than merely rarer: spliceHandoff no longer scans for a terminator at
// all, so no fence state, nested or otherwise, is left for a second
// Finish to misread. There is no trailing section here for that stale
// scan to have dropped; SCENARIO-19's family covers what a step file with
// content after the anchor gets instead.
func Test_finishing_twice_with_a_nested_fenced_handoff_is_a_fixed_point(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" +
		cfg.HandoffHeading + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-02.md"), []byte(step), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-02\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))

	srv := scaffold.NewServer(cfg, root)
	handoff := []byte("````\n```\n````")

	err := srv.Finish(context.Background(), "widgets", "STEP-02", handoff, []byte(state))
	require.NoError(t, err)

	firstWrite, readErr := os.ReadFile(filepath.Join(featureDir, "STEP-02.md"))
	require.NoError(t, readErr)
	require.Contains(t, string(firstWrite), "````\n```\n````")

	err = srv.Finish(context.Background(), "widgets", "STEP-02", handoff, []byte(state))
	require.NoError(t, err)

	secondWrite, readErr := os.ReadFile(filepath.Join(featureDir, "STEP-02.md"))
	require.NoError(t, readErr)

	assert.Equal(t, string(firstWrite), string(secondWrite))
}

// Test_finish_refuses_a_handoff_with_an_unterminated_fence reproduces the
// reviewer's original BLOCKER repro's handoff (a single opening ``` never
// closed), on a step file with no trailing section, so the only thing
// under test is the handoff-fence check itself. The refusal must name
// scaffold.HandoffSource — Finish has no path for the handoff argument's
// own bytes — with the line the fence opened on, since Finish has never
// heard of a real file for it to name instead; cli/finish.go is what
// upgrades that placeholder to the real --handoff source.
func Test_finish_refuses_a_handoff_with_an_unterminated_fence(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	stepPath := filepath.Join(featureDir, "STEP-02.md")
	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" +
		cfg.HandoffHeading + "\n"
	require.NoError(t, os.WriteFile(stepPath, []byte(step), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-02\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))

	before := snapshotTree(t, featureDir)

	srv := scaffold.NewServer(cfg, root)
	handoff := []byte("Repro:\n\n```bash\ngo test ./...\n")

	err := srv.Finish(context.Background(), "widgets", "STEP-02", handoff, []byte(state))

	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrUnterminatedFence)

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, scaffold.HandoffSource, refusal.Path)
	assert.Equal(t, 3, refusal.Line, "the fence opens on the handoff's own 3rd line")

	after := snapshotTree(t, featureDir)
	assert.Equal(t, before, after, "a refused finish must leave every file byte-identical")
}

// Test_finish_refuses_a_handoff_whose_closing_fence_is_indented_four_spaces
// reproduces the reviewer's second repro shape: a closing fence indented
// four spaces is not a fence delimiter at all per CommonMark's
// three-space tolerance, so the opening fence is left unterminated — the
// ordinary shape a nested-list handoff produces by accident.
func Test_finish_refuses_a_handoff_whose_closing_fence_is_indented_four_spaces(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	stepPath := filepath.Join(featureDir, "STEP-02.md")
	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" +
		cfg.HandoffHeading + "\n"
	require.NoError(t, os.WriteFile(stepPath, []byte(step), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-02\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))

	before := snapshotTree(t, featureDir)

	srv := scaffold.NewServer(cfg, root)
	handoff := []byte("```\nRepro body\n    ```\n")

	err := srv.Finish(context.Background(), "widgets", "STEP-02", handoff, []byte(state))

	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrUnterminatedFence)

	after := snapshotTree(t, featureDir)
	assert.Equal(t, before, after, "a refused finish must leave every file byte-identical")
}

// Test_finish_refuses_a_replacement_state_body_with_an_unterminated_fence
// is the state-argument half of the same check: a replacement state body
// whose fence never closes would leave every configured state heading
// unreadable on the next Start, since assemble.stateSections finds each
// one by scanning forward for a terminator. Named against
// scaffold.StateSource for the same reason the handoff check names
// scaffold.HandoffSource — Finish never learns the argument's real path.
func Test_finish_refuses_a_replacement_state_body_with_an_unterminated_fence(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	stepPath := filepath.Join(featureDir, "STEP-02.md")
	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" +
		cfg.HandoffHeading + "\n"
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

// Test_finish_refuses_the_structural_defect_over_the_handoff_fence_when_both_are_present
// pins the check order for a step file that is wrong in two independent
// ways at once: its handoff anchor already has a trailing "## Notes"
// section (a structural defect in the file itself), and the supplied
// handoff argument also carries an unterminated fence (a defect in the
// input). The step-file structural check runs first, so its refusal is
// the one Finish reports — a reordering that let the argument check run
// first would report ErrUnterminatedFence here instead, silently changing
// which "first thing wrong" R14a promises to name.
func Test_finish_refuses_the_structural_defect_over_the_handoff_fence_when_both_are_present(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	stepPath := filepath.Join(featureDir, "STEP-02.md")
	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" +
		cfg.HandoffHeading + "\n\n## Notes\n\nPLEASE KEEP THIS\n"
	require.NoError(t, os.WriteFile(stepPath, []byte(step), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-02\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))

	before := snapshotTree(t, featureDir)

	srv := scaffold.NewServer(cfg, root)
	handoff := []byte("Repro:\n\n```bash\ngo test ./...\n")

	err := srv.Finish(context.Background(), "widgets", "STEP-02", handoff, []byte(state))

	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrHandoffNotLast)
	require.NotErrorIs(t, err, scaffold.ErrUnterminatedFence,
		"the structural defect must be reported before the argument defect is ever checked")

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
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + cfg.HandoffHeading + "\n"
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
		"# STEP-1\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + cfg.HandoffHeading + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-1.md"), []byte(step1), 0o600))

	step10 := "---\nid: STEP-10\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-10\n\n" + cfg.ChecklistHeading + "\n\n- [ ] pending\n\n" + cfg.HandoffHeading + "\n"
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

// fixtureFileNames is the exact set of files newFinishFixture writes,
// against which the temp-file sweep below checks.
func fixtureFileNames(cfg config.Config) []string {
	return []string{cfg.SpecificationFile, cfg.StateFile, "STEP-01.md", "STEP-02.md", "STEP-03.md"}
}

func Test_finish_leaves_no_temp_file_in_the_feature_directory(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	entries, readErr := os.ReadDir(fx.featureDir())
	require.NoError(t, readErr)

	assert.ElementsMatch(t, fixtureFileNames(fx.cfg), namesOf(entries))
}

func Test_the_temp_file_sweep_sees_a_temp_file(t *testing.T) {
	fx := newFinishFixture(t)
	decoy := "." + fx.cfg.StateFile + ".brief-tmp"
	require.NoError(t, os.WriteFile(filepath.Join(fx.featureDir(), decoy), []byte("leftover"), 0o600))

	extra := unexpectedFiles(t, fx.featureDir(), fixtureFileNames(fx.cfg))

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

func Test_refuses_a_step_file_with_no_handoff_anchor(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, "STEP-02.md"), []byte(step), 0o600))

	srv := scaffold.NewServer(cfg, root)
	before := snapshotTree(t, featureDir)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", []byte("h"), []byte("s"))

	require.ErrorIs(t, err, scaffold.ErrMalformedFeature)
	assert.Equal(t, before, snapshotTree(t, featureDir))
}

// Test_refuses_a_step_file_whose_handoff_heading_is_swallowed_by_an_earlier_open_fence
// reproduces the reviewer's MAJOR directly: an unterminated fence opened
// before the handoff heading's own line makes that line unreachable —
// findHeading, fence-aware like every scanner in this package, never
// treats a line inside an open fence as a heading — so the heading is
// simply never found. Refusing with "no heading found" there would be
// untrue and actively harmful: following its advice and adding another
// "## Handoff" heading would land inside the same still-open fence and
// change nothing. Finish must instead name the fence, on the step file,
// at the line it opened.
func Test_refuses_a_step_file_whose_handoff_heading_is_swallowed_by_an_earlier_open_fence(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	stepPath := filepath.Join(featureDir, "STEP-02.md")
	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n```\nunterminated\n" + cfg.HandoffHeading + "\nstill inside the fence\n"
	require.NoError(t, os.WriteFile(stepPath, []byte(step), 0o600))

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n- [ ] STEP-02\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))

	state := "## Decisions Fixture\n\n## Left Fixture\n\n## Gotchas\n\n## Debts Fixture\n"
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))

	before := snapshotTree(t, featureDir)

	srv := scaffold.NewServer(cfg, root)

	err := srv.Finish(context.Background(), "widgets", "STEP-02", []byte("h"), []byte(state))

	require.Error(t, err)
	require.ErrorIs(t, err, scaffold.ErrUnterminatedFence)
	require.NotErrorIs(t, err, scaffold.ErrMalformedFeature,
		"the fence is the true cause; \"no heading found\" would be untrue")

	var refusal *scaffold.RefusalError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, stepPath, refusal.Path)
	assert.Equal(t, 9, refusal.Line, "the fence opens on the step file's own 9th line")

	after := snapshotTree(t, featureDir)
	assert.Equal(t, before, after, "a refused finish must leave every file byte-identical")
}

func Test_refuses_a_specification_with_no_progress_heading_on_finish(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	step := "---\nid: STEP-02\nstatus: open\ndepends-on: []\n---\n\n" +
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + cfg.HandoffHeading + "\n"
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
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + cfg.HandoffHeading + "\n"
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
		"# STEP-02\n\n" + cfg.ChecklistHeading + "\n\n- [x] done\n\n" + cfg.HandoffHeading + "\n"
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
}
