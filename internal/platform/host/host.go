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
// relative to the repository's install root (forward-slash separated; a
// caller joins it with filepath.Join, which normalizes for its own
// platform), Kind is the artifact.Kind whose Render and Recognize this
// file's bytes belong to, and Hook marks the file as the host's own hook
// wiring — the one file Plugin(false) leaves out; Agents' and Skills' own
// files never set it.
type File struct {
	RelPath string
	Kind    artifact.Kind
	Hook    bool
}

// Host adapts one agent host's own hook protocol and skills-directory
// plugin layout.
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
	// Plugin returns every file this host's skills-directory plugin
	// installs, in the order Init writes them: the plugin manifest, then
	// each skill file, then the hook file last when withHook is true.
	// withHook false omits the hook file entirely, so it is planned as if
	// it did not exist — no row, nothing written. Uninstall removes the
	// same files in the reverse of this order.
	Plugin(withHook bool) []File
	// Agents returns the three role-agent files "init --with-agents"
	// installs under this host's own plugin directory, in the order Init
	// writes them: planner, implementer, reviewer. Unlike Plugin, Agents
	// carries no flag of its own — Uninstall always plans their removal,
	// independent of any flag Init was run with, and always in the reverse
	// of this order.
	Agents() []File
	// Skills returns the one file the brief-workflow skill installs (R1/R2):
	// unlike Plugin, it is written on every install for this host, with or
	// without WithAgents, and unlike Agents its file lives outside
	// PluginDir — it is a standalone project skill any agent preloads by
	// bare name, never an entry inside the "brief" plugin. Uninstall always
	// plans its removal, independent of any flag Init was run with, the
	// same as Agents.
	Skills() []File
	// InstructionFiles returns the repository-root-relative paths (forward
	// slash separated) of every file this host reads project instructions
	// from, in the priority order setup's own CLAUDE.md-block location rule
	// (R5) tries them in. It performs no filesystem access of its own — a
	// caller Lstats each in turn.
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
