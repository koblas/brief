package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// schemaVersion is the one global integer every --json document's
// "schema" field carries (R2). It is bumped only on a breaking change to
// a document's shape.
const schemaVersion = 1

// errorKindUsage is the "error.kind" value for a usage error's JSON
// document: an invocation mistake caught before any command ran, always
// exit code 2 (R3).
const errorKindUsage = "usage"

// errorKindRefusal is the "error.kind" value for a refusal's JSON
// document: a *config.InvalidConfigError, an enriched not-found
// (*unknownFeatureError), a *scaffold.RefusalError, or a
// *assemble.RefusalError — every case classifyRefusal recognizes by type,
// always exit code 1 (R3).
const errorKindRefusal = "refusal"

// errorKindFailure is the "error.kind" value for every other non-nil error
// a command returns: an infrastructure fault or anything else
// classifyRefusal does not recognize, always exit code 1 (R3).
const errorKindFailure = "failure"

// jsonHeader is embedded, first, in every --json document: schema,
// command, ok and exit_code precede any command-specific field, with no
// "data" wrapper (R2). command is the failing or succeeding command's own
// path, as commandName renders it — "brief" at the root.
type jsonHeader struct {
	Schema   int    `json:"schema"`
	Command  string `json:"command"`
	OK       bool   `json:"ok"`
	ExitCode int    `json:"exit_code"`
}

// newJSONHeader builds a jsonHeader for command at exitCode. ok is always
// derived from exitCode here; nothing else in this package sets it.
func newJSONHeader(command string, exitCode int) jsonHeader {
	return jsonHeader{
		Schema:   schemaVersion,
		Command:  command,
		OK:       exitCode == 0,
		ExitCode: exitCode,
	}
}

// jsonError is the "error" member of a failing --json document (R3).
// Every field is a pointer, or a plain string for one that is always
// filled, so an unused member marshals to null rather than being omitted:
// path, line and problem stay null for a usage error; a refusal or
// failure (reporter.refusal) leaves problem always filled, path null only
// for "<stdin>", a bare not-found or a generic failure, and line null
// unless the refusal names a specific one. files_changed is null for a
// read command or a pointer to false for a write one (filesChangedFor).
type jsonError struct {
	Kind         string  `json:"kind"`
	Message      string  `json:"message"`
	Path         *string `json:"path"`
	Line         *int    `json:"line"`
	Problem      *string `json:"problem"`
	Fix          string  `json:"fix"`
	FilesChanged *bool   `json:"files_changed"`
}

// errorDocument is the JSON document a failing run writes to stdout: the
// common header plus the error object as R3's only payload.
type errorDocument struct {
	jsonHeader

	Error jsonError `json:"error"`
}

// writeJSONDocument writes v to w as one compact JSON document followed
// by a single newline, encoded into a buffer first so w sees either the
// complete document or nothing — the same convention
// assemble.RenderJSON uses. '<' and '&' in a string field are left
// unescaped: the payload is not HTML.
func writeJSONDocument(w io.Writer, v any) error {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("cli: render json: %w", err)
	}

	if _, err := w.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("cli: render json: %w", err)
	}

	return nil
}

// scanJSONFlag scans args for an exact "--json" token, and for a
// "--json=<v>" token (any value, including an empty one), both only
// before the first "--" — pflag's own flag-parsing terminator, never a
// flag itself, and the boundary R5 draws: nothing at or after it is ever
// inspected. It returns three values: args with every exact "--json"
// token removed ("--json=<v>" tokens are left in place, since the caller
// reports that shape as a usage error before any command ever sees the
// result); whether an exact "--json" token was found; and whether a
// "--json=<v>" token was found.
func scanJSONFlag(args []string) ([]string, bool, bool) {
	stripped := make([]string, 0, len(args))

	var jsonMode, hasValue, boundary bool

	for _, a := range args {
		switch {
		case boundary:
			stripped = append(stripped, a)
		case a == "--":
			boundary = true

			stripped = append(stripped, a)
		case a == "--json":
			jsonMode = true
		case a == "--json=" || strings.HasPrefix(a, "--json="):
			hasValue = true

			stripped = append(stripped, a)
		default:
			stripped = append(stripped, a)
		}
	}

	return stripped, jsonMode, hasValue
}

// commandName renders cmd's own path the way every --json document's
// "command" field and every usage error's command prefix name it:
// cmd.CommandPath() with the "brief " root prefix trimmed, so a leaf
// reads "start" or "new feature" and root itself reads "brief".
func commandName(cmd *cobra.Command) string {
	return strings.TrimPrefix(cmd.CommandPath(), "brief ")
}

