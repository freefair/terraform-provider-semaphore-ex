# Lifecycle review fixes and provider release

## Scope

Fix the two confirmed Claude findings and publish the verified provider through
the existing signed GitHub/Registry release workflow. Preserve the sibling server
checkout and use only disposable loopback acceptance fixtures for verification.

## Decisions

1. Confirm project-scoped HTTP 404s at the shared provider transport, covering
   generated and native EX clients. A readable project establishes access for a
   missing child. An unreadable project is considered deleted only when a fresh
   current-user response confirms administrator access. Otherwise return a
   diagnostic and preserve state. An API-wide authentication/status change in
   Semaphore would require another server rollout and is outside this fix.
2. Adopt the first keepers map when imported token state has neither a recovered
   credential nor a keepers baseline. Persist that map without an issuance call.
   Subsequent keepers changes and explicit replacement retain rotation behavior.
   Merely documenting the unexpected first rotation would not resolve the finding.

## Files and sequence

1. Add failing temporary regressions for ambiguous project 404s and keeper adoption.
2. Add the guarded transport, integrate it in provider.go, and add protocol/HTTP
   tests including denied access, real absence, proxy failures and cancellation.
3. Update runner_registration_token_schema.go and its import tests for baseline
   adoption, subsequent rotation and explicit replacement.
4. Update lifecycle/migration documentation, add ADR 0008 and regenerate Registry docs.
5. Run unit, acceptance, lint, build and documentation freshness checks. Have Claude
   review the fixes; validate and address findings within the approved scope.
6. Commit and push main, verify CI on the exact commit, merge the generated release
   PR, and publish its version tag. Verify signed assets and Registry installation.
7. Remove disposable task artifacts and archive the task journal with release evidence.
