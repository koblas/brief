// These Check scenarios stay on real disk: a step file symlinked outside
// its feature root (os.Root's own containment refusal), a symlinked or
// unreadable feature directory at the top-level adapter Check builds
// around one feature's own name, the openRoot test-injection seam, and the
// dual all-features/named-feature dispatch Check itself owns — none of
// which CheckFS, sitting one level below that adapter, ever reads through.
// CheckFS's own content rules are pinned against fstest.MapFS in
// check_test.go.
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

// diskRuleCase is one row of
// Test_check_assigns_each_producer_its_stable_rule_id_disk's table: setup
// builds the smallest real-disk fixture (or Server override) that trips
// exactly one of Check's OS-adapter producers and returns the Server plus
// the feature argument to check; wantRule is that producer's own stable
// Rule. check_test.go's own table covers every content producer.
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

// Test_check_assigns_each_producer_its_stable_rule_id_disk is R8's
// OS-adapter half: the three producers check_test.go's own MapFS table
// cannot reach, since each concerns the os.Root containment chain Check
// builds around one feature's own name, not CheckFS's content rules.
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

// Test_check_stamps_an_in_flight_ordinary_feature_finding_with_its_name_path_and_in_flight
// pins Finding.Feature/FeaturePath/InFlight for an ordinary (non
// feature-level) finding on a feature still in flight, through both the
// all-features scan and the named-feature path — Check's own dual
// dispatch, one level above anything CheckFS reads.
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

// Test_check_stamps_a_complete_ordinary_feature_finding_with_its_name_path_and_in_flight
// is the complete-feature counterpart: InFlight is false when every step
// reads as done, through both entry paths.
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

// Test_check_stamps_a_symlinked_feature_finding_with_its_own_name_path_and_in_flight_true
// pins the same three fields for symlinkFeatureFinding, which never passes
// through CheckFS's own stamping loop — the trap a new feature-level
// producer must not repeat.
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

// Test_check_stamps_an_unreadable_feature_finding_with_its_own_name_path_and_in_flight_true
// is unreadableFeatureFinding's counterpart to the symlink test above — the
// same trap, the other feature-level producer.
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

// Test_check_checks_only_the_named_feature is the scoping half of the
// contract: a malformed sibling feature must not contribute findings when
// only one feature is named — a claim about which directories Check's own
// enumeration reads, not about CheckFS's content rules.
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

// Test_check_refuses_a_named_feature_when_the_feature_root_does_not_exist
// is the named-feature half of the root-missing case: Check("") degrades a
// missing feature-directory root to zero features (nil, nil), but a named
// feature obviously has no directory when the root holding it does not
// exist either, so it must refuse with ErrNoSuchFeature rather than share
// the empty-repository degrade.
func Test_check_refuses_a_named_feature_when_the_feature_root_does_not_exist(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(t.Context(), "ghost")
	require.ErrorIs(t, err, assemble.ErrNoSuchFeature)
	assert.Nil(t, findings)
}

// Test_check_refuses_a_named_feature_that_is_a_regular_file pins the third
// leg of the same precedence chain: a name that exists under the feature
// directory but is a regular file, not a directory, is not a feature
// either — ErrNoSuchFeature, the same as a name with no entry at all,
// never an "unreadable" Finding.
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

// Test_check_reports_an_unreadable_feature_directory_in_the_all_features_scan
// is C3's companion: a feature directory that exists but cannot be opened
// as its own root must contribute a Finding naming it, not silently zero
// findings — the exact backstop failure R18 gives Check to prevent. The
// injected failure is a fake open, not chmod: root bypasses permission
// checks, so this must reproduce identically under any CI identity.
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

// Test_check_reports_an_unreadable_named_feature_directory is the
// named-feature counterpart: naming the same unreadable feature directly
// must report the same Finding, not collapse it into ErrNoSuchFeature —
// the directory plainly exists, it just could not be opened.
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

// Test_check_refuses_a_feature_argument_containing_a_path_separator pins
// validFeatureArgument's rejection of a multi-component feature name: "."
// and ".." would otherwise reopen the feature-directory root itself (or
// its parent) as if it were a feature, and a multi-component argument
// would reach a nested directory no "brief new" or "brief finish" call
// ever named — none of those are a feature this configuration knows about.
func Test_check_refuses_a_feature_argument_containing_a_path_separator(t *testing.T) {
	cfg := fixtureConfig()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))

	srv := assemble.NewServer(cfg, root)

	// "a/b" is deliberately absent: os.Root.Lstat rejects it as ErrNotExist
	// against this empty fixture whether or not the separator guard ran, so
	// it would not discriminate the guard from its absence. Each entry below
	// does: "." reopens the feature root itself and would succeed without the
	// guard, and the two escaping forms resolve to a path-escape error
	// distinct from ErrNotExist.
	for _, feature := range []string{".", "..", "demo/../.."} {
		findings, err := srv.Check(t.Context(), feature)
		require.ErrorIsf(t, err, assemble.ErrNoSuchFeature, "feature %q", feature)
		assert.Nilf(t, findings, "feature %q", feature)
	}
}

// Test_check_is_reachable_for_a_feature_name_containing_a_backslash pins
// validFeatureArgument to POSIX path-separator rules: a backslash is a
// legal filename character on POSIX, not a separator, so a feature
// genuinely named with one must remain checkable rather than being
// rejected as though it were a multi-component argument. os.IsPathSeparator
// is what makes this platform-correct rather than hardcoding the POSIX
// answer, so no build guard is needed here.
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

// Test_check_marks_a_symlinked_feature_directory_rather_than_following_it
// pins Check's symlink stance to Status's: mark, never follow. The target
// is a conforming feature, proving the Finding fires because brief never
// follows the link, not because anything about the target is malformed.
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

// Test_check_marks_a_named_symlinked_feature_directory_rather_than_following_it
// is the named-feature half of the same stance: naming the symlink
// directly must not follow it either.
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
