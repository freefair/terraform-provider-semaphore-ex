# ADR 0003: Native Semaphore EX resources and explicit operations

Status: Accepted

## Context

Semaphore EX exposes configuration outside the upstream OpenAPI snapshot. A
provider update must preserve imported fields, retain API concurrency checks,
and avoid invoking jobs or rotating credentials during refresh. Some endpoints
expose permanent immutable records or global policies rather than deletable
objects.

## Decision

Use resource-specific native HCL attributes for persisted configuration. Keep
the generated client for existing contracts and use the same configured
transport for EX routes without generated operations. Each handwritten route
is fixed in code; users cannot supply arbitrary HTTP methods, URLs, or headers.
The transport retains bearer authentication, TLS settings and cancellation,
escapes path/query parameters, and reports errors without response bodies or
credential-bearing resolved URLs.

Share mechanical CRUD handling only for ordinary records with compatible
lifecycle contracts. Revisioned policies, redacted material, associations and
one-time credentials use their own handlers. Resource identity and parent
scope are checked before accepting server state. Terraform imports use explicit
identities and hydrate current server values.

Existing optional configuration blocks retain their established removal
semantics. In particular, removing `task_params` from a template or integration
clears those managed defaults. Adding new fields does not silently turn that
operation into preservation of the previous block.

Expose server revisions as computed attributes. Send the prior state revision
on guarded mutations. A conflict remains a conflict; the provider does not
fetch a new revision and retry the write. Named permission sets resolve through
server permission catalogs instead of embedding numeric permission masks.

Use Terraform Actions for operations with no persistent resource lifecycle.
Publishing a template version is explicit and returns publication progress;
the version data source only reads immutable metadata. System option writes
are actions because the API has no deletion contract. They do not imply a
reset on Terraform destroy. Guardrail publication remains an explicit revision-fenced Action. ADR 0010
adds managed draft and retention configuration with forget-on-destroy semantics;
immutable publication history is preserved. Matching data sources expose the
current draft and effective retention policy.
Confidential action inputs use write-only
attributes, with sensitive ephemeral input variables in examples.

Workflow snapshots include the full typed definition. Read-only graph references
use server node IDs instead of inventing configuration aliases. Restoring a
snapshot is an explicit Action that creates a new revision and can cause drift
in a managed workflow resource. The current server restore endpoint has no
expected-revision input; the provider documents that contract rather than
presenting a client-side check as an atomic concurrency guarantee.

Singleton policies document their destroy behavior in their own schema.
Where the API supports a safe disable or deny-all reset, destroy uses that
operation with the same revision checks as other writes. An absent deletion
API is not permission to pretend the server configuration was removed.

Server-generated SSH keys have a separate resource because their creation never
accepts or returns private key material. Login is a write-only creation input;
a persisted version marker requests replacement when it changes. The algorithm
also requires replacement, while name changes preserve key material. Explicit
in-place rotation remains a separate Action.

Successful create identities and completed credential mutations are retained
before reporting later errors. Otherwise a failed read or follow-up operation
could orphan an object or lose its only returned credential. Desired operation
versions are recorded as completed only after the corresponding mutation succeeds.

## Alternatives

An arbitrary HTTP resource would expose the API quickly but provide little
validation, unsafe lifecycle defaults, and poor Terraform planning. A separate
HTTP client would duplicate authentication and TLS behavior. JSON-string
configuration would hide references and types. These alternatives are not
used.

## Verification

Unit tests verify codecs, scope checks, redaction and concurrency payloads.
Acceptance tests use a loopback EX server with an isolated SQLite database and
owned test credentials. Secret stores, OIDC discovery and notification targets
use disposable local fixtures. CRUD and refresh tests do not dispatch real
workloads or contact production identity systems.

## Workflow resource identity and normalization

The original display-name identity decision is superseded by ADR 0010. Workflow
keys now remain stable through server node IDs; display labels may be changed,
empty, or repeated. Unique label-based import keys remain compatible, with
ID-derived keys for ambiguous labels.

Task-parameter decoding preserves explicitly configured empty strings, lists
and nested blocks when the API omits equivalent zero values. It continues to
report removal of previously nonempty server values as drift and preserves the
established whole-block removal semantics.

## Terraform backend aliases

Use the existing fixed alias API and project login/password key type. The alias
resource owns its endpoint and key binding; deleting an alias retains workspace
state history. The server implements the HTTP backend with encrypted append-only
versions and SQL-backed locks shared by all nodes. Acceptance runs a real
Terraform CLI lifecycle with the built-in terraform_data resource in an isolated
fixture, alongside negative authentication and lock-ownership checks. Existing
plaintext state requires the explicit server vault rekey/check upgrade procedure.

## Workflow planning and explicit directory readiness

Preserve omitted optional/computed workflow settings by persisted node key or
edge endpoints when planning. Collection indices are not identity. Keep revision
outputs unknown for real mutations; a plan with no configuration change retains
the current revision. Authored parameter and edge collections still clear when
removed. Refresh never carries forward nonempty collections omitted by the API.

LDAP activation is a separate state transition after an explicitly invoked test
Action records readiness using operator-supplied test and local recovery credentials.
State-only updates do not resave directory configuration and invalidate readiness.
Configuration changes to an active provider require an explicit disable first.

Artifact schema properties are unordered sets of named property definitions.
The server stores them as a JSON object, so list ordering cannot survive a
round trip. Duplicate names are rejected before an API mutation; normalizing
authored list order would instead hide input mistakes and create apply drift.
