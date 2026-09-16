#!/usr/bin/env bash
set -Eeuo pipefail

temp_dir=''
cleanup() { [[ -z "${temp_dir}" ]] || rm -rf "${temp_dir}"; trap - SIGINT SIGTERM ERR EXIT; }
trap cleanup EXIT
trap 'exit 130' SIGINT
trap 'exit 143' SIGTERM

usage() {
  cat <<EOF
Usage: $(basename "${BASH_SOURCE[0]}") --public-key KEY --fingerprint FINGERPRINT --checksum FILE --signature FILE

Verify a binary detached Terraform Registry checksum signature in a fresh GPG keyring.

Example:
  $(basename "${BASH_SOURCE[0]}") --public-key release.pub --fingerprint ABCD... --checksum SHA256SUMS --signature SHA256SUMS.sig
EOF
}

die() { printf 'error: %s\n' "$*" >&2; exit 1; }
public_key='' fingerprint='' checksum='' signature=''
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0;;
    --public-key) public_key="${2-}"; shift 2;;
    --fingerprint) fingerprint="${2-}"; shift 2;;
    --checksum) checksum="${2-}"; shift 2;;
    --signature) signature="${2-}"; shift 2;;
    *) die "unknown argument: $1";;
  esac
done
[[ -n "${public_key}" && -n "${fingerprint}" && -n "${checksum}" && -n "${signature}" ]] || { usage >&2; die 'all arguments are required'; }
[[ -f "${public_key}" && -f "${checksum}" && -f "${signature}" ]] || die 'public key, checksum, and signature must exist'
grep -q -- '-----BEGIN PGP SIGNATURE-----' "${signature}" && die 'ASCII-armored signatures are not accepted'
temp_dir="$(mktemp -d)"
chmod 700 "${temp_dir}"
primary_key="$(gpg --homedir "${temp_dir}" --batch --with-colons --import-options show-only --import "${public_key}" 2>/dev/null | awk -F: '$1 == "pub" { count++; fingerprint=$5; algorithm=$4 } END { if (count != 1 || algorithm !~ /^(1|2|3|17)$/) exit 1; print fingerprint }')" || die 'Registry public key must contain exactly one RSA or DSA primary key'
[[ -n "${primary_key}" ]] || die 'Registry public key must contain exactly one RSA or DSA primary key'
gpg --homedir "${temp_dir}" --batch --import "${public_key}" >/dev/null
actual_fingerprint="$(gpg --homedir "${temp_dir}" --batch --with-colons --fingerprint | awk -F: '$1 == "fpr" {print $10; exit}')"
[[ "${actual_fingerprint}" == "${fingerprint}" ]] || die 'imported public-key fingerprint does not match expected fingerprint'
gpg --homedir "${temp_dir}" --batch --no-auto-key-retrieve --verify "${signature}" "${checksum}"
