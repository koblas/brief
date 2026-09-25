package agentfile_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/koblas/brief/internal/platform/agentfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parse_decodes_frontmatter_from_bytes(t *testing.T) {
	body := "---\nname: planner\nskills: [\"brief-workflow\"]\n---\n\nbody\n"
	fsys := fstest.MapFS{"agent.md": &fstest.MapFile{Data: []byte(body)}}

	want, loadErr := agentfile.LoadFS(fsys, "agent.md")
	require.NoError(t, loadErr)

	got, err := agentfile.Parse([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, want, got)

	t.Run("errors on missing closing delimiter", func(t *testing.T) {
		badBody := "---\nname: planner\n\nno closing delimiter here\n"
		badFS := fstest.MapFS{"agent.md": &fstest.MapFile{Data: []byte(badBody)}}

		_, wantErr := agentfile.LoadFS(badFS, "agent.md")
		require.Error(t, wantErr)

		_, err := agentfile.Parse([]byte(badBody))
		require.Error(t, err)
	})
}

func Test_load_decodes_one_agent_files_frontmatter(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantErr   bool
		wantName  string
		wantSkill []string
	}{
		{
			name:      "block-list skills",
			body:      "---\nname: planner\nskills:\n  - brief-workflow\n---\n\nbody\n",
			wantName:  "planner",
			wantSkill: []string{"brief-workflow"},
		},
		{
			name:     "scalar skills decodes loosely, no error",
			body:     "---\nname: planner\nskills: brief-workflow\n---\n\nbody\n",
			wantName: "planner",
		},
		{
			name:    "no frontmatter",
			body:    "just a plain agent file\n",
			wantErr: true,
		},
		{
			name:    "unparseable yaml",
			body:    "---\nname: [this is not: valid\n---\n\nbody\n",
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "agent.md")
			require.NoError(t, os.WriteFile(path, []byte(c.body), 0o600))

			fm, err := agentfile.Load(path)

			if c.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, c.wantName, fm.Name)
			assert.Equal(t, c.wantSkill, fm.Skills)
		})
	}

	t.Run("missing file", func(t *testing.T) {
		_, err := agentfile.Load(filepath.Join(t.TempDir(), "missing.md"))
		require.Error(t, err)
	})
}

// Uses real directories, not the in-memory Tree: proves Binding.Path is a
// real path Lstat can open.
func Test_resolve_binding_wires_dir_tree_over_root_and_home(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	path := filepath.Join(root, ".claude", "agents", "developer.md")
	writeAgentFile(t, path, developerAgent)

	b := agentfile.ResolveBinding(root, home, "developer")

	require.Equal(t, agentfile.BindingResolved, b.State)
	assert.Equal(t, path, b.Path)

	_, err := os.Lstat(b.Path)
	assert.NoError(t, err, "Path must name a real file a caller can Lstat")
}
