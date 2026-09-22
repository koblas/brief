package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/koblas/brief/internal/setup"
)

// uninstallInvocation is the invocation string every "brief uninstall"
// usage error names as how to fix it.
const uninstallInvocation = "brief uninstall --host claude-code"

// uninstallLong is "brief uninstall"'s help prose.
var uninstallLong = `Removes what "brief init" installed: the ".brief.yaml" config file, and,
by default (--host claude-code), the Claude Code plugin under
".claude/skills/brief/" — every file this binary would have written
(recognized by digest, never by decoding it) is removed; a file whose
bytes were edited locally is kept and reported instead, unless --force.
The feature root and everything under it are never removed, nor is
".claude/" or ".claude/skills/" above the plugin's own directory.
--dry-run prints the same report and removes nothing.

` + jsonFieldsParagraph("host", "dry_run", "created", "modified", "removed", "artifacts")

// uninstallHostFlagUsage is uninstall's own --host flag's usage string —
// see cli.go's hostFlagUsage for the backquoted "name" placeholder
// convention.
const uninstallHostFlagUsage = "the agent host `name` to remove for: claude-code or none\n(default: claude-code)"

// uninstallForceFlagUsage is uninstall's own --force flag's usage string.
const uninstallForceFlagUsage = "remove files edited locally instead of keeping them"

// uninstallDocument is uninstall's --json success document: the common
// header first, then the request's own host and dry_run, every path this
// call removed (absolute, never nil, empty under --dry-run or when nothing
// was installed), created and modified always empty (uninstall never
// writes), then one row per artifact in setup.Result's own order.
type uninstallDocument struct {
	jsonHeader

	Host      string         `json:"host"`
	DryRun    bool           `json:"dry_run"`
	Created   []string       `json:"created"`
	Modified  []string       `json:"modified"`
	Removed   []string       `json:"removed"`
	Artifacts []artifactJSON `json:"artifacts"`
}

// uninstallNextAction renders uninstall's own stderr next-action line,
// minus the "brief uninstall: " prefix: R11's stderr contract. Dry run
// names installLabel unconditionally — planning already completed, but the
// promise is about what a real run would remove, not what this one found.
// Otherwise the discriminator is not which host was requested but what
// Uninstall actually found to remove: any artifact outside KindConfig
// reporting setup.ActionRemoved — a plugin, hook, snippet or agent file —
// means a host integration really was removed, so the line names it
// (installLabel, "removed brief's claude-code install; …", right after
// "brief's" rather than trailing "for claude-code", which read awkwardly).
// A lone KindConfig ActionRemoved — the shape a default-host uninstall
// leaves after an earlier "init --host none" — never claims a host
// install that was never there; it reports "removed brief's config"
// instead. Failing both, any artifact ActionKept with detail "edited
// locally" is counted and named, since --force can remove those; a "not a
// regular file" kept artifact is excluded — --force never removes one
// (planPluginRemoval, planConfigRemoval, planSnippetRemoval) — so counting
// it would make a false promise. Zero artifacts falls back to "nothing
// installed"; a non-empty plan with nothing removed and nothing
// force-removable falls back to plain "nothing removed" — both still
// carry host's own " for <host>" suffix (withHostSuffix), dropped only for
// setup.HostNone.
func uninstallNextAction(host string, dryRun bool, artifacts []setup.Artifact) string {
	if dryRun {
		return "dry run, nothing removed; rerun without --dry-run to remove " + installLabel(host)
	}

	var hostRemoved, configRemoved bool

	var editedKept int

	for _, a := range artifacts {
		switch {
		case a.Action == setup.ActionRemoved && a.Kind == setup.KindConfig:
			configRemoved = true
		case a.Action == setup.ActionRemoved:
			hostRemoved = true
		case a.Action == setup.ActionKept && a.Detail == "edited locally":
			editedKept++
		}
	}

	switch {
	case hostRemoved:
		return fmt.Sprintf("removed %s; the feature root and its contents were left in place", installLabel(host))
	case configRemoved:
		return "removed brief's config; the feature root and its contents were left in place"
	case editedKept > 0:
		return fmt.Sprintf("nothing removed; %d file(s) edited locally were kept; run 'brief uninstall --force' to remove them", editedKept)
	case len(artifacts) == 0:
		return withHostSuffix("nothing installed", host)
	default:
		return withHostSuffix("nothing removed", host)
	}
}

// installLabel names what a real uninstall run would remove for host, used
// by uninstallNextAction's dry-run and host-artifact-removed lines: "brief's
// <host> install" unless host is setup.HostNone, in which case the host
// name is dropped — "brief's install" alone.
func installLabel(host string) string {
	if host == setup.HostNone {
		return "brief's install"
	}

	return fmt.Sprintf("brief's %s install", host)
}

// withHostSuffix appends " for <host>" to base, unless host is
// setup.HostNone, in which case base is returned unchanged.
func withHostSuffix(base, host string) string {
	if host == setup.HostNone {
		return base
	}

	return base + " for " + host
}

// runUninstall implements "brief uninstall [--host <name>] [--dry-run]
// [--force] [--json]"; rest is its positional arguments, flags already
// parsed away and must be empty. host is "" when --host was not given,
// defaulted to setup.HostClaudeCode here: uninstall's own plan for
// claude-code is a superset of none's, so an omitted --host removes
// everything brief installed.
func runUninstall(ctx context.Context, wd string, rest []string, host string, dryRun, force bool, out reporter) error {
	if len(rest) > 0 {
		return out.usageError(fmt.Sprintf("brief uninstall: too many arguments; run '%s'", uninstallInvocation))
	}

	if host == "" {
		host = setup.HostClaudeCode
	}

	srv := setup.NewServer()

	res, err := srv.Uninstall(ctx, wd, setup.UninstallRequest{Host: host, DryRun: dryRun, Force: force})
	if err != nil {
		if errors.Is(err, setup.ErrUnknownHost) {
			return out.usageError(fmt.Sprintf("brief uninstall: unknown host %q; expected one of: %s; run '%s'", host, strings.Join(setup.Hosts(), ", "), uninstallInvocation))
		}

		if errors.Is(err, setup.ErrPartialWrite) {
			return renderPartialWrite(res, err, wd, out)
		}

		return out.refusal(err)
	}

	if out.json {
		doc := uninstallDocument{
			jsonHeader: out.successHeader(),
			Host:       res.Host,
			DryRun:     res.DryRun,
			Created:    res.Created,
			Modified:   res.Modified,
			Removed:    res.Removed,
			Artifacts:  artifactsJSON(res.Artifacts),
		}

		return out.document(doc)
	}

	for _, a := range res.Artifacts {
		fmt.Fprintln(out.stdout, artifactRow(wd, a))
	}

	fmt.Fprintf(out.stderr, "brief uninstall: %s\n", uninstallNextAction(res.Host, res.DryRun, res.Artifacts))

	return nil
}
