# Installer workflow boundary audit

## Current effects and mutations

- Host discovery is partly injectable through `CommandRunner.LookPath`, `CommandRunner.Output`, and `FileStore`. `DiscoverExecutable` backs executable probes; Wine version and SSD detection use injected command output. Directory existence and directory-entry discovery use `FileStore`. GPU detection, interactive GUI/input, EAC verification, and the writable-directory probe retain direct host effects. Some precheck helpers such as Proton discovery still call package-level download/extraction code directly.
- Installation uses `AcquirePackage` for launcher installer acquisition, `MutatePrefix` for prefix commands, and `GenerateLauncher` for wrapper and desktop generation. The installer sets process environment variables directly. DXVK archive checks, extraction, setup execution, cleanup, and prefix config copying now use injected file, command, and package operations through `InstallDXVKWithBoundaries`. Launcher icon and launch-variable writes are performed by launcher package code behind the launcher generator rather than through `FileStore`.
- Configuration runs Wine registry commands through `MutatePrefix` and writes generated launch-variable files through guarded `FileStore`. These mutations include system DLL overrides and application overrides for `AstarteLauncher.exe` and `Bellum-Win64-Shipping.exe` (`d3d11` and `dxgi`). The current configuration no longer copies FSR DLLs into the game tree.
- Game-directory mutation inventory: no installer game-tree file writes are present in the current workflow. The only game-related mutations found are Wine registry overrides (system DLLs and `AstarteLauncher.exe`/`Bellum-Win64-Shipping.exe` overrides including `d3d11` and `dxgi`); they change prefix registry state, not game files. Uninstallation removes launcher/prefix files, not game-loaded DLLs. `GuardGameTreeWrites` remains a tested guard for any future writes.
- Launcher package internals are a documented exception: the workflow injects the single `GenerateLauncher` operation, while `pkg/launchers` owns its implementation details (desktop/icon generation, path conventions, and file writes). Splitting that package into its own file and command interfaces is a separate package-level refactor; no launcher operation writes into the game install tree.
- Uninstallation exposes command and file effects through `RunUninstallationWithBoundaries`. On explicit confirmation it removes `/usr/local/bin/Bellum`, user desktop/icon files, the version-selected Proton tree, and recursively deletes the selected WINEPREFIX. Prefix deletion remains destructive by design and requires the existing confirmation.

## Hard-coded paths and remaining risks

- `/usr/local/bin/Bellum` is embedded in launcher generation, status text, detection, and uninstallation. Desktop and icon paths use `$HOME` with fixed `.local/share` layouts.
- Steam client path is constructed as `$HOME/.steam/steam`. Game executable names and DLL destinations are fixed to the current Bellum launcher/install layout.
- Install commands inherit process environment state; `RunInstaller` sets global environment variables. Several historical command errors during final installer shutdown/registry setup remain ignored.
- Isolation is improved but incomplete: prefix selection and other GUI/input, writable-directory, GPU detection, EAC verification, and some package extraction paths still use direct host APIs. Launcher generation is injected at the workflow boundary; its package internals (home-directory resolution, desktop/icon writes, and optional desktop database commands) remain an explicit exception. Focused tests cover executable discovery, injected SSD probing, launch-variable file writes, game-tree write rejection, and DXVK's injected effects.

## Verification

- Verification for this revision is recorded in the TES-6 issue comment.
