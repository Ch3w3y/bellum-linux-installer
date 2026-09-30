#!/usr/bin/env bash
set -euo pipefail

version=9.9.9-test
trap 'rm -f .release-dirty-probe; rm -rf "dist/bellum-installer-linux-amd64-$version" "dist/bellum-installer-linux-arm64-$version"; rm -f "dist/bellum-installer-linux-amd64-$version.tar.gz" "dist/bellum-installer-linux-arm64-$version.tar.gz"' EXIT

touch .release-dirty-probe
if make release VERSION="$version" > /dev/null 2>&1; then
  echo 'Dirty tree was accepted' >&2
  exit 1
fi
rm .release-dirty-probe

make release VERSION="$version" GOARCH=arm64
test -f "dist/bellum-installer-linux-arm64-$version.tar.gz"
file "dist/bellum-installer-linux-arm64-$version/installer" | grep -q 'ARM aarch64'
make verify-release VERSION="$version" GOARCH=arm64

truncate -s -100 "dist/bellum-installer-linux-arm64-$version.tar.gz"
if make verify-release VERSION="$version" GOARCH=arm64 > /dev/null 2>&1; then
  echo 'Tampered archive was accepted' >&2
  exit 1
fi

# Publisher refuses a partial or extra asset set.
assets=$(mktemp -d)
trap 'rm -rf "$assets"; rm -f .release-dirty-probe; rm -rf "dist/bellum-installer-linux-amd64-$version" "dist/bellum-installer-linux-arm64-$version"; rm -f "dist/bellum-installer-linux-amd64-$version.tar.gz" "dist/bellum-installer-linux-arm64-$version.tar.gz"' EXIT
printf 'amd64' > "$assets/bellum-installer-linux-amd64-$version.tar.gz"
if scripts/check-release-assets.sh "$version" "$assets" >/dev/null 2>&1; then
  echo 'Partial release was accepted' >&2; exit 1
fi
printf 'arm64' > "$assets/bellum-installer-linux-arm64-$version.tar.gz"
scripts/check-release-assets.sh "$version" "$assets"
printf 'unexpected' > "$assets/extra.tar.gz"
if scripts/check-release-assets.sh "$version" "$assets" >/dev/null 2>&1; then
  echo 'Extra release asset was accepted' >&2; exit 1
fi
echo 'Release regression checks passed'
