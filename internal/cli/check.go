package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/koblas/brief/internal/assemble"
)

// checkLong is "brief check"'s help prose.
const checkLong = `Reports every fault in a feature's on-disk layout that finish would now
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
it never writes.`

// errCheckFindings marks a run of runCheck that printed at least one
// ERROR-severity finding: ExitCode's default branch maps any non-nil,
// non-ErrUsage error to 1, so this sentinel needs no case of its own
// there. It carries no message: every user-facing line was already
// written by RenderFindings and the stderr summary below it.
var errCheckFindings = errors.New("check reported an error-severity finding")

// checkInvocation is the invocation string every "brief check" usage error
// names as how to fix it.
const checkInvocation = "brief check [feature]"

// runCheck implements "brief check [feature]"; rest is its positional
// arguments, flags already parsed away.
func runCheck(ctx context.Context, wd string, rest []string, out reporter) error {
	if len(rest) > 1 {
		return out.usageError(fmt.Sprintf("brief check: too many arguments; run '%s'", checkInvocation))
	}

	var feature string
	if len(rest) == 1 {
		feature = rest[0]
	}

	cfg, root, err := resolveRoot(wd)
	if err != nil {
		return out.refusal(err)
	}

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(ctx, feature)
	if err != nil {
		return out.refusal(enrichUnknownFeature(ctx, cfg, root, feature, err))
	}

	if len(findings) == 0 {
		fmt.Fprintln(out.stderr, "brief check: no findings")

		return nil
	}

	groups := assemble.GroupByFeature(findings)

	if err := assemble.RenderFindings(out.stdout, displayFindings(wd, groups)); err != nil {
		return fmt.Errorf("brief check: %w", err)
	}

	fmt.Fprintf(out.stderr, "brief check: %s\n", checkSummary(groups, feature))

	for _, g := range groups {
		for _, f := range g.Findings {
			if f.Severity == assemble.SeverityError {
				return errCheckFindings
			}
		}
	}

	return nil
}

// displayFindings returns a copy of groups with every Finding.Path
// relativized to wd (R6) through displayPath — a copy of both the group
// slice and each group's own findings slice, never a mutation of groups'
// backing arrays: check --json (S09) builds its own document from the
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
	var errorCount, warnCount int

	tally := make(map[assemble.Rule]int)

	for _, g := range groups {
		for _, f := range g.Findings {
			tally[f.Rule]++

			if f.Severity == assemble.SeverityError {
				errorCount++
			} else {
				warnCount++
			}
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
