package setup_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/koblas/brief/internal/setup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedHome returns a setup.Option pinning WithHomeDir to a function that
// always reports home, nil — the injected home every detection test uses
// instead of the developer's own os.UserHomeDir.
func fixedHome(home string) setup.Option {
	return setup.WithHomeDir(func() (string, error) { return home, nil })
}

// errHomeLookup is the static error the "home func returns an error" case
// injects, so detectHost's own home() call fails the way a real
// os.UserHomeDir failure would.
var errHomeLookup = errors.New("home lookup failed")

// Test_init_detects_claude_code_from_the_install_root_or_home pins R8's
// detection rule (InitRequest.Host == ""): a root ".claude" directory, a
// root "CLAUDE.md" entry of any type, or an injected home's own ".claude"
// directory all resolve to HostClaudeCode with Result.NoHostDetected
// false; nothing anywhere resolves to HostNone with NoHostDetected true;
// an explicit "none" is never overridden by detection, and a home()
// failure is treated the same as no home directory at all, never a
// refusal.
func Test_init_detects_claude_code_from_the_install_root_or_home(t *testing.T) {
	homeWithClaude := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(homeWithClaude, ".claude"), 0o755))

	cases := []struct {
		name               string
		seedRoot           func(t *testing.T, root string)
		home               setup.Option
		reqHost            string
		wantHost           string
		wantNoHostDetected bool
	}{
		{
			name: "root .claude directory",
			seedRoot: func(t *testing.T, root string) {
				t.Helper()
				require.NoError(t, os.Mkdir(filepath.Join(root, ".claude"), 0o755))
			},
			home:               fixedHome(""),
			wantHost:           setup.HostClaudeCode,
			wantNoHostDetected: false,
		},
		{
			name: "root CLAUDE.md file",
			seedRoot: func(t *testing.T, root string) {
				t.Helper()
				require.NoError(t, os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("hi"), 0o600))
			},
			home:               fixedHome(""),
			wantHost:           setup.HostClaudeCode,
			wantNoHostDetected: false,
		},
		{
			name:               "home .claude directory, root has neither",
			seedRoot:           func(*testing.T, string) {},
			home:               fixedHome(homeWithClaude),
			wantHost:           setup.HostClaudeCode,
			wantNoHostDetected: false,
		},
		{
			name:               "nothing present",
			seedRoot:           func(*testing.T, string) {},
			home:               fixedHome(""),
			wantHost:           setup.HostNone,
			wantNoHostDetected: true,
		},
		{
			name: "explicit host none with .claude present",
			seedRoot: func(t *testing.T, root string) {
				t.Helper()
				require.NoError(t, os.Mkdir(filepath.Join(root, ".claude"), 0o755))
			},
			home:               fixedHome(""),
			reqHost:            setup.HostNone,
			wantHost:           setup.HostNone,
			wantNoHostDetected: false,
		},
		{
			name:     "home func returns an error",
			seedRoot: func(*testing.T, string) {},
			home: setup.WithHomeDir(func() (string, error) {
				return "", errHomeLookup
			}),
			wantHost:           setup.HostNone,
			wantNoHostDetected: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := t.TempDir()
			c.seedRoot(t, wd)
			srv := setup.NewServer(c.home)

			res, err := srv.Init(t.Context(), wd, setup.InitRequest{Host: c.reqHost})

			require.NoError(t, err)
			assert.Equal(t, c.wantHost, res.Host)
			assert.Equal(t, c.wantNoHostDetected, res.NoHostDetected)
		})
	}
}

// Test_init_detection_uses_the_locate_root_not_wd pins that detection is
// keyed on config.Locate's own root, not the working directory Init was
// called with: a ".claude" directory sitting only beside the working
// directory itself — never above the located root — must not be found,
// while one beside the located config wins even when wd is a subdirectory
// of it.
func Test_init_detection_uses_the_locate_root_not_wd(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".brief.yaml"), []byte("feature-directory: specs\n"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".claude"), 0o755))
	child := filepath.Join(root, "child")
	require.NoError(t, os.Mkdir(child, 0o755))
	srv := setup.NewServer(fixedHome(""))

	res, err := srv.Init(t.Context(), child, setup.InitRequest{})

	require.NoError(t, err)
	assert.Equal(t, setup.HostClaudeCode, res.Host)
	assert.False(t, res.NoHostDetected)
}
