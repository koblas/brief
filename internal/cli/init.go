package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/koblas/brief/internal/setup"
)

// initInvocation is the invocation string every "brief init" usage error
// names as how to fix it.
const initInvocation = "brief init --host none"

// initLong is "brief init"'s help prose.
var initLong = `Installs brief's own config and feature root: writes ".brief.yaml" with
every key present but commented out, documenting each setting in place,
and creates the configured feature directory. Re-running converges: a
valid existing config is kept, and every artifact already installed
reports "unchanged". An unparseable or invalid existing config refuses,
naming the fix; --force rewrites it from defaults instead. --dry-run
prints the same report and writes nothing.

` + jsonFieldsParagraph("host", "dry_run", "created", "modified", "artifacts")

// initArtifactJSON is one initDocument "artifacts" row: kind, path, action
// and detail exactly as setup.Artifact carries them, path always absolute,
// detail null when empty.
type initArtifactJSON struct {
	Kind   string  `json:"kind"`
	Path   string  `json:"path"`
	Action string  `json:"action"`
	Detail *string `json:"detail"`
}

// initDocument is init's --json success document: the common header first,
// then the request's own host and dry_run, every path this call created or
// modified (absolute, never nil, both empty under --dry-run), then one row
// per artifact in setup.Result's own order — config, feature root.
type initDocument struct {
	jsonHeader

	Host      string             `json:"host"`
	DryRun    bool               `json:"dry_run"`
	Created   []string           `json:"created"`
	Modified  []string           `json:"modified"`
	Artifacts []initArtifactJSON `json:"artifacts"`
}

// initArtifactsJSON maps artifacts to initDocument's own "artifacts" rows.
func initArtifactsJSON(artifacts []setup.Artifact) []initArtifactJSON {
	out := make([]initArtifactJSON, 0, len(artifacts))

	for _, a := range artifacts {
		var detail *string
		if a.Detail != "" {
			d := a.Detail
			detail = &d
		}

		out = append(out, initArtifactJSON{Kind: string(a.Kind), Path: a.Path, Action: string(a.Action), Detail: detail})
	}

	return out
}

// initRow renders one artifact as R11's text-mode line, minus the trailing
// newline: "<action> <relative path>[/][ (<detail>)]" — a trailing "/" on
// the feature root's own row, never on the config file's.
func initRow(wd string, a setup.Artifact) string {
	path := displayPath(wd, a.Path)
	if a.Kind == setup.KindFeatureRoot {
		path += "/"
	}

	line := fmt.Sprintf("%s %s", a.Action, path)
	if a.Detail != "" {
		line += fmt.Sprintf(" (%s)", a.Detail)
	}

	return line
}

// initNextAction renders init's own stderr next-action line, minus the
// "brief init: " prefix: dryRun's own line when set, else "installed
// config and feature root; run 'brief new feature <name>'" when at least
// one artifact was ActionCreated, else "already installed; nothing
// changed".
func initNextAction(dryRun bool, artifacts []setup.Artifact) string {
	if dryRun {
		return "dry run, no files changed; rerun without --dry-run to apply"
	}

	for _, a := range artifacts {
		if a.Action == setup.ActionCreated {
			return "installed config and feature root; run 'brief new feature <name>'"
		}
	}

	return "already installed; nothing changed"
}

// runInit implements "brief init [--host <name>] [--dry-run] [--force]
// [--json]"; rest is its positional arguments, flags already parsed away
// and must be empty. host is "" when --host was not given, defaulted to
// setup.HostNone here — S09 replaces this default with host detection.
func runInit(ctx context.Context, wd string, rest []string, host string, dryRun, force bool, out reporter) error {
	if len(rest) > 0 {
		return out.usageError(fmt.Sprintf("brief init: too many arguments; run '%s'", initInvocation))
	}

	if host == "" {
		host = setup.HostNone
	}

	srv := setup.NewServer()

	res, err := srv.Init(ctx, wd, setup.InitRequest{Host: host, DryRun: dryRun, Force: force})
	if err != nil {
		if errors.Is(err, setup.ErrUnknownHost) {
			return out.usageError(fmt.Sprintf("brief init: unknown host %q; expected one of: %s; run '%s'", host, strings.Join(setup.Hosts(), ", "), initInvocation))
		}

		return out.refusal(err)
	}

	if out.json {
		doc := initDocument{
			jsonHeader: out.successHeader(),
			Host:       res.Host,
			DryRun:     res.DryRun,
			Created:    res.Created,
			Modified:   res.Modified,
			Artifacts:  initArtifactsJSON(res.Artifacts),
		}

		return out.document(doc)
	}

	for _, a := range res.Artifacts {
		fmt.Fprintln(out.stdout, initRow(wd, a))
	}

	fmt.Fprintf(out.stderr, "brief init: %s\n", initNextAction(res.DryRun, res.Artifacts))

	return nil
}
