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
Usage: $(basename "${BASH_SOURCE[0]}") TAG DIST

Attach a verified Registry bundle to its existing version tag and publish it.
Creates a draft first when no release exists. Existing assets must match byte
for byte; this command never replaces them. Run artifact verification first.

Environment:
  GH_TOKEN  GitHub Actions token with contents:write, supplied by the workflow.

Example:
  $(basename "${BASH_SOURCE[0]}") v1.2.3 dist
EOF
}
die() { printf 'error: %s\n' "$*" >&2; exit 1; }
case "${1-}" in -h|--help) usage; exit 0 ;; esac
[[ $# -eq 2 ]] || { usage >&2; exit 1; }
tag="$1"
[[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || die 'expected a semantic version tag beginning with v'
dist_dir="$(cd "$2" && pwd)"
prefix="terraform-provider-semaphore-ex_${tag#v}"
shopt -s nullglob
assets=("$dist_dir/${prefix}_"*.zip)
[[ ${#assets[@]} -gt 0 ]] || die 'version-tag ZIP files are missing'
assets+=("$dist_dir/${prefix}_manifest.json" "$dist_dir/${prefix}_SHA256SUMS" "$dist_dir/${prefix}_SHA256SUMS.sig" "$dist_dir/signing-key.asc")
for asset in "${assets[@]}"; do
  [[ -s "$asset" ]] || die "missing release asset: $asset"
done
work_dir="$(mktemp -d)"
if ! gh release view "$tag" --json isDraft,assets > "$work_dir/release.json"; then
  gh release create "$tag" --verify-tag --draft --title "$tag" --generate-notes
  gh release view "$tag" --json isDraft,assets > "$work_dir/release.json"
fi
pending=()
for asset in "${assets[@]}"; do
  name="$(basename "$asset")"
  if jq -e --arg name "$name" '.assets[] | select(.name == $name)' "$work_dir/release.json" >/dev/null; then
    gh release download "$tag" --pattern "$name" --dir "$work_dir"
    cmp -s "$asset" "$work_dir/$name" || die "existing release asset differs: $name; release a new version instead"
  else
    pending+=("$asset")
  fi
done
if [[ ${#pending[@]} -gt 0 ]]; then
  gh release upload "$tag" "${pending[@]}"
fi
if [[ "$(jq -r .isDraft "$work_dir/release.json")" == true ]]; then
  gh release edit "$tag" --draft=false
fi
gh release view "$tag" --json url,assets --jq '{url, assets: [.assets[].name]}'