// usageHint names the invocation a usage error with no run hint of its
// own falls back to: a leaf's own invocation annotation, "brief new
// --help" for "new", "brief help <command>" for the help stub, and
// "brief --help" for root or anything else carrying neither.
func usageHint(cmd *cobra.Command) string {
	if inv := cmd.Annotations[invocationAnnotation]; inv != "" {
		return inv
	}

	switch commandName(cmd) {
	case "new":
		return "brief new --help"
	case "help":
		return "brief help <command>"
	default:
		return "brief --help"
	}
}

// usageFix names msg's own "; run '<hint>'" clause, stripped of the
// leading "; ", when msg ends with one — so a document's "fix" always
// names the same action as its "message" (R3) — else cmd's usageHint
// fallback. A message that merely contains "; run '" without ending in
// the closing quote (a run-hint clause followed by more prose) also
// falls back: the fallback and the clause happen to agree whenever cmd
// is the leaf that clause names, but usageFix never parses the clause
// out of message text that continues past it.
func usageFix(msg string, cmd *cobra.Command) string {
	const marker = "; run '"

	if idx := strings.LastIndex(msg, marker); idx != -1 && strings.HasSuffix(msg, "'") {
		return msg[idx+2:]
	}

	return "run '" + usageHint(cmd) + "'"
}

// filesChangedFor reports R3's "files_changed" value for command: false
// for a write command (new, new feature, new step, finish), nil (JSON
// null) for a read command.
func filesChangedFor(command string) *bool {
	switch command {
	case "new", "new feature", "new step", "finish":
		f := false

		return &f
	default:
		return nil
	}
}

// jsonTakesNoValueMessage renders "--json=<v>"'s always-text usage line
// (R5) for cmd: root's own bare "brief: '--json' takes no value; run
// '<hint>'" when cmd is root itself, else "brief <path>: '--json' takes
// no value; run '<hint>'" naming cmd's own resolved command path.
func jsonTakesNoValueMessage(cmd *cobra.Command) string {
	path := commandName(cmd)
	hint := usageHint(cmd)

	if path == "brief" {
		return takesNoValueMessage("--json", hint)
	}

	return fmt.Sprintf("brief %s: '--json' takes no value; run '%s'", path, hint)
}

// reporter is the one per-Run output seam every command renders through:
// stdout and stderr are Run's own writers, json is whether R5's --json
// detection turned JSON mode on for this run, wd is Run's own working
// directory (R6: the base every relative refusal path is absolutized
// against), and cmd is the command currently rendering, set by forCommand.
// Every RunE closure and the root FlagErrorFunc narrow the base reporter
// built in run with forCommand before rendering anything.
type reporter struct {
	stdout io.Writer
	stderr io.Writer
	json   bool
	wd     string
	cmd    *cobra.Command
}

// forCommand returns r narrowed to cmd: every later usageError call
// derives its "command" and its fallback "fix" from cmd.
func (r reporter) forCommand(cmd *cobra.Command) reporter {
	r.cmd = cmd

	return r
}

// successHeader builds the jsonHeader every success document embeds
// first: command from r.cmd, exit_code 0, ok true.
func (r reporter) successHeader() jsonHeader {
	return newJSONHeader(commandName(r.cmd), 0)
}

// usageError renders msg as R3's usage-error document: in JSON mode, one
// compact document on stdout (kind "usage", exit_code 2, message msg,
// path/line/problem null, fix from usageFix, files_changed from
// filesChangedFor) and zero bytes on stderr; otherwise msg plus a
// trailing newline on stderr, byte-identical to brief's plain-text usage
// error. Both modes return the same error satisfying
// errors.Is(err, ErrUsage).
func (r reporter) usageError(msg string) error {
	if r.json {
		command := commandName(r.cmd)
		doc := errorDocument{
			jsonHeader: newJSONHeader(command, ExitCode(ErrUsage)),
			Error: jsonError{
				Kind:         errorKindUsage,
				Message:      msg,
				Fix:          usageFix(msg, r.cmd),
				FilesChanged: filesChangedFor(command),
			},
		}

		_ = writeJSONDocument(r.stdout, doc)

		return fmt.Errorf("%s: %w", msg, ErrUsage)
	}

	fmt.Fprintln(r.stderr, msg)

	return fmt.Errorf("%s: %w", msg, ErrUsage)
}
