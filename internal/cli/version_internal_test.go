// This file reaches the unexported run directly so --version's exact stdout
// text can be pinned against a fake build-info reader: a go test binary's
// own debug.ReadBuildInfo never reports the real module version, so a
// black-box test through cli.Run cannot assert an exact value for either
// the verbatim pass-through or the (devel) fallback. Every other cli
// behavior stays covered by the black-box cli_test files.

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

// Test_version_flag_reports_devel_when_the_build_stored_no_version pins R2:
// sole "--version" falls back to "brief (devel)\n" on stdout, nothing on
// stderr, exit 0, whenever the build stored no usable version — Main.Version
// of "(devel)" or "", or readBuildInfo reporting ok=false, regardless of
// what info it returns alongside that.
//
// Mutation-verified per row, restored byte-identical after each: dropping
// the ok check (keeping the empty check) reddens "ok is false even though
// build info is present" (prints the stored version) and "no build info at
// all" (nil-pointer panic); dropping the empty-version check (keeping the
// ok check) reddens "the build stored an empty version" ("brief \n");
// changing the (devel) literal inside the fallback reddens both of the
// above plus "no build info at all", while "the build stored (devel)" stays
// green — pass-through and fallback emit identical bytes for that one
// input, so that row is a behavior pin for R2's own wording, not guard
// evidence, and cannot discriminate on its own.
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
