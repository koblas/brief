package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/rwfs"
	"github.com/koblas/brief/internal/scaffold"
	"github.com/koblas/brief/internal/setup"
)

// flattenOneLine collapses s to a single line: embedded newlines and runs
// of whitespace become one space each. yaml.v3 reports an unknown-key
// failure as "yaml: unmarshal errors:\n  line N: …", and every refusal's
// one-line contract requires that to survive unchanged.
func flattenOneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

// noFilesChangedTail is the "nothing changed on disk" promise a refusal's
// text line appends. classifyRefusal decides it per error, never by type
// alone: every typed refusal carries it except the named exceptions —
// *setup.RefusalError drops it only when the specific error means a write
// already partially landed (setup.ErrPartialWrite); *config.InvalidConfigError
// always carries it, whether or not the command that hit it writes anything
// itself, since config resolution runs before every command's own writes,
// never after; a bare not-found or a generic failure carries none, having
// nothing to promise about. A *unknownFeatureError inherits its own promise
// from the sentinel it wraps rather than always carrying one:
// scaffold.ErrNoSuchFeature's own callers ("new step", "finish") write and
// refused before touching disk, so it carries the tail; assemble.ErrNoSuchFeature's
// own callers ("start", "check") are read-only, so it never does.
const noFilesChangedTail = " (no files changed)"

// refusalTextLayout selects which of refusalClassification.textLine's three
// shapes a classification renders through: layoutPathProblem (the default,
// zero value) for every ordinary refusal, layoutProblemInPath for
// *unknownFeatureError, whose ruled copy puts the problem before the path
// it names rather than after it, and layoutProblemOnly for scaffold's
// unknown-step refusal, whose Problem already names the feature by its own
// argument — never a path — so path renders in JSON only.
type refusalTextLayout int

const (
	// layoutPathProblem renders "<path>[:<line>]: <problem>; <fix><tail>".
	layoutPathProblem refusalTextLayout = iota
	// layoutProblemInPath renders "<problem> in <path>; <fix><tail>".
	layoutProblemInPath
	// layoutProblemOnly renders "<problem>; <fix><tail>", ignoring path
	// entirely in text — path still flows to jsonPath for --json.
	layoutProblemOnly
)

// refusalClassification is classifyRefusal's pure output: the R14a
// text-mode line's own ingredients (path, line, problem, fix, tail, layout)
// and R3's error.kind, both built from err alone with no knowledge of the
// failing command or the working directory a relative path is resolved
// against — (reporter).refusal supplies both.
type refusalClassification struct {
	// kind is errorKindRefusal or errorKindFailure.
	kind string
	// path is the refusal's own path exactly as the error carries it —
	// relative, absolute, or "<stdin>" — used verbatim in the text-mode
	// line. It is "" for a generic failure, which names no path in either
	// mode.
	path string
	// line is the refusal's own 1-based line, 0 when it names no specific
	// line within path.
	line int
	// problem is always filled: the refusal's own Problem/Detail, or the
	// flattened err.Error() for a generic failure.
	problem string
	// fix is the refusal's own Fix. It is "" for a generic failure —
	// errorKindFailure — whose fix depends on the failing command, filled
	// in by (reporter).refusal.
	fix string
	// tail is noFilesChangedTail for a write refusal, "" otherwise — and
	// "" even for a write refusal whose err wraps setup.ErrPartialWrite,
	// since that promise is false once an earlier write already landed.
	tail string
	// layout selects textLine's rendering shape; layoutPathProblem (the
	// zero value) unless set otherwise.
	layout refusalTextLayout
}

