package scaffold_test

import (
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/require"
)

// testSpecsRoot is the absolute-looking OS directory every Mem-backed
// NewFeature/NewStep fixture in this package is rooted at — the configured
// feature directory itself ("specs" in fixtureConfig()). It equals
// filepath.Join(testSpecsRoot, "widgets"), the finish fixtures' own
// testFeaturePath one level deeper.
const testSpecsRoot = "/repo/specs"

// newFeatureRootFS returns a fresh, empty rwfs.Mem standing in for the
// configured feature directory — the Mem counterpart of
// os.MkdirAll(featureRoot) followed by rwfs.OpenOS(featureRoot) inside
// NewFeature.
func newFeatureRootFS(t *testing.T) rwfs.FS {
	t.Helper()

	return rwfs.NewMem(fstest.MapFS{})
}

// createFeatureFS calls NewFeatureFS against top (testSpecsRoot) for name,
// requiring success — the Mem counterpart of a disk fixture's
// srv.NewFeature(ctx, name) call.
func createFeatureFS(t *testing.T, srv *scaffold.Server, top rwfs.FS, name string) scaffold.Result {
	t.Helper()

	res, err := srv.NewFeatureFS(top, testSpecsRoot, name)
	require.NoError(t, err)

	return res
}

// openFeatureViewFS opens name as a nested view of top, requiring
// success — the Mem counterpart of the second of openFeatureDir's two
// opens, for a test that then calls NewStepFS or FinishFS against it.
func openFeatureViewFS(t *testing.T, top rwfs.FS, name string) rwfs.FS {
	t.Helper()

	view, err := top.OpenRoot(name)
	require.NoError(t, err)

	return view
}
