# ADR 0007: Resource lifecycle and legacy state migration

Status: accepted.

## Context

Users need to adopt existing objects, refactor resource addresses, change provider
source, recover from external deletions, and release Terraform ownership without
changing the remote object. CRUD methods alone do not establish those contracts.
The upstream numeric import parser extracted matching substrings and could ignore
extra path components or overwrite duplicate scope labels. Several legacy Read
and Delete methods treated missing objects as errors, preventing recovery.

## Decision

Use the framework's lifecycle interfaces and Terraform's own state operations.
Every registered managed resource implements ImportState. Validate complete numeric
import IDs against explicit ordered label sequences, with positive IDs and numeric
shorthand for a single identifier. Keep each native EX resource's opaque identity
contract. Read forgets confirmed missing objects; Delete tolerates already-missing
objects. Authorization, revision conflicts and transient errors retain state.

Implement MoveState on each of the 15 resource types published by
`semaphoreui/semaphore` v0.3.9, commit
`084dd4e48a0ea26cfa989be1ab36cf6c8a553e03`. Bundle its actual GetProviderSchema
Terraform types as generated Go type constructors. Check namespace, provider type,
resource type and schema version before decoding. Convert known values without
API requests, retaining numeric precision and secrets still represented by EX.
Reject incompatible fields instead of silently dropping them. Refresh populates
EX-only attributes after the move. The obsolete computed runner `token` and
`private_key` outputs are explicitly omitted with a warning; EX has no such
outputs and the remote runner registration is unchanged.

Same-type moved blocks, state mv, state rm, removed blocks, targeting, replacement
ordering, refresh-only and destroy remain Terraform operations. Converting unrelated
resource types or changing remote project ownership is not an address move.
Future incompatible schema changes must introduce an explicit state upgrader.

Add the missing integration-alias data source through the existing scoped list API.
Require a data source for each readable managed resource through a registration
parity test. The registration-token exception is deliberate: the API only returns
that secret during a mutating POST, so a data source would rotate it during planning.
Its import adopts the association with a null secret and emits a warning; it never
issues or invalidates credentials.

## Alternatives

A generic provider wrapper could hide optional framework interfaces such as plan
modification and configuration validation, so migration methods attach directly to
the existing resource implementations. Unrestricted JSON copying would lose fields
silently and accept unrelated state, so conversion validates the published schema.
The existing removed-plus-import handover remains available, but it cannot recover
redacted secrets and requires more user configuration than a typed state move.

## Verification

Unit tests cover malformed identities, confirmed absence versus 401/403/500,
list-backed disappearance, registration coverage, all 15 legacy schemas with non-null
nested values and large integers, incompatible state, and read-only alias lookup.
Acceptance tests migrate a complete graph created by the published legacy provider,
assert no-op plans and stable identities, and verify password preservation.
A separate CLI test exercises both import forms, both move forms, both forgetting
forms, refresh-only, targeted update, forced replacement, external deletion recovery
and destroy against the isolated Semaphore EX server. Existing resource acceptance
tests cover their CRUD and server-specific contracts.
