package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/koblas/brief/internal/platform/config"
	"github.com/koblas/brief/internal/platform/repo"
	"github.com/koblas/brief/internal/platform/writable"
)

// configFileName is the config file Locate looks for.
const configFileName = ".brief.yaml"

// configFixText is config-parse and config-values' own fix copy.
const configFixText = "correct the value, or delete the key to use its default"

// rootDirUnknownDetail is root-dir's own SKIP detail when config-parse
// failed.
const rootDirUnknownDetail = "feature root unknown, .brief.yaml did not parse"

// noConfigChecks builds config-file, config-parse, config-values and
// config-shadow for a repository with no ".brief.yaml" anywhere:
// config-file WARN, config-parse and config-values SKIP, config-shadow OK.
func noConfigChecks(wd string) []Check {
	placeholder := filepath.Join(wd, configFileName)

	return []Check{
		{ID: "config-file", Severity: SeverityWarn, Path: placeholder, Detail: "no .brief.yaml found", Fix: new(runInit)},
		{ID: "config-parse", Severity: SeveritySkip, Path: placeholder, Detail: "no .brief.yaml to parse"},
		{ID: "config-values", Severity: SeveritySkip, Path: placeholder, Detail: "no .brief.yaml to check"},
		{ID: "config-shadow", Severity: SeverityOK, Path: placeholder, Detail: "no ancestor configs shadowed"},
	}
}

// unparseableConfigChecks builds config-file, config-parse, config-values
// and config-shadow for a found ".brief.yaml" that failed to decode:
// config-file OK, config-parse ERROR naming the failure, config-values
// SKIP, config-shadow OK.
func unparseableConfigChecks(nearest string, shadowed []string, inspectErr error) []Check {
	return []Check{
		{ID: "config-file", Severity: SeverityOK, Path: nearest, Detail: "found"},
		{ID: "config-parse", Severity: SeverityError, Path: nearest, Detail: parseErrorDetail(inspectErr), Fix: new(configFixText)},
		{ID: "config-values", Severity: SeveritySkip, Path: nearest, Detail: ".brief.yaml did not parse"},
		configShadowCheck(nearest, shadowed),
	}
}

// parseableConfigChecks builds config-file, config-parse, config-values
// and config-shadow for a found ".brief.yaml" that decoded successfully:
// config-file and config-parse OK, config-values one ERROR row per
// violation (or one OK row when there are none), and config-shadow.
func parseableConfigChecks(nearest string, shadowed []string, violations []*config.ValueError) []Check {
	checks := make([]Check, 0, 2+max(len(violations), 1)+1)
	checks = append(checks,
		Check{ID: "config-file", Severity: SeverityOK, Path: nearest, Detail: "found"},
		Check{ID: "config-parse", Severity: SeverityOK, Path: nearest, Detail: "parses"},
	)
	checks = append(checks, configValuesChecks(nearest, violations)...)
	checks = append(checks, configShadowCheck(nearest, shadowed))

	return checks
}

// configValuesChecks builds one OK row when violations is empty, else one
// ERROR row per violation, in field-declaration order.
func configValuesChecks(nearest string, violations []*config.ValueError) []Check {
	if len(violations) == 0 {
		return []Check{{ID: "config-values", Severity: SeverityOK, Path: nearest, Detail: "no invalid values"}}
	}

	checks := make([]Check, 0, len(violations))

	for _, v := range violations {
		checks = append(checks, Check{ID: "config-values", Severity: SeverityError, Path: nearest, Detail: v.Error(), Fix: new(configFixText)})
	}

	return checks
}

// configShadowCheck builds config-shadow's own row: always OK, naming
// every shadowed ancestor config, or saying there are none.
func configShadowCheck(nearest string, shadowed []string) Check {
	if len(shadowed) == 0 {
		return Check{ID: "config-shadow", Severity: SeverityOK, Path: nearest, Detail: "no ancestor configs shadowed"}
	}

	return Check{ID: "config-shadow", Severity: SeverityOK, Path: nearest, Detail: "shadows " + strings.Join(shadowed, ", ")}
}

// parseErrorDetail renders inspectErr as one flattened line: the
// underlying cause when it wraps an *InvalidConfigError, its own text
// otherwise.
func parseErrorDetail(inspectErr error) string {
	if invalidCfg, ok := errors.AsType[*config.InvalidConfigError](inspectErr); ok {
		return flattenOneLine(invalidCfg.Err.Error())
	}

	return flattenOneLine(inspectErr.Error())
}

