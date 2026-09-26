# Release process — Bellum Linux Installer

This document describes how to build, verify, and publish a release, and the
binary/provenance policy that applies to every artifact.

## Release gate (hard requirements before tagging an RC)

A release candidate must NOT be tagged until all of the following are complete:

1. **QA evidence** — [TES-12](https://github.com/Ch3w3y/bellum-linux-installer)
   records the production release matrix and remaining limitations.
2. **EAC gate** — per the TES-13 scope update: game-directory-write removal,
   the common umu/Proton EAC path, and all Security high/medium findings fixed.
3. **CI green** — the `CI` workflow (gofmt, go vet, go test, module pinning,
   compile-only builds for amd64/arm64) passes on the release commit.
4. **Release verification** — `make release` produces the tarball, `MANIFEST.md`,
   and `SHA256SUMS`; `make verify-release` passes; rebuilding from a clean tree
   yields a byte-identical tarball (see "Reproducibility" below).
5. **Pinned artifact provenance/checksum checks** — every bundled package
   (DXVK, winetricks, FSR4 DLLs, launcher icon) is a pinned, named version.
   Its sha256 in `MANIFEST.md`/`SHA256SUMS` is checked against the pinned
   provenance table in `pkg/config/versions.go` (and the upstream source the
   artifact came from). New or replaced packages require an updated pin and a
   fresh checksum entry — never an unverified re-download.

## Building a release locally

```bash
# 1. Run the same checks CI runs
make check

# 2. Build the reproducible release (override VERSION as needed)
make release VERSION=2.1.0

# 3. Verify checksums of the staged artifacts
make verify-release
```

Outputs (in `dist/`):

- `bellum-installer-linux-amd64-<VERSION>.tar.gz` — the release archive
- `dist/bellum-installer-linux-amd64-<VERSION>/MANIFEST.md` — versioned
  manifest: version, build metadata, per-file sha256 + size, provenance notes
- `dist/bellum-installer-linux-amd64-<VERSION>/SHA256SUMS` — checksums for the
  staged files (excluding itself)

The archive contains `installer`, `uninstaller`, `MANIFEST.md`, `SHA256SUMS`,
and `packages/`.

## Reproducibility

`make release` is deterministic for the same source tree and `VERSION`:

- Go builds use `-trimpath -mod=readonly` and fixed ldflags; no timestamps or
  paths are embedded.
- `go mod tidy` is never run as a build side effect. `go.mod`/`go.sum` are
  pinned by hand; `go mod tidy -diff` (in `make check`) fails the build if
  they drift.
- The tarball is packed with fixed metadata: `--owner=0 --group=0
  --numeric-owner`, `--mtime` from `SOURCE_DATE_EPOCH` (default
  `946684800`, i.e. 2000-01-01T00:00:00Z), `--sort=name`, and a fixed mode.
- `LC_ALL=C` and a stable `find | sort` order make file lists deterministic.

Verification: run `make release VERSION=<v>` twice from a clean tree; the
tarball sha256 must match. CI and the release checklist treat any mismatch as
a release blocker.

## Binary / provenance policy (visible summary)

- **Binaries** are built by `make release` from the exact commit recorded in
  `MANIFEST.md` (`commit:` field) with the Go toolchain version recorded
  alongside. Prebuilt binaries are never committed to the repository.
- **Bundled packages** in `packages/` are pinned versions of upstream
  artifacts (proton-cachyos, DXVK, winetricks-modified, FSR4 DLLs). Their
  sha256 is recorded in the release manifest; provenance/pinning policy and
  version pins live in `pkg/config/versions.go`. A checksum mismatch on any
  bundled package fails release verification.
- **The installer and uninstaller are never executed by CI, the Makefile, or
  release tooling.** They touch Wine prefixes, download runtime artifacts,
  and mutate the host; that is exercised only by the manual QA matrix in
  TES-12. CI and `make release` are compile/package-only.
- **No RC without the gate**: tagging is blocked on the checklist above —
  QA evidence, EAC fixes, green CI, reproducibility, and provenance checks.

## Publishing

1. Confirm every release-gate item above is checked on the release commit.
2. `make release VERSION=<final-version> && make verify-release`
3. Upload the tarball to the GitHub release, attach `MANIFEST.md` and
   `SHA256SUMS`, and quote the tarball sha256 in the release notes.
4. Tag only after steps 1–3 are done and documented.