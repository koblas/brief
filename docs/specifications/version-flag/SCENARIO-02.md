---
id: SCENARIO-02
status: done
---

# SCENARIO-02: A build with no stored version reports (devel)

## Scenario

```gherkin
Scenario: SCENARIO-02 A build with no stored version reports (devel)
  Given brief's build info has version "(devel)", an empty version, or no build info at all
  When I run "brief --version"
  Then stdout is exactly "brief (devel)" followed by a newline, and the exit code is 0
```

User-visible contract: `brief --version` → stdout exactly `brief (devel)\n`, stderr empty,
exit 0 (`run`/`Run` returns nil) for each of the three inputs. No failure class exists here —
asking for the version never fails (R2).

## Implementation Plan

Existence facts (from `grep`, not whole-file reads): `versionLine` at
`internal/cli/cli.go:501` dereferences `info` unconditionally and has no fallback;
`Test_version_flag_prints_the_stored_module_version_verbatim` in
`internal/cli/version_internal_test.go` is a table whose expected value is derived from the
row's `stored` field and whose reader always returns `ok=true`;
`Test_version_flag_through_Run_prints_one_brief_line_to_stdout` in `internal/cli/run_test.go`
asserts only a `brief ` prefix.

- [x] Step 1: `internal/cli/version_internal_test.go` `Test_version_flag_reports_devel_when_the_build_stored_no_version` — sibling table through the `run` seam (the S01 table's expected value is `"brief "+stored` and its reader is fixed at `ok=true`, so the shape differs); each row supplies its own fake `readBuildInfo`; every row asserts nil error, empty stderr, and stdout equal to the **literal** `"brief (devel)\n"` — never built from a production constant or `versionLine`'s output. Rows: `Main.Version "(devel)"`; `Main.Version ""`; reader returns `(nil, false)`; reader returns `ok=false` with a non-nil info carrying a real-looking version (the non-panicking arm that proves `ok` is honoured and not just nil-checked) ; also scope `version_internal_test.go`'s file-level comment (line 3, "never reports the real module version") so it no longer reads as the preamble to both tables (red — `""` prints `brief \n`, `ok=false`+info prints the version, `(nil,false)` panics. A panic kills the test binary, so observe each row's red separately with `go test -run 'Test_version_flag_reports_devel_when_the_build_stored_no_version/<row>' ./internal/cli/`; the `(nil,false)` row's red is a crash, not an assertion failure — say so in the report)
- [x] Step 2: `internal/cli/cli.go` `versionLine` — return the fallback when `ok` is false (checked before any dereference of `info`) or `Main.Version` is empty; otherwise pass through verbatim; rewrite the doc comment as the contract (verbatim, or `(devel)` when the build stored none — no scenario ids) (green)
- [x] Step 3: `internal/cli/run_test.go` `Test_version_flag_through_Run_prints_one_brief_line_to_stdout` — replace the prefix check with an exact assertion that stdout equals the literal `"brief (devel)\n"`; rewrite its doc comment (the test binary's reader reports `(devel)`, so the exact value is now assertable; drop the "or \"\"" and "belongs to S02" wording) (update)
- [x] Step 4: mutation-verify each guard individually (stash per `.claude/rules/agent-briefs.md`, restore, prove byte-identical), naming the test+row that reddens; run each with a `-run` filter per row, because a panicking row aborts the rest of the package run:
  - (a) drop the `ok` check (keep the empty check) → Step 1's `ok=false`-with-info row reddens (prints the stored version) and the `(nil,false)` row reddens (crash, observed under its own `-run` filter)
  - (b) drop the empty-version check (keep the `ok` check) → Step 1's `""` row reddens (`brief \n`)
  - (c) change the `(devel)` literal inside `versionLine`'s fallback → Step 1's `""`, `(nil,false)` and `ok=false` rows redden; the `"(devel)"` row stays green, which is expected (see Traps)
  - (d) replace S01's mutation 11(a) — make `Run` pass a fake reader returning a distinctive non-devel version instead of `debug.ReadBuildInfo` → Step 3's test reddens. Do NOT re-run the old "reader returns `(nil,false)`" mutation as evidence: after Step 2 it prints `brief (devel)\n`, which Step 3 accepts, so it no longer reddens anything
- [x] Step 5: `go build ./...`, `go test ./...` (unpiped, report count and delta — expect +1 top-level test func and +4 subtests; the Run test's count is unchanged), `go test -race ./internal/cli/...`, `golangci-lint run ./...` all clean → check SCENARIO-02 in `specification.md`; rewrite `STATE.md` (remove the nil-deref debt, the "no fallback guard" decision, and the 11(a) dependency)

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `versionLine` returns the `(devel)` fallback when `ok == false` (regardless of `info`) or
  `Main.Version == ""`; any other value passes through verbatim — R1/R2; S01's verbatim table
  and this scenario's fallback table both pin it.
- `ok` is checked before `info` is touched — `(nil, false)` from the reader was a reachable
  production nil-pointer panic.
- The fallback text is an inline literal in `versionLine` (one use site; no constant) — a
  constant would tempt tests to assert against it and make mutation (c) vacuous. No
  `internal/version` package — version text stays in `internal/cli` (S01 decision, unchanged).
- `Test_version_flag_through_Run_prints_one_brief_line_to_stdout` asserts exact
  `brief (devel)\n` — it depends on the `go test` binary's reader returning `(devel)` (STATE,
  confirmed in S01). If a future toolchain changes that, this test fails first.

**Left unbuilt** — named so nobody assumes it exists:
- No explicit `Main.Version == "(devel)"` branch — verbatim pass-through already yields the
  right output; adding one would be an unfalsifiable guard.
- `(info=nil, ok=true)` is not guarded — outside R2 (`debug.ReadBuildInfo` never returns it).
  Contrast `ok=false` with non-nil info, which IS tested: `ok=false` is R2's own "no build
  info" input, so the guard honours `ok` regardless of `info`.
- Everything S01's STATE assigned to SCENARIO-03..07 is untouched.

**Traps** — things that look right and are not:
- The `"(devel)"` row cannot redden under any `versionLine` guard mutation: pass-through and
  fallback produce the same bytes. It is a behavior pin, not guard evidence — only the `""`,
  `(nil,false)` and `ok=false`-with-info rows prove the guards.
- S01's mutation 11(a) (wire `Run` to a `(nil,false)` reader, expect the Run test to redden)
  is now unfalsifiable — the guard turns the panic into the same `brief (devel)\n` a real test
  binary prints. Use mutation (d) above instead.
- A guard written as `info == nil` instead of `!ok` passes the `(nil,false)` row but fails the
  `ok=false`-with-info row — that row exists to catch exactly that.
