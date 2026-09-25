package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/koblas/brief/internal/setup"
)

// initInvocation is the invocation string every "brief init" usage error
// names as how to fix it.
const initInvocation = "brief init --host claude-code"

// initLong is "brief init"'s help prose.
var initLong = `Installs brief's own config, feature root and, for --host claude-code, a
Claude Code skills-directory plugin under ".claude/skills/brief/": a plugin
manifest, "/brief:start" and "/brief:finish" skills, and a PostToolUse hook
running "brief check --hook claude-code" — omit it with --no-hook. With no
--host, the host is detected: claude-code when the repository already has
".claude/" or "CLAUDE.md", or the user's own "~/.claude" exists; otherwise
only the config and feature root install, and stderr says no host was
detected. --with-agents additionally installs three role agents (planner,
implementer, reviewer) under the plugin's own "agents/" directory; it
requires the resolved host to be claude-code, and binds every role to them
in ".brief.yaml" only when this same run creates that file — an existing
config is never edited, and stderr instead lists the "roles:" lines to add
by hand for any role still unbound. Every claude-code install also writes a
"brief-workflow" skill under ".claude/skills/brief-workflow/", which agents
preload by listing it in their frontmatter "skills:". init never edits an
agent file of yours by default; stderr instead lists each planner or
implementer bound in ".brief.yaml" whose agent lacks it. --edit-agents adds
it to those agents' "skills:" lists, for agent files under ".claude/agents/"
only; one under "~/.claude" is always left for you to edit. Writes
".brief.yaml" with every key present but commented out, documenting each
setting in place (live under --with-agents only for "roles:" and its three
children, when this run creates the file), and creates the configured
feature directory. Re-running converges: a valid existing config, and any
plugin or agent file whose bytes are unedited, is kept as-is, and every
artifact already installed reports "unchanged". An unparseable or invalid
existing config refuses, naming the fix; --force rewrites it from defaults —
the bound variant under --with-agents — and never rewrites an edited plugin
or agent file. --dry-run prints the same report and writes nothing. --print
writes nothing either and instead prints each pending artifact's own path
and bytes to stdout, prefixed "# <path> (create|merge)", for wiring the
integration by hand; it cannot be combined with --dry-run. Every target is
checked for writability before anything is written: an unwritable target
refuses, naming it, with the --print output on stdout so it can still be
applied by hand.

` + jsonFieldsParagraph("host", "detected_by", "dry_run", "created", "modified", "artifacts", "roles_to_add", "agents_missing_skill") + "\n" +
	wrapWords("With --print --json, the document carries only `artifacts`, each "+
		"{`path`, `action` (create|merge), `body`}.", jsonParagraphWidth)

// initDocument is init's --json success document: the common header, the
// request's host, detected_by (null unless host was detected rather than
// given) and dry_run, every path created or modified (never nil, both
// empty under --dry-run), one row per artifact, roles_to_add (empty
// unless --with-agents left roles unbound in a config this run did not
// write), and agents_missing_skill (empty unless the resolved host is
// claude-code and a bound planner or implementer lacks the skill). Never
// written for --print, which renders initPrintDocument instead.
type initDocument struct {
	jsonHeader

	Host               string             `json:"host"`
	DetectedBy         *string            `json:"detected_by"`
	DryRun             bool               `json:"dry_run"`
	Created            []string           `json:"created"`
	Modified           []string           `json:"modified"`
	Artifacts          []artifactJSON     `json:"artifacts"`
	RolesToAdd         []string           `json:"roles_to_add"`
	AgentsMissingSkill []missingSkillJSON `json:"agents_missing_skill"`
}

// missingSkillJSON is one initDocument "agents_missing_skill" row: role,
// agent (the binding's own configured value), path (absolute) and scope
// ("project"|"user"), in that key order, exactly as setup.MissingSkillAgent
// carries them — agentfile.Scope's own string values are already "project"
// and "user" (agentfile's own doc.go), so Scope is cast, never re-derived.
type missingSkillJSON struct {
	Role  string `json:"role"`
	Agent string `json:"agent"`
	Path  string `json:"path"`
	Scope string `json:"scope"`
}

