package setup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/atomicfile"
	"github.com/koblas/brief/internal/platform/config"
)

// boundAgentFrontmatterDelim is the line that opens and closes a bound
// agent file's own YAML frontmatter block — addWorkflowSkill's own copy of
// stepfile's private convention (stepfile.SetStatus), since stepfile does
// not export it.
const boundAgentFrontmatterDelim = "---"

// boundAgentOpenLen returns the number of leading bytes of s that make up
// an opening frontmatter delimiter line, "---\n" or "---\r\n" — 0 when s
// does not begin with either. addWorkflowSkill is only ever called on
// bytes a caller already resolved through agentfile.Find (Rule 5), whose
// frontmatter is already known to parse, so this is never 0 in practice.
func boundAgentOpenLen(s string) int {
	switch {
	case strings.HasPrefix(s, boundAgentFrontmatterDelim+"\r\n"):
		return len(boundAgentFrontmatterDelim) + 2
	case strings.HasPrefix(s, boundAgentFrontmatterDelim+"\n"):
		return len(boundAgentFrontmatterDelim) + 1
	default:
		return 0
	}
}

// skillsShape classifies addWorkflowSkill's own decision about a "skills:"
// key: shapeNoKey when body's own frontmatter carries no top-level
// "skills:" key at all — including one that is only nested under another
// key, or that only appears inside another key's block scalar, both of
// which are not top-level either; shapeBlockList and shapeFlowList for the
// two list forms Rule 4 can edit; shapeAlreadyListed when the caller
// already reported the skill present; shapeOther for every shape Rule 4
// leaves alone.
type skillsShape int

const (
	shapeNoKey skillsShape = iota
	shapeBlockList
	shapeFlowList
	shapeAlreadyListed
	shapeOther
)

// addWorkflowSkill edits existing's own "skills:" frontmatter key to add
// artifact.WorkflowSkillName, following Surface & Copy's own shape table.
// alreadyListed is caller-supplied — the same loose decode
// (Binding).LackingSkill uses (agentfile.Parse) — never decided from
// existing's own text, so an "already listed" row here never has to
// re-derive quoting or list-form nuance the text scanner below does not
// understand: it is returned unedited with shapeAlreadyListed regardless
// of the underlying shape. Otherwise the frontmatter is scanned, following
// stepfile.DecodeFrontmatter's own "\n---" prefix cut, for a top-level
// "skills:" line: none found (including one nested under another key, or
// inside another key's own block scalar — neither begins at column 0)
// inserts "skills: [<name>]" as a new line immediately before the closing
// delimiter (shapeNoKey); an empty value followed by a contiguous run of
// "- <scalar>" list items appends one more, matching the first item's own
// indent (shapeBlockList); a single-line "[...]" value is rewritten with
// the name appended, or replaces "[]" outright (shapeFlowList); any other
// shape — a scalar, "null", an empty list with no items, a multi-line or
// commented flow list, a mapping, or a block list holding a non-scalar
// item — is left untouched (shapeOther). The edited or rewritten line is
// returned with its own line terminator stripped; existing is returned
// unchanged for shapeAlreadyListed and shapeOther. A "skills:" line
// appearing after the closing delimiter, in the file's own body, is never
// considered or touched.
func addWorkflowSkill(existing []byte, alreadyListed bool) ([]byte, string, skillsShape) {
	if alreadyListed {
		return existing, "", shapeAlreadyListed
	}

	s := string(existing)
	openLen := boundAgentOpenLen(s)
	afterOpen := s[openLen:]
	yamlPart, afterClose, _ := strings.Cut(afterOpen, "\n"+boundAgentFrontmatterDelim)

	lines := strings.Split(yamlPart, "\n")

	keyIdx := findTopLevelSkillsKey(lines)

	var (
		newLines []string
		line     string
		shape    skillsShape
	)

	if keyIdx == -1 {
		newLines, line, shape = insertSkillsKey(lines)
	} else {
		newLines, line, shape = editSkillsKey(lines, keyIdx)
	}

	if shape == shapeOther {
		return existing, "", shapeOther
	}

	edited := s[:openLen] + strings.Join(newLines, "\n") + "\n" + boundAgentFrontmatterDelim + afterClose

	return []byte(edited), line, shape
}

