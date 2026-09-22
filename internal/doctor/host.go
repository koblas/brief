package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/host"
)

// integrationFileState is one Claude Code integration file's own probe
// result: present mirrors an Lstat success (any file type), regular is
// true only for a present, regular file whose bytes were read and
// classified into origin — never populated (OriginEdited's zero value)
// otherwise.
type integrationFileState struct {
	relPath string
	path    string
	present bool
	regular bool
	origin  artifact.Origin
}

// probeIntegrationFile Lstats root/f.RelPath and, for a regular file, reads
// and artifact.Recognizes its bytes against f.Kind. A read failure on a
// present, regular file is reported the same as "not regular" — a subject
// file this broken can never be classified as current, older or edited.
func probeIntegrationFile(root string, f host.File) integrationFileState {
	path := filepath.Join(root, filepath.FromSlash(f.RelPath))
	state := integrationFileState{relPath: f.RelPath, path: path}

	info, err := os.Lstat(path)
	if err != nil {
		return state
	}

	state.present = true

	if !info.Mode().IsRegular() {
		return state
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return state
	}

	state.regular = true
	state.origin = artifact.Recognize(f.Kind, body)

	return state
}

// probeIntegrationFiles probes every file in files, in order.
func probeIntegrationFiles(root string, files []host.File) []integrationFileState {
	out := make([]integrationFileState, 0, len(files))
	for _, f := range files {
		out = append(out, probeIntegrationFile(root, f))
	}

	return out
}

// anyPresent reports whether any state in states is present, of any file
// type.
func anyPresent(states []integrationFileState) bool {
	for _, s := range states {
		if s.present {
			return true
		}
	}

	return false
}

// missingRelPaths returns the relPath of every state that is absent or not
// a regular file, in states' own order.
func missingRelPaths(states []integrationFileState) []string {
	var out []string

	for _, s := range states {
		if !s.present || !s.regular {
			out = append(out, s.relPath)
		}
	}

	return out
}

// relPathsWithOrigin returns the relPath of every present, regular state
// whose origin is origin, in states' own order.
func relPathsWithOrigin(states []integrationFileState, origin artifact.Origin) []string {
	var out []string

	for _, s := range states {
		if s.present && s.regular && s.origin == origin {
			out = append(out, s.relPath)
		}
	}

	return out
}

// originRow maps origin — already known to belong to a present, regular
// subject file, never a missing one — to the (Severity, Detail, Fix) triple
// every host row's own "older render"/"edited locally"/"current" arms
// share: OriginOlder is WARN "installed by an older brief release" + suffix,
// with fix olderFix; OriginEdited is OK "edited locally" + suffix;
// OriginCurrent (and any other value) is OK "installed", plain. suffix is
// appended verbatim — a multi-file row (host-plugin, host-agents) passes
// ": <rel, ...>", a single-subject row (host-hook, host-snippet) passes "".
// This is the one place every host row's own origin precedence lives, so
// the four can never drift from each other; it is also the only way to
// reach the OriginOlder arm at all today, since every compiled-in older
// digest list ships empty.
func originRow(origin artifact.Origin, olderFix, suffix string) (Severity, string, *string) {
	switch origin {
	case artifact.OriginOlder:
		return SeverityWarn, "installed by an older brief release" + suffix, new(olderFix)
	case artifact.OriginEdited:
		return SeverityOK, "edited locally" + suffix, nil
	case artifact.OriginCurrent:
		return SeverityOK, "installed", nil
	default:
		return SeverityOK, "installed", nil
	}
}

// anyIntegrationFilePresent reports whether any file of h.Plugin(true) ∪
// h.Agents() is present under root, of any file type — the predicate
// host-plugin's and host-hook's own SKIP rows share (R13).
func anyIntegrationFilePresent(root string, h host.Host) bool {
	return anyPresent(probeIntegrationFiles(root, h.Plugin(true))) || anyPresent(probeIntegrationFiles(root, h.Agents()))
}

