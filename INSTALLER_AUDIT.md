# Installer side-effect audit

Every change the installer, the `Bellum` wrapper and the uninstaller make to
the host, and the checks around them. Reviewed against `main` for v2.2.0.

## Files written

| Path | Written by | Contents |
| --- | --- | --- |
| `~/.local/share/bellum-installer/<version>/` | `install.sh` | the unpacked release (installer, uninstaller, manifest, icon) and `logs/installer.log` |
| `~/.local/share/bellum/proton/<version>/` | installer | pinned Proton-CachyOS, unmodified, with a content-digest stamp |
| `~/.local/share/bellum/umu/<version>/` | installer | pinned umu-launcher zipapp |
| `~/.local/share/bellum/bin/bellum-installer` | installer | a copy of the installer that the wrapper runs to install launcher updates |
| `<prefix>/` (default `~/Games/Bellum`, mode `0700`) | installer, Proton | the Wine prefix, the Astarte Launcher and the game |
| `<prefix>/.bellum-manifest.json` | installer | ownership record naming this exact prefix; every delete requires it |
| `<prefix>/.bellum-install-incomplete` | installer | present from prefix creation until configuration finishes |
| `<prefix>/.bellum-launcher-release` | installer, wrapper | the launcher release Bellum installed and its SHA-256 |
| `<prefix>/launch_vars.env` (`0600`) | installer | the launch settings for this GPU vendor |
| `<prefix>/launcher.log` (`0600`) | wrapper | launcher, game and launcher-update output |
| `~/.local/bin/Bellum` | installer | the launch wrapper |
| `~/.local/share/applications/Bellum.desktop`, `~/Desktop/Bellum.desktop` | installer | shortcuts (the second only if `~/Desktop` exists), mode `0644` |
| `~/.local/share/icons/hicolor/256x256/apps/bellum.png` | installer | the shortcut icon |

Nothing is written outside `$HOME`, except packages you agree to install with
your package manager.

## Phases

- **Bootstrap (`install.sh`).** It refuses root and non-x86_64 systems, then
  downloads the release archive and its `SHA256SUMS` over HTTPS (curl or
  wget). It checks the archive, then every file inside against the archive's
  own checksums. Only after that does it offer `sudo <package manager>` for a
  missing `python3` 3.10+ or `flock`, asking `[Y/n]` first. It then unpacks and
  runs the installer with the terminal as input.
- **Prechecks are read-only.** `RunPrechecks` (host effects injected through
  `precheckHost`, so the phase is tested against fakes) resolves the prefix
  path, detects the GPU and display session, and checks the tools, free space
  and the EAC runtime. It reports every problem in one message. It creates
  nothing and downloads nothing. A `--launcher-installer` file is verified into
  a private temporary copy. If the EAC runtime is missing and Steam is
  installed, it offers to run `steam steam://install/1826330` (Steam asks for
  its own confirmation) and waits up to 20 minutes.
- **Runtime.** After the user confirms the summary, `AcquireRuntime` downloads
  umu-launcher and Proton with Go's HTTP client, checks their SHA-256 pins,
  and unpacks Proton into a staging directory. Unpacking rejects path
  traversal, absolute paths, and links that leave the tree or sit under
  another link; device files and other special entries are skipped, never
  created. The staged tree is stamped and renamed into place.
- **Install.** `RunInstaller` creates the prefix (`0700`), writes the manifest
  and the incomplete marker, and runs every prefix command through `umu-run`
  and the pinned Proton: `wineboot`, the winetricks verbs (Proton's own
  `protonfixes/winetricks`), the Astarte Launcher installer and `reg add`.
  System Wine is never used. The launcher installer is downloaded to a private
  temporary directory and needs a valid Authenticode signature from
  `ASTARTE INDUSTRIES INC.` (PE digest, PKCS#7, timestamp when present, a
  code-signing chain to the system roots plus the pinned GlobalSign Code
  Signing Root R45, exactly one signer common name; no revocation check). The
  WebView2 runtime must be present afterwards.
- **Launcher update.** Right after the launcher installer, and in update mode,
  `UpdateLauncherInPrefix` reads Astarte's release list
  (`releases.astarte.industries/…/RELEASES`). It picks the newest stable
  version and accepts only the official per-version `AstarteLauncher.exe` URL.
  If the prefix's `.bellum-launcher-release` record doesn't match that
  version and the file's current SHA-256, it downloads the exe next to the
  installed one, requires the same Authenticode signer, and renames it into
  place. A failure only warns.
