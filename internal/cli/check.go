package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/koblas/brief/internal/assemble"
	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/host"
	"github.com/koblas/brief/internal/platform/rwfs"
)

// checkLong is "brief check"'s help prose.
var checkLong = `Reports every fault in a feature's on-disk layout that finish would now
refuse to write over: caps, an unclosed fence, a missing heading, an
unticked checklist item on a done step, or a broken dependency, on a tree
that predates the tool or a raised cap. With no feature given, checks
every feature directory; naming one checks only that feature.

Findings print on stdout, one feature group per feature that has findings:

  <name>  (in flight|complete)
    <SEVERITY>  <path>[:<line>]  <detail>

ERROR marks a fault on a feature still in flight (any step not done, or
unreadable); WARN marks the same fault on a feature whose every step is
done. One stderr line then counts ERROR and WARN findings, tallies each
rule id that fired, and, when relevant, says that ERRORs block finish or
that naming one feature narrows the report. brief check exits 1 when it
prints any ERROR finding, 0 otherwise — including a run that prints WARN
findings only. A conforming repository, or feature, prints nothing on
stdout and exits 0, with one line on stderr saying so. brief check reads;
it never writes.

--hook <host> reads one hook-event payload from stdin instead of taking a
feature argument, and checks only the feature containing the path that
payload names. With no ".brief.yaml" found, or a path outside the feature
directory, it is silent, exit 0. When the edited feature has at least one
ERROR finding, it writes one JSON document — the host's own hook-context
protocol, never a findings table — to stdout and exits 0; a host's hook
runner reads that as context, not as a failure. No ERROR finding is fully
silent, exit 0. --hook takes no feature argument and cannot be combined
with --json; an invalid ".brief.yaml" still refuses, exit 1.

` + jsonFieldsParagraph("counts", "features") + " " + jsonScriptHint

// errCheckFindings marks a run of runCheck that printed at least one
// ERROR-severity finding: ExitCode's default branch maps any non-nil,
// non-ErrUsage error to 1, so this sentinel needs no case of its own
// there. It carries no message: every user-facing line was already
// written by RenderFindings and the stderr summary below it.
var errCheckFindings = errors.New("check reported an error-severity finding")

// checkInvocation is the invocation string every "brief check" usage error
// names as how to fix it.
const checkInvocation = "brief check [feature]"

// checkHookInvocation is the invocation string every "check --hook" usage
// error names as how to fix it — a concrete example, since claude-code is
// the only host --hook accepts.
const checkHookInvocation = "brief check --hook claude-code"

// checkCountsJSON is checkDocument's "counts" member: the same
// countFindings tally checkSummary's text-mode line and runCheck's own
// exit decision both use, so exit_code == 1 iff counts.error > 0 by
// construction.
type checkCountsJSON struct {
	Error int `json:"error"`
	Warn  int `json:"warn"`
}

// checkFindingJSON is one checkFeatureJSON row's "findings" member:
// severity, rule, path and detail exactly as assemble.Finding carries
// them, raw and never flattened. Line is nil for a whole-file finding
// (Finding.Line == 0), else the integer.
type checkFindingJSON struct {
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	Path     string `json:"path"`
	Line     *int   `json:"line"`
	Detail   string `json:"detail"`
}

// checkFeatureJSON is one checkDocument "features" row: name, path and
// in_flight exactly as assemble.FeatureFindings carries them, plus that
// group's own findings in within-group order.
type checkFeatureJSON struct {
	Name     string             `json:"name"`
	Path     string             `json:"path"`
	InFlight bool               `json:"in_flight"`
	Findings []checkFindingJSON `json:"findings"`
}

// checkDocument is check's --json success document: the common header,
// then counts, then one row per feature with findings, no "data" wrapper.
// Findings render as this document's payload even when counts.error is
// greater than zero: check --json never renders an ERROR-carrying run as
// an error document.
type checkDocument struct {
	jsonHeader

	Counts   checkCountsJSON    `json:"counts"`
	Features []checkFeatureJSON `json:"features"`
}

// checkFeatures maps groups to checkDocument's "features" array: a sized,
// non-nil slice so zero groups encode as "[]" rather than "null".
func checkFeatures(groups []assemble.FeatureFindings) []checkFeatureJSON {
	out := make([]checkFeatureJSON, 0, len(groups))

	for _, g := range groups {
		findings := make([]checkFindingJSON, 0, len(g.Findings))

		for _, f := range g.Findings {
			var line *int
			if f.Line != 0 {
				l := f.Line
				line = &l
			}

			findings = append(findings, checkFindingJSON{
				Severity: string(f.Severity),
				Rule:     string(f.Rule),
				Path:     f.Path,
				Line:     line,
				Detail:   f.Detail,
			})
		}

		out = append(out, checkFeatureJSON{Name: g.Name, Path: g.Path, InFlight: g.InFlight, Findings: findings})
	}

	return out
}

