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
const initInvocation = "brief init --host claude-code"

// initLong is "brief init"'s help prose.
var initLong = `Installs brief's own config, feature root and, for --host claude-code, a
Claude Code skills-directory plugin under ".claude/skills/brief/": a
plugin manifest, "/brief:start" and "/brief:finish" skills, and a
PostToolUse hook running "brief check --hook claude-code" — omit it with
--no-hook. --with-agents additionally installs three role agents (planner,
implementer, reviewer) under the plugin's own "agents/" directory; it
requires --host claude-code, and binds every role to them in ".brief.yaml"
only when this same run creates that file — an existing config is never
edited, and stderr instead lists the "roles:" lines to add by hand for
any role still unbound. Writes ".brief.yaml" with every key present but
commented out, documenting each setting in place (live under
--with-agents only for "roles:" and its three children, when this run
creates the file), and creates the configured feature directory.
Re-running converges: a valid existing config, and any plugin or agent
file whose bytes are unedited, is kept as-is, and every artifact already
installed reports "unchanged". An unparseable or invalid existing config
refuses, naming the fix; --force rewrites it from defaults — the bound
variant under --with-agents — and never rewrites an edited plugin or
agent file. --dry-run prints the same report and writes nothing.

` + jsonFieldsParagraph("host", "dry_run", "created", "modified", "artifacts", "roles_to_add")

// initDocument is init's --json success document: the common header first,
// then the request's own host and dry_run, every path this call created or
// modified (absolute, never nil, both empty under --dry-run), then one row
// per artifact in setup.Result's own order — config, feature root — and
// finally roles_to_add, always present, empty unless --with-agents left
// roles unbound in a config this run did not write.
type initDocument struct {
	jsonHeader

	Host       string         `json:"host"`
	DryRun     bool           `json:"dry_run"`
	Created    []string       `json:"created"`
	Modified   []string       `json:"modified"`
	Artifacts  []artifactJSON `json:"artifacts"`
	RolesToAdd []string       `json:"roles_to_add"`
}

// initNextAction renders init's own stderr next-action line, minus the
// "brief init: " prefix: dryRun's own line when set; else, with nothing
// ActionCreated or ActionMerged, "already installed; nothing changed"; else, for
// host != setup.HostClaudeCode, "installed config and feature root; run
// 'brief new feature <name>'"; else "installed for claude-code[ in
// <rel>]; start Claude Code in <dir> (or run /reload-plugins in a session
// already <here/there>), then 'brief new feature <name>'" — Claude Code
// loads a project skills-directory plugin only from the session's own
// working directory, no walk-up, so the line names root whenever it
// differs from wd (rel = displayPath(wd, root), "this directory"/"here"
// when root == wd, else rel itself and "there").
func initNextAction(host string, dryRun bool, artifacts []setup.Artifact, wd, root string) string {
	if dryRun {
		return "dry run, no files changed; rerun without --dry-run to apply"
	}

	var changed bool
	for _, a := range artifacts {
		if a.Action == setup.ActionCreated || a.Action == setup.ActionMerged {
			changed = true

			break
		}
	}

	if !changed {
		return "already installed; nothing changed"
	}

	if host != setup.HostClaudeCode {
		return "installed config and feature root; run 'brief new feature <name>'"
	}

	if root == wd {
		return "installed for claude-code; start Claude Code in this directory (or run /reload-plugins in a session already here), then 'brief new feature <name>'"
	}

	rel := displayPath(wd, root)

	return fmt.Sprintf("installed for claude-code in %s; start Claude Code in %s (or run /reload-plugins in a session already there), then 'brief new feature <name>'", rel, rel)
}

// runInit implements "brief init [--host <name>] [--no-hook]
// [--with-agents] [--dry-run] [--force] [--json]"; rest is its positional
// arguments, flags already parsed away and must be empty. host is "" when
// --host was not given, defaulted to setup.HostNone here — S09 replaces
// this default with host detection; --with-agents is refused
// (setup.ErrAgentsNeedHost) against whatever host actually resolves to, so
// a bare "init --with-agents" is a usage error today and may not be once
// S09 lands.
func runInit(ctx context.Context, wd string, rest []string, host string, noHook, withAgents, dryRun, force bool, out reporter) error {
	if len(rest) > 0 {
		return out.usageError(fmt.Sprintf("brief init: too many arguments; run '%s'", initInvocation))
	}

	if host == "" {
		host = setup.HostNone
	}

	srv := setup.NewServer()

	res, err := srv.Init(ctx, wd, setup.InitRequest{Host: host, NoHook: noHook, WithAgents: withAgents, DryRun: dryRun, Force: force})
	if err != nil {
		if errors.Is(err, setup.ErrAgentsNeedHost) {
			return out.usageError(fmt.Sprintf("brief init: --with-agents requires --host claude-code; run '%s --with-agents'", initInvocation))
		}

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
			Artifacts:  artifactsJSON(res.Artifacts),
			RolesToAdd: res.RolesToAdd,
		}

		return out.document(doc)
	}

	for _, a := range res.Artifacts {
		fmt.Fprintln(out.stdout, artifactRow(wd, a))
	}

	if len(res.RolesToAdd) > 0 {
		fmt.Fprintf(out.stderr, "brief init: %s was not edited; to bind brief's agents, add these lines to it:\n", displayPath(wd, configArtifactPath(res.Artifacts)))

		for _, line := range res.RolesToAdd {
			fmt.Fprintln(out.stderr, line)
		}
	}

	fmt.Fprintf(out.stderr, "brief init: %s\n", initNextAction(res.Host, res.DryRun, res.Artifacts, wd, res.Root))

	return nil
}

// configArtifactPath returns artifacts' own setup.KindConfig entry's Path
// — Init's own Result always plans the config file first — or "" were it
// somehow absent.
func configArtifactPath(artifacts []setup.Artifact) string {
	for _, a := range artifacts {
		if a.Kind == setup.KindConfig {
			return a.Path
		}
	}

	return ""
}
