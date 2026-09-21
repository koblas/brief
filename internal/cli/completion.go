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
// exactly one shell name, generated against cmd.Root() so the script names
// the whole "brief" program rather than the completion leaf itself.
func runCompletion(cmd *cobra.Command, rest []string, stdout, stderr io.Writer) error {
	switch {
	case len(rest) == 0:
		return usageError(stderr, "brief completion: no shell given; run '"+completionInvocation+"'")
	case len(rest) > 1:
		return usageError(stderr, "brief completion: too many arguments; run '"+completionInvocation+"'")
	}

	shell := rest[0]
	for _, s := range completionShells {
		if s.name != shell {
			continue
		}

		return s.gen(cmd.Root(), stdout)
	}

	return usageError(stderr, fmt.Sprintf("brief completion: unknown shell %q; expected one of: %s", shell, completionShellList()))
}