// classifyRefusal renders err into the R14a text-mode line's ingredients
// and R3's error.kind. *setup.RefusalError is checked first, ahead of
// *config.InvalidConfigError: init's own config refusal
// (setup.configRefusal) wraps a *config.InvalidConfigError as its Err, and
// errors.AsType would reach that inner error through *setup.RefusalError's
// own Unwrap if the InvalidConfigError branch ran first — silently
// replacing init's "run 'brief init --force'" fix with the generic
// "remove it" copy every other command's own bare InvalidConfigError
// carries. *unknownFeatureError is checked next, ahead of the two other
// typed refusals: scaffold's own not-found (noSuchFeatureRefusal) is
// itself a *scaffold.RefusalError wrapping scaffold.ErrNoSuchFeature, so a
// *unknownFeatureError built around one — enrichUnknownFeature wraps the
// original error unchanged — would be silently reclassified by the
// scaffold.RefusalError branch below it if checked in the other order,
// keeping its old path-less copy for "new step"/"finish" while "start"
// (whose not-found is the bare assemble.ErrNoSuchFeature sentinel, never a
// *scaffold.RefusalError) looked fixed. Anything else falls to the generic
// errorKindFailure case.
func classifyRefusal(err error) refusalClassification {
	if refusal, ok := errors.AsType[*setup.RefusalError](err); ok {
		tail := noFilesChangedTail
		if errors.Is(err, setup.ErrPartialWrite) {
			tail = ""
		}

		return refusalClassification{
			kind:    errorKindRefusal,
			path:    refusal.Path,
			line:    refusal.Line,
			problem: flattenOneLine(refusal.Problem),
			fix:     flattenOneLine(refusal.Fix),
			tail:    tail,
		}
	}

	if invalidCfg, ok := errors.AsType[*config.InvalidConfigError](err); ok {
		return refusalClassification{
			kind:    errorKindRefusal,
			path:    invalidCfg.Path,
			problem: flattenOneLine(invalidCfg.Err.Error()),
			fix:     "correct the value, or delete the key to use its default",
			tail:    noFilesChangedTail,
		}
	}

	if unknown, ok := errors.AsType[*unknownFeatureError](err); ok {
		tail := ""
		if errors.Is(unknown.err, scaffold.ErrNoSuchFeature) {
			tail = noFilesChangedTail
		}

		return refusalClassification{
			kind:    errorKindRefusal,
			layout:  layoutProblemInPath,
			path:    unknown.dir,
			problem: fmt.Sprintf("no feature %q", unknown.name),
			fix:     knownFeaturesFix(unknown.known, unknown.name),
			tail:    tail,
		}
	}

	if refusal, ok := errors.AsType[*scaffold.RefusalError](err); ok {
		if errors.Is(refusal.Err, scaffold.ErrNoSuchStep) {
			return refusalClassification{
				kind:    errorKindRefusal,
				layout:  layoutProblemOnly,
				path:    refusal.Path,
				problem: flattenOneLine(refusal.Problem),
				fix:     flattenOneLine(refusal.Fix),
				tail:    noFilesChangedTail,
			}
		}

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

	return refusalClassification{
		kind:    errorKindFailure,
		problem: flattenOneLine(err.Error()),
	}
}

// unknownFeatureError enriches a not-found sentinel — scaffold.ErrNoSuchFeature
// or assemble.ErrNoSuchFeature, bare or wrapped — with the feature
// directory's own known list, so classifyRefusal's copy can name what does
// exist rather than only what doesn't. name is the feature argument as
// given (never resolved against dir), dir is the absolute feature
// directory, known is assemble.Features' own result (fs.ReadDir order,
// real directories only), and err is the original error unchanged, so
// Unwrap keeps errors.Is(scaffold.ErrNoSuchFeature) /
// errors.Is(assemble.ErrNoSuchFeature) reachable through this wrapper.
type unknownFeatureError struct {
	name  string
	dir   string
	known []string
	err   error
}

// Error renders "no feature "<name>" in <dir>" — dir absolute here; cli's
// text-mode line relativizes it through displayPath at render time.
func (e *unknownFeatureError) Error() string {
	return fmt.Sprintf("no feature %q in %s", e.name, e.dir)
}

// Unwrap exposes err so errors.Is/errors.As reach the sentinel this error
// wraps.
func (e *unknownFeatureError) Unwrap() error {
	return e.err
}

// knownFeaturesFix renders unknownFeatureError's "fix" segment: the known
// list joined by ", " when non-empty, else the suggestion to create name.
func knownFeaturesFix(known []string, name string) string {
	if len(known) == 0 {
		return fmt.Sprintf("known: none; run 'brief new feature %s' to create it", name)
	}

	return "known: " + strings.Join(known, ", ")
}

// enrichUnknownFeature recognizes a scaffold.ErrNoSuchFeature or
// assemble.ErrNoSuchFeature carried anywhere in err's chain, for feature
// under the configuration cfg resolves root against, and returns a
// *unknownFeatureError naming every known feature (assemble.Features,
// fs.ReadDir order). A failure listing the feature directory itself is
// returned in its place — never rendered as "known: none". Any other err,
// including nil, passes through unchanged. rootFS is nil in production
// (the assemble.Server this builds reads real disk); a test's withRootFS
// runSeam substitutes an rwfs.Mem, the same fixture the caller's own
// scaffold.Server or assemble.Server already reads.
func enrichUnknownFeature(ctx context.Context, cfg config.Config, root, feature string, err error, rootFS rwfs.FS) error {
	if !errors.Is(err, scaffold.ErrNoSuchFeature) && !errors.Is(err, assemble.ErrNoSuchFeature) {
		return err
	}

	srv := assemble.NewServer(cfg, root, assemble.WithFS(rootFS))

	known, listErr := srv.Features(ctx)
	if listErr != nil {
		return fmt.Errorf("cli: %w", listErr)
	}

	return &unknownFeatureError{
		name:  feature,
		dir:   filepath.Join(root, cfg.FeatureDirectory),
		known: known,
		err:   err,
	}
}

// displayPath renders p the way every text-mode line names a path (R6): ""
// and "<stdin>" pass through unchanged — neither is a real path to
// relativize — a path that is already relative (finish rewrites a refusal's
// Path to the user's own, possibly relative, "--state"/"--handoff" argument)
// also passes through unchanged, and an absolute path is rendered relative
// to wd via filepath.Rel, falling back to p itself on error. A path outside
// wd renders as a "../" chain rather than being clamped to p: a working
// directory below the configuration root that owns p is expected to see
// one, and clamping it would make status.go/start.go's own text line diverge
// from what --json's absolute "path" names.
func displayPath(wd, p string) string {
	if p == "" || p == "<stdin>" || !filepath.IsAbs(p) {
		return p
	}

	rel, err := filepath.Rel(wd, p)
	if err != nil {
		return p
	}

	return rel
}

// textLine renders c's R14a text-mode line, minus the "brief <command>: "
// prefix: c.problem alone when c.path is "" — a generic failure names no
// path in text and never appends c.fix there either —
// "<problem>; <fix>[<tail>]" when c.layout is layoutProblemOnly, which
// ignores c.path in text regardless of whether it is set (c.path still
// flows to jsonPath for --json), else one of two shapes selected by
// c.layout, both with c.path rendered relative to wd through displayPath:
// layoutPathProblem's default "<path>[:<line>]: <problem>; <fix>[<tail>]",
// or layoutProblemInPath's "<problem> in <path>; <fix>[<tail>]".
func (c refusalClassification) textLine(wd string) string {
	if c.layout == layoutProblemOnly {
		return fmt.Sprintf("%s; %s%s", c.problem, c.fix, c.tail)
	}

	if c.path == "" {
		return c.problem
	}

	location := displayPath(wd, c.path)
	if c.line > 0 {
		location = fmt.Sprintf("%s:%d", location, c.line)
	}

	if c.layout == layoutPathProblem {
		return fmt.Sprintf("%s: %s; %s%s", location, c.problem, c.fix, c.tail)
	}

	return fmt.Sprintf("%s in %s; %s%s", c.problem, location, c.fix, c.tail)
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
				Message:      fmt.Sprintf("brief %s: %s", command, c.textLine(r.wd)),
				Path:         c.jsonPath(r.wd),
				Line:         c.jsonLine(),
				Problem:      &problem,
				Fix:          fix,
				FilesChanged: filesChangedFor(r.cmd, err),
			},
		}

		r.writeErrorDocument(doc)

		return err
	}

	fmt.Fprintf(r.stderr, "brief %s: %s\n", command, c.textLine(r.wd))

	return err
}

