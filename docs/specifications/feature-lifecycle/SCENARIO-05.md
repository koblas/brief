---
id: SCENARIO-05
status: done
---

# SCENARIO-05: skill gains a Lifecycle section

## Scenario

Scenario: SCENARIO-05 skill gains a Lifecycle section
Given "brief init" for claude-code
Then ".claude/skills/brief-workflow/SKILL.md" carries the ruled frontmatter, title, Lifecycle section and "## Step protocol"
And the shipped-digest test pins the new bytes

Note (not a checklist item): `internal/platform/artifact.SkillWorkflow()` already reads
`files/skills/brief-workflow/SKILL.md` verbatim (`mustReadFile`), and `digest.go`'s
`skillWorkflowDigests` already computes its hash from that live render. So the only things
that pin the *old* bytes and must move in lockstep are two hand-transcribed test fixtures —
everything else in doctor/setup/init/uninstall calls `artifact.SkillWorkflow()` live at test
time and needs no edit (confirmed by reading every call site). Whole-repo grep for the old
digest prefix (`954bd6009e40`) and old title/description strings (`# brief step protocol`,
`brief's step protocol — pick up`) found no other hit outside the two fixtures below, the
skill file itself, and two out-of-scope spec docs (`docs/specifications/agent-workflow-skill/
specification.md`, this feature's own `specification.md`) that quote the old copy as history —
neither is touched by this scenario. No older-template bookkeeping: `olderSkillWorkflowDigests`
is already `[][32]byte{}` and stays empty (Rule 8, user ruling — no install has shipped these
bytes).

## Implementation Plan

### Red
- [x] Step 1: `internal/platform/artifact/artifact_test.go` `wantSkillWorkflow` (the transcribed-bytes const) and `Test_workflow_skill_renders_the_ruled_bytes` — replace the const with the exact ruled bytes, assembled in this order: frontmatter, blank line, `# brief workflow`, blank line, the kept intro paragraph, blank line, `## Lifecycle` heading, blank line, its 5 numbered items (Open, Plan, One step at a time, Review before finish, Roles) verbatim from the spec's Surface & Copy, blank line, `## Step protocol` heading, blank line, the existing numbered protocol unchanged (items 1–4, item 4 "Add a step" kept), blank line, the closing "Never tick…" paragraph unchanged, single trailing newline; frontmatter `description` becomes the "feature workflow" copy and `allowed-tools` is reordered/extended to `brief new feature`, `brief new step`, `brief start`, `brief finish`, `brief status`, `brief check`; fails against the still-old `SKILL.md` file
- [x] Step 2: `internal/platform/artifact/shipped_digest_test.go` `Test_render_matches_the_digest_every_install_already_carries` — replace the `KindSkillWorkflow` row's `want` with the sha256 of the Step 1 transcription (compute it by hashing the `wantSkillWorkflow` bytes in a scratch program under `$TMPDIR`, never by reading the test's own actual-value failure output — that would pin whatever the code currently produces, proving nothing); this is the file's own documented Rule-8 exception to "a deliberate change must move the old digest into that Kind's older list" (its comment), since no install has shipped the old bytes; fails against the still-old file

### Green
- [x] Step 3: `internal/platform/artifact/files/skills/brief-workflow/SKILL.md` — rewrite in place to the exact bytes from Step 1

### Sweep
- [x] Step 4: `internal/platform/artifact/plugin.go` `SkillWorkflow` doc comment — update the "step-protocol skill" characterization to name the widened preload contract (feature open/plan/review lifecycle, not just start/tick/finish/new-step), staying within the ~4-line exported-doc budget (sweep)
- [x] Step 5: fix what `go build ./... && golangci-lint run ./...` reports (sweep)

### Verify
- [x] Step 6: full verification per `.claude/rules/agent-briefs.md` — the one `go test -count=1 -coverpkg=./... ./...` run covers `internal/doctor`, `internal/setup`, `internal/cli`, and `internal/platform/host`, whose host-skill-recognition, init, and uninstall tests all derive their expectation from `artifact.SkillWorkflow()` live and need no separate re-run; mutation guard: restore-then-flip one byte in the on-disk `SKILL.md` (copy it aside per the mutation protocol first) and confirm `Test_workflow_skill_renders_the_ruled_bytes` reddens, then restore and diff to prove byte-identical

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- No older-template entry added for the skill's bytes — `olderSkillWorkflowDigests` stays `[][32]byte{}`; only the current-digest row in `shipped_digest_test.go` and the `wantSkillWorkflow` const in `artifact_test.go` move, per the user ruling that no install has shipped these bytes (spec Rule 8, Out of Scope). SCENARIO-06 (the CLAUDE.md snippet) follows the same rule: no older-template list gains an entry there either.
- `SkillWorkflow()`'s render path (`mustReadFile("skills/brief-workflow/SKILL.md")`) and `digest.go`'s live-computed `skillWorkflowDigests` are untouched — only the embedded markdown file's bytes change.

**Left unbuilt** — named so nobody assumes it exists:
- The CLAUDE.md snippet's "Multi-step work gets a feature" sentence (`internal/platform/artifact/snippet.go`) — still SCENARIO-06, untouched by this scenario.

**Traps** — things that look right and are not:
- Do not derive the `shipped_digest_test.go` hash by copying the actual-value string out of a failing test's output — that pins whatever the render currently produces, not an independently-known value (`agent-briefs.md`, *Assertions that prove nothing*). Hash the Step 1 transcription in a throwaway script instead; Steps 1 and 2 then both go red against the old file before Step 3 turns them green together.
- `internal/cli/init_internal_test.go:710` embeds `string(artifact.SkillWorkflow())` inline in an expected `--print` dump — it picks up the new bytes automatically and does not count lines or bytes; do not touch it.
- Two spec docs (`docs/specifications/agent-workflow-skill/specification.md`, this feature's own `specification.md`) still quote the old title/description as historical copy — out of scope, leave untouched, same pattern as SCENARIO-04's note on `human-output/SCENARIO-10.md`.
