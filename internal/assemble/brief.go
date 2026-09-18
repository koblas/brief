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

// Section is one section of the feature's state file: the heading text
// configured for it, and its body. Body is empty when the state file has
// no matching heading; the section is still present so a caller — the
// text renderer or a JSON encoder — gets a stable set of keys regardless
// of what the state file happens to carry.
type Section struct {
	Heading string
	Body    string
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
