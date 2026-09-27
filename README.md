# Bellum Linux Installer

Play **Bellum** on Linux. This installer sets Bellum up in its own Proton prefix,
using a pinned Proton build, Valve's Proton EasyAntiCheat Runtime, and the
official Astarte Launcher. It then adds a desktop shortcut, an app-menu entry,
and a `Bellum` terminal command.

> This is a community project. It is **not** official support and is not
> affiliated with Astarte Industries. The goal is simple: nobody who has moved to
> Linux should have to boot Windows to play Bellum.

This successor project builds on [Joheb Rahman (joepaji)'s original Bellum Linux
Installer](https://github.com/joepaji/bellum-linux-installer). Joheb created the
foundational installer and continues to own the original project. See
[Credits](CREDITS.md) for the project lineage and upstream update policy.

## Project status (2026-09-27)

| | |
| --- | --- |
| **Easy Anti-Cheat on Linux** | ✅ Astarte has enabled Proton/Linux EAC support for Bellum. |
| **Latest published release** | ⚠️ [`v2.0.1`](https://github.com/Ch3w3y/bellum-linux-installer/releases/tag/v2.0.1) (2026-05-13) **predates** the September hardening on `main`: pinned and verified downloads, the EAC-safe launcher, and ownership-checked uninstall. A new release is pending; until then, [build from `main`](#install-today-build-from-main). |
| **One-command install** | 🚧 Planned. See [#6](https://github.com/Ch3w3y/bellum-linux-installer/issues/6). Not available yet. |
| **Known blockers** | [#9](https://github.com/Ch3w3y/bellum-linux-installer/issues/9), [#10](https://github.com/Ch3w3y/bellum-linux-installer/issues/10). Workarounds are under [Known issues](#known-issues-and-workarounds). |

## Where we're going

This project is for people coming to Linux **from Windows**. The target
experience ([#6](https://github.com/Ch3w3y/bellum-linux-installer/issues/6)) is a
single command:

```bash
# Planned: this script does not exist yet.
bash <(curl -fsSL https://raw.githubusercontent.com/Ch3w3y/bellum-linux-installer/main/install.sh)
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

The installer checks for all of these before it changes anything, lists
everything that's missing at once, and prints the install command for your
distro. It never runs your package manager for you.

| Tool | Why | Required? |
| --- | --- | --- |
| `umu-run` (umu-launcher) | Runs every prefix step, the launcher and the game inside Proton's runtime container | Yes |
| `osslsigncode` | Verifies the Astarte Launcher's signature | Yes |
| `wget` | Downloads Proton and the launcher | Yes |
| `glxinfo` | Better GPU detection (falls back to `lspci` or sysfs without it) | Recommended |
| `zenity` or `kdialog` | Graphical folder picker (falls back to a terminal prompt) | Optional |
| `wine`, `winetricks` | Not needed. Everything runs on the pinned Proton, including the winetricks it bundles | No |

| Distro family | Command |
| --- | --- |
| Arch / CachyOS / EndeavourOS / Manjaro | `sudo pacman -S umu-launcher wget mesa-utils zenity` (`umu-launcher` is in `[multilib]`, which must be enabled), plus `osslsigncode` from the AUR (e.g. `yay -S osslsigncode`) |
| Fedora | `sudo dnf install osslsigncode wget glx-utils zenity`, and `umu-launcher` from its [GitHub releases](https://github.com/Open-Wine-Components/umu-launcher/releases) or your spin's repo (Bazzite and Nobara ship it) |
| Debian / Ubuntu / Mint / Pop!_OS | `sudo apt install osslsigncode wget mesa-utils zenity`, and `umu-launcher` from the `.deb` on its [GitHub releases](https://github.com/Open-Wine-Components/umu-launcher/releases) (not in the distro repos) |
| openSUSE Tumbleweed | `sudo zypper install osslsigncode wget Mesa-demo-x zenity`, and `umu-launcher` from the OBS [`games`](https://build.opensuse.org/package/show/games/umu-launcher) repo |
| SteamOS / Bazzite (immutable) | Install only through the host's supported method (distrobox/toolbox or the system's layering tool). The installer will not modify an immutable host. |

You also need the **Proton EasyAntiCheat Runtime** from Steam (free, no game
purchase needed): run `steam steam://install/1826330`.

## Install today (build from `main`)

Until a new release is published, build the current code. You need
[Go 1.26+](https://go.dev/dl/), `git` and `make`.

```bash
git clone https://github.com/Ch3w3y/bellum-linux-installer.git
cd bellum-linux-installer
make release
./dist/bellum-installer-linux-amd64-2.0.1/installer   # can be run from any folder
```

Once a new release is out, you'll be able to download the tarball from
[Releases](https://github.com/Ch3w3y/bellum-linux-installer/releases/latest),
extract it with `tar -xzf`, and run the `installer` inside it.
Check that you're on **this** repository's Releases page; the original
`joepaji/bellum-linux-installer` publishes separate, older builds.

### What happens during install

1. **Choose where to install.** A folder picker opens. Pick the **parent**
   folder: the installer creates a `Bellum` folder inside it (picking `~/Games`
   gives `~/Games/Bellum`).
2. **Prechecks.** The installer detects your GPU and checks tools, the EAC
   runtime and free disk space. Nothing is downloaded or changed yet, and every
   problem is reported together.
3. **Confirm the summary.**
4. **Downloads and prefix setup.** It downloads and verifies Proton (several
   hundred MB, one time only), creates the prefix and installs runtime
   components (Visual C++, .NET 9 and others) through Proton. This takes a
   while.
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

For `--wineprefix`, `WINEPREFIX` and the folder picker alike, `Bellum` is
appended unless the path already ends in a `Bellum` folder. The target folder
must not exist yet, or must be empty. Nothing is created until you confirm the
summary.

If an install fails, the installer removes the folder it created. If it was
interrupted instead (for example, the terminal was closed), just run the
installer again. It recognises the unfinished install and offers to start over.

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
| *EAC runtime digest mismatch* after a Steam update | No workaround yet; wait for a pin update | [#9](https://github.com/Ch3w3y/bellum-linux-installer/issues/9) |
| *SHA-256 mismatch* for the launcher installer after an Astarte update | No workaround yet; wait for a pin update | [#9](https://github.com/Ch3w3y/bellum-linux-installer/issues/9) |
| Install fails at the very end with *unsupported GPU type* (VMs, unrecognised GPUs) | Not supported yet | [#10](https://github.com/Ch3w3y/bellum-linux-installer/issues/10) |

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

The uninstaller removes the prefix, plus the `Bellum` command, desktop
shortcuts and icon when they point at that prefix. Shared Proton is kept,
because other installs may use it. To remove it too:

```bash
rm -rf ~/.local/share/bellum/proton     # shared Proton; only if no other Bellum install uses it
# Installs made before winetricks moved into Proton also left a copy here:
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
