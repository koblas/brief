package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/scaffold"
)

// flattenOneLine collapses s to a single line: embedded newlines and runs
// of whitespace become one space each. yaml.v3 reports an unknown-key
// failure as "yaml: unmarshal errors:\n  line N: …", and every refusal's
// one-line contract requires that to survive unchanged.
func flattenOneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

// noFilesChangedTail is the "nothing changed on disk" promise appended to
// a write command's refusal: a *config.InvalidConfigError or a
// *scaffold.RefusalError, both of which concern a write that never
// happened. A *assemble.RefusalError carries no such promise — assemble
// never writes, so there is nothing for it to promise — and neither does a
// bare not-found or a generic failure.
const noFilesChangedTail = " (no files changed)"

// refusalClassification is classifyRefusal's pure output: the R14a
// text-mode line's own ingredients (path, line, problem, fix, tail) and
// R3's error.kind, both built from err alone with no knowledge of the
// failing command or the working directory a relative path is resolved
// against — (reporter).refusal supplies both.
type refusalClassification struct {
	// kind is errorKindRefusal or errorKindFailure.
	kind string
	// path is the refusal's own path exactly as the error carries it —
	// relative, absolute, or "<stdin>" — used verbatim in the text-mode
	// line. It is "" for a bare not-found or a generic failure, neither of
	// which names a path in either mode.
	path string
	// line is the refusal's own 1-based line, 0 when it names no specific
	// line within path.
	line int
	// problem is always filled: the refusal's own Problem/Detail, or the
	// flattened err.Error() for a bare not-found or a generic failure.
	problem string
	// fix is the refusal's own Fix, or the bare not-found's constant fix.
	// It is "" for a generic failure — errorKindFailure — whose fix
	// depends on the failing command, filled in by (reporter).refusal.
	fix string
	// tail is noFilesChangedTail for a write refusal, "" otherwise.
	tail string
}

// classifyRefusal renders err into the R14a text-mode line's ingredients
// and R3's error.kind. The three typed refusals are checked first, the
// bare assemble.ErrNoSuchFeature sentinel after them, and anything else
// falls to the generic errorKindFailure case: assemble.Start and
// assemble.Check return ErrNoSuchFeature bare, never wrapped in a
// *assemble.RefusalError, so a sentinel check ahead of, or instead of, the
// typed checks would never fire for them; running it after guarantees a
// *scaffold.RefusalError (which always wraps scaffold's own, distinct
// ErrNoSuchFeature sentinel) is classified by the scaffold.RefusalError
// check first, keeping its path, problem and fix rather than being
// stripped to the bare not-found's path-less shape.
func classifyRefusal(err error) refusalClassification {
	if invalidCfg, ok := errors.AsType[*config.InvalidConfigError](err); ok {
		return refusalClassification{
			kind:    errorKindRefusal,
			path:    invalidCfg.Path,
			problem: flattenOneLine(invalidCfg.Err.Error()),
			fix:     "fix it or remove it to fall back to the shipped defaults",
			tail:    noFilesChangedTail,
		}
	}

	if refusal, ok := errors.AsType[*scaffold.RefusalError](err); ok {
		return refusalClassification{
			kind:    errorKindRefusal,
			path:    refusal.Path,
			line:    refusal.Line,
			problem: flattenOneLine(refusal.Problem),
			fix:     flattenOneLine(refusal.Fix),
			tail:    noFilesChangedTail,
		}
	}

	if refusal, ok := errors.AsType[*assemble.RefusalError](err); ok {
		return refusalClassification{
			kind:    errorKindRefusal,
			path:    refusal.Path,
			line:    refusal.Line,
			problem: flattenOneLine(refusal.Detail),
			fix:     flattenOneLine(refusal.Fix),
		}
	}

	if errors.Is(err, assemble.ErrNoSuchFeature) {
		return refusalClassification{
			kind:    errorKindRefusal,
			problem: flattenOneLine(err.Error()),
			fix:     "run 'brief status' to list the known features",
		}
	}

	return refusalClassification{
		kind:    errorKindFailure,
		problem: flattenOneLine(err.Error()),
	}
}

// textLine renders c's R14a text-mode line, minus the "brief <command>: "
// prefix: c.problem alone when c.path is "" — a bare not-found or a
// generic failure names no path in text and never appends c.fix there
// either — else "<path>[:<line>]: <problem>; <fix>[<tail>]".
func (c refusalClassification) textLine() string {
	if c.path == "" {
		return c.problem
	}

	location := c.path
	if c.line > 0 {
		location = fmt.Sprintf("%s:%d", c.path, c.line)
	}

	return fmt.Sprintf("%s: %s; %s%s", location, c.problem, c.fix, c.tail)
}

// jsonPath renders c.path for --json's "path" field: null for "" or
// "<stdin>" (R6 never names a placeholder or piped source as a real
// path), else c.path joined onto wd when relative, unchanged when already
// absolute.
func (c refusalClassification) jsonPath(wd string) *string {
	if c.path == "" || c.path == "<stdin>" {
		return nil
	}

	path := c.path
	if !filepath.IsAbs(path) {
		path = filepath.Join(wd, path)
	}

	return &path
}

// jsonLine renders c.line for --json's "line" field: null when c.line is
// not a positive 1-based line number.
func (c refusalClassification) jsonLine() *int {
	if c.line <= 0 {
		return nil
	}

	line := c.line

	return &line
}

// refusal renders err — a refusal that changed nothing on disk, or any
// other non-nil error a command returns — as R14a's one-line text-mode
// message on stderr, or R3's error document on stdout under --json, and
// returns err unchanged for ExitCode to classify. path absolutizes against
// r.wd (R6); a JSON document's "message" is always the same text-mode line
// R14a would have printed, byte for byte.
func (r reporter) refusal(err error) error {
	c := classifyRefusal(err)
	command := commandName(r.cmd)

	if r.json {
		fix := c.fix
		if c.kind == errorKindFailure {
			fix = fmt.Sprintf("resolve the problem, then run '%s' again", usageHint(r.cmd))
		}

		problem := c.problem
		doc := errorDocument{
			jsonHeader: newJSONHeader(command, ExitCode(err)),
			Error: jsonError{
				Kind:         c.kind,
				Message:      fmt.Sprintf("brief %s: %s", command, c.textLine()),
				Path:         c.jsonPath(r.wd),
				Line:         c.jsonLine(),
				Problem:      &problem,
				Fix:          fix,
				FilesChanged: filesChangedFor(command),
			},
		}

		_ = writeJSONDocument(r.stdout, doc)

		return err
	}

	fmt.Fprintf(r.stderr, "brief %s: %s\n", command, c.textLine())

	return err
}
