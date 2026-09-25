package artifact

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/koblas/brief/internal/platform/config"
	"gopkg.in/yaml.v3"
)

// configFieldDocs pairs every YAML key ConfigFile emits — top-level and
// nested — with the one-line doc prose ConfigFile writes directly above
// its commented value line. Keys are unique across the whole schema, so a
// single flat table serves both nesting levels.
var configFieldDocs = map[string]string{
	"feature-directory":           "Where feature directories are created and read, relative to this file.",
	"step-file-pattern":           "How a feature's step files are named; exactly one %d or %0Nd verb.",
	"specification-file":          "The specification file name inside every feature directory.",
	"state-file":                  "The state file name inside every feature directory.",
	"progress-heading":            "The heading marking the specification's progress list.",
	"checklist-heading":           "The heading marking a step's checklist section.",
	"handoff-file-suffix":         "The suffix a step's handoff file is named with, beside the step file.",
	"acceptance-heading":          "The heading marking a step's acceptance-criteria section.",
	"state-headings":              "The four section headings every feature's state file must carry.",
	"binding-decisions":           "The state file's binding-decisions section heading.",
	"left-unbuilt":                "The state file's left-unbuilt section heading.",
	"traps":                       "The state file's traps section heading.",
	"open-debts":                  "The state file's open-debts section heading.",
	"handoff-cap-lines":           "The maximum lines a step's handoff body may carry.",
	"state-cap-lines":             "The maximum lines a feature's state file may carry.",
	"default-output-budget-bytes": "The default byte budget for a command's own output.",
	"optional-conventions":        "Optional conventions this repository opts into.",
	"roles":                       "Agent bindings for brief's roles.",
	"planner":                     "The agent bound to the planner role.",
	"implementer":                 "The agent bound to the implementer role.",
	"reviewer":                    "The agent bound to the reviewer role.",
}

// configFileKeyChars are the characters a YAML key may contain, used to
// extract a line's key ignoring its leading indentation.
const configFileKeyChars = "abcdefghijklmnopqrstuvwxyz0123456789-"

// configFileLiveKeys names the YAML keys ConfigFileWithRoles leaves live
// (uncommented) rather than commented: "roles:" and its three children.
// Every other key stays commented in every render this package produces.
var configFileLiveKeys = map[string]bool{
	"roles":       true,
	"planner":     true,
	"implementer": true,
	"reviewer":    true,
}

// ConfigFile renders the ".brief.yaml" brief's init writes: config.Default()
// marshalled with a 2-space indent, every line commented, and a "# " doc
// line inserted directly above each one. Uncommenting reproduces exactly
// the marshalled bytes, so the result decodes to config.Default(); left as
// written, every line is a comment, which also resolves to
// config.Default(). The bytes are deterministic: no version, no date.
func ConfigFile() []byte {
	return renderConfigFile(config.Default(), false)
}

// ConfigFileWithRoles renders the ".brief.yaml" "init --with-agents"
// writes when it creates a fresh config in the same run: the same bytes as
// ConfigFile, except "roles:" and its three children are left live rather
// than commented, bound to AgentBindings(). Bindings are written only into
// a config init creates in this same run; an existing config is never
// rewritten this way.
func ConfigFileWithRoles() []byte {
	cfg := config.Default()
	cfg.Roles = AgentBindings()

	return renderConfigFile(cfg, true)
}

// AgentBindings names the plugin agent bound to each of brief's three
// roles when "init --with-agents" installs them: "brief:planner",
// "brief:implementer" and "brief:reviewer".
func AgentBindings() config.RoleBindings {
	return config.RoleBindings{
		Planner:     "brief:planner",
		Implementer: "brief:implementer",
		Reviewer:    "brief:reviewer",
	}
}

// renderConfigFile marshals cfg with a 2-space indent, comments every
// line, and inserts a "# " doc line above each one carrying a
// configFieldDocs entry. liveRoles leaves configFileLiveKeys' lines
// uncommented instead.
func renderConfigFile(cfg config.Config, liveRoles bool) []byte {
	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(cfg); err != nil {
		// cfg is always one of this package's own fixed, hand-written values
		// with no cyclic or unencodable field; a failure here would be a bug
		// in this package, not a runtime condition a caller can act on.
		panic(fmt.Sprintf("artifact: marshal config: %v", err))
	}

	_ = enc.Close()

	var out strings.Builder

	for line := range strings.SplitSeq(strings.TrimRight(buf.String(), "\n"), "\n") {
		key := configFileLineKey(line)

		if doc, ok := configFieldDocs[key]; ok {
			out.WriteString("# ")
			out.WriteString(doc)
			out.WriteByte('\n')
		}

		if liveRoles && configFileLiveKeys[key] {
			out.WriteString(line)
			out.WriteByte('\n')

			continue
		}

		out.WriteByte('#')
		out.WriteString(line)
		out.WriteByte('\n')
	}

	return []byte(out.String())
}

// configFileLineKey extracts line's YAML key — the run of lower-case
// letters, digits and hyphens before its first ":" — ignoring leading
// indentation. It returns "" for a line with no such key.
func configFileLineKey(line string) string {
	trimmed := strings.TrimLeft(line, " ")

	idx := strings.IndexByte(trimmed, ':')
	if idx <= 0 {
		return ""
	}

	key := trimmed[:idx]
	for _, r := range key {
		if !strings.ContainsRune(configFileKeyChars, r) {
			return ""
		}
	}

	return key
}
