# Semaphore EX feature usage

The provider uses the `freefair/semaphore-ex` source address and the `semaphore_ex_` type prefix.
Resources manage persistent configuration; data sources read existing objects; Actions perform explicitly requested operations.
Use the generated reference pages in `resources/`, `data-sources/` and `actions/` for exact attributes.

## Contents

- [Configuration coverage](#configuration-coverage)
- [Versions and operations](#versions-and-operations)
- [Credentials and imports](#credentials-and-imports)
- [Concurrency and destroy](#concurrency-and-destroy)
- [Server compatibility](#server-compatibility)

## Configuration coverage

All type names below have the `semaphore_ex_` prefix.

| Area | Resources and data sources |
| --- | --- |
| Workflows | `workflow_definition`, `workflow_trigger`, `cross_project_grant` |
| Project access | `project_role`, `template_acl`, custom role references on `project_user` |
| Global access | `global_role`, `global_role_assignment` |
| Credentials | `project_secret_storage`, `global_credential`, `global_credential_grant`, `project_generated_ssh_key`, external references on project keys and environments |
| Notifications | Global/project notification destinations and rules, `audit_webhook` |
| Governance | `project_deployment_window`; guardrail and artifact-retention policy data sources with separate mutation Actions |
| Identity | `ldap_configuration`, `ldap_group_mapping`, `oidc_group_mapping`, `totp_policy`, read-only `external_user` |
| Executors | `docker_execution_policy`, `kubernetes_execution_policy` |
| Integration | `project_integration_matcher`, `project_integration_extract_value` |
| Template attachments | `project_template_inventory` |
| Terraform state | `project_terraform_backend_alias` |
| Administration | `app`, `user_api_token`, read-only `option` with separate set Action |

Templates expose multiple variable groups, survey fields, JWT parameters, parallel execution and branch-override settings.
Inventories support Terragrunt and runner placement.
Schedules support recurring cron and one-off execution, task overrides and deletion after execution.
One-off schedules require lifecycle management in configuration: the server disables them after their timestamp or deletes them when `delete_after_run` is set.
After completion, remove the schedule from configuration or set a new future `run_at` to schedule another execution.
The current server also rejects updates to a past `run_at`, including unrelated edits.
The provider does not move execution times forward automatically.

Workflow resources use unique persisted node display names as their HCL `key` values.
An optional `display_name` must equal `key`. This lets import recover edge and artifact references.
Empty or duplicate persisted names must be corrected before resource import; data sources can still inspect those graphs with server-ID keys.
Node IDs survive reorder and ordinary updates. Optional/computed access policy retains the actual imported policy; explicit empty role lists clear it.

## Versions and operations

`project_template_version` reads immutable publication metadata.
The `project_template_publish` Action publishes the current template; the server reuses an identical snapshot.

`project_workflow_version` reads an exact version or the latest snapshot when `version_number` is omitted.
Its `definition` includes the graph, parameters, access policies, task parameters and artifact declarations.
Read-only graph keys use `node-<server ID>` so edge and artifact references are unambiguous.
Artifact schemas currently support four nested schema levels; deeper responses produce a diagnostic instead of losing fields silently.

```hcl
data "semaphore_ex_project_workflow_version" "reviewed" {
  project_id     = 1
  workflow_id    = 4
  version_number = 2
}

action "semaphore_ex_project_workflow_restore" "reviewed" {
  config {
    project_id     = 1
    workflow_id    = 4
    version_number = data.semaphore_ex_project_workflow_version.reviewed.version_number
    message        = "Restore reviewed definition"
  }
}
```

Restore creates a new revision with provenance and does not start execution.
It replaces the live definition, so reconcile a managed workflow resource's configuration before the next apply.
The server restore endpoint does not accept an expected revision.

Explicit Actions also support task/workflow start and stop, workflow approval, option writes, guardrail draft save/publication and artifact-retention publication.
Secret-storage sync requires a caller-supplied request ID for idempotency; connection tests and environment sync are also explicit Actions.
Generated SSH-key rotation requires an explicit confirmation input.
These operations require a Terraform CLI with Action support.
Planning and refreshing resources do not invoke them.
Start Actions return after the API accepts the request; they do not wait for execution to finish.
Where preflight proof is required, supply the separately reviewed fingerprint and token.
The provider does not obtain approval proof automatically.

## LDAP lifecycle

Configure the directory in `disabled` or `shadow` state, then explicitly invoke
`ldap_test` with directory test credentials and a local recovery administrator's
credentials. After the server records readiness, change the resource state to
`active` or `selected_users`. State-only changes retain the readiness result.
Set `bind_password_wo_version` alongside `bind_password` and increment it only when changing the bind credential.
To edit an active directory configuration, first apply only a change to `disabled`,
then edit and test the configuration before reactivating it.
Credentials supplied to the test Action are write-only; use sensitive ephemeral variables.

## Credentials and imports

Supply the provider API token through `SEMAPHOREUI_API_TOKEN` or the provider's sensitive configuration.
Data sources never recover one-time credentials.
For `user_api_token`, the resource's sensitive `credential` is populated only during creation; imports use the stable non-secret API `token_ref` and leave it null.
Changing token `keepers`, name or expiry replaces the token because the API has no update operation.
Protect state storage: `Sensitive` hides presentation but does not remove a value from state.

`project_generated_ssh_key` keeps the private key entirely on the server and exposes only public metadata.
Its write-only `login` uses `login_wo_version` to request replacement when the login changes.
Changing the algorithm also replaces the key; use the separate rotation Action only when an in-place rotation is intended.

Workflow-trigger and audit-webhook resources retain one-time signing material in sensitive outputs.
Their signing version inputs request bootstrap or staging explicitly; Promote and Revoke are separate revision-guarded Actions.
For the audit webhook, Promote swaps current and next slots so the previous key remains available during transition.
Revoke-next before staging another key.

Write-only inputs use companion version attributes where the server redacts stored values.
Keep the version unchanged to retain the server credential; increment it with new input to request a change.
See each resource's import example for its exact identity format.
Compound IDs include the owning project or other parent scope.

## Concurrency and destroy

Singleton resources such as deployment-window, executor and identity policies manage existing server configuration when first applied. Import first when you need to review existing values before changing them.

Server-owned revisions are computed outputs.
Guarded mutations send the revision from Terraform state; conflicts remain errors for the operator to resolve.
The provider does not fetch a fresh revision and automatically overwrite another writer's change.

Destroy semantics follow the API lifecycle:

- Ordinary records are deleted.
- Template-inventory attachments are detached only if still owned by that template.
- Audit webhook destroy pauses deliveries and retains its configuration and credentials.
- Executor-policy destroy restores the documented deny-all defaults.
- Immutable publication records are read through data sources; publication Actions do not imply a deletion lifecycle.
- Option writes have no reset-on-destroy behavior.

Removing an existing template or integration `task_params` block clears the managed defaults.
Other optional/computed EX settings retain imported values when omitted; consult the specific schema before relying on omission as a reset.

## Server compatibility

Use a Semaphore EX server with the contracts exercised by the provider's pinned acceptance fixture.
The provider does not emulate missing server capabilities.
For API-token creation, it checks that the authenticated user's token list exposes stable `token_ref` values before creating a credential.
A server lacking that contract fails before the POST, avoiding an unmanaged one-time token.

`project_terraform_backend_alias` binds a Terraform, OpenTofu or Terragrunt workspace inventory to a project login/password key.
Import uses `project/<project_id>/inventory/<inventory_id>/alias/<alias_id>`.
The server implements the Terraform HTTP backend protocol with encrypted state versions and durable locks.
Configure backend credentials through `TF_HTTP_USERNAME` and `TF_HTTP_PASSWORD`, and configure the same alias address for state, lock and unlock operations.
Existing plaintext server state must be migrated with `semaphore vault rekey` and checked with `semaphore vault check` before using the backend.
Deleting the Terraform alias resource removes its address; it does not erase retained state history.
Automatic backend override in Semaphore tasks requires an alias for the resolved workspace inventory.
When a workspace has multiple aliases, they must bind the same key; otherwise automatic selection fails explicitly.
External backends remain unchanged when override is disabled.

Vault/OpenBao `params` may omit the server's default `auth_method = "token"`.
Imports expose canonical server parameters, including that default; other parameter changes remain visible as drift.
