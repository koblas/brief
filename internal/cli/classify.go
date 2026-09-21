package cli

import (
	"fmt"
	"strings"
)

// argKind is classifyDashArg's result: which of the five shapes root,
// "new" and the help stub need to tell apart in their first argument,
// since all three disable cobra's own flag parsing.
type argKind int

const (
	// argNotFlag is a plain word, "", "-", or "--" — pflag's own
	// end-of-flags terminator, never a flag itself. The caller falls
	// through to its own unknown-command/unknown-type wording.
	argNotFlag argKind = iota

	// argHelpFlag is "-h"/"--help" spelled with no attached value: bare,
	// or a shorthand cluster made entirely of 'h' characters ("-hh",
	// "-hhh", ...) — pflag's own shorthand-cluster parser consumes a run
	// of defined, no-value shorthand characters without error, and "h"
	// is the only such character this tree registers.
	argHelpFlag

	// argHelpFlagWithValue is the help flag given an explicit value:
	// "--help=<v>", or a single-dash all-'h' cluster followed by "="
	// ("-h=<v>", "-hh=<v>", "-hh="). msg is that flag exactly as typed,
	// value stripped, for the caller's "takes no value" message.
	argHelpFlagWithValue

	// argUnknownFlag is any other dash-prefixed token. msg is the
	// pflag-shaped, single-line message the caller reports verbatim.
	argUnknownFlag

	// argVersionFlag is exactly "--version", spelled with no attached
	// value. msg is unknownLongFlagMessage("--version"), the same wording
	// argUnknownFlag would report for it — so a caller that has no
	// "--version" contract of its own (runNew, the help stub) can fold this
	// case into its argUnknownFlag arm byte-for-byte. "--version=<v>" does
	// not match here: it falls through to argUnknownFlag, since a value on
	// "--version" is a different rule (root's "takes no value" case) than
	// this exact-match kind carries.
	argVersionFlag
)

// classifyDashArg classifies arg the way root, "new" and the help stub
// each need to. It has no precondition on arg: every input, including "",
// "-", and any dash-prefixed garbage, resolves to one of the five argKind
// values above without panicking.
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

// isAllH reports whether s is one or more 'h' characters and nothing
// else — the only shorthand this tree registers with no value, so a
// cluster made entirely of 'h' parses exactly like a single "-h" does.
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

// unknownLongFlagMessage renders arg — a "--"-prefixed token that is
// neither "--help" nor "--help=<v>" — the way pflag's own error would for
// the same token on a leaf command: bad flag syntax when its name is
// empty or starts with "-"/"=", else "unknown flag: --<name>" with any
// attached "=<value>" trimmed. The result is already flattened to one
// line.
func unknownLongFlagMessage(arg string) string {
	name := arg[2:]
	if name == "" || name[0] == '-' || name[0] == '=' {
		return flattenOneLine("bad flag syntax: " + arg)
	}

	name, _, _ = strings.Cut(name, "=")

	return flattenOneLine("unknown flag: --" + name)
}

// unknownShortFlagMessage renders cluster — a single-dash token's
// characters after the leading "-" — the way pflag's own shorthand-cluster
// parser would: it consumes every leading 'h', the only defined no-value
// shorthand this tree registers, before quoting the first character it
// cannot resolve and the residual cluster from there. classifyDashArg
// only calls this once it has ruled out cluster's portion before any "="
// being entirely 'h', so the skip loop always stops before running off
// the end of cluster and residual is never empty. The result is already
// flattened to one line.
func unknownShortFlagMessage(cluster string) string {
	i := 0
	for i < len(cluster) && cluster[i] == 'h' {
		i++
	}

	residual := cluster[i:]

	// pflag's own NotExistError quotes a raw byte, not residual's real
	// UTF-8 rune: it widens the byte to a rune, re-encodes that as UTF-8,
	// then widens that encoding's own first byte to a rune again. For any
	// ASCII byte the round trip is the identity; for any multi-byte UTF-8
	// lead byte it collapses to a single constant, 'Ã' (U+00C3) — pflag's
	// own quirk, mirrored here byte-for-byte rather than decoded to the
	// character actually typed.
	return flattenOneLine(fmt.Sprintf("unknown shorthand flag: %q in -%s", rune(string(residual[0])[0]), residual))
}
