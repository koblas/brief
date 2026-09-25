// Package config resolves brief's configuration: the shipped default
// profile, overlaid with the repository's ".brief.yaml" when one is
// present.
//
// Every feature package (new, start, finish, status, check) consumes a
// Config; this package holds no feature knowledge, only the schema and the
// resolution rule. Locate walks upward from a start directory to the
// filesystem root for the nearest ".brief.yaml", naming every farther
// ancestor's config file as shadowed — no config file anywhere is not an
// error, it is the shipped profile in effect. Inspect decodes a found file
// onto Default(), so an omitted key keeps its shipped value, an unknown key
// is refused, and reports every decoded value that fails its own rule (a
// cap is at least 1, a heading is non-empty and distinct from the others, a
// file name carries no path separator, and so on). Resolve is Locate plus
// Inspect's first violation: the one rule set and the one walk-up every
// caller shares, so a command refusing an invalid value and a tool
// reporting every value that fails (doctor) can never drift onto two
// different checks.
package config