// findTopLevelSkillsKey returns the index of lines' own top-level
// "skills:" line — one starting at column 0, never indented under another
// key or inside a block scalar — or -1 when there is none.
func findTopLevelSkillsKey(lines []string) int {
	const (
		prefix1 = "skills:"
		prefix2 = "skills: "
	)

	for i, raw := range lines {
		body := stripBoundAgentCR(raw)
		if body == "" || body[0] == ' ' || body[0] == '\t' {
			continue
		}

		if body == prefix1 || strings.HasPrefix(body, prefix2) {
			return i
		}
	}

	return -1
}

// insertSkillsKey appends a new "skills: [<name>]" line to lines, right
// before the closing delimiter, carrying the "\r" terminator of the line it
// follows when lines' own last line has one.
func insertSkillsKey(lines []string) ([]string, string, skillsShape) {
	last := lines[len(lines)-1]

	line := "skills: [" + artifact.WorkflowSkillName + "]"
	newLines := append(append([]string{}, lines...), line+boundAgentTerminator(last))

	return newLines, line, shapeNoKey
}

// editSkillsKey classifies lines[keyIdx]'s own "skills:" value and, for a
// block or flow list, performs the edit.
func editSkillsKey(lines []string, keyIdx int) ([]string, string, skillsShape) {
	body := stripBoundAgentCR(lines[keyIdx])
	value := strings.TrimSpace(strings.TrimPrefix(body, "skills:"))

	switch {
	case value == "":
		return editBlockList(lines, keyIdx)
	case strings.HasPrefix(value, "["):
		return editFlowList(lines, keyIdx, value)
	default:
		return lines, "", shapeOther
	}
}

// editBlockList appends a new "- <name>" item to lines' own contiguous
// block-list run starting at keyIdx+1, matching the first item's own
// indent and the last item's own line terminator.
func editBlockList(lines []string, keyIdx int) ([]string, string, skillsShape) {
	items, end, ok := boundAgentBlockListItems(lines, keyIdx+1)
	if !ok {
		return lines, "", shapeOther
	}

	indent := boundAgentLeadingIndent(lines[items[0]])
	lastItem := lines[items[len(items)-1]]

	line := indent + "- " + artifact.WorkflowSkillName
	newItem := line + boundAgentTerminator(lastItem)

	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:end]...)
	newLines = append(newLines, newItem)
	newLines = append(newLines, lines[end:]...)

	return newLines, line, shapeBlockList
}

// editFlowList rewrites lines[keyIdx]'s own single-line "[...]" value with
// the skill appended, preserving that line's own terminator.
func editFlowList(lines []string, keyIdx int, value string) ([]string, string, skillsShape) {
	rewritten, ok := rewriteFlowList(value)
	if !ok {
		return lines, "", shapeOther
	}

	line := "skills: " + rewritten

	newLines := append([]string{}, lines...)
	newLines[keyIdx] = line + boundAgentTerminator(lines[keyIdx])

	return newLines, line, shapeFlowList
}

// rewriteFlowList adds artifact.WorkflowSkillName to value, a "skills:"
// line's own trimmed, CR-stripped inline value: false when value does not
// end with "]" on this same line (a multi-line flow list, or one with a
// trailing comment) or holds a nested list or mapping ("[" or "{" inside
// the brackets) — either disqualifies the whole value as unrecognized
// (shapeOther).
func rewriteFlowList(value string) (string, bool) {
	if !strings.HasSuffix(value, "]") {
		return "", false
	}

	if value == "[]" {
		return "[" + artifact.WorkflowSkillName + "]", true
	}

	inner := value[1 : len(value)-1]
	if strings.ContainsAny(inner, "[]{}") {
		return "", false
	}

	return "[" + inner + ", " + artifact.WorkflowSkillName + "]", true
}

// boundAgentBlockListItems returns the line indices of the contiguous run
// of "- <scalar>" list items starting at start, and the index immediately
// after the last one. ok is false when there are none, or when any item's
// own content is not a plain scalar (a nested mapping or list) — the whole
// value is then shapeOther rather than surgically edited around unknown
// structure.
func boundAgentBlockListItems(lines []string, start int) (indices []int, end int, ok bool) {
	i := start

	for i < len(lines) {
		body := stripBoundAgentCR(lines[i])
		trimmed := strings.TrimLeft(body, " \t")

		if trimmed != "-" && !strings.HasPrefix(trimmed, "- ") {
			break
		}

		indices = append(indices, i)
		i++
	}

	if len(indices) == 0 {
		return nil, start, false
	}

	for _, idx := range indices {
		content := boundAgentItemContent(lines[idx])
		if !boundAgentItemIsScalar(content) {
			return nil, start, false
		}
	}

	return indices, i, true
}

