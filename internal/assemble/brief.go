package assemble

// Step is the next open step a Brief carries: its id, its title, and its
// acceptance-criteria and checklist sections, each already extracted from
// the step file and carrying the configured heading text a renderer needs
// alongside the body. The lowercase field names are RenderJSON's wire
// contract; no field is ever omitted.
type Step struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Acceptance Section `json:"acceptance"`
	Checklist  Section `json:"checklist"`
}

// Section is one section of a step or state file: the heading text
// configured for it, its body, and whether that heading was found at all.
// Body is empty both when the heading is present with nothing under it
// and when the heading is missing entirely — Found is what distinguishes
// the two; a caller that only renders non-empty bodies can ignore it, but
// a caller checking for a required heading cannot. The lowercase field
// names are RenderJSON's wire contract; found is never omitted, so an
// absent heading and an empty body stay distinguishable on the wire too.
type Section struct {
	Heading string `json:"heading"`
	Body    string `json:"body"`
	Found   bool   `json:"found"`
}

// Brief is everything Start assembles for one feature: the done/open
// counts across every step file, the next open step (nil when every step
// is done), every state-file section in the order the configuration
// requires them, and any optional convention Start found missing without
// refusing over it. The lowercase field names are RenderJSON's wire
// contract: no field carries omitempty, Step marshals to null rather than
// being dropped when there is no open step, and done/open are always
// present because a zero at either one is load-bearing — it is what
// separates a complete feature from one with no step files yet.
type Brief struct {
	Done       int         `json:"done"`
	Open       int         `json:"open"`
	Step       *Step       `json:"step"`
	Inherited  []Section   `json:"inherited"`
	Shortfalls []Shortfall `json:"shortfalls"`
}

// Shortfall names one optional convention Start found missing from a
// feature it still assembled a Brief for: an absent acceptance heading in
// the briefed step, or an absent heading in the state file. Path is the
// absolute path of the file the convention belongs to, Detail is what is
// missing, and Fix is the one-line remedy. Unlike Problem, a Shortfall
// never stops Start from returning a Brief — cli/start.go writes one
// stderr line per entry and still prints the brief on stdout. The
// lowercase field names are RenderJSON's wire contract.
type Shortfall struct {
	Path   string `json:"path"`
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
}
