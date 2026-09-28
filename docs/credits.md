# Credits

## Project lineage

[Joheb Rahman (joepaji)](https://github.com/joepaji) created the
[original Bellum Linux Installer](https://github.com/joepaji/bellum-linux-installer)
and continues to own that project. This repository is an independently
maintained successor by Ch3w3y and contributors. Its Git history keeps Joheb's
foundational commits and authorship. The original project was published
without a license; its code remains Joheb's, and it is covered by this
repository's [MIT license](../LICENSE) only once he agrees.

The history also records the original project's v2.1.0 commit as an ancestor.
That records lineage only: v2.1.0 bundled an EAC runtime and changed launch and
install behaviour in ways this successor does not ship, and a user reported
that its uninstaller could delete their home folder when pointed at `/`.
Upstream changes are reviewed and credited before anything is adopted here.

This is a community project, not official support from Astarte Industries.
**Bellum**, the **Astarte Launcher** and their names and logos are the property
of Astarte Industries. The launcher icon in `packages/` is used only to
identify the game in the installer banner and desktop shortcut.

## Third-party software

Nothing below is copied into this repository or its releases unless stated.
Everything else is downloaded from its official source at install time, or
comes with Steam, and is verified before use.

| Component | Author | License | How Bellum uses it |
| --- | --- | --- | --- |
| [Proton-CachyOS](https://github.com/CachyOS/proton-cachyos) | CachyOS, built on Valve's [Proton](https://github.com/ValveSoftware/Proton) and [Wine](https://www.winehq.org/) | Proton's BSD-3-Clause scripts plus the licenses of its components (Wine: LGPL-2.1-or-later) | Downloaded at install time, pinned by SHA-256, used unmodified |
| [DXVK](https://github.com/doitsujin/dxvk), [vkd3d-proton](https://github.com/HansKristian-Work/vkd3d-proton), [dxvk-nvapi](https://github.com/jp7677/dxvk-nvapi) | their respective authors | zlib, LGPL-2.1-or-later, MIT | Shipped inside Proton-CachyOS |
| [winetricks](https://github.com/Winetricks/winetricks) | the Winetricks authors | LGPL-2.1-or-later | The copy shipped inside Proton-CachyOS |
| [umu-launcher](https://github.com/Open-Wine-Components/umu-launcher) | Open Wine Components | GPL-3.0 | Downloaded at install time (zipapp), pinned by SHA-256 |
| Proton EasyAntiCheat Runtime (Steam app 1826330) | Valve / Epic Games | proprietary | Installed through your Steam client; never downloaded or redistributed by Bellum |
| Astarte Launcher | Astarte Industries | proprietary | Downloaded from Astarte's official release server; its Authenticode signature is verified |
| [GlobalSign Code Signing Root R45](https://secure.globalsign.com/cacert/codesigningrootr45.crt) | GlobalSign | public root certificate | Copied into this repository (`pkg/packages/trust/`) and pinned by fingerprint, to verify the launcher's signature |
| [Go](https://go.dev/) and its standard library | The Go Authors | BSD-3-Clause | Compiles the installer |
| [github.com/xi2/xz](https://github.com/xi2/xz) | Michael Cross, Lasse Collin and Igor Pavlov (based on XZ Embedded) | public domain | Compiled into the installer to unpack `.tar.xz` archives |

The README screenshots are rendered with [pyte](https://github.com/selectel/pyte)
(LGPL-3.0) and [Playwright](https://playwright.dev/) (Apache-2.0), which are
development tools only.

## Contributors

Thanks to everyone who has contributed code, testing and hardware reports,
including testing on real hardware with Easy Anti-Cheat.
