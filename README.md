# Terraform Provider: Semaphore EX

Manage Semaphore EX projects, variable groups, templates, inventories, keys, repositories, schedules, users, integrations, views and runners.
The provider source address is `freefair/semaphore-ex`; resource and data-source names start with `semaphore_ex_`.
This fork derives from the Semaphore UI provider and targets the EX API contracts.

## Quick Start

Prerequisites: a running Semaphore EX instance, its API token, Terraform, Go matching `go.mod`, and Git.
Until an EX release is published to the Terraform Registry, use a local build:

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
# Supply SEMAPHOREUI_API_TOKEN through your secret manager/environment.
export SEMAPHOREUI_API_BASE_URL="http://localhost:3000/api"
```

Create `local/main.tf`:

```hcl
terraform {
  required_providers {
    semaphore = { source = "freefair/semaphore-ex" }
  }
}
provider "semaphore" {}
resource "semaphore_ex_project" "example" {
  name = "Terraform example"
}
```

Run `terraform -chdir=local plan`, inspect the target and changes, then `terraform -chdir=local apply`.
With a development override, run these commands directly without `terraform init` attempting a Registry installation.
Verify with `terraform -chdir=local plan -detailed-exitcode`: an unchanged configuration returns 0.
After Registry publication, remove the development override and pin a published release in `required_providers`.

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
The provider does not yet manage EX workflows, policy guardrails, deployment windows, or template versions/grants.

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

## Security and release

Keep API tokens in the environment or a secret manager, and protect Terraform state: sensitive attributes are still stored in state.
Existing `SEMAPHOREUI_API_BASE_URL`, `SEMAPHOREUI_API_TOKEN` and `SEMAPHOREUI_TLS_SKIP_VERIFY` environment names remain supported.
TLS verification is enabled by default.

Conventional Commits drive release-please; GoReleaser produces `terraform-provider-semaphore-ex` archives and signed checksums.
Registry publication and signing-key registration are separate from renaming the GitHub repository.
This provider is maintained on a best-effort basis with AI assistance; contributions are welcome.

## License

See [LICENSE](LICENSE).
