# Bellum Linux Installer

Play **Bellum** on Linux. This installer sets Bellum up in its own Proton prefix,
using a pinned Proton build, Valve's Proton EasyAntiCheat Runtime, and the
official Astarte Launcher. It then adds a desktop shortcut, an app-menu entry,
and a `Bellum` terminal command.

> This is a community project. It is **not** official support and is not
> affiliated with Astarte Industries. The goal is simple: nobody who has moved to
> Linux should have to boot Windows to play Bellum.

## Project status (2026-09-27)

| | |
| --- | --- |
| **Easy Anti-Cheat on Linux** | ✅ Astarte has enabled Proton/Linux EAC support for Bellum. |
| **Latest published release** | ⚠️ [`v2.0.1`](https://github.com/Ch3w3y/bellum-linux-installer/releases/tag/v2.0.1) (2026-05-13) **predates** the September hardening on `master`: pinned and verified downloads, the EAC-safe launcher, and ownership-checked uninstall. A new release is pending; until then, [build from `master`](#install-today-build-from-master). |
| **One-command install** | 🚧 Planned. See [#6](https://github.com/Ch3w3y/bellum-linux-installer/issues/6). Not available yet. |
| **Known blockers** | [#7](https://github.com/Ch3w3y/bellum-linux-installer/issues/7), [#8](https://github.com/Ch3w3y/bellum-linux-installer/issues/8), [#9](https://github.com/Ch3w3y/bellum-linux-installer/issues/9), [#10](https://github.com/Ch3w3y/bellum-linux-installer/issues/10). Workarounds are under [Known issues](#known-issues-and-workarounds). |

## Where we're going

This project is for people coming to Linux **from Windows**. The target
experience ([#6](https://github.com/Ch3w3y/bellum-linux-installer/issues/6)) is a
single command:

```bash
# Planned: this script does not exist yet.
bash <(curl -fsSL https://raw.githubusercontent.com/Ch3w3y/bellum-linux-installer/master/install.sh)
```

It will check your system, tell you the one command to install anything that's
missing, and offer a **Stable** or **Performance** preset plus optional extras
(MangoHud, gamescope). You confirm once, and it finishes with a working game.
Nothing is changed on your system before you confirm, and a failed or
interrupted install can simply be re-run.

## How it works

```
Steam ──────────────► Proton EasyAntiCheat Runtime (app 1826330, stays in your Steam library)
                                   │
installer ─► ~/.local/share/bellum/proton/…  (pinned Proton-CachyOS, SHA-256 verified)
          ─► <your folder>/Bellum            (a private Wine/Proton prefix just for Bellum)
               └─ Astarte Launcher (signature verified) → installs & updates the game
          ─► ~/.local/bin/Bellum + Bellum.desktop  (runs everything through umu-run)
```

- Bellum gets **its own prefix**. The installer won't touch your other Wine or
  Steam prefixes, and it won't install into a folder that already has files in
  it.
- The installer **never copies or replaces DLLs** in the game or the prefix.
  Upscaler behaviour comes only from Proton's built-in features. See
  [Graphics and upscalers](#graphics-and-upscalers).
- Downloads are pinned. Proton and the launcher installer are checked by
  SHA-256, and the launcher also by its Authenticode signer
  (`ASTARTE INDUSTRIES INC.`). Anything that fails a check stops the install.

## Before you start

### Hardware and system

- An **x86_64** PC with a Vulkan-capable GPU and an up-to-date driver (Mesa for
  AMD/Intel, the proprietary driver for NVIDIA).
- An **SSD or NVMe** drive for the install. Astarte strongly recommends it, and
  the installer warns you otherwise.
- **Steam** installed and signed in. It's needed for the EAC runtime below.

### 1. Install the Proton EasyAntiCheat Runtime

In Steam, open **Library → Tools**, find **Proton EasyAntiCheat Runtime**, and
install it. You can also open `steam://install/1826330` in your browser.

The installer looks for it in
`~/.local/share/Steam/steamapps/common/Proton EasyAntiCheat Runtime`. If you use
**Flatpak Steam** or another Steam library folder, point the installer at it:

```bash
export PROTON_EAC_RUNTIME="$HOME/.var/app/com.valvesoftware.Steam/.local/share/Steam/steamapps/common/Proton EasyAntiCheat Runtime"
```

> ⚠️ The installer currently expects one specific runtime build (Steam build
> `10437216`) and refuses to continue if Steam has updated it. Tracked in
> [#9](https://github.com/Ch3w3y/bellum-linux-installer/issues/9).

### 2. Install host packages

The installer checks for these tools and prints install hints, but it never runs
your package manager for you.

| Tool | Why | Required? |
| --- | --- | --- |
| `wine` | Prefix bootstrap commands | Yes. The installer currently requires **exactly Wine 11.8**, see below |
| `umu-run` (umu-launcher) | Runs the launcher and game inside Proton's runtime container | Yes |
| `osslsigncode` | Verifies the Astarte Launcher's signature | Yes. This is **not** checked up front yet ([#8](https://github.com/Ch3w3y/bellum-linux-installer/issues/8)), so install it first |
| `wget` | Downloads Proton and the launcher | Yes |
| `glxinfo` | Better GPU detection (falls back to `lspci` or sysfs without it) | Recommended |
| `zenity` or `kdialog` | Graphical folder picker (falls back to a terminal prompt) | Optional |
| `winetricks` | Not needed: a pinned copy is bundled and installed to `~/.local/bin` | No |

| Distro family | Command |
| --- | --- |
| Arch / CachyOS / EndeavourOS / Manjaro | `sudo pacman -S wine umu-launcher wget mesa-utils zenity` (`umu-launcher` is in `[multilib]`, which must be enabled), plus `osslsigncode` from the AUR (e.g. `yay -S osslsigncode`) |
| Fedora | `sudo dnf install wine osslsigncode wget glx-utils zenity`, and `umu-launcher` from its [GitHub releases](https://github.com/Open-Wine-Components/umu-launcher/releases) or your spin's repo (Bazzite and Nobara ship it) |
| Debian / Ubuntu / Mint / Pop!_OS | `sudo apt install wine osslsigncode wget mesa-utils zenity`, and `umu-launcher` from the `.deb` on its [GitHub releases](https://github.com/Open-Wine-Components/umu-launcher/releases) (not in the distro repos) |
| openSUSE Tumbleweed | `sudo zypper install wine osslsigncode wget Mesa-demo-x zenity`, and `umu-launcher` from the OBS [`games`](https://build.opensuse.org/package/show/games/umu-launcher) repo |
| SteamOS / Bazzite (immutable) | Install only through the host's supported method (distrobox/toolbox or the system's layering tool). The installer will not modify an immutable host. |

> ⚠️ **Wine version.** The installer refuses to run unless `wine --version`
> reports exactly `wine-11.8`, a development release from May 2026 that most
> distros won't ship. You can bypass the check with `--force-wine-version`. The
> game itself runs on the pinned Proton, not system Wine. Removing this
> requirement is tracked in
> [#8](https://github.com/Ch3w3y/bellum-linux-installer/issues/8).

## Install today (build from `master`)

Until a new release is published, build the current code. You need
[Go 1.24+](https://go.dev/dl/), `git` and `make`.

```bash
git clone https://github.com/Ch3w3y/bellum-linux-installer.git
cd bellum-linux-installer
make release
cd dist/bellum-installer-linux-amd64-2.0.1   # run the installer from INSIDE this folder
./installer
```

> Run `./installer` from inside the extracted folder. Some bundled files are
> currently looked up relative to your terminal's working directory
> ([#8](https://github.com/Ch3w3y/bellum-linux-installer/issues/8)).

Once a new release is out, you'll be able to download the tarball from
[Releases](https://github.com/Ch3w3y/bellum-linux-installer/releases/latest),
extract it with `tar -xzf`, `cd` into it, and run `./installer` the same way.
Check that you're on **this** repository's Releases page; the original
`joepaji/bellum-linux-installer` publishes separate, older builds.

### What happens during install

1. **Choose where to install.** A folder picker opens. Pick the **parent**
   folder: the installer creates a `Bellum` folder inside it (picking `~/Games`
   gives `~/Games/Bellum`).
2. **Prechecks.** The installer detects your GPU, checks tools, downloads and
   verifies Proton (a download of several hundred MB, one time only), and verifies the EAC runtime.
3. **Confirm the summary.**
4. **Prefix setup.** It creates the prefix and installs runtime components
   (Visual C++, .NET 9 and others). This takes a while.
5. **The Astarte Launcher installer appears.** Follow its prompts. Don't close
   the terminal.
6. **Finishing steps.** The installer writes the launcher wrapper, desktop
   shortcut and launch settings. Wait for *"Installation completed
   successfully!"*

<img width="800" alt="Selecting the install directory" src="https://github.com/user-attachments/assets/826d7e36-1471-4cd2-9c61-8440252456aa" />
<img width="800" alt="Install summary" src="https://github.com/user-attachments/assets/5347c5bd-c44d-4f37-b89b-cdbf4e137ae9" />
<img width="800" alt="Astarte Launcher installer" src="https://github.com/user-attachments/assets/5ba0340b-9d2a-45f4-954c-83befd331534" />
<img width="800" alt="Installation complete" src="https://github.com/user-attachments/assets/aab9f336-9307-4e2a-b7da-f5f8655eb92b" />

### Advanced options

```bash
./installer --wineprefix ~/Games          # skip the picker (creates ~/Games/Bellum)
WINEPREFIX=~/Games/Bellum ./installer     # same thing, via the environment
./installer --launcher-installer ./AstarteLauncher-amd64-installer.exe  # use a local copy (still verified)
./installer --help
```

For both `--wineprefix` and `WINEPREFIX`, `Bellum` is appended unless the path
already ends in `Bellum`. The target folder must not exist yet, or must be
empty.

## Playing

After installing, launch Bellum any of these ways:

- the **Bellum** desktop shortcut (if you have a `~/Desktop` folder),
- **Applications → Games → Bellum**,
- the `Bellum` command in a terminal. It lives in `~/.local/bin`. If your shell
  says *command not found*, add `export PATH="$HOME/.local/bin:$PATH"` to
  `~/.bashrc` and open a new terminal.

The Astarte Launcher must stay open while you play, because it authenticates
the game. A second launch while Bellum is already running is ignored.

**Optional extras.** Add any of these lines to `<prefix>/launch_vars.env`, or
put them before the `Bellum` command:

```bash
export BELLUM_MANGOHUD=1   # performance overlay (needs mangohud)
export BELLUM_GAMESCOPE=1  # run inside gamescope (needs gamescope)
export BELLUM_VKBASALT=1   # vkBasalt post-processing (needs vkbasalt)
```

**Logs.** The game and launcher log to `<prefix>/launcher.log`. Installer logs
are in `logs/` next to the `installer` binary.

## Graphics and upscalers

- **The installer never adds, copies or replaces DLLs** in the game directory
  or prefix. Don't hand-place proxy DLLs yourself either (ReShade or OptiScaler
  `dxgi.dll`/`d3d11.dll` and similar). Anything in the game folder loads into
  the process Easy Anti-Cheat protects.
- DXVK, vkd3d-proton and dxvk-nvapi come from the pinned Proton-CachyOS build.
  See [runtime pins](docs/runtime-pins.md).
- **NVIDIA (RTX and GTX 16-series):** NVAPI is enabled and DLSS uses the real
  driver libraries (`PROTON_NVIDIA_LIBS=1`). Older NVIDIA cards get the generic
  settings. Proton's DLSS DLL upgrade and the NGX updater are
  off. GTX 16-series cards have no DLSS.
- **AMD:** the pinned Proton build automatically provides AMD's FSR4 driver
  component on supported RDNA2–RDNA4 discrete GPUs. This is Proton's built-in
  behaviour, not something the installer does. On RDNA4 the installer also sets
  Proton's `PROTON_FSR4_UPGRADE=1` flag.
- **Intel and other GPUs:** generic Proton settings.

### NVIDIA RTX 50-series (Blackwell) driver note

Some 595-branch drivers have community-reported regressions under Proton. For
example, 595.71.05 fails Vulkan swapchain creation where 595.58.03 worked
([NVIDIA forum](https://forums.developer.nvidia.com/t/regression-595-71-05-blackwell-rtx-5070-vulkan-swapchain-creation-fails-vk-error-initialization-failed-under-proton-worked-on-595-58-03/371778)),
and there are system-wide lag reports after moving from 590 to 595
([CachyOS #378](https://github.com/CachyOS/distribution/issues/378)). If you get
shader-loading failures or a black screen on a 50-series card, try 595.58.03 or
the 590 branch. This is guidance from public reports; no driver branch has been
validated by this project.

## Known issues and workarounds

| Problem | Workaround | Tracking |
| --- | --- | --- |
| After a failed install, re-running says the `Bellum` folder *already exists* | Run `./uninstaller --wineprefix <path>/Bellum`. If the uninstaller refuses because the prefix is incomplete, check it's the right folder and delete it manually, then re-run the installer | [#7](https://github.com/Ch3w3y/bellum-linux-installer/issues/7) |
| *Wine version mismatch* | Use `--force-wine-version` | [#8](https://github.com/Ch3w3y/bellum-linux-installer/issues/8) |
| *AstarteLauncher Authenticode verification failed* | Install `osslsigncode` and re-run | [#8](https://github.com/Ch3w3y/bellum-linux-installer/issues/8) |
| *EAC runtime digest mismatch* after a Steam update | No workaround yet; wait for a pin update | [#9](https://github.com/Ch3w3y/bellum-linux-installer/issues/9) |
| *SHA-256 mismatch* for the launcher installer after an Astarte update | No workaround yet; wait for a pin update | [#9](https://github.com/Ch3w3y/bellum-linux-installer/issues/9) |
| Install fails at the very end with *unsupported GPU type* (VMs, unrecognised GPUs) | Not supported yet | [#10](https://github.com/Ch3w3y/bellum-linux-installer/issues/10) |
| The shortcut and `Bellum` command remain after uninstalling | Remove them manually (see below) | [#10](https://github.com/Ch3w3y/bellum-linux-installer/issues/10) |

## Uninstalling

```bash
./uninstaller --wineprefix ~/Games/Bellum --dry-run   # show what would be removed
./uninstaller --wineprefix ~/Games/Bellum             # remove it (asks first; default is No)
```

For the uninstaller, point `--wineprefix` or `WINEPREFIX` **at the `Bellum`
folder itself**; nothing is appended. Without either, a folder picker opens. The
uninstaller only deletes a folder named `Bellum` that contains the installer's
`.bellum-manifest.json` and a Wine prefix. It refuses `/`, your home folder and
symlinks.

The prefix holds your launcher login, certificates and WebView2 cookies. Back it
up first if you need anything in it.

The uninstaller currently removes **only the prefix**. To remove everything
else:

```bash
rm -f ~/.local/bin/Bellum ~/.local/share/applications/Bellum.desktop ~/Desktop/Bellum.desktop \
      ~/.local/share/icons/hicolor/256x256/apps/bellum.png
rm -rf ~/.local/share/bellum/proton     # shared Proton; only if no other Bellum install uses it
rm -f ~/.local/bin/winetricks           # only if you don't use winetricks elsewhere
```

## For contributors

```bash
make check                    # what CI runs: gofmt, go vet, go test, module pinning
make release VERSION=2.1.0    # reproducible tarball + MANIFEST.md + SHA256SUMS in dist/
make verify-release           # re-check the staged checksums
```

CI only runs static checks, unit tests and compile-only builds. It never runs the
installer. Live installs are tested by hand on real hardware.

| Doc | Contents |
| --- | --- |
| [RELEASE.md](RELEASE.md) | Release gate, reproducibility, and binary/provenance policy |
| [docs/runtime-pins.md](docs/runtime-pins.md) | Pinned Proton and launcher versions, hashes, and what the Proton build does with upscalers |
| [docs/eac-qa.md](docs/eac-qa.md) | How to capture EAC and launcher evidence during manual QA |
| [INSTALLER_AUDIT.md](INSTALLER_AUDIT.md) | Every host side effect the installer and uninstaller have |

Roadmap and open work:
[#6 one-command install (epic)](https://github.com/Ch3w3y/bellum-linux-installer/issues/6).
The DLL policy is recorded in
[#11](https://github.com/Ch3w3y/bellum-linux-installer/issues/11).
