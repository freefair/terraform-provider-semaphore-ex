# Pin advisory dependency floors

## Status

Accepted.

## Context

The provider and documentation generator have separate Go modules. Dependabot
reports vulnerable indirect dependencies in both: gRPC in the provider and
`golang.org/x/crypto` in the documentation tooling.

## Decision

Select the patched dependency floors in the module that owns each dependency.
Keep direct dependency APIs unchanged and include only transitive version changes
required by Go's module selection. Avoid a blanket dependency upgrade, which would
mix unrelated compatibility changes into the security fix.

## Consequences

Both module manifests and checksum files carry the correction. Provider builds,
lint, isolated Semaphore EX acceptance tests, and documentation regeneration
verify the affected runtime and tooling paths. Security scan results are reviewed
separately from Dependabot's alert count: closing existing alerts does not establish
that the selected compiler and dependency graph have no other known findings.
