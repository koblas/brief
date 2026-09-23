package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/host"
)

// classifyProbeError classifies err — from a failed os.Lstat against a
// host integration file or a CLAUDE.md candidate — into the one
// absent-vs-unreadable decision every Lstat probe in this package renders
// from: fs.ErrNotExist proves the path itself is not there, and so does
// syscall.ENOTDIR, since it means some ancestor path component is a
// regular file rather than a directory and nothing can resolve through
// one; any other error proves nothing about absence, so it is unreadable,
// its reason readFailureReason's own extracted cause. err is always
// non-nil. Checked on darwin and linux, devenv.nix's own build targets,
// where ENOTDIR carries this meaning; a windows build of this package was
// not exercised.
func classifyProbeError(err error) (bool, string) {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return true, ""
	}

	return false, readFailureReason(err)
}

// integrationFileState is one Claude Code integration file's own probe
// result, classifyProbeError's own split rendered into fields: present is
// false only when absent; unreadable is true when the file's own Lstat or
// ReadFile call failed without proving absence (reason carries its cause,
// statFailed whether it was the Lstat call rather than the ReadFile call
// that failed); regular is true only for a present, unreadable-false file
// whose bytes were read and classified into origin. present-but-neither-
// regular-nor-unreadable is host.go's own "not a regular file" shape.
type integrationFileState struct {
	relPath    string
	path       string
	present    bool
	regular    bool
	origin     artifact.Origin
	unreadable bool
	reason     string
	statFailed bool
}

