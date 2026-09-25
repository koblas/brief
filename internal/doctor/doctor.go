package doctor

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/host"
	"github.com/koblas/brief/internal/platform/repo"
)

// Severity is a Check's urgency.
type Severity string

const (
	// SeverityOK marks a check that found nothing wrong.
	SeverityOK Severity = "OK"
	// SeverityWarn marks a check whose subject is degraded but does not
	// itself block another command.
	SeverityWarn Severity = "WARN"
	// SeverityError marks a check whose subject would make another
	// command refuse or misbehave.
	SeverityError Severity = "ERROR"
	// SeveritySkip marks a check that could not run because an earlier
	// check's own subject was missing or invalid, or its own subject was
	// never installed at all.
	SeveritySkip Severity = "SKIP"
)

// Check is one row of a Report: ID is doctor's own stable id (config-file,
// config-parse, config-values, config-shadow, root-dir, env-git, env-path,
// host-plugin, host-hook, host-skill, host-snippet, host-agents, roles,
// roles-skill), Severity is this row's urgency, Path is the absolute path
// this row concerns ("" when it names none), Detail is the English
// explanation, and Fix, when non-nil, names the action that would resolve
// it.
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

// Counts is Report's own tally, by Severity.
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

// devVersion is the version string reported when the running binary
// carries no embedded module version — env-path never reports OK against
// a PATH binary at this version, since "the same (devel) as another
// (devel)" proves nothing about whether the two are the same build.
const devVersion = "(devel)"

// rootFS returns doctor's own production root FS: os.DirFS("/").
func rootFS() fs.FS {
	return os.DirFS("/")
}

