package host

import (
	"errors"
	"io"
)

// ErrMalformedPayload is returned by Host.HookPath when r is empty, not
// valid JSON, or decodes to a payload carrying no non-empty
// tool_input.file_path. Every caller treats it as a usage error, not a
// refusal.
var ErrMalformedPayload = errors.New("malformed hook payload")

// Host adapts one agent host's own hook protocol.
type Host interface {
	// Name reports the value check --hook <name> selects this Host with.
	Name() string
	// HookPath reads one hook-event payload from r and returns the edited
	// file's path exactly as the payload carries it — absolute or
	// relative, resolved by the caller, never by HookPath itself. It
	// returns ErrMalformedPayload when r is empty, not valid JSON, or the
	// payload carries no non-empty path.
	HookPath(r io.Reader) (string, error)
	// WriteHookContext writes summary to w as this host's own hook-context
	// response: content a hook returns so it reaches the host's model as
	// context rather than as an error.
	WriteHookContext(w io.Writer, summary string) error
}

// hookHosts lists every Host HookHosts and Lookup search, constructed once
// per call so neither exposes a package-level Host a caller could mutate.
func hookHosts() []Host {
	return []Host{claudeCode{}}
}

// HookHosts returns every host name check --hook accepts, in the order a
// usage error's "expected one of:" clause lists them.
func HookHosts() []string {
	hosts := hookHosts()
	names := make([]string, len(hosts))

	for i, h := range hosts {
		names[i] = h.Name()
	}

	return names
}

// Lookup returns the Host named name, and whether one was found.
func Lookup(name string) (Host, bool) {
	for _, h := range hookHosts() {
		if h.Name() == name {
			return h, true
		}
	}

	return nil, false
}
