# brief

**Status:** APPROVED. Scenarios agreed at the scoping gate; Phase 1 build order agreed.
Source of Truth for this feature. Amend here, not in a second document.

## Intent

Agent-driven development that proceeds in steps needs a way to carry context between them. The
common pattern is a directory of markdown per feature: a specification, a file per step, and a
notes file holding what has been decided so far. It is prompt convention. Nothing computes
with it.

A step needs three things to start: its own acceptance criteria, the constraints it inherits
from steps already done, and what it must not break. Without a tool an agent recovers all
three by reading whole files and inferring, and on exit relies on the finishing agent
voluntarily recording what the next step should know. Both halves fail quietly — a missing
handoff is indistinguishable from a step with nothing to hand off, and an oversized one is
nobody's error.

`brief` computes both halves: **assemble the step's context on entry, capture its delta on
exit.** Context quality follows from the second; token cost follows from the first.

Compacting a chain of handoffs into a single carried state is the move existing spec-driven
tooling does not make, and the reason this is built rather than adopted (see *Prior art*).

## Scope

A standalone tool: a library, a CLI over it, and a configuration file describing the
conventions of the repository it is pointed at.

- A single binary. No runtime dependency on the host project's build system, language, or
  agent framework.
- **Files stay the source of truth.** Same paths, same markdown, git-diffable and
  hand-editable. No database, no cache, no sidecar index, no daemon.
- Host integration is installed, not documented, and removable.

### The model it prescribes

`brief` is opinionated about structure, because every command computes over it:

- A **feature** is a directory holding one specification, an ordered set of steps, and a state
  file.
- A **step** is a file with an id and a checklist, paired with a **handoff file** recording
  what it leaves behind.
- **Progress** is a list in the specification. A step is done when its handoff is recorded —
  and because the handoff is its own file, "recorded" means *that file exists*. Doneness and
  handoff-presence cannot disagree, and there is no "present but empty" state to disambiguate.

Names are configured; existence is not. A repository whose work does not decompose this way —
no per-step files, no carried state — is out of scope rather than degraded into.

### Out of scope

- **Workflow and persona.** Two role positions exist (R14) and bind to agents the adopter
  already has. Stages, gates, review policy and what an agent does beyond calling the tool are
  the adopter's. Requiring that something calls `finish` is in scope; specifying who, or what
  else they do, is not.
- **Authoring.** `new` writes structure; an agent or a human writes the words. The tool never
  generates a specification, a step body, or a handoff.
- **Judgment about content.** See R7.
- **Generic large-document retrieval.** A document that is merely big is served by an existing
  markdown section-retrieval MCP.
- **Migration.** Existing feature directories are adopted where they conform and left alone
  where they do not.
- **Project-level task tracking** — boards, assignees, estimates, issue-tracker sync.

## Rules

### Truth and structure

- **R1 — Files are the source of truth.** Every command is a pure function of the files on
  disk at call time. Deleting the binary loses nothing; a hand edit is never "out of sync".
  This is what rejects every substrate in *Prior art*, and it is the escape hatch that makes
  the tool recoverable when its own validation locks work out.
- **R2 — Names are configuration; structure is not.** Heading text, file patterns, caps and
  the state schema come from config with shipped defaults, so pointing `brief` at an existing
  repository requires no edits to it. The model above is required: a feature missing its
  progress list, step files, checklist or state file is malformed, not degraded. A missing
  handoff file is not malformed — it is how an unfinished step looks. Conventions marked optional degrade instead — the command succeeds and names the
  shortfall.
- **R3 — Machine fields live in frontmatter; prose stays prose.** Step id, dependencies and
  status are parsed from YAML, never inferred from English. Heading drift can break a prose
  lookup but never the graph.
- **R4 — Order follows dependencies where declared, list position where not.** A step may
  declare `depends-on`. `next` returns the first step whose dependencies are done, and
  inherited context is the transitive closure of that graph. A feature declaring none degrades
  to list order.

### Reading

- **R5 — `start` is the interface.** One command returns everything needed to begin a step.
  Other read commands serve narrower questions and debugging.
- **R6 — Fallback lives in the tool, not in prompt prose.** With no state file, `state get`
  synthesizes from upstream steps' handoff files in dependency order, within the R13 budget.
  Its first line says the result is synthesized and from which files. A step with no handoff
  file is named as skipped — never silently read whole, never treated as "nothing to
  inherit".

### Writing

- **R7 — The tool owns the container; the caller owns the distillation.** `brief` enforces
  schema, caps, ordering, atomicity and accounting. It cannot judge whether an entry stopped
  being true, whether two entries should merge, or whether a decision still binds — those are
  judgments about the code just written and they stay in the caller's instructions. Rules
  requiring that judgment are out of reach by construction, and this specification says so
  rather than pretending otherwise.
- **R8 — A step closes by recording what it leaves behind, and the state file is rewritten
  rather than grown.** `finish` takes two inputs: the step's handoff, and the complete
  replacement body for the state file. It validates both, writes both atomically, and only
  then marks the step done. **Both are whole-file writes.** Neither is spliced into an
  existing document, because a boundary inferred from prose is a boundary that can be wrong,
  and a wrong boundary on a destructive write loses data. See R21. The tool never concatenates a handoff onto the state file — an
  append-only state file is the handoffs again with extra steps, which is the cost this tool
  exists to remove.
- **R9 — Deletion is accounted for, not prevented.** `finish` diffs the outgoing state body
  against the incoming one and reports every entry that disappeared, with the step it was
  tagged to. An entry whose step is not itself being closed, or a debt recorded as unowned, is
  reported. `brief` does not refuse the deletion — removing stale entries is the mechanism — it
  refuses to let one vanish unremarked.
