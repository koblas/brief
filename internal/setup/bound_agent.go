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

// boundAgentFrontmatterDelim is the line that opens and closes a bound agent file's YAML frontmatter block.
const boundAgentFrontmatterDelim = "---"

// boundAgentOpenLen returns the byte length of s's opening frontmatter
// delimiter line, "---\n" or "---\r\n", or 0 if s starts with neither.
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

// skillsShape classifies a "skills:" frontmatter key's editable shape.
type skillsShape int

const (
	shapeNoKey skillsShape = iota
	shapeBlockList
	shapeFlowList
	shapeAlreadyListed
	shapeOther
)

// addWorkflowSkill adds artifact.WorkflowSkillName to existing's "skills:"
// frontmatter key. alreadyListed skips the edit; a shape it cannot edit
// (shapeOther) is returned unchanged.
func addWorkflowSkill(existing []byte, alreadyListed bool) ([]byte, string, skillsShape) {
	if alreadyListed {
		return existing, "", shapeAlreadyListed
	}

	s := string(existing)
	openLen := boundAgentOpenLen(s)
	afterOpen := s[openLen:]
	yamlPart, afterClose, found := strings.Cut(afterOpen, "\n"+boundAgentFrontmatterDelim)

	if openLen == 0 || !found {
		return existing, "", shapeOther
	}

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

// findTopLevelSkillsKey returns the index of lines' top-level "skills:"
// line (starting at column 0), or -1 if there is none.
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
// before the closing delimiter.
func insertSkillsKey(lines []string) ([]string, string, skillsShape) {
	last := lines[len(lines)-1]

	line := "skills: [" + artifact.WorkflowSkillName + "]"
	newLines := append(append([]string{}, lines...), line+boundAgentTerminator(last))

	return newLines, line, shapeNoKey
}

// editSkillsKey classifies lines[keyIdx]'s "skills:" value and, for a block
// or flow list, performs the edit.
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

// editBlockList appends a new "- <name>" item to the contiguous block-list
// run starting at keyIdx+1, matching its indent and line terminator.
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

// editFlowList rewrites lines[keyIdx]'s single-line "[...]" value with the
// skill appended.
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
// line's trimmed inline list value. ok is false for a value that is not a
// well-formed single-line list.
func rewriteFlowList(value string) (string, bool) {
	if !strings.HasSuffix(value, "]") {
		return "", false
	}

	inner := value[1 : len(value)-1]
	if strings.ContainsAny(inner, "[]{}") {
		return "", false
	}

	trimmed := strings.TrimSpace(inner)
	if trimmed == "" {
		return "[" + artifact.WorkflowSkillName + "]", true
	}

	trimmed = strings.TrimRight(strings.TrimSuffix(trimmed, ","), " \t")

	return "[" + trimmed + ", " + artifact.WorkflowSkillName + "]", true
}

// boundAgentBlockListItems returns the line indices of the contiguous run
// of "- <scalar>" list items starting at start, and the index after the
// last one. ok is false if there are none or any item isn't a plain scalar.
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

// boundAgentItemContent returns a block-list item line's content after its
// "- " (or bare "-") marker.
func boundAgentItemContent(raw string) string {
	body := stripBoundAgentCR(raw)
	trimmed := strings.TrimLeft(body, " \t")
	content := strings.TrimPrefix(trimmed, "-")

	return strings.TrimPrefix(content, " ")
}

// boundAgentItemIsScalar reports whether a block-list item's content is a
// plain scalar, not a nested list, mapping, or "key: value" entry.
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

// boundAgentLeadingIndent returns raw's leading whitespace.
func boundAgentLeadingIndent(raw string) string {
	body := stripBoundAgentCR(raw)
	trimmed := strings.TrimLeft(body, " \t")

	return body[:len(body)-len(trimmed)]
}

// boundAgentTerminator returns "\r" if raw ends with one, "" otherwise.
func boundAgentTerminator(raw string) string {
	if strings.HasSuffix(raw, "\r") {
		return "\r"
	}

	return ""
}

// stripBoundAgentCR removes a trailing "\r" from s.
func stripBoundAgentCR(s string) string {
	return strings.TrimSuffix(s, "\r")
}

// removeWorkflowSkill removes artifact.WorkflowSkillName from existing's
// "skills:" frontmatter key, addWorkflowSkill's inverse. A key emptied by
// the removal is dropped rather than left as "skills: []".
func removeWorkflowSkill(existing []byte) ([]byte, bool) {
	s := string(existing)
	openLen := boundAgentOpenLen(s)
	afterOpen := s[openLen:]
	yamlPart, afterClose, found := strings.Cut(afterOpen, "\n"+boundAgentFrontmatterDelim)

	if openLen == 0 || !found {
		return existing, false
	}

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

// removeSkillsKey dispatches lines[keyIdx]'s "skills:" value to the block
// or flow-list remover, by the same shape editSkillsKey classifies.
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

// removeBlockListItem drops the block-list item matching
// artifact.WorkflowSkillName by exact, unquoted text from the run starting
// at keyIdx+1, dropping the "skills:" key line too if it was the only item.
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

// removeFlowListItem rewrites lines[keyIdx]'s single-line "[...]" value with
// the skill removed, dropping the whole "skills:" line if that empties it.
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
// "skills:" line's trimmed inline list value. rewritten is "" when the
// removal empties the list; ok is false if the skill isn't a plain,
// unquoted item of a well-formed single-line list.
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
		if item == "" {
			continue
		}

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

// boundAgentArtifact pairs a bound-agent file's Artifact with the bytes and
// symlink-resolved location needed to verify and apply its edit. edited,
// line, perm, resolvedRoot and rel are the zero value except for
// ActionMerged and ActionRemoved.
type boundAgentArtifact struct {
	Artifact

	existing     []byte
	edited       []byte
	line         string
	perm         fs.FileMode
	resolvedRoot string
	rel          string
}

// agentFile returns the confinedAgentFile used to re-read and apply ba's
// edit.
func (ba boundAgentArtifact) agentFile() confinedAgentFile {
	return confinedAgentFile{resolvedRoot: ba.resolvedRoot, rel: ba.rel, displayPath: ba.Path}
}

// boundAgentTargets resolves root via resolveRoot and returns the deduped,
// project-scoped agent file paths of the planner and implementer bindings
// that are bare names. A brief-bound, plugin, unbound, or unresolved
// binding contributes nothing.
func boundAgentTargets(resolveRoot func(string) (string, error), root, home string, roles config.RoleBindings) (resolvedRoot string, paths []string, err error) {
	resolvedRoot, err = resolveRoot(root)
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

// planBoundAgents plans an edit for each of boundAgentTargets' selected
// paths, via planBoundAgent.
func planBoundAgents(resolveRoot func(string) (string, error), root, home string, roles config.RoleBindings) ([]boundAgentArtifact, error) {
	resolvedRoot, paths, err := boundAgentTargets(resolveRoot, root, home, roles)
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

// planBoundAgentRemovals plans a removal for each of boundAgentTargets'
// selected paths, via planBoundAgentRemoval, in reverse order.
func planBoundAgentRemovals(resolveRoot func(string) (string, error), root, home string, roles config.RoleBindings) ([]boundAgentArtifact, error) {
	resolvedRoot, paths, err := boundAgentTargets(resolveRoot, root, home, roles)
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

// boundAgentUneditableDetail is the ActionKept detail for a "skills:" shape
// addWorkflowSkill cannot edit or verify.
const boundAgentUneditableDetail = "skills: is not a list brief can edit; add brief-workflow by hand"

// boundAgentNotRegularDetail is the ActionKept detail for a bound-agent leaf
// that is not a regular file.
const boundAgentNotRegularDetail = "not a regular file"

// relWithinRoot reports whether resolvedPath lies within resolvedRoot, both
// already symlink-resolved, returning its relative path.
func relWithinRoot(resolvedRoot, resolvedPath string) (rel string, ok bool) {
	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}

	return rel, true
}

// planBoundAgent plans one bound-agent target at path: a non-regular leaf
// is kept unread; one whose resolved path escapes resolvedRoot contributes
// no row. Otherwise its frontmatter is edited via addWorkflowSkill and
// verified via boundAgentEditVerified; an unparsable, already-listed, or
// unverified edit is kept unwritten rather than applied.
func planBoundAgent(path, resolvedRoot string) (boundAgentArtifact, bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return boundAgentArtifact{}, false, fmt.Errorf("setup: lstat %s: %w", path, err)
	}

	if !info.Mode().IsRegular() {
		return boundAgentArtifact{
			Artifact: Artifact{Kind: KindBoundAgent, Path: path, Action: ActionKept, Detail: boundAgentNotRegularDetail},
		}, true, nil
	}

	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return boundAgentArtifact{}, false, fmt.Errorf("setup: resolve %s: %w", path, err)
	}

	rel, ok := relWithinRoot(resolvedRoot, resolvedPath)
	if !ok {
		return boundAgentArtifact{}, false, nil
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		return boundAgentArtifact{}, false, fmt.Errorf("setup: read %s: %w", path, err)
	}

	art := Artifact{Kind: KindBoundAgent, Path: path}

	fm, parseErr := agentfile.Parse(existing)

	switch {
	case parseErr != nil:
		art.Action = ActionKept
		art.Detail = boundAgentUneditableDetail

		return boundAgentArtifact{Artifact: art}, true, nil
	case fm.HasSkill(artifact.WorkflowSkillName):
		art.Action = ActionUnchanged

		return boundAgentArtifact{Artifact: art}, true, nil
	}

	edited, line, shape := addWorkflowSkill(existing, false)

	if shape == shapeOther || !boundAgentEditVerified(fm, edited) {
		art.Action = ActionKept
		art.Detail = boundAgentUneditableDetail

		return boundAgentArtifact{Artifact: art}, true, nil
	}

	art.Action = ActionMerged
	art.Detail = "brief-workflow added to skills"

	return boundAgentArtifact{
		Artifact:     art,
		existing:     existing,
		edited:       edited,
		line:         line,
		perm:         info.Mode().Perm(),
		resolvedRoot: resolvedRoot,
		rel:          rel,
	}, true, nil
}

// boundAgentEditVerified reports whether edited decodes to fm's Skills with
// artifact.WorkflowSkillName appended and the same Name — the check a
// surgical text edit needs since it cannot otherwise prove its own YAML
// output correct.
func boundAgentEditVerified(fm agentfile.Frontmatter, edited []byte) bool {
	newFM, err := agentfile.Parse(edited)
	if err != nil || newFM.Name != fm.Name {
		return false
	}

	want := append(slices.Clone(fm.Skills), artifact.WorkflowSkillName)

	return slices.Equal(newFM.Skills, want)
}

// planBoundAgentRemoval plans one bound-agent target's removal, mirroring
// planBoundAgent: a non-regular or escaping leaf contributes no row.
// Otherwise the skill is removed via removeWorkflowSkill and verified via
// boundAgentRemovalVerified; an unlisted, unremovable, or unverified entry
// contributes no row either, only a removable one is ActionRemoved.
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

	rel, ok := relWithinRoot(resolvedRoot, resolvedPath)
	if !ok {
		return boundAgentArtifact{}, false, nil
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		return boundAgentArtifact{}, false, fmt.Errorf("setup: read %s: %w", path, err)
	}

	fm, parseErr := agentfile.Parse(existing)
	if parseErr != nil || !fm.HasSkill(artifact.WorkflowSkillName) {
		return boundAgentArtifact{}, false, nil
	}

	edited, removable := removeWorkflowSkill(existing)
	if !removable || !boundAgentRemovalVerified(fm, edited) {
		return boundAgentArtifact{}, false, nil
	}

	art := Artifact{Kind: KindBoundAgent, Path: path, Action: ActionRemoved, Detail: "brief-workflow from skills"}

	return boundAgentArtifact{
		Artifact:     art,
		existing:     existing,
		edited:       edited,
		perm:         info.Mode().Perm(),
		resolvedRoot: resolvedRoot,
		rel:          rel,
	}, true, nil
}

// boundAgentRemovalVerified reports whether edited decodes to fm's Skills
// with exactly the artifact.WorkflowSkillName entries removed and the same
// Name, boundAgentEditVerified's removal-side twin.
func boundAgentRemovalVerified(fm agentfile.Frontmatter, edited []byte) bool {
	newFM, err := agentfile.Parse(edited)
	if err != nil || newFM.Name != fm.Name || newFM.HasSkill(artifact.WorkflowSkillName) {
		return false
	}

	want := make([]string, 0, len(fm.Skills))

	for _, s := range fm.Skills {
		if s != artifact.WorkflowSkillName {
			want = append(want, s)
		}
	}

	return slices.Equal(newFM.Skills, want)
}

// subtractMergedBoundAgents removes every path merged in boundAgentArts
// from list.
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

// confinedAgentFile reads and writes a bound-agent file through an os.Root
// confined to resolvedRoot/rel, so a directory symlink re-pointed between
// planning and apply cannot redirect either call outside the resolved root.
// displayPath is the leaf's original, possibly-symlinked location and names
// every returned error.
type confinedAgentFile struct {
	resolvedRoot string
	rel          string
	displayPath  string
}

// read reads c's file through an os.Root confined to c.resolvedRoot/c.rel.
// The returned error is unwrapped so os.IsNotExist still classifies it,
// except an os.OpenRoot failure, wrapped with c.displayPath.
func (c confinedAgentFile) read() ([]byte, error) {
	root, err := os.OpenRoot(c.resolvedRoot)
	if err != nil {
		return nil, fmt.Errorf("setup: open %s: %w", c.displayPath, err)
	}
	defer func() { _ = root.Close() }()

	return root.ReadFile(c.rel)
}

// write atomically replaces c's file via atomicfile, confined to
// c.resolvedRoot/c.rel, preserving perm rather than a fixed mode since a
// bound agent file belongs to the adopter, not brief.
func (c confinedAgentFile) write(body []byte, perm fs.FileMode) error {
	root, err := os.OpenRoot(c.resolvedRoot)
	if err != nil {
		return fmt.Errorf("setup: open %s: %w", c.displayPath, err)
	}
	defer func() { _ = root.Close() }()

	dir := filepath.Dir(c.rel)
	if dir != "." {
		sub, subErr := root.OpenRoot(dir)
		if subErr != nil {
			return fmt.Errorf("setup: open %s: %w", c.displayPath, subErr)
		}
		defer func() { _ = sub.Close() }()

		root = sub
	}

	name := filepath.Base(c.rel)

	w, err := atomicfile.Create(root, name, perm)
	if err != nil {
		return fmt.Errorf("setup: write %s: %w", c.displayPath, err)
	}

	if _, err := w.Write(body); err != nil {
		_ = w.Close()

		return fmt.Errorf("setup: write %s: %w", c.displayPath, err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("setup: write %s: %w", c.displayPath, err)
	}

	return nil
}

// verifyBoundAgentUnchanged re-reads ba's file through ba.agentFile() and
// compares it to ba.existing before that same file is overwritten. A read
// refused by the confinement (boundAgentPathEscaped) is reported as a
// concurrent edit rather than a bare error.
func verifyBoundAgentUnchanged(ba boundAgentArtifact, rerunCommand string) error {
	current, err := ba.agentFile().read()
	if err != nil && !os.IsNotExist(err) && boundAgentPathEscaped(ba.Path, ba.resolvedRoot, ba.rel) {
		return &RefusalError{
			Path:    ba.Path,
			Problem: "changed since it was planned",
			Fix:     "rerun '" + rerunCommand + "'",
			Err:     ErrConcurrentEdit,
		}
	}

	return verifyReadUnchanged(ba.Path, true, ba.existing, current, err, rerunCommand)
}

// boundAgentPathEscaped reports whether displayPath no longer resolves to
// resolvedRoot/rel. A resolve failure other than not-exist is not treated
// as escaped, since it is a genuine error rather than a re-point.
func boundAgentPathEscaped(displayPath, resolvedRoot, rel string) bool {
	resolved, err := filepath.EvalSymlinks(displayPath)
	if err != nil {
		return os.IsNotExist(err)
	}

	return resolved != filepath.Join(resolvedRoot, rel)
}
