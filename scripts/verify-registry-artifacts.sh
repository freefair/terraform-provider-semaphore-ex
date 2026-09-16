#!/usr/bin/env bash
set -Eeuo pipefail

work_dir=''
cleanup() {
  trap - SIGINT SIGTERM ERR EXIT
  [[ -z "$work_dir" ]] || rm -rf "$work_dir"
}
trap cleanup EXIT
trap 'exit 130' SIGINT
trap 'exit 143' SIGTERM

usage() {
  cat <<EOF
Usage: $(basename "${BASH_SOURCE[0]}") DIST

Verify a signed Registry bundle, inspect every ZIP, and load the packaged host
binary through Terraform. Requires bash, gpg, jq, unzip, shasum and terraform.

Environment:
  GPG_FINGERPRINT  Expected public signing-key fingerprint.

Example:
  GPG_FINGERPRINT=... $(basename "${BASH_SOURCE[0]}") dist
EOF
}
die() { printf 'error: %s\n' "$*" >&2; exit 1; }
case "${1-}" in -h|--help) usage; exit 0 ;; esac
[[ $# -eq 1 && -d "$1" ]] || { usage >&2; exit 1; }
[[ -n "${GPG_FINGERPRINT-}" ]] || die 'GPG_FINGERPRINT is required'
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
dist_dir="$(cd "$1" && pwd)"
provider='terraform-provider-semaphore-ex'
shopt -s nullglob
sums=("$dist_dir/${provider}_"*_SHA256SUMS)
[[ ${#sums[@]} -eq 1 ]] || die 'expected exactly one SHA256SUMS file'
checksum="${sums[0]}"
prefix="$(basename "$checksum" _SHA256SUMS)"
version="${prefix#"${provider}"_}"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || die 'invalid artifact version'

"$script_dir/verify-registry-signature.sh" \
  --public-key "$dist_dir/signing-key.asc" --fingerprint "$GPG_FINGERPRINT" \
  --checksum "$checksum" --signature "$checksum.sig"

archives=("$dist_dir/${prefix}_"*.zip)
[[ ${#archives[@]} -gt 0 ]] || die 'no provider ZIP archives'
manifest="${prefix}_manifest.json"
jq -e '.version == 1 and .metadata.protocol_versions == ["6.0"]' "$dist_dir/$manifest" >/dev/null
expected_count=$((${#archives[@]} + 1))
actual_count=0
while read -r hash name extra; do
  [[ "$hash" =~ ^[a-fA-F0-9]{64}$ && -z "$extra" ]] || die 'malformed checksum entry'
  case "$name" in
    "$manifest") ;;
    "${prefix}_"*.zip) [[ "$name" != */* ]] || die 'archive paths must be flat' ;;
    *) die "unexpected checksummed asset: $name" ;;
  esac
  [[ -f "$dist_dir/$name" ]] || die "missing checksummed asset: $name"
  actual_count=$((actual_count + 1))
done < "$checksum"
[[ "$actual_count" -eq "$expected_count" ]] || die 'ZIP/manifest checksum coverage mismatch'
[[ "$(awk '{print $2}' "$checksum" | sort -u | wc -l | tr -d ' ')" -eq "$expected_count" ]] || die 'duplicate checksum entries'
(cd "$dist_dir" && shasum -a 256 -c "$(basename "$checksum")")

for archive in "${archives[@]}"; do
  name="$(basename "$archive")"
  platform="${name#"${prefix}"_}"
  platform="${platform%.zip}"
  [[ "$platform" =~ ^(linux|darwin|windows|freebsd)_(amd64|arm64|386|arm)$ ]] || die "unexpected platform: $platform"
  binary="${provider}_v${version}"
  [[ "$platform" != windows_* ]] || binary+='.exe'
  found=0
  while IFS= read -r entry; do
    case "$entry" in
      "$binary") found=$((found + 1)) ;;
      LICENSE|LICENSE.*|README|README.md|CHANGELOG|CHANGELOG.md) ;;
      *) die "unexpected ZIP entry in $name: $entry" ;;
    esac
  done < <(unzip -Z1 "$archive")
  [[ "$found" -eq 1 ]] || die "missing or duplicate provider binary in $name"
  unzip -tq "$archive"
done

case "$(uname -s)" in Darwin) host_os=darwin ;; Linux) host_os=linux ;; *) die 'smoke test requires Linux or macOS' ;; esac
case "$(uname -m)" in x86_64) host_arch=amd64 ;; arm64|aarch64) host_arch=arm64 ;; *) die 'unsupported smoke-test CPU' ;; esac
native="$dist_dir/${prefix}_${host_os}_${host_arch}.zip"
[[ -f "$native" ]] || die 'bundle has no host-platform archive'
work_dir="$(mktemp -d)"
mirror="$work_dir/mirror/registry.terraform.io/freefair/semaphore-ex/$version/${host_os}_${host_arch}"
mkdir -p "$mirror" "$work_dir/config"
unzip -q "$native" "${provider}_v${version}" -d "$mirror"
[[ -x "$mirror/${provider}_v${version}" ]] || die 'packaged provider is not executable'
cat > "$work_dir/terraform.rc" <<EOF
provider_installation {
  filesystem_mirror { path = "$work_dir/mirror" }
}
EOF
cat > "$work_dir/config/main.tf" <<EOF
terraform {
  required_providers {
    semaphore = {
      source = "freefair/semaphore-ex"
      version = "= $version"
    }
  }
}
EOF
export TF_CLI_CONFIG_FILE="$work_dir/terraform.rc" TF_DATA_DIR="$work_dir/data" TF_INPUT=false CHECKPOINT_DISABLE=1
unset TF_CLI_ARGS TF_CLI_ARGS_init TF_CLI_ARGS_providers
terraform -chdir="$work_dir/config" init -backend=false -input=false -no-color
terraform -chdir="$work_dir/config" providers schema -json > "$work_dir/schema.json"
jq -e '.provider_schemas["registry.terraform.io/freefair/semaphore-ex"].resource_schemas as $r |
  ($r.semaphore_ex_project_template.block.attributes.environment_ids != null) and
  ($r.semaphore_ex_project_user.block.attributes.revision.computed == true) and
  ($r.semaphore_ex_runner.block.attributes.registration_policy != null)' "$work_dir/schema.json" >/dev/null
printf 'Verified %s archives, manifest, checksums, signature, and Terraform plugin startup for %s.\n' "${#archives[@]}" "$version"
