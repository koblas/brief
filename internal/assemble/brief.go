package assemble

// Step is the next open step a Brief carries: its id, its title, and its
// acceptance-criteria and checklist sections, each already extracted from
// the step file and carrying the configured heading text a renderer needs
// alongside the body.
type Step struct {
	ID         string
	Title      string
	Acceptance Section
	Checklist  Section
}

// Section is one section of a step or state file: the heading text
// configured for it, its body, and whether that heading was found at all.
// Body is empty both when the heading is present with nothing under it
// and when the heading is missing entirely — Found is what distinguishes
// the two; a caller that only renders non-empty bodies can ignore it, but
// a caller checking for a required heading cannot.
type Section struct {
	Heading string
	Body    string
	Found   bool
}

// Brief is everything Start assembles for one feature: the done/open
// counts across every step file, the next open step (nil when every step
// is done), and every state-file section in the order the configuration
// requires them.
type Brief struct {
	Done      int
	Open      int
	Step      *Step
	Inherited []Section
}
