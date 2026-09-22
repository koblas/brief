---
id: SCENARIO-07
status: done
---

# SCENARIO-07: init adds the CLAUDE.md instruction block

## Scenario

```gherkin
Scenario: SCENARIO-07 init adds the CLAUDE.md instruction block
  When I run "brief init --host claude-code"
  Then CLAUDE.md gains the snippet between <!-- brief:begin --> and <!-- brief:end -->, naming the configured feature directory
  And re-running replaces only the marked span; uninstall removes it, leaving the rest byte-identical
  And a lone begin or end marker refuses naming the file and line (no files changed)
```

Rules: R5 (block, location, lone marker), R6 (brief-written vs edited), R11 (verbs, JSON
`kind: snippet`, `modified`). Inherited context: `STATE.md` only; no prior SCENARIO file read.

## User-visible contract

`brief init --host claude-code` (also `--dry-run`, `--force`, `--no-hook`, `--json`):

- stdout row, after the plugin rows (config/feature root/plugin files first):
  - `created CLAUDE.md` — neither `CLAUDE.md` nor `.claude/CLAUDE.md` exists; the file is the block alone
  - `merged CLAUDE.md` — block appended to an existing file without one
  - `merged CLAUDE.md (block updated)` — a brief-written block that differs from today's render (other feature directory, older template) replaced in place
  - `unchanged CLAUDE.md` — block equals today's render for the configured feature directory
  - `kept CLAUDE.md (edited locally)` — block matches no brief render; never rewritten, `--force` included
  - `kept CLAUDE.md (not a regular file)` — symlink or other non-regular path; Lstat only, never followed
  - path is `.claude/CLAUDE.md` when that is the chosen file — the row is how init "says which"
- JSON: `kind: "snippet"`; `created` → `created[]`; `merged` → `modified[]`
- stderr next action: any `created` **or `merged`** row counts as "installed", not "already installed; nothing changed"
- refusal (exit 1, nothing written, `--dry-run` included): `<rel>:<line>: <problem>; <fix>` via the existing `RefusalError.Line` rendering, for: lone begin, lone end, end before begin, a second begin (multiple blocks, also across both candidate files), CRLF line endings (no line)

`brief uninstall [--host claude-code]` (default host):

- `removed CLAUDE.md (brief block)` — block removed, file kept → path in `modified[]`
- `removed CLAUDE.md` — remaining bytes empty → file deleted → path in `removed[]`
- `kept CLAUDE.md (edited locally)` unless `--force` (then removed as above)
- same refusals as init; `--host none` never touches CLAUDE.md
- a snippet-only uninstall reaches the "removed brief's install ..." stderr line

## Byte rules (pinned — the developer implements exactly these)

- Block B = `SnippetBlock(dir)`: the R5 text, marker lines included, **no trailing newline**; `dir` has any trailing `/` trimmed so the rendered text reads `` `<dir>/` `` once.
- Span = from the first byte of the begin-marker line through the last byte of the end-marker text, excluding its line terminator. A marker line is a whole line exactly equal to the marker.
- Create: file = B + `"\n"`.
- Append to non-empty O: O ends with `"\n"` → O + `"\n"` + B + `"\n"`; O without trailing newline → O + `"\n\n"` + B. Existing empty O → B + `"\n"` (same as create).
- Replace (recognized, not current): substitute the span in place; surrounding bytes untouched.
- Remove: span terminated by `"\n"` and preceded by `"\n\n"` → drop one preceding `"\n"` + span + its `"\n"`; span unterminated (EOF) and preceded by `"\n\n"` → drop the `"\n\n"` + span; span at offset 0, or any other position (a block the user moved) → drop span + its `"\n"` if present. Remaining bytes empty → delete the file (never its parent: `.claude/` is never pruned).
- Rewrites go through `atomicfile` and keep the existing file's mode.
- Location: a candidate already holding a block wins; else root `CLAUDE.md` if it exists; else `.claude/CLAUDE.md` if it exists; else create root `CLAUDE.md`. Uninstall examines both candidates. (The "block wins" clause amends R5 — Step 16a.)
- Refusal Path/Line: two blocks in one file → that file, the *second* begin line; blocks in both candidates → `.claude/CLAUDE.md` (root is the preferred location), its begin line, fix = delete that block; lone end / end before begin → the end line; lone begin → the begin line.