// flattenOneLine collapses s to a single line: newlines and runs of
// whitespace become one space each.
func flattenOneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// relPath renders p relative to wd, falling back to p itself when it
// cannot be relativized.
func relPath(wd, p string) string {
	rel, err := filepath.Rel(wd, p)
	if err != nil {
		return p
	}

	return rel
}

// checkRootDir builds root-dir's own row: the feature directory must
// exist, be a directory, be readable and be writable, checked in that
// order so an unreadable directory is reported as such rather than
// falling through to the write probe.
func checkRootDir(wd, root, featureDirectory string) Check {
	path := filepath.Join(root, featureDirectory)

	info, statErr := os.Stat(path)
	if statErr != nil {
		return Check{ID: "root-dir", Severity: SeverityError, Path: path, Detail: "does not exist", Fix: new("mkdir -p " + relPath(wd, path))}
	}

	if !info.IsDir() {
		return Check{ID: "root-dir", Severity: SeverityError, Path: path, Detail: "is not a directory", Fix: new("set feature-directory in .brief.yaml")}
	}

	f, openErr := os.Open(path)
	if openErr != nil {
		return Check{ID: "root-dir", Severity: SeverityError, Path: path, Detail: "not readable", Fix: new("chmod u+rwx " + relPath(wd, path))}
	}
	_ = f.Close()

	if !writable.Probe(path) {
		return Check{ID: "root-dir", Severity: SeverityError, Path: path, Detail: "not writable", Fix: new("chmod u+rwx " + relPath(wd, path))}
	}

	return Check{ID: "root-dir", Severity: SeverityOK, Path: path, Detail: "exists, readable and writable"}
}

// checkEnvGit builds env-git's own row: a ".git" entry (directory or
// file) found by walking up from wd is OK; none anywhere is WARN with fix
// "git init".
func checkEnvGit(fsys fs.FS, wd string) Check {
	if root, ok := repo.RootFS(fsys, wd); ok {
		return Check{ID: "env-git", Severity: SeverityOK, Path: filepath.Join(root, ".git"), Detail: "found"}
	}

	return Check{ID: "env-git", Severity: SeverityWarn, Detail: "no .git found above the working directory", Fix: new("git init")}
}

// checkEnvPath builds env-path's own row: "brief" missing from PATH is
// ERROR when the Claude Code integration is installed, WARN otherwise.
// Found and the same file as the running binary is OK; a different file
// at the same non-dev version is OK; any other outcome is WARN, naming
// the PATH binary in its own fix.
func (s *Server) checkEnvPath(installed bool) Check {
	found, lookErr := s.lookPath("brief")
	if lookErr != nil {
		if installed {
			return Check{ID: "env-path", Severity: SeverityError, Detail: "brief not found on PATH; the Claude Code integration runs it", Fix: new("install brief on PATH")}
		}

		return Check{ID: "env-path", Severity: SeverityWarn, Detail: "brief not found on PATH", Fix: new("install brief on PATH")}
	}

	foundReal := resolveSymlinks(found)

	if self, selfErr := s.executable(); selfErr == nil {
		if sameFile(foundReal, resolveSymlinks(self)) {
			return Check{ID: "env-path", Severity: SeverityOK, Path: found, Detail: "matches the running binary"}
		}
	}

	version, ok := s.binaryVersion(foundReal)
	if ok && version == s.version && version != devVersion {
		return Check{ID: "env-path", Severity: SeverityOK, Path: found, Detail: "matches the running version " + version}
	}

	detail := "a different version is installed"
	if !ok {
		detail = "the installed version could not be read"
	}

	return Check{ID: "env-path", Severity: SeverityWarn, Path: found, Detail: detail, Fix: new(fmt.Sprintf("update %s to the running version", found))}
}

// resolveSymlinks returns filepath.EvalSymlinks(path), falling back to
// path itself when it cannot be resolved.
func resolveSymlinks(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}

	return resolved
}

// sameFile reports whether a and b name the same file on disk; false when
// either cannot be stat'd.
func sameFile(a, b string) bool {
	fa, errA := os.Stat(a)
	if errA != nil {
		return false
	}

	fb, errB := os.Stat(b)
	if errB != nil {
		return false
	}

	return os.SameFile(fa, fb)
}
