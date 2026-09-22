package setup

import (
	"os"
	"path/filepath"
)

// WithHomeDir overrides the function Init calls to find the current user's
// home directory while resolving InitRequest.Host == "" (detectHost, R8).
// It defaults to os.UserHomeDir; a test injects a fixed directory so
// detection never depends on the developer's own "~/.claude".
func WithHomeDir(fn func() (string, error)) Option {
	return func(s *Server) { s.homeDir = fn }
}

// detectHost resolves InitRequest.Host == "" (R8): HostClaudeCode when root
// — config.LocateInRepo's own directory, the same root Init plans every
// artifact against, never wd itself when they differ — holds a ".claude" directory,
// or a "CLAUDE.md" entry of any type, or when home reports (without error)
// a directory whose own ".claude" is a directory; HostNone with
// detected=false otherwise. A home error, or home returning "", is treated
// the same as no home directory at all — never a refusal.
func detectHost(root string, home func() (string, error)) (string, bool) {
	if info, err := os.Stat(filepath.Join(root, ".claude")); err == nil && info.IsDir() {
		return HostClaudeCode, true
	}

	if _, err := os.Lstat(filepath.Join(root, "CLAUDE.md")); err == nil {
		return HostClaudeCode, true
	}

	if home != nil {
		if h, err := home(); err == nil && h != "" {
			if info, err := os.Stat(filepath.Join(h, ".claude")); err == nil && info.IsDir() {
				return HostClaudeCode, true
			}
		}
	}

	return HostNone, false
}
