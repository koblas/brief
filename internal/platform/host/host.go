package host

import (
	"errors"
	"io"

	"github.com/koblas/brief/internal/platform/artifact"
)

// ErrMalformedPayload is returned by Host.HookPath when r is empty, not
// valid JSON, or decodes to a payload carrying no non-empty
// tool_input.file_path. Every caller treats it as a usage error, not a
// refusal.
var ErrMalformedPayload = errors.New("malformed hook payload")

// File names one file Plugin, Agents or Skills lists: RelPath is its path
// relative to the repository's install root (forward-slash separated),
// Kind is the artifact.Kind whose Render and Recognize this file's bytes
// belong to, and Hook marks the file as the host's hook wiring — the one
// file Plugin(false) leaves out; Agents' and Skills' files never set it.
type File struct {
	RelPath string
	Kind    artifact.Kind
	Hook    bool
}

// Host adapts one agent host's hook protocol and skills-directory plugin
// layout.
type Host interface {
	// Name reports the value check --hook <name> selects this Host with.
	Name() string
	// HookPath reads one hook-event payload from r and returns the edited
	// file's path exactly as the payload carries it, resolved by the
	// caller, never by HookPath itself. It returns ErrMalformedPayload when
	// r is empty, not valid JSON, or the payload carries no non-empty path.
	HookPath(r io.Reader) (string, error)
	// WriteHookContext writes summary to w as this host's hook-context
	// response, so it reaches the host's model as context rather than as
	// an error.
	WriteHookContext(w io.Writer, summary string) error
	// Plugin returns every file this host's skills-directory plugin
	// installs, in the order Init writes them, with the hook file last when
	// withHook is true and omitted when false. Uninstall removes the same
	// files in reverse order.
	Plugin(withHook bool) []File
	// Agents returns the three role-agent files "init --with-agents"
	// installs, in write order: planner, implementer, reviewer. Unlike
	// Plugin, Uninstall always plans their removal regardless of flags.
	Agents() []File
	// Skills returns the one file the brief-workflow skill installs. Unlike
	// Plugin, it is written on every install for this host; unlike Agents
	// its file lives outside PluginDir, since any agent preloads it by bare
	// name rather than as a plugin entry. Uninstall always plans its
	// removal regardless of flags.
	Skills() []File
	// InstructionFiles returns the repository-root-relative paths of every
	// file this host reads project instructions from, in priority order.
	// It performs no filesystem access — a caller Lstats each in turn.
	InstructionFiles() []string
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
