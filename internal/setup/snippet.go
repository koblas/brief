package setup

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/koblas/brief/internal/platform/artifact"
	"github.com/koblas/brief/internal/platform/host"
	"github.com/koblas/brief/internal/platform/rwfs"
)

// snippetSpan locates one recognized marker block inside a candidate
// file's bytes: [start:end) is the span, beginLine its 1-based line number.
type snippetSpan struct {
	start, end int
	beginLine  int
}

// toSnippetSpan converts an *artifact.SnippetSpan into a *snippetSpan, nil in, nil out.
func toSnippetSpan(s *artifact.SnippetSpan) *snippetSpan {
	if s == nil {
		return nil
	}

	return &snippetSpan{start: s.Start, end: s.End, beginLine: s.BeginLine}
}

// mergeSnippet computes the bytes a CLAUDE.md candidate should hold after
// installing block: a non-nil span replaces that span in place; otherwise
// block is appended with a separator removeSnippet can undo byte-for-byte.
func mergeSnippet(existing []byte, span *snippetSpan, block []byte) []byte {
	if len(existing) == 0 {
		return append(append([]byte{}, block...), '\n')
	}

	if span != nil {
		out := make([]byte, 0, len(existing)-(span.end-span.start)+len(block))
		out = append(out, existing[:span.start]...)
		out = append(out, block...)
		out = append(out, existing[span.end:]...)

		return out
	}

	if bytes.HasSuffix(existing, []byte("\n")) {
		out := make([]byte, 0, len(existing)+1+len(block)+1)
		out = append(out, existing...)
		out = append(out, '\n')
		out = append(out, block...)
		out = append(out, '\n')

		return out
	}

	out := make([]byte, 0, len(existing)+2+len(block))
	out = append(out, existing...)
	out = append(out, '\n', '\n')
	out = append(out, block...)

	return out
}

// removeSnippet computes the bytes remaining after dropping span from
// existing, undoing mergeSnippet's separator encoding.
func removeSnippet(existing []byte, span snippetSpan) []byte {
	before := existing[:span.start]
	after := existing[span.end:]

	terminated := bytes.HasPrefix(after, []byte("\n"))
	precededByBlank := bytes.HasSuffix(before, []byte("\n\n"))

	switch {
	case terminated && precededByBlank:
		out := make([]byte, 0, len(before)-1+len(after)-1)
		out = append(out, before[:len(before)-1]...)
		out = append(out, after[1:]...)

		return out
	case !terminated && precededByBlank:
		return append([]byte{}, before[:len(before)-2]...)
	case terminated:
		out := make([]byte, 0, len(before)+len(after)-1)
		out = append(out, before...)
		out = append(out, after[1:]...)

		return out
	default:
		out := make([]byte, 0, len(before)+len(after))
		out = append(out, before...)
		out = append(out, after...)

		return out
	}
}

// candidateSnippetFile is one CLAUDE.md candidate's scan result; body and
// span are populated only for a regular file.
type candidateSnippetFile struct {
	path    string
	exists  bool
	regular bool
	body    []byte
	span    *snippetSpan
}

