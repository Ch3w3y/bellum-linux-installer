#!/usr/bin/env bash
set -euo pipefail
version=${1:?release version required}
asset_dir=${2:-dist}
[[ $version =~ ^[0-9A-Za-z][0-9A-Za-z._-]*$ ]] || { echo 'Invalid version' >&2; exit 1; }
for arch in amd64 arm64; do
  test -s "$asset_dir/bellum-installer-linux-${arch}-${version}.tar.gz" || {
    echo "Missing ${arch} archive" >&2; exit 1;
  }
done
# Only the expected pair is accepted; stale files must not enter a release.
test "$(find "$asset_dir" -maxdepth 1 -name '*.tar.gz' | wc -l)" -eq 2 || {
  echo 'Unexpected archive count' >&2; exit 1;
}
(cd "$asset_dir" && sha256sum "bellum-installer-linux-amd64-${version}.tar.gz" "bellum-installer-linux-arm64-${version}.tar.gz" > SHA256SUMS && sha256sum --check --strict SHA256SUMS)
