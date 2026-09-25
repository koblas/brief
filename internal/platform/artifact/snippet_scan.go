package artifact

import "strings"

// SnippetSpan locates one recognized marker block inside a candidate
// file's bytes: [Start:End) runs from the first byte of the begin-marker
// line through the last byte of the end-marker text, excluding its line
// terminator. BeginLine is the begin marker's 1-based line number.
type SnippetSpan struct {
	Start, End int
	BeginLine  int
}

// MarkerProblem is ScanSnippetMarkers' refusal shape: Line is the 1-based
// line a caller-built refusal should cite, 0 for a defect naming no line.
// Problem and Fix are that refusal's copy.
type MarkerProblem struct {
	Line    int
	Problem string
	Fix     string
}

// ScanSnippetMarkers walks body's lines looking for SnippetBegin and
// SnippetEnd marker lines and reports exactly one of: nil, nil when body
// carries no marker at all; the one valid block's SnippetSpan; or a
// MarkerProblem for the first marker-ordering defect found in file order
// (a second begin, a lone begin, a lone end, or an end before any begin).
// CRLF endings are not checked here — a CRLF line never exactly matches
// the LF-based marker constants, so it is reported as no marker at all.
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
