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

// fsName maps abs, an absolute OS path, onto the name an fs.FS expects:
// the leading path separator stripped, forward-slash separated, "." for
// the root itself.
func fsName(abs string) string {
	trimmed := strings.TrimPrefix(filepath.ToSlash(abs), "/")
	if trimmed == "" {
		return "."
	}

	return trimmed
}

// classifyProbeError classifies a failed fs.Lstat into the
// absent-vs-unreadable decision every probe in this package renders from:
// fs.ErrNotExist proves absence, and so does syscall.ENOTDIR — an
// ancestor path component is itself a regular file, so nothing can
// resolve through it; any other error is unreadable, its reason
// readFailureReason's own extracted cause.
func classifyProbeError(err error) (bool, string) {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return true, ""
	}

	return false, readFailureReason(err)
}

// integrationFileState is one Claude Code integration file's own probe
// result: present is false only when absent; unreadable is true when the
// file's Lstat or ReadFile call failed without proving absence; regular
// is true only for a present, readable file whose bytes were classified
// into origin.
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

// probeIntegrationFile Lstats root/f.RelPath through fsys and, for a
// regular file, reads and artifact.Recognizes its bytes against f.Kind. A
// ReadFile failure against a file Lstat resolved as regular is always
// unreadable, never reclassified as absent.
func probeIntegrationFile(fsys fs.FS, root string, f host.File) integrationFileState {
	path := filepath.Join(root, filepath.FromSlash(f.RelPath))
	state := integrationFileState{relPath: f.RelPath, path: path}

	info, err := fs.Lstat(fsys, fsName(path))
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

	body, err := fs.ReadFile(fsys, fsName(path))
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
func probeIntegrationFiles(fsys fs.FS, root string, files []host.File) []integrationFileState {
	out := make([]integrationFileState, 0, len(files))
	for _, f := range files {
		out = append(out, probeIntegrationFile(fsys, root, f))
	}

	return out
}

// anyPresent reports whether any state in states is present.
func anyPresent(states []integrationFileState) bool {
	for _, s := range states {
		if s.present {
			return true
		}
	}

	return false
}

// missingRelPaths returns the relPath of every state that is absent or
// not a regular file, excluding any unreadable state.
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

// unreadableRelPaths returns the relPath of every unreadable state.
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
// Fix once any of states holds an unreadable file: every unreadable file
// is named "not readable (<reason>)", reason taken from the first such
// state; any file still genuinely missing is named too, in its own
// fragment. ok is false when states holds no unreadable file at all.
func integrationFileRowDetail(fsys fs.FS, wd, root string, states []integrationFileState) (string, string, bool) {
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

	return strings.Join(fragments, "; "), notReadableFix(fsys, wd, root, first.path, first.statFailed), true
}

// relPathsWithOrigin returns the relPath of every present, regular state
// whose origin is origin.
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
// subject file — to the (Severity, Detail, Fix) triple every host row's
// "older render"/"edited locally"/"current" arms share: OriginOlder is
// WARN "installed by an older brief release" + suffix, fix olderFix;
// OriginEdited is OK "edited locally" + suffix; OriginCurrent (and any
// other value) is OK "installed", plain. suffix is appended verbatim.
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
// h.Agents() is present under root, the predicate host-plugin's,
// host-hook's and host-skill's own SKIP rows share.
func anyIntegrationFilePresent(fsys fs.FS, root string, h host.Host) bool {
	// h.Skills() is excluded: uninstall can leave an edited SKILL.md
	// behind after every other file is removed, and it is not itself
	// evidence of an install.
	return anyPresent(probeIntegrationFiles(fsys, root, h.Plugin(true))) || anyPresent(probeIntegrationFiles(fsys, root, h.Agents()))
}

// hostPluginCheck builds host-plugin's own row: not installed anywhere is
// SKIP; an unreadable, missing or non-regular subject file is ERROR
// "incomplete", naming every such file; an older render is WARN; an
// edited one is OK "edited locally"; every subject file current is OK
// "installed".
func hostPluginCheck(fsys fs.FS, wd, root string, h host.Host, installed bool) Check {
	path := filepath.Join(root, host.PluginDir)

	if !installed {
		return Check{ID: "host-plugin", Severity: SeveritySkip, Path: path, Detail: "not installed", Fix: new(runInitClaudeCode)}
	}

	states := probeIntegrationFiles(fsys, root, h.Plugin(false))

	if detail, fix, ok := integrationFileRowDetail(fsys, wd, root, states); ok {
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

// hookFileOf returns h.Plugin(true)'s trailing hook entry.
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
// from --no-hook; an unreadable hook file is WARN, naming the reason; a
// present but non-regular hook is ERROR; an older render is WARN; an
// edited one is OK "edited locally"; a current one is OK "installed".
func hostHookCheck(fsys fs.FS, wd, root string, h host.Host, installed bool) Check {
	hookFile := hookFileOf(h)
	state := probeIntegrationFile(fsys, root, hookFile)

	if !state.present {
		if !installed {
			return Check{ID: "host-hook", Severity: SeveritySkip, Path: state.path, Detail: "not installed", Fix: new(runInitClaudeCode)}
		}

		return Check{ID: "host-hook", Severity: SeverityWarn, Path: state.path, Detail: "installed without the check hook", Fix: new("run 'brief init' to add it")}
	}

	if state.unreadable {
		return Check{ID: "host-hook", Severity: SeverityWarn, Path: state.path, Detail: notReadableReason(state.reason), Fix: new(notReadableFix(fsys, wd, root, state.path, state.statFailed))}
	}

	if !state.regular {
		return Check{ID: "host-hook", Severity: SeverityError, Path: state.path, Detail: "not a regular file", Fix: new(runInit)}
	}

	sev, detail, fix := originRow(state.origin, runInit, "")

	return Check{ID: "host-hook", Severity: sev, Path: state.path, Detail: detail, Fix: fix}
}

// hostAgentsCheck builds host-agents' own row: none of the three role
// agent files present is SKIP; an unreadable, missing or non-regular one
// is WARN (never ERROR — an unbound role is reported by the roles row,
// not enforced here), naming every such file; an older render is WARN;
// an edited one is OK "edited locally"; all three current is OK
// "installed". Every fix names --with-agents, except an unreadable
// file's own shared chmod-then-reinit fix.
func hostAgentsCheck(fsys fs.FS, wd, root string, h host.Host) Check {
	agentFiles := h.Agents()
	path := filepath.Join(root, host.PluginDir, "agents")
	states := probeIntegrationFiles(fsys, root, agentFiles)

	if !anyPresent(states) {
		return Check{ID: "host-agents", Severity: SeveritySkip, Path: path, Detail: "not installed", Fix: new(runInitWithAgents)}
	}

	if detail, fix, ok := integrationFileRowDetail(fsys, wd, root, states); ok {
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

// snippetCandidateState is one CLAUDE.md candidate's own scan result:
// present is false only when absent; unreadable is true when the read
// failed without proving absence; notRegular and kind are populated only
// for a non-regular file; body and span are populated only for a
// regular, readable file scanned clean, prob carrying the first marker
// defect found otherwise. notRegular and unreadable are mutually
// exclusive.
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
// reports as not a regular file: a symlink (checked first, since a
// symlink to a directory reports both bits) or a directory; any other
// mode falls back to "".
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
// that is not a regular file: kind in parentheses when named, the bare
// sentence otherwise.
func notRegularDetail(kind string) string {
	if kind == "" {
		return "not a regular file; brief block not installed"
	}

	return fmt.Sprintf("not a regular file (%s); brief block not installed", kind)
}

// readFailureReason extracts the underlying reason behind a failed
// os.Lstat or os.ReadFile call: both fail only with a *fs.PathError, so
// this unwraps that error's own inner cause rather than the full
// "<op> <path>: …" text.
func readFailureReason(err error) string {
	pathErr, _ := errors.AsType[*fs.PathError](err)

	return pathErr.Err.Error()
}

// notReadableReason renders "not readable (<reason>)", the wording every
// host row shares for an unreadable subject.
func notReadableReason(reason string) string {
	return fmt.Sprintf("not readable (%s)", reason)
}

// notReadableDetail renders host-snippet's own WARN detail for an
// unreadable candidate: notReadableReason plus a note that the block
// could not be checked.
func notReadableDetail(reason string) string {
	return notReadableReason(reason) + "; cannot check for brief block"
}

// blockingDir walks from filepath.Dir(path) upward, Lstat'ing each
// ancestor until one resolves: that ancestor is the directory actually
// missing its own search (+x) bit. The walk never rises above root, and
// returns root itself when no ancestor below it resolves.
func blockingDir(fsys fs.FS, root, path string) string {
	// Lstat needs +x on a path's parent to find its directory entry,
	// never on the path itself, so an unsearchable directory still
	// resolves its own Lstat but blocks every Lstat nested inside it.
	dir := filepath.Dir(path)

	for dir != root {
		if _, err := fs.Lstat(fsys, fsName(dir)); err == nil {
			return dir
		}

		dir = filepath.Dir(dir)
	}

	return root
}

// notReadableFix renders the shared WARN fix for an unreadable subject:
// a failed Lstat targets blockingDir's own result with both bits restored
// (chmod u+rwx), since init must still create entries under it; a failed
// ReadFile targets the file itself (chmod +r). Both then re-run
// 'brief init --host claude-code'.
func notReadableFix(fsys fs.FS, wd, root, path string, statFailed bool) string {
	if statFailed {
		return fmt.Sprintf("chmod u+rwx %s, then %s", relPath(wd, blockingDir(fsys, root, path)), runInitClaudeCode)
	}

	return fmt.Sprintf("chmod +r %s, then %s", relPath(wd, path), runInitClaudeCode)
}

// scanSnippetCandidateStates Lstats and scans every h.InstructionFiles()
// candidate under root, in that order. A ReadFile failure against a
// candidate Lstat resolved as regular is always unreadable, never
// reclassified as absent. A candidate that exists but is not a regular
// file reports notRegular true and kind set, never read.
func scanSnippetCandidateStates(fsys fs.FS, root string, h host.Host) []snippetCandidateState {
	rel := h.InstructionFiles()
	out := make([]snippetCandidateState, 0, len(rel))

	for _, r := range rel {
		path := filepath.Join(root, filepath.FromSlash(r))

		info, err := fs.Lstat(fsys, fsName(path))
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

		body, err := fs.ReadFile(fsys, fsName(path))
		if err != nil {
			out = append(out, snippetCandidateState{path: path, present: true, unreadable: true, readErr: readFailureReason(err)})

			continue
		}

		span, prob := artifact.ScanSnippetMarkers(body)
		out = append(out, snippetCandidateState{path: path, present: true, body: body, span: span, prob: prob})
	}

	return out
}

// snippetBlockFound reports whether any state in states holds a
// recognized marker span.
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
// feature directory (dirKnown false only when ".brief.yaml" is
// unparseable). A marker defect in any candidate is ERROR, citing its
// path and line; two candidates each holding a span is ERROR on the
// second. Once neither ERROR arm fires, a candidate holding a span wins
// regardless of the other candidate's shape, classified by
// artifact.RecognizeSnippet: OriginEdited is OK "edited locally",
// OriginOlder is WARN, OriginCurrent is OK "installed" when its Dir
// matches dir and WARN naming both directories otherwise. Only once no
// candidate holds a span does existence matter, for the first candidate
// present at all: notRegular is WARN naming the shape; unreadable is WARN
// naming the reason. No candidate present, or the first present one is a
// regular, readable, blockless file, is SKIP "not installed".
func hostSnippetCheck(fsys fs.FS, wd, root string, states []snippetCandidateState, dir string, dirKnown bool) Check {
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
				Fix:    new(notReadableFix(fsys, wd, root, firstPresent.path, firstPresent.statFailed)),
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

// resolveRoleBinding classifies value against s.projectTree(root) and
// s.userTree() via agentfile.ResolveBindingIn.
func (s *Server) resolveRoleBinding(root, value string) agentfile.Binding {
	return agentfile.ResolveBindingIn(s.projectTree(root), s.userTree(), value)
}

// projectTree returns the agentfile.Tree a bare-name role binding's own
// search runs against for the project scope: s.rootFS()'s own subtree
// rooted at root, Dir set to root itself. A root fs.Sub that cannot
// resolve falls back to the zero FS.
func (s *Server) projectTree(root string) agentfile.Tree {
	sub, err := fs.Sub(s.rootFS(), fsName(root))
	if err != nil {
		return agentfile.Tree{Dir: root}
	}

	return agentfile.Tree{FS: sub, Dir: root}
}

// roleBinding names one role position alongside its own configured value.
type roleBinding struct {
	name, value string
}

// rolesCheckNoConfig builds roles' own row when no ".brief.yaml" was found
// anywhere.
func rolesCheckNoConfig() Check {
	return Check{ID: "roles", Severity: SeveritySkip, Detail: "no roles bound", Fix: new(runInitWithAgents)}
}

// rolesCheckUnparseable builds roles' own row when nearest exists but
// does not parse.
func rolesCheckUnparseable(nearest string) Check {
	return Check{ID: "roles", Severity: SeveritySkip, Path: nearest, Detail: ".brief.yaml did not parse"}
}

// duplicateDefinitionProblem returns roles' own duplicate-definition WARN
// text when defs names more than one project-scope definition, or "" when
// there is at most one, or when defs' scope is ScopeUser.
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
// binding, or duplicated bare-name binding is WARN, never ERROR; everything
// bound and resolved is OK, with one suffix per role: "user-level: <role>"
// for a bare name resolved only through the injected home, "not verified:
// <role>" for any other "<plugin>:<name>" binding.
func (s *Server) rolesCheck(root, nearest string, bindings [3]roleBinding) Check {
	if bindings[0].value == "" && bindings[1].value == "" && bindings[2].value == "" {
		return Check{ID: "roles", Severity: SeveritySkip, Path: nearest, Detail: "no roles bound", Fix: new(runInitWithAgents)}
	}

	var problems, suffixes []string

	for _, b := range bindings {
		res := s.resolveRoleBinding(root, b.value)

		switch res.State {
		case agentfile.BindingUnbound:
			problems = append(problems, b.name+" unbound")
		case agentfile.BindingUnresolved:
			problems = append(problems, fmt.Sprintf("%s: %s not found", b.name, b.value))
		case agentfile.BindingUnverified:
			suffixes = append(suffixes, "not verified: "+b.name)
		case agentfile.BindingResolved:
			if dup := duplicateDefinitionProblem(root, b.name, b.value, res.Defs); dup != "" {
				problems = append(problems, dup)
			} else if len(res.Defs) > 0 && res.Defs[0].Scope == agentfile.ScopeUser {
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

// hostSkillMissingDetail is host-skill's own WARN detail when the skill
// file is absent while some other integration file is present.
const hostSkillMissingDetail = "not installed; bound agents cannot preload it"

// skillFileOf returns h.Skills()'s own first entry, host-skill's subject
// file. Every Host today returns exactly one; if a second is ever added,
// this row reports only the first — it does not surface the rest.
func skillFileOf(h host.Host) host.File {
	files := h.Skills()
	if len(files) == 0 {
		return host.File{}
	}

	return files[0]
}

// hostSkillNotRegularFix is host-skill's own "not a regular file" ERROR
// fix: init never overwrites an existing path of the wrong kind, so the
// fix names removing it first.
const hostSkillNotRegularFix = "remove " + host.WorkflowSkillDir + "/SKILL.md, then " + runInit

// hostSkillRow builds host-skill's own row from state and installed,
// whether any Plugin(true) ∪ Agents() file is present: absent while
// nothing else is installed is SKIP "not installed"; absent while
// something else is present is WARN (hostSkillMissingDetail); unreadable
// is WARN; not a regular file is ERROR (hostSkillNotRegularFix); everything
// else — present and regular — goes through originRow.
func hostSkillRow(fsys fs.FS, wd, root string, state integrationFileState, installed bool) Check {
	if !state.present {
		if !installed {
			return Check{ID: "host-skill", Severity: SeveritySkip, Path: state.path, Detail: "not installed", Fix: new(runInitClaudeCode)}
		}

		return Check{ID: "host-skill", Severity: SeverityWarn, Path: state.path, Detail: hostSkillMissingDetail, Fix: new(runInit)}
	}

	if state.unreadable {
		return Check{ID: "host-skill", Severity: SeverityWarn, Path: state.path, Detail: notReadableReason(state.reason), Fix: new(notReadableFix(fsys, wd, root, state.path, state.statFailed))}
	}

	if !state.regular {
		return Check{ID: "host-skill", Severity: SeverityError, Path: state.path, Detail: "not a regular file", Fix: new(hostSkillNotRegularFix)}
	}

	sev, detail, fix := originRow(state.origin, runInit, "")

	return Check{ID: "host-skill", Severity: sev, Path: state.path, Detail: detail, Fix: fix}
}

// hostSkillCheck probes skillFileOf(h) under root and builds host-skill's
// own row from the result via hostSkillRow.
func hostSkillCheck(fsys fs.FS, wd, root string, h host.Host, installed bool) Check {
	return hostSkillRow(fsys, wd, root, probeIntegrationFile(fsys, root, skillFileOf(h)), installed)
}

// rolesSkillNoConfigDetail is roles-skill's own SKIP detail whenever no
// config was found, a found config did not parse, or neither planner nor
// implementer resolved to an agent this row could check.
const rolesSkillNoConfigDetail = "no bound planner or implementer brief can check"

// rolesSkillOKText renders roles-skill's own OK sentence for checked, the
// roles this row actually verified, in planner-then-implementer order:
// "planner, implementer preload brief-workflow" for both, "<role> preloads
// brief-workflow" for one. checked is never empty here.
func rolesSkillOKText(checked []string) string {
	if len(checked) == 1 {
		return fmt.Sprintf("%s preloads %s", checked[0], artifact.WorkflowSkillName)
	}

	return fmt.Sprintf("%s preload %s", strings.Join(checked, ", "), artifact.WorkflowSkillName)
}

// rolesSkillMissingFix is the WARN fix roles-skill shares across every
// case where at least one resolved role does not preload the skill.
const rolesSkillMissingFix = `add "brief-workflow" to the "skills:" list of each agent named, or run 'brief init --edit-agents' for those in the repository`

// rolesSkillCheckNoConfig builds roles-skill's own row when no
// ".brief.yaml" was found anywhere: SKIP, Fix nil.
func rolesSkillCheckNoConfig() Check {
	return Check{ID: "roles-skill", Severity: SeveritySkip, Detail: rolesSkillNoConfigDetail}
}

// rolesSkillCheckUnparseable builds roles-skill's own row when nearest
// exists but does not parse.
func rolesSkillCheckUnparseable(nearest string) Check {
	return Check{ID: "roles-skill", Severity: SeveritySkip, Path: nearest, Detail: rolesSkillNoConfigDetail}
}

// rolesSkillMissingEntry renders roles-skill's own WARN entry for one
// role whose resolved agent does not preload artifact.WorkflowSkillName,
// plus, when that agent's frontmatter sets omitClaudeMd: true, a suffix
// naming the consequence.
func rolesSkillMissingEntry(role, value string, omitClaudeMd bool) string {
	entry := fmt.Sprintf("%s: %s does not preload %s", role, value, artifact.WorkflowSkillName)
	if omitClaudeMd {
		entry += " and omits CLAUDE.md, so it never sees brief's instructions"
	}

	return entry
}

// roleLacksSkill reports whether res's resolved agent(s) do not preload
// artifact.WorkflowSkillName, and whether any of them sets
// omitClaudeMd: true.
func roleLacksSkill(res agentfile.Binding) (bool, bool) {
	defs := res.LackingSkill(artifact.WorkflowSkillName)
	if len(defs) == 0 {
		return false, false
	}

	for _, d := range defs {
		if d.Frontmatter.OmitClaudeMd {
			return true, true
		}
	}

	return true, false
}

// rolesSkillCheck builds roles-skill's own row for a parsed config,
// considering only planner and implementer: SKIP when neither resolves;
// any resolved role whose agent does not preload the skill is WARN, one
// entry per lacking role; otherwise OK, naming only the roles actually
// checked, plus "; not verified: <role>" per role bound to another
// plugin. A role left unbound or unresolved contributes neither, since
// the roles row already reports it.
func (s *Server) rolesSkillCheck(root, nearest string, planner, implementer roleBinding) Check {
	bindings := [2]roleBinding{planner, implementer}

	var (
		checked  []string
		problems []string
		suffixes []string
	)

	for _, b := range bindings {
		res := s.resolveRoleBinding(root, b.value)

		switch res.State {
		case agentfile.BindingUnverified:
			suffixes = append(suffixes, "not verified: "+b.name)
		case agentfile.BindingResolved:
			checked = append(checked, b.name)

			if lacking, omitClaudeMd := roleLacksSkill(res); lacking {
				problems = append(problems, rolesSkillMissingEntry(b.name, b.value, omitClaudeMd))
			}
		case agentfile.BindingUnbound, agentfile.BindingUnresolved:
			// The roles row already reports this position; roles-skill
			// has nothing of its own to add.
		}
	}

	if len(checked) == 0 {
		return Check{ID: "roles-skill", Severity: SeveritySkip, Path: nearest, Detail: rolesSkillNoConfigDetail}
	}

	if len(problems) > 0 {
		fix := rolesSkillMissingFix

		return Check{ID: "roles-skill", Severity: SeverityWarn, Path: nearest, Detail: strings.Join(problems, "; "), Fix: &fix}
	}

	detail := strings.Join(append([]string{rolesSkillOKText(checked)}, suffixes...), "; ")

	return Check{ID: "roles-skill", Severity: SeverityOK, Path: nearest, Detail: detail}
}

// runInit is the fix text repeated whenever a plain re-run repairs the
// finding.
const runInit = "run 'brief init'"

// runInitClaudeCode is the SKIP fix every "not installed" host row shares.
const runInitClaudeCode = "run 'brief init --host claude-code'"

// runInitPrintSnippet is host-snippet's own WARN fix for a CLAUDE.md
// candidate that is not a regular file: a re-run can never write through
// it, so the fix points at --print's manual-application path instead.
const runInitPrintSnippet = "run 'brief init --print' and add the CLAUDE.md block by hand"

// runInitWithAgents is the fix host-agents and roles share whenever the
// remedy needs --with-agents specifically.
const runInitWithAgents = "run 'brief init --with-agents'"
