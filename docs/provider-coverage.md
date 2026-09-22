# Provider contract coverage

> **Known Action limitation:** WriteOnly Action inputs are currently rejected by
> the official framework, including LDAP apply tokens and execution preflight
> tokens. See [affected Actions and behavior](known-limitations.md#write-only-action-inputs).


This inventory maps the audited Semaphore EX configuration and operational contracts
to Terraform. All names below have the `semaphore_ex_` prefix. The baseline is
provider v1.0.4 and server `v2.20.0-ex.2` (`f6e216f7eec730a1fa4fe381e189c9a973ecfa0d`);
current server source was also checked. Runtime-secrets configured reads additionally
require server commit `5abb42991dc72f8d7673ac042d5f784907238719` or a descendant.
This source requirement does not identify a new released server version.

## Configuration field mapping

| Backend contract | Terraform mapping | Ownership and compatibility |
| --- | --- | --- |
| `Template.task_params` flat Ansible fields | `project_template.ansible_settings`: `allow_debug`, `allow_override_inventory`, `allow_override_limit`, `allow_override_tags`, `allow_override_skip_tags`, `allow_override_skip_galaxy_install`, `skip_galaxy_install`, `hide_dry_run`, `hide_diff`, `limit`, `tags`, `skip_tags`, `galaxy_role_args`, `galaxy_collection_args` | Resource and data source. Omitted settings preserve actual values; explicit empty lists/false clear. Unknown unmanaged API fields survive updates. |
| `Template.task_params` flat Terraform-family fields | `project_template.terraform_settings`: `allow_destroy`, `allow_auto_approve`, `auto_approve`, `override_backend`, `backend_filename` | Resource and data source. `allow_destroy` is a backend UI flag, not an independently enforced server permission. |
| Legacy nested template invocation envelope | Deprecated `project_template.task_params` | Preserved for configuration/state compatibility; never promoted into effective flat defaults during upgrade. |
| `Template.survey_vars[].values` ordered label/value pairs | `survey_vars[].choices` | Ordered list supports repeated labels. Legacy `enum_values` map stays available and is null for unrepresentable duplicate labels. |
| Environment `json` and `env` JSON documents | `project_environment.variables_json`, `environment_json` | Lossless JSON including nested values and exact numbers. Published string maps remain supported; choose one representation per input. Null and malformed API responses are handled explicitly. |
| Template description/branch/view/survey/vault membership | Existing `project_template` fields | Refresh reports external removal. Terragrunt templates do not require an Ansible playbook. |
| Workflow graph node IDs and display names | `workflow_definition.nodes[].server_id`, `key`, `display_name` | Keys remain stable by server ID. Import supports empty/repeated labels with deterministic keys. HCL-only arbitrary keys cannot be recovered after discarding state. |
| Guardrail authored draft and revision | Global/project `policy_guardrail` resources | Manage draft only; updates fence with prior state revision. Publish/rollback remain explicit Actions. Destroy forgets ownership and retains history. |
| Artifact retention scope policy | Global/project `workflow_artifact_retention` resources | Own `retention_seconds`, `max_artifact_bytes`, `max_run_bytes`; effective inherited limits remain data-source outputs. Revision conflicts are not retried. Destroy forgets ownership. |
| Runtime-secrets configured state/expiry | `runtime_secrets` resource and data source | Exact configured state, including expired settings. Import `runtime_secrets`; destroy forgets ownership. Older servers fail before a write. |
| Task request flat `params` | `project_task_start.params` | Ansible limit/tags/skip_tags/debug/debug_level/diff/dry_run/skip_galaxy_install; Terraform plan/destroy/auto_approve/upgrade/reconfigure. Wrong app families and ignored overrides are rejected. |
| Task build/deploy version and commit selection | `project_task_start.version`, `build_task_id`, `commit_hash` | Build version generation still belongs to the server. Branch/commit pinning obeys the template override setting. |
| Task survey secret JSON | `project_task_start.secret` | Native HCL object, write-only input; never emitted in Action progress. |
| Existing EX durable configuration | Resources/data sources documented in [EX features](ex-features.md#configuration-coverage) | Includes roles/assignments/ACLs, workflow triggers/grants, notifications, LDAP/OIDC/TOTP, secret stores/credentials, SSH policy, executor policies, deployment windows, backend aliases, apps and tokens. |

## Selection and read-only metadata

| Backend collection / metadata | Terraform exposure | Read behavior |
| --- | --- | --- |
| Projects | `project` by ID/name; `projects` collection | Exact unique name lookup; accessible records only. |
| Project environments, templates, inventories, repositories, keys, integrations, schedules, runners, views, secret stores | Singular types by ID or exact name (`title` for views); corresponding plural types | Explicit project scope, detail hydration after name resolution, missing/duplicate names are errors. Generated SSH-key lookup uses the key collection. |
| Workflows and triggers | `workflow_definition`, `workflow_trigger`; `project_workflows`, `workflow_triggers` | Project scope; triggers also require workflow scope. |
| Global/project roles, global credentials, global/project notification destinations | Singular exact lookup and plural collection | Credential selector is `display_name`. Credentials and destinations use bounded count/offset pagination; incomplete/repeated pages fail. |
| Global runners | `runner`, `runners` | ID/name lookup and filtered collection. |
| LDAP/OIDC mappings | `ldap_group_mappings`, `oidc_group_mappings` | Require `provider_id`; expose non-secret summaries. |
| Plural results | `ids`, `items`, `name_filter`, meaningful `type_filter` | Stable ID ordering; exact filters; missing metadata null. Full fields come from singular readers. |
| External users | `external_user` | Lookup only. Missing users fail; creation uses the user resource with `external = true`. |
| Redacted key payloads | `project_key` | Detail GET after identity resolution. Unavailable secret/login outputs remain null; no invented empty credentials. |
| Runner runtime and security metadata | Runner data sources | Status/version/platform/load, executor type, Docker/Kubernetes policy metadata, timestamps, registration/security/trust fields. |
| Schedule runtime metadata | `project_schedule` data source | `effective_timezone`, `next_run`. |
| Environment/secret-store synchronization | Corresponding data sources | `last_synced_at`, `last_sync_failed_at`, `sync_path_status` IDs/paths/fingerprints/remote versions. |
| Workflow/trigger runtime identity | Corresponding data sources | `current_version_id`, `owner_user_id`. |
| LDAP readiness | `ldap_configuration` data source | Readiness status/check results/time, eligible users, recovery administrator, created/updated metadata. |

Volatile observations are not configurable resource attributes. Ordinary data-source
reads never create users, issue credentials, run tests, generate preview records or
perform mutations.

## Operational API mapping

Global/project pairs below each have separate registered types. Every operation is
an explicit Action; rows marked **ephemeral** additionally offer a matching
ephemeral resource with a complete sensitive `result` that does not enter state.

| API method and route family | Terraform suffix | Result / side effect |
| --- | --- | --- |
| POST `/capabilities/ldap/group-mappings/preview` | `ldap_group_preview` | **Ephemeral**; persists server preview history, exposes freshness-bound `preview_token`. |
| POST `/capabilities/ldap/group-mappings/apply` | `ldap_group_apply` | Explicitly applies the supplied fresh token; no automatic retry. |
| POST `/capabilities/ldap/group-mappings/reconcile` | `ldap_group_reconcile` | Immediately changes role assignments. |
| POST `/capabilities/oidc/group-mappings/preview` | `oidc_group_preview` | **Ephemeral**; persists diagnostic history. There is no public token-apply endpoint. |
| POST global/project `/notification-governance/destinations/{id}/test` | `{scope}_notification_destination_test` | Queues an actual notification delivery. |
| POST global/project `/notification-governance/deliveries/{id}/retry` | `{scope}_notification_delivery_retry` | Requeues a delivery under server revision/claim checks. |
| POST global/project `/notification-governance/routing/preview` | `{scope}_notification_routing_preview` | **Ephemeral**; evaluates routing without sending. Scope cannot be overridden in event data. |
| POST `/audit-webhook/test?key=current|next` | `audit_webhook_test` | Actual outbound test and delivery record. |
| POST `/runners/docker-policy/test` | `docker_execution_policy_test` | **Ephemeral**; in-process evaluation, no container. |
| POST `/runners/kubernetes-policies/{cluster_alias}/test` | `kubernetes_execution_policy_test` | **Ephemeral**; evaluates a manifest summary, no Kubernetes Job. |
| POST global/project `/policy-guardrails/validate` | `{scope}_policy_guardrail_validate` | **Ephemeral**; validates authored YAML without storing it. |
| GET global/project `/policy-guardrails/diff` | `{scope}_policy_guardrail_diff` | **Ephemeral**; compares two stored revisions. |
| POST global/project `/policy-guardrails/test` | `{scope}_policy_guardrail_test` | **Ephemeral**; evaluates a value-free fixture against authored policy. |
| POST global/project `/policy-guardrails/impact` | `{scope}_policy_guardrail_impact` | **Ephemeral**; evaluates 1–100 scoped fixtures. |
| POST global/project `/policy-guardrails/rollback` | `{scope}_policy_guardrail_rollback` | Publishes a new revision from history; explicit reason and expected draft revision. |
| POST project `/deployment-windows/preview` | `project_deployment_window_preview` | **Ephemeral**; evaluates proposed policy for one template or workflow. |
| POST project `/tasks/{id}/confirm`, `/reject` | `project_task_confirm`, `project_task_reject` | Controls an active task runner; inactive task IDs fail. |
| POST project `/tasks/{id}/retry-recovery` | `project_task_retry_recovery` | Requests eligible persisted-task recovery. |
| POST workflow `/runs/{id}/retry-reconcile` | `project_workflow_retry_reconcile` | Resets reconciliation retry/quarantine state. |
| POST workflow `/triggers/{id}/test` | `workflow_trigger_test` | Starts an actual trigger invocation/run; not a dry run. |
| Existing start/stop/approval/publication/sync/key rotation/signing operations | Existing Actions in [operations guide](ex-features.md#versions-and-operations) | Keep explicit preflight, revision, idempotency and confirmation contracts. Secret-storage connection test already exists. |

See [operational previews](operational-previews.md) for HCL usage and effects.
`allowed = false` or `valid = false` is a successful evaluation result. Use an
explicit ephemeral postcondition to make that outcome fail a Terraform operation.
Action progress contains only a bounded summary, not the full result or token.

## Terraform lifecycle coverage

All managed types implement import. The existing lifecycle suite covers ordinary
create/read/update/delete, refresh, replacement, address moves, import blocks,
`removed` blocks and state removal. Singleton delete behavior follows its documented
reset/disable/forget contract. `removed { lifecycle { destroy = false } }` and
`terraform state rm` remove ownership without calling a deletion API.

Cross-provider `moved` blocks support the 15 published legacy resource schemas from
`semaphoreui/semaphore` v0.3.9, schema version 0. State moves make no API writes;
refresh hydrates EX fields. See [lifecycle](lifecycle.md) and [migration](migration.md).
New server settings use additive schema fields, so old states remain readable.
All generated client operations pass the Terraform request context directly to the
operation method; cancellation/deadline tests cover blocked requests.

## Deliberate exclusions and limits

- Browser authentication ceremonies, TOTP enrollment/recovery, logs/streams, process
  startup settings and lifecycle-test capabilities are not ordinary owned resources.
- One-time credentials cannot be recovered from data sources or import. Sensitive
  resource outputs that intentionally hold credentials still require protected state.
- General options remain `option` plus `option_set`: keys lack a uniform reset/delete
  contract. No managed resource silently guesses a default on destroy.
- Immutable publication/history records have reads and explicit operations, not a
  fabricated destructive CRUD lifecycle. Preflight approval is never obtained automatically.
- Cache clearing and cluster drain/maintenance controls were optional in the approved
  plan and are not added by this change.
- The template API has no revision fence. GET/merge/PUT preserves unmanaged fields
  but cannot eliminate another client's concurrent update race.
- No new backend enforcement of the `allow_destroy` UI flag is introduced. That is
  a separate backend finding.
- Operational HTTP contract tests exercise every new Action. Full LDAP directory,
  outbound notification and workload effects require their external systems; local
  provider tests do not claim to execute those services.

## Verification map

| Contract | Executable evidence |
| --- | --- |
| Template settings and actual task arguments | `template_settings_test.go`, `task_start_params_test.go`; server `services/tasks/provider_task_start_wire_contract_test.go` |
| Typed environment round trips and import | `coverage_environment_json_test.go`, environment acceptance tests |
| Lookup cardinality/pagination/detail reads | `named_lookup_test.go`, `collection_test.go`, data-source tests |
| Runtime metadata and redaction | `read_metadata_test.go`, key data-source regression tests |
| Workflow labels/IDs/import | `workflow_identity_test.go` and workflow acceptance tests |
| Revision-fenced governance and ownership | `governance_resource_test.go`, existing governance Action tests |
| Configured runtime-secrets / old-server rejection | `runtime_secrets_test.go`; server capability GET tests |
| Operational routing, scope, summaries and ephemeral results | `operational_test.go` |
| Ordered survey choices and legacy maps | `survey_choices_test.go` and template acceptance tests |
| Context propagation and endpoint validation | Client-context and provider-configuration regression tests |
| Existing Terraform lifecycle / legacy moves | [Lifecycle verification](lifecycle.md) |

Generated reference pages contain the complete registered attribute schemas; this
inventory records field ownership and behavior that type counts cannot express.
