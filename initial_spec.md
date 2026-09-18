# brief

**Status:** DRAFT rev 9. Not approved. Scenarios are proposed, not agreed.

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
- A **step** is a file with an id, a checklist, and a handoff block recording what it leaves
  behind.
- **Progress** is a list in the specification. A step is done when its handoff is recorded.

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
  progress list, step files, checklist, handoff anchor or state file is malformed, not
  degraded. Conventions marked optional degrade instead — the command succeeds and names the
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
  synthesizes from upstream steps' handoff blocks in dependency order, within the R13 budget.
  Its first line says the result is synthesized and from which files. A step file with no
  handoff anchor is named as skipped — never silently read whole, never treated as "nothing to
  inherit".

### Writing

- **R7 — The tool owns the container; the caller owns the distillation.** `brief` enforces
  schema, caps, ordering, atomicity and accounting. It cannot judge whether an entry stopped
  being true, whether two entries should merge, or whether a decision still binds — those are
  judgments about the code just written and they stay in the caller's instructions. Rules
  requiring that judgment are out of reach by construction, and this specification says so
  rather than pretending otherwise.
- **R8 — A step closes by recording what it leaves behind, and the state file is rewritten
  rather than grown.** `finish` takes two inputs: the step's handoff block, and the complete
  replacement body for the state file. It validates both, writes both atomically, and only
  then marks the step done. The tool never concatenates a handoff onto the state file — an
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
- **R11 — `finish` is idempotent and ordered.** Finishing a finished step succeeds and changes
  nothing. Finishing a step with open checklist items is refused, naming the first. Finishing
  a step with an unfinished dependency is refused, naming it.
- **R12 — Writes are validated before they land, and land atomically.** An over-cap or
  incomplete body is refused and every existing file is left byte-identical. Writes go to a
  temp file in the same directory, then rename. A refusal names the first thing wrong.

### Output

- **R13 — Output is bounded, and truncation is never silent.** Each read command has a
  configurable byte budget. Output over budget is cut at a line boundary and ends with one
  line stating how many bytes were dropped and the command that returns the rest.
- **R14 — Errors are one line and actionable.** An unknown feature lists the known ones; an
  unknown section lists that file's headings. Exit codes: 0 ok, 1 validation failure, 2 usage.
  No stack traces.

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
| `brief finish <feature> <step>` | Takes the handoff block and the replacement state body, validates both, writes both, reports dropped entries, marks the step done |
| `brief new <feature>` | Scaffolds a conforming feature: specification skeleton, empty progress list, state file. Structure only |
| `brief new step <feature>` | Scaffolds the next step file with id, frontmatter, empty checklist and handoff anchor, and its progress entry |
| `brief status` | One line per feature: name, done/open, next step, blocked count |
| `brief next <feature>` | The next step id and counts |
| `brief show <feature> <heading>` | One section of the specification. `--list` gives headings with byte sizes |
| `brief state get <feature>` | The state file, or the R6 synthesis |
| `brief state set <feature>` | Replaces the state file. Escape hatch; `finish` is the normal path |
| `brief handoff <feature> <step>` | The handoff block of one step file |
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

**Handoff block** — last section of every step file; three labelled groups (binding decisions,
left unbuilt, traps); cap ~60 lines. It is the audit trail. The state file supersedes it for
reading, not for the record.

**Progress** — a checklist in the specification under one stable heading. Per-step checkboxes
live in the step file.

**Findings** — `check` and `finish` emit `[SEVERITY] <path>:<line> — <finding>` by default, so
output drops into an existing review flow.

The profile deliberately carries no instruction for *how* to distil. That is R7's line.

## Configuration

One file at the repository root, resolved upward from the working directory:

- Where feature directories live, and how step files are named.
- Heading text for the progress list, the handoff block and each required state section.
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
brief new <feature>            # skeleton → planner writes intent, rules, steps
                               #          → adopter approves the step list
brief new step <feature>       # once per approved step → planner writes each plan
brief start <feature>          # first step: plan, checklist, nothing inherited yet
brief finish <feature> <NN>    # handoff + replacement state; reports dropped entries
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

## Scenarios (Gherkin) — PROPOSED

