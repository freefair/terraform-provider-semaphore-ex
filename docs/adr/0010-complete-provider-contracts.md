# ADR 0010: Preserve behavior while completing provider contracts

Status: accepted.

## Decision

Treat the EX backend's active request/response and execution contracts as the source
of truth. Track fields and operations explicitly, including justified exclusions;
resource and data-source counts alone do not establish coverage.

Template settings and invocation parameters have separate models. Introduce explicit
Ansible and Terraform template settings. Retain the old template `task_params`
configuration and its recoverable invocation envelope as deprecated compatibility
metadata without promoting its previously
inert fields into effective settings. In particular, upgrading the provider must
not enable automatic approval or change run limits without an explicit configuration
change. Read current template settings before updates and preserve unmanaged keys.
The template API has no revision fence; this merge cannot eliminate concurrent-write
races.

Preserve the published string-map environment inputs and add lossless JSON inputs
for typed and nested values. Decode null safely and report malformed responses.
Existing string-map states and legacy-provider moves remain readable. Refresh must
report externally removed data, retaining prior values only for redacted credentials
or equivalent explicitly configured empty values.

Data sources perform lookups only. Missing external users produce an error; their
creation belongs to `semaphore_ex_user` with `external = true`. Name lookups reject
ambiguous matches within the owning scope and process pagination where the API uses
it. Operational tests, previews that create review records, and mutations remain
explicit Actions.

Workflow node identity follows the persisted server ID independently of display
labels. Existing Terraform keys remain stable across refresh and label changes.
Import assigns deterministic keys without requiring unique nonempty display names.

Managed guardrail drafts do not publish themselves. Append-only retention policies
and runtime-secrets configuration have explicit ownership/forget semantics rather
than pretending the API can delete historical revisions. Runtime-secrets import and
drift detection require reading configured state and expiry, not a resolved expired
capability decision. Volatile runtime metadata belongs on data sources.

## Verification

Reproduce corrected contracts before implementation. Verify API wire shape and
backend behavior independently of provider state round trips. Exercise updates,
clearing, external drift, import, old state, unknown/null values and no-op plans.
Include regression coverage for data-source writes, duplicate lookup names and
request cancellation. Use a disposable loopback backend for acceptance tests.

## Alternatives

Reusing invocation schemas for templates makes structurally valid JSON semantically
inert. Automatically activating old template values changes behavior during upgrade.
Replacing published string-map inputs with dynamic values forces an avoidable state
migration; additive JSON inputs retain compatibility. Removing unmapped fields on
update loses settings managed through other clients. Mapping previews with stored
review tokens into ordinary data-source reads introduces hidden operations during
planning.

## Lookup implementation

A shared data-source adapter changes only selection: it resolves an exact scoped
name, then delegates to the existing reader using the resolved ID. This keeps detail
conversion and secret redaction in one place. Collection schemas deliberately list
allowed summary fields instead of copying arbitrary API response objects into state.
Only routes with documented count/offset pagination receive those parameters;
unpaginated APIs are read once. Ordering follows stable server IDs.

## Workflow identity compatibility

Refresh maps persisted server IDs to existing Terraform keys. Import retains unique
nonempty labels as initial keys for compatibility with earlier imports, and assigns
ID-derived keys where labels are empty or ambiguous. Labels never replace existing
keys on refresh. The server's POST/PUT implementation preserves request node order
while replacing temporary IDs; the provider binds those returned IDs once and then
uses ID-based reconciliation. Existing positive IDs and node counts are checked.
This avoids requiring unique labels or adding a server-side Terraform naming field.

Survey options use an additive ordered `choices` list. The published `enum_values`
map type remains available and becomes a computed counterpart where labels are
unique. Repeated labels yield a null map rather than silently dropping an option.
Import hydrates both representations. An unchanged legacy map preserves the server's
current order; changed maps use deterministic lexical label order. Explicit lists
preserve their authored order. Omission retains options; an explicit empty list clears.

## Operational results

Fixed-route Actions cover explicit backend operations. Synchronous previews also
provide ephemeral resources so callers can inspect complete results and pass a
preview token to a write-only Action input without persistent Terraform state.
LDAP/OIDC previews still create backend history and are documented accordingly.
Mutating delivery tests, workflow starts, retries and rollback remain Actions only.
No automatic review, retry, renewal or audit-history cleanup is implied.

## Official framework and accepted release defect

Use the unmodified official HashiCorp framework. Provider v1.0.5 explicitly retains
the reproduced framework v1.19.0 WriteOnly Action validation defect. Publish its
affected operations in the release notes and generated references. Preserve
write-only inputs; resource ignore_changes is not an Action validation workaround.
The acceptance test records the exact known rejection and ensures no apply API call
occurs. Ordinary resource and ephemeral functionality keeps its positive tests.
A framework fork or local patch is excluded by the owner.
