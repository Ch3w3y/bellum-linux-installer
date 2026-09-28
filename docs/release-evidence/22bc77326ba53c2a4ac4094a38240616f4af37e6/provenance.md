# Provenance: v2.3.0 (candidate v2.3.0-rc.1)

- **Candidate:** `v2.3.0-rc.1`, commit `22bc77326ba53c2a4ac4094a38240616f4af37e6`
- **Checked:** 2026-09-28

## Runtime pins (`pkg/config/versions.go`)

**Unchanged since v2.2.0** (see `docs/release-evidence/59ae4837bfde5a72e010741cc8792e6b08993f4a/provenance.md`):

| Component | Pin |
| --- | --- |
| Proton-CachyOS SLR | `proton-cachyos-11.0-20260703-slr-x86_64`, SHA-256 `62ff4b2750180723cc00538608fe687e21d1d91a31ef64ce1a7c9f46c3db310b` |
| umu-launcher | `1.4.4` zipapp, SHA-256 `eb590691841f7fad3fc3ad8fd5db4ccb87849fe7948e62b28ece7a4ee48cc851` |
| winetricks, DXVK, vkd3d-proton, dxvk-nvapi | inside the Proton archive |

On 2026-09-28 the component versions inside the pinned Proton archive were read from the release URL:
- DXVK `v3.0.2-2-g0ff9cd3`;
- vkd3d-proton `vkd3d-1.1-5438-g3dfc6f0`;
- dxvk-nvapi `v0.9.2-70-gffb351d`.

The NVIDIA minimum driver check (575.51.02) is DXVK 3.x's documented minimum. `workflow.ProtonContract()` now requires `dxvk (v3.` in `files/lib/wine/dxvk/version`, so the automated pin update cannot move to another DXVK major unreviewed.

Signed and vendor-updated components (Astarte Launcher signer `ASTARTE INDUSTRIES INC.`, the pinned GlobalSign Code Signing Root R45, the Steam-installed EAC runtime) are unchanged since v2.2.0.

## The candidate build

- Built by [release run 36441431798](https://github.com/Ch3w3y/bellum-linux-installer/actions/runs/36441431798) from commit `22bc77326ba53c2a4ac4094a38240616f4af37e6` with Go 1.26.8, `-trimpath -mod=readonly`, version `2.3.0-rc.1`.
- The only module dependency is still `github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8`, verified by `go mod verify`.

| Archive | SHA-256 | Attestation |
| --- | --- | --- |
| `bellum-installer-linux-amd64-2.3.0-rc.1.tar.gz` | `d9c79a8a2872b51576c64bd8c7141b7569fddb52a2f66d6cb206d4037bfd49ab` | [50811442](https://github.com/Ch3w3y/bellum-linux-installer/attestations/50811442), Rekor index 2983743797 |
| `bellum-installer-linux-arm64-2.3.0-rc.1.tar.gz` | `c01ea307c9b5cf62c7282caf797d4c37322da976270c1c5d9cbe3f5303e74ab0` | [50811561](https://github.com/Ch3w3y/bellum-linux-installer/attestations/50811561), Rekor index 2983747210 |

**Independent reproduction.** A separate clean build of the candidate outside GitHub Actions (`make release VERSION=2.3.0-rc.1`, Go 1.26.8, amd64 and `GOARCH=arm64`) produced both archives with exactly the digests GitHub attested. So the published binaries correspond to the reviewed source.

## The final release

The final `v2.3.0` is built from a commit that differs from this candidate only in `RELEASE-GATE.md` and `docs/release-evidence/`; the gate enforces that. Its archives carry the version `2.3.0`, so their digests differ from the candidate's, and they get their own attestations.
