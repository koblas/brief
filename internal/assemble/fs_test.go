package assemble_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/stepfile"
	"github.com/stretchr/testify/require"
)

// testFeaturePath is the absolute-looking OS directory every MapFS-backed
// FeatureFS in this package's tests is rooted at. Nothing reads it from
// disk; it only proves a Problem/Finding/RefusalError/Shortfall path is
// FeatureFS.Path joined with the file's own name inside FS, the same way a
// real feature directory would produce it.
const testFeaturePath = "/repo/docs/specifications/demo"

// featureFS builds a FeatureFS over an in-memory filesystem holding files —
// each key the file's name relative to the feature directory, each value
// its content — rooted at testFeaturePath.
func featureFS(files map[string]string) assemble.FeatureFS {
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}

	return assemble.FeatureFS{FS: fsys, Path: testFeaturePath}
}

// failFS wraps an in-memory filesystem, replacing the result of reading
// failReadFile or listing failReadDir with err: the in-memory substitute
// for injecting a permission-denied read or directory listing into
// CheckFS or StatusFS, without depending on OS permission bits or
// effective uid. Either field left empty never fails that operation.
type failFS struct {
	fs.FS

	failReadFile string
	failReadDir  string
	err          error
}

// ReadFile implements fs.ReadFileFS, so fs.ReadFile(f, name) calls this
// directly rather than falling back to Open.
func (f failFS) ReadFile(name string) ([]byte, error) {
	if name == f.failReadFile {
		return nil, &fs.PathError{Op: "open", Path: name, Err: f.err}
	}

	return fs.ReadFile(f.FS, name)
}

// ReadDir implements fs.ReadDirFS, so fs.ReadDir(f, name) calls this
// directly rather than falling back to Open.
func (f failFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == f.failReadDir {
		return nil, &fs.PathError{Op: "readdirent", Path: name, Err: f.err}
	}

	return fs.ReadDir(f.FS, name)
}

// conformingFeatureFiles returns a specification and state body that pass
// specFault/readStateFile — the same two checks StartFS refuses on — keyed
// by cfg's own configured names, so a MapFS-backed fixture can start from a
// clean base and add or override step files.
func conformingFeatureFiles(cfg config.Config) map[string]string {
	return map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		cfg.StateFile:         checkConformingState(cfg),
	}
}

// statusPattern compiles cfg's own step-file pattern, the same way
// (*assemble.Server).Status does once per call, for a test that calls
// StatusFS directly against a MapFS fixture.
func statusPattern(t *testing.T, cfg config.Config) stepfile.Pattern {
	t.Helper()

	pattern, err := stepfile.Compile(cfg.StepFilePattern)
	require.NoError(t, err)

	return pattern
}
