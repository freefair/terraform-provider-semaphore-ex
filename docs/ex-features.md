# Semaphore EX feature usage

The provider uses the `freefair/semaphore-ex` source address and the `semaphore_ex_` type prefix.
Resources manage persistent configuration; data sources read existing objects; Actions perform explicitly requested operations.
Use the generated reference pages in `resources/`, `data-sources/` and `actions/` for exact attributes.

## Contents

- [Configuration coverage](#configuration-coverage)
- [SSH keys for private dependencies](#ssh-keys-for-private-dependencies)
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
| Task SSH keys | `project_ssh_key_policy`, `ssh_keys` on templates and the task-start Action |
| Notifications | Global/project notification destinations and rules, `audit_webhook` |
| Governance | `project_deployment_window`; guardrail and artifact-retention policy data sources with separate mutation Actions |
| Identity | `ldap_configuration`, `ldap_group_mapping`, `oidc_group_mapping`, `totp_policy`, read-only `external_user` |
| Executors | `docker_execution_policy`, `kubernetes_execution_policy` |
| Integration | `integration_alias`, `project_integration_matcher`, `project_integration_extract_value` |
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

## SSH keys for private dependencies

Semaphore EX makes the selected SSH keys available during task execution, including Terraform module downloads and Ansible dependencies. The repository credential remains usable beyond the initial clone.
Use Semaphore EX `v2.20.0-ex.2` or a compatible newer server; the acceptance fixture pins that release.

Create project-owned SSH keys first, then manage project defaults and always-added keys through `semaphore_ex_project_ssh_key_policy`. Keeping this singleton separate from `semaphore_ex_project` avoids a project/key dependency cycle and allows one apply to create the project, keys and policy.
Only one policy resource should manage a project. Import uses the numeric project ID.
When omitting previously configured bindings while keeping their keys managed by Terraform, retain the dependency with explicit `depends_on` entries for those key resources. Terraform cannot derive dependency edges from server-retained numeric IDs; the keys must be destroyed after their policies and templates.
Omitted policy collections preserve existing values; explicit `[]` clears a collection. Destroy clears both SSH collections without deleting the project or keys.

Each binding contains `access_key_id` and an optional `hosts` list:

```hcl
resource "semaphore_ex_project_ssh_key_policy" "dependencies" {
  project_id = semaphore_ex_project.example.id
  default_ssh_keys = [{
    access_key_id = semaphore_ex_project_generated_ssh_key.dependencies.id
    hosts         = ["github.com", "gitlab.com"]
  }]
  always_ssh_keys = []
}
```

Authorize the generated public key on the Git hosts before executing a task. The private key remains on Semaphore EX.
Bindings accept project-owned SSH credentials. Foreign-project, global and non-SSH credentials are rejected by the server.

| Template configuration | Behavior |
| --- | --- |
| Omit `ssh_keys` | Preserve the existing setting on refresh/import; new templates inherit by default |
| `ssh_keys = { inherit = true }` | Explicitly restore inheritance from project defaults |
| `ssh_keys = { inherit = false, bindings = [] }` | Select no non-always keys |
| `ssh_keys = { inherit = false, bindings = [{ access_key_id = 7, hosts = ["github.com"] }] }` | Select these keys instead of project defaults |

For `semaphore_ex_project_task_start`, `ssh_keys` is a list directly: omit it to inherit the template selection, use `[]` for no non-always keys, or supply bindings to override it for that run. The caller needs the server's resource-management permission to override task keys.
Project `always_ssh_keys` are added to every effective selection, including explicit empty template or task selections.

Host mappings use exact lowercase DNS names or IP addresses. Wildcards, URLs, ports and usernames are invalid. Below five distinct effective SSH public-key identities, routing is optional; at five or more it is required. The server counts the effective selection, including repository and inventory credentials, and validates routing at runtime. The provider cannot infer those public identities from key IDs during planning.
Host mappings select keys for supported SSH clients; they do not prevent arbitrary task-controlled code from using credentials available to that task.

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

## Exact lookups and collections

Named data sources accept exactly one of `id` and their existing name attribute
(`name`, `title`, or `display_name`). Matching is exact and scoped by the required
project or workflow identity. Missing names and duplicate matches fail explicitly.
The provider resolves the identity from the list, then reads the detail endpoint;
list responses do not substitute for complete resource metadata. Results are limited
to records the authenticated caller can access.

Plural data sources expose ordered `ids` and non-secret `items` summaries, with
`name_filter` and, where meaningful, `type_filter`. For example:

```hcl
data "semaphore_ex_project_environments" "deployment" {
  project_id  = semaphore_ex_project.example.id
  name_filter = "deployment"
}
```

Global credentials and notification destinations are paginated. The provider reads
all pages before filtering or deciding name uniqueness. A failed page, repeated
identity, or pagination bound produces an error instead of a partial result.
Collections intentionally expose summary fields; use the singular data source for
complete details. Missing summary metadata stays null. LDAP/OIDC mapping collections
require `provider_id`.

When resources and their collections are created in the same apply, express the
appropriate dependency with `depends_on` so Terraform reads the list after creation.
A reference to only the parent project's ID does not depend on its child resources.

## Operational metadata

Data sources expose current runner status, version, platform, load, executor policy
identity, and registration/security diagnostics. Schedule data sources expose
`effective_timezone` and `next_run`. Environment and secret-storage data sources
expose synchronization timestamps and `sync_path_status` fingerprints. Workflow
and trigger data sources expose `current_version_id` and `owner_user_id`; LDAP
configuration exposes readiness, eligible users, and recovery-administrator identity.

These are read-only snapshots. They are deliberately absent from managed resource
schemas so changing runtime observations do not create configuration changes. An
unavailable or omitted API field remains null instead of receiving an invented value.

## Runtime-secrets configuration

The `semaphore_ex_runtime_secrets` resource and data source expose the configured
`state` (`active`, `disabled`, or `read_only`) and optional `expires_at`. An expired
configuration keeps its original state and timestamp on refresh/import; the resolved
capability state is a different API concept.

This feature requires `GET /api/capabilities/runtime-secrets`, added to the server in
commit `5abb42991dc72f8d7673ac042d5f784907238719`. The previously supported
`v2.20.0-ex.2` server does not provide that configured-state read. The resource checks
read support before writing and produces an upgrade diagnostic on older servers.
No newer server release version is implied by this source-level requirement.

The singleton import ID is `runtime_secrets`. Destroy removes Terraform ownership
and leaves the global server setting intact. To change the setting, configure the
intended state explicitly and apply before removing ownership. Omitting `expires_at`
clears the configured expiry. Administrator authorization is enforced by the server.
