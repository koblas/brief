// These Check scenarios stay on real disk: a step file symlinked outside
// its feature root, a symlinked or unreadable feature directory, and the
// dual all-features/named-feature dispatch Check itself owns. CheckFS's
// own content rules are pinned against fstest.MapFS in check_test.go.
package assemble_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checkFeatureDir returns, and creates, the path of feature under root's
// configured feature directory.
func checkFeatureDir(cfg config.Config, root, feature string) string {
	return filepath.Join(root, cfg.FeatureDirectory, feature)
}

// checkWriteFeature writes spec and state at their configured names under
// featureDir.
func checkWriteFeature(t *testing.T, cfg config.Config, featureDir, spec, state string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(featureDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.SpecificationFile), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(featureDir, cfg.StateFile), []byte(state), 0o600))
}

// checkWriteStep writes one step file under featureDir.
func checkWriteStep(t *testing.T, featureDir, id, body string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(featureDir, id+".md"), []byte(body), 0o600))
}

// checkWriteHandoff writes id's handoff file under featureDir, using cfg's
// configured suffix.
func checkWriteHandoff(t *testing.T, cfg config.Config, featureDir, id, body string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(featureDir, id+cfg.HandoffFileSuffix), []byte(body), 0o600))
}

// diskRuleCase is one row of the table below: setup builds the smallest
// real-disk fixture that trips exactly one of Check's OS-adapter
// producers and returns the Server plus the feature argument to check.
type diskRuleCase struct {
	name     string
	setup    func(t *testing.T) (*assemble.Server, string)
	wantRule assemble.Rule
}

// diskRuleCaseStepUnreadable builds a feature whose step file is a symlink
// escaping the feature root, so reading it fails: os.Root refuses to
// follow it, a containment behavior no in-memory fs.FS reproduces.
func diskRuleCaseStepUnreadable(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	require.NoError(t, os.Symlink(filepath.Join(featureDir, "nonexistent-target.md"), filepath.Join(featureDir, "STEP-01.md")))

	return assemble.NewServer(cfg, root), "demo"
}

// diskRuleCaseFeatureSymlink builds a symlink where a feature directory is
// expected.
func diskRuleCaseFeatureSymlink(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	realDir := filepath.Join(root, "real-demo")
	checkWriteFeature(t, cfg, realDir, checkConformingSpec(cfg), checkConformingState(cfg))
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	require.NoError(t, os.Symlink(realDir, filepath.Join(root, cfg.FeatureDirectory, "demo")))

	return assemble.NewServer(cfg, root), "demo"
}

// diskRuleCaseFeatureUnreadable builds a Server whose openRoot seam is
// overridden to fail opening a feature directory.
func diskRuleCaseFeatureUnreadable(t *testing.T) (*assemble.Server, string) {
	t.Helper()

	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	srv := assemble.NewServer(cfg, root)
	assemble.SetOpenRootForTest(srv, func(parent *os.Root, name string) (*os.Root, error) {
		if name == "demo" {
			return nil, &fs.PathError{Op: "openat", Path: name, Err: syscall.EACCES}
		}

		return parent.OpenRoot(name)
	})

	return srv, "demo"
}

// The OS-adapter half of the rule-id table: three producers
// check_test.go's own MapFS table cannot reach.
func Test_check_assigns_each_producer_its_stable_rule_id_disk(t *testing.T) {
	cases := []diskRuleCase{
		{name: "step unreadable", setup: diskRuleCaseStepUnreadable, wantRule: assemble.RuleStepUnreadable},
		{name: "feature symlink", setup: diskRuleCaseFeatureSymlink, wantRule: assemble.RuleFeatureSymlink},
		{name: "feature unreadable", setup: diskRuleCaseFeatureUnreadable, wantRule: assemble.RuleFeatureUnreadable},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, feature := c.setup(t)

			findings, err := srv.Check(t.Context(), feature)
			require.NoError(t, err)

			f := onlyFinding(t, findings)
			assert.Equal(t, c.wantRule, f.Rule)
		})
	}
}

// Pins Finding.Feature/FeaturePath/InFlight through both the all-features
// scan and the named-feature path.
func Test_check_stamps_an_in_flight_ordinary_feature_finding_with_its_name_path_and_in_flight(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "open", []string{"STEP-99"}, []string{"- [ ] a task"}))

	srv := assemble.NewServer(cfg, root)

	allFindings, err := srv.Check(t.Context(), "")
	require.NoError(t, err)
	allFinding := onlyFinding(t, allFindings)
	assert.Equal(t, "demo", allFinding.Feature)
	assert.Equal(t, featureDir, allFinding.FeaturePath)
	assert.True(t, allFinding.InFlight)

	namedFindings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)
	namedFinding := onlyFinding(t, namedFindings)
	assert.Equal(t, "demo", namedFinding.Feature)
	assert.Equal(t, featureDir, namedFinding.FeaturePath)
	assert.True(t, namedFinding.InFlight)
}