// hostPluginCheck builds host-plugin's own row: not installed anywhere is
// SKIP; otherwise a missing or non-regular subject file (h.Plugin(false))
// is ERROR "incomplete", naming every such file; an older render is WARN;
// an edited one is OK "edited locally"; every subject file current is OK
// "installed" — in that precedence (originRow, applied across every
// subject file at once rather than one row at a time, since a single
// host-plugin row must summarize all three).
func hostPluginCheck(root string, h host.Host, installed bool) Check {
	path := filepath.Join(root, host.PluginDir)

	if !installed {
		return Check{ID: "host-plugin", Severity: SeveritySkip, Path: path, Detail: "not installed", Fix: new(runInitClaudeCode)}
	}

	states := probeIntegrationFiles(root, h.Plugin(false))

	if missing := missingRelPaths(states); len(missing) > 0 {
		return Check{ID: "host-plugin", Severity: SeverityError, Path: path, Detail: "incomplete: missing " + strings.Join(missing, ", "), Fix: new(runInit)}
	}

	if older := relPathsWithOrigin(states, artifact.OriginOlder); len(older) > 0 {
		sev, detail, fix := originRow(artifact.OriginOlder, runInit, ": "+strings.Join(older, ", "))

		return Check{ID: "host-plugin", Severity: sev, Path: path, Detail: detail, Fix: fix}
	}

	if edited := relPathsWithOrigin(states, artifact.OriginEdited); len(edited) > 0 {
		sev, detail, fix := originRow(artifact.OriginEdited, runInit, ": "+strings.Join(edited, ", "))

		return Check{ID: "host-plugin", Severity: sev, Path: path, Detail: detail, Fix: fix}
	}

	return Check{ID: "host-plugin", Severity: SeverityOK, Path: path, Detail: "installed"}
}

// hookFileOf returns h.Plugin(true)'s own trailing hook entry.
func hookFileOf(h host.Host) host.File {
	for _, f := range h.Plugin(true) {
		if f.Hook {
			return f
		}
	}

	return host.File{}
}

// hostHookCheck builds host-hook's own row: not installed anywhere is
// SKIP; a hook file absent while something else is installed is WARN
// "installed without the check hook" — doctor cannot tell a lost file
// from --no-hook, so this stays a WARN rather than an ERROR; a present
// but non-regular hook is ERROR; an older render is WARN; an edited one
// is OK "edited locally"; a current one is OK "installed".
func hostHookCheck(root string, h host.Host, installed bool) Check {
	hookFile := hookFileOf(h)
	state := probeIntegrationFile(root, hookFile)

	if !state.present {
		if !installed {
			return Check{ID: "host-hook", Severity: SeveritySkip, Path: state.path, Detail: "not installed", Fix: new(runInitClaudeCode)}
		}

		return Check{ID: "host-hook", Severity: SeverityWarn, Path: state.path, Detail: "installed without the check hook", Fix: new("run 'brief init' to add it")}
	}

	if !state.regular {
		return Check{ID: "host-hook", Severity: SeverityError, Path: state.path, Detail: "not a regular file", Fix: new(runInit)}
	}

	sev, detail, fix := originRow(state.origin, runInit, "")

	return Check{ID: "host-hook", Severity: sev, Path: state.path, Detail: detail, Fix: fix}
}

// hostAgentsCheck builds host-agents' own row: none of the three role
// agent files present is SKIP; otherwise a missing or non-regular one is
// WARN (never ERROR — an unbound role is reported by the roles row, not
// enforced here), naming every such file; an older render is WARN; an
// edited one is OK "edited locally"; all three current is OK "installed".
// Every fix here names --with-agents: a plain "brief init" never plans an
// agent file at all, so it can never repair one on its own.
func hostAgentsCheck(root string, h host.Host) Check {
	agentFiles := h.Agents()
	path := filepath.Join(root, host.PluginDir, "agents")
	states := probeIntegrationFiles(root, agentFiles)

	if !anyPresent(states) {
		return Check{ID: "host-agents", Severity: SeveritySkip, Path: path, Detail: "not installed", Fix: new(runInitWithAgents)}
	}

	if missing := missingRelPaths(states); len(missing) > 0 {
		return Check{ID: "host-agents", Severity: SeverityWarn, Path: path, Detail: "missing " + strings.Join(missing, ", "), Fix: new(runInitWithAgents)}
	}

	if older := relPathsWithOrigin(states, artifact.OriginOlder); len(older) > 0 {
		sev, detail, fix := originRow(artifact.OriginOlder, runInitWithAgents, ": "+strings.Join(older, ", "))

		return Check{ID: "host-agents", Severity: sev, Path: path, Detail: detail, Fix: fix}
	}

	if edited := relPathsWithOrigin(states, artifact.OriginEdited); len(edited) > 0 {
		sev, detail, fix := originRow(artifact.OriginEdited, runInitWithAgents, ": "+strings.Join(edited, ", "))

		return Check{ID: "host-agents", Severity: sev, Path: path, Detail: detail, Fix: fix}
	}

	return Check{ID: "host-agents", Severity: SeverityOK, Path: path, Detail: "installed"}
}