```gherkin
Feature: brief

  Scenario: SCENARIO-01 A start brief carries everything a step needs
    Given a feature with two finished steps and three open
    And the finished steps recorded decisions and constraints
    When I start the feature
    Then I get the next step's id, title, acceptance criteria and checklist
    And I get the decisions and constraints from the finished steps
    And I get no other step's acceptance criteria
    And I get the counts "2 done, 3 open"

  Scenario: SCENARIO-02 A completed feature has no next step
    Given a feature whose steps are all done
    When I start the feature
    Then I am told the feature is complete
    And the command succeeds

  Scenario: SCENARIO-03 New is the only path to a conforming feature
    Given a project where init has run
    When I create a new feature
    Then a specification skeleton, an empty progress list and a state file exist
    And no prose has been written into any of them
    When I create a new step
    Then the step file has an id, a checklist and a handoff anchor
    And a matching entry appears in the progress list
    Given a feature directory whose steps were created by hand without a handoff anchor
    When I start that feature
    Then the command refuses, names the missing anchor and the command that fixes it
    And no partial brief is returned
    Given a feature missing only an optional convention
    Then start succeeds and names the shortfall

  Scenario: SCENARIO-04 Finishing writes the handoff and replaces the state, then marks done
    Given an open step whose checklist is complete
    And an existing state file carrying entries from earlier steps
    When I finish it with a valid handoff block and a valid replacement state body
    Then the handoff block of that step file holds what I supplied
    And the state file holds exactly the replacement body, not the old body plus the handoff
    And the step is marked done in the progress list
    When I finish the same step again with the same inputs
    Then the command succeeds and all three files are byte-identical

  Scenario: SCENARIO-05 Entries that disappear from the state are reported
    Given a state file carrying a trap, a left-unbuilt symbol and an unowned debt
    When I finish a step with a body that drops the trap tagged to the step being closed
    Then the drop is reported as accounted for and the write succeeds
    When I finish a step with a body that drops the unowned debt
    Then the drop is reported as a finding naming the debt
    And the write still succeeds, because removing stale entries is the mechanism
    When I finish a step with a body that drops an entry tagged to an open step
    Then the finding names that entry and the step it came from

  Scenario: SCENARIO-06 Finishing is refused rather than landing half a write
    Given an open step
    When I finish it with a handoff block over the configured cap
    Then the write is refused naming the cap, and both files are byte-identical
    When I finish it with a replacement state body over the configured cap
    Then the write is refused naming the cap
    When I finish it with a replacement state body missing a required heading
    Then the write is refused naming the missing heading
    Given a step with an open checklist item
    When I finish it
    Then the refusal names that item and nothing is written
    Given a step with an unfinished dependency
    When I finish it
    Then the refusal names the dependency and nothing is written

  Scenario: SCENARIO-07 Order follows dependencies where declared, list position where not
    Given a feature where step 4 declares depends-on step 7
    And step 7 is unfinished
    When I ask for the next step
    Then step 4 is not offered and the reason names step 7
    And a step with no unfinished dependencies is offered instead
    Given a feature whose steps declare no dependencies
    Then next returns the first unfinished entry in list order

  Scenario: SCENARIO-08 Inherited context is synthesized when the state file is absent
    Given a feature with no state file and three step files
    And two of them have a handoff block and one has no handoff anchor
    When I get the state
    Then the first line says the state is synthesized and names the two source files
    And the handoff blocks appear in dependency order
    And the file with no anchor is named as skipped
    And the output respects the output budget

  Scenario: SCENARIO-09 Sections are addressed as written, and oversized output is cut visibly
    Given a specification whose headings differ from the shipped defaults
    When I ask for one of its sections by its heading as written
    Then I get that section up to the next heading of the same level
    When I ask for a section that does not exist
    Then the error lists the headings that do exist
    Given a section larger than the output budget
    When I ask for that section
    Then the output ends at a line boundary within the budget
    And the last line states how many bytes were dropped and the command that returns the rest

  Scenario: SCENARIO-10 A greenfield project is usable straight after init
    Given an empty directory with no feature directories and no agents
    When I run init
    Then the feature root, the hook, the commands and the instruction snippet exist
    And the shipped profile is in effect, whether or not a config file was written
    And show reports what was installed and at which scope
    When I run status
    Then it reports no features, and does not error
    When I create a feature, create a step, and start it
    Then the brief is assembled from a project that had nothing in it before init ran

  Scenario: SCENARIO-11 An existing tree is adopted without being edited
    Given a repository of feature directories that has never used brief
    And its headings and file names differ from the shipped defaults
    When I run init
    Then a config is written matching what is on disk
    And anything that could not be determined is named rather than guessed
    When I start a feature there
    Then the brief is assembled and no file in the repository has changed

  Scenario: SCENARIO-12 Roles are bound to existing agents, and scaffolded only on request
    Given a host with agents already named architect and developer
    When I run init and bind the planner to architect and the implementer to developer
    Then the binding is written to config and no agent definition is created
    Given a host with no agents
    When I run init with agents
    Then namespaced role definitions exist, each stating only how to call brief
    Given a position bound to an agent that does not exist
    When I run check
    Then it reports the unbound position, and start and finish still work

  Scenario: SCENARIO-13 Installing host integration is additive and fully reversible
    Given a host config file that already contains an unrelated hook
    When I run init for that host
    Then the brief artifacts are installed and the unrelated hook is unmodified
    When I run init again
    Then nothing is duplicated and the result is byte-identical
    When I uninstall
    Then every brief-owned artifact is gone
    And the config file is byte-identical to before the first init
    Given a host config file that cannot be written
    When I run init
    Then no file is modified, the snippet is printed, and the exit is non-zero

  Scenario: SCENARIO-14 The MCP front end returns what the CLI returns (decision point)
    Given the brief MCP server running over stdio
    When a client calls the start tool for a feature
    Then the result text equals the CLI's output for the same feature
    When a client finishes a step with an over-cap body through the tool
    Then the call reports the same refusal as the CLI and the files are unchanged
```

