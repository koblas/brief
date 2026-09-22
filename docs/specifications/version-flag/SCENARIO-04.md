---
id: SCENARIO-04
status: done
depends-on: []
---

# SCENARIO-04: --version takes no value

## Scenario

```gherkin
Scenario: SCENARIO-04 --version takes no value
  When I run "brief --version=x" or "brief --version="
  Then stderr is exactly: brief: '--version' takes no value; run 'brief --version'
  And stdout is empty and the exit code is 2
```

## User-visible contract

- `brief --version=x`, `brief --version=`: stdout empty, stderr exactly
  `brief: '--version' takes no value; run 'brief --version'`, exit 2.
- `brief --version=x extra`, `brief --version= --version`: the first argument decides (R8), so
  the **value** error above fires, not `takes no arguments`.
- Unchanged (R6), confirmed from a built binary before planning:
  - `brief new --version=x` / `brief new --version=` → `brief new: unknown flag: --version; run 'brief new <type> --help'`, exit 2
  - `brief help --version=x` / `brief help --version=` → `brief help: unknown flag: --version; run 'brief help <command>'`, exit 2
- `brief --versionx` stays `brief: unknown flag: --versionx; run 'brief <command> --help'`.
  Only the exact `--version=` prefix is the version flag given a value.

## Classification decision

Add a new `argKind`, `argVersionFlagWithValue`, for any token starting with `--version=`
(including an empty value). Its `msg` is `unknownLongFlagMessage(arg)`, which is
`unknown flag: --version` because that function already drops the `=value` part. That is
the same `msg` `argVersionFlag` carries, so `runNew` and the help stub can handle the new
kind in their existing `argUnknownFlag, argVersionFlag` arm with no other change, and their
output stays byte-identical. Only root reads the kind for its own copy, and root ignores `msg`.

Rejected: widening `argVersionFlag` to cover `--version=...`. Root would then have to re-read
the raw token to tell the sole-flag case from the value case, which puts classification back
into a caller. Also rejected: reusing `argHelpFlagWithValue`. Its `msg` is the flag name,
which `new` and `help` render as `'--help' takes no value`, and that would break R6.

## Implementation Plan

- [x] Step 1: `internal/cli/run_test.go` `Test_reports_a_version_flag_with_a_value_as_taking_no_value` — black-box `cli.Run` table next to SCENARIO-03's trailing-argument test. Rows: `--version=x`, `--version=`, and the first-argument-decides rows `--version=x extra` and `--version= --version`. Each row asserts `ErrUsage`, exit 2, empty stdout, and the exact one-line stderr (red)
- [x] Step 2: `internal/cli/flag_error_test.go` `Test_classifies_dash_prefixed_tokens_consistently_across_disabled_parsing_sites` — add the root/new/help triple for `"--version=x"` and for `"--version="`. The root rows expect the new takes-no-value copy (red). The `new`/`help` rows are the R6 control rows and pin today's unknown-flag wording above (green on arrival; this is a behavior pin, so say so)
- [x] Step 3: `internal/cli/classify.go` — declare `argVersionFlagWithValue` in `argKind`, with a doc comment covering the msg fold contract. Update `argVersionFlag`'s doc, which currently says `--version=<v>` becomes `argUnknownFlag`. Update the "five" count in the `argKind` doc and in `classifyDashArg`'s doc. `go build` stays green. Until Step 6, `golangci-lint`'s `exhaustive` check fails on the root, `new` and help-stub switches. That is expected in the middle of the sequence and is not a defect (new)
- [x] Step 4: `internal/cli/classify_internal_test.go` `Test_classifyDashArg_classifies_every_token_shape` — add row `"--version=x"` → `argVersionFlagWithValue` with msg `unknown flag: --version`. It fails on the assertion, because the token still classifies as `argUnknownFlag`. Also add control row `"--versionx"` → `argUnknownFlag` with msg `unknown flag: --versionx`, which guards against a too-loose prefix match (red)
- [x] Step 5: `internal/cli/classify.go` `classifyDashArg` — add a `strings.HasPrefix(arg, "--version=")` branch after the exact `--version` check and before the generic `--` branch. It returns `argVersionFlagWithValue` with `unknownLongFlagMessage(arg)` (green for Step 4)
- [x] Step 6: `internal/cli/new.go` `runNew` and `internal/cli/cli.go` `newHelpCommand` — add `argVersionFlagWithValue` to the existing `argUnknownFlag, argVersionFlag` case list. The `exhaustive` lint requires this, and it keeps R6 byte-identical (update)
- [x] Step 7: `internal/cli/cli.go` `takesNoValueMessage(flag, runHint string) string` — unexported sibling of `takesNoArgumentsMessage` that renders `brief: '<flag>' takes no value; run '<runHint>'`. Root only (new)
- [x] Step 8: `internal/cli/cli.go` `runRoot` — move the `argHelpFlagWithValue` arm onto `takesNoValueMessage(msg, "brief --help")`. The existing root `--help=true` / `-h=x` / `-hh=x` / `-hh=` rows must stay green with no edits (refactor under green)
- [x] Step 9: `internal/cli/cli.go` `runRoot` — add an `argVersionFlagWithValue` arm that returns `usageError(stderr, takesNoValueMessage("--version", "brief --version"))` without checking `len(args)`. Use the literal `"--version"`, not `msg` (which is unknown-flag wording) and not `args[0]` (which still has the value). Add a sentence to `runRoot`'s doc comment saying a value on `--version` is reported before any trailing argument is looked at (green for Steps 1–2)
- [x] Step 10: Mutation-verify each check on its own, and restore after each one (stash or `$TMPDIR` copy, then `diff` to prove the file is byte-identical):
  - Delete Step 5's prefix branch. The root rows in Steps 1–2 and the `--version=x` classify row go red. The `new`/`help` control rows stay green.
  - Loosen the prefix to `"--version"` without the `=`. The `--versionx` classify row goes red.
  - Route `runNew`'s `argVersionFlagWithValue` to the bodyless `argNotFlag` arm. Only the `new --version=*` rows go red. Then repeat the same mutation for the help stub, where only the `help --version=*` rows should go red.
  - In the root arm, return `takesNoArgumentsMessage` when `len(args) > 1`. Only the `--version=x extra` and `--version= --version` rows go red.
  - Change `takesNoValueMessage`'s wording. The root `--help=`/`-h=` rows and the root `--version=` rows both go red, which proves the helper is shared.
  - Record each mutation and the test it turned red in the test doc comments, following the SCENARIO-03 style.
