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

The provider and tooling require Go 1.26.6, and the provider selects gRPC 1.83.2.
These floors also address reachable findings identified by `govulncheck` beyond
the original Dependabot alert set. Keep the local toolchain pin and contributor
instructions aligned with both module directives so release binaries include
the standard-library fixes.

The acceptance job selects the server's declared toolchain for its build and
then explicitly selects the provider's toolchain before running provider tests.
This keeps the independently pinned server revision compatible without leaving
provider tests on an older compiler when automatic toolchain selection is disabled.

## Consequences

Both module manifests and checksum files carry the correction. Provider builds,
lint, isolated Semaphore EX acceptance tests, and documentation regeneration
verify the affected runtime and tooling paths. Security scan results are reviewed
separately from Dependabot's alert count: closing existing alerts does not establish
that the selected compiler and dependency graph have no other known findings.
