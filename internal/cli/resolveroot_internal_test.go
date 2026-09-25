// resolveRootFS must produce byte-identical output to resolveRoot for the
// same config, read once from real disk and once from an rwfs.Mem fixture.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_resolveRootFS_matches_resolveRoot_on_an_invalid_config(t *testing.T) {
	cases := []struct {
		name       string
		yamlSource string
	}{
		{
			name:       "out-of-range value",
			yamlSource: "handoff-cap-lines: 0\n",
		},
		{
			name:       "malformed YAML",
			yamlSource: "handoff-cap-lines: [\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wd := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(wd, ".brief.yaml"), []byte(c.yamlSource), 0o600))

			var diskStdout, diskStderr bytes.Buffer
			diskErr := run(t.Context(), wd, []string{"status"}, nil, &diskStdout, &diskStderr, noBuildInfo)

			tree := newMemTree(memRoot).file(filepath.Join(memRoot, ".brief.yaml"), c.yamlSource)
			var memStdout, memStderr bytes.Buffer
			memErr := run(t.Context(), memRoot, []string{"status"}, nil, &memStdout, &memStderr, noBuildInfo, withRootFS(tree.mem()))

			require.Error(t, diskErr)
			require.Error(t, memErr)
			assert.Equal(t, ExitCode(diskErr), ExitCode(memErr))
			assert.Equal(t, diskStdout.String(), memStdout.String())
			assert.NotEmpty(t, diskStderr.String())
			assert.Equal(t, diskStderr.String(), memStderr.String())
		})
	}
}
