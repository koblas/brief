package setup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/atomicfile"
	"github.com/koblas/brief/internal/platform/config"
)

// configFileName is the config file Init writes and looks for — the same
// name internal/platform/config resolves.
const configFileName = ".brief.yaml"

// HostNone is the one host InitRequest.Host accepts in this release: no
// agent-host integration is installed, only the config and the feature
// root.
const HostNone = "none"

// Hosts returns every host InitRequest.Host accepts, in the order a usage
// error's "expected one of:" clause lists them.
func Hosts() []string {
	return []string{HostNone}
}

// Kind names what an Artifact reports: the config file, or the feature
// root directory.
type Kind string

const (
	// KindConfig is the ".brief.yaml" config file.
	KindConfig Kind = "config"
	// KindFeatureRoot is the configured feature directory.
	KindFeatureRoot Kind = "feature-root"
)

// Action names what Init did, or would do, to one Artifact.
type Action string

const (
	// ActionCreated marks an artifact that did not exist and was written,
	// or, for the config file under --force, one rewritten from defaults.
	ActionCreated Action = "created"
	// ActionUnchanged marks an artifact whose bytes, or whose existence as
	// a directory, already matched what Init would have written.
	ActionUnchanged Action = "unchanged"
	// ActionKept marks an existing, valid config file left as it was
	// because its bytes differ from Init's own render — a repository
	// owner's local edit — or, for Uninstall, any artifact left on disk
	// rather than removed.
	ActionKept Action = "kept"
	// ActionRemoved marks an artifact Uninstall deleted.
	ActionRemoved Action = "removed"
)

// Server plans and applies brief's own install write path. It carries no
// dependencies today; NewServer's functional-options shape is kept ready
// for the seams later scenarios add (host artifacts, detection).
type Server struct{}

// Option configures a Server built by NewServer.
type Option func(*Server)

// NewServer returns a Server ready to call Init on.
func NewServer(opts ...Option) *Server {
	s := &Server{}

	for _, o := range opts {
		o(s)
	}

	return s
}

// InitRequest is Init's own input: Host selects the agent-host integration
// (Hosts, HostNone in this release), DryRun computes the same plan without
// writing anything, and Force rewrites an existing config from defaults
// rather than keeping or refusing it.
type InitRequest struct {
	Host   string
	DryRun bool
	Force  bool
}

// Artifact is one thing Init installs or found already installed: Kind and
// Path identify it, Action reports what happened, and Detail carries an
// optional parenthetical ("edited locally", "rewritten from defaults"),
// empty when there is nothing to add.
type Artifact struct {
	Kind   Kind
	Path   string
	Action Action
	Detail string
}

// Result is what Init and Uninstall both return: Host and DryRun echo the
// request, Artifacts lists what was found and what happened to it — for
// Init, the config file then the feature root, the fixed order R11's
// stdout rows render in; for Uninstall, any host artifacts (added by a
// later scenario) then the config file last, so a partial uninstall never
// removes the repository's opt-in marker before everything else. Created
// and Modified name every path Init actually wrote, absolute, in the order
// it wrote them; Removed names every path Uninstall actually deleted, in
// removal order. No slice is ever nil; all three are empty under DryRun.
type Result struct {
	Host      string
	DryRun    bool
	Artifacts []Artifact
	Created   []string
	Modified  []string
	Removed   []string
}

