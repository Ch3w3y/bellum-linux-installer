# QA: v2.2.0 (candidate v2.2.0-rc.7)

- **Candidate:** `v2.2.0-rc.7`, commit `59ae4837bfde5a72e010741cc8792e6b08993f4a`
- **Tester:** the maintainer (Ch3w3y), on their own hardware; recorded 2026-09-27 by Claude from the tester's reports and logs.

## Hardware and software

| | |
| --- | --- |
| GPU | AMD Radeon RX 9070 XT (RDNA4, `gfx1201`), radeonsi / ACO |
| Kernel | `7.2.7-1-cachyos-server` (from the installer's GPU probe, DRM 3.64) |
| Mesa | not reported |
| Distro and desktop | CachyOS, KDE Plasma, Wayland (the game runs through XWayland) |
| Steam | native, Proton EasyAntiCheat Runtime in `~/.local/share/Steam` |
| Install folder | `~/Games/Bellum` on NVMe |

## Results

The candidate series was tested end to end. Each failure was fixed and the fix re-tested on the next candidate:

| Candidate | Result |
| --- | --- |
| rc.1 | EAC runtime installed through Steam and RDNA4 detected, then Proton extraction refused its relative symlinks → fixed in #29 |
| rc.2 | install reached configuration; silent registry commands failed (exit 120) → fixed in #30 |
| rc.3 | **full install completed** (Proton, prefix, winetricks verbs, launcher installer, configuration); the Astarte Launcher then looped on self-update under Wine → fixed in #32 |
| rc.4 | the update onto an existing install worked; the launcher opened without looping, sign-in worked and the game downloaded through the launcher |
| rc.7 | the tester was asked to update onto rc.7 and play online. They reported the game running with EAC for 50+ minutes (see [eac.md](eac.md)). |

Checklist items (see `docs/eac-qa.md`):

- [x] The banner shows the version and pins, and the B logo renders in colour ("excellently").
- [x] The system check reported RDNA4, the Wayland KDE session and the EAC runtime correctly.
- [x] The install finished and the shortcut was created (rc.3); updating an existing install worked (rc.4, rc.7).
- [x] The launcher opens once, with no update loop, after launcher updates moved to Linux (rc.4 onwards).
- [x] Sign-in works and the game downloads through the launcher.
- [x] The game runs from the shortcut (see [eac.md](eac.md)).
- [ ] Prefix and log permissions, WebView2 presence, second-click and quit behaviour: not individually reported. The installer checks WebView2 itself, and wrapper tests cover second-click behaviour.

## What differs between the tested builds and the candidate

The hardware fixes above are all in rc.4. rc.4 → rc.7 changed:
- release tooling (the gate, #33);
- documentation and screenshots (#34);
- the MIT license, a stricter `install.sh` version check, and control-character rejection in the install path (#35).

None of these changes the launch path, the Proton settings or the prefix contents.
