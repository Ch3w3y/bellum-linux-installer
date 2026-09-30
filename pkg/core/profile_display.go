package core

import (
	"strings"
)

// Step 3: what the review screen shows for a detected platform, and the
// launch profile the GPU evidence selects. Both are pure functions of the
// Platform snapshot, so they are tested from the golden fixtures.

// ProfileStatus says whether a launch profile has hardware evidence.
type ProfileStatus string

const (
	// ProfileVerified has been played on real hardware with an Easy
	// Anti-Cheat online session.
	ProfileVerified ProfileStatus = "verified"
	// ProfileExpected is derived from the code and specifications and not yet
	// tested on real hardware.
	ProfileExpected ProfileStatus = "expected"
)

// LaunchProfile identifies the launch settings a GPU gets. There is exactly
// one per GPU class; it is detected, never chosen.
type LaunchProfile string

const (
	LaunchNVIDIARTX   LaunchProfile = "nvidia-rtx"
	LaunchNVIDIABasic LaunchProfile = "nvidia"
	LaunchAMDRDNA4    LaunchProfile = "amd-rdna4"
	LaunchAMDRDNA3    LaunchProfile = "amd-rdna3"
	// LaunchAMDBaseline covers RDNA2 (the Steam Deck included), RDNA1, GCN
	// and any AMD GPU whose generation is unknown or ambiguous.
	LaunchAMDBaseline LaunchProfile = "amd-baseline"
	LaunchIntel       LaunchProfile = "intel"
	LaunchGeneric     LaunchProfile = "generic"
)

// LaunchProfileFor maps GPU evidence to its launch profile. Ambiguous or
// unknown AMD generations take the conservative baseline.
func LaunchProfileFor(caps GPUCapabilities) LaunchProfile {
	switch caps.Vendor {
	case GPUNVIDIA:
		if caps.NVAPI {
			return LaunchNVIDIARTX
		}
		return LaunchNVIDIABasic
	case GPUAMD:
		if caps.Ambiguous {
			return LaunchAMDBaseline
		}
		switch caps.Generation {
		case "RDNA4":
			if caps.FSR41 {
				return LaunchAMDRDNA4
			}
		case "RDNA3":
			return LaunchAMDRDNA3
		}
		return LaunchAMDBaseline
	case GPUIntel:
		return LaunchIntel
	}
	return LaunchGeneric
}

// Status reports whether a launch profile has been tested on real hardware.
// RDNA4 was verified on an RX 9070 XT (v2.2.0) and NVIDIA RTX on an RTX 5070
// Ti (v2.4.0); every other profile ships as expected until a tester reports
// back on it.
func (l LaunchProfile) Status() ProfileStatus {
	if l == LaunchAMDRDNA4 || l == LaunchNVIDIARTX {
		return ProfileVerified
	}
	return ProfileExpected
}

// UsesFSR4Upgrade reports whether the profile forces Proton's FSR4 offer
// (PROTON_FSR4_UPGRADE=1). Only RDNA4 has the native FP8 path.
func (l LaunchProfile) UsesFSR4Upgrade() bool { return l == LaunchAMDRDNA4 }

// UsesRDNA3Workaround reports whether the profile sets
// DXIL_SPIRV_CONFIG=wmma_rdna3_workaround. It targets RDNA3's matrix
// instructions; RDNA2 and older lack them, so FSR4 through FP16 emulation
// is expected to cost more than it saves there and the game keeps its own
// FSR 3.x instead.
func (l LaunchProfile) UsesRDNA3Workaround() bool { return l == LaunchAMDRDNA3 }

// SKULabel is the display name of a hardware SKU, or "" for generic and
// unknown hardware.
func SKULabel(sku HardwareSKU) string {
	switch sku {
	case SKUDeckLCD:
		return "Steam Deck LCD"
	case SKUDeckOLED:
		return "Steam Deck OLED"
	case SKUSteamMachineCandidate:
		return "Steam Machine (provisional)"
	}
	return ""
}

// IsValveHardware reports an exact or provisional Valve SKU.
func IsValveHardware(sku HardwareSKU) bool {
	return sku == SKUDeckLCD || sku == SKUDeckOLED || sku == SKUSteamMachineCandidate
}

// OSLabel is a short display name for the distribution: "SteamOS 3",
// "Bazzite", otherwise the os-release PRETTY_NAME, ID or "Unknown Linux".
func OSLabel(o OSInfo) string {
	major := o.VersionID
	if i := strings.IndexByte(major, '.'); i >= 0 {
		major = major[:i]
	}
	switch o.ID {
	case "steamos":
		return strings.TrimSpace("SteamOS " + SanitizeField(major, 16))
	case "bazzite":
		if strings.Contains(strings.ToLower(o.VariantID), "deck") {
			return "Bazzite (Deck image)"
		}
		return "Bazzite"
	}
	if name := SanitizeField(o.PrettyName, 64); name != "" {
		return name
	}
	if id := SanitizeField(o.ID, 32); id != "" {
		return id
	}
	return "Unknown Linux"
}

// GPULabel is "RDNA2" for AMD with a known generation, "NVIDIA Blackwell",
// "NVIDIA", "Intel" or "Unknown GPU".
func GPULabel(caps GPUCapabilities) string {
	switch {
	case caps.Vendor == GPUAMD && caps.Generation != "":
		return caps.Generation
	case caps.Vendor == GPUAMD:
		return "AMD"
	case caps.Vendor == GPUNVIDIA && caps.Generation != "":
		return "NVIDIA " + caps.Generation
	case caps.Vendor == GPUNVIDIA || caps.Vendor == GPUIntel:
		return string(caps.Vendor)
	}
	return "Unknown GPU"
}

// SessionLabel is "Game Mode", "gamescope", "Wayland", "X11" or "".
func SessionLabel(s SessionInfo) string {
	switch s.Kind {
	case SessionGamescope:
		if s.GameMode.Normalize() == TriYes {
			return "Game Mode"
		}
		return "gamescope"
	case SessionWayland:
		return "Wayland"
	case SessionX11:
		return "X11"
	}
	return ""
}

// PlatformLabel is the one-line detected profile for the review screen, for
// example "Steam Deck OLED · SteamOS 3 · RDNA2 · Game Mode".
func PlatformLabel(p Platform) string {
	var parts []string
	for _, s := range []string{SKULabel(p.Hardware.SKU), OSLabel(p.OS), GPULabel(p.GPU), SessionLabel(p.Session)} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " · ")
}

// SupportsGameModeIntegration reports platforms where Bellum should be
// launched through Steam: Valve hardware, SteamOS, Bazzite's Deck images and
// any active Game Mode session. On these, Steam Input is what turns the
// built-in controls into a gamepad the game can see.
func SupportsGameModeIntegration(p Platform) bool {
	if IsValveHardware(p.Hardware.SKU) || p.OS.ID == "steamos" {
		return true
	}
	if p.OS.ID == "bazzite" && strings.Contains(strings.ToLower(p.OS.VariantID), "deck") {
		return true
	}
	return p.Session.GameMode.Normalize() == TriYes
}
