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
// file's own bytes: [start:end) is the span itself — from the first byte of
// the begin-marker line through the last byte of the end-marker text,
// excluding its own line terminator — and beginLine is the begin marker's
// 1-based line number, the position a refusal citing this span reports.
type snippetSpan struct {
	start, end int
	beginLine  int
}

// toSnippetSpan converts an *artifact.SnippetSpan — artifact.ScanSnippetMarkers'
// own exported result — into setup's own private snippetSpan, nil in, nil
// out.
func toSnippetSpan(s *artifact.SnippetSpan) *snippetSpan {
	if s == nil {
		return nil
	}

	return &snippetSpan{start: s.Start, end: s.End, beginLine: s.BeginLine}
}

// mergeSnippet computes the bytes a CLAUDE.md candidate should hold after
// installing block: existing nil or empty (the file did not exist, or
// existed with zero bytes) produces block + "\n", the same as a fresh
// create. A non-nil span replaces that span in place, surrounding bytes
// untouched. Otherwise block is appended: existing ending in "\n" gets one
// "\n" before block and one "\n" after; existing not ending in "\n" gets a
// blank line ("\n\n") before block and nothing after — the asymmetry that
// lets removeSnippet undo either shape byte-for-byte.
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
// existing, undoing mergeSnippet's own separator encoding: a span
// terminated by "\n" (not at EOF) and preceded by "\n\n" drops one of the
// two preceding newlines, the span, and its own trailing newline; a span
// unterminated (at EOF) and preceded by "\n\n" drops both preceding
// newlines and the span; any other position — offset 0, or a block the
// user relocated next to their own prose — drops only the span and its own
// trailing newline, if present.
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

// candidateSnippetFile is one CLAUDE.md candidate's own scan result:
// exists/regular mirror an Lstat, body and span are populated only for a
// regular file (never for a missing or non-regular path, so a symlink is
// never followed to read one).
type candidateSnippetFile struct {
	path    string
	exists  bool
	regular bool
	body    []byte
	span    *snippetSpan
}

// scanSnippetCandidates Lstats and scans every h.InstructionFiles()
// candidate under root, in that order (root CLAUDE.md, then
// ".claude/CLAUDE.md"): a missing path reports exists=false; a path that
// exists but is not a regular file reports exists=true, regular=false,
// never read; a regular file is read and scanned for a marker defect
// (artifact.ScanSnippetMarkers) — a defect anywhere in either candidate refuses
// immediately, citing that candidate. Once both candidates are scanned
// clean, two of them each holding a valid block is refused too, citing
// ".claude/CLAUDE.md" (root is the preferred location) and its own begin
// line.
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

// chooseSnippetLocation picks the one candidate Init writes to or Uninstall
// examines for a block: a candidate already holding a valid span wins
// first (scanSnippetCandidates already refused if both do); otherwise the
// first candidate that exists at all, regular or not, in priority order.
// ok is false when no candidate exists — Init falls back to creating the
// first (root) candidate; Uninstall reports nothing installed.
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

// crlfRefusal reports c's own *RefusalError when its bytes contain a CRLF
// line ending, nil otherwise. It is checked only against the one candidate
// a caller has already resolved to act on — never blanket across both
// candidates — so an unrelated, untouched CLAUDE.md authored on Windows
// never blocks an install or removal aimed at the other one.
func crlfRefusal(c candidateSnippetFile) error {
	if !c.regular || !bytes.Contains(c.body, []byte("\r\n")) {
		return nil
	}

	return &RefusalError{Path: c.path, Problem: "has CRLF line endings", Fix: "convert it to LF line endings"}
}

// snippetArtifact pairs one CLAUDE.md candidate's own Artifact with the
// bytes apply needs to actually write or remove: existing and span are the
// candidate's own pre-write state (nil/nil for a fresh create), dir is the
// configured feature directory Init renders into a create or merge, and
// remains is the bytes Uninstall would leave behind after stripping the
// span — populated only when Action is ActionRemoved, so apply can tell a
// rewrite (remains non-empty) from a delete (remains empty) without
// recomputing it. notRegular marks planSnippet's own ActionKept "not a
// regular file" branch, the one ActionKept shape printArtifacts still
// emits a body for (R9): apply never writes through it either way, but
// unlike an edited-locally kept block, there is no existing content to
// preserve, so --print still shows the block to add by hand.
type snippetArtifact struct {
	Artifact

	existing   []byte
	span       *snippetSpan
	dir        string
	remains    []byte
	notRegular bool
}

