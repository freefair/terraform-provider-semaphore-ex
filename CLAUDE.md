# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Terraform provider for [SemaphoreUI](https://semaphoreui.com/), built on the terraform-plugin-framework (not the legacy SDKv2). Targets provider source `freefair/semaphore-ex` with resource prefix `semaphore_ex_`. Registry publication is a separate release step.

Conventional Commits drive release-please (CHANGELOG.md + version bumps), so commit messages matter.

## Common commands

The project uses [Task](https://taskfile.dev) (`Taskfile.yml`). Common targets:

- `task build` — `go build -v ./...`
- `task fmt` — `gofmt -s -w -e .`
- `task lint` — `golangci-lint run --tests=false` (tests are intentionally excluded)
- `task test` — unit tests: `go test -v -cover -timeout=120s -parallel=10 ./internal/...`
- `task testacc` — acceptance tests against an isolated Semaphore EX binary (see below)
- `task generate` — regenerates `docs/` via tfplugindocs (CI fails if the diff is non-empty)
- `task client` — regenerates `semaphoreui/` API client from `api-docs.yml` via `go-swagger` (requires `swagger` binary)
- `task docker:start` / `task docker:stop` — bring the test environment up/down manually

Run a single test:
```
go test -v -run TestAcc_ProjectResource_basic ./internal/provider/
```

For acceptance tests, `TF_ACC=1` plus the `SEMAPHOREUI_*` env vars must be set — `task testacc` does this for you. Tests named `TestAcc_*` require the live API.

## Acceptance test environment

`SEMAPHORE_EX_TEST_BINARY=/absolute/path/to/semaphore task testacc` starts a temporary loopback-only EX server with its own SQLite database and generated credentials.
The test fixture owns and cleans up that process and database; failure logs are retained.
CI builds Semaphore EX `v2.20.0-ex.2` at the exact revision pinned in `.github/workflows/test.yml`.
For an existing explicitly authorized test instance, set `TF_ACC=1`, `SEMAPHOREUI_API_BASE_URL`, and `SEMAPHOREUI_API_TOKEN` and run `go test` directly.
Do not infer API compatibility from `task test`: acceptance tests are skipped without `TF_ACC`.

## Architecture

```
main.go                  — providerserver entrypoint; version injected by goreleaser
internal/provider/       — all resources, data sources, schemas, and tests (single package)
internal/stringvalidator — custom validators (e.g. cron format)
semaphoreui/client/      — GENERATED go-swagger HTTP client (do not hand-edit)
semaphoreui/models/      — GENERATED request/response models
api-docs.yml             — upstream OpenAPI 2.0 spec, source of truth for the client
tools/                   — separate Go module hosting tfplugindocs for `task generate`
examples/                — TF examples consumed by tfplugindocs to render `docs/`
```

### Resource/data-source structure

Each Terraform type follows a three-file convention in `internal/provider/`:

- `<name>_schema.go` — defines the `<Name>Model` struct + a `<Name>Schema()` that returns a `superschema.Schema` (from `orange-cloudavenue/terraform-plugin-framework-superschema`). The superschema lets one definition serve both resource and data-source variants by tagging attributes with `Resource:` / `DataSource:` / `Common:` overrides.
- `<name>_resource.go` — implements `resource.Resource` with `Configure`, `Schema` (delegates to the schema file), `Create`, `Read`, `Update`, `Delete`, and `ImportState`.
- `<name>_data_source.go` — implements `datasource.DataSource`.

Resources and data sources are wired up by adding constructors to `Resources()` / `DataSources()` in `provider.go`. A new resource means: schema file, resource file, registration in `provider.go`, an `examples/resources/<name>/` directory, and a test file. Then `task generate` produces the matching `docs/` page.

### Client wiring

`provider.go` `Configure()` builds a `go-openapi/runtime/client` httptransport with bearer-token auth from `SEMAPHOREUI_API_TOKEN` and supplies the generated `*apiclient.SemaphoreUI` as data-source, resource and Action configuration. Each implementation checks that type in `Configure`.

The OpenAPI-generated client splits endpoints into per-resource sub-clients (driven by `tags` in `api-docs.yml`). The `SemaphoreUI` struct exposes:

```
Authentication, Integration, Inventory, KeyStore, Operations,
Project, Repository, Schedule, Task, Template, User, VariableGroup.
```

`VariableGroup` owns the environment endpoints — non-obvious because the resource is `semaphore_ex_project_environment`. Operations for templates, inventories, keys, repositories, and schedules live on their dedicated sub-clients, not on `Project`. When grepping for a new operation, search by HTTP path (e.g. `PostProjectProjectIDInventory`) rather than guessing the sub-client.

The provider supports `tls_skip_verify` for self-signed TLS; if set, `Configure` constructs an `http.Client` with `InsecureSkipVerify` and hands it to `httptransport.NewWithClient`. The host string omits the port when it matches the scheme's default (`:443` for https, `:80` for http) — some upstream proxies reject SNI/Host headers that include the default port (issue #56).

### Import IDs

Nested resources use slash-delimited compound IDs like `project/1/template/2`. `internal/provider/import.go` `parseImportFields` parses these via a `(\w+)/(\d+)` regex into a `map[string]int64`, and legacy resources call it from `ImportState`. Native EX records use their explicit import labels and declared attribute types, including opaque string identifiers such as role and backend-alias IDs. Each `examples/resources/<name>/import.sh` documents the format.

### Nil-handling pattern

Several upstream API responses return `nil` for fields that should be zero/false (a known SemaphoreUI quirk — see commit `b345643` on `max_parallel_tasks`). When mapping API responses to Terraform models, explicitly check for `nil` pointers and substitute the zero value rather than using `types.Int64PointerValue` directly. `convertProjectResponseToProjectModel` in `project_resource.go` is the canonical example.

### Nullability patches in `api-docs.yml`

The local `api-docs.yml` is a *patched* copy of the upstream spec from a tagged release (currently `v2.18.6`). Upstream tends to drop nullable annotations from fields it considers always-set, but the Semaphore API genuinely returns `null` for several optional fields. The patches re-add `x-nullable: true` for those:

- `Project.alert_chat` / `ProjectRequest.alert_chat`
- `ViewRequest.id` — upstream omits it, but the PUT views endpoint returns 400 without it

Runner detail GETs return `Runner`, without credentials. Create responses and the separate registration-token endpoint have distinct one-time credential semantics; resource state stores only durable runner configuration.

When bumping `api-docs.yml`, re-import upstream verbatim first (one commit), then re-apply the nullability patches based on which tests fail (a follow-up commit). The two-commit split keeps the diff legible — reviewers can see what came from upstream versus what we patched locally.

### EX contracts

Templates accept the set `environment_ids` or deprecated singular `environment_id`.
SQL returns unique memberships in ascending ID order, so use set semantics and retain the full collection on refresh and import.
Optional/computed EX settings preserve imported values when configuration omits them.
Project-user revision is computed from the server response and prior state is echoed on updates; do not fetch a fresh revision or retry 409 automatically.
Runner resources expose durable settings and registration policy; the dedicated registration-token resource owns one-time registration credentials.
See `docs/adr/0001-semaphore-ex-provider.md` and `docs/migration.md`.

Native EX implementations reuse this transport through `exRequest` with fixed routes.
Resource-specific lifecycle handlers preserve revisions, parent scope and redacted values.
Ordinary compatible records share `ex_record`; it is not an arbitrary HTTP resource.
Actions are registered in `Actions()` and execute only through `Invoke`.
One-time credentials belong in sensitive resource outputs, not Action progress messages.
See `docs/ex-features.md` and `docs/adr/0003-native-ex-coverage.md`.

Use `TestProviderRegisteredSchemasAreValid` to verify the real protocol schema.
Dynamic attributes cannot be nested inside collection elements in this framework;
use static object schemas or typed scalar unions there.
Mutable computed revision fields must remain unknown during planning, since the
server increments them on writes; `UseStateForUnknown` would make apply inconsistent.

### Task SSH bindings

The project SSH policy is a separate singleton so a project, its generated keys and the policy form an acyclic Terraform graph. It owns default/always selections and imports by numeric project ID.
Template `ssh_keys` uses an explicit `inherit` discriminator: true has null bindings, false requires a list including empty. Omission preserves state; unknown key IDs must remain valid in planning. Send the selection in the same template mutation as all other fields.
Task-start Action `ssh_keys` is a direct optional list. The server owns permission, key-scope and host-routing validation. See `docs/adr/0005-task-ssh-key-bindings.md` and `docs/ex-features.md`.

### Environment secret update gotcha

The Semaphore API does not honor type changes on secret update operations — only `name` and `secret` (value) are persisted. `convertProjectEnvironmentModelToEnvironmentRequest` sets the `Secret` field when marking an update; without it, the API treats the update as a no-op for the value (issue #68 / PR #74). Type changes are not supported in-place; the resource's acceptance tests reflect this.

## Regenerating code

- **API client** (after `api-docs.yml` changes): `task client`. Requires `swagger` binary from `go-swagger` (`go install github.com/go-swagger/go-swagger/cmd/swagger@v0.36.6`).
- **Docs** (after schema changes): `task generate`. Requires `terraform` in PATH (for `terraform fmt`). CI's `generate` job fails if the resulting diff isn't committed.

## Tooling notes

- Go version: 1.26.6 (see `.tool-versions` and `go.mod`).
- Linter: `golangci-lint` v2, config at `.golangci.yml` — `forcetypeassert`, `errcheck`, `staticcheck`, etc. enabled; tests excluded from lint.
- Pre-commit hooks (`.pre-commit-config.yaml`) run golangci-lint, end-of-file-fixer, and `terraform fmt`. **Pre-commit lint runs on every commit** — if the code doesn't compile or lint, the commit is blocked. This means broken-intermediate-state commits aren't possible; bundle dependent changes (e.g. client regen + provider reconciliation) into one commit.
- Documentation generation is in a separate module (`tools/go.mod`) so tfplugindocs dependencies don't bloat the main module.
- Dependabot auto-merge (`.github/workflows/dependabot-auto-merge.yml`) auto-approves and squash-merges patch/minor/security PRs but leaves majors for human review. For grouped PRs, `fetch-metadata` reports the highest semver bump — any group containing a major won't qualify.

## Signed Registry builds

`main` CI checks produce signed prerelease bundles; `v*` tags on main-history commits build and publish exact-version assets after verification.
Release Please prepares version/changelog PRs only, avoiding token-suppressed release workflows and incomplete public releases.
The artifact workflow is reusable and manually dispatchable for existing tags.
Use the `FREEFAIR_TERRAFORM_PRIVATE_KEY`, `FREEFAIR_TERRAFORM_PASSPHRASE`, and `FREEFAIR_TERRAFORM_PUBLIC_KEY` organization secrets without reading private values.
Require the Registry signing key ID `719010B911115D8E`; verify against the separately configured public key.
Signing passes the passphrase via stdin; verification uses a fresh public-key-only keyring and exact fingerprint.
GoReleaser is pinned to the version in `.tool-versions` and the workflow; keep those pins aligned.
Verify ZIP names, checksum coverage, manifest protocol, signature and packaged Terraform startup before uploading assets.
Existing release assets must match on rerun; never overwrite a published version.
See `docs/releases.md` and `docs/adr/0002-signed-registry-artifacts.md`.