// boundAgentItemContent returns a block-list item line's own content after
// its "- " (or bare "-") marker, CR-stripped.
func boundAgentItemContent(raw string) string {
	body := stripBoundAgentCR(raw)
	trimmed := strings.TrimLeft(body, " \t")
	content := strings.TrimPrefix(trimmed, "-")

	return strings.TrimPrefix(content, " ")
}

// boundAgentItemIsScalar reports whether a block-list item's own content is
// a plain scalar — not empty, not a nested flow list or mapping ("[" or
// "{"), and not an unquoted "key: value" mapping entry.
func boundAgentItemIsScalar(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	if strings.HasPrefix(s, "[") || strings.HasPrefix(s, "{") {
		return false
	}

	if idx := strings.Index(s, ":"); idx >= 0 && (idx+1 == len(s) || s[idx+1] == ' ') {
		return false
	}

	return true
}

// boundAgentLeadingIndent returns raw's own leading whitespace, CR-stripped.
func boundAgentLeadingIndent(raw string) string {
	body := stripBoundAgentCR(raw)
	trimmed := strings.TrimLeft(body, " \t")

	return body[:len(body)-len(trimmed)]
}

// boundAgentTerminator returns "\r" when raw — a line split only on "\n" —
// ends with a literal "\r" (a CRLF file), "" otherwise.
func boundAgentTerminator(raw string) string {
	if strings.HasSuffix(raw, "\r") {
		return "\r"
	}

	return ""
}

// stripBoundAgentCR removes a trailing "\r" from s, if present.
func stripBoundAgentCR(s string) string {
	return strings.TrimSuffix(s, "\r")
}

// removeWorkflowSkill edits existing's own "skills:" frontmatter key to
// remove artifact.WorkflowSkillName, addWorkflowSkill's own inverse
// (Surface & Copy, Rule 8): a block-list item exactly matching the plain
// scalar "brief-workflow" is dropped; when it was the run's only item, the
// "skills:" key line is dropped too, the same "drop when empty" rule a
// flow list follows — a single-entry or now-emptied flow list drops the
// whole "skills:" line, never rewritten as "skills: []". Every shape
// addWorkflowSkill would itself call shapeOther — a scalar, "null", a
// mapping, a quoted or commented item, a multi-line flow list, or a
// non-scalar block item — is unremovable (ok is false), the same as no
// top-level "skills:" key at all. A "skills:" line after the closing
// delimiter, in the file's own body, is never considered.
func removeWorkflowSkill(existing []byte) ([]byte, bool) {
	s := string(existing)
	openLen := boundAgentOpenLen(s)
	afterOpen := s[openLen:]
	yamlPart, afterClose, _ := strings.Cut(afterOpen, "\n"+boundAgentFrontmatterDelim)

	lines := strings.Split(yamlPart, "\n")

	keyIdx := findTopLevelSkillsKey(lines)
	if keyIdx == -1 {
		return existing, false
	}

	newLines, ok := removeSkillsKey(lines, keyIdx)
	if !ok {
		return existing, false
	}

	edited := s[:openLen] + strings.Join(newLines, "\n") + "\n" + boundAgentFrontmatterDelim + afterClose

	return []byte(edited), true
}

// removeSkillsKey removes artifact.WorkflowSkillName from lines[keyIdx]'s
// own "skills:" value, dispatching on the same shape editSkillsKey
// classifies: ok is false for every shape addWorkflowSkill would itself
// call shapeOther.
func removeSkillsKey(lines []string, keyIdx int) ([]string, bool) {
	body := stripBoundAgentCR(lines[keyIdx])
	value := strings.TrimSpace(strings.TrimPrefix(body, "skills:"))

	switch {
	case value == "":
		return removeBlockListItem(lines, keyIdx)
	case strings.HasPrefix(value, "["):
		return removeFlowListItem(lines, keyIdx, value)
	default:
		return lines, false
	}
}