// missingSkillJSONRows maps agents to initDocument's own
// "agents_missing_skill" rows, in setup.Result.AgentsMissingSkill's own
// order, never nil.
func missingSkillJSONRows(agents []setup.MissingSkillAgent) []missingSkillJSON {
	out := make([]missingSkillJSON, 0, len(agents))

	for _, a := range agents {
		out = append(out, missingSkillJSON{Role: a.Role, Agent: a.Agent, Path: a.Path, Scope: string(a.Scope)})
	}

	return out
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

// renderPrint writes --print's text-mode body to w: one
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

// initNextAction renders init's stderr next-action line, minus the
// "brief init: " prefix: dryRun's own line when set; "already installed;
// nothing changed" when nothing was created or merged; a config-only
// message for a non-claude-code host; otherwise an "installed for
// claude-code" message naming the detection signal, if any, and Claude
// Code's own startup step.
func initNextAction(host string, dryRun bool, artifacts []setup.Artifact, wd, root, detectedBy string) string {
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

	label := "installed for claude-code"
	// The detected clause appears only when host was inferred rather than
	// given explicitly, so --host none's own escape hatch stays
	// discoverable on a detected install.
	if detectedBy != "" {
		label = fmt.Sprintf("installed for claude-code (detected %s; use --host none to skip)", detectedBy)
	}

	// Claude Code loads a project skills-directory plugin only from the
	// session's own working directory, no walk-up, so the line names root
	// whenever it differs from wd.
	if root == wd {
		return label + "; start Claude Code in this directory (or run /reload-plugins in a session already here), then 'brief new feature <name>'"
	}

	rel := displayPath(wd, root)

	return fmt.Sprintf("%s in %s; start Claude Code in %s (or run /reload-plugins in a session already there), then 'brief new feature <name>'", label, rel, rel)
}

// missingSkillHeaderBase is the missing-skill stderr block header's invariant prefix.
const missingSkillHeaderBase = `bound agents do not preload the brief-workflow skill; add "brief-workflow" to the "skills:" list in each`

// missingSkillFixable reports whether a is a row "--edit-agents" could
// still reach: a ScopeProject row whose Reach is not one of the three
// setup marks unmergeable. A ScopeUser row is never fixable regardless of
// Reach, since --edit-agents never targets one.
func missingSkillFixable(a setup.MissingSkillAgent) bool {
	if a.Scope != agentfile.ScopeProject {
		return false
	}

	switch a.Reach {
	case setup.ReachNotRegular, setup.ReachUneditable, setup.ReachEscaped:
		return false
	case setup.ReachFixable, setup.ReachNone:
		return true
	default:
		// Any Reach value setup does not yet assign a row falls back to
		// fixable, so a row can never silently vanish from the report if
		// that invariant is broken elsewhere.
		return true
	}
}

// missingSkillHeader renders init's missing-skill stderr block header: the
// suffix ", or rerun with --edit-agents:" is appended only when editAgents
// was not given on this run and at least one listed agent is one it could
// still reach (missingSkillFixable); otherwise the header ends plain ":",
// since suggesting a flag that cannot help any row left would be a dead end.
func missingSkillHeader(editAgents bool, agents []setup.MissingSkillAgent) string {
	if !editAgents && slices.ContainsFunc(agents, missingSkillFixable) {
		return missingSkillHeaderBase + `, or rerun with --edit-agents:`
	}

	return missingSkillHeaderBase + `:`
}

// missingSkillLines renders one line per agents entry, in four groups —
// fixable, unreachable project-scope, escaped project-scope, then
// user-scope — each in agents' own relative order, so the rows an adopter
// can fix by rerunning init with --edit-agents come before the ones
// always left for them to edit by hand. The grouping is a display concern
// only; setup.Result.AgentsMissingSkill itself stays in role-major order.
func missingSkillLines(wd string, agents []setup.MissingSkillAgent) []string {
	lines := make([]string, 0, len(agents))

	for _, a := range agents {
		if missingSkillFixable(a) {
			lines = append(lines, fmt.Sprintf("  %s (%s)", displayPath(wd, a.Path), a.Role))
		}
	}

	for _, a := range agents {
		switch a.Reach {
		case setup.ReachNotRegular:
			lines = append(lines, fmt.Sprintf("  %s (%s; not a regular file, edit by hand)", displayPath(wd, a.Path), a.Role))
		case setup.ReachUneditable:
			lines = append(lines, fmt.Sprintf("  %s (%s; skills: is not a list brief can edit, edit by hand)", displayPath(wd, a.Path), a.Role))
		case setup.ReachFixable, setup.ReachEscaped, setup.ReachNone:
			// Rendered in a different group; nothing to do here.
		}
	}

	for _, a := range agents {
		if a.Reach == setup.ReachEscaped {
			lines = append(lines, fmt.Sprintf("  %s (%s; outside the repository, edit by hand)", displayPath(wd, a.Path), a.Role))
		}
	}

	for _, a := range agents {
		if a.Scope == agentfile.ScopeUser {
			lines = append(lines, fmt.Sprintf("  ~/%s (%s; user-level, edit by hand)", a.ScopeRelPath, a.Role))
		}
	}

	return lines
}

// unwrittenLine renders --print's "printed only" or "already installed"
// stderr line, minus the "brief init: " prefix: the nothing-pending line
// when artifacts is empty, else the "printed only" line.
func unwrittenLine(artifacts []setup.PrintArtifact) string {
	if len(artifacts) == 0 {
		return "already installed; nothing changed"
	}

	return "printed only, no files changed; apply the output above by hand, or rerun without --print"
}

// editAgentsNothingToEditLine is --edit-agents' exit-0 stderr line, text mode only, never --json.
const editAgentsNothingToEditLine = `--edit-agents: no planner or implementer bound to an agent under .claude/agents; nothing to edit`

// hasBoundAgentArtifact reports whether artifacts carries a
// setup.KindBoundAgent row.
func hasBoundAgentArtifact(artifacts []setup.Artifact) bool {
	for _, a := range artifacts {
		if a.Kind == setup.KindBoundAgent {
			return true
		}
	}

	return false
}

// printEditAgentsNothingToEdit writes editAgentsNothingToEditLine to
// out.stderr when editAgents is set and artifacts carries no
// setup.KindBoundAgent row — the print-mode and write-mode render paths'
// own shared guard.
func printEditAgentsNothingToEdit(out reporter, editAgents bool, artifacts []setup.Artifact) {
	if editAgents && !hasBoundAgentArtifact(artifacts) {
		fmt.Fprintf(out.stderr, "brief init: %s\n", editAgentsNothingToEditLine)
	}
}

// runInit implements "brief init [--host <name>] [--no-hook]
// [--with-agents] [--edit-agents] [--dry-run | --print] [--force] [--json]";
// rest is its positional arguments, flags already parsed away and must be
// empty. host is "" when --host was not given, passed through unchanged to
// setup.InitRequest.Host: setup.Init treats "" as "detect" rather than
// defaulting it here.
func runInit(ctx context.Context, wd string, rest []string, host string, noHook, withAgents, editAgents, dryRun, printOnly, force bool, out reporter, extraSetupOpts ...setup.Option) error {
	if len(rest) > 0 {
		return out.usageError(fmt.Sprintf("brief init: too many arguments; run '%s'", initInvocation))
	}

	if dryRun && printOnly {
		return out.usageError("brief init: --dry-run and --print cannot be combined; run 'brief init --print'")
	}

	srv := setup.NewServer(extraSetupOpts...)

	res, err := srv.Init(ctx, wd, setup.InitRequest{Host: host, NoHook: noHook, WithAgents: withAgents, EditAgents: editAgents, DryRun: dryRun, Force: force, Print: printOnly})
	if err != nil {
		if errors.Is(err, setup.ErrAgentsNeedHost) {
			return out.usageError(fmt.Sprintf("brief init: --with-agents requires --host claude-code; run '%s --with-agents'", initInvocation))
		}

		if errors.Is(err, setup.ErrEditAgentsNeedHost) {
			return out.usageError(fmt.Sprintf("brief init: --edit-agents requires --host claude-code; run '%s --edit-agents'", initInvocation))
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
			return out.document(initPrintDocument{jsonHeader: out.successHeader(), Artifacts: printArtifactsJSON(res.Print)}, false)
		}

		doc := initDocument{
			jsonHeader:         out.successHeader(),
			Host:               res.Host,
			DetectedBy:         nonEmptyString(res.DetectedBy),
			DryRun:             res.DryRun,
			Created:            res.Created,
			Modified:           res.Modified,
			Artifacts:          artifactsJSON(res.Artifacts),
			RolesToAdd:         res.RolesToAdd,
			AgentsMissingSkill: missingSkillJSONRows(res.AgentsMissingSkill),
		}

		return out.document(doc, changedFiles(res))
	}

	if printOnly {
		renderPrint(out.stdout, wd, res.Print)

		printEditAgentsNothingToEdit(out, editAgents, res.Artifacts)

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

	printEditAgentsNothingToEdit(out, editAgents, res.Artifacts)

	if len(res.AgentsMissingSkill) > 0 {
		fmt.Fprintf(out.stderr, "brief init: %s\n", missingSkillHeader(editAgents, res.AgentsMissingSkill))

		for _, line := range missingSkillLines(wd, res.AgentsMissingSkill) {
			fmt.Fprintln(out.stderr, line)
		}
	}

	if !res.DryRun && res.NoHostDetected {
		fmt.Fprintln(out.stderr, "brief init: no agent host detected; run 'brief init --host claude-code' to install integration")

		return nil
	}

	fmt.Fprintf(out.stderr, "brief init: %s\n", initNextAction(res.Host, res.DryRun, res.Artifacts, wd, res.Root, res.DetectedBy))

	return nil
}

// renderUnwritable renders the refusal for setup.ErrUnwritable: text mode
// renders the ordinary refusal line on stderr and then res.Print's body
// on stdout, byte for byte what --print would have shown; JSON mode
// writes the standard error document with its fix overridden to point at
// "brief init --print --json", carrying no artifacts field at all, unlike
// the print-only success document above.
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

	out.writeErrorDocument(doc)

	return err
}

// nonEmptyString returns nil for "", else a pointer to s — the same
// null-unless-populated shape artifactJSON's own Detail field uses.
func nonEmptyString(s string) *string {
	if s == "" {
		return nil
	}

	return &s
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