## Implementation Plan

- [x] Step 1: `internal/platform/artifact/artifact_test.go` `Test_snippet_block_names_the_feature_directory` + `Test_recognize_snippet_*` (current for its dir / brief-written for another dir / edited / CRLF-normalized is not recognized / trailing-slash dir renders one slash) (red)
- [x] Step 2: `internal/platform/artifact/snippet.go` `SnippetBlock(featureDir)`, `RecognizeSnippet(block)`, snippet render-func list (extraction of the dir is per render func, using that func's own anchors — never one global anchor) — marker constants `SnippetBegin`/`SnippetEnd` exported for setup; `KindSnippet` stays off `Render`/`digestsFor` with a doc line saying so (new → green)
- [x] Step 3: `internal/platform/host/claudecode_test.go` `Test_claude_code_lists_its_instruction_files_in_priority_order` (red)
- [x] Step 4: `internal/platform/host/host.go` + `claudecode.go` — `Host.InstructionFiles() []string` (`CLAUDE.md`, `.claude/CLAUDE.md`); no filesystem access (update → green)
- [x] Step 5: `internal/setup/snippet_test.go` — Init matrix through `(*Server).Init`: created / merged append (with and without trailing newline, empty file, file ending in a blank line) / unchanged on rerun / merged-updated after feature-directory change / kept edited (also under `--force`) / kept non-regular / fallback to `.claude/CLAUDE.md` / block in `.claude/CLAUDE.md` wins over a later root file / `--no-hook` still plans the snippet / `--host none` plans none / dry run writes nothing (red)
- [x] Step 6: `internal/setup/snippet_test.go` — refusal matrix: lone begin, lone end, end before begin, two blocks in one file, blocks in both candidates, CRLF; each asserts `*RefusalError` Path + Line and a tree snapshot unchanged, for Init and Uninstall (red)
- [x] Step 7: `internal/setup/setup.go` — `KindSnippet`, `ActionMerged`; `Result`/`Artifact` docs updated, incl. `Result.Modified` now also naming files Uninstall rewrote; `(*Server).Uninstall` doc in `uninstall.go` likewise (new)
- [x] Step 8: `internal/setup/snippet.go` — `planSnippet` (location choice, marker scan → refusal, classify), `mergeSnippet`/`removeSnippet` pure byte funcs per the rules above, `writeSnippetFile` (new → green)
- [x] Step 9: `internal/setup/setup.go` `Init` + `apply` — plan snippet for claude-code after plugin files; apply order feature root, plugin files, snippet, config last; `created` → `Created`, `merged` → `Modified`; partial-write wrapping unchanged (update → green)
- [x] Step 10: `internal/setup/uninstall.go` — plan snippet removal first (before hook); apply writes the stripped file (`Modified`) or deletes an emptied one (`Removed`); `--force` for edited (update → green)
- [x] Step 11: `internal/setup/roundtrip_test.go` `Test_init_then_uninstall_leaves_claude_md_byte_identical` — table: trailing newline / none / trailing blank line / `.claude/CLAUDE.md` fallback / no CLAUDE.md at all (file absent again afterwards); a separate test pins the accepted exception that a pre-existing empty CLAUDE.md is deleted, not restored; each asserts post-init differs from pre-init before asserting byte identity (red → green on arrival is acceptable only if Steps 8-10 landed; say so)
- [x] Step 12: `internal/cli/init_test.go` `Test_init_merging_only_the_snippet_reports_installed_not_nothing_changed` + row/JSON tests (`kind: snippet`, `modified[]`), and update the existing claude-code row-list assertions for the new row (red)
- [x] Step 13: `internal/cli/init.go` `initNextAction` — treat `ActionMerged` as a change (update → green); `artifact_render.go` needs no change unless a test says otherwise
- [x] Step 14: `internal/cli/uninstall_test.go` — `removed CLAUDE.md (brief block)` row, `modified[]` vs `removed[]` split, snippet-only uninstall stderr line, lone-marker refusal `<rel>:<line>:` text + exit 1 (red → green; `uninstall.go` update only if red)
- [x] Step 15: `internal/setup/doc.go`, `internal/platform/artifact/doc.go`, `internal/platform/host/doc.go` — package docs name the snippet and the byte rules as contract (update)
- [x] Step 16: mutation verification, one at a time, stashed per agent-briefs: (a) delete the lone-marker guard → Step 6 lone-begin/lone-end tests red; (b) make append always add `"\n\n"` → Step 11 trailing-newline row red; (c) drop the `ActionMerged` arm in `initNextAction` → Step 12 merge-only test red; (d) make `RecognizeSnippet` return current for any block → kept-edited test red; (e) skip the empty-file delete → Step 11 create row red; (f) drop the CRLF check → CRLF refusal red. Record mutation → test reddened.
- [x] Step 16a: `docs/specifications/init-doctor/specification.md` R5 — add "**Amended during SCENARIO-07 planning**": a candidate file already holding the block wins over the root-first rule (Claude Code's own `/init` can create a root `CLAUDE.md` after the block landed in `.claude/CLAUDE.md`; the plain rule would install a second block); CRLF files refuse; an emptied file is deleted on uninstall (update)
- [x] Step 17: `go build ./...`, `go test ./...` (count + delta), `go test -race ./internal/setup/... ./internal/cli/... ./internal/platform/...`, `golangci-lint run ./...` → mark SCENARIO-07 done in specification.md

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- Snippet recognition is config-independent: `RecognizeSnippet` extracts the directory between the template's anchors and re-renders each known snippet render func with it; a match to today's render for the *configured* dir is `unchanged`, any other match is brief-written → replaced (`merged (block updated)`), no match is edited → `kept`. Uninstall therefore never decodes `.brief.yaml` (STATE's digest-only rule holds) — S10's `host-snippet` doctor row reuses the same recognizer — valid only because extraction is per render func (each older template carries its own anchors).
- `KindSnippet` is off `artifact.Render`/`digestsFor` — the snippet is the one parameterized artifact; S09 `--print` must call `SnippetBlock(dir)` directly, not `Render`.
- Byte rules above (separator encoded by the block's own terminator) are what make the round trip byte-identical with no sidecar — changing the append separator breaks every installed repo's uninstall.
- An emptied CLAUDE.md is deleted on uninstall; that is the only "brief created it" signal. A pre-existing empty CLAUDE.md is therefore deleted too (accepted).
- CRLF files refuse (init and uninstall): written and recognized bytes stay one value.
- Symlinked / non-regular CLAUDE.md → `kept (not a regular file)`, never followed or replaced.
- Init never rewrites an edited block, `--force` included; only `Uninstall --force` removes it.
- R5 amended (Step 16a): a candidate already holding the block wins over root-first; blocks in both → refusal naming `.claude/CLAUDE.md` and its begin line.
- Location choice lives in `setup` (stat), candidates in `host.Host.InstructionFiles()`; the snippet is never a `host.File`, so `--no-hook` cannot affect it.
- Order: init applies snippet after plugin files, before config; uninstall removes snippet first. Block rewrite → `Modified`; deleted file → `Removed`; created → `Created`.

**Left unbuilt** — named so nobody assumes it exists:
- `artifact.OriginOlder` for snippets (older render funcs list has one entry) — S10
- `--print` emission of the block as `# CLAUDE.md (merge)` — S09
- Writability pre-check of CLAUDE.md (R10) — S09
- doctor `host-snippet` row — S10
- Marker detection inside fenced code blocks — not handled; any exact marker line counts

**Traps** — things that look right and are not:
- `initNextAction` scanned only `ActionCreated`; a merge-only run printed "nothing changed".
- A round-trip assertion passes if init wrote nothing — assert post-init differs first.
- `"foo"` and `"foo\n"` collide under any fixed separator; only the block-terminator encoding separates them.
- `feature-directory: docs/specs/` renders `//` unless the render trims.
- Existing claude-code row-list tests in `internal/cli/init_test.go` gain a snippet row; the S06 round-trip test now also creates/deletes CLAUDE.md and must stay green.
