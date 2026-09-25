package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// completionLong is "brief completion"'s help prose.
const completionLong = `Prints a shell completion script for the named shell to stdout, so it can
be sourced directly or written to your shell's completion directory.`

// completionInvocation is the usage line every completion error names,
// including a flag error, via leafCommand's invocation argument.
const completionInvocation = "brief completion <bash|zsh|fish|powershell>"

// completionJSONUnsupportedMessage refuses --json: completion's stdout is a shell script's own bytes, not a document.
const completionJSONUnsupportedMessage = "brief completion: completion prints a shell script; --json does not apply"

// completionShell names one supported shell and the cobra generator that
// writes its script to w for root's own tree.
type completionShell struct {
	name string
	gen  func(root *cobra.Command, w io.Writer) error
}

// completionShells is the ordered bash, zsh, fish, powershell table both
// runCompletion's dispatch and its unknown-shell "expected one of:" list
// derive from.
var completionShells = []completionShell{
	{name: "bash", gen: func(root *cobra.Command, w io.Writer) error { return root.GenBashCompletionV2(w, true) }},
	{name: "zsh", gen: func(root *cobra.Command, w io.Writer) error { return root.GenZshCompletion(w) }},
	{name: "fish", gen: func(root *cobra.Command, w io.Writer) error { return root.GenFishCompletion(w, true) }},
	{name: "powershell", gen: func(root *cobra.Command, w io.Writer) error { return root.GenPowerShellCompletionWithDesc(w) }},
}

// completionShellList joins completionShells' names in order, for the
// unknown-shell usage error's "expected one of:" clause.
func completionShellList() string {
	names := make([]string, len(completionShells))
	for i, s := range completionShells {
		names[i] = s.name
	}

	return strings.Join(names, ", ")
}

// runCompletion implements "brief completion <bash|zsh|fish|powershell>";
// rest is its positional arguments, flags already parsed away. It requires
// exactly one known shell name, generated against cmd.Root() so the script
// names the whole "brief" program rather than the completion leaf itself.
func runCompletion(cmd *cobra.Command, rest []string, out reporter) error {
	switch {
	case len(rest) == 0:
		return out.usageError("brief completion: no shell given; run '" + completionInvocation + "'")
	case len(rest) > 1:
		return out.usageError("brief completion: too many arguments; run '" + completionInvocation + "'")
	}

	shell := rest[0]
	for _, s := range completionShells {
		if s.name != shell {
			continue
		}

		// Checked only once the shell name is known valid, so an
		// unresolved shell keeps its own usage error above and below.
		if out.json {
			return out.usageError(completionJSONUnsupportedMessage)
		}

		return s.gen(cmd.Root(), out.stdout)
	}

	return out.usageError(fmt.Sprintf("brief completion: unknown shell %q; expected one of: %s", shell, completionShellList()))
}
