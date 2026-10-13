# Releasing

How a release is built, tested and published. The installer is never run in
CI; it is tested by hand on real hardware.

## Overview

1. **Merge to `main`.** After the push's full CI run passes, the **Release**
   workflow automatically selects the next patch tag, for example
   `v2.4.0` → `v2.4.1`. This applies to all main updates, including runtime
   pins and Go security updates; direct pushes to main follow the same rule.
2. **Build and verify.** The workflow tags the exact commit checked by CI,
   runs code and vulnerability checks again, builds both architectures,
   verifies archives and attests their provenance.
3. **Publish automatically.** After both builds and the complete asset-set
   check pass, the patch becomes a published GitHub release marked **latest**.
   The one-line installer then picks it up. Automatic patches use the
   `automatic-release` environment without the manual approval gate.

Automatic versioning increments only the patch component of the highest
stable `vX.Y.Z` tag. It ignores release candidates and unrelated tags, and
never chooses a new major or minor version. A manually created stable major
or minor tag becomes the baseline for subsequent automatic patches. No
version-bump commit is needed: the tag's version is stamped into both binaries
and the manifest; the Makefile's default version is only for local builds.

Release runs are serialized to avoid allocating the same version twice.
Superseded main commits are skipped; GitHub may coalesce pending runs when
merges arrive quickly, so a release can include several merges. A failed CI
run never starts an automatic release. A failed build may leave its tag in
place: rerun the failed release workflow to reuse it. Rerunning CI on an
already released commit does not create another version or replace assets.

## Manual major/minor releases and candidates

From the Actions tab: **Release → Run workflow**, enter an explicit tag such
as `v3.0.0` or `v3.0.0-rc.1` and choose the branch to release. Pushing a `v*`
tag also starts this path. Use a development branch to cut candidates before
merging changes to `main`, since main merges automatically publish patches.

Manual releases retain the `release` environment approval gate. Stable tags
create **drafts** for review and publication; `-rc.N` tags publish as
**pre-releases**, which the installer only uses when explicitly requested:

```bash
curl -fsSL https://raw.githubusercontent.com/Ch3w3y/bellum-linux-installer/main/install.sh | bash -s -- --version v3.0.0-rc.1
```

The [QA and EAC checklist](eac-qa.md) lists what to try on hardware.
`scripts/test-release.sh` covers the dirty-tree check, archive tampering and
the asset-set check. `python3 scripts/test-next-release-tag.py` tests automatic
version selection, including retries, candidate filtering and manual majors.

## Building locally

```bash
make check                        # what CI runs
make release VERSION=2.2.0        # needs a clean tree; GOARCH=arm64 for ARM
make verify-release               # re-check the last staged release
scripts/test-release.sh           # release tooling regression tests
```

Outputs in `dist/`:

- `bellum-installer-linux-<arch>-<version>.tar.gz`, containing `installer`,
  `uninstaller`, `MANIFEST.md` (version, commit, Go toolchain, per-file SHA-256
  and sizes), `SHA256SUMS` and the tracked files in `packages/`;
- `SHA256SUMS` for the archives.

`config.InstallerVersion` is stamped into the binaries with `-ldflags -X`.
`make release` refuses a dirty tree, and `make verify-release` fails if the
archive differs from the staged files.

## Reproducibility

`make release` is deterministic for the same source tree, `VERSION` and Go
toolchain:

- builds use `-trimpath -mod=readonly` and fixed ldflags, so no timestamps or
  local paths are embedded;
- `go.mod` and `go.sum` are maintained by hand; `make check` fails if
  `go mod tidy -diff` finds drift;
- the tarball is packed with fixed owner, group, mode and order, and an mtime
  from `SOURCE_DATE_EPOCH` (default 2000-01-01);
- `LC_ALL=C` and sorted file lists keep ordering stable.

Building twice from a clean tree must give the same tarball SHA-256; a
mismatch blocks the release.

## Verifying a published release

```bash
v=2.2.0
base=https://github.com/Ch3w3y/bellum-linux-installer/releases/download/v$v
curl -fLO "$base/bellum-installer-linux-amd64-$v.tar.gz" -fLO "$base/bellum-installer-linux-arm64-$v.tar.gz" -fLO "$base/SHA256SUMS"
sha256sum --check --strict SHA256SUMS
gh attestation verify "bellum-installer-linux-amd64-$v.tar.gz" --repo Ch3w3y/bellum-linux-installer
gh attestation verify "bellum-installer-linux-arm64-$v.tar.gz" --repo Ch3w3y/bellum-linux-installer
```


## Provenance policy

- Binaries are built only by `make release` from the commit recorded in
  `MANIFEST.md`. Prebuilt binaries are never committed.
- The only bundled file is the launcher icon in `packages/`, recorded in the
  manifest.
- Proton-CachyOS and umu-launcher are downloaded at install time and pinned by
  SHA-256 in `pkg/config/versions.go`. Pins are proposed by the daily
  [`update-pins`](../.github/workflows/update-pins.yml) workflow only after the
  checks in [runtime pins](runtime-pins.md#automated-pin-updates) pass.
- DXVK, vkd3d-proton, dxvk-nvapi and winetricks come from the pinned Proton
  archive, never from separate downloads.
- The Astarte Launcher is accepted on its Authenticode signature from
  `ASTARTE INDUSTRIES INC.`; known build digests are recorded, and an unknown
  one only warns.
- Any checksum or signature failure stops the install.
