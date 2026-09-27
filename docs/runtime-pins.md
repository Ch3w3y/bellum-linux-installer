# Runtime pins and provenance

The installer downloads CachyOS Proton SLR and checks the complete archive against the SHA-256 pin in `pkg/config/versions.go` before extraction. Proton installs into a sibling staging directory, receives the Bellum settings patch, and is atomically renamed into place. A content stamp detects partial or modified trees before reuse. The old cwd-relative `./packages/proton-*` cache is not used. Installer and uninstaller refuse uid 0.

| Runtime | Pin and source |
| --- | --- |
| CachyOS Proton SLR | `proton-cachyos-11.0-20260703-slr-x86_64`, SHA-256 `62ff4b2750180723cc00538608fe687e21d1d91a31ef64ce1a7c9f46c3db310b`; [CachyOS release](https://github.com/CachyOS/proton-cachyos/releases/tag/cachyos-11.0-20260703-slr). |
| Proton EasyAntiCheat Runtime | Steam app 1826330 / depot 1826331, manifest 3310269496439035229; approved combined file digests are an allowlist in `pkg/config/versions.go`. Users install it through Steam; Bellum does not redistribute depot files. |
| Astarte Launcher | SHA-256 allowlist and exactly one case-sensitive leaf signer CN are required in `pkg/config/versions.go`; subject parsing accepts both legacy OpenSSL slash format and RFC 2253. Signature verification requires `osslsigncode`. |
| winetricks | Upstream tag `20250102` from [Winetricks/winetricks](https://github.com/Winetricks/winetricks/tree/20250102); the vendored archive is separately pinned in `pkg/packages/versions.go`. Upstream `COPYING` is shipped at the repository root. |

The SHA-256 pins are content pins for the exact downloaded files. A missing or
mismatched pin fails closed. The vendored legacy DXVK archive has been removed
from the release payload; its old installer helper is not part of the runtime
setup path. Runtime
availability does not establish Bellum EAC compatibility; the game-directory
integrity boundary remains enforced separately.

## Effective FSR behavior in the pinned runtime

The pinned Proton-CachyOS tag [cachyos-11.0-20260703-slr](https://github.com/CachyOS/proton-cachyos/releases/tag/cachyos-11.0-20260703-slr)
is based on the 2026-07-02 runtime changes that automatically copy
`amdxcffx64.dll` for supported discrete RDNA2-RDNA4 GPUs. The release notes say
`PROTON_FSR4_UPGRADE` is no longer required for those GPUs and remove
`PROTON_FSR4_RDNA3_UPGRADE`. They do not document a switch that disables the
automatic RDNA3 staging. The tag's [upscaler source patch](https://github.com/CachyOS/proton-cachyos/blob/cachyos-11.0-20260703-slr/patches/protonfixes/0002-upscalers/0004-upscalers-update-handling-for-FSR4-4.1.1.patch)
marks FSR4 DLL setup enabled unconditionally, but only exports the FSR upgrade
activation when the compatibility configuration requests `fsr3` or `fsr4`.
Thus auto-staging is not the same as forcing every game's upscaler, and an
environment variable alone does not give Bellum a deterministic RDNA3 disable.
The launcher's `PROTON_FSR4_UPGRADE=1` preference is exported only for an
unambiguous RDNA4 renderer. The source patch also gates the effective
`FSR4_UPGRADE` activation on Proton's `fsr3`/`fsr4` compatibility config; Bellum
does not claim that this request forces the game's upscaler path on. This also
does not guarantee RDNA3 native FSR paths are untouched.
The runtime's MLFG option is separate from FSR upscaling and is not enabled by
Bellum.
The bundled winetricks Makefile was compared with upstream tag `20250102`. Its only changes are: add `uninstall` to the `all` target's help text and add an `uninstall` target that removes the installed executable, man page, desktop entry, metainfo, icon, and bash completion. The version label `20250102-modified` records that delta. To update: fetch a tagged upstream source archive, review its license and Makefile, apply only reviewed local changes, update the archive SHA-256 and version together, then run the package integrity and installer regression checks.

DXVK, vkd3d-proton, and dxvk-nvapi are supplied by the pinned Proton archive; Bellum does not overlay separate versions. The archive hash is the reproducible content pin.
