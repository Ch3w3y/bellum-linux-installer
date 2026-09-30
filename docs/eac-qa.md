# Hardware and Easy Anti-Cheat QA

What to try when testing a release or release candidate on real hardware
(see [docs/releasing.md](releasing.md)). CI can't do this: it never runs the
installer, and Easy Anti-Cheat only runs on real hardware with a real account. Astarte has enabled Proton/Linux EAC support for
Bellum; this checklist confirms that each release's configuration works with
it.

## Install the candidate

```bash
curl -fsSL https://raw.githubusercontent.com/Ch3w3y/bellum-linux-installer/main/install.sh | bash -s -- --version vX.Y.Z-rc.N
```

Test a fresh install where possible, and an update of an existing install
(run the same command on it).

## Profiles

Since v2.3 the installer picks a launch profile from the detected platform
and marks it **verified** or **expected** (see the README's
[Platforms](../README.md#platforms)). A profile becomes verified only once
someone has played on real hardware running it, with an Easy Anti-Cheat
online session; untested profiles ship as expected, with settings no riskier
than the nearest verified one. When reporting a test, include the review
screen's **Detected** line.

| Profile | Controller check | Status |
| --- | --- | --- |
| RDNA4 · Arch/CachyOS · KDE Wayland | desktop pad, Steam closed and running | verified in v2.2.0 (controller not yet tested) |
| Steam Deck (LCD or OLED) · SteamOS · Game Mode and Desktop | built-in controls through Steam Input | needs a tester |
| Steam Machine · SteamOS | Steam Controller through Steam Input | needs a tester |
| RDNA2 or RDNA3 desktop · Bazzite | desktop pad | needs a tester |
| NVIDIA RTX · Arch (Omarchy) · Hyprland Wayland | — | verified in v2.4.0 (RTX 5070 Ti, driver 610.57.04; controller not yet tested) |
| NVIDIA RTX · Fedora or Ubuntu | desktop pad | needs a tester |
| Ubuntu with Snap Steam | — | needs a tester (Snap EAC runtime) |

## Checklist

**Install**

- [ ] The banner shows the candidate version and the pinned Proton,
  umu-launcher and winetricks versions.
- [ ] The system check reports the GPU (vendor and generation), the display
  session and the Proton EasyAntiCheat Runtime correctly.
- [ ] The review screen's **Detected** line matches the machine (hardware,
  distro, GPU generation, session) and names the expected status.
- [ ] NVIDIA only: any driver warning is accurate and its fix command is right
  for this distribution; a healthy driver gives no warning.
- [ ] `launch_vars.env` matches the profile: RDNA4 has
  `PROTON_FSR4_UPGRADE="1"`; RDNA3 has `DXIL_SPIRV_CONFIG=wmma_rdna3_workaround`;
  RDNA2 and older AMD have neither.
- [ ] The install finishes with *Bellum is installed*, with no `✖` lines.
- [ ] The launcher update step reports `Astarte Launcher vX.Y.Z is up to date`
  or `updated to vX.Y.Z`.
- [ ] `<prefix>` is mode `0700`; `launch_vars.env` and `launcher.log` are
  `0600`.
- [ ] WebView2 is installed: `msedgewebview2.exe` exists under
  `<prefix>/drive_c/Program Files (x86)/Microsoft/EdgeWebView/Application/`
  (or `Program Files`). The installer checks this itself.

**Launcher**

- [ ] The shortcut opens the Astarte Launcher once, with no update-and-restart
  loop. `launcher.log` shows the launcher-update line before the launcher
  starts.
- [ ] Signing in works and the game downloads through the launcher.
- [ ] A second click on the shortcut while it's running returns at once
  (*Bellum is already running*).
- [ ] Quitting the launcher ends the session, and the next click starts it
  again.

**Easy Anti-Cheat**

- [ ] The game starts from the launcher, and the EAC splash or initialisation
  passes.
- [ ] Joining an **online match** works, and it lasts for a meaningful session
  (note how long) without an EAC error, kick or disconnect.
- [ ] Fresh EAC logs under
  `<prefix>/drive_c/users/steamuser/AppData/Roaming/EasyAntiCheat/` show the
  Linux module loading and the session being admitted. Match the IDs below;
  the IDs alone don't prove success.
- [ ] The game stays running until you quit it.

**Steam Deck and Steam Machine**

- [ ] The review screen names the device (`Steam Deck LCD`, `Steam Deck OLED`
  or `Steam Machine (provisional)`). For a Steam Machine, record
  `/sys/class/dmi/id/product_name` and the `glxinfo -B` renderer string.
- [ ] Installing to a microSD card shows the microSD note; installing to the
  internal SSD doesn't.
- [ ] With Steam closed, *Add Bellum to Steam* adds the entry, and a
  `shortcuts.vdf.bellum-backup-*` file appears. With Steam running, it
  refuses and leaves `shortcuts.vdf` unchanged.
- [ ] In **Game Mode**, Bellum starts from the library. The launcher is usable
  with the trackpad (as a mouse) and **Steam + X** keyboard for sign-in, and
  the Play button works.
- [ ] The game's menus and gameplay work on the built-in controls.
- [ ] The Steam button and Quick Access overlay work over the game.
- [ ] Suspend and resume mid-session keep input and the EAC session (note what
  happens).
- [ ] The uninstaller removes the Steam entry (Steam closed).

**Controllers (desktop)**

- [ ] Xbox, PlayStation and 8BitDo pads (whichever you have), started from the
  desktop shortcut with Steam **closed**: menus and gameplay; rumble.
- [ ] The same with Steam **running**: note any double input; `launcher.log`
  shows the double-input note.
- [ ] Started from Steam as a non-Steam game: the pad works through Steam
  Input; no double input.
- [ ] Hot-plugging a pad mid-session.
- [ ] Easy Anti-Cheat is unaffected by the input path.

**Graphics (optional but useful)**

- [ ] AMD: with `PROTON_FSR4_INDICATOR=1` in `launch_vars.env`, the FSR
  watermark appears when FSR is selected in game.
- [ ] Rough performance notes: resolution, preset, upscaler, typical FPS.

EAC identifiers for Bellum: product `087dc666152349c68aa8e1962237c472`,
sandbox `84c3e73046e546d282c07eee30ac3162`, deployment
`1ec8679293294023bb158112821a4041`.

## Useful commands

```bash
# Launcher, updates and game output for the last launch
tail -n 200 ~/Games/Bellum/launcher.log

# The EAC and umu lines only
grep -iE 'umu|proton|easyanticheat|eac|anti.cheat' ~/Games/Bellum/launcher.log

# More detail on the next launch
PROTON_LOG=1 UMU_LOG=1 Bellum
```

## Privacy

The prefix holds your launcher login, access certificates and WebView2
cookies. Before sharing logs or screenshots, remove usernames, account and
session IDs, tokens, certificate material and cookies.

## EAC runtime updates

Steam updates the Proton EasyAntiCheat Runtime (app `1826330`) on its own.
The installer requires all six runtime files. If their combined digest is new,
it logs the digest as a warning and continues. After a candidate passes this
checklist on that runtime, add the digest to `EACRuntimeSHA256Allowlist` in
`pkg/config/versions.go`.
