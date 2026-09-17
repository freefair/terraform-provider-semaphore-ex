# Complete Semaphore EX provider coverage

## Authorized outcome

Expose the missing Semaphore EX configuration and complete existing resources identified
in the coverage assessment. Preserve existing Terraform configurations and imported EX
settings. Use the local EX checkout at `448976be2a2177907067cb88a7463e9fd0ba5510`
and the approved disposable acceptance fixture. Publishing and a new release version
are separate decisions after implementation and verification.

## Implementation sequence

1. Complete existing resource schemas and wire contracts in `api-docs.yml` and
   `internal/provider/project_*`: template fields and surveys, inventory types and
   placement, one-off schedules, custom membership roles, secret references/sync,
   integration matchers and value extraction. Regenerate `semaphoreui/` from the spec.
2. Establish the shared EX API transport and native Terraform lifecycle conventions
   in `internal/provider/`, preserving configured authentication, TLS, context,
   optimistic revisions, explicit deletion, imports and not-found behavior.
3. Add resources and read-only data sources for workflows/triggers, immutable
   versions and cross-project template grants; custom roles and assignments;
   secret storages and global credentials/grants; notifications and audit webhook;
   deployment windows, policy guardrails and artifact retention; identity policy;
   executor policy; apps, options and user tokens; Terraform backend aliases.
4. Model operational transitions separately from persistent configuration. Use
   native Terraform Actions where appropriate and supported; a refresh or plan
   must never start work, approve execution or rotate a credential.
5. Add examples in `examples/`, generate reference pages with `task generate`,
   update `README.md`, `CLAUDE.md` and durable ADRs for design choices.
6. Run meaningful unit/transport tests and isolated acceptance tests covering
   create/read/update/import/delete, external changes, stale revision rejection,
   redacted secrets and preservation of omitted settings. Finish with build, lint,
   reproducible documentation generation and independent review.

## Design decisions

- Retain the public provider address and `semaphore_ex_` type names.
- Prefer explicit resource-specific schemas and lifecycle behavior over an arbitrary
  HTTP request resource. Complex configuration uses native HCL blocks and typed
  objects; guardrail policies retain an authored YAML escape hatch.
- Reuse the existing authenticated transport for EX endpoints. Do not hand-edit
  generated client files or introduce a second independently configured connection.
- Security-related resource implementation and review use a dedicated security
  agent as required by the Semaphore EX project agreement.
- Preserve published tags and release artifacts. No production/shared API mutations.

## Acceptance matrix

| Area | Deliverable |
| --- | --- |
| Templates | Parallel/branch settings, JWT parameters, complete surveys, inventory attachments, immutable versions |
| Membership and access | Custom roles, project/global roles and assignments, template ACL |
| Secrets | Storage resources, external references, managed sync, global credentials and grants |
| Inventory | Terragrunt workspaces and runner placement |
| Schedules | Cron and one-off schedules, task overrides and delete-after-run |
| Integrations | Matchers and extracted values |
| Workflows | Definitions, triggers, versions and cross-project grants |
| Governance | Deployment windows, policy guardrails and artifact retention |
| Notifications | Destinations, rules and audit webhook |
| Identity | LDAP configuration, LDAP/OIDC mappings and TOTP policy |
| Executors | Docker/Kubernetes execution policies |
| Administration | Custom apps, system options, user API tokens and backend aliases |
| Operations | Explicit actions for supported execution, approval, rotation and lifecycle transitions |

## Completion sequence

1. Apply the six reviewed server contract fixes and the provider task-parameter
   normalization patch; run their focused regression suites.
2. Finish workflow resource identity using unique persisted node display names,
   hydrate complete imported graphs and preserve API policies and references.
3. Replace the selected backend-alias controller stub with durable scoped CRUD,
   add the matching provider resource/data source and API contract documentation.
4. Build the actual server checkout and run all provider acceptance tests against
   its disposable fixture; regenerate reference pages, lint and obtain peer review.
5. Commit verified changes locally and remove task-owned temporary test artifacts.
   Publication remains a separate approval.

Unique names avoid a storage migration and manual import mappings. A separate
client key would decouple renames but expand the server API/storage contract;
manual mappings would make imports error-prone. Names are therefore the selected
resource identity, with explicit duplicate/empty-name validation.
