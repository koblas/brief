package scaffold_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/require"
)

// redactRoot replaces every occurrence of root in s with a stable
// placeholder, so a message built from a t.TempDir() path can be compared
// with assert.Equal across runs.
func redactRoot(s, root string) string {
	return strings.ReplaceAll(s, root, "<root>")
}

// This file pins the exact user-visible error text scaffold.NewFeature,
// NewStep and Finish produce for a write blocked by a filesystem obstacle
// or a directory standing where a feature entry belongs — text cli prints
// verbatim. It runs only on the OS adapter: the obstacles it plants (a
// directory at atomicfile's own temp-sibling name, a regular file standing
// in for a directory) are real-filesystem mechanics rwfs.Mem does not
// reproduce.

func Test_finish_blocked_at_the_handoff_write_reports_the_atomicfile_rename_failure(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	blocked := filepath.Join(fx.featureDir(), "STEP-02"+fx.cfg.HandoffFileSuffix)
	require.NoError(t, os.Mkdir(blocked, 0o755))

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)

	require.Error(t, err)
	require.Equal(t,
		"scaffold: atomicfile: rename STEP-02.fixture-handoff.md: "+
			"renameat .STEP-02.fixture-handoff.md.brief-tmp STEP-02.fixture-handoff.md: file exists; "+
			"run 'brief finish widgets STEP-02 --handoff <path> --state <path>' to retry",
		err.Error())
}

func Test_finish_blocked_at_the_state_write_reports_the_atomicfile_open_failure(t *testing.T) {
	fx := newFinishFixture(t)
	srv := scaffold.NewServer(fx.cfg, fx.root)
	blocked := filepath.Join(fx.featureDir(), "."+fx.cfg.StateFile+".brief-tmp")
	require.NoError(t, os.Mkdir(blocked, 0o755))

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", fx.newHandoff, fx.newState)

	require.Error(t, err)
	require.Equal(t,
		"scaffold: atomicfile: write NOTES.md: openat .NOTES.md.brief-tmp: is a directory; "+
			"run 'brief finish widgets STEP-02 --handoff <path> --state <path>' to retry",
		err.Error())
}

func Test_new_step_blocked_at_the_specification_write_reports_the_atomicfile_open_failure(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)
	_, err := srv.NewFeature(context.Background(), "widgets")
	require.NoError(t, err)

	featureDir := filepath.Join(root, cfg.FeatureDirectory, "widgets")
	blocked := filepath.Join(featureDir, "."+cfg.SpecificationFile+".brief-tmp")
	require.NoError(t, os.Mkdir(blocked, 0o755))

	_, err = srv.NewStep(context.Background(), "widgets")

	require.Error(t, err)
	require.Equal(t, "scaffold: atomicfile: write SPEC.md: openat .SPEC.md.brief-tmp: is a directory", err.Error())
}

func Test_new_feature_blocked_at_the_specification_write_reports_the_openat_failure(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	cfg.SpecificationFile = filepath.Join("sub", "SPEC.md")
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewFeature(context.Background(), "widgets")

	require.Error(t, err)
	require.Equal(t, "write widgets/sub/SPEC.md: openat widgets/sub/SPEC.md: no such file or directory", err.Error())
}

func Test_new_feature_blocked_at_the_state_write_reports_the_openat_failure(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	cfg.StateFile = cfg.SpecificationFile
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewFeature(context.Background(), "widgets")

	require.Error(t, err)
	require.Equal(t, "write widgets/SPEC.md: openat widgets/SPEC.md: file exists", err.Error())
}

func Test_new_step_reports_the_configured_feature_directory_s_own_path_when_it_is_a_regular_file(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	featureDirPath := filepath.Join(root, cfg.FeatureDirectory)
	require.NoError(t, os.WriteFile(featureDirPath, []byte("not a directory"), 0o600))
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewStep(context.Background(), "widgets")

	require.Error(t, err)
	require.Equal(t, "scaffold: open feature widgets: open <root>/specs: not a directory", redactRoot(err.Error(), root))
}

func Test_finish_reports_the_configured_feature_directory_s_own_path_when_it_is_a_regular_file(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	featureDirPath := filepath.Join(root, cfg.FeatureDirectory)
	require.NoError(t, os.WriteFile(featureDirPath, []byte("not a directory"), 0o600))
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", []byte("h"), []byte("s"))

	require.Error(t, err)
	require.Equal(t, "scaffold: open feature widgets: open <root>/specs: not a directory", redactRoot(err.Error(), root))
}

// peelWriteErr deliberately does not touch this case: OpenRoot classifies
// "exists but is not a directory" as a bare syscall.ENOTDIR, with no path
// of its own, so the *fs.PathError wrapping (Op "openroot") stays.
func Test_new_step_reports_a_bare_not_a_directory_when_the_feature_entry_itself_is_a_regular_file(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, cfg.FeatureDirectory, "widgets"), []byte("not a directory"), 0o600))
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewStep(context.Background(), "widgets")

	require.Error(t, err)
	require.Equal(t, "scaffold: open feature widgets: openroot widgets: not a directory", err.Error())
}

func Test_finish_reports_a_bare_not_a_directory_when_the_feature_entry_itself_is_a_regular_file(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	require.NoError(t, os.MkdirAll(filepath.Join(root, cfg.FeatureDirectory), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, cfg.FeatureDirectory, "widgets"), []byte("not a directory"), 0o600))
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.Finish(context.Background(), "widgets", "STEP-02", []byte("h"), []byte("s"))

	require.Error(t, err)
	require.Equal(t, "scaffold: open feature widgets: openroot widgets: not a directory", err.Error())
}

// The other case peelWriteErr does not touch: Mkdir rejects "../escaped"
// via fs.ValidPath before os.Root runs, so the *fs.PathError wrapping (Op
// "mkdir") stays, naming what was rejected.
func Test_new_feature_reports_invalid_argument_for_a_traversing_name(t *testing.T) {
	root := t.TempDir()
	cfg := fixtureConfig()
	srv := scaffold.NewServer(cfg, root)

	_, err := srv.NewFeature(context.Background(), "../escaped")

	require.Error(t, err)
	require.Equal(t, "scaffold: mkdir ../escaped: invalid argument", err.Error())
}
