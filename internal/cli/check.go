package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/koblas/brief/internal/assemble"
)

// checkLong is "brief check"'s help prose.
const checkLong = `Reports every fault in a feature's on-disk layout that finish would now
refuse to write over: caps, an unclosed fence, a missing heading, an
unticked checklist item on a done step, or a broken dependency, on a tree
that predates the tool or a raised cap. With no feature given, checks
every feature directory; naming one checks only that feature.

Findings print on stdout, one per line:

  [SEVERITY] <path>:<line> — <problem>

ERROR marks a fault on a feature still in flight (any step not done, or
unreadable); WARN marks the same fault on a feature whose every step is
done. brief check exits 1 when it prints any ERROR finding, 0 otherwise —
including a run that prints WARN findings only. A conforming repository,
or feature, prints nothing on stdout and exits 0, with one line on stderr
saying so. brief check reads; it never writes.`

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
		return renderRefusal(out.stderr, "check", err)
	}

	srv := assemble.NewServer(cfg, root)

	findings, err := srv.Check(ctx, feature)
	if err != nil {
		return renderRefusal(out.stderr, "check", err)
	}

	if len(findings) == 0 {
		fmt.Fprintln(out.stderr, "brief check: no findings")

		return nil
	}

	if err := assemble.RenderFindings(out.stdout, findings); err != nil {
		return fmt.Errorf("brief check: %w", err)
	}

	for _, f := range findings {
		if f.Severity == assemble.SeverityError {
			fmt.Fprintf(out.stderr, "brief check: %d finding(s), at least one ERROR\n", len(findings))

			return errCheckFindings
		}
	}

	fmt.Fprintf(out.stderr, "brief check: %d finding(s), no ERROR\n", len(findings))

	return nil
}
