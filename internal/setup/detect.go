package setup

import (
	"path/filepath"

	"github.com/koblas/brief/internal/platform/rwfs"
)

// WithHomeDir overrides the function detectHost calls to find the current
// user's home directory. It defaults to os.UserHomeDir.
func WithHomeDir(fn func() (string, error)) Option {
	return func(s *Server) { s.homeDir = fn }
}

// detectHost resolves InitRequest.Host == "": HostClaudeCode when root
// holds a ".claude" directory or a "CLAUDE.md" entry, or home's own
// ".claude" is a directory; HostNone otherwise. The third return value
// names the signal found (Result.DetectedBy), "" when detected is false.
func detectHost(fsys rwfs.FS, root string, home func() (string, error)) (string, bool, string) {
	if info, err := fsys.Stat(fsName(filepath.Join(root, ".claude"))); err == nil && info.IsDir() {
		return HostClaudeCode, true, ".claude"
	}

	if _, err := fsys.Lstat(fsName(filepath.Join(root, "CLAUDE.md"))); err == nil {
		return HostClaudeCode, true, "CLAUDE.md"
	}

	if home != nil {
		if h, err := home(); err == nil && h != "" {
			if info, err := fsys.Stat(fsName(filepath.Join(h, ".claude"))); err == nil && info.IsDir() {
				return HostClaudeCode, true, "~/.claude"
			}
		}
	}

	return HostNone, false, ""
}