// planSnippet decides the CLAUDE.md instruction block's own Artifact for
// Init (R5): scanSnippetCandidates locates and scans both
// host.InstructionFiles() candidates, refusing on any marker defect or a
// block in both before this function decides anything. The location is the
// candidate already holding a recognized block; otherwise the first
// existing candidate, regular or not; otherwise root CLAUDE.md is created.
// A chosen candidate carrying CRLF line endings refuses. A non-regular
// chosen candidate is kept, never followed — its own Artifact carries
// notRegular, the one ActionKept shape printArtifacts still emits a body
// for, so --print shows the block to add by hand even though apply never
// writes there. A regular candidate with no span merges by appending. A
// regular candidate whose span is
// artifact.RecognizeSnippet's OriginEdited is kept, "edited locally";
// OriginCurrent for dir (its own trailing "/" trimmed, matching
// SnippetBlock's own rule) is unchanged; OriginCurrent for any other
// directory is merged, "block updated".
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

// planSnippetRemoval decides the CLAUDE.md instruction block's own removal
// Artifact for Uninstall, present=false with a zero snippetArtifact when
// there is nothing to report: no candidate exists, or the chosen one exists
// but carries no recognized block. A chosen candidate carrying CRLF line
// endings refuses, the same as planSnippet. A non-regular chosen candidate
// is kept, plain "not a regular file" — unlike planSnippet's own longer
// install-side detail, there is no block to add by hand on a removal:
// Uninstall never suggests writing one. A regular candidate whose span is
// artifact.RecognizeSnippet's OriginEdited is kept, "edited locally",
// ForceRemovable true, unless force, in which case — like an OriginCurrent
// span always — it is removed: remains holds the bytes left after
// removeSnippet strips the span, and Detail is "brief block" when remains
// is non-empty (apply rewrites the file) or empty when remains is empty
// (apply deletes it).
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

// writeSnippetFile atomically replaces path's bytes with body, through
// fsys's own WriteFile (diskFS's own body: internal/platform/atomicfile),
// so a reader never observes a truncated or half-written CLAUDE.md,
// preserving the file's own mode across a replace. Unlike writePluginFile
// it never creates path's parent directory: a chosen CLAUDE.md candidate's
// own directory already exists by construction — root always does, and
// ".claude/CLAUDE.md" is only ever chosen when it already exists — so
// brief never creates ".claude/" itself just to hold this file.
func writeSnippetFile(fsys rwfs.FS, path string, body []byte) error {
	return writeThrough(fsys, fsName(path), path, body)
}

// verifyFileUnchanged re-reads path immediately before Init or Uninstall
// writes to it and reports ErrConcurrentEdit (wrapped in a *RefusalError,
// fix naming rerunCommand) unless its bytes still match exactly what
// planning read: existedBefore true and existing byte-identical to the
// current bytes, or existedBefore false and path still absent. A read
// failure other than "does not exist" is returned unwrapped — the same
// shape planSnippet's own reads use. This is apply's own read-modify-write
// guard, shared by every file apply rewrites in place rather than merely
// creating — the CLAUDE.md block and an ActionMerged plugin, skill or agent
// file (an OriginOlder render Rule 6 upgrades) alike: planning and applying
// are not atomic with respect to a concurrent brief invocation, or a person
// editing the file by hand, in between.
func verifyFileUnchanged(fsys rwfs.FS, path string, existedBefore bool, existing []byte, rerunCommand string) error {
	current, err := fsys.ReadFile(fsName(path))

	return verifyReadUnchanged(path, existedBefore, existing, current, err, rerunCommand)
}

// verifyReadUnchanged is verifyFileUnchanged's own shared compare —
// verifyBoundAgentUnchanged's own re-read goes through a different path
// (readBoundAgentFile, confined to resolvedRoot/rel rather than a raw
// os.ReadFile), but the comparison and refusal it renders from current,
// existing and readErr is identical either way. displayPath names the
// RefusalError and any wrapped read error; it need not be the path actually
// read.
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
