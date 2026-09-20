#!/usr/bin/env bash
set -euo pipefail

release_dir="${1:-}"
version="${2:-}"
commit="${3:-}"
archive="$release_dir/modelcairn_${version}_linux_amd64.tar.gz"
checksums="$release_dir/SHA256SUMS"
verifier="$(cd "$(dirname "$0")" && pwd)/verify-release-package.sh"

[[ -f "$archive" && -f "$checksums" ]] || { echo "release directory is incomplete" >&2; exit 2; }
work="$(mktemp -d)"
trap 'rm -rf -- "$work"' EXIT

expect_failure() {
  local label="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "$label unexpectedly passed" >&2
    exit 1
  fi
}

cp "$archive" "$work/$(basename "$archive")"
cp "$checksums" "$work/bad.SHA256SUMS"
sed -i "1s/^[0-9a-f]\{64\}/0000000000000000000000000000000000000000000000000000000000000000/" "$work/bad.SHA256SUMS"
expect_failure "incorrect checksum" "$verifier" "$work/$(basename "$archive")" "$work/bad.SHA256SUMS" "$version" "$commit" amd64

size="$(stat -c %s "$work/$(basename "$archive")")"
truncate -s "$((size / 2))" "$work/$(basename "$archive")"
(cd "$work" && sha256sum --binary "$(basename "$archive")" > truncated.SHA256SUMS)
expect_failure "truncated archive" "$verifier" "$work/$(basename "$archive")" "$work/truncated.SHA256SUMS" "$version" "$commit" amd64

expect_failure "wrong architecture" "$verifier" "$archive" "$checksums" "$version" "$commit" arm64
printf 'release package negative cases passed\n'
