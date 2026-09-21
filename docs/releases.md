# Signed builds and Terraform Registry releases

## Build outputs

Every successful `main` CI run produces a downloadable, signed prerelease bundle.
The version is derived from Git as `<version>-dev.<commit>`; a repository without tags starts at `0.0.0-dev.<commit>`.
These CI bundles are useful for testing and are not published as stable versions.

Pushing an existing-source version tag such as `v1.2.3` runs checks against that exact ref, builds and verifies its bundle, then publishes a GitHub Release automatically.
The tagged commit must belong to `main` history.
Prepare the exact version explicitly selected by the owner directly on `main`.
Update version references, verify and commit the changes, then push `main` and the approved version tag.
Maintain release notes and version history exclusively in GitHub Releases.
Release preparation uses no pull requests; the workflow does not generate them.

The **Signed Registry artifacts** workflow also accepts an existing version tag through manual dispatch for recovery.
Leaving the tag empty produces a signed snapshot of the selected main-history commit.
Tags and released assets are never replaced: a rerun accepts matching files and fails if an existing asset differs.

Bundles contain:

| File | Purpose |
| --- | --- |
| `terraform-provider-semaphore-ex_<version>_<os>_<arch>.zip` | One executable provider for one platform |
| `terraform-provider-semaphore-ex_<version>_manifest.json` | Registry manifest declaring plugin protocol 6.0 |
| `terraform-provider-semaphore-ex_<version>_SHA256SUMS` | SHA256 hashes of every platform ZIP and the manifest |
| `terraform-provider-semaphore-ex_<version>_SHA256SUMS.sig` | Binary detached OpenPGP signature over the checksum file |
| `signing-key.asc` | Public signing key for verification and Registry registration |

The executable inside each ZIP is `terraform-provider-semaphore-ex_v<version>` (`.exe` on Windows).
The build covers Linux and FreeBSD on amd64, arm64, 386 and ARM; Windows on amd64, arm64 and 386; and macOS on amd64 and arm64.
GoReleaser runs with two concurrent builds and `CGO_ENABLED=0`.
Only distributable files are uploaded; private keys, metadata/config dumps and intermediate binaries are excluded.

## Signing configuration

The repository uses these existing Freefair organization secrets:

- `FREEFAIR_TERRAFORM_PRIVATE_KEY`: ASCII-armored private signing key or its base64 encoding, accepted by the import action.
- `FREEFAIR_TERRAFORM_PASSPHRASE`: passphrase for that key.
- `FREEFAIR_TERRAFORM_PUBLIC_KEY`: ASCII-armored public key registered with the Registry.

The Registry signing identity is key ID `719010B911115D8E`.
The workflow checks that the imported primary-key fingerprint matches this ID
before building, then uses that fingerprint for signing and verification.
Verification uses the separately configured public key and requires its primary
fingerprint to match the imported private key.
The configured secrets must contain this key; a valid signature from another
Freefair key is insufficient for Registry publication.
The passphrase reaches GPG through standard input and is absent from command arguments and configuration files.
Pull request checks have no signing step; release signing is limited to this repository and checked main-history commits.

The public Terraform Registry requires an RSA or DSA signing key.
The verification gate rejects incompatible algorithms, unexpected fingerprints, invalid signatures and armored signature files.
Changing the key requires registering the corresponding public key with the Registry before publishing with it.

## Verification gates

The reusable test workflow checks build, lint, generated documentation, and Terraform acceptance against the pinned Semaphore EX server revision.
Packaging then verifies:

1. Version tag matches the checked-out commit and the source belongs to `main` history.
2. The detached signature validates against the configured Registry public key and expected fingerprint in a fresh keyring.
3. Every ZIP and the versioned manifest occurs exactly once in the checksum file and matches its hash.
4. ZIP contents contain the correctly named platform executable and expected documentation files.
5. Terraform installs the host-platform executable from the ZIP through a local filesystem mirror and successfully requests its provider schema.

The filesystem-mirror smoke test reports the local installation as unauthenticated because Terraform does not authenticate mirrors itself.
The separate GPG verification gate authenticates the bundle before that smoke test.
CI executes the Linux amd64 archive; local verification can also execute the macOS archive.

## First publication to the public Registry

1. Push the verified provider/workflow commits and inspect the signed CI bundle.
2. Confirm that `signing-key.asc` belongs to the registered key `719010B911115D8E` in the Terraform Registry's signing-key settings for the `freefair` namespace.
3. Push the chosen semantic-version tag and wait for **Signed Registry artifacts** to finish.
4. Confirm the public GitHub Release contains the platform ZIPs, manifest, checksums and binary signature.
5. In the Terraform Registry, choose **Publish → Provider**, select `freefair/terraform-provider-semaphore-ex`, and finish the onboarding flow.

Once registered, the Registry's release webhook handles subsequent published version tags.
The workflow constructs a draft first and only finalizes it after verified assets have been uploaded.
It does not require a Terraform Registry API token to build or sign packages.
If a release is complete but missing in the Registry, inspect its webhook delivery and use the Registry's Resync action.

See [HashiCorp's publication requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing) for file naming, signing and Registry onboarding.

## Local verification

GoReleaser is pinned in `.tool-versions` and in the artifact workflow.
Use a signing key already imported into an isolated keyring, with `GPG_FINGERPRINT` and `SIGNING_PASSPHRASE` supplied by the caller.

```sh
goreleaser check
goreleaser release --snapshot --skip=publish --parallelism=2
# Materialize the versioned manifest using the name in dist/*_SHA256SUMS.
# Export only the public key to dist/signing-key.asc.
scripts/verify-registry-artifacts.sh dist
```

The scripts' `--help` output documents their arguments.
Local disposable test keys prove signing and verification behavior, but are not release identities and must not be registered or used for production publication.
