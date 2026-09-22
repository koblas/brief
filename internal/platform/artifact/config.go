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
}

// configFileKeyPrefix matches a YAML line's own key, ignoring its leading
// indentation, so each line can be looked up in configFieldDocs.
const configFileKeyChars = "abcdefghijklmnopqrstuvwxyz0123456789-"

// ConfigFile renders the ".brief.yaml" brief's init writes: config.Default()
// marshalled with a 2-space indent, every line commented, and a "# " doc
// line inserted directly above each one — every key present but none of
// them live (R2). Uncommenting (stripping one leading "#" from every line
// not beginning "# ") reproduces exactly the marshalled bytes, so the
// result decodes to config.Default() with no violations; the render itself,
// left exactly as written, decodes as an empty file (every line is a
// comment), which also resolves to config.Default() — see
// Test_config_file_resolves_to_the_shipped_defaults. The bytes are
// deterministic: no version, no date, no map (Config carries none).
func ConfigFile() []byte {
	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(config.Default()); err != nil {
		// config.Default() is a fixed, hand-written value with no cyclic or
		// unencodable field; a failure here would be a bug in this package,
		// not a runtime condition a caller can act on.
		panic(fmt.Sprintf("artifact: marshal default config: %v", err))
	}

	_ = enc.Close()

	var out strings.Builder

	for line := range strings.SplitSeq(strings.TrimRight(buf.String(), "\n"), "\n") {
		if doc, ok := configFieldDocs[configFileLineKey(line)]; ok {
			out.WriteString("# ")
			out.WriteString(doc)
			out.WriteByte('\n')
		}

		out.WriteByte('#')
		out.WriteString(line)
		out.WriteByte('\n')
	}

	return []byte(out.String())
}

// configFileLineKey extracts line's own YAML key — the run of lower-case
// letters, digits and hyphens before its first ":" — ignoring leading
// indentation. It returns "" for a line with no such key (never produced by
// config.Default()'s own encoding, but safe: configFieldDocs simply has no
// entry for "").
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
