# Bellum EAC Linux mode check

1. Obtain the runtime through the Steam client using an entitled Steam account. In Steam, open **Library → Tools**, search for **Proton EasyAntiCheat Runtime**, and install it (Steam app `1826330`; direct client URI: `steam://install/1826330`). Keep it in that Steam library; do not copy it into the installer or redistribute depot files. For a non-default library, set `PROTON_EAC_RUNTIME` to `<library>/steamapps/common/Proton EasyAntiCheat Runtime` when starting the installer and launcher. The default lookup is `~/.local/share/Steam/steamapps/common/Proton EasyAntiCheat Runtime`.
2. Confirm the selected Proton build and runtime match the approved release manifest. Bellum hashes the six required files under `v2/lib32` and `v2/lib64`; a missing file or digest mismatch must stop setup before launch. The official source selected for the pin is app `1826330`, depot `1826331`, manifest `3310269496439035229` (build `10437216`). The runtime digest has not yet been verified from an entitled client install, so the pin remains empty and the current installer intentionally fails closed. Installing the runtime alone does not bypass this gate; release use requires the verified digest to be added first.
3. Start `~/.local/bin/Bellum` with `PROTON_LOG=1` and `UMU_LOG=1`. The wrapper sources `<prefix>/launch_vars.env`, checks `PROTON_EAC_RUNTIME`, and executes `umu-run` for AMD, NVIDIA, and Intel. It writes `<prefix>/launcher.log`.
4. Inspect the new log after the game's protected process starts:

   ```sh
   rg -i 'umu|proton|easyanticheat|eac|anti.cheat' "$WINEPREFIX/launcher.log"
   ```

5. Record the actual log lines showing umu selected the pinned Proton and that the EasyAntiCheat Linux runtime initialized for the protected game process. A launch or sign-in alone is insufficient evidence; if no explicit EAC initialization line appears, mark the check inconclusive and collect the Proton/umu debug logs. Record the GPU vendor, runtime path, Proton version, and result on the QA issue. Static hash verification is not proof that Bellum's protected session works in Linux mode; that live evidence belongs to TES-12.

The launcher never stages DLLs in the game's directory. RDNA4 enables Proton's FSR4 driver component by default; RDNA3 and DLSS upgrades stay off. MangoHud, vkBasalt, and gamescope are opt-in through `BELLUM_MANGOHUD=1`, `BELLUM_VKBASALT=1`, and `BELLUM_GAMESCOPE=1` on the wrapper command.

Match EAC evidence to product `087dc666152349c68aa8e1962237c472`, sandbox `84c3e73046e546d282c07eee30ac3162`, and deployment `1ec8679293294023bb158112821a4041`. TES-16 is verifying the authoritative log location and Linux-mode indicators; do not infer success from these IDs alone.

## Launcher lifecycle and private data

The Astarte Launcher must remain running while Bellum runs because it authenticates the game. Verify that the umu container survives launcher self-update, tray minimization, and the launcher's initial process handoff; confirm the protected game stays alive until Quit. A second wrapper click must return promptly with an already-running message. Confirm that Quit ends the session and a later click starts it again. Verify WebView2's `msedgewebview2.exe` is present after install; a bootstrapper alone is insufficient. Confirm the prefix is `0700` and `launch_vars.env` plus `launcher.log` are `0600`.

The prefix contains saved login credentials, access certificates, and WebView2 cookies. Redact usernames, tokens, certificate material, and cookies from logs or screenshots before attaching QA evidence.