// probeIntegrationFile Lstats root/f.RelPath and, for a regular file,
// reads and artifact.Recognizes its bytes against f.Kind. A Lstat failure
// renders through classifyProbeError; a ReadFile failure against a file
// Lstat itself just resolved as regular is always unreadable — typically
// the file's own mode denying read, but never reclassified as absent,
// since only a race between the two calls could make that reclassification
// correct.
func probeIntegrationFile(root string, f host.File) integrationFileState {
	path := filepath.Join(root, filepath.FromSlash(f.RelPath))
	state := integrationFileState{relPath: f.RelPath, path: path}

	info, err := os.Lstat(path)
	if err != nil {
		absent, reason := classifyProbeError(err)
		if !absent {
			state.present = true
			state.unreadable = true
			state.reason = reason
			state.statFailed = true
		}

		return state
	}

	state.present = true

	if !info.Mode().IsRegular() {
		return state
	}

	body, err := os.ReadFile(path)
	if err != nil {
		state.unreadable = true
		state.reason = readFailureReason(err)

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
// a regular file, excluding any state classifyProbeError read as
// unreadable (unreadableRelPaths reports those), in states' own order.
func missingRelPaths(states []integrationFileState) []string {
	var out []string

	for _, s := range states {
		if s.unreadable {
			continue
		}

		if !s.present || !s.regular {
			out = append(out, s.relPath)
		}
	}

	return out
}

// unreadableRelPaths returns the relPath of every state classifyProbeError
// read as unreadable, in states' own order.
func unreadableRelPaths(states []integrationFileState) []string {
	var out []string

	for _, s := range states {
		if s.unreadable {
			out = append(out, s.relPath)
		}
	}

	return out
}

// integrationFileRowDetail renders a multi-file host row's own Detail and
// Fix once any of states holds an unreadable file, checked ahead of
// missingRelPaths' own "missing" wording; the caller decides the row's
// own Severity. Every unreadable file is named "not readable (<reason>)"
// (notReadableReason), reason taken from the first unreadable state in
// states' own order — every probe failure under one broken directory tree
// shares the same cause in practice; any file still genuinely missing is
// named too, in its own fragment, since classifyProbeError only
// reclassifies the files it actually failed against. Fix is
// notReadableFix against that same first unreadable state, root bounding
// its own ancestor walk. ok is false when states holds no unreadable file
// at all.
func integrationFileRowDetail(wd, root string, states []integrationFileState) (string, string, bool) {
	unreadable := unreadableRelPaths(states)
	if len(unreadable) == 0 {
		return "", "", false
	}

	var first integrationFileState

	for _, s := range states {
		if s.unreadable {
			first = s

			break
		}
	}

	fragments := []string{notReadableReason(first.reason) + ": " + strings.Join(unreadable, ", ")}

	if missing := missingRelPaths(states); len(missing) > 0 {
		fragments = append(fragments, "missing "+strings.Join(missing, ", "))
	}

	return strings.Join(fragments, "; "), notReadableFix(wd, root, first.path, first.statFailed), true
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
// the four can never drift from each other. The planner and implementer
// agent renders are the one pair whose own older-digest list is non-empty
// (Rule 6), so host-agents' own OriginOlder arm is reachable through a real
// fixture today; host-plugin's, host-hook's and host-snippet's own
// OriginOlder arms still need host_internal_test.go's direct, synthetic
// call, since every other Kind's older-digest list still ships empty.
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
// SKIP; otherwise an unreadable subject file (h.Plugin(false)) is ERROR
// (integrationFileRowDetail) — the same severity a missing or non-regular
// one gets, since Claude Code cannot load the skill through a file it
// cannot read any more than one that is not there; a missing or
// non-regular one is ERROR "incomplete", naming every such file; an older
// render is WARN; an edited one is OK "edited locally"; every subject
// file current is OK "installed" — in that precedence (originRow, applied
// across every subject file at once rather than one row at a time, since
// a single host-plugin row must summarize all three).
func hostPluginCheck(wd, root string, h host.Host, installed bool) Check {
	path := filepath.Join(root, host.PluginDir)

	if !installed {
		return Check{ID: "host-plugin", Severity: SeveritySkip, Path: path, Detail: "not installed", Fix: new(runInitClaudeCode)}
	}

	states := probeIntegrationFiles(root, h.Plugin(false))

	if detail, fix, ok := integrationFileRowDetail(wd, root, states); ok {
		return Check{ID: "host-plugin", Severity: SeverityError, Path: path, Detail: detail, Fix: new(fix)}
	}

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
// from --no-hook, so this stays a WARN rather than an ERROR; an unreadable
// hook file is WARN, naming the reason (notReadableReason); a present but
// non-regular hook is ERROR; an older render is WARN; an edited one is OK
// "edited locally"; a current one is OK "installed".
func hostHookCheck(wd, root string, h host.Host, installed bool) Check {
	hookFile := hookFileOf(h)
	state := probeIntegrationFile(root, hookFile)

	if !state.present {
		if !installed {
			return Check{ID: "host-hook", Severity: SeveritySkip, Path: state.path, Detail: "not installed", Fix: new(runInitClaudeCode)}
		}

		return Check{ID: "host-hook", Severity: SeverityWarn, Path: state.path, Detail: "installed without the check hook", Fix: new("run 'brief init' to add it")}
	}

	if state.unreadable {
		return Check{ID: "host-hook", Severity: SeverityWarn, Path: state.path, Detail: notReadableReason(state.reason), Fix: new(notReadableFix(wd, root, state.path, state.statFailed))}
	}

	if !state.regular {
		return Check{ID: "host-hook", Severity: SeverityError, Path: state.path, Detail: "not a regular file", Fix: new(runInit)}
	}

	sev, detail, fix := originRow(state.origin, runInit, "")

	return Check{ID: "host-hook", Severity: sev, Path: state.path, Detail: detail, Fix: fix}
}

// hostAgentsCheck builds host-agents' own row: none of the three role
// agent files present is SKIP; otherwise an unreadable one is WARN
// (integrationFileRowDetail); a missing or non-regular one is WARN too
// (never ERROR — an unbound role is reported by the roles row, not
// enforced here), naming every such file; an older render is WARN; an
// edited one is OK "edited locally"; all three current is OK "installed".
// Every fix here names --with-agents, except an unreadable file's own
// shared chmod-then-reinit fix (notReadableFix): a plain "brief init"
// never plans an agent file at all, so it can never repair a missing one
// on its own, but a file that already exists needs no re-plan, only
// permission repair.
func hostAgentsCheck(wd, root string, h host.Host) Check {
	agentFiles := h.Agents()
	path := filepath.Join(root, host.PluginDir, "agents")
	states := probeIntegrationFiles(root, agentFiles)

	if !anyPresent(states) {
		return Check{ID: "host-agents", Severity: SeveritySkip, Path: path, Detail: "not installed", Fix: new(runInitWithAgents)}
	}

	if detail, fix, ok := integrationFileRowDetail(wd, root, states); ok {
		return Check{ID: "host-agents", Severity: SeverityWarn, Path: path, Detail: detail, Fix: new(fix)}
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
// mirroring internal/setup's own candidateSnippetFile but read-only,
// classifyProbeError's own split rendered the same way integrationFileState
// renders it: present is false only when absent; unreadable is true when
// classifyProbeError read the failure as neither absent nor a resolved
// file (readErr its cause, statFailed whether it was the Lstat call
// rather than the ReadFile call that failed); notRegular and kind are
// populated only when Lstat itself resolved the candidate to a non-regular
// file (nonRegularKind); body and span are populated only for a regular,
// readable file scanned clean, prob carrying the first marker defect
// artifact.ScanSnippetMarkers found otherwise. notRegular and unreadable
// are mutually exclusive: only a regular candidate Lstat could itself
// resolve is ever read at all.
type snippetCandidateState struct {
	path       string
	present    bool
	body       []byte
	span       *artifact.SnippetSpan
	prob       *artifact.MarkerProblem
	notRegular bool
	kind       string
	unreadable bool
	readErr    string
	statFailed bool
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
// os.Lstat or os.ReadFile call: both fail only with a *fs.PathError, so
// this unwraps that error's own inner cause (e.g. "permission denied" out
// of "lstat <path>: permission denied" or "open <path>: permission
// denied") rather than the full "<op> <path>: …" text, since host-snippet's
// own WARN detail already names the path via the row's Path field. Every
// caller passes a non-nil err it already knows came from one of those two
// calls.
func readFailureReason(err error) string {
	pathErr, _ := errors.AsType[*fs.PathError](err)

	return pathErr.Err.Error()
}

// notReadableReason renders "not readable (<reason>)" — the wording every
// host row shares for a subject classifyProbeError read as unreadable
// rather than absent or a resolved file. A caller appends its own suffix,
// or none.
func notReadableReason(reason string) string {
	return fmt.Sprintf("not readable (%s)", reason)
}

// notReadableDetail renders host-snippet's own WARN detail for an
// unreadable candidate: notReadableReason plus host-snippet's own suffix,
// stating that the block cannot be checked rather than asserting it is
// simply absent — a claim scanning could not actually verify.
func notReadableDetail(reason string) string {
	return notReadableReason(reason) + "; cannot check for brief block"
}

// blockingDir walks from filepath.Dir(path) upward, Lstat'ing each
// ancestor, until one resolves: that ancestor is the directory actually
// missing its own search (+x) bit, since every descendant beneath it
// failed to Lstat while it itself did not — Lstat needs +x on a path's
// parent to find its directory entry, never on the path itself, so an
// unsearchable directory still resolves its own Lstat but blocks every
// Lstat of anything nested inside it. The walk never rises above root —
// the one directory every host check already treats as its own install
// boundary — and returns root itself when no ancestor below it resolves,
// which is exactly the answer when root is the unsearchable directory.
// A symlinked ancestor resolves its own Lstat without its target being
// consulted, so the walk stops at the link, not at an unsearchable
// directory behind it.
func blockingDir(root, path string) string {
	dir := filepath.Dir(path)

	for dir != root {
		if _, err := os.Lstat(dir); err == nil {
			return dir
		}

		dir = filepath.Dir(dir)
	}

	return root
}

// notReadableFix renders the shared WARN fix for a subject
// classifyProbeError read as unreadable: when the failure was in the
// Lstat call itself (statFailed — an ancestor directory not searchable),
// the fix targets blockingDir's own result — the actual directory missing
// its search bit, not necessarily the subject's immediate parent — with
// both the search and write bits restored (chmod u+rwx), since init must
// still be able to create entries under it; when it was the ReadFile call
// instead (the file itself, present and regular, not readable), the fix
// targets the file (chmod +r). Both then re-run
// 'brief init --host claude-code', the same repair every "not installed"
// row already points at. path and its fix target are rendered relative to
// wd, the same wd Diagnose was called with, since every row's own Path is
// rendered relative to wd too; root bounds blockingDir's own walk.
func notReadableFix(wd, root, path string, statFailed bool) string {
	if statFailed {
		return fmt.Sprintf("chmod u+rwx %s, then %s", relPath(wd, blockingDir(root, path)), runInitClaudeCode)
	}

	return fmt.Sprintf("chmod +r %s, then %s", relPath(wd, path), runInitClaudeCode)
}

// scanSnippetCandidateStates Lstats and scans every h.InstructionFiles()
// candidate under root, in that order. A Lstat failure renders through
// classifyProbeError, the one place this function's own absent-vs-
// unreadable call is made. A ReadFile failure against a candidate Lstat
// itself just resolved as regular is always unreadable — typically the
// file's own mode denying read, but never reclassified as absent, since
// only a race between the two calls could make that reclassification
// correct — the same split probeIntegrationFile applies for a host
// integration file. A candidate that exists but is not a regular file
// reports notRegular true and kind set (nonRegularKind), never read.
func scanSnippetCandidateStates(root string, h host.Host) []snippetCandidateState {
	rel := h.InstructionFiles()
	out := make([]snippetCandidateState, 0, len(rel))

	for _, r := range rel {
		path := filepath.Join(root, filepath.FromSlash(r))

		info, err := os.Lstat(path)
		if err != nil {
			absent, reason := classifyProbeError(err)
			if absent {
				out = append(out, snippetCandidateState{path: path})

				continue
			}

			out = append(out, snippetCandidateState{path: path, present: true, unreadable: true, readErr: reason, statFailed: true})

			continue
		}

		if !info.Mode().IsRegular() {
			out = append(out, snippetCandidateState{path: path, present: true, notRegular: true, kind: nonRegularKind(info)})

			continue
		}

		body, err := os.ReadFile(path)
		if err != nil {
			out = append(out, snippetCandidateState{path: path, present: true, unreadable: true, readErr: readFailureReason(err)})

			continue
		}

		span, prob := artifact.ScanSnippetMarkers(body)
		out = append(out, snippetCandidateState{path: path, present: true, body: body, span: span, prob: prob})
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
// wins regardless of the other candidate's own shape, classified by
// artifact.RecognizeSnippet: OriginEdited is OK "edited locally",
// OriginOlder is WARN, OriginCurrent is OK "installed" when its own Dir
// matches dir (or dirKnown is false) and WARN naming both directories
// otherwise. Only once no candidate holds a span does existence matter,
// and only for the one candidate planSnippet itself would then choose —
// the first candidate present at all (states' own priority order),
// mirroring setup's own chooseSnippetLocation: notRegular is WARN naming
// which shape (nonRegularKind), fix pointing at --print
// (runInitPrintSnippet) since brief can neither write nor scan through
// it; unreadable is WARN (notReadableDetail, classifyProbeError's own
// split), fix notReadableFix, since a stat or read failure proves nothing
// about whether a block is actually there. No candidate present at all,
// or the first present one is a regular, readable, blockless file, is
// SKIP "not installed", Path naming that first-present candidate (or the
// first candidate in priority order when none is present at all).
func hostSnippetCheck(wd, root string, states []snippetCandidateState, dir string, dirKnown bool) Check {
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
				Fix:    new(notReadableFix(wd, root, firstPresent.path, firstPresent.statFailed)),
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

// roleResolution is resolveRoleBinding's own result: state classifies the
// binding, path is the resolved file's own absolute path when state is
// roleResolved (nil-string otherwise), and defs carries every
// agentfile.Definition a bare-name binding's own agentfile.Find returned
// — nil for a "brief:<name>", any other "<plugin>:<name>", or an unbound
// or unresolved binding. rolesCheck derives the duplicate-definition WARN
// and the "user-level:" OK suffix from defs; path exists so a later
// consumer (S05's roles-skill) can read frontmatter from whichever file
// actually resolved without re-deriving the "brief:*" filename rule.
type roleResolution struct {
	state roleBindingResult
	path  string
	defs  []agentfile.Definition
}

// resolveRoleBinding classifies value (a config.RoleBindings field)
// against root and, for a bare name, an injected home (R7, Rule 5): "" is
// roleUnbound; a "brief:<name>" binding is roleResolved via
// "<root>/.claude/agents/<name>.md", overriding
// "<root>/.claude/skills/brief/agents/<name>.md" when both are regular
// files, roleUnresolved when neither is; any other "<plugin>:<name>" is
// roleUnverified; a bare "<name>" is roleResolved when
// agentfile.Find(root, home, name) returns at least one Definition (home
// errors, or an empty home, are treated as no home directory at all),
// roleUnresolved otherwise — path and defs then carry its first and every
// result respectively, in the scope agentfile.Find chose (project agents
// shadow a same-named user one entirely, never mixed).
func (s *Server) resolveRoleBinding(root, value string) roleResolution {
	if value == "" {
		return roleResolution{state: roleUnbound}
	}

	if plugin, agent, ok := strings.Cut(value, ":"); ok {
		if plugin != "brief" {
			return roleResolution{state: roleUnverified}
		}

		overridePath := filepath.Join(root, ".claude", "agents", agent+".md")
		pluginPath := filepath.Join(root, host.PluginDir, "agents", agent+".md")

		switch {
		case fileIsRegular(overridePath):
			return roleResolution{state: roleResolved, path: overridePath}
		case fileIsRegular(pluginPath):
			return roleResolution{state: roleResolved, path: pluginPath}
		default:
			return roleResolution{state: roleUnresolved}
		}
	}

	home, err := s.homeDir()
	if err != nil {
		home = ""
	}

	defs := agentfile.Find(root, home, value)
	if len(defs) == 0 {
		return roleResolution{state: roleUnresolved}
	}

	return roleResolution{state: roleResolved, path: defs[0].Path, defs: defs}
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

// duplicateDefinitionProblem returns roles' own duplicate-definition WARN
// text when defs — a bare-name binding's own agentfile.Find results —
// names more than one project-scope definition, or "" when there is at
// most one, or when defs' own scope is ScopeUser: the duplicate WARN is
// project scope only, matching its own copy's "under .claude/agents".
// <rel> is root-relative and slash-separated; defs is already in lexical
// walk order, so the message's own path order follows it unchanged.
func duplicateDefinitionProblem(root, role, value string, defs []agentfile.Definition) string {
	if len(defs) < 2 || defs[0].Scope != agentfile.ScopeProject {
		return ""
	}

	rels := make([]string, 0, len(defs))

	for _, d := range defs {
		rel, err := filepath.Rel(root, d.Path)
		if err != nil {
			rel = d.Path
		}

		rels = append(rels, filepath.ToSlash(rel))
	}

	return fmt.Sprintf("%s: %s defined %d times under .claude/agents (%s)", role, value, len(defs), strings.Join(rels, ", "))
}

// rolesCheck builds roles' own row for a parsed config: every binding
// empty is SKIP "no roles bound"; any unbound position, unresolved
// binding, or bare-name binding with more than one project-scope
// definition is WARN, detail listing each problem in RoleBindings' own
// order ("planner unbound; reviewer: brief:reviewer not found"), never
// ERROR (roles are reported, not enforced); everything else bound and
// resolved is OK "planner, implementer, reviewer bound", with one suffix
// per role, in RoleBindings' own order: "user-level: <role>" for a bare
// name resolved only through the injected home, "not verified: <role>"
// for any other "<plugin>:<name>" binding.
func (s *Server) rolesCheck(root, nearest string, bindings [3]roleBinding) Check {
	if bindings[0].value == "" && bindings[1].value == "" && bindings[2].value == "" {
		return Check{ID: "roles", Severity: SeveritySkip, Path: nearest, Detail: "no roles bound", Fix: new(runInitWithAgents)}
	}

	var problems, suffixes []string

	for _, b := range bindings {
		res := s.resolveRoleBinding(root, b.value)

		switch res.state {
		case roleUnbound:
			problems = append(problems, b.name+" unbound")
		case roleUnresolved:
			problems = append(problems, fmt.Sprintf("%s: %s not found", b.name, b.value))
		case roleUnverified:
			suffixes = append(suffixes, "not verified: "+b.name)
		case roleResolved:
			if dup := duplicateDefinitionProblem(root, b.name, b.value, res.defs); dup != "" {
				problems = append(problems, dup)
			} else if len(res.defs) > 0 && res.defs[0].Scope == agentfile.ScopeUser {
				suffixes = append(suffixes, "user-level: "+b.name)
			}
		}
	}

	if len(problems) > 0 {
		fix := "bind each role to an existing agent in .brief.yaml, or " + runInitWithAgents

		return Check{ID: "roles", Severity: SeverityWarn, Path: nearest, Detail: strings.Join(problems, "; "), Fix: &fix}
	}

	verifiedDetail := make([]string, 0, 1+len(suffixes))
	verifiedDetail = append(verifiedDetail, "planner, implementer, reviewer bound")
	verifiedDetail = append(verifiedDetail, suffixes...)

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