// scanSnippetCandidates Lstats and scans every h.InstructionFiles()
// candidate under root for a marker defect, refusing on the first one
// found or on a valid block present in both.
func scanSnippetCandidates(fsys rwfs.FS, root string, h host.Host) ([]candidateSnippetFile, error) {
	rel := h.InstructionFiles()
	out := make([]candidateSnippetFile, 0, len(rel))

	for _, r := range rel {
		path := filepath.Join(root, filepath.FromSlash(r))

		info, err := fsys.Lstat(fsName(path))

		switch {
		case errors.Is(err, fs.ErrNotExist):
			out = append(out, candidateSnippetFile{path: path})

			continue
		case err != nil:
			return nil, fmt.Errorf("setup: lstat %s: %w", path, err)
		case !info.Mode().IsRegular():
			out = append(out, candidateSnippetFile{path: path, exists: true})

			continue
		}

		body, err := fsys.ReadFile(fsName(path))
		if err != nil {
			return nil, fmt.Errorf("setup: read %s: %w", path, err)
		}

		span, prob := artifact.ScanSnippetMarkers(body)
		if prob != nil {
			return nil, &RefusalError{Path: path, Line: prob.Line, Problem: prob.Problem, Fix: prob.Fix}
		}

		out = append(out, candidateSnippetFile{path: path, exists: true, regular: true, body: body, span: toSnippetSpan(span)})
	}

	if len(out) == 2 && out[0].span != nil && out[1].span != nil {
		return nil, &RefusalError{
			Path:    out[1].path,
			Line:    out[1].span.beginLine,
			Problem: "a brief block already exists in CLAUDE.md",
			Fix:     "delete that block",
		}
	}

	return out, nil
}

// chooseSnippetLocation picks the candidate Init writes to or Uninstall
// examines: one already holding a valid span wins, else the first that
// exists at all. ok is false when none exists.
func chooseSnippetLocation(candidates []candidateSnippetFile) (candidateSnippetFile, bool) {
	for _, c := range candidates {
		if c.span != nil {
			return c, true
		}
	}

	for _, c := range candidates {
		if c.exists {
			return c, true
		}
	}

	return candidateSnippetFile{}, false
}

// crlfRefusal reports c's *RefusalError if its bytes contain a CRLF line
// ending, nil otherwise. Checked only against the candidate a caller has
// resolved to act on, so an untouched CLAUDE.md on Windows never blocks
// an install or removal aimed at the other one.
func crlfRefusal(c candidateSnippetFile) error {
	if !c.regular || !bytes.Contains(c.body, []byte("\r\n")) {
		return nil
	}

	return &RefusalError{Path: c.path, Problem: "has CRLF line endings", Fix: "convert it to LF line endings"}
}

// snippetArtifact pairs one CLAUDE.md candidate's Artifact with the bytes
// apply needs to write or remove: existing/span are the pre-write state,
// dir is the feature directory to render, and remains (set only for
// ActionRemoved) is the bytes left after stripping the span, empty for a
// delete. notRegular marks the "not a regular file" branch that --print
// still shows the block to add by hand for.
type snippetArtifact struct {
	Artifact

	existing   []byte
	span       *snippetSpan
	dir        string
	remains    []byte
	notRegular bool
}

// planSnippet decides the CLAUDE.md instruction block's Artifact for Init:
// a chosen candidate with no block merges by appending; a current block
// for dir is unchanged, any other current block is merged "block updated",
// and an edited one is kept.
func planSnippet(fsys rwfs.FS, root string, h host.Host, dir string) (snippetArtifact, error) {
	candidates, err := scanSnippetCandidates(fsys, root, h)
	if err != nil {
		return snippetArtifact{}, err
	}

	chosen, ok := chooseSnippetLocation(candidates)
	if !ok {
		chosen = candidates[0]
	}

	if err := crlfRefusal(chosen); err != nil {
		return snippetArtifact{}, err
	}

	if !chosen.exists {
		return snippetArtifact{
			Kind: KindSnippet, Path: chosen.path, Action: ActionCreated,
			dir: dir,
		}, nil
	}

	if !chosen.regular {
		return snippetArtifact{
			Kind: KindSnippet, Path: chosen.path, Action: ActionKept,
			Detail: "not a regular file; add the block by hand, see 'brief init --print'",
			dir:    dir, notRegular: true,
		}, nil
	}

	if chosen.span == nil {
		return snippetArtifact{
			Kind: KindSnippet, Path: chosen.path, Action: ActionMerged,
			existing: chosen.body,
			dir:      dir,
		}, nil
	}

	match := artifact.RecognizeSnippet(chosen.body[chosen.span.start:chosen.span.end])

	if match.Origin != artifact.OriginCurrent {
		return snippetArtifact{
			Kind: KindSnippet, Path: chosen.path, Action: ActionKept, Detail: "edited locally",
		}, nil
	}

	if match.Dir == strings.TrimRight(dir, "/") {
		return snippetArtifact{
			Kind: KindSnippet, Path: chosen.path, Action: ActionUnchanged,
		}, nil
	}

	return snippetArtifact{
		Kind: KindSnippet, Path: chosen.path, Action: ActionMerged, Detail: "block updated",
		existing: chosen.body,
		span:     chosen.span,
		dir:      dir,
	}, nil
}