// removeBlockListItem drops the block-list item at keyIdx+1.. exactly
// matching the plain scalar "brief-workflow", via boundAgentBlockListItems
// (the same run addWorkflowSkill's own editBlockList appends to). ok is
// false when there is no contiguous, all-scalar block-list run, or none of
// its items matches the skill by exact, unquoted text — a quoted
// "\"brief-workflow\"" item decodes identically for agentfile.Parse's own
// membership check, but is left alone here, unremovable. When the matched
// item was the run's only one, lines[keyIdx] (the "skills:" key line
// itself) is dropped along with it, never left as an empty key.
func removeBlockListItem(lines []string, keyIdx int) ([]string, bool) {
	items, end, ok := boundAgentBlockListItems(lines, keyIdx+1)
	if !ok {
		return lines, false
	}

	target := -1

	for _, idx := range items {
		if boundAgentItemContent(lines[idx]) == artifact.WorkflowSkillName {
			target = idx

			break
		}
	}

	if target == -1 {
		return lines, false
	}

	if len(items) == 1 {
		newLines := make([]string, 0, len(lines)-2)
		newLines = append(newLines, lines[:keyIdx]...)
		newLines = append(newLines, lines[end:]...)

		return newLines, true
	}

	newLines := make([]string, 0, len(lines)-1)
	newLines = append(newLines, lines[:target]...)
	newLines = append(newLines, lines[target+1:]...)

	return newLines, true
}

// removeFlowListItem rewrites lines[keyIdx]'s own single-line "[...]" value
// with the skill removed, via removeFromFlowList, preserving that line's
// own terminator — or, when removing it empties the list, drops the whole
// "skills:" line instead of rewriting it "skills: []".
func removeFlowListItem(lines []string, keyIdx int, value string) ([]string, bool) {
	rewritten, ok := removeFromFlowList(value)
	if !ok {
		return lines, false
	}

	if rewritten == "" {
		newLines := make([]string, 0, len(lines)-1)
		newLines = append(newLines, lines[:keyIdx]...)
		newLines = append(newLines, lines[keyIdx+1:]...)

		return newLines, true
	}

	newLines := append([]string{}, lines...)
	newLines[keyIdx] = "skills: " + rewritten + boundAgentTerminator(lines[keyIdx])

	return newLines, true
}

// removeFromFlowList removes artifact.WorkflowSkillName from value, a
// "skills:" line's own trimmed, CR-stripped inline value, mirroring
// rewriteFlowList's own disqualifiers: false when value does not end with
// "]" on this same line (a multi-line flow list, or one with a trailing
// comment), holds a nested list or mapping ("[" or "{" inside the
// brackets), or does not name the skill as a plain, unquoted,
// comma-separated token. rewritten is "" when removing the skill empties
// the list — removeFlowListItem then drops the whole "skills:" line rather
// than writing "skills: []", the same "drop when empty" rule
// removeBlockListItem follows for its own last item.
func removeFromFlowList(value string) (rewritten string, ok bool) {
	if !strings.HasSuffix(value, "]") {
		return "", false
	}

	inner := value[1 : len(value)-1]
	if strings.ContainsAny(inner, "[]{}") {
		return "", false
	}

	if strings.TrimSpace(inner) == "" {
		return "", false
	}

	items := strings.Split(inner, ",")
	kept := make([]string, 0, len(items))
	found := false

	for _, raw := range items {
		item := strings.TrimSpace(raw)
		if item == artifact.WorkflowSkillName && !found {
			found = true

			continue
		}

		kept = append(kept, item)
	}

	if !found {
		return "", false
	}

	if len(kept) == 0 {
		return "", true
	}

	return "[" + strings.Join(kept, ", ") + "]", true
}

// boundAgentArtifact pairs one bound-agent file's own Artifact with the
// bytes planning read (existing), the bytes a merge would write (edited),
// the inserted or rewritten line (line, terminator stripped — --print's
// own body), and the file's own Lstat'd permission bits (perm) — writing
// through them preserves the adopter's own mode rather than a fixed one.
// edited, line and perm are the zero value for every Action other than
// ActionMerged.
type boundAgentArtifact struct {
	Artifact

	existing []byte
	edited   []byte
	line     string
	perm     fs.FileMode
}

