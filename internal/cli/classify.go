package cli

import (
	"fmt"
	"strings"
)

// argKind is classifyDashArg's result: which of six shapes a first
// argument takes, for the commands that disable cobra's own flag parsing.
type argKind int

const (
	// argNotFlag is a plain word, "", "-", or "--" (pflag's end-of-flags terminator).
	argNotFlag argKind = iota

	// argHelpFlag is "-h"/"--help" with no attached value: bare, or a shorthand cluster made entirely of 'h' ("-hh", "-hhh", ...).
	argHelpFlag

	// argHelpFlagWithValue is the help flag with an explicit value ("--help=<v>", "-hh=<v>"); msg is the flag as typed, value stripped.
	argHelpFlagWithValue

	// argUnknownFlag is any other dash-prefixed token; msg is the pflag-shaped, single-line message to report verbatim.
	argUnknownFlag

	// argVersionFlag is exactly "--version" with no attached value; msg is unknownLongFlagMessage("--version"). Root alone has its own "--version" contract and uses neither msg.
	argVersionFlag

	// argVersionFlagWithValue is "--version=<v>", including an empty value; msg is unknownLongFlagMessage(arg), identical to argVersionFlag's own msg.
	argVersionFlagWithValue
)

// classifyDashArg classifies arg into one of the argKind values above. Any
// input, including "", "-" and dash-prefixed garbage, resolves without
// panicking.
func classifyDashArg(arg string) (argKind, string) {
	if len(arg) < 2 || arg[0] != '-' || arg == "--" {
		return argNotFlag, ""
	}

	if arg == "--help" {
		return argHelpFlag, ""
	}

	if strings.HasPrefix(arg, "--help=") {
		return argHelpFlagWithValue, "--help"
	}

	if arg == "--version" {
		return argVersionFlag, unknownLongFlagMessage(arg)
	}

	if strings.HasPrefix(arg, "--version=") {
		return argVersionFlagWithValue, unknownLongFlagMessage(arg)
	}

	if strings.HasPrefix(arg, "--") {
		return argUnknownFlag, unknownLongFlagMessage(arg)
	}

	cluster := arg[1:]

	name, _, hasEq := strings.Cut(cluster, "=")
	if isAllH(name) {
		if hasEq {
			return argHelpFlagWithValue, "-" + name
		}

		return argHelpFlag, ""
	}

	return argUnknownFlag, unknownShortFlagMessage(cluster)
}

// isAllH reports whether s is one or more 'h' characters and nothing else.
func isAllH(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		if r != 'h' {
			return false
		}
	}

	return true
}

// unknownLongFlagMessage renders arg (a "--"-prefixed token other than
// "--help"/"--help=<v>") the way pflag's own error would: bad flag syntax
// when the name is empty or starts with "-"/"=", else "unknown flag:
// --<name>" with any "=<value>" trimmed.
func unknownLongFlagMessage(arg string) string {
	name := arg[2:]
	if name == "" || name[0] == '-' || name[0] == '=' {
		return flattenOneLine("bad flag syntax: " + arg)
	}

	name, _, _ = strings.Cut(name, "=")

	return flattenOneLine("unknown flag: --" + name)
}

// unknownShortFlagMessage renders cluster (a single-dash token's
// characters after "-") the way pflag's shorthand-cluster parser would:
// skip every leading 'h', then quote the first unresolved character and
// the residual cluster. classifyDashArg only calls this once cluster's
// portion before any "=" is known not to be entirely 'h', so residual is
// never empty.
func unknownShortFlagMessage(cluster string) string {
	i := 0
	for i < len(cluster) && cluster[i] == 'h' {
		i++
	}

	residual := cluster[i:]

	// Mirrors pflag's own NotExistError, which quotes a raw byte rather
	// than residual's real UTF-8 rune: any multi-byte lead byte collapses
	// to the constant 'Ã' (U+00C3) instead of the character actually typed.
	return flattenOneLine(fmt.Sprintf("unknown shorthand flag: %q in -%s", rune(string(residual[0])[0]), residual))
}
