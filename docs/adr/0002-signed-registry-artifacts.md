# ADR 0002: Signed CI and Terraform Registry release artifacts

Status: Accepted

## Context

The existing release build runs only after a published-release event and references signing secrets that are absent from the repository.
Available organization secrets provide the Freefair signing key and password.
Release Please uses `GITHUB_TOKEN`, so its generated release/tag events do not trigger another GitHub Actions workflow.
Ordinary CI builds do not retain installable provider packages.

## Decision

Build signed prerelease bundles after successful checks on `main`.
Build exact-version bundles for existing semantic-version tags and publish them automatically, as requested.
Use a reusable artifact workflow for main builds, pushed version tags, and manual dispatch for existing tags.
Release Please prepares version/changelog pull requests only; the owner chooses and pushes the version tag.
The artifact workflow creates a draft, attaches verified assets, and publishes it after completion.
Pull request checks do not receive release signing secrets.

Use the existing `FREEFAIR_TERRAFORM_PRIVATE_KEY` and `FREEFAIR_TERRAFORM_PASSPHRASE` organization secrets.
Distribute the registered public key from `FREEFAIR_TERRAFORM_PUBLIC_KEY` with each bundle and verify the detached binary signature in a fresh keyring against the expected fingerprint.
Keep passwords in environment variables and a file descriptor to GPG, not command arguments or configuration.
Accept Registry-compatible RSA/DSA keys and reject incompatible key algorithms before publishing.

Build with the pinned GoReleaser version, bounded parallelism, and explicit provider archive names.
Verify every archive, manifest and checksum, then execute the host-platform archive through Terraform's plugin protocol.
Publish only the ZIP files, versioned manifest, checksum file, detached signature and public key, rather than the whole GoReleaser working directory.

## Alternatives and consequences

A published-release event alone is insufficient because token-created events are suppressed.
Calling a build after Release Please creates an already-published release also exposes an incomplete release to Registry webhooks; separating version PRs from tag publication avoids that race.
Unsigned snapshots cannot meet the requested CI deliverable.
Draft-only releases would require a manual publication step; the requested behavior is automatic publication for version tags.
Initial provider registration and signing-key registration in the public Terraform Registry remain owner actions.
Existing released assets are not overwritten; reruns must match their bytes or fail.

## Plan

1. Correct `.goreleaser.yml` and add noninteractive signing/signature verification scripts.
2. Add artifact validation and a packaged-provider smoke test.
3. Make `.github/workflows/test.yml` reusable and connect checks, CI bundles and tag releases in the release workflows.
4. Document secret names, artifact names, Registry onboarding and manual recovery.
5. Validate workflow syntax, build a full local bundle with a disposable RSA test key, verify tamper rejection and Terraform startup, review the diff, and commit.

The disposable local signing key is verification material only and is never promoted to a release identity.
