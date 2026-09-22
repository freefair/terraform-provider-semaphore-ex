# Known limitations

## Write-only Action inputs

The provider retains the official Terraform Plugin Framework v1.19.0 and its
write-only input declarations. That framework incorrectly rejects non-null
write-only Action inputs during configuration validation, before API invocation.
The diagnostic is `WriteOnly Attribute Not Allowed`. Its suggestion to upgrade
to Terraform 1.11 is misleading: the same failure is reproduced with Terraform
1.16.0 and 1.16.3.

| Action (prefix `semaphore_ex_`) | Affected inputs | Current behavior |
| --- | --- | --- |
| `ldap_test` | `provider_id`, `username`, `password`, `recovery_admin_password` | Required inputs prevent invocation. |
| `option_set` | `value` | Required input prevents invocation. |
| `ldap_group_apply` | `preview_token` | Required input prevents invocation. |
| `oidc_group_preview` | `claim` | Required input prevents Action invocation; the matching ephemeral preview works. |
| `project_task_start` | `preflight_token`, `secret` | Fails when either input is set. Calls that require no such inputs are unaffected. |
| `project_workflow_start` | `preflight_token` | Fails when this input is set. Calls requiring no token are unaffected. |

Managed-resource write-only inputs work independently of this Action defect.
Ephemeral previews still open and return results, but their tokens cannot currently
be consumed by the affected Actions. `ignore_changes` applies to managed-resource
update planning; it does not bypass Action schema validation or replace write-only
protection. Required server review proof must not be omitted to work around this.

This is an explicitly accepted known defect for provider v1.0.5. No framework fork,
local framework patch, diagnostic suppression or downgrade to persistent inputs is
included. The real Terraform regression test asserts this specific rejection and
zero apply API calls. When upstream corrects validation, the test must fail until
the limitation and its expectation are updated.

Source: [HashiCorp action validation](https://github.com/hashicorp/terraform-plugin-framework/blob/c9992175c71f126d74863127de32ca1a81263c71/internal/fwserver/server_validateactionconfig.go#L100-L107).

## Runtime-secrets server prerequisite

The runtime-secrets resource and data source require a configured-state GET endpoint
at `/api/capabilities/runtime-secrets`. Server `v2.20.0-ex.2` lacks this endpoint.
The provider checks support before writing and reports an upgrade requirement.
The companion server implementation is identified in [EX features](ex-features.md#runtime-secrets-configuration);
this provider release does not publish a new Semaphore server version.

## Terraform template destroy control

The provider maps `terraform_settings.allow_destroy` to the backend's existing UI
flag. The backend does not independently enforce it as a server permission.
See the [coverage matrix](provider-coverage.md#deliberate-exclusions-and-limits) for
additional API ownership and lifecycle limits.