- [x] Step 11: `go build ./...`, `go test ./...` (unpiped; report the count and the delta), `go test -race ./internal/cli/...`, `golangci-lint run ./...` (`exhaustive` must be clean). Then mark SCENARIO-04 done in `specification.md`

## Handoff

**Binding decisions** — a later scenario must not contradict these without saying so:
- `classifyDashArg` returns `argVersionFlagWithValue` for any `--version=` prefix, including an empty value, with msg `unknownLongFlagMessage(arg)` = `unknown flag: --version`. This is because `new` and the help stub fold it into their unknown-flag arm, and R6 needs their bytes unchanged. S06's byte-identity table relies on this.
- Root's `argVersionFlagWithValue` arm never looks at `len(args)`. A value on the first argument always wins over trailing arguments (R8), and this is pinned by the `--version=x extra` row.
- `takesNoValueMessage(flag, runHint)` is root-only, in `internal/cli/cli.go`, with the `brief: ` prefix built in. It is shared by root's `argHelpFlagWithValue` (`"brief --help"`) and `argVersionFlagWithValue` (`"brief --version"`) arms. `runNew` and the help stub keep their own inline takes-no-value copy.
- `argKind` now has six values. The `exhaustive` lint makes every switch on `classifyDashArg` name all six. Those switches are exactly root `runRoot`, `runNew` and the help stub `newHelpCommand`. A grep of `internal/` and `cmd/` found no other switch on `argKind`, and the classify table and fuzz test do not branch on kind.

**Left unbuilt** — named so nobody assumes it exists:
- `-v`, `-v=x`, `-vh`, `-hv` pinning tests: SCENARIO-05. No code change is expected, because these already produce unknown-shorthand wording.
- The leaf `--version` byte-identity rows (`start --version demo`) and a bare `help --version` row: SCENARIO-06. This scenario adds only the `new`/`help` `--version=x` and `--version=` rows.
- The root-help trailer line: SCENARIO-07.

**Traps** — things that look right and are not:
- Rendering root's value error from `msg` gives `'unknown flag: --version' takes no value`, and rendering it from `args[0]` gives `'--version=x' takes no value`. Both are wrong. The flag name has to be the literal `--version`.
- `strings.HasPrefix(arg, "--version")` without the `=` catches `--versionx` and `--version-foo`. Only the `--versionx` classify row catches that mistake.
- The `new`/`help` `--version=*` rows pass before any change is made. They are R6 behavior pins, not evidence of red. Their guard evidence is the per-site `argNotFlag` routing mutation in Step 10.
- `unknownLongFlagMessage` already removes `=value`, so `--version=x` and `--version` share the same `msg`. Tests cannot tell `argVersionFlag` and `argVersionFlagWithValue` apart at `new`/`help`, and they are not supposed to be able to. Only the classify table and root tell the two kinds apart.
