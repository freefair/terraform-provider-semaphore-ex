# Bind releases to the registered signing identity

## Status

Accepted.

## Context

A checksum signature can be cryptographically valid while the public Terraform
Registry rejects it because it was created by a different key. Verifying against
a public key exported from the same imported CI secret establishes consistency,
but does not establish that CI used the identity registered for this provider.

## Decision

Require the Registry primary-key ID `719010B911115D8E` before building and signing.
Validate the imported full fingerprint's format and long key ID, then carry that
fingerprint through signing and artifact verification. Fail before publication
when the configured secret imports a different identity.

Use the dedicated `FREEFAIR_TERRAFORM_PRIVATE_KEY` and
`FREEFAIR_TERRAFORM_PASSPHRASE` secrets to sign, and the independently configured
`FREEFAIR_TERRAFORM_PUBLIC_KEY` to verify and distribute the registered public key.

## Consequences

The CI secret and Registry configuration must agree. A general-purpose Freefair
signing key cannot silently replace the registered provider key. Changing the
release identity requires an explicit configuration change and registration of
the corresponding public key.
