# Runtime pins and provenance

Snapshot reviewed 2026-09-26. The installer downloads the CachyOS Proton SLR
runtime and verifies its SHA-256 before extraction. DXVK, vkd3d-proton, and
dxvk-nvapi are supplied by that Proton runtime; Bellum does not install
separate copies into the prefix. The component versions below are upstream
comparison pins from the TES-3 research, not independently downloaded
artifacts or claims that Bellum has been integration-tested against them.

| Component | Pin / handling | Provenance and license |
| --- | --- | --- |
| CachyOS Proton SLR | `proton-cachyos-11.0-20260703-slr-x86_64`; SHA-256 `62ff4b2750180723cc00538608fe687e21d1d91a31ef64ce1a7c9f46c3db310b` | [CachyOS release](https://github.com/CachyOS/proton-cachyos/releases/tag/cachyos-11.0-20260703-slr); upstream Proton/CachyOS source and license notices are in the release. The archive is verified before extraction. |
| DXVK | Integrated with the pinned Proton runtime; upstream comparison target `3.1.1` | [Upstream release](https://github.com/doitsujin/dxvk/releases/tag/v3.1.1); zlib license. The integrated component's exact version is not independently claimed or overlaid. |
| vkd3d-proton | Integrated with the pinned Proton runtime; upstream comparison target `3.0.1` | [Upstream release](https://github.com/HansKristian-Work/vkd3d-proton/releases/tag/v3.0.1); LGPL-2.1-or-later. The integrated component's exact version is not independently claimed or overlaid. |
| dxvk-nvapi | Integrated with the pinned Proton runtime; upstream comparison target `0.9.2` | [Upstream release](https://github.com/jp7677/dxvk-nvapi/releases/tag/v0.9.2); MIT license. The release identifies its source and CI digest. The integrated component's exact version is not independently claimed or overlaid. |
| AstarteLauncher installer | Official updater URL; SHA-256 `2c2d17b724bee70883eae782d2ff9ead2533d2d339fd4ee1b9326c60bb3f064a`; Authenticode signer `ASTARTE INDUSTRIES INC.` | Downloaded from the official Astarte release endpoint; SHA-256, signer name, and signature verification are required before use. |

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
