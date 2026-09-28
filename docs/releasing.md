# Releasing

How a release is built, tested on real hardware, gated and published. The
installer is never run in CI; it is tested by hand through release candidates.

## Overview

1. **Merge to `main`** with CI green (gofmt, vet, tests, module pinning,
   builds, `install.sh` shellcheck and dry run).
2. **Cut a release candidate** `vX.Y.Z-rc.N` (see [Starting a
   release](#starting-a-release)). The workflow builds both architectures,
   attests them and publishes a GitHub **pre-release**. Candidates skip the
   evidence gate because they exist to gather it.
3. **Test the candidate on real hardware** with the one-line installer:

   ```bash
   curl -fsSL https://raw.githubusercontent.com/Ch3w3y/bellum-linux-installer/main/install.sh | bash -s -- --version vX.Y.Z-rc.N
   ```

   Follow the [QA and EAC checklist](eac-qa.md). Any fix means a new
   candidate.
4. **Record the evidence** for the tested candidate commit in
   [`docs/release-gate.md`](release-gate.md) and `docs/release-evidence/<candidate
   SHA>/` (QA, EAC, security review, provenance of the pins), in one commit on
   `main` that changes nothing else.
5. **Cut the final release** `vX.Y.Z`. The workflow runs the gate, builds,
   attests and waits for approval on the `release` environment, then
   publishes a **draft** release.
6. **Review and publish the draft.** The one-line installer only picks up
   published, non-pre-release versions (GitHub's "latest release").

## The evidence gate

`scripts/check-release-gate.sh` runs for every final release and fails
closed. It requires:

- a `Candidate:` line naming the full SHA of the tested release candidate;
- that candidate to be an ancestor of the release commit, with **nothing but
  `docs/release-gate.md` and `docs/release-evidence/` changed since**, so the
  released code is exactly the code that was tested;
- all four items (QA, EAC, Security, Provenance) checked, each with an HTTPS
  link that contains the candidate SHA and isn't a placeholder.

A link is not an approval. The reviewer on the `release` environment must
read each record before approving. Configure **Settings → Environments →
release → Required reviewers**; don't publish a final release without it.

`scripts/test-release.sh` covers the gate (valid, unchecked, placeholder,
stale and missing-candidate evidence, and code changed after the candidate),
the dirty-tree check, archive tampering and the asset-set check.

## Starting a release

From the Actions tab: **Tagged release → Run workflow**, enter the tag (for
example `v2.2.0-rc.6` or `v2.2.0`). This creates the tag at the head of the
chosen branch. Pushing a `v*` tag does the same.

The workflow checks that the tag matches, the tree is clean and the gate
passes (final releases only). It then runs `make check` and `govulncheck`,
builds `amd64` and `arm64` with `make release`, verifies each archive,
creates GitHub build-provenance attestations, and publishes once the asset
set is complete.

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

Keep the output, the archive digests, the workflow run, tag and source commit
with the release review record.

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
