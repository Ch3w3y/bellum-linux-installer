# Step-2 collection gaps (SOF-7)

All golden fixtures in this matrix are synthetic, constructed from the SOF-4
contract after the board's SOF-6 gate revision (2026-09-28): real hardware
captures are optional future QA targets, not a step-2 prerequisite. Nothing
here is labeled a real capture.

Covered by synthetic fixtures: Deck LCD/OLED exact mappings, provisional
Steam Machine predicate and its negatives, NVIDIA proprietary and open module
flavors, nouveau/NVK and mixed-adapter ambiguity, native/Flatpak/Snap Steam
kinds, simultaneous installs, symlink/cycle/escape handling, permission and
malformed faults, controller inventories, Steam running/stopped/unknown.

Upstream-derived (not yet imported): additional os-release family aliases are
exercised in unit tests from documented distro values rather than image
captures.

Unverified QA gaps (not settled by golden tests): gameplay performance,
EAC compatibility, Vulkan runtime health, controller routing through Steam
Input, hotplug behavior, RDNA3 tuning, and any hardware certification. The
review example `Steam Deck OLED · SteamOS 3 · RDNA2 · Game Mode` remains a
formatter contract for step 3, not a claim established here.