// countFindings tallies groups' findings by severity: the one count
// checkSummary's text-mode line, checkDocument's "counts" and runCheck's
// own exit decision all use, so a run's ERROR/WARN totals can never
// disagree between the text and --json paths.
func countFindings(groups []assemble.FeatureFindings) (int, int) {
	var errorCount, warnCount int

	for _, g := range groups {
		for _, f := range g.Findings {
			if f.Severity == assemble.SeverityError {
				errorCount++
			} else {
				warnCount++
			}
		}
	}

	return errorCount, warnCount
}

// runCheck implements "brief check [feature] [--hook <host>]"; rest is its
// positional arguments, flags already parsed away. hookHost is "" unless
// --hook was given, in which case it dispatches to runCheckHook instead of
// checking by feature argument; stdin backs --hook's own payload read (no
// other check path reads it). runCheckHook never receives rootFS, since
// --hook stays real-disk-only regardless (see its own doc comment).
func runCheck(ctx context.Context, wd string, rest []string, hookHost string, stdin io.Reader, out reporter, rootFS rwfs.FS) error {
	if hookHost != "" {
		return runCheckHook(ctx, wd, rest, hookHost, stdin, out)
	}

	if len(rest) > 1 {
		return out.usageError(fmt.Sprintf("brief check: too many arguments; run '%s'", checkInvocation))
	}

	var feature string
	if len(rest) == 1 {
		feature = rest[0]
	}

	cfg, root, err := resolveRoot(rootFS, wd)
	if err != nil {
		return out.refusal(err)
	}

	srv := assemble.NewServer(cfg, root, assemble.WithFS(rootFS))

	findings, err := srv.Check(ctx, feature)
	if err != nil {
		return out.refusal(enrichUnknownFeature(ctx, cfg, root, feature, err, rootFS))
	}

	groups := assemble.GroupByFeature(findings)

	errorCount, warnCount := countFindings(groups)

	var runErr error
	if errorCount > 0 {
		runErr = errCheckFindings
	}

	// Decided before either the no-findings notice or displayFindings'
	// relativized copy, so a --json run writes zero stderr bytes and sees
	// Check's own absolute paths.
	if out.json {
		doc := checkDocument{
			jsonHeader: out.headerFor(ExitCode(runErr)),
			Counts:     checkCountsJSON{Error: errorCount, Warn: warnCount},
			Features:   checkFeatures(groups),
		}

		if err := out.document(doc); err != nil {
			return err
		}

		return runErr
	}

	if len(findings) == 0 {
		if feature != "" {
			fmt.Fprintf(out.stderr, "brief check: %s: no findings\n", feature)
		} else {
			fmt.Fprintln(out.stderr, "brief check: no findings")
		}

		return nil
	}

	if err := assemble.RenderFindings(out.stdout, displayFindings(wd, groups)); err != nil {
		return fmt.Errorf("brief check: %w", err)
	}

	fmt.Fprintf(out.stderr, "brief check: %s\n", checkSummary(groups, feature))

	return runErr
}

// errMalformedHookPayload marks a "check --hook" run whose stdin payload
// could not be parsed inside an opted-in repository: exit 1, a PostToolUse
// hook's own non-blocking failure, never usage-error's exit 2 — the
// payload is host-supplied, not user-typed, so a malformed one is a
// runtime fault rather than a misuse of the CLI.
var errMalformedHookPayload = errors.New("brief check: malformed hook payload on stdin")

