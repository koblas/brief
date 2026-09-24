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
	// SeverityWarn marks a check whose subject is degraded or missing in a
	// way that does not itself block "brief check" or "brief start" — an
	// un-inited repository, an ancestor config shadowed, brief on PATH at
	// a different version than the one running, a host-plugin/host-hook/
	// host-skill/host-snippet/host-agents file installed by an older
	// brief release, a hook file missing while the rest of the plugin is
	// installed (doctor cannot tell that apart from --no-hook), a
	// host-agents file missing or not regular, a host-skill file missing
	// while some other integration file is installed (a bound role can
	// never preload a skill that is not there), a roles position unbound
	// or unresolved, or a bound planner or implementer whose resolved
	// agent does not preload the brief-workflow skill (roles and
	// roles-skill are both reported, never enforced).
	SeverityWarn Severity = "WARN"
	// SeverityError marks a check whose subject would make another
	// command refuse or misbehave: an unparseable config, an invalid
	// value, a feature root that does not exist, is not a directory, or
	// is not readable or writable; a host-plugin file missing, not a
	// regular file, or unreadable, or a host-snippet file missing or not a
	// regular file, while some other integration file is installed; a
	// host-skill file present but not a regular file; brief missing from
	// PATH while the integration is installed, since the hook that runs
	// "brief check" can never find it.
	SeverityError Severity = "ERROR"
	// SeveritySkip marks a check that could not run because an earlier
	// check's own subject was missing or invalid — config-values and
	// root-dir when config-parse itself failed, or the whole config
	// family when no ".brief.yaml" exists at all — or because its own
	// subject was never installed at all: host-plugin, host-hook,
	// host-skill, host-snippet and host-agents when no file of their own
	// kind is present anywhere, roles when no config was found or every
	// binding is empty, and roles-skill under those same conditions or
	// when neither planner nor implementer is bound and resolved.
	SeveritySkip Severity = "SKIP"
)

// Check is one row of a Report: ID is doctor's own stable id (config-file,
// config-parse, config-values, config-shadow, root-dir, env-git, env-path,
// host-plugin, host-hook, host-skill, host-snippet, host-agents, roles,
// roles-skill), Severity is this row's urgency, Path is the absolute path
// this row concerns ("" when it names none), Detail is the English
// explanation, and Fix, when non-nil, names the action that would resolve
// it. A SKIP row carries a Fix too when its subject was simply never
// installed (naming the install command); one whose subject could not be
// determined at all (root-dir or roles behind an unparseable config)
// carries a nil Fix, the same as every OK row.
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

// rootFS returns doctor's own production root FS: the "/"-rooted
// namespace config.LocateWithinFS, config.InspectFS and repo.RootFS
// already read through for their own OS adapters (Locate/Inspect, Root).
// Doctor calls only those exported *FS entry points, never fs.Stat/fs.Open
// directly, so — unlike config and repo — it needs no fsName mapping of
// its own: each already applies its own internally.
func rootFS() fs.FS {
	return os.DirFS("/")
}

// Server diagnoses one repository's setup for brief. Its environment
// seams default to the real PATH lookup, the real running binary's own
// path, a debug/buildinfo.ReadFile adapter, os.UserHomeDir, and the
// production root FS (rootFS); a test overrides them with WithLookPath,
// WithExecutable, WithBinaryVersion, WithHomeDir, and the package-private
// seams export_test.go exposes for its own fs.FS-backed tests.
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

// WithHomeDir overrides the function the roles check uses to find the
// current user's home directory while resolving a bare `<name>` role
// binding (Rule 5) against every "*.md" file under `<home>/.claude/agents/`
// whose frontmatter `name:` matches, searched only when the repository's
// own tree has none. It defaults to os.UserHomeDir; a test injects a
// fixed, empty directory so roles never depends on the developer's own
// "~/.claude/agents". A home error, or home returning "", means no home
// agents are ever found — never a refusal.
func WithHomeDir(fn func() (string, error)) Option {
	return func(s *Server) { s.homeDir = fn }
}