// boundAgentTargets selects --edit-agents' and Uninstall's own shared
// target set (Rule 3, Rule 4): resolvedRoot is root, symlinks resolved, for
// planBoundAgent's and planBoundAgentRemoval's own escape check; paths is
// every bare-name planner or implementer binding's own ScopeProject
// agentfile.Definition, deduped by path (the first role wins — planner is
// walked before implementer), in that order. A binding that is
// BindingBrief, BindingPlugin, unbound or unresolved contributes nothing —
// neither caller ever touches a "brief:*" binding, another plugin's, or one
// under "~/.claude" (Rule 3).
func boundAgentTargets(root, home string, roles config.RoleBindings) (resolvedRoot string, paths []string, err error) {
	resolvedRoot, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", nil, fmt.Errorf("setup: resolve %s: %w", root, err)
	}

	seen := make(map[string]bool)

	for _, r := range []string{roles.Planner, roles.Implementer} {
		b := agentfile.ResolveBinding(root, home, r)
		if b.Kind != agentfile.BindingBare {
			continue
		}

		for _, d := range b.Defs {
			if d.Scope != agentfile.ScopeProject || seen[d.Path] {
				continue
			}

			seen[d.Path] = true

			paths = append(paths, d.Path)
		}
	}

	return resolvedRoot, paths, nil
}

// planBoundAgents plans --edit-agents' own targets (Rule 3, Rule 4):
// boundAgentTargets' own selector, each planned through planBoundAgent.
func planBoundAgents(root, home string, roles config.RoleBindings) ([]boundAgentArtifact, error) {
	resolvedRoot, paths, err := boundAgentTargets(root, home, roles)
	if err != nil {
		return nil, err
	}

	var out []boundAgentArtifact

	for _, path := range paths {
		art, ok, planErr := planBoundAgent(path, resolvedRoot)
		if planErr != nil {
			return nil, planErr
		}

		if !ok {
			continue
		}

		out = append(out, art)
	}

	return out, nil
}

// planBoundAgentRemovals plans Uninstall's own bound-agent rows (Rule 8):
// boundAgentTargets' own selector decides the targets, each planned through
// planBoundAgentRemoval, in the reverse of the selector's own order —
// implementer before planner, the strict reversal of planBoundAgents' own
// planner-first walk, matching Uninstall's own reversed row order
// elsewhere (the agent files, the plugin files).
func planBoundAgentRemovals(root, home string, roles config.RoleBindings) ([]boundAgentArtifact, error) {
	resolvedRoot, paths, err := boundAgentTargets(root, home, roles)
	if err != nil {
		return nil, err
	}

	var out []boundAgentArtifact

	for _, path := range slices.Backward(paths) {
		art, ok, planErr := planBoundAgentRemoval(path, resolvedRoot)
		if planErr != nil {
			return nil, planErr
		}

		if !ok {
			continue
		}

		out = append(out, art)
	}

	return out, nil
}

// planBoundAgent plans one bound-agent target at path: a non-regular leaf
// (Lstat) is ActionKept, detail "not a regular file", never read; a
// regular leaf whose own resolved path escapes resolvedRoot (a ".claude"
// symlinked outside the repository — filepath.Rel starting with "..")
// contributes no row at all (ok is false), and is never edited; otherwise
// path is read once, membership decided via agentfile.Parse on that same
// byte slice, and addWorkflowSkill's own shape maps to ActionUnchanged
// (already listed), ActionKept (any other shape, Rule 4's own boundary),
// or ActionMerged (edited).
func planBoundAgent(path, resolvedRoot string) (boundAgentArtifact, bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return boundAgentArtifact{}, false, fmt.Errorf("setup: lstat %s: %w", path, err)
	}

	if !info.Mode().IsRegular() {
		return boundAgentArtifact{
			Artifact: Artifact{Kind: KindBoundAgent, Path: path, Action: ActionKept, Detail: "not a regular file"},
		}, true, nil
	}

	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return boundAgentArtifact{}, false, fmt.Errorf("setup: resolve %s: %w", path, err)
	}

	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return boundAgentArtifact{}, false, nil
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		return boundAgentArtifact{}, false, fmt.Errorf("setup: read %s: %w", path, err)
	}

	fm, parseErr := agentfile.Parse(existing)
	alreadyListed := parseErr == nil && slices.Contains(fm.Skills, artifact.WorkflowSkillName)

	edited, line, shape := addWorkflowSkill(existing, alreadyListed)

	art := Artifact{Kind: KindBoundAgent, Path: path}

	switch shape {
	case shapeAlreadyListed:
		art.Action = ActionUnchanged
	case shapeOther:
		art.Action = ActionKept
		art.Detail = "skills: is not a list brief can edit; add brief-workflow by hand"
	case shapeNoKey, shapeBlockList, shapeFlowList:
		art.Action = ActionMerged
		art.Detail = "brief-workflow added to skills"
	}

	return boundAgentArtifact{
		Artifact: art,
		existing: existing,
		edited:   edited,
		line:     line,
		perm:     info.Mode().Perm(),
	}, true, nil
}

