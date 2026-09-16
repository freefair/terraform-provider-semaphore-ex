# ADR 0001: Semaphore EX provider identity and API contracts

Status: Accepted

## Context

The fork targets Semaphore EX, whose update and registration contracts differ from upstream Semaphore UI.
The existing provider represents only one variable group per template and omits several EX fields, so updates can erase configuration.

## Decision

Use repository and release name `terraform-provider-semaphore-ex`, source address `freefair/semaphore-ex`, and resource/data-source prefix `semaphore_ex_` as requested.
Use the local Terraform provider name `semaphore` in examples so implicit resource lookup resolves the first underscore-delimited prefix.
Rename the Go module to `github.com/freefair/terraform-provider-semaphore-ex`.
The local checkout directory is independent of the remote repository name.

Keep the generated API client and amend its source specification only for the contracts used by this provider.
Regenerate the client rather than editing generated files.
Expose server-owned membership revisions as computed state and echo the prior revision during updates without automatically retrying concurrency conflicts.
Model EX settings so imported values survive unrelated updates.
Model runner registration as one-time registration tokens and preserve the configured registration policy.

Support `environment_ids` in addition to a deprecated singular `environment_id`, with mutually exclusive configuration.
Preserve all group IDs through create, read, update, data sources, and import, including an explicitly empty collection.
Use a set: SQL deduplicates memberships and returns them in ascending environment ID order.
The server determines merge order; configuration order is not a supported precedence control.
EX strings, flags, tags, and policies are optional/computed so imported values survive when configuration omits them.
Runner resources expose durable configuration; one-time tokens belong to the separate registration-token resource, and unsupported token/private_key outputs are removed.

## Alternatives and consequences

Keeping the old resource prefix would reduce migration work, but the requested new identity includes resource names.
Users must migrate configuration and state deliberately; a provider-source replacement alone does not rename resource types.
Replacing the complete specification with the EX specification would generate unrelated clients; targeted changes keep this correction reviewable.
Only updating the first variable group would preserve the old limitation and could erase additional groups.

## Implementation and verification plan

1. Amend `api-docs.yml` for membership revision, schedule success response/timezone, template EX fields, and runner policy/registration response; regenerate `semaphoreui/`.
2. Update `internal/provider/project_user_*` and runner resources/data sources with regression coverage.
3. Update template and schedule schemas/converters, including multiple variable groups and preservation tests.
4. Rename module imports, provider metadata, examples, documentation generator, README and release identity; document migration.
5. Run unit tests, build, lint and documentation generation, then acceptance tests against the explicitly authorized isolated local EX server and temporary database.
6. Review the completed diff, commit verified changes, rename the GitHub repository through `gh`, and verify the new remote identity.

No production or shared instance participates in verification.
