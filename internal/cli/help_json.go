package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// helpDocument is a help document's success shape: the common header
// (command always "help"), plus commands — either the full index or
// exactly the one entry for the command asked about, never both.
type helpDocument struct {
	jsonHeader

	Commands []helpCommandJSON `json:"commands"`
}

// helpCommandJSON is one command's entry in a help document: its name,
// usage, summary/description and local flags, drawn from the same cobra
// fields the text help renders, so the two can never disagree.
type helpCommandJSON struct {
	Name        string         `json:"name"`
	Usage       string         `json:"usage"`
	Summary     string         `json:"summary"`
	Description string         `json:"description"`
	Flags       []helpFlagJSON `json:"flags"`
}

// helpFlagJSON is one row of a helpCommandJSON's flags: its pflag name,
// Value.Type(), and pflag.UnquoteUsage's usage text with the backquoted
// varname marker stripped.
type helpFlagJSON struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Usage string `json:"usage"`
}

// newHelpDocument builds a help document's header with command always
// "help", never the command actually being described.
func newHelpDocument() helpDocument {
	return helpDocument{
		jsonHeader: newJSONHeader("help", 0),
		Commands:   []helpCommandJSON{},
	}
}

// listedForHelp reports whether cmd belongs in root help and as a "brief
// help" topic: available, or hidden but carrying listedInHelpAnnotation.
func listedForHelp(cmd *cobra.Command) bool {
	return cmd.IsAvailableCommand() || cmd.Annotations[listedInHelpAnnotation] != ""
}

// helpFlags builds cmd's flags[] from cmd.LocalFlags().VisitAll, skipping
// any hidden flag. It is never nil, even with no flags at all.
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

// helpEntry builds cmd's helpCommandJSON. It calls
// cmd.InitDefaultHelpFlag() first, since cobra only calls it on the
// command Execute() actually resolves, and an index entry built for any
// other command would otherwise lack the -h/--help row.
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

// helpIndex walks root's children depth-first, in registration order,
// building one helpCommandJSON per command listedForHelp. Root itself and
// the hidden "help" stub never appear.
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
