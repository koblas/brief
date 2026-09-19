package stepfile

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidHandoffSuffix is returned by CompileHandoff when a
// handoff-file-suffix configuration value cannot be used to name a step's
// handoff file.
var ErrInvalidHandoffSuffix = errors.New("invalid handoff-file suffix")

// HandoffPattern is a compiled handoff-file-suffix: a step's Pattern paired
// with the suffix appended to Pattern.ID to name that step's handoff file.
type HandoffPattern struct {
	step   Pattern
	suffix string
}

// Name renders the handoff filename for step number n: step.ID(n) + suffix.
func (h HandoffPattern) Name(n int) string {
	return h.step.ID(n) + h.suffix
}

// collisionProbeNumbers are the step numbers CompileHandoff renders and
// compares when checking for a name collision. 1 and 2 are the pair a
// non-varying Pattern.ID (see the ID-varies check below) always collides
// on; 9, 10, 99, 100 additionally cross a digit-run width boundary.
var collisionProbeNumbers = []int{1, 2, 9, 10, 99, 100}

// CompileHandoff validates suffix against step and returns the compiled
// form, refusing with ErrInvalidHandoffSuffix when suffix:
//
//   - is empty;
//   - contains a path separator ("/" or "\"), which would name a
//     subdirectory instead of a flat file beside the step file;
//   - contains "%", which would make fmt.Sprintf-based rendering ambiguous;
//   - contains a digit — suffix "1.md" against step pattern "STEP-%d.md"
//     would render step 1's handoff as "STEP-11.md", which
//     Pattern.Number recognizes as step 11: a handoff file a directory
//     scan would misread as a step file;
//   - is paired with a step Pattern whose ID does not vary with the step
//     number — a step-file-pattern whose literal suffix carries no dot
//     but whose literal prefix does (e.g. ".%d") makes filepath.Ext
//     consume the entire rendered name at every step number, so
//     Pattern.ID renders the same string for every step. Checked by
//     comparing step.ID(1) against step.ID(2): whenever the two differ,
//     Pattern.ID embeds the step's own decimal digits verbatim and is
//     therefore injective for every step number, not only these two —
//     when they are equal, Pattern.ID is that same constant string for
//     every step number, which the pair already exposes. A handoff
//     pattern built on a non-varying ID would alias every step's handoff
//     onto one file, so this is refused before any name is rendered.
//   - renders a handoff filename that, compared case-insensitively,
//     equals the step's own filename at that step number, the configured
//     state filename, or the configured specification filename — any of
//     the three would make a write to the handoff file overwrite a file
//     finish depends on reading correctly. The step's own filename is
//     compared case-insensitively because the filesystems this repository
//     targets (Finish's own dev platform among them) are commonly
//     case-insensitive, so a suffix differing from the step's extension
//     only by case still names the same file on disk. The comparison is
//     rendered names, not literal suffix strings, because a step pattern
//     with more than one literal dot (e.g. "SCENARIO-%02d.step.md") makes
//     step.suffix ("...step.md") wider than the extension
//     filepath.Ext actually strips ("...md") when computing Pattern.ID;
//     comparing suffix strings misses that the rendered names still
//     collide. This holds for every step number once the ID-varies check
//     above has passed, because the rendered self-collision reduces to a
//     comparison of suffix against the step pattern's literal suffix, and
//     that literal is fixed. Collision against the state and specification
//     filenames is not provably invariant across step numbers in general,
//     so it is checked at the numbers in collisionProbeNumbers: every one
//     of them catches the failure this rule exists to prevent — a
//     non-varying Pattern.ID is already refused above, so any remaining
//     collision with a fixed reserved name can occur for at most one step
//     number, and checking several catches it regardless of which one.
func CompileHandoff(step Pattern, suffix, stateFile, specificationFile string) (HandoffPattern, error) {
	if suffix == "" {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	if strings.ContainsAny(suffix, `/\`) {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	if strings.Contains(suffix, "%") {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	if strings.ContainsFunc(suffix, func(r rune) bool { return r >= '0' && r <= '9' }) {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	if step.ID(1) == step.ID(2) {
		return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
	}

	handoff := HandoffPattern{step: step, suffix: suffix}

	for _, n := range collisionProbeNumbers {
		rendered := handoff.Name(n)

		if strings.EqualFold(rendered, step.Name(n)) ||
			strings.EqualFold(rendered, stateFile) ||
			strings.EqualFold(rendered, specificationFile) {
			return HandoffPattern{}, fmt.Errorf("%q: %w", suffix, ErrInvalidHandoffSuffix)
		}
	}

	return handoff, nil
}
