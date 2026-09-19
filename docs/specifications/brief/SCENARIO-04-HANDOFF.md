**Binding decisions** — a later scenario must not contradict these without saying so:

- **`internal/assemble` is the read-side feature package**; `status`, `next`, `show`, `handoff`
  and `state get` are further `Server` methods on it, not new packages — the dependency rule
  forbids feature packages importing each other, and they all need the same step-enumeration
  code. `finish` writes and is unsettled; SCENARIO-05 chooses.
- **`acceptance-heading` (default `"## Scenario"`) is a new, OPTIONAL config key, and
  `scaffold.stepSkeleton` was deliberately NOT changed** — R2's malformed list omits acceptance
  criteria, SCENARIO-03's Gherkin enumerates what the scaffold writes, and SCENARIO-02 forbids
  the tool writing prose. SCENARIO-14 owns naming the shortfall when the heading is absent;
  extraction happens whenever the heading is present, regardless of `optional-conventions`.
- **`markdown.Section` ends a section at the next heading of the same-or-higher level and
  ignores `#` lines inside ``` / ~~~ fences.** `show` (R14's "unknown section lists that file's
  headings") and `finish`'s splice must use the same rule.
- **Next = lowest step NUMBER whose `status:` is not `done`**, numeric not lexicographic, from
  `stepfile.Number`. `depends-on` is parsed and ignored for ordering until R4's phase.
- **Everything not exactly `done` (trim + case-fold) counts as open**, including `blocked` —
  SCENARIO-09's blocked count must revisit this rather than assume two states.
- **`Brief.Step` is a pointer; nil means no next step.** SCENARIO-12's empty stdout and
  SCENARIO-15's `"step": null` both key off it.
- **`Brief.Inherited` carries all four state sections including empty ones**; only `RenderText`
  skips empties. SCENARIO-15's JSON must encode the loaded struct, not the text.
- **`start` refuses a feature with no state file** (`ErrMalformedFeature`, empty stdout, exit 1)
  and reads the state file only — R6's synthesis fallback is not built.
- **stdout shape is the architect's choice, not the spec's** — the specification fixes no
  literal `start` output. Pinned: `<id> — <D> done, <O> open`, blank, `# <title>`, then each
  section under its configured heading verbatim.

**Left unbuilt** — named so nobody assumes it exists:

- `assemble.Server.Status`, `.Next`, `.Show`, `.Handoff`, `.StateGet` — SCENARIO-09/…; `--json`
  and `Brief`'s JSON tags — SCENARIO-15; the complete-feature stderr line and exit-0 contract —
  SCENARIO-12.
- `markdown.Headings(body) []string` (for `show --list` and R14's heading enumeration) and any
  checklist-item parser (`- [ ]` scanning) — SCENARIO-20/22 own them.
- `assemble`'s errors carry **no** `RefusalError` shape and no user-facing copy — SCENARIO-13.
- **`start` validates only the state file's existence.** It does not check the specification
  file, the progress list, or any step's handoff anchor, even though R2 lists all of them as
  structural. SCENARIO-13 owns all three — and SCENARIO-13's own Gherkin (a step with no handoff
  anchor) is exactly the case this scenario deliberately lets through.
- R13 output truncation: `Brief` has no budget field and `RenderText` no cap.
- Known-feature enumeration in the unknown-feature error — SCENARIO-09.

**Traps** — things that look right and are not:

- **SCENARIO-13's real cost:** the read-refusal template needs `RefusalError` visible to
  `assemble`, and `assemble` must not import `internal/scaffold`. That means moving
  `scaffold.RefusalError` down to a platform package — which touches every construction site in
  `scaffold` plus `cli/refusal.go`'s `errors.AsType[*scaffold.RefusalError]` branch. Until then
  `assemble.ErrNoSuchFeature`/`ErrMalformedFeature` deliberately duplicate scaffold's sentinels.
- **Nothing binds `scaffold.stepSkeleton`'s frontmatter literal to
  `stepfile.ParseFrontmatter`.** The only proof is that the SCENARIO-04 fixture writes the
  frontmatter byte-for-byte as `stepSkeleton` emits it. A round-trip test is the durable fix and
  is unowned.
- **`scaffold.insertProgressEntry` ends a section at ANY `#` line and is not fence-aware**,
  unlike `markdown.Section`. Two rules now exist in the tree. It is the write path; do not
  "unify" them opportunistically — SCENARIO-05's splice must keep its own behaviour. Debt,
  unowned.
- **`root = wd` unless a `.brief.yaml` is found**, and there is none in this repo, so `brief
  start` works from the repo root only — `runStart` must compute root exactly as `new.go` does.
- A disk-unchanged sweep with no control arm proves nothing; Step 23 is the control and must not
  be dropped as redundant.
