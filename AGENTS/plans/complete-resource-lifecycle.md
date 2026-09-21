# Complete resource lifecycle and data-source coverage

## Objective

Support Terraform resource operations across every registered Semaphore EX resource,
including migration from `semaphoreui/semaphore`, and add missing readable data sources.

## Approach and alternatives

Use the framework's existing CRUD/import interfaces and explicit legacy state movers.
Terraform owns same-type address moves, forgetting, targeting and replacement ordering;
test those through Terraform rather than adding provider-side state mutation commands.
Accept only known legacy source types and schema versions for cross-provider moves.
Arbitrary conversions between unrelated objects would lose identity and are excluded.

## Implementation order

1. Audit registrations, schemas, API handlers and existing acceptance coverage.
2. Add regression tests for import validation, missing-object refresh/delete and state
   conversion; fix `import.go` and affected lifecycle methods.
3. Add explicit migration methods in `state_move.go`, retaining compatible values and
   validating incompatible source fields before changing state.
4. Add the integration-alias data source using its existing list API. Assess one-time
   credentials against the server contract; reads/imports must not rotate credentials.
5. Exercise CLI/block imports, moves, forgetting, refresh, update, replacement and
   destroy in isolated test fixtures; run unit, acceptance, build, lint and docs checks.
6. Document all resource contracts, import formats, migration boundaries and evidence
   in `docs/lifecycle.md`, `docs/migration.md` and an ADR. Commit verified work locally.

## Environment

Provider checkout: existing clean `main`. API tests use the repository's disposable
loopback server with its own SQLite database and generated credentials. No shared
instance or existing Terraform state is a test target. Preserve sibling checkout WIP.
