// This file reaches the unexported classifyDashArg directly to pin its
// per-branch contract at the unit level, and to fuzz it for the one
// property every call site depends on: it never panics, whatever the
// input. The three call sites' own observable behavior (stderr, exit
// code) stays covered by the black-box cli_test files.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_classifyDashArg_classifies_every_token_shape pins classifyDashArg's
// branches directly, independent of any call site's own wording around the
// (kind, msg) pair it returns.
func Test_classifyDashArg_classifies_every_token_shape(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		wantKind argKind
		wantMsg  string
	}{
		{name: "empty string is not a flag", arg: "", wantKind: argNotFlag},
		{name: "a bare dash is not a flag", arg: "-", wantKind: argNotFlag},
		{name: "a bare double dash is the terminator, not a flag", arg: "--", wantKind: argNotFlag},
		{name: "a plain word is not a flag", arg: "feature", wantKind: argNotFlag},
		{name: "--help is the help flag", arg: "--help", wantKind: argHelpFlag},
		{name: "-h is the help flag", arg: "-h", wantKind: argHelpFlag},
		{name: "-hh is an all-h cluster", arg: "-hh", wantKind: argHelpFlag},
		{name: "-hhh is an all-h cluster", arg: "-hhh", wantKind: argHelpFlag},
		{name: "--help=true is the help flag given a value", arg: "--help=true", wantKind: argHelpFlagWithValue, wantMsg: "--help"},
		{name: "-h=x is the help flag given a value", arg: "-h=x", wantKind: argHelpFlagWithValue, wantMsg: "-h"},
		{name: "-hh=x is an all-h cluster given a value", arg: "-hh=x", wantKind: argHelpFlagWithValue, wantMsg: "-hh"},
		{name: "-hh= is an all-h cluster given an explicit empty value", arg: "-hh=", wantKind: argHelpFlagWithValue, wantMsg: "-hh"},
		{name: "-hx is an unknown shorthand after a leading defined -h", arg: "-hx", wantKind: argUnknownFlag, wantMsg: "unknown shorthand flag: 'x' in -x"},
		{name: "-hhx is an unknown shorthand after two leading defined -h", arg: "-hhx", wantKind: argUnknownFlag, wantMsg: "unknown shorthand flag: 'x' in -x"},
		{name: "-x is an unknown shorthand", arg: "-x", wantKind: argUnknownFlag, wantMsg: "unknown shorthand flag: 'x' in -x"},
		{name: "-xy is an unknown shorthand cluster", arg: "-xy", wantKind: argUnknownFlag, wantMsg: "unknown shorthand flag: 'x' in -xy"},
		{name: "--bogus is an unknown long flag", arg: "--bogus", wantKind: argUnknownFlag, wantMsg: "unknown flag: --bogus"},
		{name: "--=x is bad flag syntax", arg: "--=x", wantKind: argUnknownFlag, wantMsg: "bad flag syntax: --=x"},
		{name: "---x is bad flag syntax", arg: "---x", wantKind: argUnknownFlag, wantMsg: "bad flag syntax: ---x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, msg := classifyDashArg(tt.arg)

			assert.Equal(t, tt.wantKind, kind)
			assert.Equal(t, tt.wantMsg, msg)
		})
	}
}

// FuzzClassifyDashArg asserts classifyDashArg's one precondition-free
// guarantee: it never panics on any input, and every argUnknownFlag
// message it builds is already one line — the property every call site
// relies on to embed msg in its own single-line usage error without
// flattening it again.
func FuzzClassifyDashArg(f *testing.F) {
	seeds := []string{
		"", "-", "--", "-h", "--help", "-hh", "-hhh",
		"-h=x", "-hh=x", "-hh=", "--help=x", "--help=",
		"-hx", "-hhx", "-x", "-xy", "-=", "-=x",
		"--bogus", "--=x", "---x", "--", "--fo\no", "-z\nq",
		"plain", "feature", "-h\nx",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, arg string) {
		kind, msg := classifyDashArg(arg)

		if kind == argUnknownFlag {
			assert.NotContains(t, msg, "\n")
		}
	})
}
