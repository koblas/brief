// Package setup owns brief's own install write path: Init plans and then
// applies the ".brief.yaml" config file, the feature root a repository
// needs before brief's other commands work in it, and, for
// HostClaudeCode, that host's own skills-directory plugin files
// (internal/platform/host.Host.Plugin), the brief-workflow skill
// (internal/platform/host.Host.Skills) on every claude-code install
// regardless of WithAgents, under WithAgents its three role-agent files
// (host.Host.Agents, R7), and the CLAUDE.md instruction block (R5);
// Uninstall plans and applies removing everything Init installed, agent
// and skill files included regardless of any flag Init was run with. Both
// plan every artifact and decide every refusal before the first byte is
// written or removed, so a DryRun request computes exactly what a real run
// would and a refusal never leaves a partial tree behind on its own
// account.
//
// WithAgents binds every role only in a config this same Init call
// creates or, under --force, rewrites — R7's "an existing config is never
// edited": a kept or already-current config is never rewritten to add
// bindings, however unbound its own roles are. Result.RolesToAdd is the
// hint instead: a "roles:" header plus one "  <role>: brief:<role>" line
// per role the config still leaves unbound, empty whenever WithAgents did
// write the bindings itself or there is nothing left to add. ConfigFile
// and ConfigFileWithRoles (internal/platform/artifact) are two recognized
// bodies for one artifact.KindConfig; Render still returns the plain one,
// so only this package's own configDigests-derived Recognize check, and
// planConfig's decodeCurrentConfig, ever have to tell them apart.
//
// The CLAUDE.md block lives at whichever of internal/platform/host's own
// InstructionFiles candidates already holds a recognized block, else the
// first that exists, else root CLAUDE.md is created — scanSnippetCandidates
// and chooseSnippetLocation own that choice, shared by Init's planSnippet
// and Uninstall's planSnippetRemoval. A marker defect (a lone or
// out-of-order begin/end, two blocks in one file, a block in both
// candidates) or a CRLF chosen candidate refuses, naming the file and, for
// a marker defect, its 1-based line; CRLF is checked only against the one
// candidate a caller resolved to act on, never blanket across both, so an
// untouched CLAUDE.md elsewhere never blocks an install or removal aimed at
// the other one. mergeSnippet and removeSnippet are exact inverses: the
// separator mergeSnippet encodes around an appended block (one newline
// before it when the file already ended in one, a blank line before it
// when it did not, no newline after it in that second case) is exactly
// what removeSnippet strips back out, so a round trip through Init then
// Uninstall reproduces byte-identical bytes with no sidecar recording which
// shape was used. An emptied CLAUDE.md is deleted on Uninstall — the only
// "brief created it" signal this package keeps, so a pre-existing,
// already-empty CLAUDE.md is deleted too, not restored.
//
// Init never resolves configuration the way every other command does
// (internal/platform/config.Resolve, via internal/cli's resolveRoot): a
// repository's own ".brief.yaml" may be invalid, and --force must still be
// able to rewrite it from defaults, so Init uses Locate and Inspect
// directly and classifies what it finds itself. Every plugin file, and
// Uninstall's own removal of the config, is recognized digest-only
// (internal/platform/artifact.Recognize) and never decoded, so none of
// them has a refusal class of its own — an unparseable, R1-invalid, or
// locally edited file is simply "edited locally", the same as any other
// byte mismatch, kept unless --force. An OriginOlder file (Rule 6: bytes
// equal to an earlier release's own render, never today's) is not treated
// as an edit at all: planPluginFile plans it ActionMerged, detail
// "updated", and apply rewrites it unconditionally, guarded only by
// verifyFileUnchanged's own concurrent-edit check — no --force needed,
// since nothing about it is the adopter's own content. --force otherwise
// rewrites only the config from defaults; it never rewrites an edited
// plugin file, only removes one under Uninstall, and Uninstall removes an
// OriginOlder file the same as a current one, without --force.
//
// InitRequest.Host == "" means detect rather than refuse: detectHost
// resolves HostClaudeCode when the install root (the same root every
// artifact is planned against) holds a ".claude" directory or a
// "CLAUDE.md" entry of any type, or when WithHomeDir's own home function —
// os.UserHomeDir by default — reports a directory whose own ".claude" is a
// directory; otherwise HostNone. Result.NoHostDetected is true only when
// detection ran and found nothing, the one signal a caller needs to
// distinguish that from an explicit HostNone.
//
// InitRequest.Print, like DryRun, computes the same plan and writes
// nothing; Result.Print (printArtifacts) is always populated — never nil
// — with one PrintArtifact per pending (ActionCreated or ActionMerged)
// artifact, the feature root excluded, carrying the exact bytes a real run
// would write there. Before applying anything, a real run (neither DryRun
// nor Print) also runs checkWritable over every target: a target whose
// nearest existing ancestor is not a directory, or is a directory that
// cannot be written to (internal/platform/writable.Probe, shared with
// internal/doctor's own root-dir check), refuses as ErrUnwritable — the only Init
// error path that still returns a populated Result (Artifacts and Print)
// alongside the error, so a caller can render the by-hand output the
// refusal's own Fix points at.
//
// A caller-facing refusal is a *RefusalError: a path, what was wrong, and
// how to fix it, wrapping ErrUnknownHost, ErrUnwritable, or an
// internal/platform/config.InvalidConfigError. A write failure after at
// least one artifact already landed is wrapped in ErrPartialWrite instead,
// distinguishing it from a refusal that changed nothing on disk. Init's
// own artifact list carries the config first, the feature root, then the
// plugin's own files in host.Host.Plugin's write order, then the
// brief-workflow skill, then, under WithAgents, the three role-agent
// files, then, under EditAgents, one KindBoundAgent row per
// planBoundAgents target, then the CLAUDE.md block last; applying writes
// in that same
// order, so the config — the repository's opt-in marker — never lands
// before everything else has. Uninstall's own list carries the CLAUDE.md
// block first, then bound-agent…, agent…, skill, plugin…, config — the
// reverse of Init's own order above — and applies in that same order, so a
// partial uninstall never removes the opt-in marker while something else
// still stands. The feature root and everything under it is never an
// Uninstall artifact at all, and is never removed; nor is ".claude/" or
// ".claude/skills/" above the plugin's own directory or the skill's own
// directory — brief owns only host.PluginDir and below,
// host.WorkflowSkillDir, and the CLAUDE.md candidates InstructionFiles
// names (R6).
//
// Uninstall's own bound-agent rows (Rule 8, planBoundAgentRemovals) are
// planned only when the brief-workflow skill's own row is not itself
// ActionKept — an edited SKILL.md without --force, or one that is not a
// regular file even with --force keeps every bound agent's own entry too,
// no row at all. removeWorkflowSkill is addWorkflowSkill's own inverse; a
// shape it cannot edit — the same shapeOther boundary addWorkflowSkill
// draws, plus a quoted or commented entry the loose membership decode
// (agentfile.Parse) cannot tell from a plain one — gets no row rather than
// ActionKept: Uninstall reports only what it removed. The row is
// ActionRemoved, but the file is rewritten in place, never deleted — its
// path lands in Result.Modified, not Result.Removed.
//
// setup writes through the real filesystem — internal/platform/atomicfile
// for the config file's, every plugin and skill file's and CLAUDE.md's own
// byte-identical replace, os.MkdirAll for the feature root and a plugin or
// skill file's parent directories, os.Remove for Uninstall's own file and
// now-empty-directory removals (never RemoveAll) — imports only
// internal/platform/agentfile, internal/platform/config,
// internal/platform/artifact, internal/platform/atomicfile,
// internal/platform/host and internal/platform/rwfs alongside the standard
// library, and never internal/scaffold or internal/doctor: those own the
// write and read paths over a feature's own content, a question setup
// never asks.
//
// Every read and write Init and Uninstall perform under a repository
// root, and the config-location walk above it (locateInRepo,
// inspectConfig, mirroring internal/doctor's own locateInRepo/inspect),
// go through (*Server).fsRoot — production diskFS (fs.go), a stateless
// rwfs.FS reproducing exactly the os.* calls this package always made,
// deliberately unconfined rather than rwfs.OS's own os.Root confinement:
// planPluginFile's DryRun planning has a passing test that depends on
// today's symlink-following read behavior through a ".claude" pointed
// outside the repository (bound_agent_test.go), and R10's own writability
// pre-check (checkWritable, still real os.Lstat/writable.Probe,
// unconfined) does not gate every write a broader confinement would newly
// refuse (init_disk_test.go pins this with a mutation). A test substitutes
// an rwfs.Mem via the package-private WithFSRoot (export_test.go).
// boundAgentTargets' and agentsMissingSkill's own root-resolution
// (filepath.EvalSymlinks(root), before either walks agentfile bindings) is
// a separate seam, (*Server).resolveRoot (WithResolveRoot,
// export_test.go) — planBoundAgent, planBoundAgentRemoval,
// confinedAgentFile and agentfile.ResolveBinding's own file search all
// still read and write through real disk regardless of fsRoot or
// resolveRoot: bound-agent confinement is never routed through this seam.
//
// Result.AgentsMissingSkill (agentsMissingSkill) is Init's own report of
// every bare-name planner or implementer binding — never a "brief:*" or
// other-plugin one — whose resolved agent does not preload the
// brief-workflow skill, via internal/platform/agentfile's own
// ResolveBinding and (Binding).LackingSkill, the same decision doctor's
// roles-skill row makes. It reads from planConfig's own post-run
// config.Roles, computed only for HostClaudeCode, before the DryRun/Print
// return so both carry it, and is empty — never nil — otherwise, including
// on every Uninstall Result. Under EditAgents, subtractMergedBoundAgents
// removes every path planBoundAgents actually merged before this same
// return, so a caller's post-run "still missing" view and the bound-agent
// rows it also rendered never disagree.
//
// EditAgents (planBoundAgents, bound_agent.go) edits only a bare-name
// planner or implementer binding's own ScopeProject agentfile.Definition —
// never a "brief:*" binding, another plugin's, or one under "~/.claude" —
// deduped by path so two roles bound to the same file produce one merged
// row. addWorkflowSkill performs the actual "skills:" frontmatter edit,
// following the shape table doc.go's specification names: a top-level key
// missing entirely (including one only nested under another key, or only
// present inside another key's own block scalar) gets a fresh
// "skills: [brief-workflow]" line; a block or single-line flow list gets
// the name appended; every other shape is left alone (ActionKept). A
// non-regular leaf (a symlink) is ActionKept without being read; a regular
// leaf whose own resolved path escapes the resolved install root is
// skipped entirely, contributing no row. confinedAgentFile.write preserves
// the file's own Lstat'd permission bits, unlike writePluginFile's fixed
// 0o644, since a bound agent file is the adopter's own.
package setup
