package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/koblas/brief/internal/scaffold"
	"github.com/koblas/brief/internal/setup"
	"github.com/spf13/cobra"
)

// schemaVersion is every --json document's "schema" field, bumped only on
// a breaking change to a document's shape.
const schemaVersion = 1

// errorKindUsage is the "error.kind" value for a usage error's JSON
// document: an invocation mistake caught before any command ran, always
// exit code 2.
const errorKindUsage = "usage"

// errorKindRefusal is the "error.kind" value for a refusal's JSON
// document — every case classifyRefusal recognizes by type — always exit code 1.
const errorKindRefusal = "refusal"

// errorKindFailure is the "error.kind" value for every other non-nil
// error a command returns, always exit code 1.
const errorKindFailure = "failure"

// jsonHeader is embedded, first, in every --json document: schema,
// command, ok and exit_code precede any command-specific field, with no
// "data" wrapper. command is the failing or succeeding command's own
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

// jsonError is the "error" member of a failing --json document. Every
// field is a pointer, or a plain string for one that is always filled, so
// an unused member marshals to null rather than being omitted:
// path, line and problem stay null for a usage error; a refusal or
// failure (reporter.refusal) leaves problem always filled, path null only
// for "<stdin>", a bare not-found or a generic failure, and line null
// unless the refusal names a specific one. files_changed is null for a
// read command; for a write command it is what actually happened on disk —
// false when nothing was written, true when at least one write landed
// before the failure (filesChangedFor).
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
// common header plus the error object as its only payload.
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

// scanJSONFlag scans args for an exact "--json" token and for a
// "--json=<v>" token, both only before the first "--" (pflag's
// end-of-flags terminator; nothing at or after it is inspected). It
// returns args with every exact "--json" token removed ("--json=<v>" is
// left in place, since the caller reports that shape as a usage error),
// whether an exact "--json" token was found, and whether "--json=<v>" was.
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
		case strings.HasPrefix(a, "--json="):
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
// names the same action as its "message" — else cmd's usageHint fallback.
// A message that merely contains "; run '" without ending in the closing
// quote also falls back, rather than parsing a clause out of text that
// continues past it.
func usageFix(msg string, cmd *cobra.Command) string {
	const marker = "; run '"

	if idx := strings.LastIndex(msg, marker); idx != -1 && strings.HasSuffix(msg, "'") {
		return msg[idx+2:]
	}

	return "run '" + usageHint(cmd) + "'"
}

// filesChangedFor reports the "files_changed" value for cmd: nil for any
// command not carrying writesFilesAnnotation, since a read command never
// changes anything to report on; for a write command, whether err wraps
// scaffold.ErrPartialWrite or setup.ErrPartialWrite — true only when at
// least one write landed before the failure that reached cli.
func filesChangedFor(cmd *cobra.Command, err error) *bool {
	if cmd.Annotations[writesFilesAnnotation] == "" {
		return nil
	}

	f := errors.Is(err, scaffold.ErrPartialWrite) || errors.Is(err, setup.ErrPartialWrite)

	return &f
}

// jsonTakesNoValueMessage renders "--json=<v>"'s always-text usage line
// for cmd: root's own bare "brief: '--json' takes no value; run '<hint>'"
// when cmd is root itself, else "brief <path>: '--json' takes no value;
// run '<hint>'" naming cmd's own resolved command path. hint appends
// " --json" to usageHint's invocation only when cmd carries one
// (cmd.Annotations[invocationAnnotation]) — usageHint's generic fallbacks
// name no JSON-capable leaf invocation to append it to.
func jsonTakesNoValueMessage(cmd *cobra.Command) string {
	path := commandName(cmd)
	hint := usageHint(cmd)

	if cmd.Annotations[invocationAnnotation] != "" {
		hint += " --json"
	}

	if path == "brief" {
		return takesNoValueMessage("--json", hint)
	}

	return fmt.Sprintf("brief %s: '--json' takes no value; run '%s'", path, hint)
}

// reporter is the one per-Run output seam every command renders through:
// stdout and stderr are Run's own writers, json is whether --json
// detection turned JSON mode on for this run, wd is the base every
// relative refusal path is absolutized against, and cmd is the command
// currently rendering, set by forCommand.
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

// headerFor builds the jsonHeader a success-shaped document embeds first,
// at exitCode: command from r.cmd, ok derived from exitCode by
// newJSONHeader. A document with findings-as-data, like check --json's,
// uses this directly at a non-zero exitCode; successHeader is the
// exitCode-0 special case.
func (r reporter) headerFor(exitCode int) jsonHeader {
	return newJSONHeader(commandName(r.cmd), exitCode)
}

// successHeader builds the jsonHeader every exit-0 success document embeds
// first: command from r.cmd, exit_code 0, ok true.
func (r reporter) successHeader() jsonHeader {
	return r.headerFor(0)
}

// usageError renders msg as a usage-error document: in JSON mode, one
// compact document on stdout (kind "usage", exit_code 2, message msg,
// path/line/problem null, fix from usageFix, files_changed from
// filesChangedFor) and zero bytes on stderr; otherwise msg plus a
// trailing newline on stderr. Both modes return the same error satisfying
// errors.Is(err, ErrUsage).
func (r reporter) usageError(msg string) error {
	return r.usageErrorWithFix(msg, usageFix(msg, r.cmd))
}

// usageErrorWithFix renders msg exactly like usageError, but uses fix
// verbatim for JSON's own "fix" field rather than deriving it from msg via
// usageFix. It exists for a message, such as init's unknown-host line,
// that carries a "; run '...'" clause but also trails prose after its
// closing quote: usageFix's extraction would stop at that quote and
// silently substitute the wrong fix, so the call site supplies it directly.
func (r reporter) usageErrorWithFix(msg, fix string) error {
	if r.json {
		command := commandName(r.cmd)
		doc := errorDocument{
			jsonHeader: newJSONHeader(command, ExitCode(ErrUsage)),
			Error: jsonError{
				Kind:         errorKindUsage,
				Message:      msg,
				Fix:          fix,
				FilesChanged: filesChangedFor(r.cmd, nil),
			},
		}

		_ = writeJSONDocument(r.stdout, doc)

		return fmt.Errorf("%s: %w", msg, ErrUsage)
	}

	fmt.Fprintln(r.stderr, msg)

	return fmt.Errorf("%s: %w", msg, ErrUsage)
}

// document writes v — one of status, check, start, finish, new feature or
// new step's own success document — to r.stdout as one JSON document,
// wrapping a write failure with "brief <path>: " naming r.cmd's own
// command path.
func (r reporter) document(v any) error {
	if err := writeJSONDocument(r.stdout, v); err != nil {
		return fmt.Errorf("brief %s: %w", commandName(r.cmd), err)
	}

	return nil
}
