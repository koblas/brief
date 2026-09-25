package assemble

// Step is the next open step a Brief carries: its id, title, and its
// acceptance-criteria and checklist sections extracted from the step file.
// The lowercase field names are RenderJSON's wire contract.
type Step struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Acceptance Section `json:"acceptance"`
	Checklist  Section `json:"checklist"`
}

// Section is one section of a step or state file: the configured heading
// text, its body, and whether that heading was found at all. Body is empty
// both when the heading has nothing under it and when it is missing
// entirely; Found distinguishes the two.
type Section struct {
	Heading string `json:"heading"`
	Body    string `json:"body"`
	Found   bool   `json:"found"`
}

// Brief is everything Start assembles for one feature: the done/open
// counts across every step file, the next open step (nil when every step
// is done), every state-file section, and any optional convention Start
// found missing without refusing over it. No field carries omitempty, so a
// zero done/open count stays distinguishable from an absent one.
type Brief struct {
	Done       int         `json:"done"`
	Open       int         `json:"open"`
	Step       *Step       `json:"step"`
	Inherited  []Section   `json:"inherited"`
	Shortfalls []Shortfall `json:"shortfalls"`
}

// Shortfall names one optional convention Start found missing from a
// feature it still assembled a Brief for. Path is the file the convention
// belongs to, Detail is what is missing, and Fix is the one-line remedy.
// Unlike Problem, a Shortfall never stops Start from returning a Brief.
type Shortfall struct {
	Path   string `json:"path"`
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
}
