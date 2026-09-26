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
