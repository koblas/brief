package artifact

import "strings"

// SnippetSpan locates one recognized marker block inside a candidate file's
// own bytes: [Start:End) is the span itself — from the first byte of the
// begin-marker line through the last byte of the end-marker text, excluding
// its own line terminator — and BeginLine is the begin marker's 1-based
// line number, the position a refusal citing this span reports.
type SnippetSpan struct {
	Start, End int
	BeginLine  int
}

// MarkerProblem is ScanSnippetMarkers' own refusal shape: Line is the
// 1-based line a caller-built refusal should cite, 0 for a defect (CRLF)
// that names no line — never produced by ScanSnippetMarkers itself, which
// reports only marker-ordering defects. Problem and Fix are that refusal's
// own copy.
type MarkerProblem struct {
	Line    int
	Problem string
	Fix     string
}

// ScanSnippetMarkers walks body's own lines looking for SnippetBegin and
// SnippetEnd marker lines — a whole line exactly equal to the marker — and
// reports exactly one of: a nil span and nil problem when body carries no
// marker at all; the one valid block's own SnippetSpan; or a MarkerProblem
// for the first marker defect it finds, in file order — a second begin
// marker (whether or not the first block was ever closed), a lone begin (no
// matching end before EOF), a lone end (no begin ever preceded it), or an
// end before any begin.
//
// CRLF line endings are not checked here: a CRLF file's own lines carry a
// trailing "\r" the LF-based marker constants can never exactly match, so
// this function reports it the same as body carrying no marker at all — a
// caller checks CRLF separately, scoped to the one candidate it is actually
// about to read or write.
func ScanSnippetMarkers(body []byte) (*SnippetSpan, *MarkerProblem) {
	var (
		beginLine, beginOffset int
		pendingEndLine         int
		open, done             bool
		found                  *SnippetSpan
	)

	offset := 0
	lineNo := 0

	for line := range strings.SplitSeq(string(body), "\n") {
		lineNo++
		lineStart := offset
		offset += len(line) + 1 // account for the "\n" split consumed, harmless past EOF

		switch line {
		case SnippetBegin:
			if open || done {
				return nil, &MarkerProblem{Line: lineNo, Problem: "a second brief:begin marker; a file may hold only one brief block", Fix: "delete the extra block"}
			}

			if pendingEndLine > 0 {
				return nil, &MarkerProblem{Line: pendingEndLine, Problem: "brief:end marker appears before any brief:begin", Fix: "reorder the markers, or remove them"}
			}

			beginLine = lineNo
			beginOffset = lineStart
			open = true
		case SnippetEnd:
			if !open {
				if pendingEndLine == 0 && !done {
					pendingEndLine = lineNo
				}

				continue
			}

			open = false
			done = true
			found = &SnippetSpan{Start: beginOffset, End: lineStart + len(line), BeginLine: beginLine}
		}
	}

	switch {
	case open:
		return nil, &MarkerProblem{Line: beginLine, Problem: "brief:begin marker with no matching brief:end", Fix: "add " + SnippetEnd + " after it, or remove the lone marker"}
	case done:
		return found, nil
	case pendingEndLine > 0:
		return nil, &MarkerProblem{Line: pendingEndLine, Problem: "brief:end marker with no matching brief:begin", Fix: "add " + SnippetBegin + " before it, or remove the lone marker"}
	default:
		return nil, nil
	}
}
