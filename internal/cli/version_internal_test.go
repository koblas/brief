// This file reaches the unexported run directly so --version's exact
// stored-version output can be pinned against a fake build-info reader: a
// go test binary's own debug.ReadBuildInfo never reports the real module
// version, so a black-box test through cli.Run cannot assert one. Every
// other cli behavior stays covered by the black-box cli_test files.

package cli

import (
	"bytes"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_version_flag_prints_the_stored_module_version_verbatim pins R1: sole
// "--version" writes "brief <Main.Version>\n" to stdout, nothing to stderr,
// exit 0, for every shape Main.Version takes once a build actually stamps
// one — a released tag, a pseudo-version, and a pseudo-version marked dirty.
// The three rows are one behavior (verbatim pass-through) exercised against
// the specification's own Examples table, not three independent rules —
// each proves the same pass-through for a different literal, not a
// different branch.
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
