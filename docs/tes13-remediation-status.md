# TES-13 remediation status

## 2026-09-26 approved-pin follow-up

- Pinned proton-cachyos `cachyos-11.0-20260703-slr` x86_64 to SHA-256 `62ff4b2750180723cc00538608fe687e21d1d91a31ef64ce1a7c9f46c3db310b`, from [CachyOS's release asset metadata](https://github.com/CachyOS/proton-cachyos/releases/tag/cachyos-11.0-20260703-slr). The earlier CI artifact digest differs from the published release digest; the release asset digest is the one used.
- Bellum's [official download page](https://playbellum.com/download/) redirects through `astarte.launcher.link` to `releases.astarte.industries`. Pinned that current executable to SHA-256 `2c2d17b724bee70883eae782d2ff9ead2533d2d339fd4ee1b9326c60bb3f064a`; its embedded certificate subject is `ASTARTE INDUSTRIES INC.`. The runtime `osslsigncode` verification remains mandatory.
- RDNA4 now enables Proton's FSR4 driver component by default, with the RDNA3 upgrade off. NVIDIA's DLSS replacement and NGX updater remain off. `BELLUM_MANGOHUD=1`, `BELLUM_VKBASALT=1`, and `BELLUM_GAMESCOPE=1` opt into the permitted overlays.
- Selected the official Steam EAC runtime app `1826330`, depot `1826331`, public manifest `3310269496439035229`, build `10437216` as the reproducible source. SteamCMD anonymous login returned `No subscription` for both `download_depot` and `app_update`. An authenticated Steam install is needed to calculate the six-file SHA-256 digest. The installer now checks the exact depot files and fails closed while `EACRuntimeSHA256` is empty. No runtime blob is committed.
- Focused Go package tests and `git diff --check` pass. Paperclip checkout failed twice with connection refused, so the issue status and work products could not be updated in this heartbeat.

## Implemented in the shared workspace

- Removed `packages/fsr4/` and the `--fsr41` CLI option. Removed game-directory DLL copies and added a write boundary plus regression test.
- Generated wrappers for AMD, NVIDIA, and Intel use `umu-run` with the selected Proton and require `PROTON_EAC_RUNTIME` to exist. They write the launch log in the prefix.
- Runtime upscaler upgrades remain off by default; the game's shipped upscalers are left alone.
- Vendored winetricks is hash checked and installed to `~/.local/bin` without sudo or self-update. The uninstaller requires a Bellum prefix with Wine markers, refuses root/home/symlink paths, and defaults its destructive prompt to No.
- Installer logs use a private directory and `O_NOFOLLOW`; tar entries and link targets are checked before extraction. The user launcher is installed in `~/.local/bin`; desktop permissions are `0644`.
- SHA-256 pins are implemented for the vendored DXVK, winetricks, and icon files. Proton and launcher downloads require release pins and fail closed while those pins are empty. Launcher verification also requires an approved Authenticode signer and a successful `osslsigncode verify`.
- `go test ./...` and `git diff --check` pass. See `docs/eac-qa.md` for the Linux-mode log check.
- A follow-up fixed literal `\\n` escapes in the Intel/generic `launch_vars.env` writer. Targeted `go test ./pkg/workflow ./pkg/launchers ./pkg/packages` passes with a writable Go cache.
- The WebView2 follow-up adds a nonblocking prefix `flock` for second clicks, keeps umu's session-wait verb, verifies the installed WebView2 runtime after launcher setup, and restricts the prefix, launch variables, and launcher log to private permissions. NVIDIA's NGX updater defaults off; the capability-gated FSR runtime switch uses `1` instead of the removed `4.1.0` path. The QA checklist now covers launcher lifecycle and evidence redaction. Focused package tests and `git diff --check` pass.

## Remaining release blockers

1. Install Steam app `1826330` through an entitled Steam account, verify its manifest is `3310269496439035229`, compute `EACRuntimeDigest` for those six files, set `EACRuntimeSHA256`, and add a provisioner that validates this exact manifest before use. Anonymous SteamCMD cannot fetch it. Verify `osslsigncode` output against the pinned official launcher in a test environment.
2. Run the TES-12 EAC Linux-mode QA check against a real Bellum installation and attach the observed log lines.
   Include a real umu container lifecycle test through launcher self-update and tray Quit; static tests cannot establish whether the container stays alive after a launcher process handoff.
3. Review overlap with TES-10/TES-11 in the shared checkout and update the Paperclip issue/work products. The Paperclip API at `127.0.0.1:3100` refused connections in both heartbeats, so the board status and work products could not be changed.

The installer deliberately fails before installation while the release pins are missing. Do not publish this state as a working release.
