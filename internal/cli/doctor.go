package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/koblas/brief/internal/doctor"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/repo"
)

// doctorLong is "brief doctor"'s help prose.
var doctorLong = `Checks brief's own setup: the ".brief.yaml" config (present, parses,
every value valid, and whether it shadows an ancestor config), the
feature root (exists, is a directory, is readable and writable), the host
environment (a ".git" above the working directory, and whether the
"brief" on PATH matches the one running), and, for Claude Code, the
skills-directory plugin, its hook, the CLAUDE.md instruction block and the
three role agents — plus whether each role bound in ".brief.yaml" resolves
to an agent, matched by its frontmatter "name:" anywhere under
".claude/agents/", then "~/.claude/agents/" when the repository defines
none, and whether the bound planner and implementer preload the
"brief-workflow" skill. It never reads a feature's own content — that
is brief check's job.

Prints one row per check, in a fixed order:

  <SEVERITY>  <id>  <path>  <detail>

SEVERITY is OK, WARN, ERROR or SKIP. brief doctor exits 1 when any check
is ERROR, 0 otherwise — a WARN alone still exits 0. brief doctor reads,
except for a brief writability probe in the feature root, immediately
removed.

` + jsonFieldsParagraph("counts", "checks") + " " + jsonScriptHint

// doctorInvocation is the invocation string every "brief doctor" usage
// error names as how to fix it.
const doctorInvocation = "brief doctor"

// errDoctorFindings marks a run of runDoctor that printed at least one
// ERROR-severity check: ExitCode's default branch maps any non-nil,
// non-ErrUsage error to 1, so this sentinel needs no case of its own
// there. It carries no message: every user-facing line was already
// written by the row renderer and the stderr summary below it.
var errDoctorFindings = errors.New("doctor reported an error-severity check")

// doctorCountsJSON is doctorDocument's "counts" member: the same tally
// doctorSummary's text-mode line and runDoctor's own exit decision both
// use, so exit_code == 1 iff counts.error > 0 by construction.
type doctorCountsJSON struct {
	Error int `json:"error"`
	Warn  int `json:"warn"`
	OK    int `json:"ok"`
	Skip  int `json:"skip"`
}

// doctorCheckJSON is one doctorDocument "checks" row: id, severity, path,
// detail and fix exactly as doctor.Check carries them, raw and never
// flattened. Path is nil when doctor.Check.Path is "".
type doctorCheckJSON struct {
	ID       string  `json:"id"`
	Severity string  `json:"severity"`
	Path     *string `json:"path"`
	Detail   string  `json:"detail"`
	Fix      *string `json:"fix"`
}

// doctorDocument is doctor's --json success document: the common header,
// then counts, then one row per check in Diagnose's fixed order, no
// "data" wrapper. Checks render as this document's payload even when
// counts.error is greater than zero: doctor --json never renders an
// ERROR-carrying run as an error document.
type doctorDocument struct {
	jsonHeader

	Counts doctorCountsJSON  `json:"counts"`
	Checks []doctorCheckJSON `json:"checks"`
}

// doctorChecksJSON maps checks to doctorDocument's "checks" array: a
// sized, non-nil slice so zero checks encode as "[]" rather than "null" —
// never reachable in practice, since Diagnose always returns at least the
// config family, but held to the same convention as checkFeatures and
// statusFeatures.
func doctorChecksJSON(checks []doctor.Check) []doctorCheckJSON {
	out := make([]doctorCheckJSON, 0, len(checks))

	for _, c := range checks {
		var path *string
		if c.Path != "" {
			p := c.Path
			path = &p
		}

		out = append(out, doctorCheckJSON{ID: c.ID, Severity: string(c.Severity), Path: path, Detail: c.Detail, Fix: c.Fix})
	}

	return out
}

// doctorRow renders one Check as a text-mode line (no trailing newline):
// SEVERITY, id, path and detail, joined by exactly two spaces, no column
// padding. path is "-" when c.Path is "". A non-nil Fix renders as
// "; fix: <fix>" appended to detail.
func doctorRow(wd string, c doctor.Check) string {
	path := "-"
	if c.Path != "" {
		path = displayPath(wd, c.Path)
	}

	detail := c.Detail
	if c.Fix != nil {
		detail = fmt.Sprintf("%s; fix: %s", detail, *c.Fix)
	}

	return strings.Join([]string{string(c.Severity), c.ID, path, detail}, "  ")
}

