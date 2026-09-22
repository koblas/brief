package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
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
--no-hook. With no --host, the host is detected: claude-code when the
repository already has ".claude/" or "CLAUDE.md", or the user's own
"~/.claude" exists; otherwise only the config and feature root install,
and stderr says no host was detected. --with-agents additionally installs
three role agents (planner, implementer, reviewer) under the plugin's own
"agents/" directory; it requires the resolved host to be claude-code, and
binds every role to them in ".brief.yaml" only when this same run creates
that file — an existing config is never edited, and stderr instead lists
the "roles:" lines to add by hand for any role still unbound. Writes
".brief.yaml" with every key present but commented out, documenting each
setting in place (live under --with-agents only for "roles:" and its
three children, when this run creates the file), and creates the
configured feature directory. Re-running converges: a valid existing
config, and any plugin or agent file whose bytes are unedited, is kept
as-is, and every artifact already installed reports "unchanged". An
unparseable or invalid existing config refuses, naming the fix; --force
rewrites it from defaults — the bound variant under --with-agents — and
never rewrites an edited plugin or agent file. --dry-run prints the same
report and writes nothing. --print writes nothing either and instead
prints each pending artifact's own path and bytes to stdout, prefixed
"# <path> (create|merge)", for wiring the integration by hand; it cannot
be combined with --dry-run. Every target is checked for writability before
anything is written: an unwritable target refuses, naming it, with the
--print output on stdout so it can still be applied by hand.

