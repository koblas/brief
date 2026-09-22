package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// helpDocument is a help document's own success shape: the common header,
// literally "help" (newHelpDocument), plus commands — either the full
// index (root asked for its own help) or exactly the one entry describing
// the command that was actually asked about, never both.
type helpDocument struct {
	jsonHeader

	Commands []helpCommandJSON `json:"commands"`
}

// helpCommandJSON is one command's own entry in a help document:
// commandName's path, UseLine's generated usage, Short/Long verbatim, and
// its own local flags — the same fields and the same source cobra's own
// text help renders, so the two can never disagree.
type helpCommandJSON struct {
	Name        string         `json:"name"`
	Usage       string         `json:"usage"`
	Summary     string         `json:"summary"`
	Description string         `json:"description"`
	Flags       []helpFlagJSON `json:"flags"`
}

// helpFlagJSON is one row of a helpCommandJSON's own flags: the pflag
// name, its Value.Type() ("bool", "string"), and pflag.UnquoteUsage's own
// usage text — its backquoted varname marker stripped, embedded newlines
// kept.
type helpFlagJSON struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Usage string `json:"usage"`
}

// newHelpDocument builds a help document's header: the literal "help",
// never the command actually being described — scripts dispatch on
// "command" for payload shape, and "start" already names start --json's
// own brief document.
func newHelpDocument() helpDocument {
	return helpDocument{
		jsonHeader: newJSONHeader("help", 0),
		Commands:   []helpCommandJSON{},
	}
}

// listedForHelp reports whether cmd belongs in root help and as a "brief
// help" topic: available, or hidden but carrying listedInHelpAnnotation —
// the same predicate helpTemplate's own cmdList row loop and the help
// stub's own topic-acceptance check use.
func listedForHelp(cmd *cobra.Command) bool {
	return cmd.IsAvailableCommand() || cmd.Annotations[listedInHelpAnnotation] != ""
}

// helpFlags builds cmd's own flags[], from cmd.LocalFlags().VisitAll — the
// same set helpTemplate's own Flags table renders — skipping any hidden
// flag. It is never nil, even for a command with no flags at all.
func helpFlags(cmd *cobra.Command) []helpFlagJSON {
	flags := []helpFlagJSON{}

	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}

		_, usage := pflag.UnquoteUsage(f)
		flags = append(flags, helpFlagJSON{
			Name:  f.Name,
			Type:  f.Value.Type(),
			Usage: usage,
		})
	})

	return flags
}

// helpEntry builds cmd's own helpCommandJSON: name from commandName,
// usage from cmd.UseLine(), summary/description from cmd's own
// Short/Long, and flags from helpFlags. cmd.InitDefaultHelpFlag() runs
// first, since cobra only calls it on the command Execute() actually
// resolves — without it, an index entry built for a command other than
// the one dispatch resolved would lack the -h/--help row its own
// "--help" render carries.
func helpEntry(cmd *cobra.Command) helpCommandJSON {
	cmd.InitDefaultHelpFlag()

	return helpCommandJSON{
		Name:        commandName(cmd),
		Usage:       cmd.UseLine(),
		Summary:     cmd.Short,
		Description: cmd.Long,
		Flags:       helpFlags(cmd),
	}
}

// helpIndex walks root's own children depth-first, in registration order,
// building one helpCommandJSON per command listedForHelp — "new" itself,
// ahead of its own children, the same order root help's own cmdList rows
// derive from with "new" inserted. Root and the hidden "help" stub are
// never entries: root is never passed to the walk, and the stub fails
// listedForHelp (hidden, unlisted).
func helpIndex(root *cobra.Command) []helpCommandJSON {
	entries := []helpCommandJSON{}

	var walk func([]*cobra.Command)
	walk = func(cmds []*cobra.Command) {
		for _, cmd := range cmds {
			if !listedForHelp(cmd) {
				continue
			}

			entries = append(entries, helpEntry(cmd))
			walk(cmd.Commands())
		}
	}
	walk(root.Commands())

	return entries
}
