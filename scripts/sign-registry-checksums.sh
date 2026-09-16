#!/usr/bin/env bash
set -Eeuo pipefail

cleanup() { trap - SIGINT SIGTERM ERR EXIT; }
trap cleanup EXIT
trap 'exit 130' SIGINT
trap 'exit 143' SIGTERM

usage() {
  cat <<EOF
Usage: $(basename "${BASH_SOURCE[0]}") CHECKSUM SIGNATURE

Create a binary detached OpenPGP signature for a Terraform Registry checksum file.

Environment:
  GPG_FINGERPRINT    Required signing-key fingerprint.
  SIGNING_PASSPHRASE Required signing-key passphrase; supplied to GPG only on stdin.

Example:
  GPG_FINGERPRINT=... SIGNING_PASSPHRASE=... $(basename "${BASH_SOURCE[0]}") SHA256SUMS SHA256SUMS.sig
EOF
}

die() { printf 'error: %s\n' "$*" >&2; exit 1; }

case "${1-}" in -h|--help) usage; exit 0;; esac
[[ "$#" -eq 2 ]] || { usage >&2; die 'expected CHECKSUM and SIGNATURE'; }
[[ -n "${GPG_FINGERPRINT-}" ]] || die 'GPG_FINGERPRINT is required'
[[ -n "${SIGNING_PASSPHRASE-}" ]] || die 'SIGNING_PASSPHRASE is required'
[[ -f "$1" ]] || die "checksum file does not exist: $1"

printf '%s' "${SIGNING_PASSPHRASE}" | gpg --batch --yes --pinentry-mode loopback \
  --passphrase-fd 0 --local-user "${GPG_FINGERPRINT}" --output "$2" --detach-sign "$1"
