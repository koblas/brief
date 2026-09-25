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

// testFeaturePath is the OS directory every MapFS-backed FeatureFS in this
// package's tests is rooted at; nothing reads it from disk.
const testFeaturePath = "/repo/docs/specifications/demo"

// featureFS builds a FeatureFS over an in-memory filesystem holding files,
// keyed by name relative to the feature directory, rooted at testFeaturePath.
func featureFS(files map[string]string) assemble.FeatureFS {
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}

	return assemble.FeatureFS{FS: fsys, Path: testFeaturePath}
}

// failFS wraps an in-memory filesystem, replacing the result of reading
// failReadFile or listing failReadDir with err. Either field left empty
// never fails that operation.
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
// specFault/readStateFile, keyed by cfg's own configured names.
func conformingFeatureFiles(cfg config.Config) map[string]string {
	return map[string]string{
		cfg.SpecificationFile: checkConformingSpec(cfg),
		cfg.StateFile:         checkConformingState(cfg),
	}
}

// statusPattern compiles cfg's own step-file pattern for a test calling
// StatusFS directly against a MapFS fixture.
func statusPattern(t *testing.T, cfg config.Config) stepfile.Pattern {
	t.Helper()

	pattern, err := stepfile.Compile(cfg.StepFilePattern)
	require.NoError(t, err)

	return pattern
}
