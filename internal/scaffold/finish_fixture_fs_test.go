package scaffold_test

import (
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/require"
)

// testFeaturePath is the absolute-looking OS directory every Mem-backed
// finish fixture in this package is rooted at — the assemble.FeatureFS
// convention (internal/assemble/fs_test.go's testFeaturePath). Nothing
// reads it from disk: it only proves a Result/FinishResult/RefusalError
// path is featurePath joined with the file's own name inside the fixture's
// rwfs.Mem, the same way a real feature directory would produce it.
const testFeaturePath = "/repo/specs/widgets"

// finishFixtureFS is newFinishFixture's in-memory counterpart: the same
// "widgets" feature content (three step files, a state file and a
// specification carrying a progress entry for each step), built on an
// rwfs.Mem instead of t.TempDir(), plus the pattern/handoffPattern
// FinishFS needs and the handoff/state bytes a happy-path
// FinishFS("widgets", "STEP-02", ...) call supplies.
type finishFixtureFS struct {
	mem            *rwfs.Mem
	cfg            config.Config
	pattern        stepfile.Pattern
	handoffPattern stepfile.HandoffPattern
	newHandoff     []byte
	newState       []byte
}

// stepPath returns the absolute-looking path of one of the fixture's step
// files, for assertions against a RefusalError.Path, an err.Error() or a
// Result path — finishFixture.stepPath's Mem counterpart.
func (fx finishFixtureFS) stepPath(name string) string {
	return filepath.Join(testFeaturePath, name)
}

// handoffName returns the fixture's target step's handoff file's name,
// relative to fx.mem — the name a direct fx.mem.ReadFile/WriteFile call
// takes, as opposed to handoffPath's absolute, assertion-facing form.
func (fx finishFixtureFS) handoffName() string {
	return "STEP-02" + fx.cfg.HandoffFileSuffix
}

// handoffPath returns the path of the fixture's target step's handoff
// file, deriving the name by literal string concatenation rather than
// through stepfile.CompileHandoff, matching finishFixture.handoffPath.
func (fx finishFixtureFS) handoffPath() string {
	return filepath.Join(testFeaturePath, fx.handoffName())
}

// finish runs FinishFS against the fixture's own rwfs.Mem, feature
// "widgets" — the Mem counterpart of calling
// scaffold.NewServer(fx.cfg, fx.root).Finish(ctx, "widgets", step, ...)
// against a disk fixture.
func (fx finishFixtureFS) finish(t *testing.T, step string, handoff, state []byte) (scaffold.FinishResult, error) {
	t.Helper()

	srv := scaffold.NewServer(fx.cfg, "")

	return srv.FinishFS(fx.mem, testFeaturePath, "widgets", step, handoff, state, fx.pattern, fx.handoffPattern)
}

// stepfilePattern compiles cfg's own step-file pattern, for a test that
// builds its own bespoke Mem fixture rather than newFinishFixtureFS's.
func stepfilePattern(cfg config.Config) (stepfile.Pattern, error) {
	return stepfile.Compile(cfg.StepFilePattern)
}

// stepfileHandoffPattern compiles cfg's own handoff-file pattern against
// pattern, the counterpart of stepfilePattern for a test's own bespoke
// fixture.
func stepfileHandoffPattern(cfg config.Config, pattern stepfile.Pattern) (stepfile.HandoffPattern, error) {
	return stepfile.CompileHandoff(pattern, cfg.HandoffFileSuffix, cfg.StateFile, cfg.SpecificationFile)
}

// newFinishFixtureFS builds newFinishFixture's exact "widgets" feature
// content on an rwfs.Mem: three step files (STEP-01 done, STEP-02 the
// target, STEP-03 untouched), a state file and a specification carrying a
// progress entry for each step, using fixtureConfig so every field this
// package reads differs from config.Default().
func newFinishFixtureFS(t *testing.T) finishFixtureFS {
	t.Helper()

	cfg := fixtureConfig()

	pattern, err := stepfile.Compile(cfg.StepFilePattern)
	require.NoError(t, err)
	handoffPattern, err := stepfile.CompileHandoff(pattern, cfg.HandoffFileSuffix, cfg.StateFile, cfg.SpecificationFile)
	require.NoError(t, err)

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

	spec := "# widgets\n\n" + cfg.ProgressHeading + "\n\n" +
		"- [x] STEP-01: already done\n" +
		"- [ ] STEP-02: Assemble the thing\n" +
		"- [ ] STEP-03\n" +
		"\n" +
		"## Notes\n" +
		"\n" +
		"Something else entirely.\n"

	mem := rwfs.NewMem(fstest.MapFS{
		"STEP-01.md":          &fstest.MapFile{Data: []byte(step01), Mode: 0o600},
		"STEP-02.md":          &fstest.MapFile{Data: []byte(step02Body(cfg)), Mode: 0o600},
		"STEP-03.md":          &fstest.MapFile{Data: []byte(step03), Mode: 0o600},
		cfg.StateFile:         &fstest.MapFile{Data: []byte(oldStateBody(cfg)), Mode: 0o600},
		cfg.SpecificationFile: &fstest.MapFile{Data: []byte(spec), Mode: 0o600},
	})

	return finishFixtureFS{
		mem: mem, cfg: cfg, pattern: pattern, handoffPattern: handoffPattern,
		newHandoff: []byte("NEW-HANDOFF-02\n\n```\n# not a heading\n```\n"),
		newState:   newStateBody(cfg),
	}
}

// putStepFS replaces fx's own name entry (a step file, "STEP-02.md" in
// every caller so far) with body, for a test that needs the fixture's
// target step to carry content newFinishFixtureFS did not seed — the Mem
// counterpart of os.WriteFile(fx.stepPath(name), ...).
func putStepFS(t *testing.T, fx finishFixtureFS, name, body string) {
	t.Helper()

	require.NoError(t, fx.mem.WriteFile(name, []byte(body), 0o600))
}

// newFinishedFixtureFS builds newFinishFixtureFS's "widgets" feature and
// runs one FinishFS call with its handoff and state, so every test built
// against it opens on a step that is already done — the Mem counterpart of
// newFinishedFixture.
func newFinishedFixtureFS(t *testing.T) finishFixtureFS {
	t.Helper()

	fx := newFinishFixtureFS(t)

	_, err := fx.finish(t, "STEP-02", fx.newHandoff, fx.newState)
	require.NoError(t, err)

	return fx
}
