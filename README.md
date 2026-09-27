# Bellum Linux Installer

Welcome to the Bellum Linux Installer and Uninstaller. This is a linux Proton/Wine based install orchestrator for Bellum.
Please note, this is NOT official support or an official native release. I am just a supporter of the game and am not affiliated with Astarte Industries.
That said, my goal is making sure none of my fellow linux gamers have to see the light of Windows to play this awesome game.

## Download & Install

### Unpack the Release Package

1. Download the release tarball from the [Releases Page](https://github.com/joepaji/bellum-linux-installer/releases/latest)

2. Open a terminal in the directory where you downloaded the release tarball

3. Extract the release tarball & access extracted directory:

```bash
tar -xzf bellum-installer-linux-amd64-v2.0.1.tar.gz
cd bellum-installer-linux-amd64-v2.0.1
```

### Install Game

1. Run the installer:

```bash
./installer
```


The launcher never copies DLLs into the game directory. The pinned CachyOS
Proton runtime automatically stages `amdxcffx64.dll` for supported discrete
RDNA2-RDNA4 GPUs. Bellum explicitly requests `PROTON_FSR4_UPGRADE=1` only for
an unambiguous RDNA4 adapter. The runtime does not provide a deterministic
environment switch to prevent its automatic RDNA3 staging, so Bellum cannot
promise that native FSR3/4 paths remain unchanged on RDNA3. Runtime FSR and
DLSS replacement downloads remain disabled. The generated launcher uses
umu-launcher, Proton, and the Proton EasyAntiCheat Runtime for every GPU.
Set `PROTON_EAC_RUNTIME` to the installed runtime directory if it is outside
Steam's default location. The launcher logs to `launcher.log` in the prefix;
check that log for EAC initialization when validating Linux mode.
See [the EAC QA checklist](docs/eac-qa.md) for the evidence to record.


2. Select the directory where you want to install Bellum and confirm the install summary. A WINEPREFIX named `Bellum` will be created in the selected directory.
   
<img width="800" alt="image" src="https://github.com/user-attachments/assets/826d7e36-1471-4cd2-9c61-8440252456aa" />
<img width="800" alt="image" src="https://github.com/user-attachments/assets/5347c5bd-c44d-4f37-b89b-cdbf4e137ae9" />


3. Let the installer do its thing until the AstarteLauncher pops up.
4. Install AstarteLauncher by following the instructions in the launcher.
<img width="800"  alt="image" src="https://github.com/user-attachments/assets/5ba0340b-9d2a-45f4-954c-83befd331534" />

5. Let the installer complete some post launcher install steps and you're done!

<img width="800" alt="image" src="https://github.com/user-attachments/assets/aab9f336-9307-4e2a-b7da-f5f8655eb92b" />


**Note:** `WINEPREFIX` environment variable can also be used to install Bellum:

```bash
export WINEPREFIX=/path/to/wineprefix
./installer
```

## Playing the Game

### Option 1 - Desktop Shortcut
This guy will be added to your Desktop once install is complete:

<img width="106" height="117" alt="image" src="https://github.com/user-attachments/assets/1305bf0b-994f-4726-a75f-64e5f5bbac7e" />

### Option 2 - Application Menu
This guy will be added under the **Games** category in your Application Menu:

<img width="537" height="67" alt="image" src="https://github.com/user-attachments/assets/d6acdfed-7569-415d-8e42-dac896d7bce9" />

### Option 3 - Terminaal
Just open a terminal anywhere, and run the `Bellum` command.

<img width="800" alt="image" src="https://github.com/user-attachments/assets/e24b60bc-7aaa-4fc8-99ff-26ed24fbe7e7" />

## Uninstallation

Set the `WINEPREFIX` env var to the one used to install the game. Then run unintsaller script.
```bash
export WINEPREFIX=/path/to/wineprefix
./uninstaller
```

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
make release VERSION=2.1.0       # reproducible tarball + MANIFEST + SHA256SUMS
make verify-release              # verify staged checksums
```

See [RELEASE.md](RELEASE.md) for the full release procedure, the release gate
(QA + EAC + green CI + reproducibility + provenance checks before any RC tag),
and the binary/provenance policy.

## NVIDIA Blackwell (RTX 5000 series) driver note

The 595 driver branch has multiple community-reported regressions on Blackwell under Wine/Proton, and reports of UE5 problems on earlier branches (580 and 590 included) also exist. Examples: 595.71.05 fails Vulkan swapchain creation under Proton where 595.58.03 worked ([NVIDIA forum](https://forums.developer.nvidia.com/t/regression-595-71-05-blackwell-rtx-5070-vulkan-swapchain-creation-fails-vk-error-initialization-failed-under-proton-worked-on-595-58-03/371778)), and 595 performance regressions ([CachyOS #378](https://github.com/CachyOS/distribution/issues/378)).

If Bellum fails to load shaders or renders a black screen on a 5000 series GPU with a 595 driver, try 595.58.03 or the 590 branch. No driver branch is validated by the Bellum project itself; this is guidance from public reports, not a guarantee.

## Implementation Notes

- No DLLs are copied into the game directory. The pinned CachyOS Proton runtime auto-stages AMD's `amdxcffx64.dll` for supported RDNA2-RDNA4 discrete GPUs; this is independent of Bellum's RDNA4-only `PROTON_FSR4_UPGRADE=1` request. RDNA3 auto-staging cannot be deterministically disabled through the documented runtime settings. See [runtime pins](docs/runtime-pins.md).
- FSR upscaling and ML frame generation are separate capabilities. NVIDIA RTX 40 series has single frame generation; RTX 50 adds multi-frame generation. GTX 16 has no DLSS. Availability still depends on the runtime, driver, and game implementation; Bellum has not validated these features in-game.
- DXVK, vkd3d-proton and dxvk-nvapi come from the pinned CachyOS Proton runtime (see [runtime pins](docs/runtime-pins.md)).
- The launcher uses umu-launcher with the Proton EasyAntiCheat Runtime (see [EAC QA](docs/eac-qa.md)).
- The installer and uninstaller are Go binaries; packages are bundled in the release tarball, not embedded.
- All install logging is written to `logs/installer.log`; the uninstaller writes `uninstaller.log`.
- The uninstaller removes the Bellum-owned prefix after confirmation and retains shared Proton and user-wide launcher files.