`check` (R18) is not a scenario of its own: a flag on `status` plus unit tests, built alongside
SCENARIO-06 since both exercise the same cap checks.

SCENARIO-14 is a decision point. Build the MCP front end only if shell permission prompts or
malformed invocations proved an actual cost in phases 2–3, or if a target client has no shell;
otherwise mark it deliberately not built, with the measurement. Tool schemas cost roughly
1–1.5 KB in every context that loads them, and that figure goes into the decision.

## Phasing

Ordered so the tool can manage its own construction as early as possible.

**Phase 1 — self-hosting core, built by hand.** SCENARIO-01, 02, 03, 04, 06, plus `status` and
the atomicity half of R12. No `init` needed: a hand-written config, or the shipped profile with
no config at all, suffices. Atomicity is phase 1 whatever else moves, because from the
crossover onward a bad write damages this tool's own specification.

**Crossover.** Convert this document into a conforming feature directory: this specification
becomes the specification, each remaining scenario a step file, phase 1's scenarios marked done
with handoffs written from what was actually built.

**Phase 2 — built through the tool.** SCENARIO-05, 07, 08, 09. By SCENARIO-05 the tool's own
state file is under accounting; by SCENARIO-07 its remaining steps can declare dependencies.

**Phase 3 — adoption and integration.** SCENARIO-10, 11, 12, 13. None are needed to build the
tool; all are needed before anyone else runs it.

**Phase 4 — optional.** SCENARIO-14.

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

## Open questions

1. **Concurrency.** R9 requires reading the state body to diff it, so `finish` is
   read-modify-write on a file every concurrent step also reads. If two steps in one feature
   can be open at once, that needs a compare-and-swap against the prior content, not a lock. If
   they cannot, say so in the rules and skip the machinery. Decide before SCENARIO-04.
2. Subcommand naming: `start` reads well against the tool name but does not signal that it is
   read-only.
3. How much does `init` infer? The floor is a commented default config and a clear error when
   it does not match.
4. Does `--json` apply to `start` only, or to every read command?
5. Is the state schema fixed or configured? R2 implies configured; validation messages are
   simpler if fixed.
6. Which host first, and how many after? Each is a distinct config format, hook vocabulary and
   command mechanism — a matrix of hosts is a maintenance commitment, not a feature.
7. How are installed artifacts marked `brief`-owned so `uninstall` is exact — naming
   convention, manifest, or marker inside each artifact?
8. Does `init` install the hook by default or behind a flag? It is the point of the
   integration and also the most intrusive thing the installer does.
9. Two positions or three? A reviewer position has no protocol call of its own, which suggests
   it belongs to the adopter's workflow.
10. What does `new step` do about dependencies? Empty `depends-on` degrades to list order;
    inferring an edge from the previous step would be convenient and often wrong.
11. Per-step checkboxes are ticked during implementation. Hand edit that `finish` reads as a
    precondition, or a command of its own?
12. Should the profile ship as a versioned artifact, so a repository can adopt "default
    profile v1" by reference rather than by copying values into config?

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

**Decision:** build, on R1 and on that inversion. Revisit if either stops mattering.