# Managed task groups

Implement the approved Semaphore EX managed-group API through the existing native EX transport. Add a revision-aware project task-group resource and data source, then extend templates with an optional/computed set of group IDs. Keep omission and import stable; an explicit empty set clears bindings. Use an owner-project compound import ID and propagate revision conflicts without retries.

A custom lifecycle preserves compare-and-swap revisions; the generic ordinary-record handler does not support that contract. Avoid modifying the generated upstream client. Verify request payloads and protocol schemas, then run lifecycle acceptance tests against an isolated current Semaphore EX binary, including update, import, shared-project visibility and binding removal. Generate documentation and run project gates before committing.

The provider release version is not selected by this implementation.
