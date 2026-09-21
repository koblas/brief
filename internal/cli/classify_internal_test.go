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
// (kind, msg) pair it returns. Every other shape this function resolves —
// "-", "--help", "-h", "-hhh", the "=value" variants, "-hx"/"-hhx", "-x",
// "--bogus", "--=x", "---x" — is reachable and already pinned through the
// black-box cli_test tables (flag_error_test.go's per-site tables and its
// cross-site table), so this table keeps only the shapes that table can't
// reach on its own: "" and "-xy" are never exercised as root/"new"/help's
// args[0] by any black-box test today; "feature" and one all-h cluster
// stand in as non-decorative anchors for argNotFlag and argHelpFlag; and
// "--version" pins argVersionFlag's own (kind, msg) pair directly, since no
// black-box test asserts that pairing by name — root's own sole-argument
// "--version" behavior is covered separately, through the run seam. This
// table still documents every argKind without re-covering ground the
// black-box tables already own.
//
// Mutation-verified: dropping classifyDashArg's "arg[0] != '-'" guard reds
// the "feature" row (a plain word starts falling into the flag branches
// instead of argNotFlag).
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, msg := classifyDashArg(tt.arg)

			assert.Equal(t, tt.wantKind, kind)
			assert.Equal(t, tt.wantMsg, msg)
		})
	}
}

// FuzzClassifyDashArg asserts classifyDashArg's precondition-free
// guarantees over arbitrary input: it never panics, and msg is always
// already one line — true for every argKind, not just argUnknownFlag,
// since argNotFlag and argHelpFlag always return "" and
// argHelpFlagWithValue's msg is built from a name isAllH already proved
// contains only 'h' characters — the property every call site relies on
// to embed msg in its own single-line usage error without flattening it
// again.
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
