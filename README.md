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

## Install

Open a terminal and paste:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/Ch3w3y/bellum-linux-installer/main/install.sh)
```

Then press **Enter** at each question to accept the defaults. The installer:

1. downloads the latest release from this repository and checks its checksum;
2. provides what it needs itself: the pinned Proton, umu-launcher and
   winetricks are downloaded and verified, and signatures are checked
   in-process. The only host packages involved are `python3` and `flock`,
   which almost every distro already has. If one is missing, it asks once and
   installs it with your package manager;
3. if the Proton EasyAntiCheat Runtime isn't in Steam yet, offers to ask
   Steam to install it and waits until it's done;
4. asks where to install (default `~/Games/Bellum`), then shows one summary
   and asks you to confirm. **Nothing is downloaded or changed before that.**

There are no presets to choose. Bellum gets one configuration: the most
stable one, then the fastest that stays stable, picked for your GPU vendor.
It works the same on X11, Wayland (through XWayland) and gamescope sessions.

Then the Astarte Launcher installer opens; follow its prompts. When it's done
you get a desktop shortcut, an app-menu entry and a `Bellum` command. If the
install fails or is interrupted, just run the same command again.

You need **Steam** installed and signed in. The installer fetches the free
Proton EasyAntiCheat Runtime through it. See [Before you start](#before-you-start).

> ⚠️ **The one-liner needs a release built from the current `main`.** The last
> published release, [`v2.0.1`](https://github.com/Ch3w3y/bellum-linux-installer/releases/tag/v2.0.1)
> (2026-05-13), predates it and uses a different layout. Until the next
> release is published, [build from `main`](#advanced-build-from-main-or-use-a-tarball).

## Project status (2026-09-27)

| | |
| --- | --- |
| **Easy Anti-Cheat on Linux** | ✅ Astarte has enabled Proton/Linux EAC support for Bellum. |
| **One-command install** | ✅ `install.sh` on `main`; it works once the next release is published. |
| **Known blockers** | None open. See [Known issues](#known-issues-and-workarounds). |

## How it works

```
Steam ──────────────► Proton EasyAntiCheat Runtime (app 1826330, stays in your Steam library)
                                   │
installer ─► ~/.local/share/bellum/proton/…  (pinned Proton-CachyOS, SHA-256 verified)
          ─► ~/.local/share/bellum/umu/…     (pinned umu-launcher zipapp, SHA-256 verified)
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

### 1. The Proton EasyAntiCheat Runtime

The installer offers to ask Steam to install it and then waits for Steam to
finish. To do it yourself beforehand: in Steam, open **Library → Tools**, find
**Proton EasyAntiCheat Runtime**, and install it (or open
`steam://install/1826330`).

The installer finds it in native or **Flatpak** Steam, in any of your Steam
library folders. Steam keeps it up to date, and newer builds are accepted. If
you keep it somewhere unusual, point the installer at the folder with
`export PROTON_EAC_RUNTIME="/path/to/Proton EasyAntiCheat Runtime"`.

### 2. Host packages

There's almost nothing to install. The installer downloads and verifies
Proton and umu-launcher itself, checks the launcher's Authenticode signature
in-process, and uses the winetricks that ships with Proton.

| Tool | Why | Required? |
| --- | --- | --- |
| `python3` 3.10+ | Runs the pinned umu-launcher | Yes (preinstalled on nearly every distro, SteamOS and Bazzite included) |
| `flock` (util-linux) | Stops a second launch while Bellum is running | Yes (part of every standard install) |
| `glxinfo` | Better GPU detection (falls back to `lspci` or sysfs without it) | Optional |
| `zenity` or `kdialog` | Graphical folder picker when you type `b` | Optional |
| `wine`, `winetricks`, `umu-launcher`, `osslsigncode`, `wget` | Not needed | No |

If `python3` or `flock` is missing, the one-line install offers to install it
(`pacman`, `dnf`, `apt` or `zypper`). Immutable systems ship both.

## Advanced: build from `main` or use a tarball

Use this if you'd rather not pipe a script into bash, or until the next
release is published. To build the current code you need
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

