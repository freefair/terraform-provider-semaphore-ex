# Terraform Provider: Semaphore EX

Manage Semaphore EX projects, templates, variable groups, inventories, credentials, workflows, access policies, notifications, identity settings and executor policies.
The provider source address is `freefair/semaphore-ex`; resource and data-source names start with `semaphore_ex_`.
This fork derives from the Semaphore UI provider and targets the EX API contracts.

## Quick Start

Prerequisites: Semaphore EX `v2.20.0-ex.2` or a compatible newer server, its API token, and Terraform `1.15.2` or newer.
Provider `v1.0.2` includes the native EX resources and task SSH-key selection.

Create `main.tf`:

```hcl
terraform {
  required_version = ">= 1.15.2"
  required_providers {
    semaphore = {
      source  = "freefair/semaphore-ex"
      version = "= 1.0.2"
    }
  }
}

provider "semaphore" {}

resource "semaphore_ex_project" "example" {
  name = "Terraform example"
}
```

Supply `SEMAPHOREUI_API_BASE_URL` with the intended server's API address and
`SEMAPHOREUI_API_TOKEN` through your environment or secret manager.
Run `terraform init`, inspect `terraform plan`, then run `terraform apply`.
Verify with `terraform plan -detailed-exitcode`: an unchanged configuration returns 0.
For local provider development, see [Using a local build](#using-a-local-build).

## Multiple variable groups

```hcl
resource "semaphore_ex_project_template" "deploy" {
  project_id      = semaphore_ex_project.example.id
  name            = "Deploy"
  playbook        = "deploy.yml"
  repository_id   = semaphore_ex_project_repository.git.id
  inventory_id    = semaphore_ex_project_inventory.hosts.id
  environment_ids = [
    semaphore_ex_project_environment.common.id,
    semaphore_ex_project_environment.production.id,
  ]
}
```

`environment_ids` is a set because EX persists unique memberships and returns them in ascending ID order.
Use `[]` to detach all groups.
The deprecated `environment_id` remains available for a single group; configure exactly one of these attributes.
Configuration order does not control the server's variable precedence.

## EX compatibility

- Membership `revision` is computed by the server, stored by the provider, and echoed on update; concurrent changes remain conflicts.
- Schedule updates accept EX HTTP 200 and preserve their optional `timezone`.
- Templates preserve `working_directory`, `executor_image`, `suppress_error_alerts`, and runner-tag placement settings when omitted after import.
- Runner `registration_policy` supports `standard` and `secure` and survives refresh/import and unrelated updates.
- Runners are created inactive; use `semaphore_ex_runner_registration_token` for one-time registration before enabling a runner.
- Runner resources expose durable settings, not nonexistent `private_key` or redacted `token` outputs.

See [migration](docs/migration.md) before changing an existing Terraform configuration.
See [EX feature usage](docs/ex-features.md) for the additional resources, immutable versions, explicit Actions and lifecycle semantics.

## Development and verification

```sh
task build
task test
task lint
task generate
SEMAPHORE_EX_TEST_BINARY=/absolute/path/to/semaphore-ex/bin/semaphore task testacc
```

The acceptance fixture creates a temporary SQLite database and loopback-only EX server, seeds a synthetic administrator and ephemeral API token, exercises Terraform CRUD/import, and cleans up its own instance.
It retains diagnostic files on failure.
Without `TF_ACC`, acceptance tests are skipped; `task test` alone is not an API compatibility check.
CI builds the exact EX revision recorded in `.github/workflows/test.yml` and runs the complete acceptance suite.

`api-docs.yml` is the source of the generated client under `semaphoreui/`.
Regenerate with go-swagger `v0.36.6` and `task client`; generated Go files are not edited by hand.
The implementation uses Terraform Plugin Framework (versions in `go.mod`); see [the design decision](docs/adr/0001-semaphore-ex-provider.md).

### Using a local build

Build with the Go version in `go.mod` and keep the development override local to
this checkout:

```sh
git clone https://github.com/freefair/terraform-provider-semaphore-ex.git
cd terraform-provider-semaphore-ex
mkdir -p bin local
go build -o "$PWD/bin/terraform-provider-semaphore-ex" .
cat > local/terraform.rc <<EOF
provider_installation {
  dev_overrides { "freefair/semaphore-ex" = "$PWD/bin" }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="$PWD/local/terraform.rc"
```

Put a disposable Terraform configuration under `local/` and supply the intended
API endpoint and token. With a development override, run
`terraform -chdir=local plan` and `terraform -chdir=local apply` directly without
attempting a Registry installation. Remove the override to return to the pinned
Registry release.

## Security and release

Keep API tokens in the environment or a secret manager, and protect Terraform state: sensitive attributes are still stored in state.
Existing `SEMAPHOREUI_API_BASE_URL`, `SEMAPHOREUI_API_TOKEN` and `SEMAPHOREUI_TLS_SKIP_VERIFY` environment names remain supported.
TLS verification is enabled by default.

Conventional Commits drive release-please version/changelog PRs.
Main CI produces signed downloadable bundles; pushed version tags automatically publish verified `terraform-provider-semaphore-ex` ZIPs, a Registry manifest, checksums and a detached GPG signature.
The workflows use the existing Freefair organization signing secrets.
See [signed builds and Registry publication](docs/releases.md) for artifact verification, tag releases and initial Registry onboarding.
This provider is maintained on a best-effort basis with AI assistance; contributions are welcome.

## License

See [LICENSE](LICENSE).
