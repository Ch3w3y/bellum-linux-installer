# Runtime pins and provenance

The installer downloads CachyOS Proton SLR and checks the complete archive against the SHA-256 pin in `pkg/config/versions.go` before extraction. Proton installs into a sibling staging directory, receives the Bellum settings patch, and is atomically renamed into place. A content stamp detects partial or modified trees before reuse. The old cwd-relative `./packages/proton-*` cache is not used. Installer and uninstaller refuse uid 0.

| Runtime | Pin and source |
| --- | --- |
| CachyOS Proton SLR | `proton-cachyos-11.0-20260703-slr-x86_64`, SHA-256 `62ff4b2750180723cc00538608fe687e21d1d91a31ef64ce1a7c9f46c3db310b`; [CachyOS release](https://github.com/CachyOS/proton-cachyos/releases/tag/cachyos-11.0-20260703-slr). |
| Proton EasyAntiCheat Runtime | Steam app 1826330 / depot 1826331, manifest 3310269496439035229; approved combined file digests are an allowlist in `pkg/config/versions.go`. Users install it through Steam; Bellum does not redistribute depot files. |
| Astarte Launcher | SHA-256 allowlist and exactly one case-sensitive leaf signer CN are required in `pkg/config/versions.go`; subject parsing accepts both legacy OpenSSL slash format and RFC 2253. Signature verification requires `osslsigncode`. |
| winetricks | Upstream tag `20250102` from [Winetricks/winetricks](https://github.com/Winetricks/winetricks/tree/20250102); the vendored archive is separately pinned in `pkg/packages/versions.go`. Upstream `COPYING` is shipped at the repository root. |

The bundled winetricks Makefile was compared with upstream tag `20250102`. Its only changes are: add `uninstall` to the `all` target's help text and add an `uninstall` target that removes the installed executable, man page, desktop entry, metainfo, icon, and bash completion. The version label `20250102-modified` records that delta. To update: fetch a tagged upstream source archive, review its license and Makefile, apply only reviewed local changes, update the archive SHA-256 and version together, then run the package integrity and installer regression checks.

DXVK, vkd3d-proton, and dxvk-nvapi are supplied by the pinned Proton archive; Bellum does not overlay separate versions. The archive hash is the reproducible content pin.
