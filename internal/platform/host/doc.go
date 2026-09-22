// Package host adapts brief to one agent host's own protocols: reading a
// hook-event payload from that host and rendering brief's own findings back
// into whatever shape that host expects on its hook's stdout. It carries no
// feature knowledge — a caller supplies the summary text and the path to
// look for, never a feature name or a *config.Config.
package host
