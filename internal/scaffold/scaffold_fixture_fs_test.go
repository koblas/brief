package scaffold_test

import (
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/stretchr/testify/require"
)

// testSpecsRoot is the absolute-looking OS directory every Mem-backed
// NewFeature/NewStep fixture in this package is rooted at, one level above
// the finish fixtures' own testFeaturePath.
const testSpecsRoot = "/repo/specs"

// newFeatureRootFS returns a fresh, empty rwfs.Mem standing in for the
// configured feature directory.
func newFeatureRootFS(t *testing.T) rwfs.FS {
	t.Helper()

	return rwfs.NewMem(fstest.MapFS{})
}

// createFeatureFS calls NewFeatureFS against top (testSpecsRoot) for name,
// requiring success.
func createFeatureFS(t *testing.T, srv *scaffold.Server, top rwfs.FS, name string) scaffold.Result {
	t.Helper()

	res, err := srv.NewFeatureFS(top, testSpecsRoot, name)
	require.NoError(t, err)

	return res
}

// openFeatureViewFS opens name as a nested view of top, requiring success.
func openFeatureViewFS(t *testing.T, top rwfs.FS, name string) rwfs.FS {
	t.Helper()

	view, err := top.OpenRoot(name)
	require.NoError(t, err)

	return view
}
