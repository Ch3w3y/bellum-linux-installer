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

# The release gate must fail closed for unchecked, placeholder, stale and
# partial evidence, and for code changed after the tested candidate.
gate_script=$PWD/scripts/check-release-gate.sh
repo=$(mktemp -d)
trap 'rm -rf "$repo"; rm -f .release-dirty-probe; rm -rf "dist/bellum-installer-linux-amd64-$version" "dist/bellum-installer-linux-arm64-$version"; rm -f "dist/bellum-installer-linux-amd64-$version.tar.gz" "dist/bellum-installer-linux-arm64-$version.tar.gz"' EXIT
(
  cd "$repo"
  git init -q
  git config user.email test@example.com
  git config user.name test
  echo code > main.go
  printf 'QA: [ ] https://example.invalid/qa\n' > RELEASE-GATE.md
  git add . && git commit -qm candidate
  candidate=$(git rev-parse HEAD)
  gate_passes() { "$gate_script" "$(git rev-parse HEAD)" RELEASE-GATE.md >/dev/null 2>&1; }
  write_gate() {
    printf 'Candidate: %s\n' "$candidate" > RELEASE-GATE.md
    for label in QA EAC Security Provenance; do
      printf '%s: [x] https://github.com/example/blob/v1/docs/release-evidence/%s/%s.md\n' "$label" "$candidate" "$label" >> RELEASE-GATE.md
    done
  }
  write_gate
  mkdir -p "docs/release-evidence/$candidate" && echo record > "docs/release-evidence/$candidate/QA.md"
  git add . && git commit -qm evidence
  "$gate_script" "$(git rev-parse HEAD)" RELEASE-GATE.md

  sed -i 's/QA: \[x\]/QA: [ ]/' RELEASE-GATE.md && git commit -qam unchecked
  if gate_passes; then echo 'Unchecked evidence was accepted' >&2; exit 1; fi
  write_gate && sed -i 's#https://github.com/example#https://example.invalid#' RELEASE-GATE.md && git commit -qam placeholder
  if gate_passes; then echo 'Placeholder evidence was accepted' >&2; exit 1; fi
  write_gate && sed -i "s#release-evidence/$candidate#release-evidence/ffffffffffffffffffffffffffffffffffffffff#" RELEASE-GATE.md && git commit -qam stale
  if gate_passes; then echo 'Stale candidate evidence was accepted' >&2; exit 1; fi
  write_gate && sed -i '/^Candidate:/d' RELEASE-GATE.md && git commit -qam nocandidate
  if gate_passes; then echo 'Missing candidate was accepted' >&2; exit 1; fi
  write_gate && git commit -qam restore && gate_passes
  echo changed > main.go && git commit -qam 'code after candidate'
  if gate_passes; then echo 'Code changed after the candidate was accepted' >&2; exit 1; fi
)
# Publisher refuses a partial or extra asset set.
assets=$(mktemp -d)
trap 'rm -rf "$assets"; rm -rf "$repo"; rm -f .release-dirty-probe; rm -rf "dist/bellum-installer-linux-amd64-$version" "dist/bellum-installer-linux-arm64-$version"; rm -f "dist/bellum-installer-linux-amd64-$version.tar.gz" "dist/bellum-installer-linux-arm64-$version.tar.gz"' EXIT
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