// doctorSummary renders runDoctor's stderr summary, minus the
// "brief doctor: " prefix: "setup ok; run 'brief check' for feature
// content" when counts carries no ERROR and no WARN, else "N ERROR, M
// WARN; this checks setup only, run 'brief check' for feature content".
func doctorSummary(counts doctor.Counts) string {
	if counts.Error == 0 && counts.Warn == 0 {
		return "setup ok; run 'brief check' for feature content"
	}

	return fmt.Sprintf("%d ERROR, %d WARN; this checks setup only, run 'brief check' for feature content", counts.Error, counts.Warn)
}

// runDoctor implements "brief doctor [--json]"; rest is its positional
// arguments, flags already parsed away, and must be empty. It never calls
// resolveRoot: an invalid or unparseable ".brief.yaml" is reported as a
// row, never a refusal. Its own config-location pre-check — a
// nonexistent-startDir guard unreachable when wd comes from os.Getwd — is
// the one refusal runDoctor emits; every other setup fault becomes a
// Check. extraOpts are appended after runDoctor's own doctor.WithVersion,
// so a caller can override any seam, including the version, by supplying
// it again.
func runDoctor(ctx context.Context, wd string, rest []string, readBuildInfo func() (*debug.BuildInfo, bool), out reporter, rootFS fs.FS, extraOpts ...doctor.Option) error {
	if len(rest) > 0 {
		return out.usageError(fmt.Sprintf("brief doctor: too many arguments; run '%s'", doctorInvocation))
	}

	locateErr := locateInRepo(wd)
	if rootFS != nil {
		locateErr = locateInRepoFS(rootFS, wd)
	}

	if locateErr != nil {
		return out.refusal(locateErr)
	}

	opts := append([]doctor.Option{doctor.WithVersion(versionString(readBuildInfo))}, extraOpts...)
	srv := doctor.NewServer(opts...)

	report := srv.Diagnose(ctx, wd)
	counts := report.Counts()

	var runErr error
	if counts.Error > 0 {
		runErr = errDoctorFindings
	}

	// Decided before any text-mode line is written, so a --json run
	// writes zero stderr bytes and sees Diagnose's own absolute paths.
	if out.json {
		doc := doctorDocument{
			jsonHeader: out.headerFor(ExitCode(runErr)),
			Counts:     doctorCountsJSON{Error: counts.Error, Warn: counts.Warn, OK: counts.OK, Skip: counts.Skip},
			Checks:     doctorChecksJSON(report.Checks),
		}

		if err := out.document(doc, false); err != nil {
			return err
		}

		return runErr
	}

	for _, c := range report.Checks {
		fmt.Fprintln(out.stdout, doctorRow(wd, c))
	}

	fmt.Fprintf(out.stderr, "brief doctor: %s\n", doctorSummary(counts))

	return runErr
}

// locateInRepo is runDoctor's own production pre-check: config.LocateInRepo,
// its nearest and shadowed results discarded — runDoctor only ever branches
// on whether wd itself resolves.
func locateInRepo(wd string) error {
	_, _, err := config.LocateInRepo(wd)

	return err
}

// locateInRepoFS is locateInRepo's fsys-backed twin: the walk and the
// git-repository boundary both run against fsys instead of the real
// filesystem, so a withRootFS-backed test can substitute an rwfs.Mem.
func locateInRepoFS(fsys fs.FS, wd string) error {
	abs, err := filepath.Abs(wd)
	if err != nil {
		return fmt.Errorf("resolve config: %w", err)
	}

	boundary := ""
	if root, ok := repo.RootFS(fsys, abs); ok {
		boundary = root
	}

	if _, _, err := config.LocateWithinFS(fsys, abs, boundary); err != nil {
		return fmt.Errorf("resolve config: %w", err)
	}

	return nil
}
