// Package host adapts brief to one agent host's own protocols: reading a
// hook-event payload from that host, rendering brief's own findings back
// into whatever shape that host expects on its hook's stdout, and listing
// the files that host's skills-directory plugin installs. It carries no
// feature knowledge — a caller supplies the summary text and the path to
// look for, never a feature name or a *config.Config; a plugin File names
// only a relative path and an internal/platform/artifact.Kind, never a
// body or an install root, both of which the caller supplies.
package host