// Server diagnoses one repository's setup for brief. Its environment
// seams default to the real PATH lookup, the running binary's own path, a
// debug/buildinfo.ReadFile adapter, os.UserHomeDir, and the production
// root FS; a test overrides them with the WithX options below.
type Server struct {
	lookPath      func(string) (string, error)
	executable    func() (string, error)
	binaryVersion func(string) (string, bool)
	version       string
	homeDir       func() (string, error)
	rootFS        func() fs.FS
	homeTree      func() agentfile.Tree
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
// binary's embedded module version — production defaults to an adapter
// over debug/buildinfo.ReadFile.
func WithBinaryVersion(f func(string) (string, bool)) Option {
	return func(s *Server) { s.binaryVersion = f }
}

// WithVersion sets the running binary's own version, the value env-path
// compares a different PATH binary's version against.
func WithVersion(v string) Option {
	return func(s *Server) { s.version = v }
}

// WithHomeDir overrides the function the roles check uses to find the
// current user's home directory while resolving a bare `<name>` role
// binding. It defaults to os.UserHomeDir; a home error, or home returning
// "", means no home agents are ever found — never a refusal.
func WithHomeDir(fn func() (string, error)) Option {
	return func(s *Server) { s.homeDir = fn }
}

// WithRootFS overrides the production root FS (os.DirFS("/")) every probe
// in this package reads through, so a test can substitute an in-memory
// fs.FS.
func WithRootFS(fsys fs.FS) Option {
	return func(s *Server) { s.rootFS = func() fs.FS { return fsys } }
}

// WithHomeTree overrides the agentfile.Tree a bare-name role binding's
// user-scope search runs against, bypassing WithHomeDir and
// agentfile.DirTree entirely.
func WithHomeTree(fn func() agentfile.Tree) Option {
	return func(s *Server) { s.homeTree = fn }
}

// NewServer builds a Server with opts applied over its production
// defaults: exec.LookPath, os.Executable, an adapter over
// debug/buildinfo.ReadFile, devVersion for the running version,
// os.UserHomeDir, and the production root FS.
func NewServer(opts ...Option) *Server {
	s := &Server{
		lookPath:      exec.LookPath,
		executable:    os.Executable,
		binaryVersion: readBinaryVersion,
		version:       devVersion,
		homeDir:       os.UserHomeDir,
		rootFS:        rootFS,
	}

	for _, o := range opts {
		o(s)
	}

	return s
}

// userTree returns the agentfile.Tree a bare-name role binding's own
// user-scope search runs against: s.homeTree() when a test injected one,
// else agentfile.DirTree over s.homeDir().
func (s *Server) userTree() agentfile.Tree {
	if s.homeTree != nil {
		return s.homeTree()
	}

	home, err := s.homeDir()
	if err != nil {
		home = ""
	}

	return agentfile.DirTree(home)
}

// locateInRepo is config.LocateInRepo's own fs.FS-backed twin: the walk
// and the git-repository boundary both run against s.rootFS(), so a test
// can substitute a fstest.MapFS for both.
func (s *Server) locateInRepo(absWd string) (string, []string, error) {
	fsys := s.rootFS()

	boundary := ""
	if root, ok := repo.RootFS(fsys, absWd); ok {
		boundary = root
	}

	nearest, shadowed, err := config.LocateWithinFS(fsys, absWd, boundary)
	if err != nil {
		return "", nil, fmt.Errorf("resolve config: %w", err)
	}

	return nearest, shadowed, nil
}

// inspect is config.Inspect's own fs.FS-backed twin, run against
// s.rootFS(): abs is already absolute, so unlike Inspect this never calls
// filepath.Abs itself.
func (s *Server) inspect(abs string) (config.Config, []*config.ValueError, error) {
	cfg, violations, err := config.InspectFS(s.rootFS(), abs)
	if err != nil {
		if invalidCfg, ok := errors.AsType[*config.InvalidConfigError](err); ok {
			return config.Config{}, nil, fmt.Errorf("resolve config: %w", invalidCfg)
		}

		return config.Config{}, nil, fmt.Errorf("resolve config: %s: %w", abs, err)
	}

	return cfg, violations, nil
}

// Diagnose reports wd's setup health: config-file, config-parse,
// config-values, config-shadow, root-dir, env-git, env-path, host-plugin,
// host-hook, host-skill, host-snippet, host-agents, roles and roles-skill,
// in that fixed order. It never returns an error — every fault becomes a
// Check — and never reads a feature's own contents.
func (s *Server) Diagnose(ctx context.Context, wd string) Report {
	_ = ctx

	absWd, err := filepath.Abs(wd)
	if err != nil {
		absWd = wd
	}

	fsys := s.rootFS()

	nearest, shadowed, locateErr := s.locateInRepo(absWd)

	// The install root the host rows, roles and roles-skill check against
	// is the nearest config's own directory when one was found, else wd.
	root := absWd
	if locateErr == nil && nearest != "" {
		root = filepath.Dir(nearest)
	}

	var (
		checks          []Check
		dir             string
		dirKnown        bool
		rolesCheck      Check
		rolesSkillCheck Check
	)

	switch {
	case locateErr != nil || nearest == "":
		checks = append(checks, noConfigChecks(absWd)...)
		checks = append(checks, checkRootDir(absWd, absWd, config.Default().FeatureDirectory))
		dir, dirKnown = config.Default().FeatureDirectory, true
		rolesCheck = rolesCheckNoConfig()
		rolesSkillCheck = rolesSkillCheckNoConfig()
	default:
		cfg, violations, inspectErr := s.inspect(nearest)
		if inspectErr != nil {
			checks = append(checks, unparseableConfigChecks(nearest, shadowed, inspectErr)...)
			checks = append(checks, Check{ID: "root-dir", Severity: SeveritySkip, Detail: rootDirUnknownDetail})
			rolesCheck = rolesCheckUnparseable(nearest)
			rolesSkillCheck = rolesSkillCheckUnparseable(nearest)
		} else {
			checks = append(checks, parseableConfigChecks(nearest, shadowed, violations)...)
			checks = append(checks, checkRootDir(absWd, filepath.Dir(nearest), cfg.FeatureDirectory))
			dir, dirKnown = cfg.FeatureDirectory, true
			rolesCheck = s.rolesCheck(root, nearest, [3]roleBinding{
				{name: "planner", value: cfg.Roles.Planner},
				{name: "implementer", value: cfg.Roles.Implementer},
				{name: "reviewer", value: cfg.Roles.Reviewer},
			})
			rolesSkillCheck = s.rolesSkillCheck(root, nearest,
				roleBinding{name: "planner", value: cfg.Roles.Planner},
				roleBinding{name: "implementer", value: cfg.Roles.Implementer},
			)
		}
	}

	checks = append(checks, checkEnvGit(fsys, absWd))

	h, _ := host.Lookup(host.ClaudeCode)
	snippetStates := scanSnippetCandidateStates(fsys, root, h)
	filesInstalled := anyIntegrationFilePresent(fsys, root, h)
	integrationInstalled := filesInstalled || snippetBlockFound(snippetStates)

	checks = append(checks,
		s.checkEnvPath(integrationInstalled),
		hostPluginCheck(fsys, absWd, root, h, filesInstalled),
		hostHookCheck(fsys, absWd, root, h, filesInstalled),
		hostSkillCheck(fsys, absWd, root, h, filesInstalled),
		hostSnippetCheck(fsys, absWd, root, snippetStates, dir, dirKnown),
		hostAgentsCheck(fsys, absWd, root, h),
		rolesCheck,
		rolesSkillCheck,
	)

	return Report{Checks: checks}
}
