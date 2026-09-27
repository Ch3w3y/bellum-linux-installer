# Bellum Linux Installer

Welcome to the Bellum Linux Installer and Uninstaller. This is a linux Proton/Wine based install orchestrator for Bellum.
Please note, this is NOT official support or an official native release. I am just a supporter of the game and am not affiliated with Astarte Industries.
That said, my goal is making sure none of my fellow linux gamers have to see the light of Windows to play this awesome game.

## Before You Start: First-Run Prerequisites

### Host packages

The installer checks for these tools and tells you what's missing and how to
install it for your distribution (see `pkg/workflow/host.go`); it never runs a
package manager for you. Install them yourself first to avoid a mid-install
stop:

- **Required on every distro:** Wine, `umu-run` (umu-launcher), and `wget`.
- **Optional:** `kdialog` (KDE) or `zenity` (GNOME/other) for the graphical
  directory picker. Without either, the installer falls back to a plain
  terminal prompt.

Per-family package names:

| Family | Package manager | Packages |
| --- | --- | --- |
| Arch / Manjaro / EndeavourOS | `pacman -S` | `wine winetricks umu-launcher wget mesa-demos` |
| Fedora / RHEL / CentOS | `dnf install` | `wine winetricks umu-launcher wget glx-utils` |
| Debian / Ubuntu / Mint / Pop!_OS | `apt install` | `wine winetricks umu-launcher wget mesa-utils` |
| openSUSE | `zypper install` | `wine winetricks umu-launcher wget Mesa-demo-x` |

Add `kdialog` or `zenity` from the same package manager if you want the GUI
picker.

### Proton EasyAntiCheat Runtime

Bellum runs under Easy Anti-Cheat. Before installing, get the **Proton
EasyAntiCheat Runtime** (Steam app `1826330`) onto your system: in Steam, go
to **Library → Tools**, find "Proton EasyAntiCheat Runtime", and install it.
The generated launcher expects this runtime; set `PROTON_EAC_RUNTIME` to its
directory if it's installed somewhere other than Steam's default location.

### Immutable-host limits

On immutable/atomic hosts (e.g. SteamOS, Bazzite), the installer will not
attempt to install missing packages itself — its dependency guidance is
informational only, and you're responsible for installing packages through
that host's supported workflow (e.g. a distrobox/toolbox container or the
host's own package layering mechanism) before running the installer.

### Recovery from a failed or partial install

If the installer fails after it has already created a new WINEPREFIX, it
removes that newly created prefix automatically; it never deletes a
pre-existing directory you pointed it at. Check `logs/installer.log` in the
directory you ran the installer from for the failure reason, resolve it (for
example, install a missing host package), and re-run `./installer` — it will
create a fresh prefix. If you reused an *existing* prefix (`UseExisting`) and
the install failed partway through, that prefix is left as-is; back it up
before retrying if it may contain data you care about.

## Download & Install

### Unpack the Release Package

