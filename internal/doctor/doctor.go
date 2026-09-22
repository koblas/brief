package doctor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/config"
)

// Severity is a Check's urgency.
type Severity string

const (
	// SeverityOK marks a check that found nothing wrong.
	SeverityOK Severity = "OK"
	// SeverityWarn marks a check whose subject is degraded or missing in a
	// way that does not itself block "brief check" or "brief start" —
	// an un-inited repository, an ancestor config shadowed, brief on PATH
	// at a different version than the one running.
	SeverityWarn Severity = "WARN"
	// SeverityError marks a check whose subject would make another
	// command refuse or misbehave: an unparseable config, an invalid
	// value, a feature root that does not exist, is not a directory, or
	// is not readable or writable.
	SeverityError Severity = "ERROR"
	// SeveritySkip marks a check that could not run because an earlier
	// check's own subject was missing or invalid — config-values and
	// root-dir when config-parse itself failed, or the whole config
	// family when no ".brief.yaml" exists at all.
	SeveritySkip Severity = "SKIP"
)

// Check is one row of a Report: ID is doctor's own stable id
// (config-file, config-parse, config-values, config-shadow, root-dir,
// env-git, env-path), Severity is this row's urgency, Path is the
// absolute path this row concerns ("" when it names none), Detail is the
// English explanation, and Fix, when non-nil, names the action that would
// resolve Severity ERROR or WARN.
type Check struct {
	ID       string
	Severity Severity
	Path     string
	Detail   string
	Fix      *string
}

// Report is Diagnose's own result: every Check, in the fixed order
// Diagnose builds them.
type Report struct {
	Checks []Check
}

// Counts is Report's own tally, by Severity — the source both a text
// summary line and a --json document's own "counts" member render from,
// so the two can never disagree.
type Counts struct {
	Error int
	Warn  int
	OK    int
	Skip  int
}

// Counts tallies r.Checks by Severity.
func (r Report) Counts() Counts {
	var c Counts

	for _, check := range r.Checks {
		switch check.Severity {
		case SeverityError:
			c.Error++
		case SeverityWarn:
			c.Warn++
		case SeverityOK:
			c.OK++
		case SeveritySkip:
			c.Skip++
		}
	}

	return c
}

// devVersion is the version string versionString (internal/cli) reports
// when the running binary carries no embedded module version — env-path
// never reports OK against a PATH binary at this version, since "the same
// (devel) as another (devel)" proves nothing about whether the two are
// actually the same build.
const devVersion = "(devel)"

// Server diagnoses one repository's setup for brief. Its environment
// seams default to the real PATH lookup, the real running binary's own
// path, and a debug/buildinfo.ReadFile adapter; a test overrides them with
// WithLookPath, WithExecutable and WithBinaryVersion.
type Server struct {
	lookPath      func(string) (string, error)
	executable    func() (string, error)
	binaryVersion func(string) (string, bool)
	version       string
}

// Option configures a Server built by NewServer.
type Option func(*Server)

// WithLookPath overrides the PATH lookup env-path uses to find the
// "brief" binary — production defaults to exec.LookPath.
func WithLookPath(f func(string) (string, error)) Option {
	return func(s *Server) { s.lookPath = f }
}

// WithExecutable overrides the lookup env-path uses for the running
// binary's own path — production defaults to os.Executable.
func WithExecutable(f func() (string, error)) Option {
	return func(s *Server) { s.executable = f }
}

// WithBinaryVersion overrides the reader env-path uses to read a PATH
// binary's own embedded module version — production defaults to an
// adapter over debug/buildinfo.ReadFile, which reads the binary's
// metadata without executing it.
func WithBinaryVersion(f func(string) (string, bool)) Option {
	return func(s *Server) { s.binaryVersion = f }
}

// WithVersion sets the running binary's own version, the value env-path
// compares a different PATH binary's version against — production passes
// the same versionString "brief --version" itself reports.
func WithVersion(v string) Option {
	return func(s *Server) { s.version = v }
}

// NewServer builds a Server with opts applied over its production
// defaults: exec.LookPath, os.Executable, an adapter over
// debug/buildinfo.ReadFile, and devVersion for the running version (a
// caller that never calls WithVersion is, correctly, always reported as
// running an unknown build).
func NewServer(opts ...Option) *Server {
	s := &Server{
		lookPath:      exec.LookPath,
		executable:    os.Executable,
		binaryVersion: readBinaryVersion,
		version:       devVersion,
	}

	for _, o := range opts {
		o(s)
	}

	return s
}

// Diagnose reports wd's setup health: config-file, config-parse,
// config-values, config-shadow, root-dir, env-git and env-path, in that
// fixed order. No ".brief.yaml" anywhere reports config-file as WARN
// (fix "brief init") and skips the rest of the config family — an
// un-inited repository is not itself a fault. A found config that fails
// to decode reports config-parse as ERROR and skips config-values and
// root-dir, since the feature directory a bad config might have set is
// unknown; one that decodes reports one ERROR row per invalid value
// (config.Inspect's own violations, never only the first) and runs
// root-dir against the decoded feature directory. config-shadow is always
// OK: a shadowed ancestor config is informational, not a fault. Diagnose
// never returns an error for any of the above — every fault becomes a
// Check. It never reads a feature's own contents.
func (s *Server) Diagnose(ctx context.Context, wd string) Report {
	_ = ctx

	absWd, err := filepath.Abs(wd)
	if err != nil {
		absWd = wd
	}

	nearest, shadowed, locateErr := config.Locate(absWd)

	var checks []Check

	switch {
	case locateErr != nil || nearest == "":
		checks = append(checks, noConfigChecks(absWd)...)
		checks = append(checks, checkRootDir(absWd, absWd, config.Default().FeatureDirectory))
	default:
		cfg, violations, inspectErr := config.Inspect(nearest)
		if inspectErr != nil {
			checks = append(checks, unparseableConfigChecks(nearest, shadowed, inspectErr)...)
			checks = append(checks, Check{ID: "root-dir", Severity: SeveritySkip, Detail: rootDirUnknownDetail})
		} else {
			checks = append(checks, parseableConfigChecks(nearest, shadowed, violations)...)
			checks = append(checks, checkRootDir(absWd, filepath.Dir(nearest), cfg.FeatureDirectory))
		}
	}

	checks = append(checks, checkEnvGit(absWd), s.checkEnvPath())

	return Report{Checks: checks}
}