// snippetCandidateState is one CLAUDE.md candidate's own scan result,
// mirroring internal/setup's own candidateSnippetFile but read-only: present
// mirrors an Lstat success (any file type) — a regular candidate whose
// bytes could not be read is still present, distinguished by unreadable
// rather than folded into "nothing here", since brief does know the file
// exists and only cannot say whether it carries a block. body and span are
// populated only for a regular file that was both readable and scanned
// clean (no marker defect); prob carries the first marker defect
// artifact.ScanSnippetMarkers found, nil otherwise. notRegular and kind are
// populated only when the candidate exists but Lstat reports it is not a
// regular file — kind is "symlink" or "directory" (nonRegularKind).
// unreadable and readErr are populated only when the candidate is present,
// regular, and os.ReadFile failed — readErr is the underlying reason (a
// wrapped *fs.PathError's own inner error, e.g. "permission denied") rather
// than the full "open <path>: …" text, since host-snippet's own WARN detail
// already names the path via the row's Path field. notRegular and
// unreadable are mutually exclusive: only a regular candidate is ever read
// at all.
type snippetCandidateState struct {
	path       string
	relPath    string
	present    bool
	body       []byte
	span       *artifact.SnippetSpan
	prob       *artifact.MarkerProblem
	notRegular bool
	kind       string
	unreadable bool
	readErr    string
}

// nonRegularKind names the file type behind a CLAUDE.md candidate Lstat
// reports as not a regular file, for host-snippet's own WARN detail: a
// symlink (checked first — a symlink to a directory reports both bits, and
// "symlink" is the more useful of the two to a reader deciding what to do
// about it) or a directory; any other mode (a fifo, a socket, a device —
// never observed against a CLAUDE.md path in practice) falls back to "",
// so the caller's parenthetical is dropped rather than rendering the
// name twice ("not a regular file (not a regular file)").
func nonRegularKind(info os.FileInfo) string {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return "symlink"
	case info.IsDir():
		return "directory"
	default:
		return ""
	}
}

// notRegularDetail renders host-snippet's own WARN detail for a candidate
// Lstat reports as not a regular file: kind in parentheses when
// nonRegularKind named one, the bare sentence otherwise.
func notRegularDetail(kind string) string {
	if kind == "" {
		return "not a regular file; brief block not installed"
	}

	return fmt.Sprintf("not a regular file (%s); brief block not installed", kind)
}

// readFailureReason extracts the underlying reason behind a failed
// os.ReadFile call: a wrapped *fs.PathError's own inner error (e.g.
// "permission denied" out of "open <path>: permission denied") when err
// carries one, err's own message otherwise.
func readFailureReason(err error) string {
	if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
		return pathErr.Err.Error()
	}

	return err.Error()
}

// notReadableDetail renders host-snippet's own WARN detail for a candidate
// that is present and regular but whose bytes os.ReadFile could not read:
// reason is readFailureReason's own extracted cause, interpolated verbatim
// so the row states why rather than asserting the block is simply absent —
// a claim scanning could not actually verify.
func notReadableDetail(reason string) string {
	return fmt.Sprintf("not readable (%s); cannot check for brief block", reason)
}

// notReadableFix renders host-snippet's own WARN fix for an unreadable
// candidate: make relPath readable, then re-run init the same way the SKIP
// "not installed" row already does.
func notReadableFix(relPath string) string {
	return fmt.Sprintf("chmod +r %s, then %s", relPath, runInitClaudeCode)
}

