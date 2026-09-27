# Provenance: v2.2.0 (candidate v2.2.0-rc.7)

- **Candidate:** `v2.2.0-rc.7`, commit `59ae4837bfde5a72e010741cc8792e6b08993f4a`
- **Checked:** 2026-09-27

## Runtime pins (`pkg/config/versions.go`)

| Component | Pin | Independent check |
| --- | --- | --- |
| Proton-CachyOS SLR | `proton-cachyos-11.0-20260703-slr-x86_64` from [CachyOS/proton-cachyos](https://github.com/CachyOS/proton-cachyos/releases/tag/cachyos-11.0-20260703-slr), SHA-256 `62ff4b2750180723cc00538608fe687e21d1d91a31ef64ce1a7c9f46c3db310b` | Downloaded from the release URL on 2026-09-27; `sha256sum` matches. It matches GitHub's published asset digest (the daily `update-pins` check), unpacks with all 2,068 links resolving (#29), and passes `workflow.ProtonContract()`. |
| winetricks | the copy inside that Proton archive (`20260125-next`) | covered by the Proton digest |
| umu-launcher | `1.4.4` zipapp from [Open-Wine-Components/umu-launcher](https://github.com/Open-Wine-Components/umu-launcher/releases/tag/1.4.4), SHA-256 `eb590691841f7fad3fc3ad8fd5db4ccb87849fe7948e62b28ece7a4ee48cc851` | Downloaded from the release URL on 2026-09-27; `sha256sum` matches. |
| DXVK, vkd3d-proton, dxvk-nvapi | integrated in the Proton archive | covered by the Proton digest |

## Signed and vendor-updated components

| Component | Control | Check |
| --- | --- | --- |
| Astarte Launcher installer | Authenticode signer `ASTARTE INDUSTRIES INC.`; known digest `2c2d17b7…f064a` (allowlist, warn-only) | verified on hardware installs of rc.3 and rc.4 |
| Astarte Launcher 1.4.2 (update) | same signer; official per-version URL from `RELEASES` | the probe on GitHub Actions (2026-09-27) verified the 64,740,472-byte exe's signature. On hardware, installing it ended the update loop. |
| GlobalSign Code Signing Root R45 | embedded, fingerprint `7b9d553e1c92cb6e8803e137f4f287d4363757f5d44b37d52f9fca22fb97df86` | test `pinnedRoots` fails on a mismatch |
| Proton EasyAntiCheat Runtime | installed by Steam (app 1826330); all six files required; digest allowlist `4d18c3a5…12d1f9` (build 10437216), warn-only | never redistributed |

## The release itself

- Built by [release run 36332615564](https://github.com/Ch3w3y/bellum-linux-installer/actions/runs/36332615564) from commit `59ae4837bfde5a72e010741cc8792e6b08993f4a` with Go 1.26.8, `-trimpath -mod=readonly`.
- The only module dependency is `github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8` (public domain), verified by `go mod verify`.
- amd64 archive digest: `sha256:d1307f625bb0bf42fb5ab4579d7e74e68505a45a41c96e85e9731f61390a0510`.
- Build-provenance attestation: [attestation 50571172](https://github.com/Ch3w3y/bellum-linux-installer/attestations/50571172), in the Sigstore Rekor log at index 2977471121.
- Only `RELEASE-GATE.md` and `docs/release-evidence/` may differ between this candidate and the final release; the gate enforces that.