1. **Choose where to install.** Press Enter for `~/Games/Bellum`, type another
   folder, or type `b` to open a folder picker. A `Bellum` folder is created
   inside the folder you give unless it already ends in `Bellum` (so `~/Games`
   gives `~/Games/Bellum`).
2. **Prechecks.** The installer detects your GPU and checks tools, the EAC
   runtime and free disk space. Nothing is downloaded or changed yet, and every
   problem is reported together.
3. **Confirm the summary.** It shows the configuration picked for your GPU.
4. **Downloads and prefix setup.** It downloads and verifies umu-launcher and Proton (several
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
./installer --yes                          # accept every default (~/Games/Bellum)
./installer --wineprefix ~/Games          # skip the location question (creates ~/Games/Bellum)
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

**Optional extras.** These are off, because none of them has been validated
with Easy Anti-Cheat. To try one, set it to `1` in `<prefix>/launch_vars.env`:

```bash
export BELLUM_MANGOHUD=1   # performance overlay (needs mangohud)
export BELLUM_GAMEMODE=1   # Feral GameMode while playing (needs gamemode)
export BELLUM_GAMESCOPE=1  # run inside gamescope (needs gamescope)
export BELLUM_VKBASALT=1   # vkBasalt post-processing (needs vkbasalt)
```

**Logs.** The game and launcher log to `<prefix>/launcher.log`. Installer logs
are in `logs/` next to the `installer` binary; with the one-liner that's
`~/.local/share/bellum-installer/<version>/logs/installer.log`. Every error
message ends with the log's exact path.

## Graphics and upscalers

- **The installer never adds, copies or replaces DLLs** in the game directory
  or prefix. Don't hand-place proxy DLLs yourself either (ReShade or OptiScaler
  `dxgi.dll`/`d3d11.dll` and similar). Anything in the game folder loads into
  the process Easy Anti-Cheat protects.
- DXVK, vkd3d-proton and dxvk-nvapi come from the pinned Proton-CachyOS build.
  See [runtime pins](docs/runtime-pins.md).
- **NVIDIA:** DLSS is available through Proton's defaults (NVAPI on, and
  `nvngx.dll` taken from your NVIDIA driver). RTX and GTX 16-series cards also
  get `PROTON_NVIDIA_LIBS=1` (CUDA, NVENC and OptiX bridges). Proton's DLSS
  DLL upgrade, which downloads DLLs at launch, is off. GTX 16-series cards
  have no DLSS.
- **AMD:** the pinned Proton downloads AMD's FSR4 driver component at every
  launch and offers FSR4 to D3D12 games that use AMD's FidelityFX API (FSR 3.1
  or later) when your GPU supports it. This is Proton's built-in behaviour, not
  something the installer does. On RDNA4 the installer also sets
  `PROTON_FSR4_UPGRADE=1`, which forces the offer. Set `PROTON_FSR4_INDICATOR=1`
  in `launch_vars.env` to see an on-screen FSR watermark confirming it's
  active. [Details](docs/runtime-pins.md#upscaler-behaviour-of-the-pinned-proton).
- **Display server:** the settings are the same on X11, Wayland and gamescope.
  The pinned Proton uses XWayland unless its experimental Wayland driver is
  switched on (`PROTON_ENABLE_WAYLAND`), which the installer doesn't do.
  Proton turns on fsync by itself where the kernel supports it.
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
| GPU shown as *Unknown* (VMs, virtio-gpu, some hybrid laptops) | The install continues with generic Proton settings, without vendor-specific features. Install `glxinfo` for better detection | — |
| `Bellum: command not found` | The installer prints the command that adds `~/.local/bin` to your `PATH` for your shell | — |

## Updating

Run the same one-line command again. On an existing install it offers to
update to the latest tested Proton and umu-launcher. Your game, launcher
login and saves are kept. This project's CI checks new Proton-CachyOS and
umu-launcher releases daily and pins each one once it passes the checks (see
[runtime pins](docs/runtime-pins.md#automated-pin-updates)), so updating
every so often picks up upstream performance and stability fixes.

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