// stdoutWriteError renders cause — a failed write of a command's own
// output to stdout — as R14's one-line generic failure: the fix re-runs
// hint when filesChanged is false, and warns that files already changed
// when it is true, never carrying R14a's "(no files changed)" tail. cause
// is narrowed to the *fs.PathError the writer returned, when there is
// one, dropping the "assemble: render: "-style prefixes a renderer wraps
// it in.
func stdoutWriteError(cause error, hint string, filesChanged bool) error {
	if pathErr, ok := errors.AsType[*fs.PathError](cause); ok {
		cause = pathErr
	}

	fix := fmt.Sprintf("fix the output destination, then run '%s' again", hint)
	if filesChanged {
		fix = fmt.Sprintf("files were already changed, check them with 'git status' before running '%s' again", hint)
	}

	return fmt.Errorf("writing to stdout failed: %w; %s", cause, fix)
}

// stdoutFailure reports cause through stdoutWriteError, re-running r.cmd's
// own usageHint, on stderr in text and --json mode alike: stdout is the
// stream that just failed, so --json's error document has nowhere to go,
// and main prints nothing a command returns. filesChanged is whether this
// run changed any file before its output failed.
func (r reporter) stdoutFailure(cause error, filesChanged bool) error {
	text := r
	text.json = false

	return text.refusal(stdoutWriteError(cause, usageHint(r.cmd), filesChanged))
}