// runCheckHook implements "brief check --hook <host>": rest must be empty
// and out.json must be false. It reads one hook-event payload from stdin
// through host, resolves the edited path against wd when relative, and
// checks only the feature assemble.(*Server).FeatureContaining reports
// for it. A feature with at least one ERROR finding writes one hook-context
// document (host.WriteHookContext) to stdout, exit 0; every other outcome
// — no opted-in config above wd, a malformed payload, a path outside any
// feature directory, or no ERROR findings — is silent or a plain refusal,
// never check's own findings table.
//
// The opt-in gate (config.LocateInRepo's nearest result, not
// resolveRoot/config.Resolve) runs before the payload is even parsed, so
// an unopted-in repository never reports a malformed payload for stdin it
// was never going to check.
//
// Unlike every other command in this package, runCheckHook takes no
// rootFS: FeatureContaining resolves the edited path through os.Lstat and
// filepath.EvalSymlinks, neither of which any seam replaces, so a --hook
// run always reads real disk.
func runCheckHook(ctx context.Context, wd string, rest []string, hookHost string, stdin io.Reader, out reporter) error {
	if len(rest) > 0 {
		return out.usageError(fmt.Sprintf("brief check: --hook takes no feature argument; run '%s'", checkHookInvocation))
	}

	if out.json {
		return out.usageError(fmt.Sprintf("brief check: --hook and --json cannot be combined; run '%s'", checkHookInvocation))
	}

	h, ok := host.Lookup(hookHost)
	if !ok {
		return out.usageError(fmt.Sprintf("brief check: unknown host %q; expected one of: %s; run '%s'", hookHost, strings.Join(host.HookHosts(), ", "), checkHookInvocation))
	}

	if nearest, _, locateErr := config.LocateInRepo(wd); locateErr == nil && nearest == "" {
		return nil
	}

	editedPath, err := h.HookPath(stdin)
	if err != nil {
		fmt.Fprintln(out.stderr, "brief check: malformed hook payload on stdin; expected a claude-code "+
			"PostToolUse payload with tool_input.file_path, nothing was checked")

		return errMalformedHookPayload
	}

	if !filepath.IsAbs(editedPath) {
		editedPath = filepath.Join(wd, editedPath)
	}

	cfg, root, err := resolveRoot(nil, wd)
	if err != nil {
		return out.refusal(err)
	}

	srv := assemble.NewServer(cfg, root)

	feature, ok := srv.FeatureContaining(editedPath)
	if !ok {
		return nil
	}

	findings, err := srv.Check(ctx, feature)
	if err != nil {
		return out.refusal(enrichUnknownFeature(ctx, cfg, root, feature, err, nil))
	}

	errorCount, _ := countFindings(assemble.GroupByFeature(findings))
	if errorCount == 0 {
		return nil
	}

	noun := "finding"
	if errorCount != 1 {
		noun = "findings"
	}

	featurePath := filepath.Join(root, cfg.FeatureDirectory, feature)
	summary := fmt.Sprintf("brief check: %s: %d ERROR %s; run 'brief check %s'", displayPath(wd, featurePath), errorCount, noun, feature)

	if err := h.WriteHookContext(out.stdout, summary); err != nil {
		return fmt.Errorf("brief check: %w", err)
	}

	return nil
}

// displayFindings returns a copy of groups with every Finding.Path
// relativized to wd through displayPath — a copy of both the group slice
// and each group's own findings slice, never a mutation of groups'
// backing arrays: check --json builds its own document from the
// un-relativized findings Check returned, and must see them unchanged.
func displayFindings(wd string, groups []assemble.FeatureFindings) []assemble.FeatureFindings {
	out := make([]assemble.FeatureFindings, len(groups))

	for i, g := range groups {
		findings := make([]assemble.Finding, len(g.Findings))

		for j, f := range g.Findings {
			f.Path = displayPath(wd, f.Path)
			findings[j] = f
		}

		out[i] = g
		out[i].Findings = findings
	}

	return out
}

// checkSummary renders runCheck's stderr summary line, minus the
// "brief check: " prefix: "N ERROR, M WARN in K feature(s) (<count> <rule>,
// …)", then a tail licensed by this run's own counts — "; ERRORs block
// finish on in-flight features" when at least one ERROR fired,
// "; 'brief check <feature>' narrows to one" when featureArg is "" and K
// (groups with findings) is more than 1, both joined "; <c1> — <c2>" when
// both apply, or no tail when neither does. ERROR and WARN counts are
// never pluralized; "feature"/"features" pluralizes on K. The rule tally
// is ordered by count descending, then rule id ascending — never map
// iteration order, which would make the summary flaky.
func checkSummary(groups []assemble.FeatureFindings, featureArg string) string {
	errorCount, warnCount := countFindings(groups)

	tally := make(map[assemble.Rule]int)

	for _, g := range groups {
		for _, f := range g.Findings {
			tally[f.Rule]++
		}
	}

	rules := make([]assemble.Rule, 0, len(tally))
	for r := range tally {
		rules = append(rules, r)
	}

	sort.Slice(rules, func(i, j int) bool {
		if tally[rules[i]] != tally[rules[j]] {
			return tally[rules[i]] > tally[rules[j]]
		}

		return rules[i] < rules[j]
	})

	parts := make([]string, 0, len(rules))
	for _, r := range rules {
		parts = append(parts, fmt.Sprintf("%d %s", tally[r], r))
	}

	noun := "features"
	if len(groups) == 1 {
		noun = "feature"
	}

	summary := fmt.Sprintf("%d ERROR, %d WARN in %d %s (%s)", errorCount, warnCount, len(groups), noun, strings.Join(parts, ", "))

	var clauses []string
	if errorCount > 0 {
		clauses = append(clauses, "ERRORs block finish on in-flight features")
	}

	if featureArg == "" && len(groups) > 1 {
		clauses = append(clauses, "'brief check <feature>' narrows to one")
	}

	if len(clauses) > 0 {
		summary += "; " + strings.Join(clauses, " — ")
	}

	return summary
}
