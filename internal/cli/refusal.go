package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/scaffold"
)

// flattenOneLine collapses s to a single line: embedded newlines and runs
// of whitespace become one space each. yaml.v3 reports an unknown-key
// failure as "yaml: unmarshal errors:\n  line N: …", and R14's one-line
// contract requires every refusal to survive that unchanged.
func flattenOneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

// renderRefusal writes the R14a one-line refusal for err, prefixed by the
// failing command, to stderr, and returns err unchanged for ExitCode to
// classify.
//
// A *config.InvalidConfigError names the offending path and tells the user
// this refusal changed nothing on disk:
//
//	brief <command>: <path>: <problem>; fix it or remove it to fall back to the shipped defaults (no files changed)
//
// A *scaffold.RefusalError renders the same "nothing changed" promise
// around its own path, problem and fix, naming a line within the path
// when the refusal has one (R14a's "<path>[:<line>]"):
//
//	brief <command>: <path>[:<line>]: <problem>; <fix> (no files changed)
//
// Every other error renders as one flattened line:
//
//	brief <command>: <cause>
func renderRefusal(stderr io.Writer, cmd string, err error) error {
	if invalidCfg, ok := errors.AsType[*config.InvalidConfigError](err); ok {
		fmt.Fprintf(stderr, "brief %s: %s: %s; fix it or remove it to fall back to the shipped defaults (no files changed)\n",
			cmd, invalidCfg.Path, flattenOneLine(invalidCfg.Err.Error()))

		return err
	}

	if refusal, ok := errors.AsType[*scaffold.RefusalError](err); ok {
		path := refusal.Path
		if refusal.Line > 0 {
			path = fmt.Sprintf("%s:%d", refusal.Path, refusal.Line)
		}

		fmt.Fprintf(stderr, "brief %s: %s: %s; %s (no files changed)\n",
			cmd, path, flattenOneLine(refusal.Problem), flattenOneLine(refusal.Fix))

		return err
	}

	fmt.Fprintf(stderr, "brief %s: %s\n", cmd, flattenOneLine(err.Error()))

	return err
}
