// This file reaches the unexported run directly so --version's exact
// stdout can be pinned against a fake build-info reader: a go test
// binary's debug.ReadBuildInfo never reports the real module version.

package cli

import (
	"bytes"
	"encoding/json"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_version_flag_prints_the_stored_module_version_verbatim(t *testing.T) {
	tests := []struct {
		name   string
		stored string
	}{
		{name: "a released tag", stored: "v0.3.0"},
		{name: "a pseudo-version with no tag", stored: "v0.0.0-20260921145925-5709f33424c3"},
		{name: "a pseudo-version marked dirty", stored: "v0.0.0-20260921145925-5709f33424c3+dirty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer
			readBuildInfo := func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{Main: debug.Module{Version: tt.stored}}, true
			}

			err := run(t.Context(), wd, []string{"--version"}, nil, &stdout, &stderr, readBuildInfo)

			require.NoError(t, err)
			assert.Empty(t, stderr.String())
			assert.Equal(t, "brief "+tt.stored+"\n", stdout.String())
		})
	}
}

func Test_version_json_is_one_exact_document(t *testing.T) {
	wd := t.TempDir()
	var stdout, stderr bytes.Buffer
	readBuildInfo := func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v0.3.0"}}, true
	}

	err := run(t.Context(), wd, []string{"--version", "--json"}, nil, &stdout, &stderr, readBuildInfo)

	require.NoError(t, err)
	assert.Empty(t, stderr.String())

	want := `{"schema":1,"command":"brief","ok":true,"exit_code":0,"version":"v0.3.0"}` + "\n"

	// assert.Equal, not assert.JSONEq: this golden pins byte-exact key order.
	assert.Equal(t, want, stdout.String())
}

// The JSON "version" field shares its fallback rule with the text line's
// own test below: "(devel)" whenever the build stored no usable version.
func Test_version_json_reports_devel_when_the_build_stored_no_version(t *testing.T) {
	tests := []struct {
		name          string
		readBuildInfo func() (*debug.BuildInfo, bool)
	}{
		{
			name: "the build stored an empty version",
			readBuildInfo: func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{Main: debug.Module{Version: ""}}, true
			},
		},
		{
			name: "ok is false even though build info is present",
			readBuildInfo: func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}}, false
			},
		},
		{
			name: "no build info at all",
			readBuildInfo: func() (*debug.BuildInfo, bool) {
				return nil, false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := run(t.Context(), wd, []string{"--version", "--json"}, nil, &stdout, &stderr, tt.readBuildInfo)

			require.NoError(t, err)
			assert.Empty(t, stderr.String())

			var doc map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(stdout.Bytes(), &doc))

			var version string
			require.NoError(t, json.Unmarshal(doc["version"], &version))
			assert.Equal(t, "(devel)", version)
		})
	}
}

func Test_version_flag_reports_devel_when_the_build_stored_no_version(t *testing.T) {
	tests := []struct {
		name          string
		readBuildInfo func() (*debug.BuildInfo, bool)
	}{
		{
			name: "the build stored (devel)",
			readBuildInfo: func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true
			},
		},
		{
			name: "the build stored an empty version",
			readBuildInfo: func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{Main: debug.Module{Version: ""}}, true
			},
		},
		{
			name: "no build info at all",
			readBuildInfo: func() (*debug.BuildInfo, bool) {
				return nil, false
			},
		},
		{
			name: "ok is false even though build info is present",
			readBuildInfo: func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}}, false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := t.TempDir()
			var stdout, stderr bytes.Buffer

			err := run(t.Context(), wd, []string{"--version"}, nil, &stdout, &stderr, tt.readBuildInfo)

			require.NoError(t, err)
			assert.Empty(t, stderr.String())
			assert.Equal(t, "brief (devel)\n", stdout.String())
		})
	}
}
