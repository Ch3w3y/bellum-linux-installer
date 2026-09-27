# Bellum EAC Linux mode check

> Status (2026-09-27): Astarte has enabled Proton/Linux EAC support for Bellum.
> Use this checklist to capture per-release evidence that the installed
> configuration works. Links of the form `/TES/issues/...` point to the
> internal tracker and do not resolve on GitHub. The digest in step 2 identifies a
> known build; after a Steam update the installer warns and continues.

1. Obtain the runtime through the Steam client using an entitled Steam account. In Steam, open **Library → Tools**, search for **Proton EasyAntiCheat Runtime**, and install it (Steam app `1826330`; direct client URI: `steam://install/1826330`). Keep it in that Steam library; do not copy it into the installer or redistribute depot files. The installer finds it in any native or Flatpak Steam library; `PROTON_EAC_RUNTIME` overrides the lookup.
2. Confirm the installed app manifest reports build `10437216`; the approved source is Steam app `1826330`, depot `1826331`, manifest `3310269496439035229`. Bellum verifies SHA-256 over each required relative path, a NUL byte, that file's bytes, and a trailing NUL, in sorted manifest order. The six individual file hashes and sizes, plus the combined manifest digest `4d18c3a5b896c757be9e25bf1004b81568bc4d4e56ddd8d1a2a634eebf12d1f9`, are recorded in [TES-13 remediation status](internal/tes13-remediation-status.md). Missing or non-regular files fail closed; a changed digest (a Steam update) is logged as a warning and should be added to `EACRuntimeSHA256Allowlist` after this checklist passes. The app remains in the user's Steam library; Bellum neither downloads nor redistributes the runtime.
3. Start `~/.local/bin/Bellum` with `PROTON_LOG=1` and `UMU_LOG=1`. The wrapper sources `<prefix>/launch_vars.env`, checks `PROTON_EAC_RUNTIME`, and executes `umu-run` for AMD, NVIDIA, and Intel. It writes `<prefix>/launcher.log`.
4. Inspect the new log after the game's protected process starts:

   ```sh
   rg -i 'umu|proton|easyanticheat|eac|anti.cheat' "$WINEPREFIX/launcher.log"
   ```

5. Follow the [EAC Linux-mode verification procedure](/TES/issues/TES-16#document-eac-linux-verification) for the fresh EAC logs under `drive_c/users/*/AppData/Roaming/EasyAntiCheat`, the Proton log, and the protected multiplayer session. Confirm `linux64` module selection, successful module load and Wine mapping, and protected-session admission and retention for the same launch. The `launcher.log` check above only confirms the umu configuration. Record the GPU vendor, runtime path, Proton version, and redacted log lines on [TES-12](/TES/issues/TES-12). Mark missing or ambiguous EAC evidence inconclusive. Static hash verification is not proof that Bellum's protected session works in Linux mode; that live evidence belongs to TES-12.

The installer and launcher never add, copy, or replace DLLs (#11). The pinned Proton runtime stages AMD's FSR4 driver component itself on supported RDNA2–RDNA4 GPUs; the launcher additionally sets `PROTON_FSR4_UPGRADE=1` on RDNA4. DLSS DLL upgrades and the NGX updater stay off (see [runtime pins](runtime-pins.md)). MangoHud, vkBasalt, and gamescope are opt-in through `BELLUM_MANGOHUD=1`, `BELLUM_VKBASALT=1`, and `BELLUM_GAMESCOPE=1` on the wrapper command.

Match EAC evidence to product `087dc666152349c68aa8e1962237c472`, sandbox `84c3e73046e546d282c07eee30ac3162`, and deployment `1ec8679293294023bb158112821a4041`. Do not infer success from these IDs alone.

## Launcher lifecycle and private data

The Astarte Launcher must remain running while Bellum runs because it authenticates the game. Verify that the umu container survives launcher self-update, tray minimization, and the launcher's initial process handoff; confirm the protected game stays alive until Quit. A second wrapper click must return promptly with an already-running message. Confirm that Quit ends the session and a later click starts it again. Verify WebView2's `msedgewebview2.exe` is present after install; a bootstrapper alone is insufficient. Confirm the prefix is `0700` and `launch_vars.env` plus `launcher.log` are `0600`.

The prefix contains saved login credentials, access certificates, and WebView2 cookies. Redact usernames, tokens, certificate material, and cookies from logs or screenshots before attaching QA evidence.