// planBoundAgentRemoval plans one bound-agent target's own removal Artifact
// (Rule 8), mirroring planBoundAgent's own Lstat-then-resolve shape: a
// non-regular leaf, or a regular leaf whose own resolved path escapes
// resolvedRoot, contributes no row at all (ok is false) — unlike
// planBoundAgent's own ActionKept "not a regular file" row, Uninstall never
// reports a shape it cannot edit, only what it removed. Otherwise path is
// read once, membership decided via agentfile.Parse on that same byte
// slice (the same loose decode planBoundAgent uses); an entry not listed,
// or one removeWorkflowSkill reports unremovable (a quoted, commented or
// multi-line value, or any other shape addWorkflowSkill would call
// shapeOther), contributes no row either — Surface & Copy's own "no row for
// a shape it can't edit" rule. A removable, listed entry is ActionRemoved,
// carrying existing, edited and perm for applyUninstall's own write.
func planBoundAgentRemoval(path, resolvedRoot string) (boundAgentArtifact, bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return boundAgentArtifact{}, false, fmt.Errorf("setup: lstat %s: %w", path, err)
	}

	if !info.Mode().IsRegular() {
		return boundAgentArtifact{}, false, nil
	}

	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return boundAgentArtifact{}, false, fmt.Errorf("setup: resolve %s: %w", path, err)
	}

	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return boundAgentArtifact{}, false, nil
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		return boundAgentArtifact{}, false, fmt.Errorf("setup: read %s: %w", path, err)
	}

	fm, parseErr := agentfile.Parse(existing)
	listed := parseErr == nil && slices.Contains(fm.Skills, artifact.WorkflowSkillName)

	if !listed {
		return boundAgentArtifact{}, false, nil
	}

	edited, removable := removeWorkflowSkill(existing)
	if !removable {
		return boundAgentArtifact{}, false, nil
	}

	art := Artifact{Kind: KindBoundAgent, Path: path, Action: ActionRemoved, Detail: "brief-workflow from skills"}

	return boundAgentArtifact{
		Artifact: art,
		existing: existing,
		edited:   edited,
		perm:     info.Mode().Perm(),
	}, true, nil
}

// subtractMergedBoundAgents removes every path boundAgentArts merged from
// list, Result.AgentsMissingSkill's own post-run view (Surface & Copy):
// only a real edit ever drops an agent out of the report — a kept or
// unchanged row leaves it exactly as agentsMissingSkill already reported.
func subtractMergedBoundAgents(list []MissingSkillAgent, boundAgentArts []boundAgentArtifact) []MissingSkillAgent {
	if len(boundAgentArts) == 0 {
		return list
	}

	merged := make(map[string]bool, len(boundAgentArts))

	for _, ba := range boundAgentArts {
		if ba.Action == ActionMerged {
			merged[ba.Path] = true
		}
	}

	out := make([]MissingSkillAgent, 0, len(list))

	for _, a := range list {
		if !merged[a.Path] {
			out = append(out, a)
		}
	}

	return out
}

// writeBoundAgent atomically replaces path's bytes with body, through
// internal/platform/atomicfile, preserving perm (the file's own Lstat'd
// permission bits at planning time) rather than a fixed mode — unlike
// writePluginFile and writeSnippetFile, which hardcode 0o644 for a file
// brief itself owns, a bound agent file is the adopter's own and must keep
// whatever mode it already had.
func writeBoundAgent(path string, body []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("setup: open %s: %w", dir, err)
	}
	defer func() { _ = root.Close() }()

	w, err := atomicfile.Create(root, filepath.Base(path), perm)
	if err != nil {
		return fmt.Errorf("setup: write %s: %w", path, err)
	}

	if _, err := w.Write(body); err != nil {
		_ = w.Close()

		return fmt.Errorf("setup: write %s: %w", path, err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("setup: write %s: %w", path, err)
	}

	return nil
}
