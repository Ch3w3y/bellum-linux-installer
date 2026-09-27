# Hardware and Easy Anti-Cheat QA

Every final release needs QA and EAC evidence from a real install of the
release candidate it ships (see [RELEASE.md](../RELEASE.md)). CI can't provide
this: it never runs the installer, and Easy Anti-Cheat only runs on real
hardware with a real account. Astarte has enabled Proton/Linux EAC support for
Bellum; this checklist confirms that each release's configuration works with
it.

## Install the candidate

```bash
curl -fsSL https://raw.githubusercontent.com/Ch3w3y/bellum-linux-installer/main/install.sh | bash -s -- --version vX.Y.Z-rc.N
```

Test a fresh install where possible, and an update of an existing install
(run the same command on it).

## Checklist

**Install**

- [ ] The banner shows the candidate version and the pinned Proton,
  umu-launcher and winetricks versions.
- [ ] The system check reports the GPU (vendor and generation), the display
  session and the Proton EasyAntiCheat Runtime correctly.
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

## Recording the evidence

Write one record per item in `docs/release-evidence/<candidate SHA>/`
(`qa.md`, `eac.md`) and link it from [RELEASE-GATE.md](../RELEASE-GATE.md).
Include:

- the candidate tag and full commit SHA;
- the date, and who tested;
- hardware and software: CPU, GPU, driver (Mesa or NVIDIA version), distro and
  kernel, desktop and session (X11, Wayland or gamescope), and whether Steam is
  native or Flatpak;
- the Proton EasyAntiCheat Runtime build (Steam shows it; the installer logs
  its digest);
- each checklist item as passed, failed or not tested, with notes;
- short, **redacted** log excerpts.

Mark anything missing or ambiguous as inconclusive rather than passed.

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
