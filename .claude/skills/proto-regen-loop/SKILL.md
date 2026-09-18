---
name: proto-regen-loop
description: Regenerate all proto-derived code (Connect Go, ogen pubapi, openapi yaml, openapi-typescript) after editing any .proto file. Auto-invokes when user edits files under `protos/**`, runs `buf generate`, sees stale `pubapi.APIV1*` errors, or asks to "regen / regenerate / rebuild generated code". Knows the correct cwd per step, in what order, and how to verify each stage landed.
allowed-tools: Bash, Read, Edit
---

# Proto Regen Loop

Repo split proto gen across three roots, two languages. Miss step = consumers break with confusing errors.

## When To Invoke

Run skill when ANY true:

- File under `protos/api/` or `protos/core/` or `protos/oidcas/` edited.
- `go build ./...` reports `undefined: pubapi.APIV1*` or `cannot use ... as pubapi.*Handler`.
- New RPC added but `frontend/src/gen/openapi_*.ts` lacks it.
- ogen struct field gone from `go/gen/pubapi/`.
- User says "regen protos" / "rebuild generated code" / "openapi out of sync".

## What Lives Where

| Source                     | Tool                                                         | Output                                 | Consumers                 |
| -------------------------- | ------------------------------------------------------------ | -------------------------------------- | ------------------------- |
| `protos/core/**/*.proto`   | `buf generate` (from `go/`)                                  | `go/gen/core/**/` (Connect + protobuf) | Go services               |
| `protos/api/**/*.proto`    | `buf generate --template buf.gen.app.yaml` (from `papi/`)    | `papi/openapi_app.yaml`                | ogen + openapi-typescript |
| `protos/oidcas/**/*.proto` | `buf generate --template buf.gen.oidcas.yaml` (from `papi/`) | `papi/openapi_oidcas.yaml`             | ogen + openapi-typescript |
| `papi/openapi_app.yaml`    | `go generate ./gen` (from `go/`)                             | `go/gen/pubapi/oas_*.go`               | Go pubapi handlers        |
| `papi/openapi_oidcas.yaml` | `go generate ./gen` (from `go/`)                             | `go/gen/oidcas/oas_*.go`               | Go oidcas handlers        |
| `papi/openapi_*.yaml`      | `npm run openapi:gen` (from `frontend/`)                     | `frontend/src/gen/openapi_*.ts`        | SPA                       |

## The Full Loop (run in order)

```bash
# 1. Lint the proto (catches obvious bugs early).
cd protos && buf lint
# (Pre-existing buf-deps warnings are noise; only flag NEW errors that
# name your file.)

# 2. Regenerate Connect Go for core/*. Also re-mints minimock stubs.
cd go && make protos

# 3. Regenerate OpenAPI yaml for both app + oidcas.
cd papi && make

# 4. Regenerate Go ogen handlers from the yaml.
cd go && go generate ./gen

# 5. Regenerate the TypeScript types for the SPA.
cd frontend && npm run openapi:gen
```

## New Connect Service: Extra Minimock Step

When `protos/core/X/v1/` brand new, `make protos` gen package but NOT mint mocks unless `go/Makefile` list package. Fix once (Makefile edit), mocks land every future `make protos`. One-off mock mint:

```bash
cd go/gen/core/X/v1/Xv1connect && minimock -s _mock.go
```

## Verify Each Stage

After each step, verify next consumer compile:

```bash
# After step 2:
cd go && go build ./gen/core/...

# After step 3:
ls papi/openapi_app.yaml papi/openapi_oidcas.yaml

# After step 4:
cd go && go build ./gen/pubapi/ ./gen/oidcas/

# After step 5:
cd frontend && npx tsc --noEmit
```

## Common Failure Modes

1. **TS still shows old shape after proto change** — yaml regen but TS not. Step 5 skipped. Yaml = source of truth `openapi-typescript` read.

2. **`undefined: pubapi.APIV1FooRequest`** — proto field rename or message delete. Either:
   - Regen done but handler stale. Update handler match new field names.
   - Regen incomplete. Re-run step 4.

3. **`cannot use *Server as pubapi.XHandler (wrong type for method APIV1XServiceY)`** — ogen changed response shape (typical when operation gain extra response — `*Response` becomes `Res` interface). Update handler sig return `pubapi.APIV1XServiceYRes` not `*pubapi.APIV1XResponse`. Hit every handler whose RPC has `default:` error response in proto.

4. **`make protos` skips new package** — `go/Makefile` `protos:` target has explicit minimock lines per package. Add yours:

   ```
   (cd $(PROTO_BUILDDIR)/core/X/v1/Xv1connect && minimock -s _mock.go )
   ```

5. **Cwd confusion** — `buf generate` resolves `inputs.directory` relative to the cwd. Steps 2 + 3 + 4 MUST run from the directory shown above. See the `pubapi-gen-cwd` skill for the cwd discipline.

## Anti-Patterns

- Running `buf generate` from the repo root: silent no-op or wrong-input behavior.
- Running `go generate` before `make` in `papi/`: stale yaml -> stale Go gen.
- Running `npm run openapi:gen` before `make` in `papi/`: SPA types lag backend.
- Skip step 1: invalid proto regen fine into broken Go, find bug at handler-write time.