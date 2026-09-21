# Terraform resource lifecycle

## Contents

- [Operations](#operations)
- [Resource matrix](#resource-matrix)
- [Import and secrets](#import-and-secrets)
- [Data sources](#data-sources)
- [Verification](#verification)

## Operations

All 45 managed resource types support import, refresh, planning, apply and destroy through the Terraform plugin framework.
Mutable fields update in place; immutable identities and server-immutable settings request replacement during planning.
An Update diagnostic on an immutable resource is defensive: Terraform must plan replacement instead of calling it.

| Operation | Contract |
| --- | --- |
| `plan`, `apply` | Read current objects, validate inputs, then create/update/delete the reviewed changes. |
| `import` block, `terraform import` | Adopt an existing identity and read it without creating the object. |
| `moved` block, `terraform state mv` | Rename the address or move it into a module without changing remote identity. |
| Legacy-provider `moved` | Convert the same object from a supported legacy type; see [migration](migration.md). |
| `removed { lifecycle { destroy = false } }`, `state rm` | Release ownership without API deletion or credential rotation. |
| `-refresh-only` | Reconcile state with the API; confirmed disappearance permits recreation on the next normal apply. |
| `-replace`, lifecycle replacement | Terraform orchestrates Delete/Create according to the lifecycle configuration. |
| `-target` | Terraform limits the graph; resource semantics remain unchanged. Follow with a full plan. |
| `destroy`, `removed` with destruction | Invoke the resource's Delete contract shown below. |
| `prevent_destroy`, `ignore_changes`, conditions | Terraform enforces these during graph evaluation/planning. |
| Existing schema state | The unchanged schema version remains readable; future incompatible versions require an explicit upgrader. |

A state move does not transfer an object to another Semaphore project or server.
Changing a parent identity requests replacement when the API does not support reparenting.
`create_before_destroy` still requires both old and new objects to coexist: unique identifiers and singleton policies can prevent this.
Use a new identity where needed and review the resulting plan.
Data sources and Actions are not managed resources: reads do not support import/destroy, and Actions run through Invoke.

## Resource matrix

The prefix for every type is `semaphore_ex_`.
Every row supports both import forms, ordinary address moves and forgetting without remote destruction.
Import examples use illustrative IDs; replace them with the existing object's identity.
“Legacy move” means conversion from the matching `semaphoreui_` type in `semaphoreui/semaphore` v0.3.9, schema version 0.
“Update” permits mutable attributes; immutable attributes within that resource can still force replacement.

| Resource | Changes | Destroy effect | Import example | Data source | Legacy move |
| --- | --- | --- | --- | --- | --- |
| [app](resources/ex_app.md) | Update / replace | Delete object | `custom_tool` | Yes | No legacy type |
| [audit_webhook](resources/ex_audit_webhook.md) | Update / replace | Disable webhook | `audit_webhook` | Yes | No legacy type |
| [cross_project_grant](resources/ex_cross_project_grant.md) | Update / replace | Revoke membership/grant | `project/1/grant/42` | Yes | No legacy type |
| [docker_execution_policy](resources/ex_docker_execution_policy.md) | Update / replace | Reset safe defaults | `docker` | Yes | No legacy type |
| [global_credential](resources/ex_global_credential.md) | Update / replace | Delete object | `42` | Yes | No legacy type |
| [global_credential_grant](resources/ex_global_credential_grant.md) | Update / replace | Revoke membership/grant | `credential/42/grant/7` | Yes | No legacy type |
| [global_notification_destination](resources/ex_global_notification_destination.md) | Update / replace | Delete object | `destination/1` | Yes | No legacy type |
| [global_notification_rule](resources/ex_global_notification_rule.md) | Update / replace | Delete object | `rule/1` | Yes | No legacy type |
| [global_role](resources/ex_global_role.md) | Update / replace | Delete object | `role-id` | Yes | No legacy type |
| [global_role_assignment](resources/ex_global_role_assignment.md) | Replace | Revoke membership/grant | `user/1/assignment/2` | Yes | No legacy type |
| [integration_alias](resources/ex_integration_alias.md) | Replace | Delete object | `project/1/integration/2/alias/3`, `project/1/alias/3` | Yes | Yes |
| [kubernetes_execution_policy](resources/ex_kubernetes_execution_policy.md) | Update / replace | Reset safe defaults | `cluster-alias` | Yes | No legacy type |
| [ldap_configuration](resources/ex_ldap_configuration.md) | Update / replace | Disable provider | `provider-id` | Yes | No legacy type |
| [ldap_group_mapping](resources/ex_ldap_group_mapping.md) | Update / replace | Delete object | `provider-id/mapping/mapping-id` | Yes | No legacy type |
| [oidc_group_mapping](resources/ex_oidc_group_mapping.md) | Update / replace | Delete object | `provider-id/mapping/mapping-id` | Yes | No legacy type |
| [project](resources/ex_project.md) | Update / replace | Delete object | `project/1` | Yes | Yes |
| [project_deployment_window](resources/ex_project_deployment_window.md) | Update / replace | Reset policy | `1` | Yes | No legacy type |
| [project_environment](resources/ex_project_environment.md) | Update / replace | Delete object | `project/1/environment/2` | Yes | Yes |
| [project_generated_ssh_key](resources/ex_project_generated_ssh_key.md) | Update / replace | Delete object | `project/1/generated-ssh-key/2` | Yes | No legacy type |
| [project_integration](resources/ex_project_integration.md) | Update / replace | Delete object | `project/1/integration/2` | Yes | Yes |
| [project_integration_extract_value](resources/ex_project_integration_extract_value.md) | Update / replace | Delete object | `project/1/integration/2/value/3` | Yes | No legacy type |
| [project_integration_matcher](resources/ex_project_integration_matcher.md) | Update / replace | Delete object | `project/1/integration/2/matcher/3` | Yes | No legacy type |
| [project_inventory](resources/ex_project_inventory.md) | Update / replace | Delete object | `project/1/inventory/1` | Yes | Yes |
| [project_key](resources/ex_project_key.md) | Update / replace | Delete object | `project/1/key/2` | Yes | Yes |
| [project_notification_destination](resources/ex_project_notification_destination.md) | Update / replace | Delete object | `project/1/destination/2` | Yes | No legacy type |
| [project_notification_rule](resources/ex_project_notification_rule.md) | Update / replace | Delete object | `project/1/rule/2` | Yes | No legacy type |
| [project_repository](resources/ex_project_repository.md) | Update / replace | Delete object | `project/1/repository/2` | Yes | Yes |
| [project_role](resources/ex_project_role.md) | Update / replace | Delete object | `project/1/role/role-id` | Yes | No legacy type |
| [project_runner](resources/ex_project_runner.md) | Update / replace | Delete object | `project/1/runner/2` | Yes | Yes |
| [project_schedule](resources/ex_project_schedule.md) | Update / replace | Delete object | `project/1/schedule/2` | Yes | Yes |
| [project_secret_storage](resources/ex_project_secret_storage.md) | Update / replace | Delete object | `project/1/secret_storage/2` | Yes | No legacy type |
| [project_ssh_key_policy](resources/ex_project_ssh_key_policy.md) | Update / replace | Clear SSH selections | `1` | Yes | No legacy type |
| [project_template](resources/ex_project_template.md) | Update / replace | Delete object | `project/1/template/2` | Yes | Yes |
| [project_template_inventory](resources/ex_project_template_inventory.md) | Replace | Detach inventory | `project/1/template/2/inventory/3` | Yes | No legacy type |
| [project_terraform_backend_alias](resources/ex_project_terraform_backend_alias.md) | Update / replace | Delete object | `project/1/inventory/2/alias/opaque-alias-id` | Yes | No legacy type |
| [project_user](resources/ex_project_user.md) | Update / replace | Revoke membership/grant | `project/1/user/3` | Yes | Yes |
| [project_view](resources/ex_project_view.md) | Update / replace | Delete object | `project/1/view/2` | Yes | Yes |
| [runner](resources/ex_runner.md) | Update / replace | Delete object | `runner/1` | Yes | Yes |
| [runner_registration_token](resources/ex_runner_registration_token.md) | Replace | Forget association; token expires independently | `runner/1`, `project/1/runner/2` | One-time secret; unavailable | Yes |
| [template_acl](resources/ex_template_acl.md) | Update / replace | Revoke membership/grant | `project/1/template/2/acl/3` | Yes | No legacy type |
| [totp_policy](resources/ex_totp_policy.md) | Update / replace | Disable policy | `totp` | Yes | No legacy type |
| [user](resources/ex_user.md) | Update / replace | Delete object | `user/1` | Yes | Yes |
| [user_api_token](resources/ex_user_api_token.md) | Replace | Revoke token | `token-reference` | Yes | No legacy type |
| [workflow_definition](resources/ex_workflow_definition.md) | Update / replace | Delete object | `project/1/workflow/4` | Yes | No legacy type |
| [workflow_trigger](resources/ex_workflow_trigger.md) | Update / replace | Delete object | `project/1/workflow/2/trigger/3` | Yes | No legacy type |

## Import and secrets

Numeric compound IDs must match the documented labels and order exactly.
Extra labels, duplicate labels, signs, zero, overflow and trailing path components are rejected by the shared legacy parser.
For `project`, `user` and `runner`, a numeric ID alone is also accepted.
Opaque role, application, provider, token-reference and backend-alias IDs retain their resource-specific format.
The token-reference examples mean the server's stable `token_ref`, never a secret credential.

Import cannot reconstruct a secret that the API redacts or returns only once.
Retain your configured secret inputs or use write-only inputs with their version attributes.
Registration-token import adopts only the runner association and returns a warning with a null `registration_token`.
It never generates or invalidates a token.
The first configured `keepers` map after import is adopted as a local baseline without issuing a token.
Changing or removing an established baseline requests replacement and rotation.
Existing registration-token state remains intact during moves; explicit replacement requests a new token and remains subject to the runner's registration status.

Native EX revisions are read into state and echoed on updates/deletes where required.
A 409 conflict is an error requiring refresh and review; the provider does not silently bypass it.
Project-scoped 404s are checked against the parent project before they can remove state.
If the project is inaccessible, only a fresh administrator identity can confirm its absence; an ambiguous response produces an error and preserves state.
A non-admin must restore access or independently confirm deletion before explicitly removing an inaccessible project from state.
A missing child in a readable project, or confirmed absence for an administrator, still permits recreation on a subsequent apply.
Authentication failures, authorization failures and transient errors preserve the previous state.
Missing singleton capability endpoints remain errors where they do not establish that the managed configuration was deleted.

## Data sources

Every readable managed resource has a matching data source.
`data "semaphore_ex_integration_alias"` reads the alias with `id` and `project_id`; set `integration_id` for an integration-scoped alias.
An absent alias produces a diagnostic rather than an empty successful result.
The registration-token resource is the only exception: there is no token-read endpoint, and the issuance POST mutates credentials.
Use the matching runner data source for durable runner configuration.

## Verification

`task test` checks registry coverage, schema contracts, strict import parsing, missing-object handling, read-only lookups and all published legacy state shapes.
`SEMAPHORE_EX_TEST_BINARY=/absolute/path/to/semaphore task testacc` runs the complete suite on the disposable loopback server with generated credentials and its own SQLite database.
`TestAcc_ProviderLifecycleCLI` executes both move forms, both import forms, both forgetting forms, refresh-only, targeting, forced replacement, recovery after external deletion and destroy.
`TestAcc_LegacyProviderMovedGraph` creates all 15 resource types with the published v0.3.9 provider, requires no-op destination plans, and verifies stable IDs and retained secrets.
The registry parity test fails when a readable resource is added without its data source.
See [ADR 0008](adr/0008-confirm-absence-and-adopt-imported-keepers.md) for the absence and keeper-adoption contracts.
See [ADR 0007](adr/0007-complete-resource-lifecycle.md) for design choices and boundaries.