- **Configuration.** Registry changes through `umu-run reg add`:
  - `d3d12`, `d3d12core`, `d3d10core`, `d3d9` and `d3d8` set to
    `native,builtin`;
  - per-application `d3d11` and `dxgi` overrides for `AstarteLauncher.exe`
    (builtin) and `Bellum-Win64-Shipping.exe` (native);
  - DirectInput `RawInput=1`.

  It then writes `launch_vars.env` through `GuardGameTreeWrites`, which
  refuses any write into the game directory. The installer never adds,
  copies or replaces DLLs (#11). Finally it removes the incomplete marker.
- **Launcher files.** `pkg/launchers` writes the wrapper, desktop entries and
  icon, and runs `gio` and `update-desktop-database` when available. If
  generation fails, the previous files are restored. The same wrapper serves
  every GPU vendor.
- **Wrapper (each launch).** It sources `launch_vars.env` and checks the EAC
  runtime, Proton and umu-run. It takes a non-blocking `flock` on the prefix,
  so a second click returns at once. It runs `bellum-installer
  update-launcher <prefix>` (a failure is logged, and the launch continues),
  then `umu-run` on the launcher with `PROTON_VERB=waitforexitandrun`, logging
  to `launcher.log`.
- **Update mode.** On a finished install (manifest present, no incomplete
  marker) the installer offers an update instead of refusing. It downloads the
  current pins, regenerates the wrapper and desktop files, rewrites
  `launch_vars.env` and the registry overrides, and updates the launcher. It
  never reruns winetricks or the launcher installer.

## Deletes

Every recursive delete of a prefix goes through `removeBellumPrefix`
(`pkg/workflow/prefix_guard.go`). That covers the uninstall, replacing an
unfinished install, and rolling back a failed install. Before deleting
anything it requires:

- an absolute path named `Bellum`, that is not `/`, not the home folder or a
  folder containing it, and not inside `/proc`, `/sys`, `/dev`, `/run`,
  `/boot`, `/etc`, `/usr`, `/bin`, `/sbin`, `/lib` or `/lib64`;
- no symlink anywhere in the path, and a real directory owned by the current
  user;
- `.bellum-manifest.json` as a regular file naming this exact path;
- the contents the caller expects: a Wine prefix or the incomplete marker for
  an uninstall, the incomplete marker when replacing an unfinished install;
- no other filesystem mounted anywhere inside (checked by walking the tree
  first).

A rollback whose manifest was never written only removes an empty directory.
Links inside the prefix (such as Wine's `dosdevices/z:` → `/`) are removed,
never followed.

The uninstaller asks before deleting (default No; `--dry-run` changes nothing).
It then removes the wrapper, desktop entries and icon only when they point at
that prefix. Shared files in `~/.local/share/bellum` and
`~/.local/share/bellum-installer` are kept. Running it again after the prefix
is gone is safe.

## Remaining risks

- `~/.local/bin` isn't on `PATH` by default everywhere; the finish message
  prints the line that adds it for the user's shell.
- The EAC runtime is located through `PROTON_EAC_RUNTIME`, then
  `appmanifest_1826330.acf` in every Steam library listed in
  `libraryfolders.vdf` under the native and Flatpak Steam roots. All six
  runtime files must exist; an unknown combined digest only warns, because
  Steam updates the runtime.
- The launcher is accepted on its signer. An unknown launcher digest only
  warns, because Astarte's download URL always serves the current build.
- Some command errors are deliberately non-fatal: `winetricks win11`, the
  RawInput registry write, and the launcher update (retried at every launch).

## Verification

`make check` (gofmt, `go vet`, `go test ./...`, module pinning) and
`scripts/test-release.sh` pass on `main`. The behaviour above is covered by
tests in `pkg/workflow` (prechecks, install transaction, update mode, prefix
guard, uninstall), `pkg/packages` (extraction, pins, Authenticode, launcher
updates) and `pkg/launchers` (wrapper behaviour, run in bash). Live installs
are verified on hardware per [the QA checklist](docs/eac-qa.md).
