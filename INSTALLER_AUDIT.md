# Installer workflow boundary audit

Reviewed against `master` on 2026-09-27. Open defects found in this review are
tracked in #7–#10 and linked inline.

## Current effects and mutations

- **Host discovery** runs through the injectable `precheckHost` (commands, files, GPU detection, prompts, free space, EAC verification, launcher staging), so the whole precheck phase is tested against fakes. The writable-directory probe uses `access(2)` and writes nothing.
- **Prechecks are read-only.** `RunPrechecks` checks the prefix path, every required tool (`umu-run`, `osslsigncode`, `wget`), the EAC runtime and free disk space, and reports every problem in one message with the distro install command. It writes nothing to `$HOME`, creates no prefix and downloads nothing; a `--launcher-installer` file is verified into a private temporary copy. Proton is downloaded, verified and patched by `AcquireRuntime` only after `ConfirmProceed`. The GUI picker does not create `<picked>/Bellum`; only `RunInstaller` creates the prefix.
- **Installation** uses `AcquirePackage` for launcher download and verification (SHA-256 allowlist plus `osslsigncode` Authenticode signer check; the download goes to a private temporary directory)., `MutatePrefix` for prefix commands, and `GenerateLauncher` for the wrapper and desktop files. It writes `.bellum-manifest.json` into the prefix first. It sets process environment variables directly (`PROTONPATH`, `WINEPREFIX`, `STEAM_COMPAT_*`, `GAMEID`). Every prefix command (`wineboot --init`, winetricks verbs, `reg add`, the launcher installer) runs through `umu-run` and the pinned Proton; winetricks is the copy in Proton's `protonfixes/`. System Wine is not used. No separate DXVK/vkd3d overlay is installed; those components come from the pinned Proton.
- **Rollback and retry**: prefix-path resolution (`ResolvePrefixPath`) appends `Bellum` in one place for the flag, the environment variable and the GUI. A failed install removes the prefix only if this run created it; no precheck creates it, so this covers the GUI path too. The prefix carries `.bellum-install-incomplete` from creation until configuration finishes. A configuration failure discards the prefix and its launcher assets, and an interrupted run is offered a restart on the next run. The replacement happens only after confirmation. A failed launcher-generation step restores any wrapper, desktop and icon files that existed before.
- **Configuration** runs Wine registry commands through `MutatePrefix` and writes `launch_vars.env` through `GuardGameTreeWrites`. Registry changes: system DLL overrides (`d3d12`, `d3d12core`, `d3d10core`, `d3d9`, `d3d8` → `native,builtin`), and per-application `d3d11`/`dxgi` overrides for `AstarteLauncher.exe` (builtin) and `Bellum-Win64-Shipping.exe` (native). These change prefix registry state, not files.
- **Game-directory mutation inventory**: the installer writes no files into the game tree and never copies or replaces DLLs anywhere (board policy, #11). `GuardGameTreeWrites` is a regression guard for installer writes, not a sandbox. Upscaler DLLs that the pinned Proton runtime stages itself at launch are Proton behaviour; see `docs/runtime-pins.md`.
- **Launcher package** (`pkg/launchers`) owns its own file writes: `~/.local/bin/Bellum` (wrapper), `~/.local/share/applications/Bellum.desktop`, `~/Desktop/Bellum.desktop` (if `~/Desktop` exists), and `~/.local/share/icons/hicolor/256x256/apps/bellum.png`. It also runs `gio`/`update-desktop-database` when they are available. Desktop generation rejects GPU types other than AMD/NVIDIA/Intel (#10).
- **Uninstallation** (`RunUninstallationWithBoundaries`) requires an absolute path named `Bellum` that contains `system.reg`, `drive_c`, and `.bellum-manifest.json`, and it refuses `/`, `$HOME`, and symlinks. After an explicit `y` it recursively deletes the prefix, then removes the wrapper, desktop entries and icon only when they reference that prefix. Shared Proton remains, as does `~/.local/bin/winetricks` from installs made before winetricks moved into Proton. Re-running uninstall after the prefix is gone is safe. `--dry-run` reports the target and changes nothing.

## Hard-coded paths and remaining risks

- The wrapper is `~/.local/bin/Bellum`; that directory is not on `PATH` by default on every distro (#10). Desktop and icon paths use fixed `$HOME/.local/share` layouts.
- The EAC runtime path defaults to `~/.local/share/Steam/steamapps/common/Proton EasyAntiCheat Runtime` (override: `PROTON_EAC_RUNTIME`). Flatpak Steam and secondary libraries are not discovered (#9).
- `STEAM_COMPAT_CLIENT_INSTALL_PATH` is set to `$HOME/.steam/steam` during install and to an empty string in the NVIDIA `launch_vars.env`.
- Several command errors are deliberately ignored (`winetricks win11` and the RawInput registry write).

## Verification

- `go vet ./...` and `go test ./...` pass on `master` @ cd10936 (go1.24).
- Earlier revision evidence is recorded in the TES-6 tracker comment (internal).
