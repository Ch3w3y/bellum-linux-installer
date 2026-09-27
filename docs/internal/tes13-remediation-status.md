> **Internal process record, not end-user documentation.** This file tracks a
> specific engineering task's remediation history and is written for the
> Paperclip issue tracker/CTO audience. It duplicates and can drift from the
> user-facing statements in `README.md`, `docs/eac-qa.md`, and
> `docs/runtime-pins.md`. See TES-42's `review` document for a recommendation
> on relocating internal-process records like this one out of `docs/`.

# TES-13 remediation status

## 2026-09-26 approved-pin follow-up

- Pinned proton-cachyos `cachyos-11.0-20260703-slr` x86_64 to SHA-256 `62ff4b2750180723cc00538608fe687e21d1d91a31ef64ce1a7c9f46c3db310b`, from [CachyOS's release asset metadata](https://github.com/CachyOS/proton-cachyos/releases/tag/cachyos-11.0-20260703-slr). The earlier CI artifact digest differs from the published release digest; the release asset digest is the one used.
- Bellum's [official download page](https://playbellum.com/download/) redirects through `astarte.launcher.link` to `releases.astarte.industries`. Pinned that current executable to SHA-256 `2c2d17b724bee70883eae782d2ff9ead2533d2d339fd4ee1b9326c60bb3f064a`; its embedded certificate subject is `ASTARTE INDUSTRIES INC.`. The runtime `osslsigncode` verification remains mandatory.
- RDNA4 now sets `PROTON_FSR4_UPGRADE=1` to request Proton's FSR4 driver component by default, with `PROTON_FSR4_RDNA3_UPGRADE=0` intended to keep the RDNA3 upgrade off; per [TES-41 research](/TES/issues/TES-41#document-research) the pinned CachyOS Proton release stages FSR4 on RDNA2–4 independently of these flags and has removed `PROTON_FSR4_RDNA3_UPGRADE` upstream, so this flag pair does not confirm actual upscaler behavior — treat it as unverified until a live QA pass records what upscaler the game actually uses. NVIDIA's DLSS replacement and NGX updater remain off. `BELLUM_MANGOHUD=1`, `BELLUM_VKBASALT=1`, and `BELLUM_GAMESCOPE=1` opt into the permitted overlays.
- Selected the official Steam EAC runtime app `1826330`, depot `1826331`, public manifest `3310269496439035229`, build `10437216` as the reproducible source. SteamCMD anonymous login returned `No subscription`, so no depot files are redistributed. The board-installed entitled Steam client copy at `~/.local/share/Steam/steamapps/common/Proton EasyAntiCheat Runtime` has appmanifest `1826330` / buildid `10437216`. Its six-file manifest digest is pinned as `4d18c3a5b896c757be9e25bf1004b81568bc4d4e56ddd8d1a2a634eebf12d1f9`; per-file SHA-256 and byte counts:
  - `v2/lib32/easyanticheat_x86.dll` (75,080): `7a19573910e775dab97def92d7bea97125534a8c56b1849bf64e0224782125bb`
  - `v2/lib32/easyanticheat_x86.so` (17,244): `c61646ca21bb33d0b738c8d86a540c8bdc7026d7027b069e5e9fcd80a20efc1a`
  - `v2/lib64/easyanticheat.dll` (72,501): `f5e4d53a6bb6a8c7bfc051c3046dad586dd6b7840ca89db8dd4998227e25a8c2`
  - `v2/lib64/easyanticheat.so` (18,392): `66a443682ad1b40c9b5df027953d357ab514430f27f9cf615a2825c16fa420c2`
  - `v2/lib64/easyanticheat_x64.dll` (72,501): `f5e4d53a6bb6a8c7bfc051c3046dad586dd6b7840ca89db8dd4998227e25a8c2`
  - `v2/lib64/easyanticheat_x64.so` (18,392): `66a443682ad1b40c9b5df027953d357ab514430f27f9cf615a2825c16fa420c2`
  Users install through Steam Library → Tools (app `1826330`); for another library set `PROTON_EAC_RUNTIME` to its `steamapps/common/Proton EasyAntiCheat Runtime` path. Installer integrity checks fail closed for absent or changed files. The runtime remains outside the game directory and installer payload.
- Focused verification: `GOCACHE=<task-local writable cache> mise exec go@1.24.9 -- go test ./pkg/packages ./pkg/workflow ./pkg/launchers` passed. `git diff --check` passed. Static integrity checks do not prove EAC Linux-mode success; live protected-session evidence remains with TES-12.

## Implemented in the shared workspace

- Removed `packages/fsr4/` and the `--fsr41` CLI option. Removed game-directory DLL copies and added a write boundary plus regression test.
- Generated wrappers for AMD, NVIDIA, and Intel use `umu-run` with the selected Proton and require `PROTON_EAC_RUNTIME` to exist. They write the launch log in the prefix.
- RDNA4 requests Proton's FSR4 driver component by default (`PROTON_FSR4_UPGRADE=1`) under the approved developer guidance; the RDNA3 upgrade flag and DLSS replacement upgrades remain off. Whether the pinned Proton runtime actually leaves shipped upscalers alone on RDNA3 is unconfirmed — see the FSR-policy note above.
- Vendored winetricks is hash checked and installed to `~/.local/bin` without sudo or self-update. The uninstaller requires a Bellum prefix with Wine markers, refuses root/home/symlink paths, and defaults its destructive prompt to No.
- Installer logs use a private directory and `O_NOFOLLOW`; tar entries and link targets are checked before extraction. The user launcher is installed in `~/.local/bin`; desktop permissions are `0644`.
- SHA-256 pins are implemented for vendored artifacts. Proton and launcher downloads require release pins. Launcher verification also requires an approved Authenticode signer and a successful `osslsigncode verify`.
- `go test ./...` and `git diff --check` pass. See `docs/eac-qa.md` for the Linux-mode log check.
- A follow-up fixed literal `\\n` escapes in the Intel/generic `launch_vars.env` writer. Targeted `go test ./pkg/workflow ./pkg/launchers ./pkg/packages` passes with a writable Go cache.
- The WebView2 follow-up adds a nonblocking prefix `flock` for second clicks, keeps umu's session-wait verb, verifies the installed WebView2 runtime after launcher setup, and restricts the prefix, launch variables, and launcher log to private permissions. NVIDIA's NGX updater defaults off; the capability-gated FSR runtime switch uses `1` instead of the removed `4.1.0` path. The QA checklist now covers launcher lifecycle and evidence redaction. Focused package tests and `git diff --check` pass.

## Verification and release handoff

- On 2026-09-26, the installed entitled Steam runtime's six-file digest was independently recomputed as `4d18c3a5b896c757be9e25bf1004b81568bc4d4e56ddd8d1a2a634eebf12d1f9`, matching the configured pin. The installer requires that exact content before installation; missing or altered runtime files fail closed.
- `GOCACHE=<task-local writable cache> mise exec go@1.24.9 -- go test ./...` and `git diff --check` pass in the shared checkout. These checks cover the game-tree write guard, package integrity, generated wrappers, and uninstall safety.
- [TES-12](/TES/issues/TES-12) owns live EAC Linux-mode evidence and launcher lifecycle QA. The procedure is in [the EAC QA checklist](../eac-qa.md). Static tests and runtime hashing do not establish that a protected Bellum session succeeds.
- Verify `osslsigncode` against the pinned official launcher during release QA. Its required signer and SHA-256 gates fail closed in the installer.
