// Shared rwfs.Mem test plumbing for command-level tests: newMemSetupSeam
// for the setup seam, memTree for a feature tree via withRootFS.

package cli

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memRoot is the virtual working directory every Mem test resolves
// against — fabricated, never a real disk path.
const memRoot = "/repo"

// fsAbs joins slash-separated segments under "/", the absolute path an
// rwfs.Mem fixture is keyed against.
func fsAbs(elem ...string) string {
	return filepath.FromSlash("/" + filepath.ToSlash(filepath.Join(elem...)))
}

// memKey turns an fsAbs-fabricated path into the name a Mem's own
// fstest.MapFS is keyed against.
func memKey(abs string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), "/")
	if trimmed == "" {
		return "."
	}

	return trimmed
}

// newVirtualMem returns an rwfs.Mem seeded with root as an explicit
// directory entry, so a read confirming wd exists finds it.
func newVirtualMem(root string) *rwfs.Mem {
	return rwfs.NewMem(fstest.MapFS{memKey(root): &fstest.MapFile{Mode: fs.ModeDir | 0o755}})
}

// newMemSetupSeam returns the runSeam a command-level test runs mem's
// fixture through instead of real disk; extra opts override the defaults.
func newMemSetupSeam(mem *rwfs.Mem, extra ...setup.Option) runSeam {
	opts := append([]setup.Option{
		setup.WithFSRoot(mem),
		setup.WithResolveRoot(func(root string) (string, error) { return root, nil }),
		setup.WithHomeDir(func() (string, error) { return "", nil }),
		setup.WithWritableCheck(func([]string) error { return nil }),
	}, extra...)

	return withSetupOpts(opts...)
}

// memTree accumulates directory and file entries for an rwfs.Mem fixture,
// keyed by fsAbs-fabricated path. dir and file return t so calls chain.
type memTree struct {
	entries fstest.MapFS
}

// newMemTree returns a memTree seeded with every path in dirs as an
// explicit directory entry; root must be included so a "wd exists" check finds it.
func newMemTree(dirs ...string) *memTree {
	t := &memTree{entries: fstest.MapFS{}}

	for _, d := range dirs {
		t.dir(d)
	}

	return t
}

// dir adds path as an explicit directory entry.
func (t *memTree) dir(path string) *memTree {
	t.entries[memKey(path)] = &fstest.MapFile{Mode: fs.ModeDir | 0o755}

	return t
}

// file adds path as a regular file entry holding body.
func (t *memTree) file(path, body string) *memTree {
	t.entries[memKey(path)] = &fstest.MapFile{Data: []byte(body), Mode: 0o600}

	return t
}

// mem builds the rwfs.Mem every entry added so far backs.
func (t *memTree) mem() *rwfs.Mem {
	return rwfs.NewMem(t.entries)
}

// memJSONString marshals s the way assert.Equal compares it, for building
// a golden literal around a dynamically computed value.
func memJSONString(t *testing.T, s string) string {
	t.Helper()

	b, err := json.Marshal(s)
	require.NoError(t, err)

	return string(b)
}

// memJSONKeys returns doc's keys, for an exact-key-set assertion via assert.ElementsMatch.
func memJSONKeys(doc map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}

	return keys
}

// memDecodedError is the JSON shape of a --json error document's "error" member.
type memDecodedError struct {
	Kind         string  `json:"kind"`
	Message      string  `json:"message"`
	Path         *string `json:"path"`
	Line         *int    `json:"line"`
	Problem      *string `json:"problem"`
	Fix          string  `json:"fix"`
	FilesChanged *bool   `json:"files_changed"`
}

// memDecodeErrorDocument asserts stdout holds exactly one --json error
// document for wantCommand and returns its error object.
func memDecodeErrorDocument(t *testing.T, stdout []byte, wantCommand string) memDecodedError {
	t.Helper()

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout, &doc))
	assert.ElementsMatch(t, []string{"schema", "command", "ok", "exit_code", "error"}, memJSONKeys(doc))

	var schema int
	require.NoError(t, json.Unmarshal(doc["schema"], &schema))
	assert.Equal(t, 1, schema)

	var command string
	require.NoError(t, json.Unmarshal(doc["command"], &command))
	assert.Equal(t, wantCommand, command)

	var ok bool
	require.NoError(t, json.Unmarshal(doc["ok"], &ok))
	assert.False(t, ok)

	var exitCode int
	require.NoError(t, json.Unmarshal(doc["exit_code"], &exitCode))
	assert.Equal(t, 1, exitCode)

	var errObj map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc["error"], &errObj))
	assert.ElementsMatch(t, []string{"kind", "message", "path", "line", "problem", "fix", "files_changed"}, memJSONKeys(errObj))

	var decoded memDecodedError
	require.NoError(t, json.Unmarshal(doc["error"], &decoded))

	return decoded
}

// memDecodeUsageErrorDocument asserts stdout holds exactly one --json
// usage-error document for wantCommand and wantFilesChanged, and returns
// its error.message.
func memDecodeUsageErrorDocument(t *testing.T, stdout []byte, wantCommand string, wantFilesChanged *bool) string {
	t.Helper()

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stdout, &doc))
	assert.ElementsMatch(t, []string{"schema", "command", "ok", "exit_code", "error"}, memJSONKeys(doc))

	var schema int
	require.NoError(t, json.Unmarshal(doc["schema"], &schema))
	assert.Equal(t, 1, schema)

	var command string
	require.NoError(t, json.Unmarshal(doc["command"], &command))
	assert.Equal(t, wantCommand, command)

	var ok bool
	require.NoError(t, json.Unmarshal(doc["ok"], &ok))
	assert.False(t, ok)

	var exitCode int
	require.NoError(t, json.Unmarshal(doc["exit_code"], &exitCode))
	assert.Equal(t, 2, exitCode)

	var errObj map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc["error"], &errObj))
	assert.ElementsMatch(t, []string{"kind", "message", "path", "line", "problem", "fix", "files_changed"}, memJSONKeys(errObj))

	var kind string
	require.NoError(t, json.Unmarshal(errObj["kind"], &kind))
	assert.Equal(t, "usage", kind)

	assert.JSONEq(t, "null", string(errObj["path"]))
	assert.JSONEq(t, "null", string(errObj["line"]))
	assert.JSONEq(t, "null", string(errObj["problem"]))

	wantFilesChangedJSON := "null"
	if wantFilesChanged != nil {
		want, err := json.Marshal(*wantFilesChanged)
		require.NoError(t, err)
		wantFilesChangedJSON = string(want)
	}
	assert.JSONEq(t, wantFilesChangedJSON, string(errObj["files_changed"]))

	var message string
	require.NoError(t, json.Unmarshal(errObj["message"], &message))

	return message
}
