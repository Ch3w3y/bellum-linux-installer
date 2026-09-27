# Easy Anti-Cheat: v2.2.0 (candidate v2.2.0-rc.7)

- **Candidate:** `v2.2.0-rc.7`, commit `59ae4837bfde5a72e010741cc8792e6b08993f4a`
- **Tester:** the maintainer (Ch3w3y); reported 2026-09-27.

## Result

**Passed.** The tester reports that Easy Anti-Cheat "worked fine": the game ran in live online play for **more than 50 minutes** without an EAC error, kick or disconnect, and the session was still running when reported.

| | |
| --- | --- |
| Hardware | AMD Radeon RX 9070 XT (RDNA4), CachyOS, KDE Wayland; see [qa.md](qa.md) |
| Runtime | Proton EasyAntiCheat Runtime installed by Steam (app 1826330), located through the Steam library and passed as `PROTON_EAC_RUNTIME` |
| Proton | `proton-cachyos-11.0-20260703-slr-x86_64` through umu-launcher 1.4.4 |
| Launcher | Astarte Launcher v1.4.2, installed from Linux by Bellum's launcher updater |
| Launch settings | the AMD RDNA4 configuration (`PROTON_FSR4_UPGRADE=1`) |

## Not captured

EAC log excerpts from `drive_c/users/steamuser/AppData/Roaming/EasyAntiCheat/` were not attached. The result rests on the tester's first-hand report of sustained online play, the strongest end-to-end signal that EAC admitted and kept the protected session. Future releases should attach redacted EAC log lines as described in `docs/eac-qa.md`.
