# QA: v2.3.0 (candidate v2.3.0-rc.1)

- **Candidate:** `v2.3.0-rc.1`, commit `22bc77326ba53c2a4ac4094a38240616f4af37e6` (the merge of #43)
- **Tester:** the maintainer (Ch3w3y), on their own Steam Deck; recorded 2026-09-28 by Claude from the tester's report.

## The tested build is the candidate

The tester ran the head of #43, commit `13ce01a872131153ddfd8f176dbbcefc31c0ce53`, before it was merged. Merging it into `main` (which it already contained) produced the candidate without changing a file: both commits have the tree `545ac1b92cb7ebd33ef7569108be250139a838a0`.

```
git rev-parse 13ce01a872131153ddfd8f176dbbcefc31c0ce53^{tree}   # 545ac1b92cb7ebd33ef7569108be250139a838a0
git rev-parse 22bc77326ba53c2a4ac4094a38240616f4af37e6^{tree}   # 545ac1b92cb7ebd33ef7569108be250139a838a0
```

## Hardware and software

| | |
| --- | --- |
| Device | Valve Steam Deck (LCD or OLED model not reported) |
| GPU | the Deck's AMD APU (RDNA2, `gfx1033`); launch profile `amd-baseline` (no FSR4 emulation path) |
| OS | SteamOS 3 (exact version not reported) |
| Sessions | Game Mode (gamescope) and Desktop Mode |
| Steam | native (SteamOS) |

## Results

The tester reports everything working over about **10 hours** of use.

Checklist items (see `docs/eac-qa.md`):

- [x] The review screen's **Detected** line identified the Steam Deck.
- [x] The install completed and Bellum started.
- [x] **Game Mode, started from Steam** as a non-Steam game, playable with the Deck's built-in controls through Steam Input.
- [x] **Desktop Mode**, started from the desktop shortcut or the `Bellum` command.
- [x] Online matches with Easy Anti-Cheat (see [eac.md](eac.md)).
- [ ] Not individually reported: the Deck model and SteamOS version, whether the entry was added by the installer's opt-in or by hand, microSD versus internal install, suspend and resume, double input with Steam running, prefix and log permissions. Unit tests cover the shortcut writer, the microSD check and the wrapper's behaviour.

## Profiles this covers

| Profile | Evidence |
| --- | --- |
| Steam Deck · SteamOS 3 · RDNA2 (`amd-baseline`) · Game Mode and Desktop | **this record**: verified on real hardware with EAC |
| AMD RDNA4 · CachyOS · KDE Wayland | verified in v2.2.0 (`docs/release-evidence/59ae4837bfde5a72e010741cc8792e6b08993f4a/`). Its launch settings are unchanged in v2.3.0 (covered by `TestSingleConfigPerVendor`). |
| everything else | expected, as labelled by the installer |

The installer in v2.3.0 still labels the Deck profile "expected" (only RDNA4 is marked verified in code); relabelling it is a code change, so it follows in the next release.
