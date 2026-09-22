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

// uninstallHostFlagUsage is uninstall's own --host flag's usage string.
const uninstallHostFlagUsage = "the agent host to remove for (claude-code or `none`)"

// uninstallForceFlagUsage is uninstall's own --force flag's usage string.
const uninstallForceFlagUsage = "remove a config file edited locally instead of keeping it"

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
// minus the "brief uninstall: " prefix: R11's stderr contract, first match
// wins — dry run, then "nothing installed" for zero artifacts, then
// "removed" when at least one artifact reports setup.ActionRemoved, else
// "nothing removed" naming --force — with " for <host>" appended unless
// host is setup.HostNone, since uninstall's default host is
// setup.HostClaudeCode and a run scoped to "none" removes only the config.
func uninstallNextAction(host string, dryRun bool, artifacts []setup.Artifact) string {
	base := uninstallBaseNextAction(dryRun, artifacts)
	if host == setup.HostNone {
		return base
	}

	return base + " for " + host
}

// uninstallBaseNextAction renders uninstallNextAction's own text before the
// " for <host>" suffix is considered.
func uninstallBaseNextAction(dryRun bool, artifacts []setup.Artifact) string {
	if dryRun {
		return "dry run, no files changed; rerun without --dry-run to apply"
	}

	if len(artifacts) == 0 {
		return "nothing installed"
	}

	for _, a := range artifacts {
		if a.Action == setup.ActionRemoved {
			return "removed brief's install; the feature root and its contents were left in place"
		}
	}

	return "nothing removed; run 'brief uninstall --force' to remove edited files"
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