// Init plans then, unless req.DryRun, applies brief's own install: the
// config file and the feature root the kept or freshly written config
// names. Every refusal — an unknown host, an invalid existing config, a
// feature root that exists as something other than a directory — is
// decided during planning, before either artifact is touched; DryRun
// therefore returns exactly the plan a real run would apply, including any
// refusal, and writes nothing either way.
//
// Applying writes the feature root first and the config file last, so the
// config file — the repository's opt-in marker — never appears before
// everything else has landed. A failure writing the config file after the
// feature root was created is wrapped in ErrPartialWrite; a failure before
// anything was written is returned as-is.
func (s *Server) Init(_ context.Context, wd string, req InitRequest) (Result, error) {
	if !validHost(req.Host) {
		return Result{}, fmt.Errorf("%q: %w", req.Host, ErrUnknownHost)
	}

	nearest, _, err := config.Locate(wd)
	if err != nil {
		return Result{}, err
	}

	root := wd
	if nearest != "" {
		root = filepath.Dir(nearest)
	}

	configArt, cfg, err := planConfig(nearest, root, req.Force)
	if err != nil {
		return Result{}, err
	}

	featureRoot := filepath.Join(root, cfg.FeatureDirectory)

	featureArt, err := planFeatureRoot(featureRoot)
	if err != nil {
		return Result{}, err
	}

	res := Result{
		Host:      req.Host,
		DryRun:    req.DryRun,
		Artifacts: []Artifact{configArt, featureArt},
		Created:   []string{},
		Modified:  []string{},
		Removed:   []string{},
	}

	if req.DryRun {
		return res, nil
	}

	return apply(res, featureArt, configArt)
}

// apply writes featureArt then configArt, in that order, into res's own
// Created list, when each reports ActionCreated. A config-file write
// failure is wrapped in ErrPartialWrite iff the feature root was already
// written in this same call.
func apply(res Result, featureArt, configArt Artifact) (Result, error) {
	var wroteSomething bool

	if featureArt.Action == ActionCreated {
		if err := os.MkdirAll(featureArt.Path, 0o755); err != nil {
			return Result{}, fmt.Errorf("setup: create %s: %w", featureArt.Path, err)
		}

		res.Created = append(res.Created, featureArt.Path)
		wroteSomething = true
	}

	if configArt.Action == ActionCreated {
		if err := writeConfigFile(configArt.Path, artifact.ConfigFile()); err != nil {
			if wroteSomething {
				return Result{}, markPartial(err)
			}

			return Result{}, err
		}

		res.Created = append(res.Created, configArt.Path)
	}

	return res, nil
}

// planConfig decides the config file's own Artifact and, when it can be
// trusted, the Config governing the feature root: a fresh path (nothing
// found) always reports ActionCreated against config.Default(). Under
// --force, only the bytes-equal check ever runs — a config invalid,
// unparseable, or unreadable for any other reason (a directory at that
// path, say) still reports ActionCreated, detail "rewritten from
// defaults", since --force never refuses on the old file's content; the
// write attempt itself, not this planning step, is where an unreadable
// path's own failure surfaces. Without --force, an existing file whose
// bytes already equal artifact.ConfigFile() (artifact.Recognize's
// OriginCurrent) is ActionUnchanged; otherwise it is decoded with
// config.Inspect — a decode failure or any R1 violation refuses, naming
// the first one in Config's own field order — and a config with none is
// ActionKept, detail "edited locally", governing the feature root with its
// own decoded values.
func planConfig(nearest, root string, force bool) (Artifact, config.Config, error) {
	if nearest == "" {
		path := filepath.Join(root, configFileName)

		return Artifact{Kind: KindConfig, Path: path, Action: ActionCreated}, config.Default(), nil
	}

	if force {
		if configFileCurrent(nearest) {
			return Artifact{Kind: KindConfig, Path: nearest, Action: ActionUnchanged}, config.Default(), nil
		}

		return Artifact{Kind: KindConfig, Path: nearest, Action: ActionCreated, Detail: "rewritten from defaults"}, config.Default(), nil
	}

	existing, err := os.ReadFile(nearest)
	if err != nil {
		return Artifact{}, config.Config{}, fmt.Errorf("setup: read %s: %w", nearest, err)
	}

	if artifact.Recognize(artifact.KindConfig, existing) == artifact.OriginCurrent {
		return Artifact{Kind: KindConfig, Path: nearest, Action: ActionUnchanged}, config.Default(), nil
	}

	cfg, violations, inspectErr := config.Inspect(nearest)
	if inspectErr != nil {
		return Artifact{}, config.Config{}, configRefusal(nearest, inspectErr)
	}

	if len(violations) > 0 {
		return Artifact{}, config.Config{}, configRefusal(nearest, &config.InvalidConfigError{Path: nearest, Err: violations[0]})
	}

	return Artifact{Kind: KindConfig, Path: nearest, Action: ActionKept, Detail: "edited locally"}, cfg, nil
}

