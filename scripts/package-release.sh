#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "$0")/.." && pwd)"
version="${1:-}"
output_dir="${2:-$repository_root/dist/release}"

[[ "$version" =~ ^0\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] || {
  echo "version must be an unstable SemVer without the v prefix (for example 0.1.0 or 0.1.0-rc.1)" >&2
  exit 2
}
command -v go >/dev/null || { echo "go is required" >&2; exit 2; }
command -v tar >/dev/null || { echo "tar is required" >&2; exit 2; }
command -v gzip >/dev/null || { echo "gzip is required" >&2; exit 2; }
command -v sha256sum >/dev/null || { echo "sha256sum is required" >&2; exit 2; }

cd "$repository_root"
if [[ "${ALLOW_DIRTY_RELEASE:-0}" != "1" && -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  echo "release packaging requires a clean worktree" >&2
  exit 1
fi
commit="$(git rev-parse HEAD)"
[[ "$commit" =~ ^[0-9a-f]{40}$ ]] || { echo "unable to resolve commit" >&2; exit 2; }
source_date_epoch="${SOURCE_DATE_EPOCH:-$(git show -s --format=%ct HEAD)}"
[[ "$source_date_epoch" =~ ^[0-9]+$ ]] || { echo "invalid SOURCE_DATE_EPOCH" >&2; exit 2; }
build_date="$(date -u -d "@$source_date_epoch" '+%Y-%m-%dT%H:%M:%SZ')"

mkdir -p "$output_dir"
output_dir="$(cd "$output_dir" && pwd)"
find "$output_dir" -mindepth 1 -maxdepth 1 -type f -delete

for arch in amd64 arm64; do
  package="modelcairn_${version}_linux_${arch}"
  stage="$(mktemp -d)"
  trap 'rm -rf -- "${stage:-}"' EXIT
  mkdir -p "$stage/$package"
  ldflags="-s -w -X github.com/sxamx/modelcairn/internal/buildinfo.Version=$version -X github.com/sxamx/modelcairn/internal/buildinfo.Commit=$commit -X github.com/sxamx/modelcairn/internal/buildinfo.Date=$build_date"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags="$ldflags" -o "$stage/$package/modelcairn" ./cmd/modelcairn
  cp LICENSE NOTICE README.md TRADEMARKS.md CHANGELOG.md "$stage/$package/"
  mkdir -p "$stage/$package/scripts" "$stage/$package/packaging/systemd" "$stage/$package/docs/operacion"
  cp scripts/install-linux.sh scripts/bootstrap-linux.sh scripts/uninstall-linux.sh "$stage/$package/scripts/"
  cp packaging/systemd/modelcairn.service "$stage/$package/packaging/systemd/"
  cp docs/operacion/linux-installation-v1.md docs/operacion/instalacion-linux-v1.es.md \
    docs/operacion/backup-recovery-v1.md docs/operacion/backup-recuperacion-v1.es.md \
    docs/operacion/acceso-red-v1.md docs/operacion/acceso-red-v1.es.md "$stage/$package/docs/operacion/"
  printf '{\n  "version": "%s",\n  "commit": "%s",\n  "date": "%s",\n  "os": "linux",\n  "arch": "%s"\n}\n' \
    "$version" "$commit" "$build_date" "$arch" >"$stage/$package/manifest.json"
  chmod 0755 "$stage/$package/modelcairn"
  chmod 0755 "$stage/$package/scripts/"*.sh
  chmod 0644 "$stage/$package/"*.md "$stage/$package/LICENSE" "$stage/$package/NOTICE" "$stage/$package/manifest.json"
  archive_tar="$stage/$package.tar"
  tar --sort=name --mtime="@$source_date_epoch" --owner=0 --group=0 --numeric-owner --mode=0755 \
    --no-recursion -C "$stage" -cf "$archive_tar" "$package"
  tar --sort=name --mtime="@$source_date_epoch" --owner=0 --group=0 --numeric-owner --mode=0644 \
    -C "$stage" -rf "$archive_tar" \
    "$package/CHANGELOG.md" "$package/LICENSE" "$package/NOTICE" "$package/README.md" \
    "$package/TRADEMARKS.md" "$package/manifest.json"
  tar --sort=name --mtime="@$source_date_epoch" --owner=0 --group=0 --numeric-owner --mode='u+rwX,go+rX,go-w' \
    -C "$stage" -rf "$archive_tar" "$package/packaging" "$package/docs"
  tar --sort=name --mtime="@$source_date_epoch" --owner=0 --group=0 --numeric-owner --mode=0755 \
    -C "$stage" -rf "$archive_tar" "$package/scripts"
  tar --sort=name --mtime="@$source_date_epoch" --owner=0 --group=0 --numeric-owner --mode=0755 \
    -C "$stage" -rf "$archive_tar" "$package/modelcairn"
  gzip -n -9 <"$archive_tar" >"$output_dir/$package.tar.gz"
  rm -rf -- "$stage"
  trap - EXIT
done

(
  cd "$output_dir"
  sha256sum --binary *.tar.gz >SHA256SUMS
)
printf 'release candidate artifacts written to %s\n' "$output_dir"
