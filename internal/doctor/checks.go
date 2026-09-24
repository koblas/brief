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

// configFileName is the config file Locate looks for — the same name
// internal/platform/config resolves, repeated here only so doctor can
// name the file it would look for when none exists yet.
const configFileName = ".brief.yaml"

// configFixText is config-parse and config-values' own fix copy: the same
// remedy every other command's refusal on an invalid config already
// offers.
const configFixText = "correct the value, or delete the key to use its default"

// rootDirUnknownDetail is root-dir's own SKIP detail when config-parse
// failed: the feature directory a bad config might have named cannot be
// trusted, so root-dir does not guess Default()'s own value in its place.
const rootDirUnknownDetail = "feature root unknown, .brief.yaml did not parse"

// noConfigChecks builds the config-file, config-parse, config-values and
// config-shadow rows for a repository with no ".brief.yaml" anywhere:
// config-file WARN naming where one would be created, config-parse and
// config-values SKIP (nothing to decode or check), and config-shadow OK —
// it never depends on a config existing to parse, only on whether Locate
// found a farther ancestor config, which it cannot have without also
// finding a nearer one.
func noConfigChecks(wd string) []Check {
	placeholder := filepath.Join(wd, configFileName)

	return []Check{
		{ID: "config-file", Severity: SeverityWarn, Path: placeholder, Detail: "no .brief.yaml found", Fix: new(runInit)},
		{ID: "config-parse", Severity: SeveritySkip, Path: placeholder, Detail: "no .brief.yaml to parse"},
		{ID: "config-values", Severity: SeveritySkip, Path: placeholder, Detail: "no .brief.yaml to check"},
		{ID: "config-shadow", Severity: SeverityOK, Path: placeholder, Detail: "no ancestor configs shadowed"},
	}
}

// unparseableConfigChecks builds the config-file, config-parse,
// config-values and config-shadow rows for a found ".brief.yaml" that
// config.Inspect could not decode: config-file OK (the file exists),
// config-parse ERROR naming the decode failure, config-values SKIP (no
// values could be checked), and config-shadow OK regardless — a shadowed
// ancestor is informational and does not depend on the nearest config
// parsing.
func unparseableConfigChecks(nearest string, shadowed []string, inspectErr error) []Check {
	return []Check{
		{ID: "config-file", Severity: SeverityOK, Path: nearest, Detail: "found"},
		{ID: "config-parse", Severity: SeverityError, Path: nearest, Detail: parseErrorDetail(inspectErr), Fix: new(configFixText)},
		{ID: "config-values", Severity: SeveritySkip, Path: nearest, Detail: ".brief.yaml did not parse"},
		configShadowCheck(nearest, shadowed),
	}
}

// parseableConfigChecks builds the config-file, config-parse,
// config-values and config-shadow rows for a found ".brief.yaml" that
// decoded successfully: config-file and config-parse OK, config-values
// one ERROR row per violation (or a single OK row when there are none),
// and config-shadow.
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

// configValuesChecks builds config-values' own rows: exactly one OK row
// when violations is empty, else one ERROR row per element, in
// violations' own order (config.Inspect's field-declaration order),
// detail set to that violation's own Error() text.
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
// every farther ancestor config Locate reported as shadowed, or saying
// there are none.
func configShadowCheck(nearest string, shadowed []string) Check {
	if len(shadowed) == 0 {
		return Check{ID: "config-shadow", Severity: SeverityOK, Path: nearest, Detail: "no ancestor configs shadowed"}
	}

	return Check{ID: "config-shadow", Severity: SeverityOK, Path: nearest, Detail: "shadows " + strings.Join(shadowed, ", ")}
}

// parseErrorDetail renders inspectErr — an *InvalidConfigError
// config.Inspect returned for a decode failure — as one flattened line:
// its own underlying cause when the type assertion succeeds, inspectErr's
// own text otherwise (never reachable in production, since Inspect always
// wraps a decode failure this way).
func parseErrorDetail(inspectErr error) string {
	if invalidCfg, ok := errors.AsType[*config.InvalidConfigError](inspectErr); ok {
		return flattenOneLine(invalidCfg.Err.Error())
	}

	return flattenOneLine(inspectErr.Error())
}

// flattenOneLine collapses s to a single line: embedded newlines and runs
// of whitespace become one space each — yaml.v3 reports an unknown-key
// failure as "yaml: unmarshal errors:\n  line N: …", and a Check.Detail
// is always one line.
func flattenOneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// relPath renders p relative to wd for a Check.Fix's own copy ("mkdir -p
// <rel>"), falling back to p itself when it cannot be relativized.
func relPath(wd, p string) string {
	rel, err := filepath.Rel(wd, p)
	if err != nil {
		return p
	}

	return rel
}

// checkRootDir builds root-dir's own row: the feature directory
// (filepath.Join(root, featureDirectory)) must exist, be a directory, be
// readable (open then close, never list entries — root-dir never counts
// features) and be writable (writable.Probe, the only write doctor
// performs), checked in that order so an unreadable directory is reported
// as such rather than falling through to the write probe.
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

// checkEnvGit builds env-git's own row from repo.RootFS's own walk up from
// wd, against fsys, for a ".git" entry, directory or file (a linked
// worktree's ".git" is a file naming its real gitdir elsewhere) — found is
// OK, none anywhere is WARN with fix "git init". The same walk, against the
// same fsys, bounds Diagnose's own install root (R3, (*Server).locateInRepo).
func checkEnvGit(fsys fs.FS, wd string) Check {
	if root, ok := repo.RootFS(fsys, wd); ok {
		return Check{ID: "env-git", Severity: SeverityOK, Path: filepath.Join(root, ".git"), Detail: "found"}
	}

	return Check{ID: "env-git", Severity: SeverityWarn, Detail: "no .git found above the working directory", Fix: new("git init")}
}

// checkEnvPath builds env-path's own row: "brief" missing from PATH is
// ERROR when the Claude Code integration is installed (installed true —
// any Plugin(true) ∪ Agents() file present, or a CLAUDE.md snippet block
// found), since the hook that runs "brief check" can never find it; WARN
// otherwise, unchanged from before installed existed. Found and the same
// file as the running binary (compared through filepath.EvalSymlinks then
// os.SameFile, so a symlinked install still matches) is OK without reading
// either binary's version; a different file carrying the same version as
// the running binary, when that version is not devVersion, is OK; any
// other outcome — a different version, an unreadable one, or devVersion on
// either side — is WARN, naming the PATH binary in its own fix. The row's
// own Path is always found, exactly as lookPath reported it — the symlink
// resolution is an internal identity check, never surfaced, so a report
// never shows the user a resolved path unrelated to the PATH entry they
// configured.
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

// resolveSymlinks returns path's own filepath.EvalSymlinks result,
// falling back to path itself when it cannot be resolved (a dangling
// symlink, or a path that does not exist).
func resolveSymlinks(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}

	return resolved
}

// sameFile reports whether a and b name the same file on disk, via
// os.Stat and os.SameFile — false when either cannot be stat'd.
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