// planSnippetRemoval decides the CLAUDE.md instruction block's removal
// Artifact for Uninstall, present=false when there is nothing to report.
// An edited block is force-gated like planPluginRemoval; a removed one
// carries remains, the bytes left after stripping the span.
func planSnippetRemoval(fsys rwfs.FS, root string, h host.Host, force bool) (snippetArtifact, bool, error) {
	candidates, err := scanSnippetCandidates(fsys, root, h)
	if err != nil {
		return snippetArtifact{}, false, err
	}

	chosen, ok := chooseSnippetLocation(candidates)
	if !ok {
		return snippetArtifact{}, false, nil
	}

	if err := crlfRefusal(chosen); err != nil {
		return snippetArtifact{}, false, err
	}

	if !chosen.regular {
		return snippetArtifact{
			Kind: KindSnippet, Path: chosen.path, Action: ActionKept,
			Detail: "not a regular file",
		}, true, nil
	}

	if chosen.span == nil {
		return snippetArtifact{}, false, nil
	}

	match := artifact.RecognizeSnippet(chosen.body[chosen.span.start:chosen.span.end])

	if match.Origin != artifact.OriginCurrent && !force {
		return snippetArtifact{
			Kind: KindSnippet, Path: chosen.path, Action: ActionKept, Detail: "edited locally",
			ForceRemovable: true,
		}, true, nil
	}

	remains := removeSnippet(chosen.body, *chosen.span)
	detail := "brief block"
	if len(remains) == 0 {
		detail = ""
	}

	return snippetArtifact{
		Kind: KindSnippet, Path: chosen.path, Action: ActionRemoved, Detail: detail,
		existing: chosen.body,
		span:     chosen.span,
		remains:  remains,
	}, true, nil
}

// writeSnippetFile atomically replaces path's bytes with body. Unlike
// writePluginFile it never creates path's parent directory, since a
// chosen CLAUDE.md candidate's directory always already exists.
func writeSnippetFile(fsys rwfs.FS, path string, body []byte) error {
	return writeThrough(fsys, fsName(path), path, body)
}

// verifyFileUnchanged re-reads path immediately before apply writes to it
// and reports ErrConcurrentEdit unless its bytes still match what planning
// read, guarding every in-place rewrite against a change between planning
// and applying.
func verifyFileUnchanged(fsys rwfs.FS, path string, existedBefore bool, existing []byte, rerunCommand string) error {
	current, err := fsys.ReadFile(fsName(path))

	return verifyReadUnchanged(path, existedBefore, existing, current, err, rerunCommand)
}

// verifyReadUnchanged is the shared compare behind verifyFileUnchanged and
// verifyBoundAgentUnchanged. displayPath names the RefusalError and any
// wrapped read error; it need not be the path actually read.
func verifyReadUnchanged(displayPath string, existedBefore bool, existing, current []byte, readErr error, rerunCommand string) error {
	switch {
	case readErr == nil:
		if existedBefore && bytes.Equal(current, existing) {
			return nil
		}
	case errors.Is(readErr, fs.ErrNotExist):
		if !existedBefore {
			return nil
		}
	default:
		return fmt.Errorf("setup: read %s: %w", displayPath, readErr)
	}

	return &RefusalError{
		Path:    displayPath,
		Problem: "changed since it was planned",
		Fix:     "rerun '" + rerunCommand + "'",
		Err:     ErrConcurrentEdit,
	}
}
