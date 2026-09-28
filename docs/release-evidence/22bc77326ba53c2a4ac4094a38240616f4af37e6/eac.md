# Easy Anti-Cheat: v2.3.0 (candidate v2.3.0-rc.1)

- **Candidate:** `v2.3.0-rc.1`, commit `22bc77326ba53c2a4ac4094a38240616f4af37e6`; the tested build (`13ce01a`) has the same tree (see [qa.md](qa.md)).
- **Tester:** the maintainer (Ch3w3y); reported 2026-09-28.

## Result

**Passed.** The tester reports Easy Anti-Cheat working in online matches, with everything working after about **10 hours** of use and no EAC error, kick or disconnect reported.

| | |
| --- | --- |
| Hardware | Valve Steam Deck, SteamOS 3; see [qa.md](qa.md) |
| Launch paths | Game Mode through Steam (non-Steam game entry) and Desktop Mode |
| Runtime | Proton EasyAntiCheat Runtime installed by Steam (app 1826330), passed as `PROTON_EAC_RUNTIME` |
| Proton | `proton-cachyos-11.0-20260703-slr-x86_64` through umu-launcher 1.4.4 (unchanged from v2.2.0) |
| Launch settings | the `amd-baseline` profile: `PROTON_FSR4_UPGRADE="0"`, no `DXIL_SPIRV_CONFIG` |

## Not captured

Redacted EAC log lines from `drive_c/users/steamuser/AppData/Roaming/EasyAntiCheat/` were not attached. As for v2.2.0, the result rests on the tester's first-hand report of sustained online play, here about ten hours across both launch paths.