// Complete-feature counterpart: InFlight is false through both entry paths.
func Test_check_stamps_a_complete_ordinary_feature_finding_with_its_name_path_and_in_flight(t *testing.T) {
	cfg := fixtureConfig()
	cfg.HandoffCapLines = 10
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")

	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, featureDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))
	checkWriteHandoff(t, cfg, featureDir, "STEP-01", string(checkBodyOfLines(11)))

	srv := assemble.NewServer(cfg, root)

	allFindings, err := srv.Check(t.Context(), "")
	require.NoError(t, err)
	allFinding := onlyFinding(t, allFindings)
	assert.Equal(t, "demo", allFinding.Feature)
	assert.Equal(t, featureDir, allFinding.FeaturePath)
	assert.False(t, allFinding.InFlight)

	namedFindings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)
	namedFinding := onlyFinding(t, namedFindings)
	assert.Equal(t, "demo", namedFinding.Feature)
	assert.Equal(t, featureDir, namedFinding.FeaturePath)
	assert.False(t, namedFinding.InFlight)
}

// Pins the same three fields for symlinkFeatureFinding, which never
// passes through CheckFS's own stamping loop.
func Test_check_stamps_a_symlinked_feature_finding_with_its_own_name_path_and_in_flight_true(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	realDir := filepath.Join(root, "real-gamma")
	checkWriteFeature(t, cfg, realDir, checkConformingSpec(cfg), checkConformingState(cfg))

	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	linkPath := filepath.Join(root, cfg.FeatureDirectory, "gamma")
	require.NoError(t, os.Symlink(realDir, linkPath))

	srv := assemble.NewServer(cfg, root)

	allFindings, err := srv.Check(t.Context(), "")
	require.NoError(t, err)
	allFinding := onlyFinding(t, allFindings)
	assert.Equal(t, "gamma", allFinding.Feature)
	assert.Equal(t, linkPath, allFinding.FeaturePath)
	assert.True(t, allFinding.InFlight)

	namedFindings, err := srv.Check(t.Context(), "gamma")
	require.NoError(t, err)
	namedFinding := onlyFinding(t, namedFindings)
	assert.Equal(t, "gamma", namedFinding.Feature)
	assert.Equal(t, linkPath, namedFinding.FeaturePath)
	assert.True(t, namedFinding.InFlight)
}

// unreadableFeatureFinding's counterpart to the symlink test above.
func Test_check_stamps_an_unreadable_feature_finding_with_its_own_name_path_and_in_flight_true(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	injectUnreadable := func(parent *os.Root, name string) (*os.Root, error) {
		if name == "demo" {
			return nil, &fs.PathError{Op: "openat", Path: name, Err: syscall.EACCES}
		}

		return parent.OpenRoot(name)
	}

	allSrv := assemble.NewServer(cfg, root)
	assemble.SetOpenRootForTest(allSrv, injectUnreadable)

	allFindings, err := allSrv.Check(t.Context(), "")
	require.NoError(t, err)
	allFinding := onlyFinding(t, allFindings)
	assert.Equal(t, "demo", allFinding.Feature)
	assert.Equal(t, featureDir, allFinding.FeaturePath)
	assert.True(t, allFinding.InFlight)

	namedSrv := assemble.NewServer(cfg, root)
	assemble.SetOpenRootForTest(namedSrv, injectUnreadable)

	namedFindings, err := namedSrv.Check(t.Context(), "demo")
	require.NoError(t, err)
	namedFinding := onlyFinding(t, namedFindings)
	assert.Equal(t, "demo", namedFinding.Feature)
	assert.Equal(t, featureDir, namedFinding.FeaturePath)
	assert.True(t, namedFinding.InFlight)
}

func Test_check_returns_nil_when_the_feature_directory_root_does_not_exist(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "")
	require.NoError(t, err)
	assert.Nil(t, findings)
}

func Test_check_refuses_an_unknown_named_feature(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "ghost")
	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
	assert.Nil(t, findings)
}

// A malformed sibling feature must not contribute findings when only one
// feature is named.
func Test_check_checks_only_the_named_feature(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	goodDir := checkFeatureDir(cfg, root, "alpha")
	checkWriteFeature(t, cfg, goodDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, goodDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))

	badDir := checkFeatureDir(cfg, root, "beta")
	checkWriteFeature(t, cfg, badDir, "# beta\n\nno progress list here\n", checkConformingState(cfg))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "alpha")
	require.NoError(t, err)
	assert.Empty(t, findings)
}

