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