- **R10 — The protocol is the only way through.** `new` is the only path to a conforming
  feature or step; `finish` is the only path to marking one done. There is no "mark done" verb
  and no template to copy by hand. An agent ignoring the protocol therefore produces no state
  rather than malformed state, and `status` shows the feature stalled. Correspondingly `start`
  refuses a malformed feature, naming what is missing and the command that fixes it, rather
  than assembling a partial brief — a brief that looks complete while silently omitting
  inherited constraints is the worst output this tool could produce.
- **R11 — `finish` is idempotent and ordered.** Finishing a finished step **with the same
  inputs** succeeds and changes nothing — identity is detected and the write skipped outright,
  rather than re-spliced to an identical result, so mtime is preserved. Finishing a finished
  step with **different** inputs is refused, naming the divergence: silently discarding a
  caller's handoff while reporting success is the failure R11 exists to prevent. Finishing a
  step with open checklist items is refused, naming the first. Finishing a step with an
  unfinished dependency is refused, naming it.
- **R12 — Writes are validated before they land, and land atomically.** An over-cap or
  incomplete body is refused and every existing file is left byte-identical. Writes go to a
  temp file in the same directory, then rename — same directory because `os.Rename` is atomic
  only within one filesystem. A refusal names the first thing wrong. A completed write leaves
  no temp file behind.
- **R21 — Machine-critical boundaries are never inferred from prose, and no write is
  destructive on an inferred boundary.** R3 already forbids inferring machine *fields* from
  English; this extends it to machine *boundaries*. The handoff therefore lives in its own
  file rather than as a section spliced into the step file: a whole-file write has no boundary
  to compute and so no boundary to get wrong. Where an in-file write is unavoidable — the
  progress checkbox, the frontmatter `status:` line — it must preserve everything outside the
  span it edits, so that a mistaken span duplicates or misplaces content rather than deleting
  it. Duplication is visible and recoverable; deletion is neither.
- **R20 — One step per feature is open at a time; there is no concurrency machinery.**
  Settles open question 1. `finish` is a plain read-modify-write against the state file, and
  atomicity comes from R12's temp-file-plus-rename alone. No compare-and-swap, no lock file,
  no advisory locking ships. Should the single-writer assumption ever break, the escape is
  `finish --if-state-matches <sha256>` — it is named here so that it is a deliberate later
  decision rather than a retrofit, and it is **not** built now.

### Output

- **R13 — Output is bounded, and truncation is never silent.** Each read command has a
  configurable byte budget. Output over budget is cut at a line boundary and ends with one
  line stating how many bytes were dropped and the command that returns the rest.
- **R14 — Errors are one line and actionable.** An unknown feature lists the known ones; an
  unknown section lists that file's headings. Exit codes: 0 ok, 1 validation failure, 2 usage.
  No stack traces. **Nothing to return is not an error: empty stdout, one line on stderr,
  exit 0.** A completed feature and a repository with no features are both this shape, so
  `brief start <f> | wc -c` of 0 means complete and `brief status | wc -l` of 0 means no
  features — no banner to strip, no sentinel to match. `--json` gives the structured caller
  the same discriminator as `"step": null`.