// Named-feature half of the root-missing case: Check("") degrades a
// missing root to zero features, but a named feature must refuse instead.
func Test_check_refuses_a_named_feature_when_the_feature_root_does_not_exist(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "ghost")
	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
	assert.Nil(t, findings)
}

// A regular file where a feature directory belongs is ErrNoSuchFeature,
// the same as a name with no entry at all, never an "unreadable" Finding.
func Test_check_refuses_a_named_feature_that_is_a_regular_file(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, cfg.FeatureDirectory, "README.md"), []byte("not a feature\n"), 0o600))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "README.md")
	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
	assert.Nil(t, findings)
}

// A feature directory that exists but cannot be opened as its own root
// must contribute a Finding naming it, not silently zero findings.
func Test_check_reports_an_unreadable_feature_directory_in_the_all_features_scan(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	srv := assemble.NewServer(cfg, root)
	assemble.SetOpenRootForTest(srv, func(parent *os.Root, name string) (*os.Root, error) {
		if name == "demo" {
			return nil, &fs.PathError{Op: "openat", Path: name, Err: syscall.EACCES}
		}

		return parent.OpenRoot(name)
	})

	findings, err := srv.Check(t.Context(), "")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, featureDir, f.Path)
	assert.Contains(t, f.Detail, "permission denied")
	assert.Equal(t, assemble.SeverityError, f.Severity)
}

// Named-feature counterpart: must report the same Finding, not collapse
// into ErrNoSuchFeature.
func Test_check_reports_an_unreadable_named_feature_directory(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, "demo")
	require.NoError(t, os.MkdirAll(featureDir, 0o755))

	srv := assemble.NewServer(cfg, root)
	assemble.SetOpenRootForTest(srv, func(parent *os.Root, name string) (*os.Root, error) {
		if name == "demo" {
			return nil, &fs.PathError{Op: "openat", Path: name, Err: syscall.EACCES}
		}

		return parent.OpenRoot(name)
	})

	findings, err := srv.Check(t.Context(), "demo")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, featureDir, f.Path)
	assert.Contains(t, f.Detail, "permission denied")
	assert.Equal(t, assemble.SeverityError, f.Severity)
}

// "." and ".." would otherwise reopen the feature-directory root itself
// (or its parent) as if it were a feature.
func Test_check_refuses_a_feature_argument_containing_a_path_separator(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	// Each case below would succeed without the guard, unlike a bare "a/b".
	for _, feature := range []string{".", "..", "demo/../.."} {
		findings, err := srv.Check(t.Context(), feature)
		require.ErrorIsf(t, err, assemble.ErrNoSuchFeature, "feature %q", feature)
		assert.Nilf(t, findings, "feature %q", feature)
	}
}

// A backslash is a legal POSIX filename character, not a separator, so a
// feature genuinely named with one must remain checkable.
func Test_check_is_reachable_for_a_feature_name_containing_a_backslash(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	featureDir := checkFeatureDir(cfg, root, `foo\bar`)
	checkWriteFeature(t, cfg, featureDir, checkConformingSpec(cfg), checkConformingState(cfg))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), `foo\bar`)

	require.NoError(t, err)
	assert.Empty(t, findings)
}

// The target is a conforming feature, proving the Finding fires because
// brief never follows the link, not because the target is malformed.
func Test_check_marks_a_symlinked_feature_directory_rather_than_following_it(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	realDir := filepath.Join(root, "real-gamma")
	checkWriteFeature(t, cfg, realDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, realDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))

	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	linkPath := filepath.Join(root, cfg.FeatureDirectory, "gamma")
	require.NoError(t, os.Symlink(realDir, linkPath))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, linkPath, f.Path)
	assert.Contains(t, f.Detail, "symbolic link")
	assert.Equal(t, assemble.SeverityError, f.Severity)
}

// Named-feature half of the same stance: naming the symlink directly
// must not follow it either.
func Test_check_marks_a_named_symlinked_feature_directory_rather_than_following_it(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	realDir := filepath.Join(root, "real-gamma")
	checkWriteFeature(t, cfg, realDir, checkConformingSpec(cfg), checkConformingState(cfg))
	checkWriteStep(t, realDir, "STEP-01", checkStepBody(cfg, "STEP-01", "done", nil, []string{"- [x] first thing"}))

	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	linkPath := filepath.Join(root, cfg.FeatureDirectory, "gamma")
	require.NoError(t, os.Symlink(realDir, linkPath))

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "gamma")
	require.NoError(t, err)

	f := onlyFinding(t, findings)
	assert.Equal(t, linkPath, f.Path)
	assert.Contains(t, f.Detail, "symbolic link")
	assert.Equal(t, assemble.SeverityError, f.Severity)
}
