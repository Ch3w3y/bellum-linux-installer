# Bellum EAC Linux mode check

1. Confirm the selected Proton build and Proton EasyAntiCheat Runtime match the approved release manifest. The installer must fail before launch when either pin is absent or the runtime directory is missing.
2. Start `~/.local/bin/Bellum` with `PROTON_LOG=1` and `UMU_LOG=1`. The wrapper sources `<prefix>/launch_vars.env`, checks `PROTON_EAC_RUNTIME`, and executes `umu-run` for AMD, NVIDIA, and Intel. It writes `<prefix>/launcher.log`.
3. Inspect the new log after the game's protected process starts:

   ```sh
   rg -i 'umu|proton|easyanticheat|eac|anti.cheat' "$WINEPREFIX/launcher.log"
   ```

4. Record the actual log lines showing umu selected the pinned Proton and that the EasyAntiCheat Linux runtime initialized for the protected game process. A launch or sign-in alone is insufficient evidence; if no explicit EAC initialization line appears, mark the check inconclusive and collect the Proton/umu debug logs. Record the GPU vendor, runtime path, Proton version, and result on the QA issue.

The launcher never stages DLLs in the game's directory. RDNA4 enables Proton's FSR4 driver component by default; RDNA3 and DLSS upgrades stay off. MangoHud, vkBasalt, and gamescope are opt-in through `BELLUM_MANGOHUD=1`, `BELLUM_VKBASALT=1`, and `BELLUM_GAMESCOPE=1` on the wrapper command.

Match EAC evidence to product `087dc666152349c68aa8e1962237c472`, sandbox `84c3e73046e546d282c07eee30ac3162`, and deployment `1ec8679293294023bb158112821a4041`. TES-16 is verifying the authoritative log location and Linux-mode indicators; do not infer success from these IDs alone.

## Launcher lifecycle and private data

The Astarte Launcher must remain running while Bellum runs because it authenticates the game. Verify that the umu container survives launcher self-update, tray minimization, and the launcher's initial process handoff; confirm the protected game stays alive until Quit. A second wrapper click must return promptly with an already-running message. Confirm that Quit ends the session and a later click starts it again. Verify WebView2's `msedgewebview2.exe` is present after install; a bootstrapper alone is insufficient. Confirm the prefix is `0700` and `launch_vars.env` plus `launcher.log` are `0600`.

The prefix contains saved login credentials, access certificates, and WebView2 cookies. Redact usernames, tokens, certificate material, and cookies from logs or screenshots before attaching QA evidence.
