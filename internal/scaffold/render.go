package scaffold

import (
	"fmt"
	"strings"

	"github.com/koblas/brief/internal/platform/config"
)

// specificationSkeleton renders a new feature's specification.md: a title
// line, a blank line, and the configured progress heading — nothing under
// it, since no scenario work has happened yet.
func specificationSkeleton(cfg config.Config, name string) string {
	return fmt.Sprintf("# %s\n\n%s\n", name, cfg.ProgressHeading)
}

// stateSkeleton renders a new feature's state file: the four configured
// state headings, blank-line separated, with no title line. A title is
// deliberately omitted — SCENARIO-05 replaces the whole state body on
// "finish", and a title line would be silently dropped by that replacement.
func stateSkeleton(cfg config.Config) string {
	return strings.Join(cfg.StateHeadings.Ordered(), "\n\n") + "\n"
}
