# Task SSH key bindings

Status: Accepted

## Context

Semaphore EX supports project default and always-added SSH bindings, template selection and per-run overrides. Terraform must preserve the API distinction between inherited null and an explicit empty selection. Project-owned keys require an existing project, so embedding key references in project creation would create a dependency cycle.

## Decision

Expose project collections through a dedicated singleton resource and data source, `semaphore_ex_project_ssh_key_policy`, keyed by project ID. Reads and writes preserve unrelated project settings. Replacing its project ID clears the old project's policy; destroy clears only the two owned collections.

Templates use an optional/computed `ssh_keys` object with an explicit `inherit` discriminator and an optional `bindings` list. Inheritance requires omitted bindings; explicit selection requires a list, including the empty list. A typed null list represents inheritance in state. Planning retains the computed variable-group counterpart only when its configured source equals refreshed state, so removing an SSH override does not leave a spurious group update. Actual group changes remain unknown until applied. Omission preserves existing or imported settings. Unknown key IDs remain valid during planning.

The task-start Action uses an optional list directly because it has no persistent resource state. Omission inherits, and an empty list overrides non-always selection.

Native EX transport serializes SSH selection together with the template's existing request fields in one mutation. A separate SSH update after template creation could leave an unmanaged template when selection validation fails. Generated upstream models cannot preserve this nullable extension without changing their shared serialization contract.

The server remains authoritative for credential ownership, permissions and effective host routing. It can resolve public identities and inherited repository/inventory keys that Terraform cannot know at plan time. No private key is requested or transported by these bindings.

## Alternatives and consequences

Embedding collections in the project resource would be simpler to discover but prevents a normal single-apply project/key graph. Separate resources for every binding would obscure ordered selection and ownership of null versus empty values. The singleton keeps that ownership explicit and imports existing settings before changes.

The template discriminator is more verbose than a plain list, but makes restoring inheritance possible without conflating omission with reset. Project policy updates use the existing project read/update API, which does not offer a dedicated revision-guarded policy endpoint; concurrent external project writers must coordinate changes.

## Verification

Acceptance tests use Semaphore EX `v2.20.0-ex.2`. They cover same-apply keys, project settings preservation, import, selection changes, policy replacement and reset. Focused transport tests verify explicit null/empty payloads and atomic template writes.