1. Download the release tarball from the [Releases Page](https://github.com/Ch3w3y/bellum-linux-installer/releases/latest)
   (this is the repository you are reading right now; `v2.0.1` is its latest
   release as of 2026-09-27 — a different fork, `joepaji/bellum-linux-installer`,
   publishes its own independent releases under the same project name, so
   double-check you're on this repo's Releases page).

2. Open a terminal in the directory where you downloaded the release tarball

3. Extract the release tarball & access extracted directory (substitute the
   version you actually downloaded if it differs from the example below):

```bash
tar -xzf bellum-installer-linux-amd64-v2.0.1.tar.gz
cd bellum-installer-linux-amd64-v2.0.1
```

### Install Game

1. Run the installer:

```bash
./installer
```


Bellum plays under Easy Anti-Cheat, so the installer treats the game
directory and any upscaler DLL as things it should not touch without
developer approval and a live protected-session test: no DLLs are ever
copied into the game directory, and DLSS replacement/NGX-updater are off by
default on every GPU. This is a safety default, not proof of EAC
compatibility — Steam requires the game developer to separately enable
Proton/Linux support for anti-cheat, and no live evidence of Bellum's Linux
EAC enablement is published as of 2026-09-27
([Steamworks Proton anti-cheat instructions](https://partner.steamgames.com/doc/steamhardware/proton)).
FSR is different: on AMD RDNA4 GPUs the installer opts in to Proton's FSR4
upscaling component (`PROTON_FSR4_UPGRADE=1`) rather than leaving upscaling
untouched — see [Implementation Notes](#implementation-notes) below for what
that does and does not guarantee. The generated launcher uses umu-launcher,
Proton, and the Proton EasyAntiCheat Runtime for every GPU. Set
`PROTON_EAC_RUNTIME` to the installed runtime directory if it is outside
Steam's default location. The launcher logs to `launcher.log` in the prefix;
check that log for EAC initialization when validating Linux mode.
See [the EAC QA checklist](docs/eac-qa.md) for the evidence to record.

**EAC hygiene — don't hand-place proxy DLLs:** Do not manually copy a proxy
DLL (a ReShade `dxgi.dll`/`d3d11.dll`, an OptiScaler `dxgi.dll`, or similar)
into the game directory yourself. Anything sitting there loads into the same
protected process EAC watches, and Bellum has no way to know it's benign.
The installer's own refusal to write DLLs into the game directory
(`GuardGameTreeWrites`) is a regression guard against the installer
accidentally corrupting or overwriting game files during install/uninstall —
it is **not** a sandbox and does not stop you, another program, or a game
update from placing files there, and it does not vouch for anything you add
yourself.


2. Select the **parent** directory where you want to install Bellum and confirm the install summary. The installer appends `Bellum` to your selection, so a WINEPREFIX named `Bellum` is created inside the directory you pick (e.g. picking `~/Games` creates `~/Games/Bellum`).
   
<img width="800" alt="image" src="https://github.com/user-attachments/assets/826d7e36-1471-4cd2-9c61-8440252456aa" />
<img width="800" alt="image" src="https://github.com/user-attachments/assets/5347c5bd-c44d-4f37-b89b-cdbf4e137ae9" />


3. Let the installer do its thing until the AstarteLauncher pops up.
4. Install AstarteLauncher by following the instructions in the launcher.
<img width="800"  alt="image" src="https://github.com/user-attachments/assets/5ba0340b-9d2a-45f4-954c-83befd331534" />

5. Let the installer complete some post launcher install steps and you're done!

<img width="800" alt="image" src="https://github.com/user-attachments/assets/aab9f336-9307-4e2a-b7da-f5f8655eb92b" />


**Note:** `WINEPREFIX` environment variable can also be used to install Bellum.
Unlike the directory picker or the `--wineprefix` flag, the env var is used
exactly as given — it is **not** appended with `Bellum` — so point it at the
prefix itself:

```bash
export WINEPREFIX=/path/to/Bellum
./installer
```

## Playing the Game

### Option 1 - Desktop Shortcut
This guy will be added to your Desktop once install is complete:

<img width="106" height="117" alt="image" src="https://github.com/user-attachments/assets/1305bf0b-994f-4726-a75f-64e5f5bbac7e" />

### Option 2 - Application Menu
This guy will be added under the **Games** category in your Application Menu:

<img width="537" height="67" alt="image" src="https://github.com/user-attachments/assets/d6acdfed-7569-415d-8e42-dac896d7bce9" />

### Option 3 - Terminal
Just open a terminal anywhere, and run the `Bellum` command.

<img width="800" alt="image" src="https://github.com/user-attachments/assets/e24b60bc-7aaa-4fc8-99ff-26ed24fbe7e7" />

## Uninstallation

Set the `WINEPREFIX` env var to the prefix used to install the game (used
literally, same as install — no `Bellum` is appended). Then run the
uninstaller script:
```bash
export WINEPREFIX=/path/to/Bellum
./uninstaller
```

Alternatively, run `./uninstaller` with no arguments and no `WINEPREFIX` set
to use the GUI picker — for uninstall, **select the `Bellum` prefix directory
itself** (e.g. `~/Games/Bellum`), not its parent. This is the opposite of the
install picker, which selects the parent directory.

The installer writes `.bellum-manifest.json` inside each new prefix to identify
the Bellum-owned instance. Uninstall verifies that manifest, shows the resolved
prefix, and requires an explicit `y` before deleting that prefix; Enter defaults
to cancel. Run `./uninstaller --wineprefix /path/to/Bellum --dry-run` to inspect
the target without changing files. Uninstall retains shared Proton and
user-wide launcher files, which may be used by other Bellum instances. Back up
the entire prefix first if it contains saves, credentials, or other data you
want to keep. If installation fails after creating a new prefix, the installer
removes that newly created prefix; it leaves pre-existing directories intact.

## Release Tarball Structure

The release tarball (`bellum-installer-linux-amd64-v2.0.1.tar.gz`) contains:

```
bellum-installer-linux-amd64-v2.0.1.tar.gz
├── installer          # Installer binary
├── uninstaller        # Uninstaller binary
├── MANIFEST.md        # Versioned manifest (per-file sha256 + sizes)
├── SHA256SUMS         # Checksums for all staged files
└── packages/          # All bundled packages
```

## Building and verifying releases

CI runs gofmt/go vet/go test, module-pinning checks, and compile-only Linux
builds (amd64/arm64) on every PR — the installer itself is never executed by
CI. Release archives are reproducible and ship a versioned `MANIFEST.md` and
`SHA256SUMS`.

```bash
make check                       # same checks CI runs
make release VERSION=2.0.2       # reproducible tarball + MANIFEST + SHA256SUMS
                                  # (VERSION defaults to 2.0.1 if omitted; pass
                                  # the version you are actually cutting)
make verify-release              # verify staged checksums
```

See [RELEASE.md](RELEASE.md) for the full release procedure, the release gate
(QA + EAC + green CI + reproducibility + provenance checks before any RC tag),
and the binary/provenance policy.

## NVIDIA driver note

As of 27 September 2026, NVIDIA lists **595.104.02** as its current Linux
production driver and **615.71.09** as its current new-feature driver
([NVIDIA Unix driver archive](https://www.nvidia.com/en-us/drivers/unix/)).
Some users have reported a Blackwell (RTX 5000 series) Vulkan swapchain
failure on 595.71.05 that did not reproduce on 595.58.03
([NVIDIA forum report](https://forums.developer.nvidia.com/t/regression-595-71-05-blackwell-rtx-5070-vulkan-swapchain-creation-fails-vk-error-initialization-failed-under-proton-worked-on-595-58-03/371778),
2026-05-30), and separate game-specific UE5/Proton regressions on
615.71.09 have been reported for several titles
([615 release feedback thread](https://forums.developer.nvidia.com/t/615-release-feedback-discussion/382815),
2026-09). Neither issue has a confirmed vendor fix version as of this
writing, and Bellum itself has not validated any driver branch.

Start with a current driver supported by your distribution. If a specific
update causes a reproducible failure (shader load failure, black screen, or
crash), compare against a known-working earlier version on your system and
report the GPU, driver version, compositor, Proton version, and logs. There
is no universally-safe rollback version for every GPU/driver combination —
590/595.58.03 is not blanket current advice, and neither is treating 615.x
as a guaranteed-safe upgrade.

## Implementation Notes

- Bellum's own upscaler DLL replacement is off by default on every GPU, and no DLLs are ever copied into the game directory — that boundary is enforced for EAC safety, independent of upscaler choice.
- On AMD RDNA4 GPUs, the installer sets `PROTON_FSR4_UPGRADE=1` to opt in to Proton's FSR4 upscaling component; on RDNA3 it sets `PROTON_FSR4_RDNA3_UPGRADE=0`, intending to keep RDNA3 upgraded FSR off. However, the pinned CachyOS Proton runtime (`proton-cachyos-11.0-20260703-slr`, see [runtime pins](docs/runtime-pins.md)) independently stages an AMD FSR driver DLL in the prefix and, per its own release notes, no longer requires `PROTON_FSR4_UPGRADE` except to request a specific version, and permits FSR4 on supported RDNA2–4 discrete GPUs regardless of that flag. `PROTON_FSR4_RDNA3_UPGRADE` was removed from that CachyOS release entirely. In practice this means Bellum's flags express an intent (FSR4 requested on RDNA4, not requested on RDNA3) but **do not prove** the pinned runtime honors them, and shipped-upscaler-only behavior on RDNA3 is not guaranteed by this installer. Treat upscaler behavior as unverified for Bellum until a live protected-session test confirms it. ([CachyOS 11.0-20260702 SLR release notes](https://github.com/CachyOS/proton-cachyos/releases/tag/cachyos-11.0-20260702-slr), 2026-07-12)
- DXVK, vkd3d-proton and dxvk-nvapi come from the pinned CachyOS Proton runtime (see [runtime pins](docs/runtime-pins.md)). `PROTON_ENABLE_NVAPI=1` is set for every GPU; RTX 20/30 support DLSS Super Resolution only (no Frame Generation), RTX 40 adds single Frame Generation, and RTX 50 adds Multi Frame Generation — availability still depends on the runtime, your driver, and whether Bellum's build integrates DLSS/Streamline at all, which has not been validated. Bellum's own DLSS replacement DLL and NVIDIA's NGX updater stay off by default.
- The launcher uses umu-launcher with the Proton EasyAntiCheat Runtime (see [EAC QA](docs/eac-qa.md)).
- The installer and uninstaller are Go binaries; packages are bundled in the release tarball, not embedded.
- All install logging is written to `logs/installer.log`; the uninstaller writes `uninstaller.log`.
- The uninstaller removes the Bellum-owned prefix after confirmation and retains shared Proton and user-wide launcher files.
