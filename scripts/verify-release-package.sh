#!/usr/bin/env bash
set -euo pipefail

archive="${1:-}"
checksums="${2:-}"
expected_version="${3:-}"
expected_commit="${4:-}"
expected_arch="${5:-}"

[[ -f "$archive" && -f "$checksums" ]] || { echo "archive and checksum file are required" >&2; exit 2; }
[[ "$expected_version" =~ ^0\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] || { echo "invalid expected version" >&2; exit 2; }
[[ "$expected_commit" =~ ^[0-9a-f]{40}$ ]] || { echo "invalid expected commit" >&2; exit 2; }
[[ "$expected_arch" == "amd64" || "$expected_arch" == "arm64" ]] || { echo "invalid expected architecture" >&2; exit 2; }

archive="$(cd "$(dirname "$archive")" && pwd)/$(basename "$archive")"
checksums="$(cd "$(dirname "$checksums")" && pwd)/$(basename "$checksums")"
(
  cd "$(dirname "$archive")"
  grep -F " *$(basename "$archive")" "$checksums" | sha256sum --check --strict -
)

root="modelcairn_${expected_version}_linux_${expected_arch}"
allowed="$(printf '%s\n' CHANGELOG.md LICENSE NOTICE README.md TRADEMARKS.md manifest.json modelcairn | sed "s#^#$root/#" | sort)"
actual="$(tar -tzf "$archive" | sed '/\/$/d' | sort)"
[[ "$actual" == "$allowed" ]] || { echo "archive allowlist mismatch" >&2; diff -u <(printf '%s\n' "$allowed") <(printf '%s\n' "$actual") || true; exit 1; }
tar -tzf "$archive" | grep -Eq '(^|/)\.\.?(/|$)' && { echo "unsafe archive path" >&2; exit 1; } || true
[[ "$(tar -tvzf "$archive" "$root" | awk 'NR==1 {print $1}')" == "drwxr-xr-x" ]] || { echo "invalid package-directory mode" >&2; exit 1; }
[[ "$(tar -tvzf "$archive" "$root/modelcairn" | awk 'NR==1 {print $1}')" == "-rwxr-xr-x" ]] || { echo "binary is not mode 0755" >&2; exit 1; }
for file in CHANGELOG.md LICENSE NOTICE README.md TRADEMARKS.md manifest.json; do
  [[ "$(tar -tvzf "$archive" "$root/$file" | awk 'NR==1 {print $1}')" == "-rw-r--r--" ]] || { echo "invalid mode for $file" >&2; exit 1; }
done

stage="$(mktemp -d)"
trap 'rm -rf -- "$stage"' EXIT
tar -xzf "$archive" -C "$stage"
node -e 'const fs=require("fs");const [path,version,commit,arch]=process.argv.slice(1);const value=JSON.parse(fs.readFileSync(path,"utf8"));if(value.version!==version||value.commit!==commit||value.os!=="linux"||value.arch!==arch||!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/.test(value.date))process.exit(1)' \
  "$stage/$root/manifest.json" "$expected_version" "$expected_commit" "$expected_arch"
go version -m "$stage/$root/modelcairn" | grep -Fq $'path\tgithub.com/sxamx/modelcairn/cmd/modelcairn'
case "$expected_arch" in
  amd64) file "$stage/$root/modelcairn" | grep -Eq 'x86-64|x86_64' ;;
  arm64) file "$stage/$root/modelcairn" | grep -Eq 'ARM aarch64|ARM64|aarch64' ;;
esac
if [[ "$(uname -s)" == "Linux" && "$(uname -m)" == "x86_64" && "$expected_arch" == "amd64" ]]; then
  "$stage/$root/modelcairn" version | grep -Fq "modelcairn $expected_version (commit=$expected_commit"
fi
printf 'verified %s\n' "$(basename "$archive")"
