# ADR 0008: Confirm project absence and adopt imported keeper baselines

Status: accepted.

## Context

Semaphore returns HTTP 404 both for missing projects and for non-admin users whose
project membership has been removed. Treating both responses as object deletion
can remove still-existing objects from Terraform state during refresh-only apply.
An imported registration-token association has no recovered credential or keepers
map. Unconditional keeper replacement rotates credentials on the first apply even
when the user only wants to adopt existing infrastructure.

## Decision

Wrap the configured HTTP transport for both generated and native EX API clients.
On project-scoped GET/DELETE 404s, a successful parent-project lookup with the correct
identity establishes access for a missing child. If the project itself is missing
or inaccessible, a fresh current-user lookup must confirm administrator access
before the provider accepts the 404 as absence. Otherwise return an error and keep
state. Probes retain the original origin, authentication, TLS settings and context,
use only GET, bypass the wrapper to avoid recursion, and do not follow redirects.
No authorization result is cached. Failed, malformed or mismatched probe responses
retain state. Ordinary successful requests do not add probes.

This deliberately requires independent confirmation before a non-admin can forget
an entire inaccessible project. Restore access and refresh, use an authorized
administrator, or explicitly remove state after independently confirming deletion.
Child disappearance within a readable project and confirmed deletion for admins
continue to support automatic drift recovery. The provider cannot establish an
atomic snapshot across separate API requests; permissions can still change while
an operation is in progress.

When both the imported registration credential and prior keepers map are null,
adopt the first configured keepers map as local metadata through Update without
issuing a token. An established keeper map always uses normal replacement semantics
when changed, including removal. Credentials created by the provider also retain
normal replacement behavior when keepers are first added. Identity changes and
explicit Terraform replacement continue to rotate through Create.

## Alternatives

Changing the server's project authorization status codes would require a separate
backend release and alter information-disclosure behavior. The provider-side guard
works with the existing supported server contract. Keeping all 404s indefinitely
would prevent legitimate child-deletion recovery; accepting all 404s loses state
when access is revoked. A warning about first-import rotation would document rather
than resolve the token issue. Import cannot read keepers from Semaphore because
keepers are Terraform-only metadata.

## Verification

Temporary tests reproduce both original failures before implementation. Committed
HTTP tests cover generated and native resources, denied access, confirmed absence,
wrong parent identity, malformed user metadata and failed probes. A real-server
acceptance test revokes a synthetic non-admin's project membership and verifies
that refresh reports an error, preserves state, and leaves the project present.
The token acceptance test imports with configured keepers, verifies the credential
remains null and the next plan is empty, then changes keepers and requires replacement
with a new credential. The CLI lifecycle test verifies actual deletion recovery.