// scanSnippetCandidateStates Lstats and scans every h.InstructionFiles()
// candidate under root, in that order. A missing candidate reports a zero
// snippetCandidateState (no body, no span, no problem, present false) —
// the "nothing here" shape host-snippet's own SKIP path expects. A
// candidate that exists but is not a regular file reports notRegular true
// and kind set (nonRegularKind), never read. A candidate that exists, is
// regular, but whose bytes os.ReadFile could not read reports present true,
// unreadable true and readErr set (readFailureReason) — present, since
// brief does know the file is there, just not what it contains.
func scanSnippetCandidateStates(root string, h host.Host) []snippetCandidateState {
	rel := h.InstructionFiles()
	out := make([]snippetCandidateState, 0, len(rel))

	for _, r := range rel {
		path := filepath.Join(root, filepath.FromSlash(r))

		info, err := os.Lstat(path)

		switch {
		case err != nil:
			out = append(out, snippetCandidateState{path: path, relPath: r})

			continue
		case !info.Mode().IsRegular():
			out = append(out, snippetCandidateState{path: path, relPath: r, present: true, notRegular: true, kind: nonRegularKind(info)})

			continue
		}

		body, err := os.ReadFile(path)
		if err != nil {
			out = append(out, snippetCandidateState{path: path, relPath: r, present: true, unreadable: true, readErr: readFailureReason(err)})

			continue
		}

		span, prob := artifact.ScanSnippetMarkers(body)
		out = append(out, snippetCandidateState{path: path, relPath: r, present: true, body: body, span: span, prob: prob})
	}

	return out
}

// snippetBlockFound reports whether any state in states holds a recognized
// marker span — env-path's own "or a snippet block found" clause (R13).
func snippetBlockFound(states []snippetCandidateState) bool {
	for _, s := range states {
		if s.span != nil {
			return true
		}
	}

	return false
}

