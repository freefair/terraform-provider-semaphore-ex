# ADR 0009: Prepare explicitly selected releases directly on main

Status: accepted. Supersedes the version-preparation portion of ADR 0002.

## Decision

Prepare releases directly on main, without release pull requests or release
branches. The owner explicitly selects the version. Update
version references in a normal main commit, run the relevant verification gates,
and push the approved version tag. The signed artifact workflow checks that exact
ref, verifies the packages, and publishes the release after successful verification.

GitHub Releases holds release notes and version history. Remove the repository's
CHANGELOG.md to keep a single maintained source of release information.

Remove the automatic Release Please job from the main workflow so a main push
cannot create an unsolicited version PR. Preserve ordinary main CI, signed snapshot
builds and the existing tag-triggered release verification and publication workflow.

## Rationale

The owner requires direct work on main and retains control of the exact release
version. A semantic-version suggestion from automation is not approval to select
or prepare that version. Signed tag publication remains reproducible and gated
without a separate version PR.