` + jsonFieldsParagraph("host", "dry_run", "created", "modified", "artifacts", "roles_to_add")

// initDocument is init's --json success document: the common header first,
// then the request's own host and dry_run, every path this call created or
// modified (absolute, never nil, both empty under --dry-run), then one row
// per artifact in setup.Result's own order — config, feature root — and
// finally roles_to_add, always present, empty unless --with-agents left
// roles unbound in a config this run did not write. Never written for
// --print, which renders initPrintDocument instead.
type initDocument struct {
	jsonHeader

	Host       string         `json:"host"`
	DryRun     bool           `json:"dry_run"`
	Created    []string       `json:"created"`
	Modified   []string       `json:"modified"`
	Artifacts  []artifactJSON `json:"artifacts"`
	RolesToAdd []string       `json:"roles_to_add"`
}

// printArtifactJSON is one initPrintDocument "artifacts" row: path
// (absolute), action ("create" or "merge") and body exactly as
// setup.PrintArtifact carries them.
type printArtifactJSON struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Body   string `json:"body"`
}

// initPrintDocument is "brief init --print --json"'s own success document:
// the common header, then artifacts alone — never host, dry_run, created,
// modified or roles_to_add, none of which --print computes a meaningful
// value for. artifacts is never nil.
type initPrintDocument struct {
	jsonHeader

	Artifacts []printArtifactJSON `json:"artifacts"`
}

// printArtifactsJSON maps artifacts to initPrintDocument's own "artifacts"
// rows, in setup.Result.Print's own order, never nil.
func printArtifactsJSON(artifacts []setup.PrintArtifact) []printArtifactJSON {
	out := make([]printArtifactJSON, 0, len(artifacts))

	for _, a := range artifacts {
		out = append(out, printArtifactJSON{Path: a.Path, Action: string(a.Action), Body: a.Body})
	}

	return out
}

// renderPrint writes R9's --print text-mode body to w: one
// "# <path> (create|merge)" header per artifact, path relative to wd,
// followed by its own body — a trailing newline appended only when the
// body does not already end with one — with one blank line between
// artifacts and none after the last.
func renderPrint(w io.Writer, wd string, artifacts []setup.PrintArtifact) {
	for i, a := range artifacts {
		if i > 0 {
			fmt.Fprintln(w)
		}

		fmt.Fprintf(w, "# %s (%s)\n", displayPath(wd, a.Path), a.Action)
		fmt.Fprint(w, a.Body)

		if !strings.HasSuffix(a.Body, "\n") {
			fmt.Fprintln(w)
		}
	}
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

// unwrittenLine renders R9/R10's own "printed only" or "already installed"
// stderr line for --print, minus the "brief init: " prefix: the
// nothing-pending line when artifacts is empty, else the "printed only"
// line.
func unwrittenLine(artifacts []setup.PrintArtifact) string {
	if len(artifacts) == 0 {
		return "already installed; nothing changed"
	}

	return "printed only, no files changed; apply the output above by hand, or rerun without --print"
}

// runInit implements "brief init [--host <name>] [--no-hook]
// [--with-agents] [--dry-run | --print] [--force] [--json]"; rest is its
// positional arguments, flags already parsed away and must be empty. host
// is "" when --host was not given, passed through unchanged to
// setup.InitRequest.Host — setup.Init treats "" as "detect" (R8) rather
// than defaulting it here. extraSetupOpts threads a test's own
// setup.WithHomeDir override (withSetupOpts) to setup.NewServer.
func runInit(ctx context.Context, wd string, rest []string, host string, noHook, withAgents, dryRun, printOnly, force bool, out reporter, extraSetupOpts ...setup.Option) error {
	if len(rest) > 0 {
		return out.usageError(fmt.Sprintf("brief init: too many arguments; run '%s'", initInvocation))
	}

	if dryRun && printOnly {
		return out.usageError("brief init: --dry-run and --print cannot be combined; run 'brief init --print'")
	}

	srv := setup.NewServer(extraSetupOpts...)

	res, err := srv.Init(ctx, wd, setup.InitRequest{Host: host, NoHook: noHook, WithAgents: withAgents, DryRun: dryRun, Force: force, Print: printOnly})
	if err != nil {
		if errors.Is(err, setup.ErrAgentsNeedHost) {
			return out.usageError(fmt.Sprintf("brief init: --with-agents requires --host claude-code; run '%s --with-agents'", initInvocation))
		}

		if errors.Is(err, setup.ErrUnknownHost) {
			const fix = "run 'brief init --print' to wire it by hand"

			return out.usageErrorWithFix(fmt.Sprintf("brief init: unknown host %q; expected one of: %s; %s", host, strings.Join(setup.Hosts(), ", "), fix), fix)
		}

		if errors.Is(err, setup.ErrUnwritable) {
			return renderUnwritable(res, err, wd, out)
		}

		if errors.Is(err, setup.ErrPartialWrite) {
			return renderPartialWrite(res, err, wd, out)
		}

		return out.refusal(err)
	}

	if out.json {
		if printOnly {
			return out.document(initPrintDocument{jsonHeader: out.successHeader(), Artifacts: printArtifactsJSON(res.Print)})
		}

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

	if printOnly {
		renderPrint(out.stdout, wd, res.Print)
		fmt.Fprintf(out.stderr, "brief init: %s\n", unwrittenLine(res.Print))

		return nil
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

	if !res.DryRun && res.NoHostDetected {
		fmt.Fprintln(out.stderr, "brief init: no agent host detected; run 'brief init --host claude-code' to install integration")

		return nil
	}

	fmt.Fprintf(out.stderr, "brief init: %s\n", initNextAction(res.Host, res.DryRun, res.Artifacts, wd, res.Root))

	return nil
}

// renderUnwritable renders R10's own refusal for setup.ErrUnwritable: text
// mode renders the ordinary refusal line on stderr (out.refusal handles
// *setup.RefusalError generically) and then res.Print's own body on
// stdout, byte for byte what --print would have shown; JSON mode writes
// the standard error document with its fix overridden to point at
// "brief init --print --json" — R10's own JSON contract carries no
// artifacts field at all, unlike the print-only success document above.
func renderUnwritable(res setup.Result, err error, wd string, out reporter) error {
	if !out.json {
		_ = out.refusal(err)
		renderPrint(out.stdout, wd, res.Print)

		return err
	}

	c := classifyRefusal(err)
	problem := c.problem
	command := commandName(out.cmd)

	doc := errorDocument{
		jsonHeader: newJSONHeader(command, ExitCode(err)),
		Error: jsonError{
			Kind:         c.kind,
			Message:      fmt.Sprintf("brief %s: %s", command, c.textLine(wd)),
			Path:         c.jsonPath(wd),
			Line:         c.jsonLine(),
			Problem:      &problem,
			Fix:          "run 'brief init --print --json' and apply the artifacts by hand",
			FilesChanged: filesChangedFor(out.cmd, err),
		},
	}

	_ = writeJSONDocument(out.stdout, doc)

	return err
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