// hostSnippetCheck builds host-snippet's own row from states — already
// scanned by scanSnippetCandidateStates — and dir/dirKnown, the configured
// feature directory (dirKnown false only when ".brief.yaml" itself is
// unparseable). A marker defect in any candidate is ERROR, citing that
// candidate's own path and line; two candidates each holding a span is
// ERROR on the second (".claude/CLAUDE.md" — root is the preferred
// location). Once neither ERROR arm fires, a candidate holding a span still
// wins regardless of the other candidate's own shape — a real block in
// ".claude/CLAUDE.md" reports installed even when root's own "CLAUDE.md" is
// a symlink or a directory — classified by artifact.RecognizeSnippet:
// OriginEdited is OK "edited locally", OriginOlder is WARN, OriginCurrent
// is OK "installed" when its own Dir matches dir (or dirKnown is false —
// nothing to compare against) and WARN naming both directories otherwise.
// Only once no candidate holds a span does existence matter, and only for
// the one candidate planSnippet itself would then choose — the first
// candidate that is present at all (states' own priority order), regular
// or not, readable or not. This mirrors setup's own chooseSnippetLocation
// for every case both reach: a later candidate's own shape is never
// consulted, so a regular-but-blockless first candidate reports the
// ordinary SKIP below, naming that same candidate's own path, even when a
// farther candidate happens to be a symlink or a directory. The one
// divergence: a regular candidate whose bytes could not be read is
// present here but never reaches chooseSnippetLocation at all, since
// setup's own scan aborts with a hard error on the same failure — brief
// has no way to write a snippet block through a file it cannot even read,
// so there is nothing for setup's own selection rule to reach. When that
// first present candidate is itself notRegular, the row is WARN, naming
// which (nonRegularKind) — brief can neither write nor scan through it, so
// the fix points at --print (runInitPrintSnippet) rather than a plain
// re-run. When it is instead unreadable, the row is WARN "not readable
// (<reason>); cannot check for brief block" (notReadableDetail) — brief
// does not know whether a block is present, so it reports that rather than
// the "not installed" a genuinely absent candidate gets, with a fix
// (notReadableFix) that names making the file readable before the plain
// re-run. No candidate present at all, or the first present one is a
// regular, readable, blockless file, is SKIP "not installed", Path naming
// that first-present candidate (falling back to the first candidate in
// priority order only when none is present at all).
func hostSnippetCheck(states []snippetCandidateState, dir string, dirKnown bool) Check {
	for _, s := range states {
		if s.prob != nil {
			return Check{ID: "host-snippet", Severity: SeverityError, Path: s.path, Detail: fmt.Sprintf("%s (line %d)", s.prob.Problem, s.prob.Line), Fix: &s.prob.Fix}
		}
	}

	if len(states) == 2 && states[0].span != nil && states[1].span != nil {
		return Check{ID: "host-snippet", Severity: SeverityError, Path: states[1].path, Detail: "a brief block already exists in CLAUDE.md", Fix: new("delete that block")}
	}

	var chosen *snippetCandidateState

	for i := range states {
		if states[i].span != nil {
			chosen = &states[i]

			break
		}
	}

	if chosen == nil {
		var firstPresent *snippetCandidateState

		for i := range states {
			if !states[i].present {
				continue
			}

			firstPresent = &states[i]

			break
		}

		if firstPresent != nil && firstPresent.notRegular {
			return Check{
				ID: "host-snippet", Severity: SeverityWarn, Path: firstPresent.path,
				Detail: notRegularDetail(firstPresent.kind),
				Fix:    new(runInitPrintSnippet),
			}
		}

		if firstPresent != nil && firstPresent.unreadable {
			return Check{
				ID: "host-snippet", Severity: SeverityWarn, Path: firstPresent.path,
				Detail: notReadableDetail(firstPresent.readErr),
				Fix:    new(notReadableFix(firstPresent.relPath)),
			}
		}

		path := ""

		switch {
		case firstPresent != nil:
			path = firstPresent.path
		case len(states) > 0:
			path = states[0].path
		}

		return Check{ID: "host-snippet", Severity: SeveritySkip, Path: path, Detail: "not installed", Fix: new(runInitClaudeCode)}
	}

	block := chosen.body[chosen.span.Start:chosen.span.End]
	match := artifact.RecognizeSnippet(block)

	if match.Origin == artifact.OriginOlder || match.Origin == artifact.OriginEdited {
		sev, detail, fix := originRow(match.Origin, runInit, "")

		return Check{ID: "host-snippet", Severity: sev, Path: chosen.path, Detail: detail, Fix: fix}
	}

	if !dirKnown {
		return Check{ID: "host-snippet", Severity: SeverityOK, Path: chosen.path, Detail: "installed (feature directory not compared: .brief.yaml did not parse)"}
	}

	trimmedDir := strings.TrimRight(dir, "/")
	if match.Dir == trimmedDir {
		return Check{ID: "host-snippet", Severity: SeverityOK, Path: chosen.path, Detail: "installed"}
	}

	return Check{
		ID: "host-snippet", Severity: SeverityWarn, Path: chosen.path,
		Detail: fmt.Sprintf("names %s/, .brief.yaml says %s/", match.Dir, trimmedDir),
		Fix:    new(runInit),
	}
}

// roleBindingResult classifies one role's own binding value against the
// filesystem.
type roleBindingResult int

const (
	// roleUnbound marks an empty binding.
	roleUnbound roleBindingResult = iota
	// roleResolved marks a binding whose own agent file was found.
	roleResolved
	// roleUnresolved marks a non-empty binding whose own agent file was
	// not found.
	roleUnresolved
	// roleUnverified marks a "<plugin>:<name>" binding for a plugin other
	// than "brief" — doctor has no file layout to check it against, so it
	// counts as bound without being verified.
	roleUnverified
)

// resolveRoleBinding classifies value (a config.RoleBindings field) against
// root and, for a bare name, an injected home (R7): "" is roleUnbound; a
// "brief:<name>" binding is roleResolved when
// "<root>/.claude/skills/brief/agents/<name>.md" or, overriding it,
// "<root>/.claude/agents/<name>.md" is a regular file, roleUnresolved
// otherwise; any other "<plugin>:<name>" is roleUnverified; a bare "<name>"
// is roleResolved via "<root>/.claude/agents/<name>.md" or
// "<home>/.claude/agents/<name>.md" (home errors, or an empty home, are
// treated as no home directory at all), roleUnresolved otherwise.
func (s *Server) resolveRoleBinding(root, value string) roleBindingResult {
	if value == "" {
		return roleUnbound
	}

	if plugin, agent, ok := strings.Cut(value, ":"); ok {
		if plugin != "brief" {
			return roleUnverified
		}

		if fileIsRegular(filepath.Join(root, host.PluginDir, "agents", agent+".md")) ||
			fileIsRegular(filepath.Join(root, ".claude", "agents", agent+".md")) {
			return roleResolved
		}

		return roleUnresolved
	}

	if fileIsRegular(filepath.Join(root, ".claude", "agents", value+".md")) {
		return roleResolved
	}

	if home, err := s.homeDir(); err == nil && home != "" && fileIsRegular(filepath.Join(home, ".claude", "agents", value+".md")) {
		return roleResolved
	}

	return roleUnresolved
}

