# Managed task groups

Status: accepted.

Semaphore EX owns task group membership, runner intersections, project grants and admission limits. The provider exposes `semaphore_ex_project_task_group` as a revision-aware resource and data source, and template `task_groups` as a set of IDs. The API requires Semaphore EX v2.20.0-ex.2.1.1 or later.

Use the existing native EX transport rather than modifying the upstream generated client. This preserves configured authentication, TLS and confirmed-absence handling. The ordinary record helper does not implement revision compare-and-swap, so the group has an explicit lifecycle. Updates and deletes use the revision from state without fetching a replacement or retrying conflicts. A successful update returns HTTP 204 and is followed by a read to obtain the new revision.

Optional/computed group policy settings preserve imported values; explicit empty sets clear restrictions or grants. Resources belong to the owner project. Data sources can read through a receiving project's explicit grant and report `owner_project_id` separately. They leave the hidden full grant list null, rather than claiming it is empty. Template bindings are optional/computed: omission preserves imported or existing values, while an explicit empty set removes them. Bindings are sent in the same template mutation as other fields.

Verification covers protocol schemas, revision conflicts, successful 204 updates, template omission and clearing, lifecycle import, multiple groups and reads through project grants. Group membership applies to templates; individual task invocations cannot replace it.
