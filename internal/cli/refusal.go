package cli

import (
	"context"
	"errors"
	"fmt"
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
// text line appends; classifyRefusal decides per error whether it applies.
const noFilesChangedTail = " (no files changed)"

// refusalTextLayout selects which of refusalClassification.textLine's
// three shapes a classification renders through.
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

// refusalClassification is classifyRefusal's pure output: the text-mode
// line's own ingredients (path, line, problem, fix, tail, layout) and the
// error.kind, both built from err alone with no knowledge of the failing
// command or the working directory a relative path is resolved against —
// (reporter).refusal supplies both.
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

// classifyRefusal renders err into the text-mode line's ingredients and
// its error.kind, checking the typed refusals below in a fixed order:
// anything else falls to the generic errorKindFailure case.
func classifyRefusal(err error) refusalClassification {
	// Checked ahead of *config.InvalidConfigError: init's own config
	// refusal wraps one as its Err, and checking that branch first would
	// silently reach the inner error through Unwrap, replacing init's own
	// "run 'brief init --force'" fix with the generic "remove it" copy.
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

	// Checked ahead of *scaffold.RefusalError: scaffold's own not-found is
	// itself a *scaffold.RefusalError wrapping scaffold.ErrNoSuchFeature,
	// so checking that branch first would silently reclassify an enriched
	// *unknownFeatureError back to its path-less copy.
	if unknown, ok := errors.AsType[*unknownFeatureError](err); ok {
		tail := ""
		// scaffold's callers ("new step", "finish") write and refused
		// before touching disk; assemble's callers ("start", "check") are
		// read-only and never carry the tail.
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
// exist rather than only what doesn't. err is the original error
// unchanged, so Unwrap keeps it reachable through this wrapper.
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
// *unknownFeatureError naming every known feature. A failure listing the
// feature directory itself is returned in its place, never rendered as
// "known: none". Any other err, including nil, passes through unchanged.
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

// displayPath renders p the way every text-mode line names a path: "" and
// "<stdin>" pass through unchanged, as does a path that is already
// relative (finish rewrites a refusal's Path to the user's own, possibly
// relative, "--state"/"--handoff" argument); an absolute path renders
// relative to wd via filepath.Rel, falling back to p itself on error. A
// path outside wd renders as a "../" chain rather than being clamped to
// p, so the text line never diverges from what --json's absolute "path"
// names.
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

// textLine renders c's text-mode line, minus the "brief <command>: "
// prefix: c.problem alone when c.path is "" (a generic failure names no
// path in text), "<problem>; <fix>[<tail>]" when c.layout is
// layoutProblemOnly (c.path still flows to jsonPath for --json), else one
// of two shapes selected by c.layout, both with c.path rendered relative
// to wd through displayPath: layoutPathProblem's default
// "<path>[:<line>]: <problem>; <fix>[<tail>]", or layoutProblemInPath's
// "<problem> in <path>; <fix>[<tail>]".
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
// "<stdin>", neither a real path to name, else c.path joined onto wd when
// relative, unchanged when already absolute.
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
// other non-nil error a command returns — as a one-line text-mode message
// on stderr, or an error document on stdout under --json, and returns err
// unchanged for ExitCode to classify. A JSON document's "message" is
// always the same text-mode line, byte for byte.
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

		_ = writeJSONDocument(r.stdout, doc)

		return err
	}

	fmt.Fprintf(r.stderr, "brief %s: %s\n", command, c.textLine(r.wd))

	return err
}