// fileIsRegular reports whether path exists and is a regular file,
// following symlinks.
func fileIsRegular(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.Mode().IsRegular()
}

// roleBinding names one role position alongside its own configured value,
// the input rolesCheck's own per-role loop walks in RoleBindings' own field
// order.
type roleBinding struct {
	name, value string
}

// rolesCheckNoConfig builds roles' own row when no ".brief.yaml" was found
// anywhere: SKIP "no roles bound", Path "" (R13's own "" when none).
func rolesCheckNoConfig() Check {
	return Check{ID: "roles", Severity: SeveritySkip, Detail: "no roles bound", Fix: new(runInitWithAgents)}
}

// rolesCheckUnparseable builds roles' own row when nearest exists but does
// not parse: SKIP ".brief.yaml did not parse", Fix nil — nothing to bind
// until the config itself is fixed.
func rolesCheckUnparseable(nearest string) Check {
	return Check{ID: "roles", Severity: SeveritySkip, Path: nearest, Detail: ".brief.yaml did not parse"}
}

// rolesCheck builds roles' own row for a parsed config: every binding
// empty is SKIP "no roles bound"; any unbound position or unresolved
// binding is WARN, detail listing each ("planner unbound; reviewer:
// brief:reviewer not found"), never ERROR (roles are reported, not
// enforced); everything bound and resolved is OK "planner, implementer,
// reviewer bound", with "not verified: <role>" appended for each
// roleUnverified binding.
func (s *Server) rolesCheck(root, nearest string, bindings [3]roleBinding) Check {
	if bindings[0].value == "" && bindings[1].value == "" && bindings[2].value == "" {
		return Check{ID: "roles", Severity: SeveritySkip, Path: nearest, Detail: "no roles bound", Fix: new(runInitWithAgents)}
	}

	var problems, notVerified []string

	for _, b := range bindings {
		switch s.resolveRoleBinding(root, b.value) {
		case roleUnbound:
			problems = append(problems, b.name+" unbound")
		case roleUnresolved:
			problems = append(problems, fmt.Sprintf("%s: %s not found", b.name, b.value))
		case roleUnverified:
			notVerified = append(notVerified, b.name)
		case roleResolved:
			// nothing to report
		}
	}

	if len(problems) > 0 {
		fix := "bind each role to an existing agent in .brief.yaml, or " + runInitWithAgents

		return Check{ID: "roles", Severity: SeverityWarn, Path: nearest, Detail: strings.Join(problems, "; "), Fix: &fix}
	}

	verifiedDetail := make([]string, 0, 1+len(notVerified))
	verifiedDetail = append(verifiedDetail, "planner, implementer, reviewer bound")

	for _, name := range notVerified {
		verifiedDetail = append(verifiedDetail, "not verified: "+name)
	}

	detail := strings.Join(verifiedDetail, "; ")

	return Check{ID: "roles", Severity: SeverityOK, Path: nearest, Detail: detail}
}

// runInit is short for "run 'brief init'" — the fix text repeated across
// host-plugin, host-hook and host-snippet rows whenever a plain re-run
// would repair the finding.
const runInit = "run 'brief init'"

// runInitClaudeCode is the SKIP fix every "not installed" host row shares.
const runInitClaudeCode = "run 'brief init --host claude-code'"

// runInitPrintSnippet is host-snippet's own WARN fix for a CLAUDE.md
// candidate that exists but is not a regular file (a symlink or a
// directory): a plain re-run can never write through it, so the fix points
// at --print's own manual-application path (R9) instead.
const runInitPrintSnippet = "run 'brief init --print' and add the CLAUDE.md block by hand"

// runInitWithAgents is the fix host-agents and roles share whenever the
// remedy needs --with-agents specifically.
const runInitWithAgents = "run 'brief init --with-agents'"