// NewServer builds a Server with opts applied over its production
// defaults: exec.LookPath, os.Executable, an adapter over
// debug/buildinfo.ReadFile, devVersion for the running version (a caller
// that never calls WithVersion is, correctly, always reported as running an
// unknown build), os.UserHomeDir, and the production root FS.
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

// userTree returns the agentfile.Tree a bare-name role binding's own Rule 5
// search runs against for the user scope: s.homeTree() when a test
// injected one (export_test.go's WithHomeTree), else agentfile.DirTree
// over s.homeDir() — a home lookup error or an empty result yields the
// zero Tree, so that scope is never searched (WithHomeDir's own contract).
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

// locateInRepo is config.LocateInRepo's own fs.FS-backed twin: the walk and
// the git-repository boundary both run against s.rootFS() rather than the
// production-only root FS config.LocateInRepo and repo.Root hardcode
// internally, so a test can substitute a fstest.MapFS for both. It
// reproduces config.LocateWithin's own "resolve config:" wrap verbatim;
// Diagnose only branches on whether the returned error is nil, so the wrap
// affects nothing observable today, but keeping it means a caller reading
// the error text is never surprised by two adapters disagreeing.
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
// filepath.Abs itself. It reproduces Inspect's own error wrap verbatim —
// an *InvalidConfigError wrapped by config.Inspect (%w, "resolve config:
// %w") errors.AsType still reaches, unaffected by the wrap either way,
// since parseErrorDetail unwraps to it directly; any other failure (a
// config file that cannot be opened) keeps the same "resolve config: <abs>:
// <cause>" text Inspect itself would produce, so config-parse's Detail
// never depends on which adapter ran.
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
// in that fixed order. No
// ".brief.yaml" anywhere reports config-file as WARN (fix "brief init")
// and skips the rest of the config family — an un-inited repository is
// not itself a fault. A found config that fails to decode reports
// config-parse as ERROR and skips config-values and root-dir, since the
// feature directory a bad config might have set is unknown; one that
// decodes reports one ERROR row per invalid value (config.Inspect's own
// violations, never only the first) and runs root-dir against the decoded
// feature directory. config-shadow is always OK: a shadowed ancestor
// config is informational, not a fault.
//
// The install root the six host rows, roles and roles-skill check against
// is config.LocateInRepo's own directory whenever a config was found —
// parseable or not — else wd; a config found above the nearest enclosing
// git repository (walked from wd) is treated as though none existed, the
// same install-root rule init and uninstall apply (R3) — its config family
// then reports noConfigChecks rather than naming that ancestor file. With
// no enclosing git repository anywhere above wd, root-finding keeps its own
// plain ancestor walk, unbounded. The host is always Claude Code, with no
// detection.
// env-path is ERROR, rather than WARN, when brief is missing from PATH and
// the integration is installed (any Plugin(true) ∪ Agents() file present,
// or a CLAUDE.md snippet block found). Diagnose never returns an error for
// any of the above — every fault becomes a Check. It never reads a
// feature's own contents.
func (s *Server) Diagnose(ctx context.Context, wd string) Report {
	_ = ctx

	absWd, err := filepath.Abs(wd)
	if err != nil {
		absWd = wd
	}

	nearest, shadowed, locateErr := s.locateInRepo(absWd)

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

	checks = append(checks, checkEnvGit(s.rootFS(), absWd))

	h, _ := host.Lookup(host.ClaudeCode)
	snippetStates := scanSnippetCandidateStates(root, h)
	filesInstalled := anyIntegrationFilePresent(root, h)
	integrationInstalled := filesInstalled || snippetBlockFound(snippetStates)

	checks = append(checks,
		s.checkEnvPath(integrationInstalled),
		hostPluginCheck(absWd, root, h, filesInstalled),
		hostHookCheck(absWd, root, h, filesInstalled),
		hostSkillCheck(absWd, root, h, filesInstalled),
		hostSnippetCheck(absWd, root, snippetStates, dir, dirKnown),
		hostAgentsCheck(absWd, root, h),
		rolesCheck,
		rolesSkillCheck,
	)

	return Report{Checks: checks}
}
