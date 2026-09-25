// White-box: reaches the unexported classifyDashArg to pin its per-branch
// contract directly and fuzz it for the never-panics guarantee.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Pins classifyDashArg's branches directly; keeps only the shapes not
// already reachable through the black-box cli_test tables.
func Test_classifyDashArg_classifies_every_token_shape(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		wantKind argKind
		wantMsg  string
	}{
		{name: "empty string is not a flag", arg: "", wantKind: argNotFlag},
		{name: "a plain word is not a flag", arg: "feature", wantKind: argNotFlag},
		{name: "-hh is an all-h cluster", arg: "-hh", wantKind: argHelpFlag},
		{name: "-xy is an unknown shorthand cluster", arg: "-xy", wantKind: argUnknownFlag, wantMsg: "unknown shorthand flag: 'x' in -xy"},
		{name: "--version is the version flag", arg: "--version", wantKind: argVersionFlag, wantMsg: "unknown flag: --version"},
		{name: "--version=x is the version flag given a value", arg: "--version=x", wantKind: argVersionFlagWithValue, wantMsg: "unknown flag: --version"},
		{name: "--versionx is not the version flag, just an unknown flag with a similar name", arg: "--versionx", wantKind: argUnknownFlag, wantMsg: "unknown flag: --versionx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, msg := classifyDashArg(tt.arg)

			assert.Equal(t, tt.wantKind, kind)
			assert.Equal(t, tt.wantMsg, msg)
		})
	}
}

// FuzzClassifyDashArg asserts classifyDashArg never panics and msg is
// always one line, over arbitrary input.
func FuzzClassifyDashArg(f *testing.F) {
	seeds := []string{
		"", "-", "--", "-h", "--help", "-hh", "-hhh",
		"-h=x", "-hh=x", "-hh=", "--help=x", "--help=",
		"-hx", "-hhx", "-x", "-xy", "-=", "-=x",
		"--bogus", "--=x", "---x", "--", "--fo\no", "-z\nq",
		"plain", "feature", "-h\nx", "--version", "--version=x",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, arg string) {
		_, msg := classifyDashArg(arg)

		assert.NotContains(t, msg, "\n")
	})
}