// configFileCurrent reports whether path's own bytes already equal
// artifact.ConfigFile(), false for any read failure — a directory at path,
// a permission error, or a genuinely different render all take the same
// "not current" branch here, since --force's own decision only ever needs
// to distinguish "already correct" from "needs (re)writing".
func configFileCurrent(path string) bool {
	existing, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	return artifact.Recognize(artifact.KindConfig, existing) == artifact.OriginCurrent
}

// configRefusal builds the *RefusalError an invalid or unparseable existing
// config reports: Problem recovers the underlying *config.ValueError or
// decode failure's own text via errors.AsType[*config.InvalidConfigError],
// and Fix always points at --force, the only way init rewrites an existing
// file. err is wrapped unchanged, so errors.Is(result, config.ErrInvalidConfig)
// still holds.
func configRefusal(path string, err error) error {
	problem := err.Error()
	if invalidCfg, ok := errors.AsType[*config.InvalidConfigError](err); ok {
		problem = flattenOneLine(invalidCfg.Err.Error())
	}

	return &RefusalError{
		Path:    path,
		Problem: problem,
		Fix:     "fix it, or run 'brief init --force' to rewrite it from defaults",
		Err:     err,
	}
}

// planFeatureRoot decides the feature root's own Artifact: ActionUnchanged
// when path already exists as a directory, ActionCreated when nothing
// exists there yet, or a *RefusalError wrapping ErrNotADirectory when path
// exists as something else.
func planFeatureRoot(path string) (Artifact, error) {
	info, err := os.Stat(path)

	switch {
	case err == nil && info.IsDir():
		return Artifact{Kind: KindFeatureRoot, Path: path, Action: ActionUnchanged}, nil
	case err == nil:
		return Artifact{}, &RefusalError{
			Path:    path,
			Problem: "exists and is not a directory",
			Fix:     "remove it, or set feature-directory in .brief.yaml to a different path",
			Err:     ErrNotADirectory,
		}
	case os.IsNotExist(err):
		return Artifact{Kind: KindFeatureRoot, Path: path, Action: ActionCreated}, nil
	default:
		return Artifact{}, fmt.Errorf("setup: stat %s: %w", path, err)
	}
}

// writeConfigFile atomically replaces name — an absolute path — with body,
// through internal/platform/atomicfile so a reader never observes a
// truncated or half-renamed config file.
func writeConfigFile(name string, body []byte) error {
	dir := filepath.Dir(name)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("setup: open %s: %w", dir, err)
	}
	defer func() { _ = root.Close() }()

	w, err := atomicfile.Create(root, filepath.Base(name), 0o644)
	if err != nil {
		return fmt.Errorf("setup: write %s: %w", name, err)
	}

	if _, err := w.Write(body); err != nil {
		_ = w.Close()

		return fmt.Errorf("setup: write %s: %w", name, err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("setup: write %s: %w", name, err)
	}

	return nil
}

// validHost reports whether host appears in Hosts().
func validHost(host string) bool {
	return slices.Contains(Hosts(), host)
}

// flattenOneLine collapses s to a single line: embedded newlines and runs
// of whitespace become one space each — yaml.v3 reports an unknown-key
// failure as "yaml: unmarshal errors:\n  line N: …", and a RefusalError's
// Problem is always one line.
func flattenOneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