- **R14a — One refusal template, and refusals are not findings.** Every write refusal reads
  `brief <command>: <path>[:<line>]: <problem, stating the measured value and the limit>;
  <imperative next action> (no files changed)`. The problem states the actual number or name,
  never "too long". Read refusals drop the `(no files changed)` tail — it answers a question
  the reader did not ask. **Refusals** (exit 1, stderr) and **findings** (R9, exit 0, the
  profile's `[SEVERITY] <path>:<line> — <finding>` shape) are deliberately different shapes so
  that a script tells "finished with reported drops" from "refused" by exit code alone. That
  holds only because findings never appear on a failed run.

### Integration

- **R15 — Roles are positions, bound to the adopter's agents.** A **planner** turns a
  specification into conforming steps; an **implementer** calls `start`, works, and closes with
  `finish`. Config binds each to an agent that already exists. `brief` ships scaffolded
  definitions only for adopters who have none — namespaced, thin, opt-in, removable — and never
  shadows an existing agent. Unbound positions are reported by `check`, not enforced at
  runtime; R10 is what makes non-compliance harmless.
- **R16 — `init` is additive, idempotent and reversible.** It never removes or rewrites a
  setting it did not add. Every artifact it writes is marked `brief`-owned so `uninstall`
  removes exactly those; an install/uninstall round trip leaves pre-existing files
  byte-identical. Re-running converges rather than duplicating. Where a host's config is not
  writable, `init` prints the snippet to apply by hand and exits non-zero rather than
  half-installing.
- **R17 — Installed instructions are thin.** What `init` writes into always-loaded context is
  a pointer: the invocation, when to call it, where the full help lives. Prose installed into
  every context is paid for in every context.
- **R18 — Caps are enforced at the write path; `check` is a backstop.** `finish` makes an
  over-cap handoff unwritable going forward. `check` reports what predates the tool and fails
  only on features still in flight.
- **R19 — One stable invocation.** A single binary on PATH, resolving config from the working
  directory upward. No absolute paths in any instruction that names it.

## Command surface

| Command | Returns |
| --- | --- |
| `brief start <feature>` | Everything needed to begin the next step: id, title, acceptance criteria, checklist, inherited decisions, upstream constraints, known traps. `--json` for structured callers |
| `brief finish <feature> <step> --handoff <path> --state <path>` | Takes the handoff body and the **complete** replacement state body as two required path flags (`-` reads stdin, permitted on at most one), validates both, writes both, reports dropped entries, marks the step done |
| `brief new feature <name>` | Scaffolds a conforming feature: specification skeleton, empty progress list, state file. Structure only |
| `brief new step <feature>` | Scaffolds the next step file with id, frontmatter and an empty checklist, and its progress entry. It writes no handoff file — only `finish` does |
| `brief status` | One line per feature: name, done/open, next step, blocked count |
| `brief next <feature>` | The next step id and counts |
| `brief show <feature> <heading>` | One section of the specification. `--list` gives headings with byte sizes |
| `brief state get <feature>` | The state file, or the R6 synthesis |
| `brief state set <feature>` | Replaces the state file. Escape hatch; `finish` is the normal path |
| `brief handoff <feature> <step>` | The handoff file of one step |
| `brief check [feature]` | Findings, one per line as `path:line: message` |
| `brief roles` | Each position, the agent bound to it, whether that agent exists |
| `brief init` / `brief uninstall` | Install or remove config and host integration |
| `brief mcp` | Stdio MCP server over the above |

## Default profile

The shipped defaults are lifted from conventions an existing pipeline already describes in
prose, so `init` against such a tree writes no config. The profile is data and is overridable
in full.

**State file** — four required headings, entries carrying the step that introduced them:

```
## Binding decisions   <decision> — <the constraint that forces it> (SCENARIO-XX)
## Left unbuilt        <exact symbol/route/method> — <who owns it, or "unowned"> (SCENARIO-XX)
## Traps               <the trap> — <what it breaks> (SCENARIO-XX)
## Open debts          <debt> — <step that must close it, or "unowned — dies unless re-opened">
```

Cap ~80 lines. The *unowned debt* wording is load-bearing: it is the entry R9 must never let
disappear unremarked.

**Handoff file** — one per step, named by a configured pattern beside its step file; three
labelled groups (binding decisions, left unbuilt, traps); cap ~60 lines. It is the audit
trail. The state file supersedes it for reading, not for the record. It is written only by
`finish`, so its absence is exactly "this step is not done" — there is no anchor to scaffold
and no empty-versus-absent distinction to make.

**Progress** — a checklist in the specification under one stable heading. Per-step checkboxes
live in the step file.

**Findings** — `check` and `finish` emit `[SEVERITY] <path>:<line> — <finding>` by default, so
output drops into an existing review flow.

The profile deliberately carries no instruction for *how* to distil. That is R7's line.

## Configuration

One file at the repository root, resolved upward from the working directory:

- Where feature directories live, and how step files and handoff files are named.
- Heading text for the progress list and each required state section.
- Caps: handoff length, state length, default output budget.
- Which optional conventions this repository uses. The structural requirements above are not
  among them and cannot be switched off.
- Role bindings: which agent fills the planner position, and which the implementer.

Everything has a default. `init` proposes values by inspection and reports what it could not
determine rather than guessing.

## Init and integration

```
brief init [--global] [--host <name>]    # config + host integration
brief init --dry-run | --print | --show  # preview, emit for manual use, report state
brief init --with-agents                 # scaffold role agents; omit when binding existing ones
brief uninstall [--global]
```

Installs: the config; a thin instruction snippet (R17); commands for `start` and `finish`; a
hook running `check` on edits under the feature root; role bindings, with definitions only on
`--with-agents` and never under a name already in use.

Constraints that come from how such installs fail in practice: host config files are merged,
never replaced, and `brief`'s entry is individually removable. Where a host offers several
places to register a hook, `init` writes the canonical one, falls back only when the fallback
is already in live use, and says which it chose. `--global` and project installs do not
silently shadow each other; `--show` reports both and flags a conflict. An unknown host, or an
unwritable config, is an error naming what to do — `--print` is the supported path in an agent
sandbox. Host coverage is open-ended: one host first, an internal interface for the rest, and
an unsupported host degrading to `--print` plus manual wiring.

## First run

**`init` on a new project produces:** config (or nothing, where the profile already fits), the
feature-directory root, the `check` hook, the commands, the instruction snippet, and role
bindings. No feature directories, no templates, nothing to migrate.

**The adopter supplies:**

1. **Intent** — what the feature is. Irreducible.
2. **Approval of the step list.** `brief` does not decide what the steps are.
3. **Domain instructions.** `brief` knows the protocol and nothing about any stack. Scaffolded
   roles know how to call `start` and `finish`; they do not know an architecture, a testing
   discipline, or which directories are generated. Adopting `brief` greenfield yields correct
   bookkeeping around agents that do not yet know how to build anything. That is the intended
   division, not a shortfall.
4. **The content of every step** — plan, code, handoff, state rewrite (R7).

**The loop:**

```
brief new feature <name>       # skeleton → planner writes intent, rules, steps
                               #          → adopter approves the step list
brief new step <feature>       # once per approved step → planner writes each plan
brief start <feature>          # first step: plan, checklist, nothing inherited yet
brief finish <feature> <NN> \  # handoff + replacement state; reports dropped entries
  --handoff h.md --state s.md
brief start <feature>          # next step, now carrying the distilled state
brief status                   # across features: done/open, next, blocked
```

**Where the saving comes from.** Not from slicing markdown. Untooled, step N reads the
specification plus N−1 handoffs, so cost across a feature grows with the square of the step
count; tooled, step N reads one payload plus one capped state file. Flat instead of quadratic.
The consequence: the first step saves almost nothing and a short feature saves little. The
payoff is on long features, and an adopter whose features are uniformly short should take the
accounting and ignore the token argument.

## Success measure

Measured on first adoption against a real feature, and recorded:

- An agent starting a step makes **one** `start` call and **zero** subsequent whole-file reads
  of the specification or step files. Read counts before and after.
- `start` output as a fraction of the specification plus state file read whole. Reported, not
  asserted — the ratio is corpus-dependent.
- A `finish` carrying an over-cap handoff is refused, and every file is byte-identical after.
- An entry dropped from the state without its step closing is reported.
- `init` followed by `uninstall` leaves pre-existing files byte-identical, including a host
  config that already carried unrelated hooks.
- **The adopter's agent instructions get shorter.** Every procedure the tool now performs —
  the missing-state fallback, both schemas, both caps — is deleted from the prose describing
  it, not supplemented. Net line count goes down and the figure is recorded. What survives is
  the distillation judgment (R7). If those files do not shrink, the tool codified nothing and
  this measure has failed whatever the byte numbers say.

## Scenarios (Gherkin) — APPROVED

Agreed at the scoping gate. The design-doc Gherkin of earlier revisions carried several
`When`/`Then` cycles per scenario; these are executable units, one behavior each, because the
pipeline gives one scenario to one `architect` call and one `developer` call and never batches.
`orig` names the rev-9 scenario each derives from, so `## Phasing` and `## Success measure`
still read against the original numbering.

```gherkin
Feature: brief

  # ---- Pre-crossover: built by hand, the minimum to self-host ----

  Scenario: SCENARIO-01 Configuration resolves from the working directory upward  [orig: R2, OQ5]
    Given a repository with a brief config file at its root
    And a working directory several levels below that root
    When configuration is resolved
    Then the root config is found and its values are in effect
    Given a repository with no config file at all
    When configuration is resolved
    Then the shipped default profile is in effect
    And the four required state headings come from the resolved configuration, not from constants

  Scenario: SCENARIO-02 New feature scaffolds a conforming feature  [orig: 03a]
    Given a project using the shipped default profile
    When I create a new feature
    Then a specification skeleton, an empty progress list and a state file exist
    And no prose has been written into any of them

  Scenario: SCENARIO-03 New step scaffolds the next step file and its progress entry  [orig: 03b]
    Given a conforming feature
    When I create a new step
    Then the step file has an id, frontmatter and an empty checklist
    And its frontmatter carries an empty depends-on key
    And no handoff file exists yet, because only finish writes one
    And a matching entry appears in the progress list

  Scenario: SCENARIO-04 A start brief carries everything a step needs  [orig: 01]
    Given a feature with two finished steps and three open
    And the finished steps recorded decisions and constraints
    When I start the feature
    Then I get the next step's id, title, acceptance criteria and checklist
    And I get the decisions and constraints from the finished steps
    And I get the counts "2 done, 3 open"
    And I get no other step's acceptance criteria
    # The last assertion is an absence claim and passes vacuously on a single-step fixture.
    # It requires a fixture of at least three steps, and a control arm proving the other
    # steps' acceptance criteria are readable from the same files by the same probe — so
    # that deleting the filter reddens this scenario. See .claude/rules/agent-briefs.md.
    And nothing is written to disk

  Scenario: SCENARIO-05 Finishing writes the handoff and replaces the state, then marks done  [orig: 04a]
    Given an open step whose checklist is complete
    And an existing state file carrying entries from earlier steps
    When I finish it with a valid handoff and a valid replacement state body
    Then the step's handoff file holds exactly what I supplied
    And the step file itself is byte-identical apart from its status field
    And the state file holds exactly the replacement body, not the old body plus the handoff
    And the step is marked done in the progress list
    And no temp file remains in the feature directory

  Scenario: SCENARIO-06 Finishing a finished step with the same inputs changes nothing  [orig: 04b]
    Given a step already finished
    When I finish it again with the same handoff and the same state body
    Then the command succeeds
    And all three files are byte-identical, mtime included, because the write was skipped

  # ---- Crossover ----
  #
  # This document becomes brief's own feature directory. The remaining scenarios are built
  # through the tool. See ## Phasing.

  # ---- Post-crossover: built through the tool ----

  Scenario: SCENARIO-07 A feature name that would break the status contract is refused  [orig: new]
    Given a project using the shipped default profile
    When I create a feature whose name contains whitespace
    Then the command is refused with a usage exit
    And no directory is created
    # status is a whitespace-separated four-field contract; it dies the moment a name has a
    # space in it. Rejecting at creation is what makes that contract hold.

  Scenario: SCENARIO-08 Creating a feature that already exists is refused  [orig: new]
    Given an existing conforming feature
    When I create a feature of the same name
    Then the command is refused naming the existing directory
    And every file in that directory is byte-identical

  Scenario: SCENARIO-09 Status reports one four-field line per feature  [orig: new]
    Given three features in different states
    When I ask for status
    Then I get one line per feature, sorted by name
    And each line carries name, steps done over total, next step, and blocked count
    And a feature with no next step shows "-" in that field
    And there is no header line and no legend

  Scenario: SCENARIO-10 Status on a repository with no features succeeds silently  [orig: 10a]
    Given a project with no feature directories
    When I ask for status
    Then stdout is empty and the command succeeds
    And one line on stderr says no features were found

  Scenario: SCENARIO-11 One malformed feature does not blind status to the rest  [orig: new]
    Given four features, one of them malformed
    When I ask for status
    Then the malformed feature's line shows "!" and the reason is written to stderr
    And the other three features report normally
    And the command succeeds
    # R18 gives the failing role to check. status answers "what is the state of the world",
    # and "one of them is broken" is the true answer to that question.

  Scenario: SCENARIO-12 A completed feature has no next step  [orig: 02]
    Given a feature whose steps are all done
    When I start the feature
    Then stdout is empty and the command succeeds
    And one line on stderr says the feature is complete

  Scenario: SCENARIO-13 Start refuses a malformed feature rather than assembling half a brief  [orig: 03c]
    Given a feature directory whose specification is missing its progress list
    When I start that feature
    Then the command refuses, names what is missing and the command that fixes it
    And no partial brief is returned
    # A missing handoff file is deliberately NOT this scenario: under the model it means
    # "not done", which is an ordinary state, not a malformed feature.

  Scenario: SCENARIO-14 Start degrades on a missing optional convention and says so  [orig: 03d]
    Given a feature missing only an optional convention
    When I start that feature
    Then the brief is assembled on stdout and the command succeeds
    And the shortfall is named on stderr, so the payload stays clean

  Scenario: SCENARIO-15 Start emits a structured payload on request  [orig: new, OQ4]
    Given a feature with an open step
    When I start it asking for JSON
    Then I get the same brief as structured fields
    Given a feature whose steps are all done
    When I start it asking for JSON
    Then the step field is null, which is how a structured caller detects completion

  Scenario: SCENARIO-16 Finishing a done step with different inputs is refused  [orig: new]
    Given a step already finished
    When I finish it again with a handoff that differs from the recorded one
    Then the command is refused, naming the divergence and what to do instead
    And every file is byte-identical
    # R11 read literally would succeed and discard the new handoff, leaving the caller
    # believing it recorded something it did not.

  Scenario: SCENARIO-17 An over-cap handoff is refused and nothing lands  [orig: 06a]
    Given an open step whose checklist is complete
    When I finish it with a handoff over the configured cap
    Then the refusal names the measured line count and the cap
    And both files are byte-identical

  Scenario: SCENARIO-18 An over-cap state body is refused and nothing lands  [orig: 06b]
    Given an open step whose checklist is complete
    When I finish it with a replacement state body over the configured cap
    Then the refusal names the measured line count and the cap
    And both files are byte-identical

  Scenario: SCENARIO-19 A state body missing a required heading is refused  [orig: 06c]
    Given an open step whose checklist is complete
    When I finish it with a replacement state body missing a required heading
    Then the refusal names the missing heading
    And the refusal says an empty section is valid
    And both files are byte-identical

  Scenario: SCENARIO-20 A step with an open checklist item cannot be finished  [orig: 06d]
    Given a step with an open checklist item
    When I finish it
    Then the refusal names that item and its line
    And nothing is written

  Scenario: SCENARIO-21 A step with an unfinished dependency cannot be finished  [orig: 06e]
    Given a step whose frontmatter declares depends-on another step
    And that step is unfinished
    When I finish it
    Then the refusal names the dependency
    And nothing is written
    # Refusing on a dependency is one frontmatter read and one done-check. ORDERING by
    # dependencies — R4's transitive closure, "next returns the first step whose dependencies
    # are done" — is a later phase and is not in scope here.

  Scenario: SCENARIO-22 Check reports what the write path would now refuse  [orig: R18, :451]
    Given a feature carrying an over-cap handoff written before the caps existed
    When I run check
    Then the finding names the file, the line, the measured value and the cap
    And it is emitted in the profile's finding shape, not the refusal shape
    Given every feature conforming
    When I run check
    Then it reports nothing and succeeds
```

## Phasing

Ordered so the tool manages its own construction as early as possible. The pre-crossover set
is the **minimum that lets `brief` run against its own feature directory** — not the minimum
that is useful to anyone else. Name validation, `status`, `check` and every refusal are
deliberately after the crossover, because none of them are needed to self-host and all of them
are better built through the tool than by hand.

**Pre-crossover — built by hand.** SCENARIO-01 through SCENARIO-06, plus the atomic-write
primitive from R12. No `init` is needed: a hand-written config, or the shipped profile with no
config file at all, suffices.

The atomic write is **not a scenario of its own.** Temp-file-plus-rename cannot be reached
through the CLI by refusing something — a refusal proves validation ran, not that the write
landed atomically. It is a unit-level obligation on the platform write primitive, and
SCENARIO-05 asserts only its observable consequence: no temp file is left behind. Atomicity is
pre-crossover whatever else moves, because from the crossover onward a bad write damages this
tool's own specification.

**Crossover.** This document becomes `brief`'s own feature directory: this specification stays
the specification, each remaining scenario becomes a step file, and SCENARIO-01 through 06 are
marked done with handoffs written from what was actually built. `docs/specifications/brief/` is
both the pipeline's spec directory and `brief`'s own feature directory — they are the same
thing, not two things that must later be reconciled.

**Post-crossover — built through the tool.** SCENARIO-07 through SCENARIO-22. Sixteen of the
twenty-two scenarios therefore exercise `brief` on itself before anyone else runs it.

**Accepted risk of this ordering.** The five `finish` refusals (17–21) and `check` (22) land
after the crossover, so for that window `brief` manages its own specification with caps
unenforced at the write path and no backstop. R18 assigns those two roles to exactly those
scenarios; neither exists yet during the window. The mitigation is git and a branch, per
*Bootstrap hazards* below — not care. This was weighed against pulling 17 and 18 forward and
the ordering was chosen deliberately.

**Later phases.** SCENARIO-05's dropped-entry accounting (R9), dependency *ordering* (R4),
the R6 synthesis fallback, section addressing and output truncation, then adoption and host
integration — `init`, `uninstall`, role bindings — then the MCP front end, which remains a
decision point rather than a commitment.

The MCP front end ships only if shell permission prompts or malformed invocations proved an
actual cost, or if a target client has no shell; otherwise it is marked deliberately not built,
with the measurement. Tool schemas cost roughly 1–1.5 KB in every context that loads them, and
that figure goes into the decision.

### Bootstrap hazards

- **A `finish` bug corrupts this specification.** Mitigated by atomicity landing in phase 1 and
  by git, not by care. Work on a branch across the crossover.
- **A schema change after the crossover invalidates the tool's own files.** Any profile change
  from phase 2 onward requires migrating this feature directory first — a useful forcing
  function: if migrating one small tree is painful, adopters with twenty will refuse.
- **`start` refusing a malformed feature can lock the work out of its own workflow.** R1 is the
  escape hatch, and this is the case that justifies it.
- **Self-hosting cannot validate the token measure.** This feature has too few steps, and short
  features save little by construction. The read-count and ratio figures must come from a
  longer corpus. Self-hosting tests the protocol, not the economics.
- **Dogfooding biases the design toward its only user.** Phase 3 is the counterweight: its
  scenarios are written against a tree whose conventions differ from the defaults, precisely
  because this tool's own tree will not.

## Decisions taken

Settled at the scoping gate. Recorded with the reason so they are not re-litigated at review.

0. **The handoff lives in its own file; nothing is spliced into the step file.** Decided after
   implementation, against five rounds of evidence, and it reverses what SCENARIO-03, 05 and 06
   originally shipped. It is recorded first because it is the most expensive lesson here.

   The original design replaced a `## Handoff` section inside the step file. That required
   computing where the section started and ended, and the end was inferred by scanning prose.
   Seven distinct inputs moved that boundary and each produced a wrong write: mismatched fence
   delimiters, an indented fence, an unterminated fence in the input, an unterminated fence
   already on disk, an ordinary `##` heading inside handoff prose, CRLF line endings, and a
   doneness gate that switched a guard off exactly where the tool itself had set the flag.
   Four fix passes closed six of them; the seventh was introduced *by* the third fix.

   Two things made this expensive rather than merely buggy. First, the boundary was inferred
   from prose while its failure mode was data loss — R3's own logic, applied to boundaries
   instead of fields, forbids that; it is now written down as **R21**. Second, one fix made the
   splice destructive (replace to EOF) to cure an unbounded-growth bug, which converted every
   remaining boundary error from recoverable duplication into unrecoverable deletion and made
   the guards load-bearing for data safety.

   The decisive evidence: the state file has always been a whole-file write, has no boundary to
   compute, and produced **zero** boundary defects across all five rounds. The handoff now
   works the same way.

   Consequences: `spliceHandoff` and the scanning helpers that existed only to serve it are
   deleted; `new step` writes no handoff file; a missing handoff file means "not done" rather
   than "malformed"; and SCENARIO-13's absent-versus-present-but-empty trichotomy dissolves,
   because a file either exists or it does not.

1. **Concurrency — one step per feature is open at a time.** Was open question 1. `finish` is
   a plain read-modify-write; atomicity comes from R12 alone. No CAS, no locking. The escape,
   if the assumption ever breaks, is `finish --if-state-matches <sha256>`; it is named so it
   stays a deliberate decision rather than a retrofit, and it is not built. See **R20**.
2. **`start` keeps its name.** Was open question 2. The objection — that it does not signal
   read-only — is an output defect, not a naming one, and a rename is the expensive fix. R15
   already writes the sentence: *"an implementer calls `start`, works, and closes with
   `finish`."* `start`/`finish` is a matched lifecycle pair over a step, not a claim about the
   filesystem; `git log` does not mutate either. `--help` says so in six words. Rejected:
   `brief brief` (stutters in every sentence and in R17's installed snippet), `brief next
   --full` (demotes the spine of the product to a flag on a debugging command, and changes the
   return kind from scalar to payload — the classic sign of two commands), `brief context`
   (a noun against a verb spine, and it invites the standing DON'T BUILD of generic document
   retrieval).
3. **`--json` on `start` only, for now.** Was open question 4. It is already in the command
   table and the MCP front end needs it. `status`'s four-field table already *is* the machine
   format; a second stable format on an already-parseable surface is a schema to keep stable
   forever for nothing. The rule of thumb for later commands: `--json` where the output is a
   structured payload or a record set, not where it is a scalar.
4. **The state schema is configured, not compiled in.** Was open question 5. R2 says names are
   configuration, and this is a name. SCENARIO-01 pins it: the four required headings that
   SCENARIO-19 refuses on come from resolved configuration, with the shipped profile as the
   default.
5. **Config is YAML.** R3's frontmatter already forces a YAML parser; TOML would mean two
   config formats and a second dependency in a module that has none.
6. **`new` takes a type: `brief new feature <name>`, `brief new step <feature>`.** Not a
   preference — under the bare form, `brief new step` parses as *"create a feature named
   `step`"*, so any dispatcher must special-case a feature literally called `step` and a user
   who typos the feature argument silently creates one. `new <type> <target>` is the regular
   form. Bare `brief new <name>` is not kept as an alias, because the alias reintroduces
   exactly the ambiguity.
7. **`depends-on` ships in the Phase 1 scaffold; ordering does not.** Was open question 10.
   These separate cleanly: *refusing* `finish` on an unfinished dependency (R11, SCENARIO-21)
   is one frontmatter read and one done-check, while *ordering* by dependencies (R4) is a
   transitive closure. Scaffolding the empty key now costs one line of YAML and avoids
   migrating every step file later — which this document's own hazard note calls a forcing
   function worth taking seriously.
8. **`check` is in the first build.** Its cap checks are the same code as SCENARIO-17 through
   19, and without it `status`'s malformed-feature line would tell the user to run a command
   that does not exist.

## Open questions

1. How much does `init` infer? The floor is a commented default config and a clear error when
   it does not match.
2. Which host first, and how many after? Each is a distinct config format, hook vocabulary and
   command mechanism — a matrix of hosts is a maintenance commitment, not a feature.
3. How are installed artifacts marked `brief`-owned so `uninstall` is exact — naming
   convention, manifest, or marker inside each artifact?
4. Does `init` install the hook by default or behind a flag? It is the point of the
   integration and also the most intrusive thing the installer does.
5. Two positions or three? A reviewer position has no protocol call of its own, which suggests
   it belongs to the adopter's workflow.
6. Per-step checkboxes are ticked during implementation. Hand edit that `finish` reads as a
   precondition, or a command of its own? The on-disk format (`- [ ]` / `- [x]`) is settled
   either way, because SCENARIO-20 must read it; only the write mechanism is open.
7. Should the profile ship as a versioned artifact, so a repository can adopt "default
   profile v1" by reference rather than by copying values into config?
8. Does `--json` extend past `start` — to `next`, `show`, `state get`, `handoff`? Deferred
   rather than settled; see *Decisions taken* 3 for the rule of thumb it should be decided by.

## Prior art

An independent rebuild of a corner of spec-driven development, a category with 30+ entrants.
Evaluated and rejected as substrates:

- **Backlog.md** — markdown cards, board, MCP server; closest match to a directory of step
  files. **Rejected:** enforces "never edit markdown files directly" through a CLI/MCP gate,
  the negation of R1.
- **BMAD-METHOD** — the same file-based handoff architecture, each agent reading the prior
  agent's artifact. **Rejected:** carries artifacts forward whole rather than compacting them,
  which is the cost this tool removes; and adoption means adopting its agent roster.
- **Beads** — git-native DAG issue tracker. **Not adopted, model borrowed:** R4's `depends-on`
  is its edge by another name.
- **OpenSpec / Spec Kit** — own their file format and expect the repository to adopt it.
  `brief` inverts that: the repository keeps its format and describes it in config.

**Decision:** build, on R1 and on that inversion. Revisit if either stops mattering.---

## Triage Brief

From the `triage` agent, against this worktree. **Binding on the architect** — the
"already exists" line especially.

**Already exists — do not re-plan: nothing.** `go.mod` (`github.com/koblas/brief`, go 1.27.1,
zero `require` lines), `.golangci.yaml`, `devenv.nix` and the `.claude/` tree are the entire
footprint. No `cmd/`, no `internal/`, no Go source anywhere. Every capability below is new.

**Toolchain, confirmed by running it.** `go version` → go1.27.1 darwin/arm64, matching
`devenv.nix:11`. `golangci-lint version` → 2.13.2. On the empty module: `go build ./...` exits
0 with "matched no packages"; `go test ./...` exits 1 with "no packages to test";
`golangci-lint run ./...` exits 5 with "no go files to analyze". All three are the empty-module
signature, not a broken pin. A Bash call failing with `operation not permitted` is the sandbox;
re-run with the sandbox disabled.

**Lint constraints that shape the code.** `.golangci.yaml` is `default: all` minus an explicit
disable list, so anything not disabled is on.
- `wrapcheck` on — every error crossing a boundary is wrapped with `%w`, once.
- `forbidigo` on with the default pattern set — no raw `fmt.Print*`. Output goes through the
  injected `io.Writer` that `clean-architecture` already mandates.
- `depguard` and `exhaustruct` off — no import allow-list to satisfy, and functional-options
  defaulting is unconstrained.
- `gci`, `gofmt`, `gofumpt`, `goimports` all on — `gofumpt`-strict formatting.
- Two dead exclusions copied from another repo: `.golangci.yaml:56-59` names wrapcheck
  ignore-signatures for `fxsync`/`singleflight`/`refreshcache`, none of which are or will be
  dependencies; `.golangci.yaml:76-78` excludes `testpackage` for a `services/publicapi/...`
  path that does not exist here. White-box tests are blessed by `clean-architecture`, so if any
  are written they may need their own exclusion rather than reusing that dead one. NIT.

**Prior art in this repo, binding.** `.claude/skills/clean-architecture/SKILL.md:14-27` fixes
the layout: `cmd/brief/main.go` thin wiring only, `internal/cli/` for flag parsing and output
formatting with no business rules, `internal/<feature>/` for logic plus its `Store` port and
`memory.go` adapter, `internal/platform/<name>/` for dependency-light infra. Dependency rule at
`:101-118` — `cmd/brief → internal/cli → internal/<feature> → internal/platform/*`, and a
feature package never imports another feature package or `internal/cli`. Functional options at
`:51-72` are the mandated DI shape. Error handling at `:123-135` — wrap once with `%w`, export
sentinels for conditions callers branch on, and decide exit codes in `cmd/brief` by inspecting
the returned error; nothing below `main` calls `os.Exit`. That maps straight onto R14.
`go doc` conventions at `:151-245` bind every file. `.claude/rules/go-code.md:16` mandates
testify — not a candidate to evaluate.

**The constraint that rules out a library.** SCENARIO-06's byte-identity on re-finish and
SCENARIO-17/18/19's byte-identity on refusal disqualify any markdown library that parses to an
AST and re-renders the document — re-rendering is not lossless for spacing, list markers or
line endings. That much still holds.

**Superseded.** This brief originally concluded "the write path is a surgical text splice on
known section boundaries", and called that derivable from the spec rather than a preference.
`## Decisions taken` item 0 reverses it: the handoff is a whole-file write and nothing is
spliced. The splice was the derivation that cost five review rounds, which is worth leaving
visible rather than editing out — a conclusion can be sound about libraries and wrong about
the write path in the same paragraph. The rest of this brief is still binding.

**New dependencies, named because there are none today.** `gopkg.in/yaml.v3` for frontmatter
(R3) and for the config file. Markdown body handling is hand-rolled heading scanning, per the
constraint above. Checklist parsing is a few lines of scanning, no library. Atomic write is
`os.CreateTemp` in the same directory plus `os.Rename`, no library — same directory because
rename is atomic only within one filesystem. Subcommand dispatch is two-level (`new feature`,
`new step`, `state get`, `state set`), which stdlib `flag` does not provide; cobra or a
hand-rolled dispatcher over per-subcommand `flag.FlagSet` are the two live candidates and the
choice is the architect's.

**Open, and deliberately not resolved by triage.** What `start` does with a feature whose state
file is missing in the pre-crossover phase — R2 plus R10 imply "refuse", but that is inference,
not spec text. The architect should settle it in SCENARIO-04's plan and record it.

## Product Verdict

**SHIP WITH CHANGES**, from `product-vision` on the pre-crossover surface. All accepted; the
substantive ones are folded into the rules and scenarios above rather than left here. Recorded
so the architect does not re-derive them:

1. **`finish`'s invocation is now written down** — two required path flags, `-` on at most one.
   The spec previously never wrote out its highest-friction call. Rejected: inline flag values
   (a 60-line and an 80-line markdown body in `argv` via nested heredocs — quoting failures
   produce a validly-shaped but *truncated* state body, so the write succeeds and R9 later
   reports phantom deletions, which is the silently-wrong class this design most needs to
   avoid); one stdin stream with a sentinel (invents a micro-format, and a missing sentinel is
   indistinguishable from an empty state body); four positionals (order-sensitive between two
   same-typed arguments). Paths also make SCENARIO-06 testable by hand — up-arrow, enter —
   and an agent already has a file-write tool but no quoting-safe multiline-argv tool. Both
   flags stay mandatory: "omit `--state` to carry it forward" would quietly reintroduce
   growth-by-default, which is what R8 exists to prevent.
2. **Re-finish with different inputs** was undefined and R11 read literally discarded the
   caller's handoff silently. Now R11 and SCENARIO-16.
3. **`status` gained a contract and scenarios** — four whitespace-separated fields, no header,
   no legend, sorted by name; `!` for a malformed feature with the reason on stderr, still
   exit 0. SCENARIO-09, 10, 11.
4. **`brief new feature <name>`** — see *Decisions taken* 6. A parse defect, not a preference.
5. **One refusal template, refusals distinct from findings** — now R14a.
6. **Whitespace in feature names is rejected at creation, exit 2** — SCENARIO-07. The
   four-field `status` contract depends on it.
7. **"Nothing to return" unified across SCENARIO-10 and SCENARIO-12** — empty stdout, one
   stderr line, exit 0. Now the closing sentences of R14.
8. **SCENARIO-03's `Given a project where init has run` was reworded** to "a project using the
   shipped default profile". Phase 1 ships no `init`; the precondition was wording, not a
   dependency.

**Naming settled:** `start` stays, with the read-only concern answered in `--help` rather than
by a rename. See *Decisions taken* 2.

---

## BDD Acceptance Progress

Pre-crossover — built by hand:

- [x] SCENARIO-01: Configuration resolves from the working directory upward
- [x] SCENARIO-02: New feature scaffolds a conforming feature
- [x] SCENARIO-03: New step scaffolds the next step file and its progress entry
- [x] SCENARIO-04: A start brief carries everything a step needs
- [x] SCENARIO-05: Finishing writes the handoff and replaces the state, then marks done
- [x] SCENARIO-06: Finishing a finished step with the same inputs changes nothing

Crossover — this document becomes `brief`'s own feature directory.

Post-crossover — built through the tool:

- [x] SCENARIO-07: A feature name that would break the status contract is refused
- [x] SCENARIO-08: Creating a feature that already exists is refused
- [x] SCENARIO-09: Status reports one four-field line per feature
- [x] SCENARIO-10: Status on a repository with no features succeeds silently
- [x] SCENARIO-11: One malformed feature does not blind status to the rest
- [ ] SCENARIO-12: A completed feature has no next step
- [ ] SCENARIO-13: Start refuses a malformed feature rather than assembling half a brief
- [ ] SCENARIO-14: Start degrades on a missing optional convention and says so
- [ ] SCENARIO-15: Start emits a structured payload on request
- [ ] SCENARIO-16: Finishing a done step with different inputs is refused
- [ ] SCENARIO-17: An over-cap handoff is refused and nothing lands
- [ ] SCENARIO-18: An over-cap state body is refused and nothing lands
- [ ] SCENARIO-19: A state body missing a required heading is refused
- [ ] SCENARIO-20: A step with an open checklist item cannot be finished
- [ ] SCENARIO-21: A step with an unfinished dependency cannot be finished
- [ ] SCENARIO-22: Check reports what the write path would now refuse
