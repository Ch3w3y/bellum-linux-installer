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

grep -q "tags: \['v\*'\]" .github/workflows/release.yml
grep -q 'Check tag and release gate' .github/workflows/release.yml
echo 'Release regression checks passed'
