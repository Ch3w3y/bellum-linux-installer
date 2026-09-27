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
| winetricks | Bundled in the pinned Proton at `protonfixes/winetricks` (`20260125-next`, SHA-256 `58778c4f0c6fccfd66b0f8abfff4fd4d27b25536f35d699128821a572d29dddb`); run as `umu-run winetricks` | [Winetricks](https://github.com/Winetricks/winetricks), LGPL-2.1-or-later, shipped by the Proton archive. Covered by the Proton archive hash. Bellum no longer vendors its own copy or needs system Wine. After a Proton re-pin, run `BELLUM_PROTON_DIR=<extracted tree> go test ./pkg/workflow -run Verbs` to confirm every verb still exists. |
| AstarteLauncher installer | Official updater URL; SHA-256 `2c2d17b724bee70883eae782d2ff9ead2533d2d339fd4ee1b9326c60bb3f064a`; Authenticode signer `ASTARTE INDUSTRIES INC.` | Downloaded from the official Astarte release endpoint; SHA-256, signer name, and signature verification are required before use. |

The SHA-256 pins are content pins for the exact downloaded files. A missing or
mismatched pin fails closed. Proton is extracted into a sibling staging
directory, patched, stamped with a content digest and atomically renamed into
place; a partial or modified tree is re-downloaded rather than reused. The
launcher is downloaded into a private temporary directory. Installer and
uninstaller refuse to run as root. The vendored legacy DXVK archive has been removed
from the release payload; its old installer helper is not part of the runtime
setup path. The game-directory integrity boundary is enforced separately.

**Pin durability.** Fail closed only on things this project ships or downloads
from a versioned URL; accept vendor updates to things the user installs.

| Artifact | Hard control (fails closed) | Allowlist (warns only) |
| --- | --- | --- |
| Proton | Versioned URL + SHA-256 pin | none |
| Astarte Launcher installer | Authenticode signature from `ASTARTE INDUSTRIES INC.` (the URL is not versioned and always serves the current build) | `LauncherSHA256Allowlist` |
| Proton EasyAntiCheat Runtime | All six runtime files present; installed by Steam as app `1826330` (found through `appmanifest_1826330.acf`) or set explicitly with `PROTON_EAC_RUNTIME` | `EACRuntimeSHA256Allowlist` |

An unknown launcher or EAC digest is logged with the digest value, so the
allowlists can be extended from user logs.

## How to refresh a pin

Run this monthly, and whenever a Proton-CachyOS release notes EAC, driver or
upscaler fixes (CachyOS releases often).

1. **Proton.** Check [CachyOS releases](https://github.com/CachyOS/proton-cachyos/releases)
   for a newer `-slr` build. Download the `x86_64` archive and compare its
   SHA-256 with the release asset's published digest. Update `ProtonVer` and
   `ProtonSHA256` in `pkg/config/versions.go` together.
2. Extract the archive and run
   `BELLUM_PROTON_DIR=<extracted tree> go test ./pkg/workflow -run Verbs` to
   confirm that the bundled winetricks still has every verb. Note its
   `WINETRICKS_VERSION` in `WinetricksVer` and in the table above.
3. Re-read the new tag's `protonfixes` upscaler patches and update
   [Upscaler behaviour](#upscaler-behaviour-of-the-pinned-proton).
4. **Launcher.** Download `AstarteLauncher-amd64-installer.exe`, run
   `osslsigncode verify -in <file>`, confirm the leaf signer, and append its
   SHA-256 to `LauncherSHA256Allowlist` (keep the older entries).
5. **EAC runtime.** After Steam updates app `1826330`, compute the digest with
   `packages.EACRuntimeDigest`, or take it from an installer-log warning, run the
   [EAC checklist](eac-qa.md), and append the digest to
   `EACRuntimeSHA256Allowlist`.
6. Update the snapshot date and table in this file, run `make check`, and open
   a PR.

**Newer Proton-CachyOS builds.** As of 2026-09-27, `cachyos-11.0-20260703-slr`
(published 2026-07-22) is still the latest tagged release. Any re-pin must
re-check the upscaler behaviour described below.

## Upscaler behaviour of the pinned Proton

Board policy (#11): **the installer never adds, copies, or replaces DLLs.**
Upscaler features built into the pinned Proton runtime are accepted as Proton
behaviour.

- Since [`cachyos-11.0-20260702-slr`](https://github.com/CachyOS/proton-cachyos/releases/tag/cachyos-11.0-20260702-slr),
  Proton-CachyOS copies AMD's `amdxcffx64.dll` FSR4 driver component
  automatically on supported RDNA2–RDNA4 discrete GPUs. The release notes say
  `PROTON_FSR4_UPGRADE` is no longer needed but can still select a DLL version,
  and that `PROTON_FSR4_RDNA3_UPGRADE` was removed.
- In the pinned tag's protonfixes patch
  [`0004-upscalers-update-handling-for-FSR4-4.1.1.patch`](https://github.com/CachyOS/proton-cachyos/blob/cachyos-11.0-20260703-slr/patches/protonfixes/0002-upscalers/0004-upscalers-update-handling-for-FSR4-4.1.1.patch),
  the FSR4 entry is unconditionally enabled (`('fsr4', fsr4_version, True)`).
  The DLL is staged under `drive_c/windows/system32/umu/` and registered through
  `WINE_UPSCALER_REPLACE`. `get_version()` treats `0` and `1` identically, so
  no environment value disables it.
- The installer exports `PROTON_FSR4_UPGRADE=1` only for an unambiguous RDNA4
  GPU (`launch_vars.env`). It exports `PROTON_DLSS_UPGRADE=0` for NVIDIA and
  generic GPUs and `PROTON_ENABLE_NGX_UPDATER=0` for NVIDIA, and it never
  requests DLSS/XeSS DLL upgrades. MLFG is not enabled.
