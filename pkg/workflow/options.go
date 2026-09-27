package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bellum-installer/pkg/core"
)

// There is one configuration: the most stable one, then the fastest that
// stays stable. It differs only by GPU vendor (see configure.go); nothing is
// left for the user to choose. Every setting is a Proton or driver feature;
// the installer never adds or replaces DLLs (#11).

// ConfigSummary describes, for the confirmation screen, what the launch
// settings will be on this GPU.
func ConfigSummary(caps core.GPUCapabilities) string {
	switch {
	case caps.Vendor == core.GPUNVIDIA && caps.NVAPI:
		return "NVIDIA: DLSS and Reflex through the driver (NVAPI), CUDA/NVENC bridges"
	case caps.Vendor == core.GPUNVIDIA:
		return "NVIDIA (no RTX features): standard Proton settings"
	case caps.Vendor == core.GPUAMD && caps.Generation == "RDNA4" && !caps.Ambiguous && caps.FSR41:
		return "AMD RDNA4: native FSR4 (FP8) through Proton"
	case caps.Vendor == core.GPUAMD:
		return "AMD: FSR4 through Proton where your GPU supports it"
	case caps.Vendor == core.GPUIntel:
		return "Intel: standard Proton settings"
	}
	return "Unrecognised GPU: standard Proton settings"
}

// DisplaySession names the graphical session. Bellum runs through XWayland
// on Wayland sessions (the pinned Proton only uses its Wayland driver when
// PROTON_ENABLE_WAYLAND is set, which stays off because it's experimental),
// so the launch settings are the same on X11, Wayland and gamescope.
func DisplaySession() string {
	return displaySession(os.Getenv)
}

func displaySession(env func(string) string) string {
	desktop := env("XDG_CURRENT_DESKTOP")
	switch {
	case strings.EqualFold(desktop, "gamescope") || env("GAMESCOPE_WAYLAND_DISPLAY") != "":
		return "gamescope (Steam Deck / Game Mode)"
	case env("WAYLAND_DISPLAY") != "" || strings.EqualFold(env("XDG_SESSION_TYPE"), "wayland"):
		return strings.TrimSpace("Wayland " + desktop + " (the game runs through XWayland)")
	case env("DISPLAY") != "":
		return strings.TrimSpace("X11 " + desktop)
	}
	return "no graphical session detected"
}

// launchVarsToggles documents the optional wrapper extras. They stay off:
// each one adds a layer that hasn't been validated with Easy Anti-Cheat.
func launchVarsToggles() string {
	return "# Optional extras, off by default (set to 1 to try them)\n" +
		"export BELLUM_MANGOHUD=\"0\"   # MangoHud overlay\n" +
		"export BELLUM_GAMEMODE=\"0\"   # Feral GameMode\n" +
		"export BELLUM_GAMESCOPE=\"0\"  # run inside gamescope\n"
}

// DefaultInstallLocation is where Bellum goes when the user just presses Enter.
func DefaultInstallLocation() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Games", "Bellum")
}

// PromptInstallLocation asks where to install, defaulting to ~/Games/Bellum.
// Typing "b" opens the graphical folder picker.
func PromptInstallLocation(logger *core.Logger) (string, error) {
	return promptInstallLocationWith(logger, core.Prompt, PickWINEPREFIXWithGUI)
}

func promptInstallLocationWith(logger *core.Logger, prompt func(string, string) string, pick func(*core.Logger) (string, error)) (string, error) {
	def := DefaultInstallLocation()
	fmt.Println()
	fmt.Println("Where should Bellum be installed? Use a fast SSD with plenty of free space.")
	answer := strings.TrimSpace(prompt(fmt.Sprintf("Press Enter for %s, type a folder, or type b to browse: ", def), def))
	switch {
	case strings.EqualFold(answer, "b"):
		return pick(logger)
	case answer == "~":
		answer, _ = os.UserHomeDir()
	case strings.HasPrefix(answer, "~/"):
		home, _ := os.UserHomeDir()
		answer = filepath.Join(home, answer[2:])
	}
	return ResolvePrefixPath(answer)
}
