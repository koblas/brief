---
description: brief's step protocol — pick up a feature's next open step with brief start, tick its checklist as items go green, close it with brief finish. Use when planning or implementing a step of a brief-tracked feature.
user-invocable: false
allowed-tools:
  - Bash(brief start *)
  - Bash(brief finish *)
  - Bash(brief new step *)
  - Bash(brief status *)
  - Bash(brief check *)
---

# brief step protocol

A feature is a directory of markdown: a specification, ordered step files, and one
state file. `brief status` lists every feature and its next open step.

1. **Start.** `brief start <feature>` prints the next open step — its id, acceptance
   criteria and checklist — and the decisions it inherits from the state file. Work
   from that output; do not read the specification or earlier handoffs whole. It
   writes nothing.
2. **Work.** As each checklist item goes green, tick it by hand in the step file:
   `- [ ]` becomes `- [x]`. This is the only bookkeeping edit you make yourself.
   `brief finish` refuses while any item is unticked.
3. **Finish.** Write two bodies to scratch files (or pass `-` for one, read from stdin):
   - the handoff: what this step decided, what it left undone, what the next step
     must know;
   - the state: a COMPLETE replacement of the feature's state file — every inherited
     section `brief start` printed, updated, not just this step's delta. Anything you
     leave out is dropped.
   Then run `brief finish <feature> <step> --handoff <path> --state <path>`. It ticks
   the progress list, marks the step done, writes the step's handoff file and replaces
   the state file — all or nothing. A refusal names what to fix (an unticked item, a
   missing state heading, a body over its line cap) and changes no files; fix it and
   run it again.
4. **Add a step.** `brief new step <feature>` scaffolds the next step file and its
   progress entry; fill in its body.

Never tick the progress list, mark a step done, write a handoff file or edit the state
file by hand: `brief finish` is the only way a step closes. Headings, file names and
caps are configured per repository, and brief's own output names the ones in force.
`brief <command> --help` covers every flag.
